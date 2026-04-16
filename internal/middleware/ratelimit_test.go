package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Middleware(t *testing.T) {
	tests := []struct {
		name       string
		attempts   int // number of requests with the same username
		rateLimit  int // allowed per minute
		wantStatus int // expected status of the LAST request
	}{
		{name: "under limit", attempts: 3, rateLimit: 5, wantStatus: 200},
		{name: "at limit", attempts: 5, rateLimit: 5, wantStatus: 200},
		{name: "over limit", attempts: 6, rateLimit: 5, wantStatus: 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create a RateLimiter with tt.rateLimit and a long cleanup interval
			//   - Create tt.attempts requests with body {"username":"testuser","password":"pass"}
			//   - The last request's status MUST match tt.wantStatus
			//   - If 429: response Content-Type MUST be "application/problem+json"
			//   - Call rl.Stop() at end of test
			panic("not implemented")
		})
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	// Acceptance criteria:
	//   - Create a RateLimiter with a very short cleanup interval (e.g., 50ms)
	//   - Make one request to create a limiter entry
	//   - Wait for cleanup to run
	//   - Verify the entry was removed (next request should be allowed, not rate limited)
	panic("not implemented")
}

// Ensure imports are used.
var _ = httptest.NewRequest
var _ http.Handler
var _ time.Duration
