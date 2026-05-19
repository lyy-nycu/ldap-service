//go:build integration

package integration

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestModify_PasswordReplace exercises the full activation-commit flow
// the consumer in NYCUITSC/portal-backend depends on: replace the
// `userpassword` attribute via POST /api/v1/ldap/modify, then verify
// the new password works via POST /api/v1/ldap/authenticate.
//
// This test runs against the docker-compose OpenLDAP fixture
// (osixia/openldap with the standard inetOrgPerson schema). The
// `userpassword` attribute is part of that schema, so the write
// succeeds end-to-end.
//
// The other contract attributes (`disable`, `altemate-email`,
// `temppassword`) belong to the NYCU production custom schema, which
// the test fixture does NOT load. Those are covered by
// TestModify_UnknownAttributeReturns409 below, which verifies the
// 409 schema-violation mapping is wired correctly.
func TestModify_PasswordReplace(t *testing.T) {
	const subjectID = "110550002"
	const newPassword = "rotated-pass-XYZ-987"

	hashed := sshaHash(t, newPassword)
	if !strings.HasPrefix(hashed, "{SSHA}") {
		t.Fatalf("ssha hash malformed: %s", hashed)
	}

	resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
		"subject_id": subjectID,
		"attrs": map[string]any{
			"userpassword": hashed,
		},
	}, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("modify status = %d, want 200, body = %s", resp.StatusCode, body)
	}

	var modResp struct {
		Modified []string `json:"modified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modResp); err != nil {
		t.Fatalf("decode modify response: %v", err)
	}
	if len(modResp.Modified) != 1 || modResp.Modified[0] != "userpassword" {
		t.Fatalf("modified = %v, want [userpassword]", modResp.Modified)
	}

	// Restore the original password at end of test so other tests in
	// the suite aren't flaky on re-runs.
	t.Cleanup(func() {
		restoreHash := sshaHash(t, "testpass123")
		r := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
			"subject_id": subjectID,
			"attrs":      map[string]any{"userpassword": restoreHash},
		}, true)
		_ = r.Body.Close()
	})

	// Verify by binding with the NEW password.
	authResp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
		"username": subjectID,
		"password": newPassword,
	}, true)
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(authResp.Body)
		t.Fatalf("authenticate with new password: status = %d, want 200, body = %s", authResp.StatusCode, body)
	}
	var authBody struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.NewDecoder(authResp.Body).Decode(&authBody); err != nil {
		t.Fatalf("decode authenticate response: %v", err)
	}
	if !authBody.Authenticated {
		t.Fatalf("authenticated = false after password modify; SSHA was not accepted by upstream LDAP")
	}

	// Verify the OLD password no longer works (defensive — guards
	// against a future bug where Replace is silently translated to Add).
	oldAuthResp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
		"username": subjectID,
		"password": "testpass123",
	}, true)
	defer oldAuthResp.Body.Close()
	if oldAuthResp.StatusCode == http.StatusOK {
		var oldBody struct {
			Authenticated bool `json:"authenticated"`
		}
		_ = json.NewDecoder(oldAuthResp.Body).Decode(&oldBody)
		if oldBody.Authenticated {
			t.Fatalf("old password still works after modify; replace op did not take effect")
		}
	}
}

// TestModify_UnknownAttributeReturns409 verifies the schema-violation
// → 409 mapping the consumer adapter (ErrUpstream sentinel) depends on.
//
// The osixia/openldap fixture does NOT load the NYCU custom schema, so
// any of `disable` / `altemate-email` / `temppassword` triggers a real
// LDAPResultUndefinedAttributeType (16) from slapd. That exercises
// exactly the code path Pool.Modify maps to domain.ErrSchemaViolation,
// which the handler in turn maps to 409 application/problem+json.
//
// This is the closest we can get to a true production-schema integration
// test without a custom-schema bootstrap; unit tests in
// pool_modify_test.go cover the schema-loaded path with mocks.
func TestModify_UnknownAttributeReturns409(t *testing.T) {
	tests := []struct{ name, attr, value string }{
		{"disable", "disable", "0"},
		{"altemate-email (legacy production typo)", "altemate-email", "user@example.com"},
		{"temppassword", "temppassword", "NTLM:deadbeef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
				"subject_id": "110550001",
				"attrs":      map[string]any{tt.attr: tt.value},
			}, true)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusConflict {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d, want 409 (schema-violation) for unknown attr %q; body = %s",
					resp.StatusCode, tt.attr, body)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
				t.Errorf("Content-Type = %q, want application/problem+json", ct)
			}
			var p struct {
				Type   string `json:"type"`
				Status int    `json:"status"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if p.Type != "/problems/schema-violation" {
				t.Errorf("problem.type = %q, want /problems/schema-violation", p.Type)
			}
			if p.Status != 409 {
				t.Errorf("problem.status = %d, want 409", p.Status)
			}
		})
	}
}

func TestModify_404OnUnknownSubject(t *testing.T) {
	resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
		"subject_id": "ghost-no-such-user",
		"attrs":      map[string]any{"userpassword": "{SSHA}whatever"},
	}, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 404, body = %s", resp.StatusCode, body)
	}
	var p struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p.Type != "/problems/not-found" {
		t.Errorf("problem.type = %q, want /problems/not-found", p.Type)
	}
}

func TestModify_400OnEmptySubjectID(t *testing.T) {
	resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
		"subject_id": "",
		"attrs":      map[string]any{"userpassword": "{SSHA}whatever"},
	}, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestModify_PlaintextPassword_ServerSideHashing locks in the relaxed
// userpassword contract: a plaintext value is accepted, slapd applies
// its password-hash directive, and the user can subsequently bind with
// the same plaintext. This is the end-to-end proof that ppolicy
// (history, quality) now has visibility into passwords.
//
// NOTE: this test does not snapshot the stored userPassword attribute
// because the read-only bind DN used by lookup typically lacks ACL to
// read it. The bind-success leg is the authoritative signal.
func TestModify_PlaintextPassword_ServerSideHashing(t *testing.T) {
	// Use RT00001 (retire OU) to avoid rate-limit collision with
	// TestAuthenticate / TestRateLimit which exercise student subjects.
	const subjectID = "RT00001"
	const plaintextNew = "ppolicy-friendly-plain-PW-12345"

	resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
		"subject_id": subjectID,
		"attrs":      map[string]any{"userpassword": plaintextNew},
	}, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("modify(plaintext) status = %d, want 200, body = %s", resp.StatusCode, body)
	}
	t.Cleanup(func() {
		// Best-effort: restore the fixture password using the legacy
		// SSHA pass-through so subsequent tests keep working.
		restoreHash := sshaHash(t, "testpass123")
		r := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
			"subject_id": subjectID,
			"attrs":      map[string]any{"userpassword": restoreHash},
		}, true)
		r.Body.Close()
	})

	authResp := doRequest(t, http.MethodPost, "/api/v1/ldap/authenticate", map[string]any{
		"username": subjectID,
		"password": plaintextNew,
	}, true)
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(authResp.Body)
		t.Fatalf("authenticate(plaintext-new) status = %d, want 200 (slapd should have hashed plaintext); body = %s", authResp.StatusCode, body)
	}
	var ar struct {
		Authenticated bool `json:"authenticated"`
	}
	_ = json.NewDecoder(authResp.Body).Decode(&ar)
	if !ar.Authenticated {
		t.Fatalf("authenticated = false, want true — slapd did not hash the plaintext (check the server's password-hash directive)")
	}
}

// TestModify_SSHAPassThrough_StillAccepted: the legacy pre-hashed
// payload from portal-backend must keep working during rollout so the
// change is additive on the wire.
func TestModify_SSHAPassThrough_StillAccepted(t *testing.T) {
	const subjectID = "110550002"
	hashed := sshaHash(t, "ssha-passthru-XYZ")
	resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
		"subject_id": subjectID,
		"attrs":      map[string]any{"userpassword": hashed},
	}, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("modify({SSHA}) status = %d, want 200, body = %s", resp.StatusCode, body)
	}
	t.Cleanup(func() {
		restoreHash := sshaHash(t, "testpass123")
		r := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
			"subject_id": subjectID,
			"attrs":      map[string]any{"userpassword": restoreHash},
		}, true)
		r.Body.Close()
	})
}

// TestModify_400OnPlaintextWithNullByte: plaintext is allowed, but the
// input guards (NUL / C0 / DEL / >256 bytes) still reject malformed
// values. This replaces the obsolete "userpassword must start with
// {SSHA}" rejection test.
func TestModify_400OnPlaintextWithNullByte(t *testing.T) {
	resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
		"subject_id": "110550001",
		"attrs":      map[string]any{"userpassword": "abc\x00def"},
	}, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (NUL byte in plaintext userpassword)", resp.StatusCode)
	}
	var p struct {
		Type   string `json:"type"`
		Detail string `json:"detail"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&p)
	if p.Type != "/problems/invalid-attr-value" {
		t.Errorf("problem.type = %q, want /problems/invalid-attr-value", p.Type)
	}
	if strings.Contains(p.Detail, "abc") || strings.Contains(p.Detail, "def") {
		t.Errorf("problem.detail leaks user-supplied bytes: %q", p.Detail)
	}
}

func TestModify_401WithoutAPIKey(t *testing.T) {
	resp := doRequest(t, http.MethodPost, "/api/v1/ldap/modify", map[string]any{
		"subject_id": "110550001",
		"attrs":      map[string]any{"userpassword": "{SSHA}whatever"},
	}, false)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when X-Api-Key is missing", resp.StatusCode)
	}
}

// sshaHash builds an OpenLDAP-compatible {SSHA} hash: base64(SHA1(pw||salt) || salt)
// with a 4-byte random salt.
func sshaHash(t *testing.T, password string) string {
	t.Helper()
	salt := make([]byte, 4)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand: %v", err)
	}
	h := sha1.New()
	h.Write([]byte(password))
	h.Write(salt)
	digest := h.Sum(nil)
	return "{SSHA}" + base64.StdEncoding.EncodeToString(append(digest, salt...))
}
