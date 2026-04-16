package middleware

import (
	"context"
	"net/http"
)

// ctxKeyRequestID is the context key for request ID.
// Uses unexported struct type to prevent collisions.
type ctxKeyRequestID struct{}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKeyRequestID{}).(string); ok {
		return id
	}
	return ""
}

// RequestID is a middleware that injects a request ID into every request's context.
//
// Acceptance criteria:
//   - If caller provides X-Request-ID header, MUST use that value
//   - If no X-Request-ID header, MUST generate a UUID v4 using github.com/google/uuid
//   - MUST set the request ID in the response X-Request-ID header
//   - MUST store the request ID in request context using ctxKeyRequestID
//   - MUST call next.ServeHTTP with the updated context
func RequestID(next http.Handler) http.Handler {
	panic("not implemented")
}
