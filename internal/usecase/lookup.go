package usecase

import (
	"context"
	"fmt"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// MaxBatchSize is the maximum number of usernames in a batch lookup.
const MaxBatchSize = 50

// LookupService implements domain.LookupUseCase.
type LookupService struct {
	repo domain.LDAPRepository
}

// NewLookupService creates a LookupService.
func NewLookupService(repo domain.LDAPRepository) *LookupService {
	return &LookupService{repo: repo}
}

// Lookup finds a single account by username.
// See domain.LookupUseCase.Lookup for acceptance criteria.
//
// Implementation steps:
//  1. Validate username with domain.ValidateUsername()
//  2. Validate attributes with domain.ValidateAttributes()
//  3. Call r.repo.Lookup() — fan-out across sources is transparent
//  4. Return result or domain error
func (s *LookupService) Lookup(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	if err := domain.ValidateUsername(username); err != nil {
		return nil, err
	}

	if err := domain.ValidateAttributes(attributes); err != nil {
		return nil, err
	}

	return s.repo.Lookup(ctx, username, attributes)
}

// LookupBatch finds multiple accounts by username.
// See domain.LookupUseCase.LookupBatch for acceptance criteria.
//
// Implementation steps:
//  1. Validate batch size: len(usernames) <= MaxBatchSize, else return error
//  2. Validate each username with domain.ValidateUsername() — fail fast on first invalid
//  3. Validate attributes with domain.ValidateAttributes()
//  4. Call r.repo.LookupBatch()
func (s *LookupService) LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*domain.Account, []string, error) {
	if len(usernames) > MaxBatchSize {
		return nil, nil, fmt.Errorf("batch size exceeds maximum of %d", MaxBatchSize)
	}

	for _, username := range usernames {
		if err := domain.ValidateUsername(username); err != nil {
			return nil, nil, err
		}
	}

	if err := domain.ValidateAttributes(attributes); err != nil {
		return nil, nil, err
	}

	return s.repo.LookupBatch(ctx, usernames, attributes)
}

// Compile-time interface check.
var _ domain.LookupUseCase = (*LookupService)(nil)

// Ensure imports are used.
var _ = fmt.Sprintf
