## Why

The LDAP service needs to be built from scratch as a Go microservice to replace LDAP access logic currently embedded in the PHP 8.3 monolith (portal.nycu.edu.tw). This is the first phase of a Strangler Fig migration. Without this service, all LDAP operations remain tightly coupled to the monolith, blocking the MFA Service and future services from independent LDAP access.

The university operates two independent on-prem OpenLDAP servers: one for internal users (students, employees, retirees) and one for external users (cooperators, alumni). Both share the same base DN (`o=nycu`) but have separate hosts and credentials. The service must transparently fan out across both sources.

## What Changes

- Implement Go project scaffolding: `go.mod`, `main.go` with graceful shutdown, conditional `.env` loading
- Implement domain layer: entities (`Account`), interfaces (`LDAPRepository`, `LookupUseCase`, `AuthenticateUseCase`), attribute whitelist, input validation, error types, RFC 7807 Problem Details
- Implement infrastructure layer: environment variable config loader, two LDAP connection pools (internal + external) with health check and re-bind, LDAP repository with fan-out logic (search internal first, fallback to external)
- Implement middleware stack: API Key validation (constant-time compare), per-username rate limiting (Token Bucket), request ID injection, structured zap logging
- Implement use cases: single lookup, batch lookup, authenticate (search-then-bind)
- Implement HTTP handlers: lookup, batch lookup, authenticate, health checks (`/healthz`, `/readyz`)
- Implement router: `net/http.ServeMux` with method-based routing
- Add Dockerfile (multi-stage, scratch runtime) and `.env.example`

## Capabilities

### New Capabilities
- `domain-types`: Domain entities, interfaces, attribute whitelist, validation rules, error definitions, RFC 7807 Problem Details
- `config-loader`: Environment variable loading with defaults and validation
- `ldap-repository`: LDAP connection pool management and search/bind operations
- `middleware-stack`: API Key auth, per-username rate limiting, request ID, structured logging
- `lookup-usecase`: Single and batch account attribute lookup logic
- `authenticate-usecase`: Search-then-bind authentication with security constraints
- `http-handlers`: HTTP handlers for all API endpoints including health checks
- `server-entrypoint`: Main entrypoint, router setup, graceful shutdown, Dockerfile

### Modified Capabilities
<!-- No existing capabilities to modify — this is a greenfield implementation -->

## Impact

- **New files**: ~20 Go source files across `cmd/`, `internal/` directory tree
- **New dependencies**: `go-ldap/ldap/v3`, `uber-go/zap`, `google/uuid`, `joho/godotenv`, `x/time/rate`
- **APIs exposed**: 5 HTTP endpoints (`/healthz`, `/readyz`, `/api/v1/ldap/lookup`, `/api/v1/ldap/lookup/batch`, `/api/v1/ldap/authenticate`)
- **Deployment artifact**: Docker image targeting Azure Container Apps (internal ingress only)
- **External systems**: Connects to on-prem OpenLDAP via Azure S2S VPN
