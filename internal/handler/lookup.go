package handler

import (
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// lookupRequest is the JSON request body for POST /api/v1/ldap/lookup.
type lookupRequest struct {
	Username   string   `json:"username"`
	Attributes []string `json:"attributes"`
}

// lookupResponse is the JSON response body for a successful lookup.
type lookupResponse struct {
	DN         string            `json:"dn"`
	UID        string            `json:"uid"`
	Source     string            `json:"source"`
	Attributes map[string]string `json:"attributes"`
}

// batchLookupRequest is the JSON request body for POST /api/v1/ldap/lookup/batch.
type batchLookupRequest struct {
	Usernames  []string `json:"usernames"`
	Attributes []string `json:"attributes"`
}

// batchLookupResponse is the JSON response body for a successful batch lookup.
type batchLookupResponse struct {
	Accounts []lookupResponse `json:"accounts"`
	NotFound []string         `json:"not_found"`
}

// HandleLookup returns a handler for POST /api/v1/ldap/lookup.
//
// Acceptance criteria:
//   - MUST only accept POST method (405 for others)
//   - MUST decode JSON body into lookupRequest
//   - If JSON decode fails: RespondProblem with domain.NewInvalidRequest
//   - If username is empty: RespondProblem with domain.NewInvalidRequest
//   - If attributes is empty: RespondProblem with domain.NewInvalidRequest
//   - MUST call uc.Lookup(ctx, username, attributes)
//   - Map domain errors to Problems:
//     - ErrInvalidUsername → domain.NewInvalidUsername
//     - ErrAttributeNotAllowed → domain.NewAttributeNotAllowed
//     - ErrAccountNotFound → domain.NewNotFound
//     - ErrServiceUnavailable → domain.NewServiceUnavailable
//     - other errors → domain.NewInternalError
//   - On success: RespondJSON with 200 and lookupResponse (include source field)
//   - MUST get request ID from middleware.RequestIDFromContext for Problem instance field
func HandleLookup(uc domain.LookupUseCase) http.HandlerFunc {
	panic("not implemented")
}

// HandleBatchLookup returns a handler for POST /api/v1/ldap/lookup/batch.
//
// Acceptance criteria:
//   - MUST only accept POST method (405 for others)
//   - MUST decode JSON body into batchLookupRequest
//   - If JSON decode fails: RespondProblem with domain.NewInvalidRequest
//   - If usernames is empty: RespondProblem with domain.NewInvalidRequest
//   - If attributes is empty: RespondProblem with domain.NewInvalidRequest
//   - MUST call uc.LookupBatch(ctx, usernames, attributes)
//   - On success: RespondJSON with 200 and batchLookupResponse
//   - Each account in response MUST include source field
//   - not_found MUST be an empty array (not null) when all users are found
func HandleBatchLookup(uc domain.LookupUseCase) http.HandlerFunc {
	panic("not implemented")
}
