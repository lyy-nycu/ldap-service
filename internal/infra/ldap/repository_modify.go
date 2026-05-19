package ldap

import (
	"context"
	"errors"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
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
	err := r.internal.Modify(ctx, subjectID, attrs)
	if err == nil {
		return nil
	}
	// Schema rejection is authoritative — never fan over.
	if errors.Is(err, domain.ErrSchemaViolation) {
		return err
	}

	if !errors.Is(err, domain.ErrAccountNotFound) {
		r.logger.Warn("internal ldap modify failed, trying external",
			zap.String("subject_id", subjectID),
			zap.Error(err))
	}

	extErr := r.external.Modify(ctx, subjectID, attrs)
	if extErr == nil {
		return nil
	}
	if errors.Is(extErr, domain.ErrSchemaViolation) {
		return extErr
	}
	if errors.Is(err, domain.ErrAccountNotFound) && errors.Is(extErr, domain.ErrAccountNotFound) {
		return domain.ErrAccountNotFound
	}
	if errors.Is(extErr, domain.ErrAccountNotFound) {
		// internal had a transport error, external definitively says
		// not found → bubble the transport-class failure as unavailable.
		return domain.ErrServiceUnavailable
	}
	if errors.Is(err, domain.ErrAccountNotFound) {
		// internal said not-found, external had a transport error →
		// caller should retry, not see a 404.
		return domain.ErrServiceUnavailable
	}
	return domain.ErrServiceUnavailable
}
