package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAuthenticate(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		mockOK     bool
		mockErr    error
		wantStatus int
	}{
		{name: "successful auth", method: http.MethodPost, body: `{"username":"110550001","password":"correct"}`, mockOK: true, wantStatus: 200},
		{name: "failed auth", method: http.MethodPost, body: `{"username":"110550001","password":"wrong"}`, mockOK: false, wantStatus: 401},
		{name: "invalid JSON", method: http.MethodPost, body: `{bad`, wantStatus: 400},
		{name: "missing username", method: http.MethodPost, body: `{"password":"pass"}`, wantStatus: 400},
		{name: "missing password", method: http.MethodPost, body: `{"username":"110550001"}`, wantStatus: 400},
		{name: "wrong method", method: http.MethodGet, body: "", wantStatus: 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - 200: body is {"authenticated":true}
			//   - 401: Content-Type is application/problem+json, detail is "authentication failed"
			//   - Response MUST NOT contain password in any form
			panic("not implemented")
		})
	}
}

// Ensure imports are used.
var _ = httptest.NewRequest
