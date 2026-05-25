package ldap

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

// ldapGeneralizedTimeLayout is the Go layout for LDAP Generalized Time
// (RFC 4517 §3.3.13). The form used by OpenLDAP ppolicy `pwdChangedTime`
// is `yyyyMMddHHmmssZ` (always UTC, integer seconds).
const ldapGeneralizedTimeLayout = "20060102150405Z"

// stateAttrs are the LDAP attributes Repository.Authenticate requests from
// Search so it can build AuthenticateResult after a successful bind.
//
// These names are deliberately NOT included in domain.AllowedAttributes:
// they are infrastructure-internal and must never leak via the Lookup API.
var stateAttrs = []string{"disable", "pwdReset", "pwdChangedTime"}

// ---------------------------------------------------------------------------
// Repository — fan-out orchestrator across two LDAP pools
// ---------------------------------------------------------------------------

// Repository implements domain.LDAPRepository by fanning out requests
// across internal and external LDAP pools.
//
// Fan-out strategy:
//  1. Try internal pool first
//  2. On ErrAccountNotFound → try external pool
//  3. On connection error from internal → log, still try external
//  4. Return ErrServiceUnavailable only if BOTH fail with connection errors
type Repository struct {
	internal       domain.LDAPPool
	external       domain.LDAPPool
	passwordMaxAge time.Duration
	logger         *zap.Logger
}

// NewRepository creates a Repository with two pool instances.
//
// passwordMaxAge is the password expiry window applied to the LDAP entry's
// `pwdChangedTime`. Zero disables the time-based expiry check (the
// `pwdReset=TRUE` → must_change mapping still applies).
func NewRepository(internal, external domain.LDAPPool, passwordMaxAge time.Duration, logger *zap.Logger) *Repository {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Repository{
		internal:       internal,
		external:       external,
		passwordMaxAge: passwordMaxAge,
		logger:         logger,
	}
}

// Lookup finds an account by username across both LDAP sources.
// See domain.LDAPRepository.Lookup for acceptance criteria.
//
// Implementation guide:
//  1. account, err := r.internal.Search(ctx, username, attributes)
//  2. If err == nil → return account (found in internal)
//  3. If errors.Is(err, domain.ErrAccountNotFound) → try r.external.Search
//  4. If err is a connection error → log it, try r.external.Search
//  5. If external also fails with connection error → return domain.ErrServiceUnavailable
func (r *Repository) Lookup(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	account, err := r.internal.Search(ctx, username, attributes)
	if err == nil {
		return account, nil
	}

	if !errors.Is(err, domain.ErrAccountNotFound) {
		r.logger.Warn("internal ldap search failed, trying external", zap.Error(err))
	}

	account, extErr := r.external.Search(ctx, username, attributes)
	if extErr == nil {
		return account, nil
	}

	if errors.Is(extErr, domain.ErrAccountNotFound) {
		return nil, domain.ErrAccountNotFound
	}

	if errors.Is(err, domain.ErrAccountNotFound) {
		return nil, extErr
	}

	return nil, domain.ErrServiceUnavailable
}

// LookupBatch finds multiple accounts across both LDAP sources.
// See domain.LDAPRepository.LookupBatch for acceptance criteria.
//
// Implementation guide:
//   - Iterate each username, call r.Lookup() for each
//   - Collect found accounts in []*Account
//   - Collect not-found usernames in []string
//   - If Lookup returns ErrAccountNotFound, add to not_found (not an error)
//   - If Lookup returns other errors, return immediately
func (r *Repository) LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*domain.Account, []string, error) {
	found := make([]*domain.Account, 0, len(usernames))
	notFound := make([]string, 0)

	for _, username := range usernames {
		account, err := r.Lookup(ctx, username, attributes)
		if err == nil {
			found = append(found, account)
			continue
		}

		if errors.Is(err, domain.ErrAccountNotFound) {
			notFound = append(notFound, username)
			continue
		}

		return nil, nil, err
	}

	return found, notFound, nil
}

// Authenticate verifies a user's password via LDAP bind and reports state.
// See domain.LDAPRepository.Authenticate for acceptance criteria.
//
// Implementation guide:
//  1. Search internal pool for the user, requesting state attributes
//  2. If not found in internal, search external pool
//  3. If found: call Bind on the SAME pool that found the user
//  4. On bind success: build AuthenticateResult from the entry's state attrs
//  5. On ANY failure: return (nil, nil) — never an error that reveals reason
//  6. NEVER log the password parameter
func (r *Repository) Authenticate(ctx context.Context, username string, password string) (*domain.AuthenticateResult, error) {
	if account, err := r.internal.Search(ctx, username, stateAttrs); err == nil {
		if bindErr := r.internal.Bind(ctx, account.DN, password); bindErr != nil {
			return nil, nil
		}
		return r.buildResult(account, username), nil
	} else if !errors.Is(err, domain.ErrAccountNotFound) {
		r.logger.Warn("internal ldap search failed during authenticate, trying external", zap.Error(err))
	}

	account, extErr := r.external.Search(ctx, username, stateAttrs)
	if extErr != nil {
		if !errors.Is(extErr, domain.ErrAccountNotFound) {
			r.logger.Warn("external ldap search failed during authenticate", zap.Error(extErr))
		}
		return nil, nil
	}

	if bindErr := r.external.Bind(ctx, account.DN, password); bindErr != nil {
		return nil, nil
	}

	return r.buildResult(account, username), nil
}

// buildResult derives an AuthenticateResult from a successfully-bound
// account's state attributes. requestUsername is used as a fallback when
// the LDAP entry has no uid attribute (defensive — historical NYCU outage
// was caused by assuming attributes always exist).
func (r *Repository) buildResult(account *domain.Account, requestUsername string) *domain.AuthenticateResult {
	uid := account.UID
	if uid == "" {
		uid = requestUsername
	}

	return &domain.AuthenticateResult{
		UID:           uid,
		AccountState:  deriveAccountState(account.Attributes),
		PasswordState: r.derivePasswordState(account.Attributes),
	}
}

// deriveAccountState maps the entry's `disable` attribute to an AccountState.
// Missing or unrecognized values default to active (fail-open).
//
//   disable=1 → pending_activation (NYCU semantics: not yet activated)
//   disable=0 or absent → active
func deriveAccountState(attrs map[string]string) domain.AccountState {
	switch attrs["disable"] {
	case "1":
		return domain.AccountStatePendingActivation
	default:
		return domain.AccountStateActive
	}
}

// derivePasswordState maps OpenLDAP ppolicy attributes to a PasswordState.
//
//   pwdReset=TRUE (case-insensitive) → must_change (admin-forced reset)
//   pwdChangedTime + r.passwordMaxAge < now → expired
//   else → current
//
// Generalized-Time parse failures are treated as `current` and logged at
// warn level (fail-open for availability: a malformed timestamp must not
// lock users out).
func (r *Repository) derivePasswordState(attrs map[string]string) domain.PasswordState {
	if strings.EqualFold(attrs["pwdReset"], "TRUE") {
		return domain.PasswordStateMustChange
	}

	if r.passwordMaxAge <= 0 {
		return domain.PasswordStateCurrent
	}

	raw := attrs["pwdChangedTime"]
	if raw == "" {
		return domain.PasswordStateCurrent
	}

	changed, err := time.Parse(ldapGeneralizedTimeLayout, raw)
	if err != nil {
		r.logger.Warn("failed to parse pwdChangedTime, treating password as current",
			zap.String("layout", ldapGeneralizedTimeLayout))
		return domain.PasswordStateCurrent
	}

	if time.Since(changed) > r.passwordMaxAge {
		return domain.PasswordStateExpired
	}

	return domain.PasswordStateCurrent
}

// HealthCheck verifies both LDAP sources are reachable.
// See domain.LDAPRepository.HealthCheck for acceptance criteria.
func (r *Repository) HealthCheck(ctx context.Context) error {
	intErr := r.internal.HealthCheck(ctx)
	extErr := r.external.HealthCheck(ctx)
	if intErr != nil || extErr != nil {
		return domain.ErrServiceUnavailable
	}

	return nil
}

// Close closes both pools.
func (r *Repository) Close() error {
	intErr := r.internal.Close()
	extErr := r.external.Close()
	if intErr != nil {
		return intErr
	}
	return extErr
}

// Compile-time interface check.
var _ domain.LDAPRepository = (*Repository)(nil)
