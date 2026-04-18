//go:build integration

package integration

import (
	"io"
	"net/http"
	"testing"
)

// TestHealthEndpoints verifies the health check endpoints against the live service.
//
// Acceptance criteria:
//   - GET /healthz MUST return 200 with body {"status":"ok"}
//   - GET /readyz MUST return 200 with body {"status":"ready"} when both LDAPs are up
//   - Both MUST return Content-Type application/json
//   - Neither endpoint requires an API key
func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCode int
		wantBody string
	}{
		{name: "healthz returns ok", path: "/healthz", wantCode: 200, wantBody: `{"status":"ok"}`},
		{name: "readyz returns ready", path: "/readyz", wantCode: 200, wantBody: `{"status":"ready"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Send GET request WITHOUT API key
			//   - Verify status code matches wantCode
			//   - Verify response body matches wantBody exactly
			//   - Verify Content-Type is application/json
			resp := doRequest(t, http.MethodGet, tt.path, nil, false)
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantCode {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			if string(body) != tt.wantBody {
				t.Fatalf("body = %q, want %q", string(body), tt.wantBody)
			}

			if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
			}
		})
	}
}
