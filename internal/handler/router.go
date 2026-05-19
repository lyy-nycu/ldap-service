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
//   - GET /healthz      → HandleHealthz()         — NO middleware (except RequestID + Logger)
//   - GET /readyz       → HandleReadyz(repo)      — NO middleware (except RequestID + Logger)
//   - POST /api/v1/ldap/lookup        → HandleLookup(lookupUC)        — APIKey middleware
//   - POST /api/v1/ldap/lookup/batch  → HandleBatchLookup(lookupUC)   — APIKey middleware
//   - POST /api/v1/ldap/authenticate  → HandleAuthenticate(authUC)    — APIKey + RateLimit middleware
//   - POST /api/v1/ldap/modify        → HandleModify(modifyUC)        — APIKey middleware
//   - Middleware chain order: RequestID → Logger → (APIKey for /api/) → (RateLimit for authenticate) → Handler
//   - Health endpoints MUST NOT require API key
//   - Rate limiting MUST only apply to the authenticate endpoint
func NewRouter(
	repo domain.LDAPRepository,
	lookupUC domain.LookupUseCase,
	authUC domain.AuthenticateUseCase,
	modifyUC domain.ModifyUseCase,
	keys map[string]string,
	rateLimiter *middleware.RateLimiter,
	logger *zap.Logger,
) http.Handler {
	mux := http.NewServeMux()

	healthz := http.Handler(HandleHealthz())
	healthz = middleware.Logger(logger)(healthz)
	healthz = middleware.RequestID(healthz)
	mux.Handle("GET /healthz", healthz)

	readyz := http.Handler(HandleReadyz(repo))
	readyz = middleware.Logger(logger)(readyz)
	readyz = middleware.RequestID(readyz)
	mux.Handle("GET /readyz", readyz)

	lookup := http.Handler(HandleLookup(lookupUC))
	lookup = middleware.APIKey(keys)(lookup)
	lookup = middleware.Logger(logger)(lookup)
	lookup = middleware.RequestID(lookup)
	mux.Handle("POST /api/v1/ldap/lookup", lookup)

	batchLookup := http.Handler(HandleBatchLookup(lookupUC))
	batchLookup = middleware.APIKey(keys)(batchLookup)
	batchLookup = middleware.Logger(logger)(batchLookup)
	batchLookup = middleware.RequestID(batchLookup)
	mux.Handle("POST /api/v1/ldap/lookup/batch", batchLookup)

	authenticate := http.Handler(HandleAuthenticate(authUC))
	if rateLimiter != nil {
		authenticate = rateLimiter.Middleware(authenticate)
	}
	authenticate = middleware.APIKey(keys)(authenticate)
	authenticate = middleware.Logger(logger)(authenticate)
	authenticate = middleware.RequestID(authenticate)
	mux.Handle("POST /api/v1/ldap/authenticate", authenticate)

	modify := http.Handler(HandleModify(modifyUC))
	modify = middleware.APIKey(keys)(modify)
	modify = middleware.Logger(logger)(modify)
	modify = middleware.RequestID(modify)
	mux.Handle("POST /api/v1/ldap/modify", modify)

	return mux
}
