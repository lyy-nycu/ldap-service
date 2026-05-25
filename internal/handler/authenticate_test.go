package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// ---------------------------------------------------------------------------
// mockAuthUseCase — implements domain.AuthenticateUseCase
// ---------------------------------------------------------------------------

type mockAuthUseCase struct {
	result   *domain.AuthenticateResult
	err      error
	called   int
	lastUser string
	lastPW   string
}

func (m *mockAuthUseCase) Authenticate(_ context.Context, username, password string) (*domain.AuthenticateResult, error) {
	m.called++
	m.lastUser = username
	m.lastPW = password
	return m.result, m.err
}

// ---------------------------------------------------------------------------
// HandleAuthenticate — full locked tests (security-critical)
// ---------------------------------------------------------------------------

func TestHandleAuthenticate(t *testing.T) {
	defaultOK := &domain.AuthenticateResult{
		UID:           "110550001",
		AccountState:  domain.AccountStateActive,
		PasswordState: domain.PasswordStateCurrent,
	}

	tests := []struct {
		name              string
		method            string
		body              string
		mockResult        *domain.AuthenticateResult
		mockErr           error
		wantStatus        int
		wantCalled        bool // should use case be invoked?
		wantUserID        string
		wantAccountState  domain.AccountState
		wantPasswordState domain.PasswordState
	}{
		// --- Success — active/current ---
		{
			name:              "successful auth (active + current)",
			method:            http.MethodPost,
			body:              `{"username":"110550001","password":"correct"}`,
			mockResult:        defaultOK,
			wantStatus:        200,
			wantCalled:        true,
			wantUserID:        "110550001",
			wantAccountState:  domain.AccountStateActive,
			wantPasswordState: domain.PasswordStateCurrent,
		},
		// --- Success — pending_activation propagates (CR §2.1: still 200) ---
		{
			name:   "successful bind, account pending_activation",
			method: http.MethodPost,
			body:   `{"username":"110550001","password":"correct"}`,
			mockResult: &domain.AuthenticateResult{
				UID:           "110550001",
				AccountState:  domain.AccountStatePendingActivation,
				PasswordState: domain.PasswordStateCurrent,
			},
			wantStatus:        200,
			wantCalled:        true,
			wantUserID:        "110550001",
			wantAccountState:  domain.AccountStatePendingActivation,
			wantPasswordState: domain.PasswordStateCurrent,
		},
		// --- Success — password must_change propagates ---
		{
			name:   "successful bind, password must_change",
			method: http.MethodPost,
			body:   `{"username":"T1234","password":"correct"}`,
			mockResult: &domain.AuthenticateResult{
				UID:           "T1234",
				AccountState:  domain.AccountStateActive,
				PasswordState: domain.PasswordStateMustChange,
			},
			wantStatus:        200,
			wantCalled:        true,
			wantUserID:        "T1234",
			wantAccountState:  domain.AccountStateActive,
			wantPasswordState: domain.PasswordStateMustChange,
		},
		// --- Success — password expired propagates ---
		{
			name:   "successful bind, password expired",
			method: http.MethodPost,
			body:   `{"username":"T1234","password":"correct"}`,
			mockResult: &domain.AuthenticateResult{
				UID:           "T1234",
				AccountState:  domain.AccountStateActive,
				PasswordState: domain.PasswordStateExpired,
			},
			wantStatus:        200,
			wantCalled:        true,
			wantUserID:        "T1234",
			wantAccountState:  domain.AccountStateActive,
			wantPasswordState: domain.PasswordStateExpired,
		},
		// --- Auth failure ---
		{
			name:       "failed auth (wrong password)",
			method:     http.MethodPost,
			body:       `{"username":"110550001","password":"wrong"}`,
			mockResult: nil,
			mockErr:    domain.ErrAuthenticationFailed,
			wantStatus: 401,
			wantCalled: true,
		},
		// --- Request validation ---
		{
			name:       "invalid JSON body",
			method:     http.MethodPost,
			body:       `{bad`,
			wantStatus: 400,
			wantCalled: false,
		},
		{
			name:       "missing username",
			method:     http.MethodPost,
			body:       `{"password":"pass"}`,
			wantStatus: 400,
			wantCalled: false,
		},
		{
			name:       "empty username",
			method:     http.MethodPost,
			body:       `{"username":"","password":"pass"}`,
			wantStatus: 400,
			wantCalled: false,
		},
		{
			name:       "missing password",
			method:     http.MethodPost,
			body:       `{"username":"110550001"}`,
			wantStatus: 400,
			wantCalled: false,
		},
		{
			name:       "empty password",
			method:     http.MethodPost,
			body:       `{"username":"110550001","password":""}`,
			wantStatus: 400,
			wantCalled: false,
		},
		{
			name:       "empty body",
			method:     http.MethodPost,
			body:       ``,
			wantStatus: 400,
			wantCalled: false,
		},
		// --- Method check ---
		{
			name:       "GET method rejected",
			method:     http.MethodGet,
			wantStatus: 405,
			wantCalled: false,
		},
		{
			name:       "PUT method rejected",
			method:     http.MethodPut,
			body:       `{"username":"u","password":"p"}`,
			wantStatus: 405,
			wantCalled: false,
		},
		{
			name:       "DELETE method rejected",
			method:     http.MethodDelete,
			wantStatus: 405,
			wantCalled: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &mockAuthUseCase{result: tt.mockResult, err: tt.mockErr}
			handler := HandleAuthenticate(uc)

			var bodyReader *strings.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			} else {
				bodyReader = strings.NewReader("")
			}
			req := httptest.NewRequest(tt.method, "/api/v1/ldap/authenticate", bodyReader)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantCalled && uc.called != 1 {
				t.Errorf("use case called %d times, want 1", uc.called)
			}
			if !tt.wantCalled && uc.called != 0 {
				t.Errorf("use case called %d times, want 0 (request validation should reject)", uc.called)
			}

			// Verify response format based on status.
			switch tt.wantStatus {
			case 200:
				ct := rec.Header().Get("Content-Type")
				if ct != "application/json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/json")
				}
				var resp authenticateResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.UserID != tt.wantUserID {
					t.Errorf("response.user_id = %q, want %q", resp.UserID, tt.wantUserID)
				}
				if resp.AccountState != tt.wantAccountState {
					t.Errorf("response.account_state = %q, want %q", resp.AccountState, tt.wantAccountState)
				}
				if resp.PasswordState != tt.wantPasswordState {
					t.Errorf("response.password_state = %q, want %q", resp.PasswordState, tt.wantPasswordState)
				}

			case 401:
				ct := rec.Header().Get("Content-Type")
				if ct != "application/problem+json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/problem+json")
				}
				var prob domain.Problem
				if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
					t.Fatalf("failed to decode problem: %v", err)
				}
				if prob.Status != 401 {
					t.Errorf("problem.Status = %d, want 401", prob.Status)
				}
				if prob.Type != "/problems/authentication-failed" {
					t.Errorf("problem.Type = %q, want %q", prob.Type, "/problems/authentication-failed")
				}
				if prob.Detail != "authentication failed" {
					t.Errorf("problem.Detail = %q, want %q", prob.Detail, "authentication failed")
				}

			case 400:
				ct := rec.Header().Get("Content-Type")
				if ct != "application/problem+json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/problem+json")
				}
				var prob domain.Problem
				if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
					t.Fatalf("failed to decode problem: %v", err)
				}
				if prob.Status != 400 {
					t.Errorf("problem.Status = %d, want 400", prob.Status)
				}

			case 405:
				// Just verify status code.
			}
		})
	}
}

// TestHandleAuthenticate_ResponseNeverContainsPassword is the critical
// security assertion: the response body must NEVER contain the password
// string, regardless of success or failure.
func TestHandleAuthenticate_ResponseNeverContainsPassword(t *testing.T) {
	password := "SuperSecret!@#$%"

	successResult := &domain.AuthenticateResult{
		UID:           "110550001",
		AccountState:  domain.AccountStateActive,
		PasswordState: domain.PasswordStateCurrent,
	}

	scenarios := []struct {
		name       string
		mockResult *domain.AuthenticateResult
		mockErr    error
	}{
		{name: "successful auth", mockResult: successResult},
		{name: "failed auth", mockErr: domain.ErrAuthenticationFailed},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			uc := &mockAuthUseCase{result: sc.mockResult, err: sc.mockErr}
			handler := HandleAuthenticate(uc)

			body := `{"username":"110550001","password":"` + password + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			respBody := rec.Body.String()
			if strings.Contains(respBody, password) {
				t.Fatalf("response body contains password %q — must never leak", password)
			}
		})
	}
}

// TestHandleAuthenticate_AuthFailedResponseIsGeneric verifies that the
// 401 response for auth failure is always the same generic message,
// regardless of whether it was wrong-password, not-found, or any other reason.
//
// This is the anti-enumeration invariant from the portal-backend CR:
// 401 response shape MUST NOT differ between failure reasons.
func TestHandleAuthenticate_AuthFailedResponseIsGeneric(t *testing.T) {
	failureCases := []struct {
		name    string
		mockErr error
	}{
		{name: "wrong password", mockErr: domain.ErrAuthenticationFailed},
		{name: "user not found", mockErr: domain.ErrAuthenticationFailed},
		{name: "repo error", mockErr: errors.New("unexpected")},
	}

	var firstBody string
	for i, tt := range failureCases {
		uc := &mockAuthUseCase{err: tt.mockErr}
		handler := HandleAuthenticate(uc)

		body := `{"username":"110550001","password":"pw"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		respBody := rec.Body.String()
		if i == 0 {
			firstBody = respBody
		} else {
			if respBody != firstBody {
				t.Errorf("case %q response differs from first failure:\n  got:  %s\n  want: %s", tt.name, respBody, firstBody)
			}
		}
	}
}

// TestHandleAuthenticate_PassesCredentialsToUseCase verifies the handler
// passes both username and password to the use case unchanged.
func TestHandleAuthenticate_PassesCredentialsToUseCase(t *testing.T) {
	uc := &mockAuthUseCase{result: &domain.AuthenticateResult{
		UID:           "T1234",
		AccountState:  domain.AccountStateActive,
		PasswordState: domain.PasswordStateCurrent,
	}}
	handler := HandleAuthenticate(uc)

	body := `{"username":"T1234","password":"P@ssw0rd!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if uc.lastUser != "T1234" {
		t.Errorf("username passed = %q, want %q", uc.lastUser, "T1234")
	}
	if uc.lastPW != "P@ssw0rd!" {
		t.Errorf("password passed = %q, want %q", uc.lastPW, "P@ssw0rd!")
	}
}

// TestHandleAuthenticate_ExternalUserEmail verifies email-style usernames
// from external LDAP are handled correctly.
func TestHandleAuthenticate_ExternalUserEmail(t *testing.T) {
	uc := &mockAuthUseCase{result: &domain.AuthenticateResult{
		UID:           "alumni@example.com",
		AccountState:  domain.AccountStateActive,
		PasswordState: domain.PasswordStateCurrent,
	}}
	handler := HandleAuthenticate(uc)

	body := `{"username":"alumni@example.com","password":"pw"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if uc.lastUser != "alumni@example.com" {
		t.Errorf("username = %q, want %q", uc.lastUser, "alumni@example.com")
	}
}
