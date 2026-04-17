package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type statusResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Logger is a middleware that logs every request using zap.
//
// Acceptance criteria:
//   - MUST log after the response is sent (use a responseWriter wrapper to capture status code)
//   - MUST include: method, path, status code, duration_ms, remote IP, request ID
//   - MUST use zap.Info for successful requests (2xx/3xx), zap.Warn for 4xx, zap.Error for 5xx
//   - MUST get request ID from RequestIDFromContext(r.Context())
//   - MUST NOT log request body or passwords
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(srw, r)

			fields := []zap.Field{
				zap.String("request_id", RequestIDFromContext(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", srw.status),
				zap.Int64("duration_ms", time.Since(start).Milliseconds()),
				zap.String("remote_addr", r.RemoteAddr),
			}

			switch {
			case srw.status >= 500:
				logger.Error("http request", fields...)
			case srw.status >= 400:
				logger.Warn("http request", fields...)
			default:
				logger.Info("http request", fields...)
			}
		})
	}
}
