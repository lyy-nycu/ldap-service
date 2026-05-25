package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

type mockHealthRepo struct {
	healthErr   error
	healthCalls int
}

func (m *mockHealthRepo) Lookup(context.Context, string, []string) (*domain.Account, error) {
	panic("Lookup should not be called in health tests")
}

func (m *mockHealthRepo) LookupBatch(context.Context, []string, []string) ([]*domain.Account, []string, error) {
	panic("LookupBatch should not be called in health tests")
}

func (m *mockHealthRepo) Authenticate(context.Context, string, string) (*domain.AuthenticateResult, error) {
	panic("Authenticate should not be called in health tests")
}

func (m *mockHealthRepo) HealthCheck(context.Context) error {
	m.healthCalls++
	return m.healthErr
}

func (m *mockHealthRepo) Close() error {
	return nil
}

func (m *mockHealthRepo) Modify(context.Context, string, []domain.ModifyAttr) error {
	return nil
}

func TestHandleHealthz(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
	}{
		{name: "GET returns ok", method: http.MethodGet, wantStatus: 200, wantBody: `{"status":"ok"}`},
		{name: "POST returns 405", method: http.MethodPost, wantStatus: 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - GET: status 200, body matches wantBody, Content-Type is application/json
			//   - Other methods: status 405
			h := HandleHealthz()
			req := httptest.NewRequest(tt.method, "/healthz", nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if rec.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
				}
				if rec.Body.String() != tt.wantBody {
					t.Fatalf("body = %q, want %q", rec.Body.String(), tt.wantBody)
				}
			}
		})
	}
}

func TestHandleReadyz(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		healthy    bool // mock health check result
		wantStatus int
	}{
		{name: "both sources healthy", method: http.MethodGet, healthy: true, wantStatus: 200},
		{name: "source unhealthy", method: http.MethodGet, healthy: false, wantStatus: 503},
		{name: "POST returns 405", method: http.MethodPost, healthy: true, wantStatus: 405},
		// TODO(copilot): for 200 case verify Content-Type is application/json and body is {"status":"ready"}
		// TODO(copilot): for 503 case verify Content-Type is application/problem+json,
		//   type is /problems/service-unavailable, status is 503
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create a mock LDAPRepository with HealthCheck returning nil or ErrServiceUnavailable
			//   - 200: body is {"status":"ready"}
			//   - 503: Content-Type is application/problem+json, type is /problems/service-unavailable
			repo := &mockHealthRepo{}
			if !tt.healthy {
				repo.healthErr = domain.ErrServiceUnavailable
			}

			h := HandleReadyz(repo)
			req := httptest.NewRequest(tt.method, "/readyz", nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.method == http.MethodGet && repo.healthCalls != 1 {
				t.Fatalf("HealthCheck called %d times, want 1", repo.healthCalls)
			}
			if tt.method != http.MethodGet && repo.healthCalls != 0 {
				t.Fatalf("HealthCheck called %d times, want 0", repo.healthCalls)
			}

			if tt.wantStatus == http.StatusOK {
				if rec.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
				}
				if rec.Body.String() != `{"status":"ready"}` {
					t.Fatalf("body = %q, want %q", rec.Body.String(), `{"status":"ready"}`)
				}
			}

			if tt.wantStatus == http.StatusServiceUnavailable {
				if rec.Header().Get("Content-Type") != "application/problem+json" {
					t.Fatalf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), "application/problem+json")
				}

				var p domain.Problem
				if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
					t.Fatalf("failed to decode problem response: %v", err)
				}
				if p.Type != "/problems/service-unavailable" {
					t.Fatalf("problem.Type = %q, want %q", p.Type, "/problems/service-unavailable")
				}
				if p.Status != http.StatusServiceUnavailable {
					t.Fatalf("problem.Status = %d, want %d", p.Status, http.StatusServiceUnavailable)
				}
			}
		})
	}
}

// Ensure imports are used.
var _ = httptest.NewRequest
