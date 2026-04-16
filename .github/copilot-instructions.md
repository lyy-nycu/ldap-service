## Project context

LDAP microservice in Go providing a controlled access layer for two independent on-prem OpenLDAP servers.
Deployed to Azure Container Apps via S2S VPN. Full spec in `openspec/specs/ldap-service-spec.md`.
Design and tasks in `openspec/changes/implement-mvp/`.

### Dual LDAP source architecture

| | Internal LDAP | External LDAP |
|---|---|---|
| **OUs** | `student`, `employee`, `retire` | `cooperator`, `alumni` |
| **Usernames** | Student/employee numbers (`110550001`, `T1234`) | Email addresses (`user@example.com`) |
| **Credentials** | Own read-only bind DN + password | Own read-only bind DN + password |
| **Base DN** | `o=nycu` | `o=nycu` |

No username overlap between servers. Fan-out strategy: search internal first, if not found search external.

## Your role

You implement code based on interface definitions and acceptance criteria written as comments in the codebase. Follow the criteria exactly — they are the contract.

Reference `openspec/changes/implement-mvp/tasks.md` for the implementation checklist.

## Code style

- Go 1.22+, use `net/http.ServeMux` for routing (no third-party router)
- Structured logging: `go.uber.org/zap` (never `log/slog`, never `fmt.Println`)
- Table-driven tests with descriptive names
- Always pass `context.Context` as first parameter
- Error messages: lowercase, no punctuation
- Use `encoding/json` for serialization

## Architecture layers

```
internal/
  domain/        ← entities, interfaces, validation, errors (zero dependencies)
  usecase/       ← business logic, depends only on domain interfaces
  handler/       ← HTTP handlers, depends on domain interfaces (NOT usecase package)
  middleware/    ← HTTP middleware chain
  infra/
    config/      ← env var loading
    ldap/
      pool.go        ← Pool: single-server connection pool (implements domain.LDAPPool)
      repository.go  ← Repository: fan-out across two Pools (implements domain.LDAPRepository)
```

**Dependency rule**: handlers and use cases depend on `domain` interfaces only. Never import `usecase` or `infra` packages from `handler`.

## Key interfaces

```go
// domain.LDAPPool — one per LDAP server
type LDAPPool interface {
    Search(ctx context.Context, username string, attributes []string) (*Account, error)
    Bind(ctx context.Context, dn string, password string) error
    HealthCheck(ctx context.Context) error
    Close() error
}

// domain.LDAPRepository — fan-out aggregate of both pools
type LDAPRepository interface {
    Lookup(ctx context.Context, username string, attributes []string) (*Account, error)
    LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*Account, []string, error)
    Authenticate(ctx context.Context, username string, password string) (bool, error)
    HealthCheck(ctx context.Context) error
}
```

## Patterns to follow

### Pool pattern (single LDAP server — `infra/ldap/pool.go`)

```go
func (p *Pool) Search(ctx context.Context, username string, attributes []string) (*Account, error) {
    conn, err := p.getConn()
    if err != nil {
        return nil, err
    }
    defer p.putConn(conn)

    filter := fmt.Sprintf("(uid=%s)", ldapv3.EscapeFilter(username))
    // search with base=p.baseDN, scope=WholeSubtree
    // set Account.Source = p.source (domain.SourceInternal or domain.SourceExternal)
}
```

### Repository fan-out pattern (`infra/ldap/repository.go`)

```go
func (r *Repository) Lookup(ctx context.Context, username string, attributes []string) (*Account, error) {
    // 1. Try internal pool
    account, err := r.internal.Search(ctx, username, attributes)
    if err == nil {
        return account, nil
    }
    // 2. If not found, try external pool
    if errors.Is(err, domain.ErrAccountNotFound) {
        return r.external.Search(ctx, username, attributes)
    }
    // 3. If internal had a connection error, log it and still try external
    r.logger.Warn("internal ldap search failed, trying external", zap.Error(err))
    account, extErr := r.external.Search(ctx, username, attributes)
    if extErr != nil {
        return nil, domain.ErrServiceUnavailable
    }
    return account, nil
}
```

### HTTP handler pattern

```go
func handleLookup(uc domain.LookupUseCase) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Decode request body
        // 2. Call use case (uc.Lookup)
        // 3. On domain error: RespondProblem(w, mapError(err, requestID))
        // 4. On success: RespondJSON(w, http.StatusOK, response)
    }
}
```

### Use case pattern

```go
func (s *LookupService) Lookup(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
    // 1. domain.ValidateUsername(username)
    // 2. domain.ValidateAttributes(attributes)
    // 3. s.repo.Lookup(ctx, username, attributes)  — fan-out is transparent
}
```

### Middleware pattern

```go
func APIKey(keys map[string]string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // validate X-Api-Key with crypto/subtle.ConstantTimeCompare
            // on success: set service name in context, call next.ServeHTTP
            // on failure: RespondProblem(w, domain.NewUnauthorized(...))
        })
    }
}
```

Middleware chain order: `RequestID → Logger → APIKey (for /api/) → RateLimit (for authenticate only) → Handler`

### Error response pattern (RFC 7807)

```go
// handler/response.go — shared helpers
RespondJSON(w, http.StatusOK, body)
RespondProblem(w, domain.NewNotFound("account not found", requestID))
RespondProblem(w, domain.NewInvalidUsername("username must match ...", requestID))
RespondProblem(w, domain.NewAuthenticationFailed(requestID))
```

### Test pattern

```go
func TestLookup(t *testing.T) {
    tests := []struct {
        name       string
        username   string
        attributes []string
        mockResult *domain.Account
        mockErr    error
        wantErr    error
    }{
        // Valid cases
        {name: "valid internal user", username: "110550001", ...},
        {name: "valid external user (email)", username: "alumni@example.com", ...},

        // Attack vectors — MUST include these categories for any user input
        {name: "LDAP injection parentheses", username: "user)(uid=*)", wantErr: domain.ErrInvalidUsername},
        {name: "LDAP injection OR filter", username: "user)(|(uid=*)", wantErr: domain.ErrInvalidUsername},
        {name: "null byte injection", username: "user\x00admin", wantErr: domain.ErrInvalidUsername},
        {name: "DN traversal", username: "uid=admin,ou=employee,o=nycu", wantErr: domain.ErrInvalidUsername},

        // Domain errors
        {name: "not found in either source", wantErr: domain.ErrAccountNotFound},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

**Test coverage requirements**: Every test file MUST include attack vector cases for any user input field. Categories to cover:
- **LDAP injection**: `()`, `*`, `|`, `&`, `!`, `\` in filter syntax
- **Encoding attacks**: null bytes `\x00`, tabs `\t`, newlines `\n`, carriage returns `\r`
- **Special characters**: `<>`, `'`, `"`, `#`, `+`, `=`, `,`
- **Sensitive attributes**: `userPassword`, `objectClass`, `*` wildcard
- **Boundary**: max length, max length + 1, empty string
```

### Context keys pattern

```go
// middleware/requestid.go
type ctxKeyRequestID struct{}
func RequestIDFromContext(ctx context.Context) string { ... }

// middleware/apikey.go
type ctxKeyServiceName struct{}
func ServiceNameFromContext(ctx context.Context) string { ... }
```

Use unexported struct types as context keys to avoid collisions.

### Rate limit body reading pattern

```go
// middleware/ratelimit.go — read username without consuming body
var buf bytes.Buffer
tee := io.TeeReader(r.Body, &buf)
// decode username from tee
// restore body: r.Body = io.NopCloser(&buf)
```

## Config — env var naming

Dual-source config with `LDAP_INTERNAL_*` and `LDAP_EXTERNAL_*` prefixes:

```
LDAP_BASE_DN                    # shared — "o=nycu"
LDAP_INTERNAL_HOST              # internal LDAP server host
LDAP_INTERNAL_PORT              # default: 636
LDAP_INTERNAL_USE_TLS           # default: true
LDAP_INTERNAL_BIND_DN           # internal read-only bind DN
LDAP_INTERNAL_BIND_PW           # internal read-only bind password
LDAP_INTERNAL_CONN_POOL_SIZE    # default: 10
LDAP_EXTERNAL_HOST              # external LDAP server host
LDAP_EXTERNAL_PORT              # default: 636
LDAP_EXTERNAL_USE_TLS           # default: true
LDAP_EXTERNAL_BIND_DN           # external read-only bind DN
LDAP_EXTERNAL_BIND_PW           # external read-only bind password
LDAP_EXTERNAL_CONN_POOL_SIZE    # default: 5
API_KEYS                        # format: key1:name1,key2:name2
```

## Account struct

```go
type Account struct {
    DN         string            `json:"dn"`
    UID        string            `json:"uid"`
    Attributes map[string]string `json:"attributes"`
    Source     string            `json:"source"` // domain.SourceInternal or domain.SourceExternal
}
```

Always include `source` in lookup responses so callers know which LDAP server the account came from.

## Security rules (never violate)

- **LDAP injection**: always `ldap.EscapeFilter()`, never string concatenation for filters
- **Username validation**: `^[a-zA-Z0-9._@-]{1,128}$` — call `domain.ValidateUsername()`. Includes `@` for email-style external usernames
- **API Key comparison**: `crypto/subtle.ConstantTimeCompare()` only — never `==`
- **Never log passwords** — not even partially, not even hashed, not at debug level
- **Auth failure responses**: always generic `"authentication failed"` — never reveal whether the user exists or which source was checked
- **Attribute whitelist**: only return values in `domain.AllowedAttributes` — reject requests for anything else
- **Never assume LDAP attributes exist** — check before access (historical outage from this)

## Do NOT

- Do not use chi, gin, echo, or any third-party HTTP router
- Do not use `log/slog`, `logrus`, or `fmt.Println` for logging
- Do not hardcode DN prefix patterns — use search-then-bind
- Do not add new dependencies without developer approval
- Do not import `usecase` or `infra` packages from `handler` — depend on domain interfaces
- Do not put fan-out logic in `Pool` — fan-out belongs in `Repository`
- Do not reuse pooled connections for user bind — create a new connection, close after bind
- Do not log the API key value on auth failure — log remote IP only

## Test file rules

Some test files are **fully written by the supervisor** (complete assertions, no `panic`). Others are **skeletons** (test cases defined, assertions are `panic("not implemented")`).

- **Full tests (DO NOT MODIFY)**: If a `_test.go` file has no `panic("not implemented")`, it is a locked specification. Implement the production code to make these tests pass. Do NOT change the test file.
- **Skeleton tests (implement assertions)**: If a `_test.go` file has `panic("not implemented")` in test bodies, fill in the assertion logic following the acceptance criteria in the comments.

## Before finishing any task

Run these checks and fix any issues before marking a task done:

```bash
go build ./...
go vet ./...
go test ./... -count=1
```

If you created a new file, verify:
- It has the correct package name
- It does not import packages that violate the dependency rule
- Exported types have GoDoc comments

If you implemented an interface, verify:
- Compile-time check exists: `var _ domain.InterfaceName = (*StructName)(nil)`
- All methods match the interface signature exactly
