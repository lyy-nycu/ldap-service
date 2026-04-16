package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

// Logger is a middleware that logs every request using zap.
//
// Acceptance criteria:
//   - MUST log after the response is sent (use a responseWriter wrapper to capture status code)
//   - MUST include: method, path, status code, duration_ms, remote IP, request ID
//   - MUST use zap.Info for successful requests (2xx/3xx), zap.Warn for 4xx, zap.Error for 5xx
//   - MUST get request ID from RequestIDFromContext(r.Context())
//   - MUST NOT log request body or passwords
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	panic("not implemented")
}
