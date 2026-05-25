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

// Authenticate verifies a user's password and returns post-bind state.
// See domain.AuthenticateUseCase.Authenticate for acceptance criteria.
//
// Implementation steps:
//  1. Validate username with domain.ValidateUsername()
//     — on failure, return (nil, domain.ErrAuthenticationFailed)
//  2. Check password is non-empty
//     — on failure, return (nil, domain.ErrAuthenticationFailed)
//  3. Call r.repo.Authenticate(ctx, username, password)
//  4. If repo returns (result, nil) with result != nil → return (result, nil)
//  5. For ALL other cases → return (nil, domain.ErrAuthenticationFailed)
//
// Security constraints:
//   - MUST NOT log the password at any level
//   - MUST NOT return different errors for different failure reasons
//   - MAY log: username, success/failure result, request ID
func (s *AuthenticateService) Authenticate(ctx context.Context, username string, password string) (*domain.AuthenticateResult, error) {
	if err := domain.ValidateUsername(username); err != nil {
		return nil, domain.ErrAuthenticationFailed
	}

	if strings.TrimSpace(password) == "" {
		return nil, domain.ErrAuthenticationFailed
	}

	result, err := s.repo.Authenticate(ctx, username, password)
	if err != nil || result == nil {
		return nil, domain.ErrAuthenticationFailed
	}

	return result, nil
}

// Compile-time interface check.
var _ domain.AuthenticateUseCase = (*AuthenticateService)(nil)
