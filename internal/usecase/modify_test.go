package usecase

import (
	"context"
	"errors"
	"reflect"
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
func (m *modifyMockRepo) Authenticate(context.Context, string, string) (bool, error) {
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
		{name: "userpassword missing SSHA prefix", subjectID: "0856001", attrs: domain.ModifyAttrs{UserPassword: "plaintext"}, wantErr: domain.ErrInvalidAttrValue},
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
