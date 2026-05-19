package ldap

import (
	"context"
	"errors"
	"fmt"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/nycuitsc/ldap-service/internal/domain"
)

// Modify atomically replaces the given attributes on the subject
// identified by subjectID on this LDAP server.
//
// See domain.LDAPPool.Modify for the full acceptance criteria.
//
// Implementation requirements (frozen by RED tests in pool_modify_test.go):
//   - MUST resolve subjectID → DN via search with filter
//     "(cn=<EscapeFilter(subjectID)>)", base = p.baseDN, scope =
//     WholeSubtree, requesting only []string{"dn"} (uid not needed).
//   - MUST return domain.ErrAccountNotFound if no entry matches.
//   - MUST construct ONE *ldapv3.ModifyRequest via ldapv3.NewModifyRequest(dn, nil)
//     and call Replace(attr.Name, []string{attr.Value}) for each entry
//     in attrs, then issue exactly ONE conn.Modify(req) call.
//   - MUST use a single borrowed pool connection for both the Search
//     and the Modify (same conn, so the ACL bind stays in scope).
//   - MUST NOT log attr.Value (passwords pass through here).
//   - On *ldapv3.Error: if the upstream result code is constraint /
//     schema / object-class / undefined-attribute / no-such-attribute,
//     return domain.ErrSchemaViolation; otherwise return the raw error
//     so Repository.Modify can decide on ErrServiceUnavailable.
func (p *Pool) Modify(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(attrs) == 0 {
		return domain.ErrNoAttrsToModify
	}

	conn, overflow, err := p.getConn()
	if err != nil {
		return err
	}
	defer p.putConn(conn, overflow)

	// 1. Resolve subjectID → DN. Uses the same filter shape as Search
	// (cn=<escaped>) so the production schema mapping stays consistent.
	filter := fmt.Sprintf("(cn=%s)", ldapv3.EscapeFilter(subjectID))
	searchReq := ldapv3.NewSearchRequest(
		p.baseDN,
		ldapv3.ScopeWholeSubtree,
		ldapv3.NeverDerefAliases,
		1, 0, false,
		filter,
		[]string{"dn"},
		nil,
	)
	result, err := conn.Search(searchReq)
	if err != nil {
		return err
	}
	if len(result.Entries) == 0 {
		return domain.ErrAccountNotFound
	}
	dn := result.Entries[0].DN

	if err := ctx.Err(); err != nil {
		return err
	}

	// 2. Build ONE ModifyRequest with one Replace op per attr, in the
	// same order the caller supplied. Attribute names go through
	// verbatim — in particular "altemate-email" must reach the wire
	// unchanged (see domain/modify.go).
	modReq := ldapv3.NewModifyRequest(dn, nil)
	for _, a := range attrs {
		modReq.Replace(a.Name, []string{a.Value})
	}

	if err := conn.Modify(modReq); err != nil {
		if isSchemaError(err) {
			return domain.ErrSchemaViolation
		}
		return err
	}
	return nil
}

// isSchemaError reports whether an LDAP error is a schema / constraint
// rejection that should surface as a 409 to the API caller rather than
// a 500. The set mirrors the legacy PHP error categorisation.
func isSchemaError(err error) bool {
	var le *ldapv3.Error
	if !errors.As(err, &le) {
		return false
	}
	switch le.ResultCode {
	case ldapv3.LDAPResultConstraintViolation,
		ldapv3.LDAPResultObjectClassViolation,
		ldapv3.LDAPResultUndefinedAttributeType,
		ldapv3.LDAPResultNoSuchAttribute,
		ldapv3.LDAPResultInvalidAttributeSyntax,
		ldapv3.LDAPResultAttributeOrValueExists,
		ldapv3.LDAPResultNamingViolation:
		return true
	}
	return false
}
