//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// TestAuthenticate verifies authentication against real LDAP.
//
// Acceptance criteria:
//   - Correct password MUST return 200 with {"authenticated":true}
//   - Wrong password MUST return 401 with generic error (RFC 7807)
//   - Response for wrong password MUST NOT contain the password string
//   - Response for wrong password MUST be identical structure regardless of failure reason
//   - Missing API key MUST return 401
func TestAuthenticate(t *testing.T) {
	var wrongPasswordProblem *domain.Problem
	var notFoundProblem *domain.Problem

	tests := []struct {
		name       string
		username   string
		password   string
		withAPIKey bool
		wantCode   int
		wantAuth   bool // only checked for 200 responses
	}{
		{name: "internal student correct password", username: "110550001", password: "testpass123", withAPIKey: true, wantCode: 200, wantAuth: true},
		{name: "internal employee correct password", username: "T1234", password: "testpass123", withAPIKey: true, wantCode: 200, wantAuth: true},
		{name: "external alumni correct password", username: "alumni01@example.com", password: "testpass123", withAPIKey: true, wantCode: 200, wantAuth: true},
		{name: "wrong password", username: "110550001", password: "wrongpass", withAPIKey: true, wantCode: 401},
		{name: "nonexistent user", username: "nobody999", password: "testpass123", withAPIKey: true, wantCode: 401},
		{name: "no api key", username: "110550001", password: "testpass123", withAPIKey: false, wantCode: 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Send POST /api/v1/ldap/authenticate with body {"username": ..., "password": ...}
			//   - For 200: verify body contains {"authenticated":true}
			//   - For 401 (wrong password vs not found): verify response bodies are identical structure
			//   - For 401: verify response body does NOT contain the password string
			//   - For 401: verify Content-Type is application/problem+json
			resp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
				"username": tt.username,
				"password": tt.password,
			}, tt.withAPIKey)
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantCode {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantCode)
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			body := string(bodyBytes)

			if tt.wantCode == http.StatusOK {
				var got map[string]any
				if err := json.Unmarshal(bodyBytes, &got); err != nil {
					t.Fatalf("failed to decode success response: %v", err)
				}
				authVal, ok := got["authenticated"].(bool)
				if !ok {
					t.Fatal("success response missing authenticated field")
				}
				if authVal != tt.wantAuth {
					t.Fatalf("authenticated = %v, want %v", authVal, tt.wantAuth)
				}
				return
			}

			if tt.wantCode == http.StatusUnauthorized {
				if strings.Contains(body, tt.password) {
					t.Fatalf("response body leaks password: %q", tt.password)
				}
				if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
					t.Fatalf("Content-Type = %q, want %q", ct, "application/problem+json")
				}

				var p domain.Problem
				if err := json.Unmarshal(bodyBytes, &p); err != nil {
					t.Fatalf("failed to decode problem response: %v", err)
				}

				switch tt.name {
				case "wrong password":
					wrongPasswordProblem = &p
				case "nonexistent user":
					notFoundProblem = &p
				}
			}
		})
	}

	if wrongPasswordProblem != nil && notFoundProblem != nil {
		if wrongPasswordProblem.Type != notFoundProblem.Type ||
			wrongPasswordProblem.Title != notFoundProblem.Title ||
			wrongPasswordProblem.Status != notFoundProblem.Status {
			t.Fatalf("wrong-password and not-found responses differ: wrong=%+v notfound=%+v", *wrongPasswordProblem, *notFoundProblem)
		}
	}
}

// TestAuthenticateResponseUniformity verifies that wrong-password and user-not-found
// produce byte-identical response structures.
//
// Acceptance criteria:
//   - Response for wrong password and nonexistent user MUST have identical
//     type, title, and status fields in the RFC 7807 response
//   - Attacker MUST NOT be able to enumerate valid usernames via response differences
func TestAuthenticateResponseUniformity(t *testing.T) {
	// Acceptance criteria:
	//   - Send auth request with wrong password for existing user
	//   - Send auth request for nonexistent user
	//   - Decode both responses as RFC 7807 Problem
	//   - Verify type, title, and status fields are identical
	wrongPasswordResp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
		"username": "110550001",
		"password": "wrongpass",
	}, true)
	defer wrongPasswordResp.Body.Close()

	notFoundResp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
		"username": "nobody999",
		"password": "testpass123",
	}, true)
	defer notFoundResp.Body.Close()

	if wrongPasswordResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-password status = %d, want 401", wrongPasswordResp.StatusCode)
	}
	if notFoundResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("not-found status = %d, want 401", notFoundResp.StatusCode)
	}

	var wrongProb domain.Problem
	var notFoundProb domain.Problem
	decodeJSON(t, wrongPasswordResp, &wrongProb)
	decodeJSON(t, notFoundResp, &notFoundProb)

	if wrongProb.Type != notFoundProb.Type {
		t.Fatalf("type mismatch: %q vs %q", wrongProb.Type, notFoundProb.Type)
	}
	if wrongProb.Title != notFoundProb.Title {
		t.Fatalf("title mismatch: %q vs %q", wrongProb.Title, notFoundProb.Title)
	}
	if wrongProb.Status != notFoundProb.Status {
		t.Fatalf("status mismatch: %d vs %d", wrongProb.Status, notFoundProb.Status)
	}
}
