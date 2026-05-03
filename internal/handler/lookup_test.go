package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

type mockLookupUseCase struct {
	lookupResult      *domain.Account
	lookupErr         error
	lookupCalled      int
	lookupBatchResult []*domain.Account
	lookupBatchMissed []string
	lookupBatchErr    error
	lookupBatchCalled int
}

func (m *mockLookupUseCase) Lookup(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	m.lookupCalled++
	return m.lookupResult, m.lookupErr
}

func (m *mockLookupUseCase) LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*domain.Account, []string, error) {
	m.lookupBatchCalled++
	return m.lookupBatchResult, m.lookupBatchMissed, m.lookupBatchErr
}

func TestHandleLookup(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		lookupRes  *domain.Account
		lookupErr  error
		wantStatus int
		wantType   string
	}{
		{name: "valid lookup", method: http.MethodPost, body: `{"username":"110550001","attributes":["mail"]}`, lookupRes: &domain.Account{DN: "uid=110550001,ou=student,o=nycu", UID: "110550001", Source: domain.SourceInternal, Attributes: map[string]string{"mail": "s@nycu.edu.tw"}}, wantStatus: 200},
		{name: "valid lookup with fullname and initials", method: http.MethodPost, body: `{"username":"110550001","attributes":["fullname","initials"]}`, lookupRes: &domain.Account{DN: "uid=110550001,ou=student,o=nycu", UID: "110550001", Source: domain.SourceInternal, Attributes: map[string]string{"fullname": "Student User", "initials": "SU"}}, wantStatus: 200},
		{name: "valid lookup external email", method: http.MethodPost, body: `{"username":"alumni@example.com","attributes":["mail"]}`, lookupRes: &domain.Account{DN: "uid=alumni@example.com,ou=alumni,o=nycu", UID: "alumni@example.com", Source: domain.SourceExternal, Attributes: map[string]string{"mail": "a@nycu.edu.tw"}}, wantStatus: 200},
		{name: "invalid JSON", method: http.MethodPost, body: `{invalid`, wantStatus: 400},
		{name: "missing username", method: http.MethodPost, body: `{"attributes":["mail"]}`, wantStatus: 400},
		{name: "missing attributes", method: http.MethodPost, body: `{"username":"110550001"}`, wantStatus: 400},
		{name: "wrong method", method: http.MethodGet, body: "", wantStatus: 405},
		{name: "account not found", method: http.MethodPost, body: `{"username":"nobody","attributes":["mail"]}`, lookupErr: domain.ErrAccountNotFound, wantStatus: 404, wantType: "/problems/not-found"},
		{name: "invalid username", method: http.MethodPost, body: `{"username":"bad)(u","attributes":["mail"]}`, lookupErr: domain.ErrInvalidUsername, wantStatus: 400, wantType: "/problems/invalid-username"},
		{name: "disallowed attribute", method: http.MethodPost, body: `{"username":"110550001","attributes":["userPassword"]}`, lookupErr: domain.ErrAttributeNotAllowed, wantStatus: 400, wantType: "/problems/attribute-not-allowed"},
		{name: "service unavailable", method: http.MethodPost, body: `{"username":"110550001","attributes":["mail"]}`, lookupErr: domain.ErrServiceUnavailable, wantStatus: 503, wantType: "/problems/service-unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create a mock LookupUseCase
			//   - Valid: status 200, response contains dn, uid, source, attributes
			//   - Invalid: status matches, Content-Type is application/problem+json
			uc := &mockLookupUseCase{lookupResult: tt.lookupRes, lookupErr: tt.lookupErr}
			h := HandleLookup(uc)

			req := httptest.NewRequest(tt.method, "/api/v1/ldap/lookup", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if rec.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
				}
				var body map[string]any
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if _, ok := body["dn"]; !ok {
					t.Fatal("missing dn in response")
				}
				if _, ok := body["uid"]; !ok {
					t.Fatal("missing uid in response")
				}
				if _, ok := body["source"]; !ok {
					t.Fatal("missing source in response")
				}
				if _, ok := body["attributes"]; !ok {
					t.Fatal("missing attributes in response")
				}
				return
			}

			if tt.wantStatus >= 400 && tt.wantStatus != http.StatusMethodNotAllowed {
				if rec.Header().Get("Content-Type") != "application/problem+json" {
					t.Fatalf("Content-Type = %q, want application/problem+json", rec.Header().Get("Content-Type"))
				}
				if tt.wantType != "" {
					var p domain.Problem
					if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
						t.Fatalf("decode problem: %v", err)
					}
					if p.Type != tt.wantType {
						t.Fatalf("problem type = %q, want %q", p.Type, tt.wantType)
					}
				}
			}
		})
	}
}

func TestHandleBatchLookup(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		batchRes   []*domain.Account
		batchMiss  []string
		wantStatus int
		wantType   string
	}{
		{name: "valid batch", method: http.MethodPost, body: `{"usernames":["110550001","T1234"],"attributes":["mail"]}`, batchRes: []*domain.Account{{DN: "uid=110550001,ou=student,o=nycu", UID: "110550001", Source: domain.SourceInternal, Attributes: map[string]string{"mail": "a"}}}, batchMiss: []string{"T1234"}, wantStatus: 200},
		{name: "valid batch with fullname and initials", method: http.MethodPost, body: `{"usernames":["110550001"],"attributes":["fullname","initials"]}`, batchRes: []*domain.Account{{DN: "uid=110550001,ou=student,o=nycu", UID: "110550001", Source: domain.SourceInternal, Attributes: map[string]string{"fullname": "Student User", "initials": "SU"}}}, batchMiss: []string{}, wantStatus: 200},
		{name: "empty usernames", method: http.MethodPost, body: `{"usernames":[],"attributes":["mail"]}`, wantStatus: 400},
		{name: "invalid JSON", method: http.MethodPost, body: `{bad`, wantStatus: 400},
		{name: "wrong method", method: http.MethodGet, body: "", wantStatus: 405},
		{name: "missing attributes", method: http.MethodPost, body: `{"usernames":["110550001"]}`, wantStatus: 400},
		{name: "all found", method: http.MethodPost, body: `{"usernames":["110550001"],"attributes":["mail"]}`, batchRes: []*domain.Account{{DN: "uid=110550001,ou=student,o=nycu", UID: "110550001", Source: domain.SourceInternal, Attributes: map[string]string{"mail": "a"}}}, batchMiss: []string{}, wantStatus: 200},
		{name: "mixed found and not found", method: http.MethodPost, body: `{"usernames":["110550001","nobody"],"attributes":["mail"]}`, batchRes: []*domain.Account{{DN: "uid=110550001,ou=student,o=nycu", UID: "110550001", Source: domain.SourceInternal, Attributes: map[string]string{"mail": "a"}}}, batchMiss: []string{"nobody"}, wantStatus: 200},
		{name: "batch service unavailable", method: http.MethodPost, body: `{"usernames":["110550001"],"attributes":["mail"]}`, wantStatus: 503, wantType: "/problems/service-unavailable"},
		{name: "batch exceeds limit maps to invalid request", method: http.MethodPost, body: `{"usernames":["110550001"],"attributes":["mail"]}`, wantStatus: 400, wantType: "/problems/invalid-request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Valid: status 200, response has accounts array and not_found array
			//   - not_found MUST be [] (empty array), never null
			uc := &mockLookupUseCase{lookupBatchResult: tt.batchRes, lookupBatchMissed: tt.batchMiss}
			if tt.wantStatus == 503 {
				uc.lookupBatchErr = domain.ErrServiceUnavailable
			}
			if tt.wantStatus == 400 && tt.wantType == "/problems/invalid-request" {
				uc.lookupBatchErr = domain.ErrBatchSizeExceeded
			}
			h := HandleBatchLookup(uc)

			req := httptest.NewRequest(tt.method, "/api/v1/ldap/lookup/batch", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if rec.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
				}

				var body struct {
					Accounts []map[string]any `json:"accounts"`
					NotFound []string         `json:"not_found"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("decode response: %v", err)
				}

				if body.NotFound == nil {
					t.Fatal("not_found is nil, want []")
				}

				for i, acc := range body.Accounts {
					if _, ok := acc["source"]; !ok {
						t.Fatalf("accounts[%d] missing source field", i)
					}
				}

				if tt.name == "all found" && len(body.NotFound) != 0 {
					t.Fatalf("not_found = %v, want empty array", body.NotFound)
				}
				if tt.name == "mixed found and not found" {
					if len(body.Accounts) != 1 || len(body.NotFound) != 1 || body.NotFound[0] != "nobody" {
						t.Fatalf("unexpected mixed response accounts=%v not_found=%v", body.Accounts, body.NotFound)
					}
				}
				return
			}

			if tt.wantStatus >= 400 && tt.wantStatus != http.StatusMethodNotAllowed {
				if rec.Header().Get("Content-Type") != "application/problem+json" {
					t.Fatalf("Content-Type = %q, want application/problem+json", rec.Header().Get("Content-Type"))
				}
				if tt.wantType != "" {
					var p domain.Problem
					if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
						t.Fatalf("decode problem: %v", err)
					}
					if p.Type != tt.wantType {
						t.Fatalf("problem type = %q, want %q", p.Type, tt.wantType)
					}
				}
			}
		})
	}
}

// Ensure imports are used.
var _ = httptest.NewRequest
