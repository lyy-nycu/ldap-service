package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter manages per-username rate limiting for the authenticate endpoint.
type RateLimiter struct {
	// TODO(copilot): add fields
	//   - limiters sync.Map          (username → *rateLimiterEntry)
	//   - rate     int               (attempts per minute from config)
	//   - cleanup  time.Duration     (cleanup interval from config)
	//   - stop     chan struct{}      (signal to stop cleanup goroutine)
}

// rateLimiterEntry holds a limiter and its last access time.
type rateLimiterEntry struct {
	// TODO(copilot): add fields
	//   - limiter  *rate.Limiter
	//   - lastSeen time.Time
}

// NewRateLimiter creates a RateLimiter and starts the background cleanup goroutine.
//
// Acceptance criteria:
//   - MUST create with the given rate (attempts per minute) and cleanup interval
//   - MUST start a background goroutine that runs every cleanupInterval
//   - The cleanup goroutine MUST remove entries where time.Since(lastSeen) > cleanupInterval
//   - MUST return a RateLimiter that can be stopped via Stop()
func NewRateLimiter(ratePerMin int, cleanupInterval time.Duration) *RateLimiter {
	panic("not implemented")
}

// Stop signals the cleanup goroutine to exit. Call during graceful shutdown.
func (rl *RateLimiter) Stop() {
	panic("not implemented")
}

// Middleware returns an HTTP middleware that enforces per-username rate limiting.
//
// Acceptance criteria:
//   - MUST only apply to POST /api/v1/ldap/authenticate
//   - MUST extract username from the JSON request body WITHOUT consuming the body
//     — use io.TeeReader or bytes.Buffer to buffer and restore r.Body
//   - MUST get or create a rate.Limiter for the username (Token Bucket: rate=ratePerMin/60, burst=ratePerMin)
//   - MUST update lastSeen time on every attempt
//   - If limiter.Allow() is true: call next.ServeHTTP
//   - If limiter.Allow() is false: respond with domain.NewRateLimitExceeded
//   - If body parse fails (can't extract username): call next.ServeHTTP (let handler return 400)
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	panic("not implemented")
}

// Ensure imports are used.
var _ sync.Map
var _ time.Duration
