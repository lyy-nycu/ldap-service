package handler

import (
	"encoding/json"
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
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// RespondProblem writes an RFC 7807 Problem Details response.
//
// Acceptance criteria:
//   - MUST set Content-Type to "application/problem+json"
//   - MUST use p.Status as the HTTP status code
//   - MUST marshal the Problem struct as JSON body
func RespondProblem(w http.ResponseWriter, p *domain.Problem) {
	if p == nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(p)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_, _ = w.Write(data)
}
