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
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// HandleReadyz returns a readiness probe handler.
//
// Acceptance criteria:
//   - MUST call repo.HealthCheck(ctx) which checks BOTH LDAP sources
//   - If healthy: respond with 200 and {"status":"ready"}
//   - If unhealthy: respond with RespondProblem using domain.NewServiceUnavailable
//   - MUST only accept GET method
func HandleReadyz(repo domain.LDAPRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := repo.HealthCheck(r.Context()); err != nil {
			RespondProblem(w, domain.NewServiceUnavailable("ldap service unavailable", ""))
			return
		}

		RespondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
