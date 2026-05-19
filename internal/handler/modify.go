package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"github.com/nycuitsc/ldap-service/internal/middleware"
)

// modifyRequest is the JSON request body for POST /api/v1/ldap/modify.
//
// Field names and tags MUST match the consumer-driven contract verbatim,
// including the intentional "altemate-email" typo on ModifyAttrs.
// See:
//
//	NYCUITSC/portal-backend openspec/changes/infra-ldap-modify-contract/
//	  openapi-fragment.yaml
//	NYCUITSC/portal-backend backend-go/internal/adapter/ldap/modify_test.go
type modifyRequest struct {
	SubjectID string             `json:"subject_id"`
	Attrs     domain.ModifyAttrs `json:"attrs"`
}

// modifyResponse is the JSON response body for a successful modify.
// `modified` lists the attribute names actually written to LDAP, in
// the wire spelling (e.g. "altemate-email").
type modifyResponse struct {
	Modified []string `json:"modified"`
}

// HandleModify returns a handler for POST /api/v1/ldap/modify.
//
// Acceptance criteria (frozen by RED tests in modify_test.go):
//   - MUST only accept POST (405 for others)
//   - MUST decode JSON body into modifyRequest
//   - On JSON decode fail: 400 /problems/invalid-request
//   - If subject_id is empty: 400 /problems/invalid-request
//   - If attrs has no fields set: 400 /problems/invalid-request
//   - MUST call uc.Modify(ctx, req.SubjectID, req.Attrs)
//   - Error mapping:
//   - ErrSubjectIDRequired      → 400 /problems/invalid-request
//   - ErrNoAttrsToModify        → 400 /problems/invalid-request
//   - ErrInvalidUsername        → 400 /problems/invalid-username
//   - ErrInvalidAttrValue       → 400 /problems/invalid-attr-value
//   - ErrAccountNotFound        → 404 /problems/not-found
//   - ErrSchemaViolation        → 409 /problems/schema-violation
//   - ErrRateLimitExceeded      → 429 /problems/rate-limit-exceeded
//   - ErrServiceUnavailable     → 500 /problems/internal-error  (per
//     contract: "Upstream LDAP unreachable or returned a server error"
//     is a 500, not a 503)
//   - default                   → 500 /problems/internal-error
//   - On success: 200 + {"modified": [...]} with the wire attribute
//     spellings preserved (e.g. "altemate-email", not "alternate-email")
//   - Content-Type "application/json" on success, "application/problem+json"
//     on error
//   - MUST get request ID from middleware.RequestIDFromContext for Problem instance
func HandleModify(uc domain.ModifyUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		requestID := middleware.RequestIDFromContext(r.Context())

		var req modifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondProblem(w, domain.NewInvalidRequest("invalid request body", requestID))
			return
		}

		if req.SubjectID == "" {
			RespondProblem(w, domain.NewInvalidRequest("subject_id is required", requestID))
			return
		}
		if !req.Attrs.HasAny() {
			RespondProblem(w, domain.NewInvalidRequest("at least one attribute is required", requestID))
			return
		}

		result, err := uc.Modify(r.Context(), req.SubjectID, req.Attrs)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrSubjectIDRequired),
				errors.Is(err, domain.ErrNoAttrsToModify):
				RespondProblem(w, domain.NewInvalidRequest(err.Error(), requestID))
			case errors.Is(err, domain.ErrInvalidUsername):
				RespondProblem(w, domain.NewInvalidUsername(err.Error(), requestID))
			case errors.Is(err, domain.ErrInvalidAttrValue):
				RespondProblem(w, domain.NewInvalidAttrValue(err.Error(), requestID))
			case errors.Is(err, domain.ErrAccountNotFound):
				RespondProblem(w, domain.NewNotFound(err.Error(), requestID))
			case errors.Is(err, domain.ErrSchemaViolation):
				RespondProblem(w, domain.NewSchemaViolation(err.Error(), requestID))
			case errors.Is(err, domain.ErrRateLimitExceeded):
				RespondProblem(w, domain.NewRateLimitExceeded(requestID))
			case errors.Is(err, domain.ErrServiceUnavailable):
				// Per contract: 500 (not 503). The OpenAPI fragment
				// declares 500 as "Upstream LDAP unreachable or
				// returned a server error"; 503 is not listed for
				// /ldap/modify.
				RespondProblem(w, domain.NewInternalError("upstream ldap unreachable", requestID))
			default:
				RespondProblem(w, domain.NewInternalError("internal server error", requestID))
			}
			return
		}

		modified := result.Modified
		if modified == nil {
			modified = []string{}
		}
		RespondJSON(w, http.StatusOK, modifyResponse{Modified: modified})
	}
}
