package handler

import (
	"encoding/json"
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"github.com/nycuitsc/ldap-service/internal/middleware"
	"go.uber.org/zap"
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
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		requestID := middleware.RequestIDFromContext(r.Context())

		var req authenticateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondProblem(w, domain.NewInvalidRequest("invalid request body", requestID))
			return
		}

		if req.Username == "" {
			RespondProblem(w, domain.NewInvalidRequest("username is required", requestID))
			return
		}
		if req.Password == "" {
			RespondProblem(w, domain.NewInvalidRequest("password is required", requestID))
			return
		}

		authenticated, err := uc.Authenticate(r.Context(), req.Username, req.Password)
		if err == nil && authenticated {
			RespondJSON(w, http.StatusOK, authenticateResponse{Authenticated: true})
			return
		}

		zap.L().Warn("authentication failed", zap.String("username", req.Username), zap.String("remote_ip", r.RemoteAddr), zap.String("request_id", requestID))
		RespondProblem(w, domain.NewAuthenticationFailed(requestID))
	}
}
