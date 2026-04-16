package handler

import (
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// RespondJSON writes a JSON response with the given status code.
//
// Acceptance criteria:
//   - MUST set Content-Type to "application/json"
//   - MUST write the status code before the body
//   - MUST marshal body using encoding/json
//   - If marshal fails, MUST write 500 with a plain error (this should never happen)
func RespondJSON(w http.ResponseWriter, status int, body any) {
	panic("not implemented")
}

// RespondProblem writes an RFC 7807 Problem Details response.
//
// Acceptance criteria:
//   - MUST set Content-Type to "application/problem+json"
//   - MUST use p.Status as the HTTP status code
//   - MUST marshal the Problem struct as JSON body
func RespondProblem(w http.ResponseWriter, p *domain.Problem) {
	panic("not implemented")
}
