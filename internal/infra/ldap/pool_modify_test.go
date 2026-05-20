package ldap

import (
	"context"
	"errors"
	"strings"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// ---------------------------------------------------------------------------
// Pool.Modify — locked contract test (security-critical, do NOT modify)
//
// These tests pin the wire-level behavior the consumer in
// NYCUITSC/portal-backend depends on (see PR #170). They are the RED
// half of the TDD pair for the /api/v1/ldap/modify endpoint.
// ---------------------------------------------------------------------------

func TestPool_Modify_OneReplaceOpPerAttr_PreservesLegacyKeyNames(t *testing.T) {
	// Setup: pool with one mock conn. The mock returns a single search
	// entry for any search (subjectID → DN resolution) and records the
	// ldap.ModifyRequest it was asked to issue.
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{{DN: "uid=0856001,ou=student,o=nycu"}},
			}, nil
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	attrs := []domain.ModifyAttr{
		{Name: "disable", Value: "0"},
		{Name: "userpassword", Value: "{SSHA}abc=="},
		{Name: "altemate-email", Value: "user@example.com"},
		{Name: "temppassword", Value: "NTLM:deadbeef"},
	}

	if err := p.Modify(context.Background(), "0856001", attrs); err != nil {
		t.Fatalf("Modify() error = %v, want nil", err)
	}

	if conn.modifyCalls != 1 {
		t.Fatalf("ldap.Modify called %d times, want exactly 1 (atomicity invariant)", conn.modifyCalls)
	}

	req := conn.lastModifyReq
	if req == nil {
		t.Fatal("Modify request was nil")
	}
	if req.DN != "uid=0856001,ou=student,o=nycu" {
		t.Errorf("Modify.DN = %q, want resolved DN from search", req.DN)
	}

	// One replace op per attr, in the same order, with the wire spelling
	// preserved verbatim — INCLUDING the legacy "altemate-email" typo.
	if len(req.Changes) != 4 {
		t.Fatalf("Modify has %d changes, want 4 (one replace per attr); changes=%+v", len(req.Changes), req.Changes)
	}
	wantOrder := []struct {
		name  string
		value string
	}{
		{"disable", "0"},
		{"userpassword", "{SSHA}abc=="},
		{"altemate-email", "user@example.com"},
		{"temppassword", "NTLM:deadbeef"},
	}
	for i, want := range wantOrder {
		ch := req.Changes[i]
		if ch.Operation != ldapv3.ReplaceAttribute {
			t.Errorf("change[%d].Operation = %d, want ReplaceAttribute (%d) — each attr must be a `replace` op",
				i, ch.Operation, ldapv3.ReplaceAttribute)
		}
		if ch.Modification.Type != want.name {
			t.Errorf("change[%d].Type = %q, want %q (do NOT rewrite the attribute name)",
				i, ch.Modification.Type, want.name)
		}
		if len(ch.Modification.Vals) != 1 || ch.Modification.Vals[0] != want.value {
			t.Errorf("change[%d].Vals = %v, want [%q]", i, ch.Modification.Vals, want.value)
		}
	}

	// The notorious typo guard.
	for _, ch := range req.Changes {
		if ch.Modification.Type == "alternate-email" {
			t.Errorf("found 'alternate-email' in upstream Modify request — the production schema uses 'altemate-email' (legacy typo). Do not silently rename.")
		}
	}
}

func TestPool_Modify_NotFoundReturnsErrAccountNotFound(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{Entries: nil}, nil
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	err := p.Modify(context.Background(), "ghost", []domain.ModifyAttr{
		{Name: "disable", Value: "0"},
	})
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("Modify() err = %v, want domain.ErrAccountNotFound", err)
	}
	if conn.modifyCalls != 0 {
		t.Errorf("ldap.Modify must NOT be called when DN resolution fails; got %d calls", conn.modifyCalls)
	}
}

func TestPool_Modify_SchemaViolationMappedToErrSchemaViolation(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{{DN: "uid=0856001,ou=student,o=nycu"}},
			}, nil
		},
		modifyFn: func(req *ldapv3.ModifyRequest) error {
			return &ldapv3.Error{ResultCode: ldapv3.LDAPResultConstraintViolation, Err: errors.New("password reuse")}
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	err := p.Modify(context.Background(), "0856001", []domain.ModifyAttr{
		{Name: "userpassword", Value: "{SSHA}reused"},
	})
	if !errors.Is(err, domain.ErrSchemaViolation) {
		t.Fatalf("Modify() err = %v, want domain.ErrSchemaViolation", err)
	}
}

func TestPool_Modify_UndefinedAttributeAlsoSchemaViolation(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{{DN: "uid=0856001,ou=student,o=nycu"}},
			}, nil
		},
		modifyFn: func(req *ldapv3.ModifyRequest) error {
			return &ldapv3.Error{ResultCode: ldapv3.LDAPResultUndefinedAttributeType, Err: errors.New("no such attr")}
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	err := p.Modify(context.Background(), "0856001", []domain.ModifyAttr{
		{Name: "altemate-email", Value: "user@example.com"},
	})
	if !errors.Is(err, domain.ErrSchemaViolation) {
		t.Fatalf("Modify() err = %v, want domain.ErrSchemaViolation for undefined attribute", err)
	}
}

func TestPool_Modify_SearchUsesEscapeFilter(t *testing.T) {
	// LDAP injection attempt in subject_id; mock asserts the filter
	// it received is the escaped form. The Search should reject (no
	// matching entry) and Modify must never be issued.
	var gotFilter string
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			gotFilter = req.Filter
			return &ldapv3.SearchResult{Entries: nil}, nil
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	_ = p.Modify(context.Background(), "evil)(uid=*", []domain.ModifyAttr{
		{Name: "disable", Value: "0"},
	})

	if gotFilter == "" {
		t.Fatal("search filter was empty — DN resolution was skipped")
	}
	// The unsafe substring "evil)(uid=*" MUST NOT appear verbatim in
	// the filter. ldapv3.EscapeFilter escapes ) and * so the original
	// sequence cannot survive.
	if containsUnescaped(gotFilter, "evil)(uid=*") {
		t.Errorf("subject_id reached LDAP filter unescaped: %q — must use ldapv3.EscapeFilter", gotFilter)
	}
}

// ---------------------------------------------------------------------------
// relax-modify-userpassword — RED tests for the TLS precondition
// (see openspec/changes/relax-modify-userpassword/specs/ldap-repository/spec.md)
// ---------------------------------------------------------------------------

// newTestPoolNonTLS returns a Pool identical to newTestPool but with
// useTLS=false, so we can assert the per-request TLS gate on the
// password-modify path.
func newTestPoolNonTLS(t *testing.T, source string, initial []*mockConn) *Pool {
	t.Helper()
	p := newTestPool(t, source, initial, nil)
	p.useTLS = false
	return p
}

// TestPool_Modify_NonTLS_RefusesUserpassword: when the pool is configured
// without TLS, a Modify request that contains userpassword MUST be
// rejected with ErrServiceUnavailable and no Modify PDU may be sent.
// The password value MUST NOT appear in any observed log entry.
func TestPool_Modify_NonTLS_RefusesUserpassword(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{{DN: "uid=0856001,ou=student,o=nycu"}},
			}, nil
		},
	}
	p := newTestPoolNonTLS(t, domain.SourceInternal, []*mockConn{conn})

	core, observed := observer.New(zap.ErrorLevel)
	p.logger = zap.New(core)

	const secret = "TLS-GATE-SECRET-TOKEN"
	err := p.Modify(context.Background(), "0856001", []domain.ModifyAttr{
		{Name: "userpassword", Value: secret},
	})
	if !errors.Is(err, domain.ErrServiceUnavailable) {
		t.Fatalf("Modify() err = %v, want domain.ErrServiceUnavailable", err)
	}
	if conn.modifyCalls != 0 {
		t.Errorf("ldap.Modify must NOT be called when TLS gate trips; got %d calls", conn.modifyCalls)
	}
	for _, entry := range observed.All() {
		serialized := entry.Message
		for _, f := range entry.Context {
			serialized += " " + f.String
		}
		if strings.Contains(serialized, secret) {
			t.Errorf("log entry leaked userpassword value: %q", serialized)
		}
	}
	if observed.FilterMessage("refusing to send userpassword over non-TLS ldap connection").Len() == 0 {
		t.Errorf("expected an error log entry explaining the TLS refusal; got: %v", observed.All())
	}
}

// TestPool_Modify_NonTLS_AllowsNonPasswordModify: the TLS gate must NOT
// affect modifications that do not include userpassword.
func TestPool_Modify_NonTLS_AllowsNonPasswordModify(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{{DN: "uid=0856001,ou=student,o=nycu"}},
			}, nil
		},
	}
	p := newTestPoolNonTLS(t, domain.SourceInternal, []*mockConn{conn})

	err := p.Modify(context.Background(), "0856001", []domain.ModifyAttr{
		{Name: "disable", Value: "1"},
		{Name: "altemate-email", Value: "user@example.com"},
	})
	if err != nil {
		t.Fatalf("Modify() err = %v, want nil (TLS gate only applies to userpassword)", err)
	}
	if conn.modifyCalls != 1 {
		t.Errorf("ldap.Modify calls = %d, want 1", conn.modifyCalls)
	}
}

// TestPool_Modify_TLS_ForwardsUserpasswordVerbatim: when TLS is on,
// userpassword is forwarded byte-for-byte to the Replace op, for both
// plaintext and {scheme} values.
func TestPool_Modify_TLS_ForwardsUserpasswordVerbatim(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{name: "plaintext", value: "p4ssw0rd!"},
		{name: "ssha", value: "{SSHA}abc=="},
		{name: "argon2", value: "{ARGON2}$argon2id$v=19$..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := &mockConn{
				searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
					return &ldapv3.SearchResult{
						Entries: []*ldapv3.Entry{{DN: "uid=0856001,ou=student,o=nycu"}},
					}, nil
				},
			}
			p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)
			if err := p.Modify(context.Background(), "0856001", []domain.ModifyAttr{
				{Name: "userpassword", Value: tc.value},
			}); err != nil {
				t.Fatalf("Modify() err = %v", err)
			}
			req := conn.lastModifyReq
			if req == nil || len(req.Changes) != 1 {
				t.Fatalf("expected exactly one Replace op, got %+v", req)
			}
			vals := req.Changes[0].Modification.Vals
			if len(vals) != 1 || vals[0] != tc.value {
				t.Errorf("Replace vals = %v, want [%q] (forward verbatim, no client-side hashing)", vals, tc.value)
			}
		})
	}
}

func containsUnescaped(filter, needle string) bool {
	// crude substring check: real escape replaces ) → \29 and * → \2a,
	// so the literal needle should not appear.
	for i := 0; i+len(needle) <= len(filter); i++ {
		if filter[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
