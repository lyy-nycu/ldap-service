# middleware-stack Specification

## Purpose
TBD - created by archiving change implement-mvp. Update Purpose after archive.
## Requirements
### Requirement: Request ID middleware
The system SHALL inject a request ID into every request's context. If the caller provides `X-Request-ID` header, that value SHALL be used. Otherwise, the system SHALL generate a UUID v4 using `google/uuid`. The request ID SHALL be set on the response `X-Request-ID` header.

#### Scenario: Caller provides request ID
- **WHEN** a request includes `X-Request-ID: abc-123`
- **THEN** the context SHALL carry `"abc-123"` and the response SHALL include `X-Request-ID: abc-123`

#### Scenario: No request ID provided
- **WHEN** a request has no `X-Request-ID` header
- **THEN** the system SHALL generate a UUID and set it in both context and response header

### Requirement: Structured logging middleware
The system SHALL log every request using `go.uber.org/zap` with JSON output. Each log entry SHALL include: method, path, status code, duration, remote IP, and request ID. The log entry SHALL be written after the response is sent.

#### Scenario: Successful request logged
- **WHEN** a `POST /api/v1/ldap/lookup` returns 200 in 15ms
- **THEN** a JSON log line SHALL be emitted with `"method":"POST"`, `"path":"/api/v1/ldap/lookup"`, `"status":200`, `"duration_ms":15`, and the request ID

### Requirement: API Key validation middleware
The system SHALL validate the `X-Api-Key` header on all `/api/` routes. Comparison MUST use `crypto/subtle.ConstantTimeCompare()`. If the key matches, the associated service name SHALL be added to the request context. The middleware SHALL NOT apply to health check endpoints (`/healthz`, `/readyz`).

#### Scenario: Valid API key
- **WHEN** `X-Api-Key` matches a configured key
- **THEN** the request SHALL proceed and the service name SHALL be available in context

#### Scenario: Missing API key
- **WHEN** `X-Api-Key` header is absent
- **THEN** the system SHALL return 401 with RFC 7807 Problem Details type `/problems/unauthorized`

#### Scenario: Invalid API key
- **WHEN** `X-Api-Key` does not match any configured key
- **THEN** the system SHALL return 401 with RFC 7807 Problem Details, and log a warning with remote IP (MUST NOT log the key value)

### Requirement: Per-username rate limiting middleware
The system SHALL enforce rate limiting on `POST /api/v1/ldap/authenticate` only. Each unique username SHALL have its own `rate.Limiter` (Token Bucket: rate=AUTH_RATE_LIMIT/60s, burst=AUTH_RATE_LIMIT). The middleware MUST extract the username from the request body without consuming it (use `io.TeeReader` or buffer).

#### Scenario: Under rate limit
- **WHEN** username `"110550001"` has made 3 attempts in the last minute (limit is 5)
- **THEN** the request SHALL proceed normally

#### Scenario: Rate limit exceeded
- **WHEN** username `"110550001"` has made 5 attempts in the last minute
- **THEN** the system SHALL return 429 with RFC 7807 Problem Details type `/problems/rate-limit-exceeded`

### Requirement: Rate limiter cleanup
The system SHALL run a background goroutine that cleans up rate limiter entries unused for longer than `AUTH_RATE_CLEANUP_MIN` minutes. The cleanup interval SHALL equal `AUTH_RATE_CLEANUP_MIN`.

#### Scenario: Stale limiter removed
- **WHEN** username `"olduser"` has not made any authenticate attempts for 10+ minutes (default cleanup interval)
- **THEN** the limiter entry for `"olduser"` SHALL be removed from memory

