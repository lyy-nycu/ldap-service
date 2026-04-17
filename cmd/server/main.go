package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/nycuitsc/ldap-service/internal/domain"
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
//  1. Conditional .env loading: only if .env file exists (os.Stat check)
//  2. Load config via config.Load() — fatal on error
//  3. Initialize zap logger (production config, JSON output)
//  4. Initialize internal LDAP pool from cfg.Internal — fatal on error
//     Parameters: cfg.Internal.Host, cfg.Internal.Port, cfg.Internal.UseTLS,
//     cfg.Internal.BindDN, cfg.Internal.BindPW, cfg.LDAPBaseDN, domain.SourceInternal,
//     cfg.Internal.ConnPoolSize, logger
//  5. Initialize external LDAP pool from cfg.External — fatal on error
//     Same parameters but with cfg.External and domain.SourceExternal
//  6. Create Repository with both pools
//  7. Create use cases: NewLookupService(repo), NewAuthenticateService(repo, logger)
//  8. Create RateLimiter: NewRateLimiter(cfg.AuthRateLimit, time.Duration(cfg.AuthRateCleanupMin)*time.Minute)
//  9. Create router: handler.NewRouter(repo, lookupUC, authUC, cfg.APIKeys, rateLimiter, logger)
//  10. Start HTTP server on cfg.Port
//  11. Log startup message with port and both LDAP hosts
//  12. Wait for SIGINT or SIGTERM
//  13. Graceful shutdown with 10-second timeout:
//     - server.Shutdown(ctx)
//     - rateLimiter.Stop()
//     - repo.Close()
//     - logger.Sync()
func main() {
	if _, err := os.Stat(".env"); err == nil {
		_ = godotenv.Load()
	}

	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		_, _ = os.Stderr.WriteString("failed to initialize logger: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()

	internalPool, err := infraldap.NewPool(
		cfg.Internal.Host,
		cfg.Internal.Port,
		cfg.Internal.UseTLS,
		cfg.Internal.BindDN,
		cfg.Internal.BindPW,
		cfg.LDAPBaseDN,
		domain.SourceInternal,
		cfg.Internal.ConnPoolSize,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to initialize internal ldap pool", zap.Error(err))
	}

	externalPool, err := infraldap.NewPool(
		cfg.External.Host,
		cfg.External.Port,
		cfg.External.UseTLS,
		cfg.External.BindDN,
		cfg.External.BindPW,
		cfg.LDAPBaseDN,
		domain.SourceExternal,
		cfg.External.ConnPoolSize,
		logger,
	)
	if err != nil {
		_ = internalPool.Close()
		logger.Fatal("failed to initialize external ldap pool", zap.Error(err))
	}

	repo := infraldap.NewRepository(internalPool, externalPool, logger)
	lookupUC := usecase.NewLookupService(repo)
	authUC := usecase.NewAuthenticateService(repo, logger)
	rateLimiter := middleware.NewRateLimiter(cfg.AuthRateLimit, time.Duration(cfg.AuthRateCleanupMin)*time.Minute)

	router := handler.NewRouter(repo, lookupUC, authUC, cfg.APIKeys, rateLimiter, logger)
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		logger.Info("starting ldap service",
			zap.String("port", cfg.Port),
			zap.String("internal_ldap_host", cfg.Internal.Host),
			zap.String("external_ldap_host", cfg.External.Host),
		)

		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Fatal("http server failed", zap.Error(serveErr))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}
	rateLimiter.Stop()
	if err := repo.Close(); err != nil {
		logger.Error("repository close failed", zap.Error(err))
	}
	_ = logger.Sync()
}
