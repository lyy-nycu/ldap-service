package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// mockRepo is a test double for domain.LDAPRepository.
// TODO(copilot): implement all methods of domain.LDAPRepository
//   - Each method should return the configured mock values
//   - Store call counts to verify methods were/weren't called
type mockRepo struct {
	lookupResult  *domain.Account
	lookupErr     error
	batchAccounts []*domain.Account
	batchNotFound []string
	batchErr      error
	lookupCalled  int
	batchCalled   int
	authResult    *domain.AuthenticateResult
	authErr       error
	authCalled    int
	healthErr     error
	healthCalled  int
}

func (m *mockRepo) Lookup(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	m.lookupCalled++
	return m.lookupResult, m.lookupErr
}

func (m *mockRepo) LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*domain.Account, []string, error) {
	m.batchCalled++
	return m.batchAccounts, m.batchNotFound, m.batchErr
}

func (m *mockRepo) Authenticate(ctx context.Context, username string, password string) (*domain.AuthenticateResult, error) {
	m.authCalled++
	return m.authResult, m.authErr
}

func (m *mockRepo) HealthCheck(ctx context.Context) error {
	m.healthCalled++
	return m.healthErr
}

func (m *mockRepo) Modify(context.Context, string, []domain.ModifyAttr) error {
	return nil
}

func TestLookupService_Lookup(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		attributes []string
		mockResult *domain.Account
		mockErr    error
		wantErr    error
		wantCalled bool // should repository be called?
	}{
		{name: "valid internal user", username: "110550001", attributes: []string{"mail"}, mockResult: &domain.Account{UID: "110550001", Source: domain.SourceInternal}, wantCalled: true},
		{name: "valid lookup with fullName and initials", username: "110550001", attributes: []string{"fullName", "initials"}, mockResult: &domain.Account{UID: "110550001", Source: domain.SourceInternal, Attributes: map[string]string{"fullName": "Student User", "initials": "SU"}}, wantCalled: true},
		{name: "valid external user", username: "alumni@example.com", attributes: []string{"mail"}, mockResult: &domain.Account{UID: "alumni@example.com", Source: domain.SourceExternal}, wantCalled: true},
		{name: "invalid username", username: "user)(evil", attributes: []string{"mail"}, wantErr: domain.ErrInvalidUsername, wantCalled: false},
		{name: "disallowed attribute", username: "110550001", attributes: []string{"userPassword"}, wantErr: domain.ErrAttributeNotAllowed, wantCalled: false},
		{name: "account not found", username: "nonexistent", attributes: []string{"mail"}, mockErr: domain.ErrAccountNotFound, wantErr: domain.ErrAccountNotFound, wantCalled: true},
		{name: "service unavailable", username: "110550001", attributes: []string{"mail"}, mockErr: domain.ErrServiceUnavailable, wantErr: domain.ErrServiceUnavailable, wantCalled: true},
		{name: "empty attributes is valid", username: "110550001", attributes: []string{}, mockResult: &domain.Account{UID: "110550001", Source: domain.SourceInternal}, wantCalled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create mockRepo with tt.mockResult/tt.mockErr
			//   - Call LookupService.Lookup
			//   - If wantErr: assert errors.Is(err, tt.wantErr)
			//   - If wantCalled is false: assert repo was NOT called (validation should short-circuit)
			//   - If wantCalled is true: assert repo was called exactly once
			repo := &mockRepo{lookupResult: tt.mockResult, lookupErr: tt.mockErr}
			svc := NewLookupService(repo)

			got, err := svc.Lookup(context.Background(), tt.username, tt.attributes)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Lookup() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Lookup() error = %v, want nil", err)
			}

			if tt.wantCalled {
				if repo.lookupCalled != 1 {
					t.Fatalf("repo.Lookup called %d times, want 1", repo.lookupCalled)
				}
				if tt.mockResult != nil && got == nil {
					t.Fatal("Lookup() returned nil account, want non-nil")
				}
			} else if repo.lookupCalled != 0 {
				t.Fatalf("repo.Lookup called %d times, want 0", repo.lookupCalled)
			}
		})
	}
}

func TestLookupService_LookupBatch(t *testing.T) {
	tests := []struct {
		name       string
		usernames  []string
		attributes []string
		wantErr    bool
		wantErrIs  error
		wantCalled bool
	}{
		{name: "valid batch", usernames: []string{"110550001", "T1234"}, attributes: []string{"mail"}, wantErr: false, wantCalled: true},
		{name: "valid batch with fullName and initials", usernames: []string{"110550001", "T1234"}, attributes: []string{"fullName", "initials"}, wantErr: false, wantCalled: true},
		{name: "batch exceeds limit", usernames: make([]string, 51), attributes: []string{"mail"}, wantErr: true, wantErrIs: domain.ErrBatchSizeExceeded, wantCalled: false},
		{name: "one invalid username in batch", usernames: []string{"valid1", "bad)(user"}, attributes: []string{"mail"}, wantErr: true, wantErrIs: domain.ErrInvalidUsername, wantCalled: false},
		{name: "empty usernames slice", usernames: []string{}, attributes: []string{"mail"}, wantErr: false, wantCalled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Batch size > 50 MUST return error without calling repo
			//   - Invalid username MUST return ErrInvalidUsername without calling repo
			repo := &mockRepo{
				batchAccounts: []*domain.Account{{UID: "110550001", Source: domain.SourceInternal}},
				batchNotFound: []string{"missing"},
			}
			svc := NewLookupService(repo)

			accounts, notFound, err := svc.LookupBatch(context.Background(), tt.usernames, tt.attributes)

			if tt.wantErr {
				if err == nil {
					t.Fatal("LookupBatch() error = nil, want non-nil")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("LookupBatch() error = %v, want %v", err, tt.wantErrIs)
				}
			} else {
				if err != nil {
					t.Fatalf("LookupBatch() error = %v, want nil", err)
				}
				if tt.wantCalled {
					if accounts == nil {
						t.Fatal("LookupBatch() accounts = nil, want non-nil")
					}
					if notFound == nil {
						t.Fatal("LookupBatch() notFound = nil, want non-nil")
					}
				}
			}

			if tt.wantCalled {
				if repo.batchCalled != 1 {
					t.Fatalf("repo.LookupBatch called %d times, want 1", repo.batchCalled)
				}
			} else if repo.batchCalled != 0 {
				t.Fatalf("repo.LookupBatch called %d times, want 0", repo.batchCalled)
			}
		})
	}
}

// Ensure imports are used.
var _ context.Context
