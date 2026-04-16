package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleLookup(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{name: "valid lookup", method: http.MethodPost, body: `{"username":"110550001","attributes":["mail"]}`, wantStatus: 200},
		{name: "invalid JSON", method: http.MethodPost, body: `{invalid`, wantStatus: 400},
		{name: "missing username", method: http.MethodPost, body: `{"attributes":["mail"]}`, wantStatus: 400},
		{name: "missing attributes", method: http.MethodPost, body: `{"username":"110550001"}`, wantStatus: 400},
		{name: "wrong method", method: http.MethodGet, body: "", wantStatus: 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Create a mock LookupUseCase
			//   - Valid: status 200, response contains dn, uid, source, attributes
			//   - Invalid: status matches, Content-Type is application/problem+json
			panic("not implemented")
		})
	}
}

func TestHandleBatchLookup(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "valid batch", body: `{"usernames":["110550001","T1234"],"attributes":["mail"]}`, wantStatus: 200},
		{name: "empty usernames", body: `{"usernames":[],"attributes":["mail"]}`, wantStatus: 400},
		{name: "invalid JSON", body: `{bad`, wantStatus: 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Valid: status 200, response has accounts array and not_found array
			//   - not_found MUST be [] (empty array), never null
			panic("not implemented")
		})
	}
}

// Ensure imports are used.
var _ = httptest.NewRequest
