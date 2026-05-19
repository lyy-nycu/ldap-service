package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// ---------------------------------------------------------------------------
// HandleModify — provider-side contract test ported from the consumer
// at NYCUITSC/portal-backend backend-go/internal/adapter/ldap/modify_test.go
// (merged via PR #170). Each assertion here mirrors a consumer assertion
// so contract drift is caught on BOTH sides of the wire.
// ---------------------------------------------------------------------------

type mockModifyUseCase struct {
	gotSubject string
	gotAttrs   domain.ModifyAttrs
	called     int
	result     *domain.ModifyResult
	err        error
}

func (m *mockModifyUseCase) Modify(_ context.Context, subjectID string, attrs domain.ModifyAttrs) (*domain.ModifyResult, error) {
	m.called++
	m.gotSubject = subjectID
	m.gotAttrs = attrs
	return m.result, m.err
}

// Test_HandleModify_Contract_RequestShape mirrors the consumer test
// TestModifyContract_RequestShape: a fully-populated request must be
// decoded into the right ModifyAttrs (with the legacy "altemate-email"
// spelling preserved) and forwarded to the use case unchanged.
func TestHandleModify_Contract_RequestShape(t *testing.T) {
	uc := &mockModifyUseCase{
		result: &domain.ModifyResult{
			Modified: []string{"disable", "userpassword", "altemate-email"},
		},
	}
	h := HandleModify(uc)

	body := `{
	  "subject_id": "0856001",
	  "attrs": {
	    "disable": "0",
	    "userpassword": "{SSHA}abc",
	    "altemate-email": "user@example.com"
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/modify", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if uc.called != 1 {
		t.Fatalf("use case called %d times, want 1", uc.called)
	}
	if uc.gotSubject != "0856001" {
		t.Errorf("use case got subject_id = %q, want 0856001", uc.gotSubject)
	}
	if uc.gotAttrs.Disable != "0" {
		t.Errorf("attrs.disable = %q, want 0", uc.gotAttrs.Disable)
	}
	if uc.gotAttrs.UserPassword != "{SSHA}abc" {
		t.Errorf("attrs.userpassword = %q, want {SSHA}abc", uc.gotAttrs.UserPassword)
	}
	// THE typo guard.
	if uc.gotAttrs.AlternateEmail != "user@example.com" {
		t.Errorf("attrs.altemate-email decode failed (got %q); the legacy production typo must be preserved", uc.gotAttrs.AlternateEmail)
	}

	// Response must contain the wire spelling in `modified`.
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var resp struct {
		Modified []string `json:"modified"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	wantModified := []string{"disable", "userpassword", "altemate-email"}
	if len(resp.Modified) != len(wantModified) {
		t.Fatalf("modified = %v, want %v", resp.Modified, wantModified)
	}
	for i, n := range wantModified {
		if resp.Modified[i] != n {
			t.Errorf("modified[%d] = %q, want %q", i, resp.Modified[i], n)
		}
		if resp.Modified[i] == "alternate-email" {
			t.Errorf("response uses 'alternate-email'; production schema is 'altemate-email'")
		}
	}
}

func TestHandleModify_Contract_RejectsCorrectedTypo(t *testing.T) {
	// If a future caller (or a "helpful" middleware) sends the corrected
	// spelling, it must NOT silently end up in AlternateEmail. The
	// struct tag is "altemate-email,omitempty"; unknown JSON keys are
	// ignored by encoding/json, so AlternateEmail must remain empty.
	uc := &mockModifyUseCase{result: &domain.ModifyResult{Modified: []string{"disable"}}}
	h := HandleModify(uc)
	body := `{"subject_id":"0856001","attrs":{"disable":"0","alternate-email":"x@y.z"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/modify", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if uc.gotAttrs.AlternateEmail != "" {
		t.Fatalf("'alternate-email' key was silently accepted into AlternateEmail (got %q); this would mask the production typo", uc.gotAttrs.AlternateEmail)
	}
}

func TestHandleModify_Contract_PartialAttrs(t *testing.T) {
	uc := &mockModifyUseCase{result: &domain.ModifyResult{Modified: []string{"userpassword"}}}
	h := HandleModify(uc)
	body := `{"subject_id":"0856001","attrs":{"userpassword":"{SSHA}xyz"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/modify", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if uc.gotAttrs.Disable != "" || uc.gotAttrs.AlternateEmail != "" || uc.gotAttrs.TempPassword != "" {
		t.Errorf("partial request leaked extra attrs: %+v", uc.gotAttrs)
	}
	if uc.gotAttrs.UserPassword != "{SSHA}xyz" {
		t.Errorf("userpassword decode wrong: %q", uc.gotAttrs.UserPassword)
	}
}

func TestHandleModify_Contract_TempPasswordKeyName(t *testing.T) {
	uc := &mockModifyUseCase{result: &domain.ModifyResult{Modified: []string{"temppassword"}}}
	h := HandleModify(uc)
	body := `{"subject_id":"0856001","attrs":{"temppassword":"NTLM:deadbeef"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/modify", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if uc.gotAttrs.TempPassword != "NTLM:deadbeef" {
		t.Errorf("temppassword decode wrong: %q (key MUST be 'temppassword', not 'tempPassword' or 'temp_password')", uc.gotAttrs.TempPassword)
	}
}

func TestHandleModify_ErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		method     string
		ucErr      error
		wantStatus int
		wantType   string
	}{
		{name: "POST only", method: http.MethodGet, body: "", wantStatus: 405},
		{name: "invalid JSON", method: http.MethodPost, body: `{not json`, wantStatus: 400},
		{name: "empty subject_id", method: http.MethodPost, body: `{"subject_id":"","attrs":{"disable":"0"}}`, wantStatus: 400, wantType: "/problems/invalid-request"},
		{name: "no attrs", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{}}`, wantStatus: 400, wantType: "/problems/invalid-request"},
		{name: "invalid subject_id from usecase", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{"disable":"0"}}`, ucErr: domain.ErrInvalidUsername, wantStatus: 400, wantType: "/problems/invalid-username"},
		{name: "invalid attr value", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{"disable":"0"}}`, ucErr: domain.ErrInvalidAttrValue, wantStatus: 400, wantType: "/problems/invalid-attr-value"},
		{name: "not found", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{"disable":"0"}}`, ucErr: domain.ErrAccountNotFound, wantStatus: 404, wantType: "/problems/not-found"},
		{name: "schema violation", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{"userpassword":"{SSHA}reused"}}`, ucErr: domain.ErrSchemaViolation, wantStatus: 409, wantType: "/problems/schema-violation"},
		{name: "rate limited", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{"disable":"0"}}`, ucErr: domain.ErrRateLimitExceeded, wantStatus: 429, wantType: "/problems/rate-limit-exceeded"},
		// Per contract: 500 (NOT 503) is "Upstream LDAP unreachable or
		// returned a server error". Consumer adapter maps both 5xx and
		// 409 to ErrUpstream, but the wire status must be 500.
		{name: "service unavailable → 500", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{"disable":"0"}}`, ucErr: domain.ErrServiceUnavailable, wantStatus: 500, wantType: "/problems/internal-error"},
		{name: "unknown error → 500", method: http.MethodPost, body: `{"subject_id":"0856001","attrs":{"disable":"0"}}`, ucErr: errors.New("boom"), wantStatus: 500, wantType: "/problems/internal-error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &mockModifyUseCase{err: tt.ucErr, result: &domain.ModifyResult{Modified: []string{}}}
			h := HandleModify(uc)
			req := httptest.NewRequest(tt.method, "/api/v1/ldap/modify", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus >= 400 && tt.method == http.MethodPost {
				if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
					t.Errorf("Content-Type = %q, want application/problem+json", ct)
				}
				if tt.wantType != "" {
					var p domain.Problem
					if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
						t.Fatalf("decode problem: %v", err)
					}
					if p.Type != tt.wantType {
						t.Errorf("problem.type = %q, want %q", p.Type, tt.wantType)
					}
					if p.Status != tt.wantStatus {
						t.Errorf("problem.status = %d, want %d", p.Status, tt.wantStatus)
					}
				}
			}
		})
	}
}

func TestHandleModify_AdditiveOnly_DoesNotAffectLookupOrAuthenticate(t *testing.T) {
	// Belt-and-suspenders guard for Hard Rule #3 ("additive only"): the
	// modify handler must be its own http.HandlerFunc constructed from
	// the ModifyUseCase only — it must not alias HandleLookup,
	// HandleAuthenticate, or share captured state across calls.
	//
	// We compare the underlying function pointers (reflect.Value.Pointer)
	// because comparing &a == &b would compare stack-local addresses,
	// which are always distinct and would make the assertion vacuous.
	ucA := &mockModifyUseCase{result: &domain.ModifyResult{}}
	ucB := &mockModifyUseCase{result: &domain.ModifyResult{}}
	a := HandleModify(ucA)
	b := HandleModify(ucB)
	if a == nil || b == nil {
		t.Fatal("HandleModify must not return nil")
	}
	if reflect.ValueOf(a).Pointer() != reflect.ValueOf(b).Pointer() {
		// Both closures share the same generated function body (expected
		// for an HOF that returns a closure literal), so the function
		// pointer should match. What MUST differ is captured state:
		// invoking each must dispatch to its own use case.
		t.Fatalf("HandleModify closures should share function body but capture distinct state; got different function pointers")
	}

	// Prove the captured state is disjoint: invoking `a` must only touch
	// ucA, and invoking `b` must only touch ucB. If the handler
	// accidentally aliased a package-level use case (or aliased
	// HandleLookup), one of these counts would be wrong.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/modify",
		strings.NewReader(`{"subject_id":"0856001","attrs":{"disable":"0"}}`))
	req.Header.Set("Content-Type", "application/json")
	a.ServeHTTP(httptest.NewRecorder(), req)
	if ucA.called != 1 || ucB.called != 0 {
		t.Fatalf("invoking handler a dispatched to wrong use case: ucA.called=%d ucB.called=%d", ucA.called, ucB.called)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/ldap/modify",
		strings.NewReader(`{"subject_id":"0856001","attrs":{"disable":"0"}}`))
	req2.Header.Set("Content-Type", "application/json")
	b.ServeHTTP(httptest.NewRecorder(), req2)
	if ucA.called != 1 || ucB.called != 1 {
		t.Fatalf("invoking handler b dispatched to wrong use case: ucA.called=%d ucB.called=%d", ucA.called, ucB.called)
	}
}
