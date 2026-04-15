## Context

The PHP 8.3 monolith at portal.nycu.edu.tw directly queries on-prem OpenLDAP for login, user lookup, and attribute retrieval. This coupling prevents other services (MFA Service) from accessing LDAP independently. The LDAP service is a Go microservice that acts as the single controlled access layer, deployed on Azure Container Apps and connected to on-prem OpenLDAP via S2S VPN.

The university operates **two independent OpenLDAP servers**:

| | Internal LDAP | External LDAP |
|---|---|---|
| **OUs** | `student`, `employee`, `retire` | `cooperator`, `alumni` |
| **Username pattern** | Student/employee numbers (e.g., `110550001`, `T1234`) | Email addresses (e.g., `user@example.com`) |
| **Bind account** | Own read-only credentials | Own read-only credentials |
| **Base DN** | `o=nycu` | `o=nycu` |

Usernames do not overlap between servers. The service fans out: search internal first, if not found search external.

No Go code exists yet. This is a greenfield implementation following Clean Architecture with four layers: domain, usecase, handler, and infra.

## Goals / Non-Goals

**Goals:**
- Implement all MVP endpoints: lookup, batch lookup, authenticate, health checks
- Support two independent LDAP sources with transparent fan-out routing
- Enforce security constraints: LDAP injection prevention, API Key auth, rate limiting, no password leakage
- Provide per-source LDAP connection pooling with liveness checks over VPN
- Produce structured JSON logs with request tracing
- Deliver a Docker image deployable to Azure Container Apps

**Non-Goals:**
- LDAP write operations (phase 2)
- Scope-based API Key permissions (phase 2)
- Redis-based distributed rate limiting (phase 2)
- CI/CD pipeline setup (separate task)
- Local OpenLDAP test container setup (separate task)

## Decisions

### 1. Clean Architecture layer ordering
**Decision**: Build bottom-up: domain → infra/config → infra/ldap → middleware → usecase → handler → router → main.
**Rationale**: Each layer depends only on layers below it. Domain has zero dependencies, making it the natural starting point. Infra implements domain interfaces.

### 2. Dual-source LDAP with fan-out routing
**Decision**: Create a `Pool` struct (single-server connection pool) and a `Repository` struct that holds two `Pool` instances (internal + external). The `Repository` implements `domain.LDAPRepository` and contains the fan-out logic: try internal pool first, on `ErrAccountNotFound` try external pool.
**Rationale**: The two LDAP servers are independent (different hosts, credentials) but serve the same base DN with no username overlap. Fan-out is simple and correct. The `Pool` is reusable infrastructure; `Repository` is the orchestration layer.
**Alternative considered**: Single pool that connects to both — rejected because the servers have independent credentials and may have different availability characteristics.

### 3. LDAP connection pool — custom implementation
**Decision**: Build a simple channel-based connection pool (`Pool` struct) rather than using a third-party pool library. Each LDAP source gets its own `Pool` instance with independent size, credentials, and health state.
**Rationale**: The pool requirements are straightforward (fixed size, liveness check, re-bind after user auth). A buffered channel of `*ldap.Conn` with health check on borrow keeps it simple. No external dependency needed.
**Alternative considered**: `go-ldap` connection pool — doesn't exist as a standalone package.

### 4. Username validation — support email-based external usernames
**Decision**: Expand username regex from `^[a-zA-Z0-9._-]{1,64}$` to `^[a-zA-Z0-9._@-]{1,128}$` to accommodate email-style usernames used by external LDAP users (cooperators, alumni).
**Rationale**: Internal users have numeric/alphanumeric IDs, external users use email addresses as usernames. The `@` character must be allowed. Max length increased to 128 for email addresses. LDAP injection is still prevented by `ldap.EscapeFilter()` on all filter inputs.
**Alternative considered**: Separate validation rules per source — rejected because callers don't know which source a user belongs to.

### 5. Rate limiter — sync.Map + x/time/rate
**Decision**: Store per-username `rate.Limiter` instances in a `sync.Map` with background cleanup goroutine.
**Rationale**: In-memory is sufficient for MVP (1-3 replicas). Token bucket with rate=5/60s, burst=5 matches the spec. Background goroutine cleans stale entries every 10 minutes to prevent memory growth.
**Alternative considered**: Redis — overkill for MVP replica count.

### 6. Middleware chain — function composition
**Decision**: Use `func(http.Handler) http.Handler` middleware pattern with manual composition in router setup.
**Rationale**: Standard Go idiom, no framework needed. Chain order: RequestID → Logger → APIKey → (RateLimit on auth only) → Handler.

### 7. RFC 7807 Problem Details — shared helper
**Decision**: Define a `Problem` struct in `domain/problem.go` with builder functions. A shared `RespondProblem()` helper in `handler/response.go` serializes and writes the response.
**Rationale**: Centralizes error formatting. All handlers produce consistent error responses.

### 8. Configuration — dual-source env vars
**Decision**: Config struct holds two `LDAPSourceConfig` sub-structs (internal and external), each with its own host, port, TLS, bind DN, bind password, and pool size. Env vars use `LDAP_INTERNAL_` and `LDAP_EXTERNAL_` prefixes. Shared settings (`LDAP_BASE_DN`) remain unprefixed.
**Rationale**: Fail fast on missing required vars. Clear separation between sources. Azure Container Apps injects env vars directly.

## Risks / Trade-offs

- **[VPN silent disconnection]** → Either LDAP server's connections can go stale silently. Mitigation: check liveness (simple search) when borrowing from each pool; reconnect if dead.
- **[One LDAP source down, other healthy]** → Fan-out will fail on the unhealthy source but succeed on the healthy one. Mitigation: if internal lookup fails with a connection error (not "not found"), log the error and still try external. Only return `ErrServiceUnavailable` if both sources are unreachable.
- **[Fan-out latency]** → Sequential internal→external search adds latency for external users. Mitigation: acceptable for MVP. Internal users (majority) are found on first try. Future optimization: parallel fan-out or username-pattern-based routing.
- **[In-memory rate limit not shared across replicas]** → Worst case allows 5 × replica count attempts. Mitigation: acceptable for MVP (max 3 replicas = 15 attempts/min). Phase 2 upgrades to Redis.
- **[Attribute removal causing outages]** → Historical incident. Mitigation: never assume attributes exist; check before using. Attribute whitelist enforced at domain layer.
- **[No integration tests in MVP tasks]** → Unit tests with mocked interfaces only. Mitigation: local OpenLDAP container setup is a separate follow-up task.
