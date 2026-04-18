//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestRateLimitAuthenticate verifies per-username rate limiting on the authenticate endpoint.
//
// Acceptance criteria:
//   - First N requests (where N = AUTH_RATE_LIMIT, default 5) MUST succeed (200 or 401)
//   - The N+1th request for the same username MUST return 429 Too Many Requests
//   - A different username MUST NOT be affected by the first username's rate limit
//   - 429 response MUST use application/problem+json Content-Type
func TestRateLimitAuthenticate(t *testing.T) {
	// Acceptance criteria:
	//   - Use a unique username (e.g. "ratelimit-test-user") that doesn't exist in LDAP
	//     so each request gets a 401 but still counts toward the rate limit
	//   - Send AUTH_RATE_LIMIT requests — all should get 401 (auth failed, not rate limited)
	//   - Send one more request — should get 429
	//   - Send a request for a DIFFERENT username — should get 401 (not rate limited)
	//   - Verify 429 response Content-Type is application/problem+json
	const limit = 5
	baseUsername := fmt.Sprintf("ratelimit-test-user-%d", time.Now().UnixNano())
	password := "irrelevant"

	for i := 0; i < limit; i++ {
		resp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
			"username": baseUsername,
			"password": password,
		}, true)
		if resp.StatusCode != http.StatusUnauthorized {
			resp.Body.Close()
			t.Fatalf("request %d status = %d, want 401", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}

	limitedResp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
		"username": baseUsername,
		"password": password,
	}, true)
	defer limitedResp.Body.Close()

	if limitedResp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("N+1 request status = %d, want 429", limitedResp.StatusCode)
	}
	if ct := limitedResp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("429 Content-Type = %q, want %q", ct, "application/problem+json")
	}

	otherUserResp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
		"username": baseUsername + "-other",
		"password": password,
	}, true)
	defer otherUserResp.Body.Close()

	if otherUserResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("different username status = %d, want 401", otherUserResp.StatusCode)
	}
}
