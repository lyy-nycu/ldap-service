package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// ---------------------------------------------------------------------------
// APIKey middleware — full locked tests (security-critical)
// ---------------------------------------------------------------------------

func TestAPIKey(t *testing.T) {
	keys := map[string]string{
		"valid-key-1": "portal",
		"valid-key-2": "mfa",
	}

	tests := []struct {
		name        string
		apiKey      string // empty means "no header set"
		wantStatus  int
		wantService string // expected service name in context
		wantCalled  bool   // whether next handler should be invoked
	}{
		{name: "valid key for portal", apiKey: "valid-key-1", wantStatus: 200, wantService: "portal", wantCalled: true},
		{name: "valid key for mfa", apiKey: "valid-key-2", wantStatus: 200, wantService: "mfa", wantCalled: true},
		{name: "missing key header", apiKey: "", wantStatus: 401, wantCalled: false},
		{name: "invalid key", apiKey: "wrong-key", wantStatus: 401, wantCalled: false},
		{name: "empty key value", apiKey: " ", wantStatus: 401, wantCalled: false},
		{name: "partial key match", apiKey: "valid-key-", wantStatus: 401, wantCalled: false},
		{name: "key with extra suffix", apiKey: "valid-key-1-extra", wantStatus: 401, wantCalled: false},
		{name: "key with different case", apiKey: "VALID-KEY-1", wantStatus: 401, wantCalled: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			var gotService string

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotService = ServiceNameFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			mw := APIKey(keys)
			handler := mw(next)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/ldap/lookup", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-Api-Key", tt.apiKey)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Errorf("handler called = %v, want %v", called, tt.wantCalled)
			}
			if tt.wantCalled && gotService != tt.wantService {
				t.Errorf("service name = %q, want %q", gotService, tt.wantService)
			}

			// On rejection, verify RFC 7807 response format.
			if !tt.wantCalled {
				ct := rec.Header().Get("Content-Type")
				if ct != "application/problem+json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/problem+json")
				}
				var prob domain.Problem
				if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
					t.Fatalf("failed to decode problem response: %v", err)
				}
				if prob.Status != 401 {
					t.Errorf("problem.Status = %d, want 401", prob.Status)
				}
				if prob.Type != "/problems/unauthorized" {
					t.Errorf("problem.Type = %q, want %q", prob.Type, "/problems/unauthorized")
				}
			}
		})
	}
}

// TestAPIKey_ConstantTimeComparison verifies that the middleware does NOT
// use naive string equality. We can't directly prove constant-time comparison
// from behavior alone, but we verify that a timing-adjacent pattern —
// comparing keys of the same length that differ only in the last character —
// still correctly rejects.
func TestAPIKey_ConstantTimeComparison(t *testing.T) {
	keys := map[string]string{
		"abcdef12345": "svc",
	}
	mw := APIKey(keys)

	// Keys that share a long common prefix but differ at the end.
	badKeys := []string{
		"abcdef12344",
		"abcdef12346",
		"abcdef1234 ",
		"abcdef1234\x00",
	}
	for _, k := range badKeys {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Api-Key", k)
		rec := httptest.NewRecorder()

		called := false
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})).ServeHTTP(rec, req)

		if called {
			t.Errorf("key %q was accepted — prefix match must NOT be sufficient", k)
		}
		if rec.Code != 401 {
			t.Errorf("key %q → status %d, want 401", k, rec.Code)
		}
	}
}

// TestAPIKey_DoesNotLeakKeyInResponse verifies the response body never
// contains the submitted API key value.
func TestAPIKey_DoesNotLeakKeyInResponse(t *testing.T) {
	keys := map[string]string{"secret-key-value": "svc"}
	mw := APIKey(keys)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "attacker-probe-key")
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for invalid key")
	})).ServeHTTP(rec, req)

	body := rec.Body.String()
	if containsStr(body, "attacker-probe-key") {
		t.Error("response body contains the submitted API key value — must never leak")
	}
	if containsStr(body, "secret-key-value") {
		t.Error("response body contains a valid API key — must never leak")
	}
}

// TestAPIKey_ServiceNameInContext verifies the service name is accessible
// via ServiceNameFromContext in downstream handlers.
func TestAPIKey_ServiceNameInContext(t *testing.T) {
	keys := map[string]string{"key1": "portal-service"}
	mw := APIKey(keys)

	var got string
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = ServiceNameFromContext(r.Context())
	})).ServeHTTP(
		httptest.NewRecorder(),
		func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("X-Api-Key", "key1")
			return r
		}(),
	)

	if got != "portal-service" {
		t.Errorf("ServiceNameFromContext = %q, want %q", got, "portal-service")
	}
}

// TestAPIKey_EmptyKeysMap rejects all requests when no keys are configured.
func TestAPIKey_EmptyKeysMap(t *testing.T) {
	mw := APIKey(map[string]string{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "anything")
	rec := httptest.NewRecorder()

	called := false
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(rec, req)

	if called {
		t.Error("handler called with empty keys map — should reject all")
	}
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// containsStr is a simple substring check without importing strings.
func containsStr(s, substr string) bool {
	if len(substr) == 0 || len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
