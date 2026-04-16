package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name           string
		headerValue    string // empty means "no header provided"
		wantFromHeader bool   // true: response should echo the input; false: response should be a generated UUID
	}{
		{name: "caller provides request ID", headerValue: "abc-123", wantFromHeader: true},
		{name: "no request ID generates UUID", headerValue: "", wantFromHeader: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create a test handler that reads RequestIDFromContext(r.Context())
			//   - If caller provides header: context value and response header MUST match input
			//   - If no header: context value and response header MUST be a valid UUID (non-empty, 36 chars)
			panic("not implemented")
		})
	}
}

// Ensure imports are used.
var _ = httptest.NewRequest
var _ http.Handler
