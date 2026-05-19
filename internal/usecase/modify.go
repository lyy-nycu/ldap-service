package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// ModifyService implements domain.ModifyUseCase.
type ModifyService struct {
	repo domain.LDAPRepository
}

// NewModifyService creates a ModifyService.
func NewModifyService(repo domain.LDAPRepository) *ModifyService {
	return &ModifyService{repo: repo}
}

// Modify validates and atomically applies a Modify request.
//
// See domain.ModifyUseCase.Modify for the full acceptance criteria.
//
// Implementation steps (frozen by RED tests in modify_test.go):
//  1. if subjectID == "" → return ErrSubjectIDRequired
//  2. domain.ValidateUsername(subjectID) — reject LDAP injection in subject_id
//  3. if !attrs.HasAny() → return ErrNoAttrsToModify
//  4. if attrs.Disable != "" && attrs.Disable not in {"0","1"} → ErrInvalidAttrValue
//  5. if attrs.UserPassword != "" && !strings.HasPrefix(attrs.UserPassword, "{SSHA}")
//     → ErrInvalidAttrValue (producer-side guard per spec)
//  6. wire := attrs.ToWireMap()  (preserves the legacy "altemate-email" spelling)
//  7. err := s.repo.Modify(ctx, subjectID, wire)
//  8. on err: return as-is (handler maps to Problem)
//  9. on success: build ModifyResult.Modified from wire[i].Name, in the
//     SAME order ToWireMap returned. The caller uses this list to
//     verify atomic success without re-reading.
func (s *ModifyService) Modify(ctx context.Context, subjectID string, attrs domain.ModifyAttrs) (*domain.ModifyResult, error) {
	if subjectID == "" {
		return nil, domain.ErrSubjectIDRequired
	}
	if err := domain.ValidateUsername(subjectID); err != nil {
		return nil, err
	}
	if !attrs.HasAny() {
		return nil, domain.ErrNoAttrsToModify
	}
	if attrs.Disable != "" && attrs.Disable != "0" && attrs.Disable != "1" {
		return nil, fmt.Errorf("%w: disable must be \"0\" or \"1\"", domain.ErrInvalidAttrValue)
	}
	if attrs.UserPassword != "" && !strings.HasPrefix(attrs.UserPassword, "{SSHA}") {
		return nil, fmt.Errorf("%w: userpassword must be SSHA-hashed (start with {SSHA})", domain.ErrInvalidAttrValue)
	}

	wire := attrs.ToWireMap()
	if err := s.repo.Modify(ctx, subjectID, wire); err != nil {
		return nil, err
	}

	modified := make([]string, len(wire))
	for i, a := range wire {
		modified[i] = a.Name
	}
	return &domain.ModifyResult{Modified: modified}, nil
}

// Compile-time interface check.
var _ domain.ModifyUseCase = (*ModifyService)(nil)
