package domain

import (
	"context"
	"errors"
)

// ---------------------------------------------------------------------------
// Modify — consumer-driven contract from portal-backend PR #170.
//
// Source of truth:
//
//	NYCUITSC/portal-backend openspec/changes/infra-ldap-modify-contract/
//	  openapi-fragment.yaml  (merged into develop on 2026-05-19)
//
// The wire JSON keys are FROZEN and intentionally preserve the legacy NYCU
// production LDAP schema verbatim. In particular the key `altemate-email`
// is misspelled — that is the real attribute name in the directory (see
// backend/api/api.php:3487-3501 in portal-backend). It MUST NOT be silently
// renamed to `alternate-email` by this producer; doing so would break every
// existing entry.
// ---------------------------------------------------------------------------

// ModifyAttrs is the set of attributes a /api/v1/ldap/modify call may
// replace on a single subject. All fields are optional; including only
// a subset means "replace just these". Omitting a field does NOT clear
// the attribute.
//
// JSON tags MUST match the consumer fragment exactly. Do not "fix" the
// typo in AlternateEmail's tag.
type ModifyAttrs struct {
	Disable        string `json:"disable,omitempty"`
	UserPassword   string `json:"userpassword,omitempty"`
	AlternateEmail string `json:"altemate-email,omitempty"`
	TempPassword   string `json:"temppassword,omitempty"`
}

// HasAny reports whether at least one attribute is present.
func (a ModifyAttrs) HasAny() bool {
	return a.Disable != "" || a.UserPassword != "" || a.AlternateEmail != "" || a.TempPassword != ""
}

// ToWireMap returns the attributes that should be sent to the upstream
// LDAP Modify call, keyed by the WIRE attribute name (the same spelling
// the consumer used). Keys are returned in a stable order to make the
// response array deterministic for caller verification.
//
// Order matches the legacy PHP $password_info construction order:
//
//	disable, userpassword, altemate-email, temppassword
func (a ModifyAttrs) ToWireMap() []ModifyAttr {
	out := make([]ModifyAttr, 0, 4)
	if a.Disable != "" {
		out = append(out, ModifyAttr{Name: "disable", Value: a.Disable})
	}
	if a.UserPassword != "" {
		out = append(out, ModifyAttr{Name: "userpassword", Value: a.UserPassword})
	}
	if a.AlternateEmail != "" {
		out = append(out, ModifyAttr{Name: "altemate-email", Value: a.AlternateEmail})
	}
	if a.TempPassword != "" {
		out = append(out, ModifyAttr{Name: "temppassword", Value: a.TempPassword})
	}
	return out
}

// ModifyAttr is a single attribute-name + value pair the repository
// will issue as an LDAP `replace` op.
type ModifyAttr struct {
	Name  string
	Value string
}

// ModifyResult is the application-level outcome of a Modify call. The
// handler maps Modified to the wire response { "modified": [...] }.
type ModifyResult struct {
	Modified []string
}

// Modify errors. These map to RFC 7807 Problem types and to the
// consumer adapter sentinels in portal-backend.
var (
	// ErrSubjectIDRequired → 400 /problems/invalid-request
	ErrSubjectIDRequired = errors.New("subject_id is required")

	// ErrNoAttrsToModify → 400 /problems/invalid-request
	ErrNoAttrsToModify = errors.New("at least one attribute is required")

	// ErrInvalidAttrValue → 400 /problems/invalid-attr-value
	// Returned for: disable not in {"0","1"}, userpassword missing {SSHA}
	// prefix, or any other producer-side value validation failure.
	ErrInvalidAttrValue = errors.New("invalid attribute value")

	// ErrSchemaViolation → 409 /problems/schema-violation
	// Returned when the upstream LDAP server rejects the modify (schema
	// violation, constraint, password-reuse policy, missing objectClass).
	ErrSchemaViolation = errors.New("ldap schema violation")
)

// ModifyUseCase defines the business logic for POST /api/v1/ldap/modify.
// Handlers depend on this interface — never on the concrete service.
type ModifyUseCase interface {
	// Modify validates the request and atomically replaces the given
	// attributes on the subject identified by subjectID.
	//
	// Acceptance criteria:
	//   - MUST return ErrSubjectIDRequired if subjectID is empty
	//   - MUST validate subjectID with ValidateUsername
	//   - MUST return ErrNoAttrsToModify if attrs.HasAny() is false
	//   - MUST return ErrInvalidAttrValue if disable is set and not "0"|"1"
	//   - MUST return ErrInvalidAttrValue if userpassword is set and does
	//     NOT start with "{SSHA}" (producer-side validation per spec)
	//   - MUST call repository.Modify with attrs as-is (no rewriting)
	//   - MUST return ModifyResult.Modified as a stable-ordered list of
	//     the attribute KEYS THAT WERE WRITTEN, using the wire spelling
	//     (e.g. "altemate-email", not "alternate-email")
	Modify(ctx context.Context, subjectID string, attrs ModifyAttrs) (*ModifyResult, error)
}
