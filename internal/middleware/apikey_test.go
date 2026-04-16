package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKey(t *testing.T) {
	keys := map[string]string{
		"valid-key-1": "portal",
		"valid-key-2": "mfa",
	}

	tests := []struct {
		name           string
		apiKeyHeader   string // empty means "no header"
		wantStatus     int
		wantService    string // expected service name in context (empty if rejected)
	}{
		{name: "valid key for portal", apiKeyHeader: "valid-key-1", wantStatus: 200, wantService: "portal"},
		{name: "valid key for mfa", apiKeyHeader: "valid-key-2", wantStatus: 200, wantService: "mfa"},
		{name: "missing key", apiKeyHeader: "", wantStatus: 401},
		{name: "invalid key", apiKeyHeader: "wrong-key", wantStatus: 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create a test handler that records ServiceNameFromContext(r.Context())
			//   - Wrap with APIKey(keys) middleware
			//   - If valid: handler MUST be called, service name MUST match, status 200
			//   - If invalid/missing: handler MUST NOT be called, status 401,
			//     response Content-Type MUST be "application/problem+json"
			panic("not implemented")
		})
	}
}

// Ensure imports are used.
var _ = httptest.NewRequest
var _ http.Handler
