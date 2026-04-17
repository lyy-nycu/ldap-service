package usecase

import (
	"context"
	"strings"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

// AuthenticateService implements domain.AuthenticateUseCase.
type AuthenticateService struct {
	repo   domain.LDAPRepository
	logger *zap.Logger
}

// NewAuthenticateService creates an AuthenticateService.
func NewAuthenticateService(repo domain.LDAPRepository, logger *zap.Logger) *AuthenticateService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &AuthenticateService{repo: repo, logger: logger}
}

// Authenticate verifies a user's password.
// See domain.AuthenticateUseCase.Authenticate for acceptance criteria.
//
// Implementation steps:
//  1. Validate username with domain.ValidateUsername()
//     — on failure, return (false, domain.ErrAuthenticationFailed) — NOT ErrInvalidUsername
//  2. Check password is non-empty
//     — on failure, return (false, domain.ErrAuthenticationFailed) — NOT a descriptive error
//  3. Call r.repo.Authenticate(ctx, username, password)
//  4. If repo returns (true, nil) → return (true, nil)
//  5. For ALL other cases → return (false, domain.ErrAuthenticationFailed)
//
// Security constraints:
//   - MUST NOT log the password at any level
//   - MUST NOT return different errors for different failure reasons
//   - MAY log: username, success/failure result, request ID
func (s *AuthenticateService) Authenticate(ctx context.Context, username string, password string) (bool, error) {
	if err := domain.ValidateUsername(username); err != nil {
		return false, domain.ErrAuthenticationFailed
	}

	if strings.TrimSpace(password) == "" {
		return false, domain.ErrAuthenticationFailed
	}

	ok, err := s.repo.Authenticate(ctx, username, password)
	if err != nil {
		return false, domain.ErrAuthenticationFailed
	}
	if !ok {
		return false, domain.ErrAuthenticationFailed
	}

	return true, nil
}

// Compile-time interface check.
var _ domain.AuthenticateUseCase = (*AuthenticateService)(nil)
