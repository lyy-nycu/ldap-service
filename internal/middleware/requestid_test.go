package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
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
			var ctxRequestID string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctxRequestID = RequestIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := RequestID(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-Request-ID", tt.headerValue)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			respRequestID := rec.Header().Get("X-Request-ID")
			if tt.wantFromHeader {
				if ctxRequestID != tt.headerValue {
					t.Fatalf("context request ID = %q, want %q", ctxRequestID, tt.headerValue)
				}
				if respRequestID != tt.headerValue {
					t.Fatalf("response request ID = %q, want %q", respRequestID, tt.headerValue)
				}
				return
			}

			if ctxRequestID == "" {
				t.Fatal("context request ID is empty, want generated UUID")
			}
			if respRequestID == "" {
				t.Fatal("response request ID is empty, want generated UUID")
			}
			if len(ctxRequestID) != 36 {
				t.Fatalf("context request ID length = %d, want 36", len(ctxRequestID))
			}
			if len(respRequestID) != 36 {
				t.Fatalf("response request ID length = %d, want 36", len(respRequestID))
			}
			if _, err := uuid.Parse(ctxRequestID); err != nil {
				t.Fatalf("context request ID is not valid UUID: %v", err)
			}
			if _, err := uuid.Parse(respRequestID); err != nil {
				t.Fatalf("response request ID is not valid UUID: %v", err)
			}
		})
	}
}

// Ensure imports are used.
var _ = httptest.NewRequest
var _ http.Handler
