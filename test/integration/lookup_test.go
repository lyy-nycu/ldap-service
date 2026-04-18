//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// TestLookupSingle verifies single account lookup against real LDAP.
//
// Acceptance criteria:
//   - MUST find internal accounts (student, employee, retire)
//   - MUST find external accounts (alumni, cooperator) via fan-out
//   - Response MUST include dn, uid, source, and requested attributes
//   - Source field MUST be "internal" or "external" matching the account's server
//   - Not-found username MUST return 404 with problem type /problems/not-found
func TestLookupSingle(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		attributes []string
		wantCode   int
		wantSource string // expected source field value, empty if error response
	}{
		{name: "internal student", username: "110550001", attributes: []string{"mail"}, wantCode: 200, wantSource: "internal"},
		{name: "internal employee", username: "T1234", attributes: []string{"mail", "dept"}, wantCode: 200, wantSource: "internal"},
		{name: "internal retire", username: "RT00001", attributes: []string{"mail"}, wantCode: 200, wantSource: "internal"},
		{name: "external alumni via fan-out", username: "alumni01@example.com", attributes: []string{"mail"}, wantCode: 200, wantSource: "external"},
		{name: "external cooperator via fan-out", username: "coop01@partner.com", attributes: []string{"mail"}, wantCode: 200, wantSource: "external"},
		{name: "not found", username: "nonexistent999", attributes: []string{"mail"}, wantCode: 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Send POST /api/v1/ldap/lookup with API key
			//   - Body: {"username": tt.username, "attributes": tt.attributes}
			//   - Verify status code matches wantCode
			//   - For 200: verify response has dn, uid, source, attributes fields
			//   - For 200: verify source matches wantSource
			//   - For 404: verify Content-Type is application/problem+json
			resp := doRequest(t, http.MethodPost, "/api/v1/ldap/lookup", map[string]any{
				"username":   tt.username,
				"attributes": tt.attributes,
			}, true)
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantCode {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantCode)
			}

			if tt.wantCode == http.StatusOK {
				var got map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if _, ok := got["dn"]; !ok {
					t.Fatal("response missing dn field")
				}
				if _, ok := got["uid"]; !ok {
					t.Fatal("response missing uid field")
				}
				source, ok := got["source"].(string)
				if !ok {
					t.Fatal("response missing source field")
				}
				if source != tt.wantSource {
					t.Fatalf("source = %q, want %q", source, tt.wantSource)
				}
				if _, ok := got["attributes"]; !ok {
					t.Fatal("response missing attributes field")
				}
				return
			}

			if tt.wantCode == http.StatusNotFound {
				if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
					t.Fatalf("Content-Type = %q, want %q", ct, "application/problem+json")
				}
				var p domain.Problem
				if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
					t.Fatalf("failed to decode problem response: %v", err)
				}
				if p.Type != "/problems/not-found" {
					t.Fatalf("problem.Type = %q, want %q", p.Type, "/problems/not-found")
				}
			}
		})
	}
}

// TestLookupBatch verifies batch lookup with mixed results.
//
// Acceptance criteria:
//   - Response MUST have accounts array and not_found array
//   - not_found MUST be [] (empty array, not null) when all found
//   - Accounts from different servers MUST have correct source field
//   - Request without API key MUST return 401
func TestLookupBatch(t *testing.T) {
	tests := []struct {
		name         string
		usernames    []string
		attributes   []string
		withAPIKey   bool
		wantCode     int
		wantFound    int // expected number of accounts
		wantNotFound int // expected number of not_found entries
	}{
		{name: "all found across sources", usernames: []string{"110550001", "alumni01@example.com"}, attributes: []string{"mail"}, withAPIKey: true, wantCode: 200, wantFound: 2, wantNotFound: 0},
		{name: "mixed found and not found", usernames: []string{"110550001", "nobody999"}, attributes: []string{"mail"}, withAPIKey: true, wantCode: 200, wantFound: 1, wantNotFound: 1},
		{name: "no api key", usernames: []string{"110550001"}, attributes: []string{"mail"}, withAPIKey: false, wantCode: 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Send POST /api/v1/ldap/lookup/batch
			//   - For 200: verify len(accounts) == wantFound, len(not_found) == wantNotFound
			//   - For "all found": verify not_found is empty array (not null)
			//   - For 401: verify Content-Type is application/problem+json
			resp := doRequest(t, http.MethodPost, "/api/v1/ldap/lookup/batch", map[string]any{
				"usernames":  tt.usernames,
				"attributes": tt.attributes,
			}, tt.withAPIKey)
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantCode {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantCode)
			}

			if tt.wantCode == http.StatusOK {
				var got struct {
					Accounts []struct {
						Source string `json:"source"`
					} `json:"accounts"`
					NotFound []string `json:"not_found"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if len(got.Accounts) != tt.wantFound {
					t.Fatalf("len(accounts) = %d, want %d", len(got.Accounts), tt.wantFound)
				}
				if len(got.NotFound) != tt.wantNotFound {
					t.Fatalf("len(not_found) = %d, want %d", len(got.NotFound), tt.wantNotFound)
				}
				if tt.name == "all found across sources" && got.NotFound == nil {
					t.Fatal("not_found is null, want empty array")
				}
				for i, acc := range got.Accounts {
					if acc.Source == "" {
						t.Fatalf("accounts[%d].source is empty", i)
					}
				}
				return
			}

			if tt.wantCode == http.StatusUnauthorized {
				if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
					t.Fatalf("Content-Type = %q, want %q", ct, "application/problem+json")
				}
			}
		})
	}
}
