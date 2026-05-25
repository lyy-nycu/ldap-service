package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// ModifyService — locked validation + happy-path tests.

type modifyMockRepo struct {
	modifyFn    func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error
	gotSubject  string
	gotAttrs    []domain.ModifyAttr
	calls       int
}

func (m *modifyMockRepo) Lookup(context.Context, string, []string) (*domain.Account, error) {
	panic("unused")
}
func (m *modifyMockRepo) LookupBatch(context.Context, []string, []string) ([]*domain.Account, []string, error) {
	panic("unused")
}
func (m *modifyMockRepo) Authenticate(context.Context, string, string) (*domain.AuthenticateResult, error) {
	panic("unused")
}
func (m *modifyMockRepo) HealthCheck(context.Context) error { panic("unused") }
func (m *modifyMockRepo) Modify(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
	m.calls++
	m.gotSubject = subjectID
	m.gotAttrs = attrs
	if m.modifyFn != nil {
		return m.modifyFn(ctx, subjectID, attrs)
	}
	return nil
}

func TestModifyService_Validation(t *testing.T) {
	tests := []struct {
		name      string
		subjectID string
		attrs     domain.ModifyAttrs
		wantErr   error
	}{
		{name: "empty subject_id", subjectID: "", attrs: domain.ModifyAttrs{Disable: "0"}, wantErr: domain.ErrSubjectIDRequired},
		{name: "LDAP injection in subject_id", subjectID: "evil)(uid=*", attrs: domain.ModifyAttrs{Disable: "0"}, wantErr: domain.ErrInvalidUsername},
		{name: "null byte in subject_id", subjectID: "user\x00admin", attrs: domain.ModifyAttrs{Disable: "0"}, wantErr: domain.ErrInvalidUsername},
		{name: "DN traversal in subject_id", subjectID: "uid=admin,ou=employee,o=nycu", attrs: domain.ModifyAttrs{Disable: "0"}, wantErr: domain.ErrInvalidUsername},
		{name: "no attrs at all", subjectID: "0856001", attrs: domain.ModifyAttrs{}, wantErr: domain.ErrNoAttrsToModify},
		{name: "disable not 0 or 1", subjectID: "0856001", attrs: domain.ModifyAttrs{Disable: "true"}, wantErr: domain.ErrInvalidAttrValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewModifyService(&modifyMockRepo{})
			_, err := s.Modify(context.Background(), tt.subjectID, tt.attrs)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestModifyService_HappyPath_PreservesWireSpellingAndOrder(t *testing.T) {
	repo := &modifyMockRepo{}
	s := NewModifyService(repo)

	res, err := s.Modify(context.Background(), "0856001", domain.ModifyAttrs{
		Disable:        "0",
		UserPassword:   "{SSHA}abc",
		AlternateEmail: "user@example.com",
		TempPassword:   "NTLM:deadbeef",
	})
	if err != nil {
		t.Fatalf("Modify() err = %v, want nil", err)
	}
	if res == nil {
		t.Fatal("result is nil")
	}

	// Repository must receive attrs with the legacy wire spelling.
	gotNames := make([]string, 0, len(repo.gotAttrs))
	for _, a := range repo.gotAttrs {
		gotNames = append(gotNames, a.Name)
	}
	wantNames := []string{"disable", "userpassword", "altemate-email", "temppassword"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("repo received attrs in wrong order or spelling: %v, want %v", gotNames, wantNames)
	}

	// Result.Modified must mirror the wire spellings (this is what the
	// consumer's response decoder asserts on).
	if !reflect.DeepEqual(res.Modified, wantNames) {
		t.Fatalf("result.Modified = %v, want %v", res.Modified, wantNames)
	}

	// Specific typo guard.
	for _, n := range res.Modified {
		if n == "alternate-email" {
			t.Errorf("result.Modified contains 'alternate-email'; production schema uses 'altemate-email'")
		}
	}
}

func TestModifyService_HappyPath_PartialAttrs(t *testing.T) {
	repo := &modifyMockRepo{}
	s := NewModifyService(repo)

	res, err := s.Modify(context.Background(), "0856001", domain.ModifyAttrs{
		UserPassword: "{SSHA}only-this",
	})
	if err != nil {
		t.Fatalf("Modify() err = %v, want nil", err)
	}
	if !reflect.DeepEqual(res.Modified, []string{"userpassword"}) {
		t.Fatalf("result.Modified = %v, want [userpassword]", res.Modified)
	}
	if len(repo.gotAttrs) != 1 {
		t.Fatalf("repo received %d attrs, want 1 (do not pad with empty fields)", len(repo.gotAttrs))
	}
}

// TestModifyService_DisableOne_IsAccepted locks in the contract that
// disable="1" is a valid value (per OpenAPI fragment + legacy PHP). An
// implementation that whitelists only "0" would silently reject a legal
// consumer request; this test catches that regression.
func TestModifyService_DisableOne_IsAccepted(t *testing.T) {
	repo := &modifyMockRepo{}
	s := NewModifyService(repo)

	res, err := s.Modify(context.Background(), "0856001", domain.ModifyAttrs{Disable: "1"})
	if err != nil {
		t.Fatalf("Modify(disable=1) err = %v, want nil", err)
	}
	if !reflect.DeepEqual(res.Modified, []string{"disable"}) {
		t.Fatalf("result.Modified = %v, want [disable]", res.Modified)
	}
	if len(repo.gotAttrs) != 1 || repo.gotAttrs[0].Name != "disable" || repo.gotAttrs[0].Value != "1" {
		t.Fatalf("repo got attrs %+v, want one Replace of disable=1", repo.gotAttrs)
	}
}

// ---------------------------------------------------------------------------
// relax-modify-userpassword — RED tests for plaintext userpassword
// (see openspec/changes/relax-modify-userpassword/specs/domain-types/spec.md)
// ---------------------------------------------------------------------------

// TestModifyService_Userpassword_AcceptsPlaintextAndScheme locks in the
// relaxed validation rule: any value whose first byte is not '{' is a
// plaintext password and must be accepted (slapd will hash it with
// password-hash and apply ppolicy). Any value matching `^\{[A-Z0-9]+\}`
// is a scheme pass-through and must also be accepted, forwarded
// verbatim, for admin-reset / migration tooling.
func TestModifyService_Userpassword_AcceptsPlaintextAndScheme(t *testing.T) {
	cases := []struct {
		name      string
		password  string
		wantValue string // what repo should receive
	}{
		{name: "plaintext", password: "Correct-Horse-Battery-Staple", wantValue: "Correct-Horse-Battery-Staple"},
		{name: "plaintext with symbols", password: "p4ssw0rd!@#$%^&*()", wantValue: "p4ssw0rd!@#$%^&*()"},
		{name: "ssha pass-through", password: "{SSHA}abcdef==", wantValue: "{SSHA}abcdef=="},
		{name: "argon2 pass-through", password: "{ARGON2}$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA", wantValue: "{ARGON2}$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"},
		{name: "crypt pass-through", password: "{CRYPT}$6$rounds=5000$abc", wantValue: "{CRYPT}$6$rounds=5000$abc"},
		{name: "plaintext at 256 bytes", password: strings.Repeat("x", 256), wantValue: strings.Repeat("x", 256)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &modifyMockRepo{}
			s := NewModifyService(repo)
			_, err := s.Modify(context.Background(), "0856001", domain.ModifyAttrs{UserPassword: tc.password})
			if err != nil {
				t.Fatalf("Modify() err = %v, want nil", err)
			}
			if len(repo.gotAttrs) != 1 || repo.gotAttrs[0].Name != "userpassword" {
				t.Fatalf("repo got %+v, want one userpassword attr", repo.gotAttrs)
			}
			if repo.gotAttrs[0].Value != tc.wantValue {
				t.Fatalf("repo got value %q, want %q (forward verbatim)", repo.gotAttrs[0].Value, tc.wantValue)
			}
		})
	}
}

// TestModifyService_Userpassword_PlaintextInputGuards locks in the safety
// guards on plaintext (non-{scheme}) values: empty, oversize, NUL byte,
// C0 control characters, DEL. The error MUST NOT echo the value.
func TestModifyService_Userpassword_PlaintextInputGuards(t *testing.T) {
	cases := []struct {
		name     string
		password string
		// substring that MUST NOT appear in the returned error text
		// (caller-supplied content we want to keep out of logs and
		// problem-detail bodies).
		mustNotAppear string
	}{
		{name: "empty plaintext", password: "", mustNotAppear: ""},
		{name: "null byte", password: "abc\x00def", mustNotAppear: "abc"},
		{name: "lf newline", password: "line1\nline2", mustNotAppear: "line1"},
		{name: "cr", password: "abc\rdef", mustNotAppear: "abc"},
		{name: "tab", password: "abc\tdef", mustNotAppear: "abc"},
		{name: "C0 control 0x01", password: "abc\x01def", mustNotAppear: "abc"},
		{name: "DEL 0x7f", password: "abc\x7fdef", mustNotAppear: "abc"},
		{name: "oversize 257 bytes", password: strings.Repeat("x", 257), mustNotAppear: ""},
	}
	for _, tc := range cases {
		// Skip empty-string case shadowing ErrNoAttrsToModify: a single
		// empty userpassword counts as "no attrs", so HasAny is false
		// and the use case returns ErrNoAttrsToModify, not
		// ErrInvalidAttrValue. The spec lists empty as a rejection case
		// for "plaintext input guards" — at the use-case layer that
		// rejection is structurally subsumed by ErrNoAttrsToModify and
		// is still a 400. Treat both as acceptable rejection signals.
		t.Run(tc.name, func(t *testing.T) {
			repo := &modifyMockRepo{}
			s := NewModifyService(repo)
			_, err := s.Modify(context.Background(), "0856001", domain.ModifyAttrs{UserPassword: tc.password})
			if err == nil {
				t.Fatalf("Modify() err = nil, want rejection")
			}
			if !errors.Is(err, domain.ErrInvalidAttrValue) && !errors.Is(err, domain.ErrNoAttrsToModify) {
				t.Fatalf("Modify() err = %v, want ErrInvalidAttrValue or ErrNoAttrsToModify", err)
			}
			if repo.calls != 0 {
				t.Errorf("repo.Modify must NOT be called on validation failure; got %d calls", repo.calls)
			}
			if tc.mustNotAppear != "" && strings.Contains(err.Error(), tc.mustNotAppear) {
				t.Errorf("error message leaks user-supplied bytes: %q contains %q", err.Error(), tc.mustNotAppear)
			}
		})
	}
}

// TestModifyService_Userpassword_NoLongerRequiresSSHAPrefix is the
// inverse of the deleted "userpassword missing SSHA prefix" rejection
// case. With the relaxed contract, a value lacking the {SSHA} prefix
// must not be rejected SOLELY on that basis.
func TestModifyService_Userpassword_NoLongerRequiresSSHAPrefix(t *testing.T) {
	repo := &modifyMockRepo{}
	s := NewModifyService(repo)
	_, err := s.Modify(context.Background(), "0856001", domain.ModifyAttrs{UserPassword: "plaintext"})
	if err != nil {
		t.Fatalf("Modify(plaintext userpassword) err = %v, want nil — {SSHA} requirement is removed", err)
	}
}

func TestModifyService_PropagatesRepoErrors(t *testing.T) {
	for _, sentinel := range []error{
		domain.ErrAccountNotFound,
		domain.ErrSchemaViolation,
		domain.ErrServiceUnavailable,
	} {
		sentinel := sentinel
		t.Run(sentinel.Error(), func(t *testing.T) {
			repo := &modifyMockRepo{modifyFn: func(context.Context, string, []domain.ModifyAttr) error {
				return sentinel
			}}
			s := NewModifyService(repo)
			_, err := s.Modify(context.Background(), "0856001", domain.ModifyAttrs{Disable: "0"})
			if !errors.Is(err, sentinel) {
				t.Fatalf("err = %v, want %v", err, sentinel)
			}
		})
	}
}
