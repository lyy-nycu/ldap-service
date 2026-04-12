You are designing a new API endpoint for the LDAP service. Follow these steps IN ORDER. Produce code skeletons only — no implementations.

## Step 1: Confirm with developer

Before producing any code, state:
- Which endpoint you are designing (method + path)
- What it does in one sentence
- Which existing files will be modified vs created

Wait for developer confirmation before proceeding.

## Step 2: Domain layer (`internal/domain/`)

Add to `domain.go`:
- Request/response structs with JSON tags and validation comments
- Any new error variables in the `var` block
- Any new entries to `AllowedAttributes` if needed
- Repository interface method with full acceptance criteria as comments

Acceptance criteria format:
```go
// MethodName does X.
//
// Acceptance criteria:
//   - MUST ...
//   - MUST NOT ...
//   - SHOULD ...
```

If a new RFC 7807 error type is needed, add it to `problem.go`.

## Step 3: Use case layer (`internal/usecase/`)

Create or update the use case file:
- Function signature with acceptance criteria comments
- `panic("not implemented")` body
- Document which domain methods it calls and in what order

## Step 4: Handler layer (`internal/handler/`)

Create the handler file:
- Handler function signature returning `http.HandlerFunc`
- `panic("not implemented")` body
- Document: expected request body, success response, error responses
- Add route registration line to `router.go` (commented out with `// TODO: uncomment after implementation`)

## Step 5: Middleware (if needed)

Only if the endpoint requires new middleware (e.g. rate limiting for authenticate).
- Interface/signature only, no implementation

## Step 6: Test skeleton (`*_test.go`)

Create test file with table-driven structure:
- Test function with `tests` slice
- Each test case: name, input, expected output, expected error
- Cover: happy path, validation failure, not found, LDAP error, rate limit (if applicable)
- `panic("not implemented")` in test body

## Step 7: Handoff summary

Print a summary for the developer:

```
## Handoff to Copilot

Files to implement:
- [ ] internal/infra/ldap/repository.go — MethodName()
- [ ] internal/usecase/xxx.go — FunctionName()
- [ ] internal/handler/xxx.go — handlerFunction()
- [ ] internal/handler/xxx_test.go — TestXxx()

Security checklist for review after implementation:
- [ ] ldap.EscapeFilter() used for all user input in LDAP filters
- [ ] No password in log output
- [ ] Generic error message for auth failures
- [ ] Attribute whitelist enforced
- [ ] context.Context propagated
```