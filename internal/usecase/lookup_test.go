package usecase

import (
	"context"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// mockRepo is a test double for domain.LDAPRepository.
// TODO(copilot): implement all methods of domain.LDAPRepository
//   - Each method should return the configured mock values
//   - Store call counts to verify methods were/weren't called
type mockRepo struct {
	lookupResult    *domain.Account
	lookupErr       error
	batchAccounts   []*domain.Account
	batchNotFound   []string
	batchErr        error
	lookupCalled    int
	batchCalled     int
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
		{name: "valid external user", username: "alumni@example.com", attributes: []string{"mail"}, mockResult: &domain.Account{UID: "alumni@example.com", Source: domain.SourceExternal}, wantCalled: true},
		{name: "invalid username", username: "user)(evil", attributes: []string{"mail"}, wantErr: domain.ErrInvalidUsername, wantCalled: false},
		{name: "disallowed attribute", username: "110550001", attributes: []string{"userPassword"}, wantErr: domain.ErrAttributeNotAllowed, wantCalled: false},
		{name: "account not found", username: "nonexistent", attributes: []string{"mail"}, mockErr: domain.ErrAccountNotFound, wantErr: domain.ErrAccountNotFound, wantCalled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create mockRepo with tt.mockResult/tt.mockErr
			//   - Call LookupService.Lookup
			//   - If wantErr: assert errors.Is(err, tt.wantErr)
			//   - If wantCalled is false: assert repo was NOT called (validation should short-circuit)
			//   - If wantCalled is true: assert repo was called exactly once
			panic("not implemented")
		})
	}
}

func TestLookupService_LookupBatch(t *testing.T) {
	tests := []struct {
		name       string
		usernames  []string
		attributes []string
		wantErr    bool
	}{
		{name: "valid batch", usernames: []string{"110550001", "T1234"}, attributes: []string{"mail"}, wantErr: false},
		{name: "batch exceeds limit", usernames: make([]string, 51), attributes: []string{"mail"}, wantErr: true},
		{name: "one invalid username in batch", usernames: []string{"valid1", "bad)(user"}, attributes: []string{"mail"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Batch size > 50 MUST return error without calling repo
			//   - Invalid username MUST return ErrInvalidUsername without calling repo
			panic("not implemented")
		})
	}
}

// Ensure imports are used.
var _ context.Context
