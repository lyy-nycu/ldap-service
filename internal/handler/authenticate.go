package handler

import (
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// authenticateRequest is the JSON request body for POST /api/v1/ldap/authenticate.
type authenticateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// authenticateResponse is the JSON response body for successful authentication.
type authenticateResponse struct {
	Authenticated bool `json:"authenticated"`
}

// HandleAuthenticate returns a handler for POST /api/v1/ldap/authenticate.
//
// Acceptance criteria:
//   - MUST only accept POST method (405 for others)
//   - MUST decode JSON body into authenticateRequest
//   - If JSON decode fails: RespondProblem with domain.NewInvalidRequest
//   - If username is empty: RespondProblem with domain.NewInvalidRequest
//   - If password is empty: RespondProblem with domain.NewInvalidRequest
//   - MUST call uc.Authenticate(ctx, username, password)
//   - If (true, nil): RespondJSON with 200 and {"authenticated": true}
//   - If (false, ErrAuthenticationFailed): RespondProblem with domain.NewAuthenticationFailed
//   - MUST NOT log the password — not even partially
//   - MUST get request ID from middleware.RequestIDFromContext for Problem instance field
func HandleAuthenticate(uc domain.AuthenticateUseCase) http.HandlerFunc {
	panic("not implemented")
}
