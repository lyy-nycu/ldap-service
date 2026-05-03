# server-entrypoint Specification

## Purpose
TBD - created by archiving change implement-mvp. Update Purpose after archive.
## Requirements
### Requirement: Router setup
The system SHALL register all routes on a `net/http.ServeMux` in `internal/handler/router.go`:
- `GET /healthz` — liveness (no middleware)
- `GET /readyz` — readiness (no middleware)
- `POST /api/v1/ldap/lookup` — single lookup (API Key middleware)
- `POST /api/v1/ldap/lookup/batch` — batch lookup (API Key middleware)
- `POST /api/v1/ldap/authenticate` — authenticate (API Key + Rate Limit middleware)

Middleware chain order: RequestID → Logger → (APIKey for /api/ routes) → (RateLimit for authenticate) → Handler.

#### Scenario: Health endpoints bypass auth
- **WHEN** `GET /healthz` is called without `X-Api-Key`
- **THEN** the request SHALL succeed with 200

#### Scenario: API endpoints require auth
- **WHEN** `POST /api/v1/ldap/lookup` is called without `X-Api-Key`
- **THEN** the request SHALL be rejected with 401

### Requirement: Main entrypoint
The system SHALL implement `cmd/server/main.go` with:
1. Conditional `.env` loading: only if `.env` file exists
2. Config loading via `infra/config.Load()`
3. Zap logger initialization
4. **Two LDAP pool initializations**: internal pool from `Config.Internal`, external pool from `Config.External` — both using shared `Config.LDAPBaseDN`
5. Repository initialization with both pools
6. Use case initialization
7. Router + middleware setup
8. HTTP server start on configured port
9. Graceful shutdown on SIGINT/SIGTERM with 10-second timeout

#### Scenario: Successful startup
- **WHEN** all config is valid and both LDAP servers are reachable
- **THEN** the server SHALL start and log the listening port, including both LDAP source hosts

#### Scenario: Config validation failure
- **WHEN** a required env var is missing (e.g., `LDAP_EXTERNAL_HOST`)
- **THEN** the process SHALL exit with a fatal log message naming the missing variable

#### Scenario: One LDAP source unreachable at startup
- **WHEN** the internal LDAP server is reachable but the external is not
- **THEN** the process SHALL exit with a fatal log message indicating which source failed

#### Scenario: Graceful shutdown
- **WHEN** SIGTERM is received
- **THEN** the server SHALL stop accepting new connections, wait up to 10 seconds for in-flight requests, close both LDAP pools, and exit cleanly

### Requirement: Dockerfile
The system SHALL provide a multi-stage Dockerfile:
- Build stage: `golang:1.22-alpine`, `CGO_ENABLED=0`, static binary
- Runtime stage: `scratch` base, copy CA certificates and binary
- The `.env` file MUST NOT be included in the image

#### Scenario: Docker build produces minimal image
- **WHEN** `docker build` is run
- **THEN** the resulting image SHALL use `scratch` base with only the binary and CA certs

### Requirement: Environment example file
The system SHALL provide a `.env.example` file listing all environment variables (including `LDAP_INTERNAL_*` and `LDAP_EXTERNAL_*` prefixed vars) with placeholder values and comments explaining each source.

#### Scenario: Developer onboarding
- **WHEN** a developer clones the repo
- **THEN** they SHALL be able to copy `.env.example` to `.env` and fill in values for both LDAP sources to run locally

