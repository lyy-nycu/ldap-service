package handler

import (
	"net/http"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"github.com/nycuitsc/ldap-service/internal/middleware"
	"go.uber.org/zap"
)

// NewRouter creates and configures the HTTP router with all routes and middleware.
//
// Acceptance criteria:
//   - MUST use net/http.ServeMux (no third-party router)
//   - Route registration:
//     - GET /healthz      → HandleHealthz()         — NO middleware (except RequestID + Logger)
//     - GET /readyz       → HandleReadyz(repo)      — NO middleware (except RequestID + Logger)
//     - POST /api/v1/ldap/lookup        → HandleLookup(lookupUC)        — APIKey middleware
//     - POST /api/v1/ldap/lookup/batch  → HandleBatchLookup(lookupUC)   — APIKey middleware
//     - POST /api/v1/ldap/authenticate  → HandleAuthenticate(authUC)    — APIKey + RateLimit middleware
//   - Middleware chain order: RequestID → Logger → (APIKey for /api/) → (RateLimit for authenticate) → Handler
//   - Health endpoints MUST NOT require API key
//   - Rate limiting MUST only apply to the authenticate endpoint
func NewRouter(
	repo domain.LDAPRepository,
	lookupUC domain.LookupUseCase,
	authUC domain.AuthenticateUseCase,
	keys map[string]string,
	rateLimiter *middleware.RateLimiter,
	logger *zap.Logger,
) http.Handler {
	panic("not implemented")
}
