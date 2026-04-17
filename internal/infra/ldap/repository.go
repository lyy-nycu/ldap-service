package ldap

import (
	"context"
	"errors"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

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
	internal domain.LDAPPool
	external domain.LDAPPool
	logger   *zap.Logger
}

// NewRepository creates a Repository with two pool instances.
func NewRepository(internal, external domain.LDAPPool, logger *zap.Logger) *Repository {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Repository{
		internal: internal,
		external: external,
		logger:   logger,
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

// Authenticate verifies a user's password via LDAP bind.
// See domain.LDAPRepository.Authenticate for acceptance criteria.
//
// Implementation guide:
//  1. Search internal pool for user's DN
//  2. If not found in internal, search external pool
//  3. If found: call Bind on the SAME pool that found the user
//  4. Return (true, nil) on successful bind
//  5. Return (false, nil) for ALL failures — never return an error that reveals the reason
//  6. NEVER log the password parameter
func (r *Repository) Authenticate(ctx context.Context, username string, password string) (bool, error) {
	account, err := r.internal.Search(ctx, username, nil)
	if err == nil {
		if bindErr := r.internal.Bind(ctx, account.DN, password); bindErr != nil {
			return false, nil
		}
		return true, nil
	}

	if !errors.Is(err, domain.ErrAccountNotFound) {
		r.logger.Warn("internal ldap search failed during authenticate, trying external", zap.Error(err))
	}

	account, extErr := r.external.Search(ctx, username, nil)
	if extErr != nil {
		if !errors.Is(extErr, domain.ErrAccountNotFound) {
			r.logger.Warn("external ldap search failed during authenticate", zap.Error(extErr))
		}
		return false, nil
	}

	if bindErr := r.external.Bind(ctx, account.DN, password); bindErr != nil {
		return false, nil
	}

	return true, nil
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
