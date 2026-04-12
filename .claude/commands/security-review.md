You are performing a security review on the LDAP service codebase. Check every file under `internal/` against the following checklist. Report findings as PASS, FAIL, or WARN.

## A01 — Broken Access Control

- [ ] All API endpoints (except /healthz, /readyz) require X-Api-Key header
- [ ] API Key validation uses `crypto/subtle.ConstantTimeCompare()`
- [ ] Attribute queries only return attributes in `domain.AllowedAttributes`
- [ ] No endpoint exposes LDAP write operations (MVP is read-only + authenticate)

## A02 — Cryptographic Failures

- [ ] LDAP connections use TLS (`ldap.DialTLS` with `tls.Config{MinVersion: tls.VersionTLS12}`)
- [ ] Password field never appears in any log statement — search ALL `zap.` and `logger.` calls
- [ ] API Key values never appear in log output
- [ ] LDAP bind passwords loaded from environment variables, not hardcoded

## A03 — Injection

- [ ] Every LDAP filter uses `ldap.EscapeFilter()` for user-provided values
- [ ] No string concatenation or `fmt.Sprintf` used to build LDAP filters WITHOUT EscapeFilter
- [ ] Username input validated against regex `^[a-zA-Z0-9._-]{1,64}$` before any LDAP operation
- [ ] Attribute names validated against whitelist before any LDAP search

## A04 — Insecure Design

- [ ] Search-then-bind pattern used for authentication (not direct bind with assumed DN)
- [ ] LDAP connection re-binds to read-only account after user bind in authenticate flow
- [ ] Connection pool properly handles stale connections (IsClosing check)
- [ ] No LDAP modify/delete operations exposed in MVP

## A07 — Identification and Authentication Failures

- [ ] Authenticate endpoint returns identical response for "user not found" and "wrong password"
- [ ] No log message distinguishes between "user not found" and "wrong password"
- [ ] Per-username rate limiting applied to authenticate endpoint
- [ ] Rate limiter cleanup goroutine prevents memory leak from accumulated entries

## A09 — Security Logging and Monitoring Failures

- [ ] Every request logged with: request_id, method, path, status, duration, remote_addr
- [ ] Authentication attempts logged with: username (never password)
- [ ] Authentication failures logged at WARN level
- [ ] Invalid API Key attempts logged with remote_addr at WARN level
- [ ] Rate limit hits logged with username at WARN level

## A05 — Security Misconfiguration

- [ ] Health check endpoints (/healthz, /readyz) do NOT require API Key
- [ ] /readyz actually tests LDAP connectivity, not just returns 200
- [ ] .env file is in .gitignore
- [ ] Dockerfile uses `scratch` base image (minimal attack surface)
- [ ] No default/sample API Keys in committed code

## RFC 7807 Compliance

- [ ] All error responses use Content-Type `application/problem+json`
- [ ] All error responses include: type, title, status fields
- [ ] Error `instance` field contains request ID for traceability
- [ ] Error type URIs are consistent with `openspec/spec.md` section 3.1

## Output format

```
## Security Review Results

### FAIL (must fix before merge)
- [A03] internal/infra/ldap/repository.go:42 — LDAP filter built with fmt.Sprintf without EscapeFilter
- ...

### WARN (should fix)
- [A09] internal/middleware/logger.go — missing duration_ms field in request log
- ...

### PASS
- [A01] API Key middleware correctly applied to all /api/v1/* routes
- ...
```