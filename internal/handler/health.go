package handler

import (
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// HandleHealthz returns a liveness probe handler.
//
// Acceptance criteria:
//   - MUST respond with 200 and {"status":"ok"}
//   - MUST NOT check LDAP connectivity
//   - MUST only accept GET method
func HandleHealthz() http.HandlerFunc {
	panic("not implemented")
}

// HandleReadyz returns a readiness probe handler.
//
// Acceptance criteria:
//   - MUST call repo.HealthCheck(ctx) which checks BOTH LDAP sources
//   - If healthy: respond with 200 and {"status":"ready"}
//   - If unhealthy: respond with RespondProblem using domain.NewServiceUnavailable
//   - MUST only accept GET method
func HandleReadyz(repo domain.LDAPRepository) http.HandlerFunc {
	panic("not implemented")
}
