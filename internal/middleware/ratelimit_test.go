package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// ---------------------------------------------------------------------------
// RateLimiter Middleware — full locked tests (security-critical)
// ---------------------------------------------------------------------------

func TestRateLimiter_Middleware(t *testing.T) {
	tests := []struct {
		name       string
		attempts   int
		rateLimit  int
		wantStatus int // expected status of the LAST request
	}{
		{name: "under limit", attempts: 3, rateLimit: 5, wantStatus: 200},
		{name: "at limit", attempts: 5, rateLimit: 5, wantStatus: 200},
		{name: "over limit (6th rejected)", attempts: 6, rateLimit: 5, wantStatus: 429},
		{name: "well over limit", attempts: 10, rateLimit: 5, wantStatus: 429},
		{name: "limit of 1", attempts: 2, rateLimit: 1, wantStatus: 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.rateLimit, 10*time.Minute)
			defer rl.Stop()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler := rl.Middleware(next)

			var lastRec *httptest.ResponseRecorder
			for i := 0; i < tt.attempts; i++ {
				body := `{"username":"testuser","password":"pass"}`
				req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				lastRec = httptest.NewRecorder()
				handler.ServeHTTP(lastRec, req)
			}

			if lastRec.Code != tt.wantStatus {
				t.Errorf("last request status = %d, want %d", lastRec.Code, tt.wantStatus)
			}

			// If rate limited, verify RFC 7807 response.
			if tt.wantStatus == 429 {
				ct := lastRec.Header().Get("Content-Type")
				if ct != "application/problem+json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/problem+json")
				}
				var prob domain.Problem
				if err := json.NewDecoder(lastRec.Body).Decode(&prob); err != nil {
					t.Fatalf("failed to decode problem response: %v", err)
				}
				if prob.Status != 429 {
					t.Errorf("problem.Status = %d, want 429", prob.Status)
				}
				if prob.Type != "/problems/rate-limit-exceeded" {
					t.Errorf("problem.Type = %q, want %q", prob.Type, "/problems/rate-limit-exceeded")
				}
			}
		})
	}
}

// TestRateLimiter_PerUsernameIsolation verifies that rate limits are tracked
// per-username, not globally. User A hitting the limit must NOT affect user B.
func TestRateLimiter_PerUsernameIsolation(t *testing.T) {
	rl := NewRateLimiter(2, 10*time.Minute)
	defer rl.Stop()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	makeReq := func(username string) int {
		body := `{"username":"` + username + `","password":"pass"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// Exhaust limit for userA.
	makeReq("userA")
	makeReq("userA")
	statusA := makeReq("userA") // 3rd attempt, limit=2 → should be 429

	// userB should still be allowed.
	statusB := makeReq("userB")

	if statusA != 429 {
		t.Errorf("userA 3rd attempt status = %d, want 429", statusA)
	}
	if statusB != 200 {
		t.Errorf("userB 1st attempt status = %d, want 200 (must not be affected by userA's limit)", statusB)
	}
}

// TestRateLimiter_Cleanup verifies the background goroutine removes stale entries.
func TestRateLimiter_Cleanup(t *testing.T) {
	// Use very short cleanup interval for test.
	rl := NewRateLimiter(1, 50*time.Millisecond)
	defer rl.Stop()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	// Exhaust the limit for testuser.
	body := `{"username":"testuser","password":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("first request status = %d, want 200", rec.Code)
	}

	// Second request should be rate limited.
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != 429 {
		t.Fatalf("second request status = %d, want 429", rec2.Code)
	}

	// Wait for cleanup to remove the stale entry.
	time.Sleep(150 * time.Millisecond)

	// After cleanup, the entry should be removed and a new limiter created.
	// The next request should be allowed.
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != 200 {
		t.Errorf("request after cleanup status = %d, want 200 (limiter entry should have been cleaned up)", rec3.Code)
	}
}

// TestRateLimiter_NonAuthEndpointPassesThrough verifies the rate limiter
// only applies to POST /api/v1/ldap/authenticate. Other routes pass through.
func TestRateLimiter_NonAuthEndpointPassesThrough(t *testing.T) {
	rl := NewRateLimiter(1, 10*time.Minute)
	defer rl.Stop()

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "GET lookup", method: http.MethodGet, path: "/api/v1/ldap/lookup"},
		{name: "POST lookup", method: http.MethodPost, path: "/api/v1/ldap/lookup"},
		{name: "GET healthz", method: http.MethodGet, path: "/healthz"},
		{name: "POST different path", method: http.MethodPost, path: "/api/v1/ldap/other"},
		{name: "GET authenticate", method: http.MethodGet, path: "/api/v1/ldap/authenticate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if !called {
				t.Errorf("next handler was not called for %s %s — rate limiter must only apply to POST /api/v1/ldap/authenticate", tt.method, tt.path)
			}
		})
	}
}

// TestRateLimiter_MalformedBodyPassesThrough verifies that if the request
// body can't be parsed (no username), the request passes through to the
// handler (which will return a 400).
func TestRateLimiter_MalformedBodyPassesThrough(t *testing.T) {
	rl := NewRateLimiter(1, 10*time.Minute)
	defer rl.Stop()

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusBadRequest)
	})
	handler := rl.Middleware(next)

	bodies := []string{
		`invalid json`,
		`{}`,
		`{"password":"pw"}`,
		``,
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if !called {
				t.Errorf("handler not called for malformed body %q — must pass through to let handler return 400", body)
			}
		})
	}
}

// TestRateLimiter_BodyNotConsumed verifies the middleware restores the
// request body after reading the username, so the downstream handler can
// still read it.
func TestRateLimiter_BodyNotConsumed(t *testing.T) {
	rl := NewRateLimiter(10, 10*time.Minute)
	defer rl.Stop()

	originalBody := `{"username":"testuser","password":"secret123"}`
	var handlerGotBody string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var parsed map[string]string
		if err := json.NewDecoder(r.Body).Decode(&parsed); err != nil {
			t.Fatalf("handler failed to decode body: %v", err)
		}
		handlerGotBody = parsed["username"]
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/authenticate", strings.NewReader(originalBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if handlerGotBody != "testuser" {
		t.Errorf("handler got username = %q, want %q — middleware consumed the body without restoring it", handlerGotBody, "testuser")
	}
}

// TestRateLimiter_Stop verifies Stop doesn't panic and is idempotent.
func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(5, 10*time.Minute)

	// Must not panic on first call.
	rl.Stop()

	// Must not panic on second call (idempotent).
	rl.Stop()
}
