//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// Test environment configuration
// ---------------------------------------------------------------------------

// Acceptance criteria:
//   - MUST read SERVICE_URL from env (default: http://localhost:8080)
//   - MUST read TEST_API_KEY from env (default: test-api-key-001)
//   - MUST provide helper functions for making HTTP requests with API key

var (
	serviceURL string
	apiKey     string
)

func TestMain(m *testing.M) {
	serviceURL = os.Getenv("SERVICE_URL")
	if serviceURL == "" {
		serviceURL = "http://localhost:8080"
	}

	apiKey = os.Getenv("TEST_API_KEY")
	if apiKey == "" {
		apiKey = "test-api-key-001"
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// doRequest sends an HTTP request to the service and returns the response.
//
// Acceptance criteria:
//   - MUST set Content-Type to application/json for non-empty bodies
//   - MUST set X-Api-Key header when withAPIKey is true
//   - MUST return the raw *http.Response for caller to inspect
func doRequest(t *testing.T, method, path string, body any, withAPIKey bool) *http.Response {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(payload)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, serviceURL+path, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAPIKey {
		req.Header.Set("X-Api-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return resp
}

// decodeJSON decodes the response body into the given target.
func decodeJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// Ensure imports are used.
var _ = bytes.NewReader
