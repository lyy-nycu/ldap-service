package usecase

import (
	"context"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

func TestAuthenticateService_Authenticate(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		mockOK     bool  // what repo.Authenticate returns
		mockErr    error // what repo.Authenticate returns
		wantOK     bool
		wantErr    error
		wantCalled bool // should repository be called?
	}{
		{name: "successful auth", username: "110550001", password: "correct", mockOK: true, wantOK: true, wantErr: nil, wantCalled: true},
		{name: "wrong password", username: "110550001", password: "wrong", mockOK: false, wantOK: false, wantErr: domain.ErrAuthenticationFailed, wantCalled: true},
		{name: "user not found", username: "nonexistent", password: "pass", mockOK: false, wantOK: false, wantErr: domain.ErrAuthenticationFailed, wantCalled: true},
		{name: "invalid username format", username: "bad)(user", password: "pass", wantOK: false, wantErr: domain.ErrAuthenticationFailed, wantCalled: false},
		{name: "empty password", username: "110550001", password: "", wantOK: false, wantErr: domain.ErrAuthenticationFailed, wantCalled: false},
		{name: "external user auth", username: "alumni@example.com", password: "correct", mockOK: true, wantOK: true, wantErr: nil, wantCalled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - ALL failure cases MUST return (false, domain.ErrAuthenticationFailed)
			//   - The error MUST be identical regardless of failure reason (security requirement)
			//   - If wantCalled is false: repo MUST NOT be called (validation short-circuits)
			//   - Logger output MUST NOT contain the password string
			panic("not implemented")
		})
	}
}

// Ensure imports are used.
var _ context.Context
var _ = zap.NewNop
