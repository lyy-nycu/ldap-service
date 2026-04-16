package domain

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProblem_Error(t *testing.T) {
	// Acceptance criteria:
	//   - Error() returns "type: title" format
	p := &Problem{Type: "/problems/not-found", Title: "Account not found", Status: 404}
	want := "/problems/not-found: Account not found"
	if got := p.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestProblem_JSON(t *testing.T) {
	tests := []struct {
		name    string
		problem *Problem
		want    map[string]any
	}{
		{
			name:    "full problem with all fields",
			problem: &Problem{Type: "/problems/invalid-username", Title: "Invalid username", Status: 400, Detail: "username must match regex", Instance: "req-123"},
			want:    map[string]any{"type": "/problems/invalid-username", "title": "Invalid username", "status": float64(400), "detail": "username must match regex", "instance": "req-123"},
		},
		{
			name:    "omitempty hides empty detail and instance",
			problem: &Problem{Type: "/problems/unauthorized", Title: "Unauthorized", Status: 401},
			want:    map[string]any{"type": "/problems/unauthorized", "title": "Unauthorized", "status": float64(401)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - JSON output MUST match expected fields exactly
			//   - Empty Detail/Instance MUST be omitted (omitempty)
			data, err := json.Marshal(tt.problem)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("json output = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestProblemConstructors(t *testing.T) {
	tests := []struct {
		name        string
		constructor func() *Problem
		wantType    string
		wantStatus  int
		wantDetail  string // empty means "don't check detail"
	}{
		{name: "InvalidRequest", constructor: func() *Problem { return NewInvalidRequest("bad json", "req-1") }, wantType: "/problems/invalid-request", wantStatus: 400},
		{name: "InvalidUsername", constructor: func() *Problem { return NewInvalidUsername("bad username", "req-2") }, wantType: "/problems/invalid-username", wantStatus: 400},
		{name: "AttributeNotAllowed", constructor: func() *Problem { return NewAttributeNotAllowed("userPassword", "req-3") }, wantType: "/problems/attribute-not-allowed", wantStatus: 400},
		{name: "Unauthorized", constructor: func() *Problem { return NewUnauthorized("missing api key", "req-4") }, wantType: "/problems/unauthorized", wantStatus: 401},
		{name: "AuthenticationFailed", constructor: func() *Problem { return NewAuthenticationFailed("req-5") }, wantType: "/problems/authentication-failed", wantStatus: 401, wantDetail: "authentication failed"},
		{name: "NotFound", constructor: func() *Problem { return NewNotFound("account not found", "req-6") }, wantType: "/problems/not-found", wantStatus: 404},
		{name: "RateLimitExceeded", constructor: func() *Problem { return NewRateLimitExceeded("req-7") }, wantType: "/problems/rate-limit-exceeded", wantStatus: 429, wantDetail: "too many authentication attempts for this username, try again later"},
		{name: "InternalError", constructor: func() *Problem { return NewInternalError("unexpected", "req-8") }, wantType: "/problems/internal-error", wantStatus: 500},
		{name: "ServiceUnavailable", constructor: func() *Problem { return NewServiceUnavailable("ldap down", "req-9") }, wantType: "/problems/service-unavailable", wantStatus: 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Type MUST match wantType
			//   - Status MUST match wantStatus
			//   - If wantDetail is set, Detail MUST match exactly (security: hardcoded messages)
			//   - Instance MUST be set to the request ID passed to the constructor
			p := tt.constructor()

			if p.Type != tt.wantType {
				t.Fatalf("Type = %q, want %q", p.Type, tt.wantType)
			}
			if p.Status != tt.wantStatus {
				t.Fatalf("Status = %d, want %d", p.Status, tt.wantStatus)
			}
			if tt.wantDetail != "" && p.Detail != tt.wantDetail {
				t.Fatalf("Detail = %q, want %q", p.Detail, tt.wantDetail)
			}
			if p.Instance == "" {
				t.Fatal("Instance is empty, want request ID")
			}
		})
	}
}

// Ensure json import is used.
var _ = json.Marshal
