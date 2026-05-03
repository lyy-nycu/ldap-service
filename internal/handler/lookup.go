package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"github.com/nycuitsc/ldap-service/internal/middleware"
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
//   - ErrInvalidUsername → domain.NewInvalidUsername
//   - ErrAttributeNotAllowed → domain.NewAttributeNotAllowed
//   - ErrAccountNotFound → domain.NewNotFound
//   - ErrServiceUnavailable → domain.NewServiceUnavailable
//   - other errors → domain.NewInternalError
//   - On success: RespondJSON with 200 and lookupResponse (include source field)
//   - MUST get request ID from middleware.RequestIDFromContext for Problem instance field
func HandleLookup(uc domain.LookupUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		requestID := middleware.RequestIDFromContext(r.Context())

		var req lookupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondProblem(w, domain.NewInvalidRequest("invalid request body", requestID))
			return
		}

		if req.Username == "" {
			RespondProblem(w, domain.NewInvalidRequest("username is required", requestID))
			return
		}
		if len(req.Attributes) == 0 {
			RespondProblem(w, domain.NewInvalidRequest("attributes are required", requestID))
			return
		}

		account, err := uc.Lookup(r.Context(), req.Username, req.Attributes)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrInvalidUsername):
				RespondProblem(w, domain.NewInvalidUsername(err.Error(), requestID))
			case errors.Is(err, domain.ErrAttributeNotAllowed):
				RespondProblem(w, domain.NewAttributeNotAllowed(err.Error(), requestID))
			case errors.Is(err, domain.ErrAccountNotFound):
				RespondProblem(w, domain.NewNotFound(err.Error(), requestID))
			case errors.Is(err, domain.ErrServiceUnavailable):
				RespondProblem(w, domain.NewServiceUnavailable(err.Error(), requestID))
			default:
				RespondProblem(w, domain.NewInternalError("internal server error", requestID))
			}
			return
		}

		RespondJSON(w, http.StatusOK, lookupResponse{
			DN:         account.DN,
			UID:        account.UID,
			Source:     account.Source,
			Attributes: account.Attributes,
		})
	}
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
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		requestID := middleware.RequestIDFromContext(r.Context())

		var req batchLookupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondProblem(w, domain.NewInvalidRequest("invalid request body", requestID))
			return
		}

		if len(req.Usernames) == 0 {
			RespondProblem(w, domain.NewInvalidRequest("usernames are required", requestID))
			return
		}
		if len(req.Attributes) == 0 {
			RespondProblem(w, domain.NewInvalidRequest("attributes are required", requestID))
			return
		}

		accounts, notFound, err := uc.LookupBatch(r.Context(), req.Usernames, req.Attributes)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrBatchSizeExceeded):
				RespondProblem(w, domain.NewInvalidRequest(err.Error(), requestID))
			case errors.Is(err, domain.ErrInvalidUsername):
				RespondProblem(w, domain.NewInvalidUsername(err.Error(), requestID))
			case errors.Is(err, domain.ErrAttributeNotAllowed):
				RespondProblem(w, domain.NewAttributeNotAllowed(err.Error(), requestID))
			case errors.Is(err, domain.ErrAccountNotFound):
				RespondProblem(w, domain.NewNotFound(err.Error(), requestID))
			case errors.Is(err, domain.ErrServiceUnavailable):
				RespondProblem(w, domain.NewServiceUnavailable(err.Error(), requestID))
			default:
				RespondProblem(w, domain.NewInternalError("internal server error", requestID))
			}
			return
		}

		respAccounts := make([]lookupResponse, 0, len(accounts))
		for _, acc := range accounts {
			respAccounts = append(respAccounts, lookupResponse{
				DN:         acc.DN,
				UID:        acc.UID,
				Source:     acc.Source,
				Attributes: acc.Attributes,
			})
		}

		if notFound == nil {
			notFound = []string{}
		}

		RespondJSON(w, http.StatusOK, batchLookupResponse{Accounts: respAccounts, NotFound: notFound})
	}
}
