package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// ---------------------------------------------------------------------------
// mockAuthRepo — minimal mock implementing domain.LDAPRepository for auth tests
// ---------------------------------------------------------------------------

type mockAuthRepo struct {
	authOK       bool
	authErr      error
	authCalled   int
	lastUsername  string
	lastPassword string

	// Unused methods required by interface — panics make sure they're not called accidentally.
	domain.LDAPRepository
}

func (m *mockAuthRepo) Authenticate(ctx context.Context, username string, password string) (bool, error) {
	m.authCalled++
	m.lastUsername = username
	m.lastPassword = password
	return m.authOK, m.authErr
}

// Override interface methods that shouldn't be called in auth tests.
func (m *mockAuthRepo) Lookup(context.Context, string, []string) (*domain.Account, error) {
	panic("Lookup should not be called in authenticate tests")
}
func (m *mockAuthRepo) LookupBatch(context.Context, []string, []string) ([]*domain.Account, []string, error) {
	panic("LookupBatch should not be called in authenticate tests")
}
func (m *mockAuthRepo) HealthCheck(context.Context) error {
	panic("HealthCheck should not be called in authenticate tests")
}

// ---------------------------------------------------------------------------
// AuthenticateService — full locked tests (security-critical)
// ---------------------------------------------------------------------------

func TestAuthenticateService_Authenticate(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		mockOK     bool
		mockErr    error
		wantOK     bool
		wantErr    error
		wantCalled bool // should repository be called?
	}{
		// --- Success cases ---
		{
			name:       "successful internal user auth",
			username:   "110550001",
			password:   "correct-pw",
			mockOK:     true,
			wantOK:     true,
			wantErr:    nil,
			wantCalled: true,
		},
		{
			name:       "successful external user auth (email)",
			username:   "alumni@example.com",
			password:   "correct-pw",
			mockOK:     true,
			wantOK:     true,
			wantErr:    nil,
			wantCalled: true,
		},
		{
			name:       "successful employee auth",
			username:   "T1234",
			password:   "correct-pw",
			mockOK:     true,
			wantOK:     true,
			wantErr:    nil,
			wantCalled: true,
		},

		// --- Failure cases: ALL must return (false, ErrAuthenticationFailed) ---
		{
			name:       "wrong password",
			username:   "110550001",
			password:   "wrong",
			mockOK:     false,
			mockErr:    nil,
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: true,
		},
		{
			name:       "user not found (repo returns false, nil)",
			username:   "nonexistent",
			password:   "pass",
			mockOK:     false,
			mockErr:    nil,
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: true,
		},
		{
			name:       "repo returns unexpected error",
			username:   "110550001",
			password:   "pass",
			mockOK:     false,
			mockErr:    errors.New("connection reset"),
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: true,
		},

		// --- Validation failures: repo MUST NOT be called ---
		{
			name:       "invalid username (LDAP injection)",
			username:   "bad)(user",
			password:   "pass",
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: false,
		},
		{
			name:       "invalid username (wildcard)",
			username:   "user*",
			password:   "pass",
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: false,
		},
		{
			name:       "empty username",
			username:   "",
			password:   "pass",
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: false,
		},
		{
			name:       "empty password",
			username:   "110550001",
			password:   "",
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: false,
		},
		{
			name:       "whitespace-only password",
			username:   "110550001",
			password:   "   ",
			wantOK:     false,
			wantErr:    domain.ErrAuthenticationFailed,
			wantCalled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockAuthRepo{authOK: tt.mockOK, authErr: tt.mockErr}
			svc := NewAuthenticateService(repo, zap.NewNop())

			ok, err := svc.Authenticate(context.Background(), tt.username, tt.password)

			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("err = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			}

			if tt.wantCalled && repo.authCalled != 1 {
				t.Errorf("repo.Authenticate called %d times, want 1", repo.authCalled)
			}
			if !tt.wantCalled && repo.authCalled != 0 {
				t.Errorf("repo.Authenticate called %d times, want 0 (validation should short-circuit)", repo.authCalled)
			}
		})
	}
}

// TestAuthenticateService_IdenticalErrorForAllFailures is the critical
// security assertion: every failure reason produces the EXACT SAME error.
// An attacker must not be able to distinguish username-not-found from
// wrong-password from invalid-username from empty-password.
func TestAuthenticateService_IdenticalErrorForAllFailures(t *testing.T) {
	failureCases := []struct {
		name     string
		username string
		password string
		mockOK   bool
		mockErr  error
	}{
		{name: "invalid username", username: "bad)(user", password: "pass"},
		{name: "empty username", username: "", password: "pass"},
		{name: "empty password", username: "110550001", password: ""},
		{name: "wrong password", username: "110550001", password: "wrong", mockOK: false},
		{name: "user not found", username: "nobody", password: "pass", mockOK: false},
		{name: "repo error", username: "110550001", password: "pass", mockErr: errors.New("conn reset")},
	}

	var firstErr error
	for i, tt := range failureCases {
		repo := &mockAuthRepo{authOK: tt.mockOK, authErr: tt.mockErr}
		svc := NewAuthenticateService(repo, zap.NewNop())

		ok, err := svc.Authenticate(context.Background(), tt.username, tt.password)
		if ok {
			t.Fatalf("case %q returned ok=true, want false", tt.name)
		}
		if err == nil {
			t.Fatalf("case %q returned err=nil, want ErrAuthenticationFailed", tt.name)
		}

		if i == 0 {
			firstErr = err
		} else {
			// Every failure must produce the identical error.
			if err.Error() != firstErr.Error() {
				t.Errorf("case %q error = %q, differs from first failure error %q — all failures must be indistinguishable", tt.name, err.Error(), firstErr.Error())
			}
		}
	}
}

// TestAuthenticateService_NeverLogsPassword verifies that no log entry
// produced during authentication contains the password string.
func TestAuthenticateService_NeverLogsPassword(t *testing.T) {
	password := "SuperSecretP@ssw0rd!"

	scenarios := []struct {
		name   string
		mockOK bool
	}{
		{name: "successful auth", mockOK: true},
		{name: "failed auth", mockOK: false},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel) // capture ALL levels
			logger := zap.New(core)

			repo := &mockAuthRepo{authOK: sc.mockOK}
			svc := NewAuthenticateService(repo, logger)

			_, _ = svc.Authenticate(context.Background(), "110550001", password)

			for _, entry := range logs.All() {
				// Check message.
				if strings.Contains(entry.Message, password) {
					t.Fatalf("log message contains password: %q", entry.Message)
				}
				// Check all field values.
				for _, field := range entry.Context {
					if strings.Contains(field.String, password) {
						t.Fatalf("log field %q contains password: %q", field.Key, field.String)
					}
				}
			}

			// Also verify the password was passed to the repo correctly (not corrupted).
			if sc.mockOK && repo.lastPassword != password {
				t.Errorf("repo received password %q, want %q", repo.lastPassword, password)
			}
		})
	}
}

// TestAuthenticateService_PassesUsernameToRepo verifies the username reaches
// the repository unchanged.
func TestAuthenticateService_PassesUsernameToRepo(t *testing.T) {
	repo := &mockAuthRepo{authOK: true}
	svc := NewAuthenticateService(repo, zap.NewNop())

	_, _ = svc.Authenticate(context.Background(), "T1234", "pw")

	if repo.lastUsername != "T1234" {
		t.Errorf("repo received username %q, want %q", repo.lastUsername, "T1234")
	}
}
