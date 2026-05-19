package ldap

import (
	"context"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// Modify atomically replaces attributes on the subject identified by
// subjectID, fanning out across both LDAP sources (internal first,
// external on not-found).
//
// See domain.LDAPRepository.Modify for the full acceptance criteria.
//
// Implementation guide:
//  1. err := r.internal.Modify(ctx, subjectID, attrs)
//  2. if err == nil → return nil
//  3. if errors.Is(err, domain.ErrSchemaViolation) → return err (authoritative)
//  4. if errors.Is(err, domain.ErrAccountNotFound) → try r.external.Modify
//  5. if connection error from internal → log + try r.external.Modify
//  6. mirror Lookup's "both unavailable" handling: if BOTH return a
//     non-NotFound non-Schema error, return domain.ErrServiceUnavailable.
//  7. NEVER log attr values (passwords pass through here).
func (r *Repository) Modify(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
	panic("not implemented")
}
