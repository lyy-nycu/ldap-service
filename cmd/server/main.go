package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/nycuitsc/ldap-service/internal/handler"
	"github.com/nycuitsc/ldap-service/internal/infra/config"
	infraldap "github.com/nycuitsc/ldap-service/internal/infra/ldap"
	"github.com/nycuitsc/ldap-service/internal/middleware"
	"github.com/nycuitsc/ldap-service/internal/usecase"
	"go.uber.org/zap"
)

// main is the server entrypoint.
//
// Acceptance criteria — implement in this order:
//   1. Conditional .env loading: only if .env file exists (os.Stat check)
//   2. Load config via config.Load() — fatal on error
//   3. Initialize zap logger (production config, JSON output)
//   4. Initialize internal LDAP pool from cfg.Internal — fatal on error
//      Parameters: cfg.Internal.Host, cfg.Internal.Port, cfg.Internal.UseTLS,
//      cfg.Internal.BindDN, cfg.Internal.BindPW, cfg.LDAPBaseDN, domain.SourceInternal,
//      cfg.Internal.ConnPoolSize, logger
//   5. Initialize external LDAP pool from cfg.External — fatal on error
//      Same parameters but with cfg.External and domain.SourceExternal
//   6. Create Repository with both pools
//   7. Create use cases: NewLookupService(repo), NewAuthenticateService(repo, logger)
//   8. Create RateLimiter: NewRateLimiter(cfg.AuthRateLimit, time.Duration(cfg.AuthRateCleanupMin)*time.Minute)
//   9. Create router: handler.NewRouter(repo, lookupUC, authUC, cfg.APIKeys, rateLimiter, logger)
//  10. Start HTTP server on cfg.Port
//  11. Log startup message with port and both LDAP hosts
//  12. Wait for SIGINT or SIGTERM
//  13. Graceful shutdown with 10-second timeout:
//      - server.Shutdown(ctx)
//      - rateLimiter.Stop()
//      - repo.Close()
//      - logger.Sync()
func main() {
	panic("not implemented")
}

// Ensure imports are used (remove these when implementing).
var (
	_ = context.Background
	_ = fmt.Sprintf
	_ = http.ListenAndServe
	_ = os.Stat
	_ = signal.Notify
	_ = syscall.SIGTERM
	_ = time.Second
	_ = godotenv.Load
	_ = handler.NewRouter
	_ = config.Load
	_ infraldap.Repository
	_ middleware.RateLimiter
	_ = usecase.NewLookupService
	_ = zap.NewProduction
)
