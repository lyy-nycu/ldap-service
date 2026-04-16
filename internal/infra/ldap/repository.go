package ldap

import (
	"context"

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
//   1. Try internal pool first
//   2. On ErrAccountNotFound → try external pool
//   3. On connection error from internal → log, still try external
//   4. Return ErrServiceUnavailable only if BOTH fail with connection errors
type Repository struct {
	internal domain.LDAPPool
	external domain.LDAPPool
	logger   *zap.Logger
}

// NewRepository creates a Repository with two pool instances.
func NewRepository(internal, external domain.LDAPPool, logger *zap.Logger) *Repository {
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
//   1. account, err := r.internal.Search(ctx, username, attributes)
//   2. If err == nil → return account (found in internal)
//   3. If errors.Is(err, domain.ErrAccountNotFound) → try r.external.Search
//   4. If err is a connection error → log it, try r.external.Search
//   5. If external also fails with connection error → return domain.ErrServiceUnavailable
func (r *Repository) Lookup(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	panic("not implemented")
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
	panic("not implemented")
}

// Authenticate verifies a user's password via LDAP bind.
// See domain.LDAPRepository.Authenticate for acceptance criteria.
//
// Implementation guide:
//   1. Search internal pool for user's DN
//   2. If not found in internal, search external pool
//   3. If found: call Bind on the SAME pool that found the user
//   4. Return (true, nil) on successful bind
//   5. Return (false, nil) for ALL failures — never return an error that reveals the reason
//   6. NEVER log the password parameter
func (r *Repository) Authenticate(ctx context.Context, username string, password string) (bool, error) {
	panic("not implemented")
}

// HealthCheck verifies both LDAP sources are reachable.
// See domain.LDAPRepository.HealthCheck for acceptance criteria.
func (r *Repository) HealthCheck(ctx context.Context) error {
	panic("not implemented")
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
