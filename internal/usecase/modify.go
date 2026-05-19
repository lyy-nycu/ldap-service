package usecase

import (
	"context"
	"fmt"
	"regexp"
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

// schemePrefixRE matches an LDAP password storage-scheme prefix
// ({SSHA}, {ARGON2}, {CRYPT}, …). Pass-through values are forwarded to
// slapd verbatim so admin-reset / migration tooling can write a known
// hash. Anything that does not match this prefix is treated as
// plaintext, which slapd will hash per its password-hash directive so
// ppolicy (history, quality) can apply.
var schemePrefixRE = regexp.MustCompile(`^\{[A-Z0-9]+\}.+`)

// validateUserPassword enforces the relaxed contract introduced by the
// relax-modify-userpassword change.
//
//   - Empty or "no attrs at all" is caught upstream by ErrNoAttrsToModify.
//   - A value matching schemePrefixRE is accepted verbatim (pass-through).
//   - Any other value is treated as plaintext and must be ≤256 bytes
//     and free of NUL / C0 controls (0x00–0x1F) / DEL (0x7F).
//
// The returned error MUST NOT contain the offending value — it leaks
// into zap logs and the RFC 7807 problem-detail body otherwise.
func validateUserPassword(value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "{") && schemePrefixRE.MatchString(value) {
		return nil
	}
	if len(value) > 256 {
		return fmt.Errorf("%w: userpassword exceeds 256 bytes", domain.ErrInvalidAttrValue)
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b < 0x20 || b == 0x7f {
			return fmt.Errorf("%w: userpassword contains a disallowed control character", domain.ErrInvalidAttrValue)
		}
	}
	return nil
}

// Modify validates and atomically applies a Modify request.
//
// See domain.ModifyUseCase.Modify for the full acceptance criteria.
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
	if err := validateUserPassword(attrs.UserPassword); err != nil {
		return nil, err
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
