package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiter manages per-username rate limiting for the authenticate endpoint.
type RateLimiter struct {
	limiters sync.Map      // username → *rateLimiterEntry
	rate     int           // max attempts per minute
	cleanup  time.Duration // cleanup interval
	stop     chan struct{} // signal to stop cleanup goroutine
	stopOnce sync.Once
}

// rateLimiterEntry holds a limiter and its last access time.
type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
	mu       sync.Mutex
}

// NewRateLimiter creates a RateLimiter and starts the background cleanup goroutine.
//
// Acceptance criteria:
//   - MUST create with the given rate (attempts per minute) and cleanup interval
//   - MUST start a background goroutine that runs every cleanupInterval
//   - The cleanup goroutine MUST remove entries where time.Since(lastSeen) > cleanupInterval
//   - MUST return a RateLimiter that can be stopped via Stop()
func NewRateLimiter(ratePerMin int, cleanupInterval time.Duration) *RateLimiter {
	if ratePerMin <= 0 {
		ratePerMin = 1
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 10 * time.Minute
	}

	rl := &RateLimiter{
		rate:    ratePerMin,
		cleanup: cleanupInterval,
		stop:    make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				now := time.Now()
				rl.limiters.Range(func(key, value any) bool {
					entry, ok := value.(*rateLimiterEntry)
					if !ok || entry == nil {
						rl.limiters.Delete(key)
						return true
					}

					entry.mu.Lock()
					stale := now.Sub(entry.lastSeen) > cleanupInterval
					entry.mu.Unlock()

					if stale {
						rl.limiters.Delete(key)
					}

					return true
				})
			case <-rl.stop:
				return
			}
		}
	}()

	return rl
}

// Stop signals the cleanup goroutine to exit. Call during graceful shutdown.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stop)
	})
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/ldap/authenticate" {
			next.ServeHTTP(w, r)
			return
		}

		var buf bytes.Buffer
		tee := io.TeeReader(r.Body, &buf)

		var payload struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(tee).Decode(&payload); err != nil {
			r.Body = io.NopCloser(&buf)
			next.ServeHTTP(w, r)
			return
		}

		r.Body = io.NopCloser(&buf)

		username := strings.TrimSpace(payload.Username)
		if username == "" {
			next.ServeHTTP(w, r)
			return
		}

		now := time.Now()
		newEntry := &rateLimiterEntry{
			limiter:  rate.NewLimiter(rate.Limit(float64(rl.rate)/60.0), rl.rate),
			lastSeen: now,
		}
		entryAny, _ := rl.limiters.LoadOrStore(username, newEntry)
		entry := entryAny.(*rateLimiterEntry)

		entry.mu.Lock()
		entry.lastSeen = now
		allowed := entry.limiter.Allow()
		entry.mu.Unlock()

		if allowed {
			next.ServeHTTP(w, r)
			return
		}

		zap.L().Warn("rate limit exceeded", zap.String("username", username), zap.String("remote_addr", r.RemoteAddr))
		problem := domain.NewRateLimitExceeded("")
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(problem.Status)
		_ = json.NewEncoder(w).Encode(problem)
	})
}
