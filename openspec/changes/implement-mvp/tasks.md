## 1. Project Scaffolding

- [ ] 1.1 Initialize Go module (`go mod init`), add all approved dependencies to `go.mod`
- [ ] 1.2 Create `.env.example` with all env vars including `LDAP_INTERNAL_*` and `LDAP_EXTERNAL_*` prefixed vars, with placeholder values and comments
- [ ] 1.3 Create `.gitignore` entries for `.env`, binary output
- [ ] 1.4 Create multi-stage `Dockerfile` — build with `golang:1.22-alpine` (`CGO_ENABLED=0`), runtime on `scratch` with CA certs

## 2. Domain Layer (`internal/domain/`)

- [ ] 2.1 Define `Account` struct with `DN`, `UID`, `Attributes`, `Source` fields in `domain.go`
- [ ] 2.2 Define source constants `SourceInternal = "internal"` and `SourceExternal = "external"` in `domain.go`
- [ ] 2.3 Define `AllowedAttributes` set (map[string]bool) with all 12 whitelisted attributes in `domain.go`
- [ ] 2.4 Implement `ValidateUsername(string) error` with regex `^[a-zA-Z0-9._@-]{1,128}$` in `domain.go` (includes `@` for email-style external usernames)
- [ ] 2.5 Implement `ValidateAttributes([]string) error` that checks each against `AllowedAttributes` in `domain.go`
- [ ] 2.6 Define sentinel errors: `ErrAccountNotFound`, `ErrInvalidUsername`, `ErrAttributeNotAllowed`, `ErrAuthenticationFailed`, `ErrServiceUnavailable`, `ErrRateLimitExceeded` in `domain.go`
- [ ] 2.7 Define `LDAPPool` interface with `Search`, `Bind`, `HealthCheck`, `Close` methods in `domain.go` — represents a single LDAP server's connection pool
- [ ] 2.8 Define `LDAPRepository` interface with `Lookup`, `LookupBatch`, `Authenticate`, `HealthCheck` methods in `domain.go` — represents the fan-out aggregate of both sources
- [ ] 2.9 Define `LookupUseCase` interface with `Lookup`, `LookupBatch` methods in `domain.go`
- [ ] 2.10 Define `AuthenticateUseCase` interface with `Authenticate` method in `domain.go`
- [ ] 2.11 Implement `Problem` struct (RFC 7807) with JSON tags and `Error()` method in `problem.go`
- [ ] 2.12 Implement Problem constructor functions (`NewInvalidRequest`, `NewInvalidUsername`, `NewAttributeNotAllowed`, `NewUnauthorized`, `NewAuthenticationFailed`, `NewNotFound`, `NewServiceUnavailable`, `NewInternalError`, `NewRateLimitExceeded`) in `problem.go`
- [ ] 2.13 Write table-driven unit tests for `ValidateUsername` (internal IDs, emails, injection chars, empty) and `ValidateAttributes` in `domain_test.go`
- [ ] 2.14 Write unit tests for `Problem` JSON serialization in `problem_test.go`

## 3. Config Loader (`internal/infra/config/`)

- [ ] 3.1 Define `LDAPSourceConfig` sub-struct with `Host`, `Port`, `UseTLS`, `BindDN`, `BindPW`, `ConnPoolSize` fields in `config.go`
- [ ] 3.2 Define `Config` struct with `Port`, `LDAPBaseDN`, `Internal` (LDAPSourceConfig), `External` (LDAPSourceConfig), `APIKeys`, `AuthRateLimit`, `AuthRateCleanupMin` in `config.go`
- [ ] 3.3 Implement `Load() (*Config, error)` — read env vars with `LDAP_INTERNAL_*` and `LDAP_EXTERNAL_*` prefixes, apply defaults, validate required fields for both sources
- [ ] 3.4 Implement `API_KEYS` parsing (format `key1:name1,key2:name2`) into `map[string]string`
- [ ] 3.5 Write table-driven unit tests for `Load()` covering: all present, missing required (internal and external), defaults per source, API key parsing in `config_test.go`

## 4. LDAP Pool (`internal/infra/ldap/pool.go`)

- [ ] 4.1 Implement `Pool` struct with buffered channel, source label, TLS support, base DN in `pool.go`
- [ ] 4.2 Implement `NewPool(host, port, tls, bindDN, bindPW, poolSize, baseDN, sourceLabel, logger)` — initialize pool, dial + bind read-only connections
- [ ] 4.3 Implement `getConn()` / `putConn()` — borrow with liveness check, return to pool; overflow connections closed after use
- [ ] 4.4 Implement `Search(ctx, username, attributes)` — LDAP search with `ldap.EscapeFilter()`, set `Account.Source` to pool's source label
- [ ] 4.5 Implement `Bind(ctx, dn, password)` — create new connection (not from pool), attempt bind, close after
- [ ] 4.6 Implement `HealthCheck(ctx)` — borrow conn, perform simple search to verify connectivity
- [ ] 4.7 Implement `Close()` — drain pool, close all connections
- [ ] 4.8 Add compile-time interface check: `var _ domain.LDAPPool = (*Pool)(nil)`

## 5. LDAP Repository — Fan-out (`internal/infra/ldap/repository.go`)

- [ ] 5.1 Implement `Repository` struct holding internal `Pool` + external `Pool` + `zap.Logger` in `repository.go`
- [ ] 5.2 Implement `NewRepository(internalPool, externalPool, logger)` constructor
- [ ] 5.3 Implement `Lookup()` — search internal first, on `ErrAccountNotFound` search external; on internal connection error log and try external; return `ErrServiceUnavailable` if both fail
- [ ] 5.4 Implement `LookupBatch()` — iterate individual fan-out lookups per username, collect accounts and not_found
- [ ] 5.5 Implement `Authenticate()` — search internal then external for DN, bind against the pool that found the user, return `(false, nil)` for all failure cases
- [ ] 5.6 Implement `HealthCheck()` — check both pools, return nil only if both healthy
- [ ] 5.7 Implement `Close()` — close both pools
- [ ] 5.8 Add compile-time interface check: `var _ domain.LDAPRepository = (*Repository)(nil)`

## 6. Middleware (`internal/middleware/`)

- [ ] 6.1 Implement Request ID middleware in `requestid.go` — extract `X-Request-ID` or generate UUID, set in context and response header
- [ ] 6.2 Implement structured logging middleware in `logger.go` — log method, path, status, duration, remote IP, request ID using zap
- [ ] 6.3 Implement API Key middleware in `apikey.go` — `crypto/subtle.ConstantTimeCompare()`, set service name in context, return 401 Problem on failure, log warning with remote IP (not key value)
- [ ] 6.4 Implement per-username rate limit middleware in `ratelimit.go` — `sync.Map` + `rate.Limiter`, extract username from body without consuming it (`io.TeeReader`), return 429 Problem when exceeded
- [ ] 6.5 Implement rate limiter background cleanup goroutine — remove entries unused for `AUTH_RATE_CLEANUP_MIN` minutes
- [ ] 6.6 Write unit tests for Request ID middleware (with and without header) in `requestid_test.go`
- [ ] 6.7 Write unit tests for API Key middleware (valid, missing, invalid key) in `apikey_test.go`
- [ ] 6.8 Write unit tests for rate limit middleware (under limit, exceeded, cleanup) in `ratelimit_test.go`

## 7. Use Cases (`internal/usecase/`)

- [ ] 7.1 Implement `LookupService` struct with `LDAPRepository` dependency in `lookup.go`
- [ ] 7.2 Implement `Lookup()` — validate username (supports both internal IDs and emails), validate attributes, delegate to repository
- [ ] 7.3 Implement `LookupBatch()` — validate batch size (max 50), validate each username, validate attributes, delegate to repository
- [ ] 7.4 Add compile-time interface check: `var _ domain.LookupUseCase = (*LookupService)(nil)`
- [ ] 7.5 Implement `AuthenticateService` struct with `LDAPRepository` + `zap.Logger` dependency in `authenticate.go`
- [ ] 7.6 Implement `Authenticate()` — validate username, check non-empty password, delegate to repository, return generic `ErrAuthenticationFailed` for all failure reasons
- [ ] 7.7 Add compile-time interface check: `var _ domain.AuthenticateUseCase = (*AuthenticateService)(nil)`
- [ ] 7.8 Write table-driven unit tests for `LookupService` with mocked repository (test both internal and external usernames) in `lookup_test.go`
- [ ] 7.9 Write table-driven unit tests for `AuthenticateService` — verify all failure paths return identical error, verify password is not logged, in `authenticate_test.go`

## 8. HTTP Handlers (`internal/handler/`)

- [ ] 8.1 Implement `RespondJSON()` and `RespondProblem()` helpers in `response.go`
- [ ] 8.2 Implement liveness handler (`GET /healthz`) in `health.go`
- [ ] 8.3 Implement readiness handler (`GET /readyz`) with `LDAPRepository.HealthCheck()` (checks both sources) in `health.go`
- [ ] 8.4 Implement lookup handler (`POST /api/v1/ldap/lookup`) — parse JSON, map domain errors to Problems, include `source` field in response, in `lookup.go`
- [ ] 8.5 Implement batch lookup handler (`POST /api/v1/ldap/lookup/batch`) — parse JSON, map domain errors to Problems, each account includes `source` field, in `lookup.go`
- [ ] 8.6 Implement authenticate handler (`POST /api/v1/ldap/authenticate`) — parse JSON (exclude password from logs), map errors to Problems, respond in `authenticate.go`
- [ ] 8.7 Write table-driven unit tests for health handlers in `health_test.go`
- [ ] 8.8 Write table-driven unit tests for lookup handlers (valid, invalid username, not found, disallowed attr) in `lookup_test.go`
- [ ] 8.9 Write table-driven unit tests for authenticate handler (success, failure, invalid body) in `authenticate_test.go`

## 9. Router & Server Entrypoint

- [ ] 9.1 Implement `NewRouter()` in `internal/handler/router.go` — register all routes on `net/http.ServeMux`, apply middleware chain (RequestID → Logger → APIKey for /api/ → RateLimit for authenticate)
- [ ] 9.2 Implement `cmd/server/main.go` — conditional godotenv load, config, logger, two LDAP pools (internal from `Config.Internal`, external from `Config.External`), repository with both pools, use cases, router, HTTP server
- [ ] 9.3 Implement graceful shutdown — listen for SIGINT/SIGTERM, 10-second shutdown timeout, close both LDAP pools via repository, flush logger
- [ ] 9.4 Verify end-to-end: `go build ./cmd/server/` compiles without errors
