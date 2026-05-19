package ldap

import (
	"context"

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
	panic("not implemented")
}
