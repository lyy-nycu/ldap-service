## Project context

LDAP microservice in Go providing a controlled access layer for on-prem OpenLDAP.
Deployed to Azure Container Apps via S2S VPN. Full spec in `openspec/specs/ldap-service-spec.md`.

## Your role

You implement code based on interface definitions and acceptance criteria written as comments in the codebase. Follow the criteria exactly — they are the contract.

## Code style

- Go 1.22+, use `net/http.ServeMux` for routing (no third-party router)
- Structured logging: `go.uber.org/zap` (never `log/slog`, never `fmt.Println`)
- Table-driven tests with descriptive names
- Always pass `context.Context` as first parameter
- Error messages: lowercase, no punctuation
- Use `encoding/json` for serialization

## Patterns to follow

### HTTP handler pattern
```go
func handleXxx(uc *usecase.Xxx) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Decode request body
        // 2. Call use case
        // 3. Handle domain errors with handleDomainError()
        // 4. Write success response with writeJSON()
    }
}
```

### Use case pattern
```go
func (uc *Xxx) DoSomething(ctx context.Context, req domain.XxxRequest) (*domain.XxxResponse, error) {
    // 1. Validate input (domain.ValidateUsername, domain.ValidateAttributes)
    // 2. Call repository interface method
    // 3. Return result or domain error
}
```

### LDAP repository pattern
```go
func (r *Repository) Method(ctx context.Context, ...) (..., error) {
    conn, err := r.getConn()
    if err != nil {
        return ..., err
    }
    defer r.putConn(conn)

    filter := fmt.Sprintf("(uid=%s)", ldapv3.EscapeFilter(username))
    // ... search or bind
}
```

### Error response pattern (RFC 7807)
```go
// Use the shared helper in handler/response.go
writeProblem(w, r, domain.ProblemNotFound("account not found"))
```

### Test pattern
```go
func TestXxx(t *testing.T) {
    tests := []struct {
        name    string
        input   domain.XxxRequest
        want    *domain.XxxResponse
        wantErr error
    }{
        {name: "valid request", ...},
        {name: "invalid username", ...},
        {name: "not found", ...},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

## Security rules (never violate)

- LDAP filters: always `ldap.EscapeFilter()`, never string concatenation
- Username: only `[a-zA-Z0-9._-]`, max 64 chars — call `domain.ValidateUsername()`
- API Key: `crypto/subtle.ConstantTimeCompare()` only
- Never log passwords — not even partially, not even hashed
- Auth failures: always generic message "authentication failed"
- Attributes: only return values in `domain.AllowedAttributes`

## Do NOT

- Do not use chi, gin, echo, or any third-party HTTP router
- Do not use `log/slog`, `logrus`, or `fmt.Println` for logging
- Do not hardcode DN prefix patterns — use search-then-bind
- Do not assume LDAP attributes exist — check before access
- Do not add new dependencies without developer approval