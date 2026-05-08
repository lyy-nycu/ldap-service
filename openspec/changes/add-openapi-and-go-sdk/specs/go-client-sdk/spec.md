## ADDED Requirements

### Requirement: Package location and module path

The SDK SHALL live at `pkg/ldapclient/` within the `ldap-service` repository. Its import path SHALL be `github.com/<org>/ldap-service/pkg/ldapclient`. The package SHALL NOT import anything under `internal/`.

#### Scenario: SDK directory exists and compiles

- **WHEN** a developer runs `go build ./pkg/ldapclient/...` from the repo root
- **THEN** the build succeeds with zero errors

#### Scenario: SDK does not import internal packages

- **WHEN** the import graph of `pkg/ldapclient` is inspected (`go list -deps ./pkg/ldapclient/...`)
- **THEN** no dependency path includes `<module>/internal/`

### Requirement: Public Client type with required-arg constructor

The package SHALL export a `Client` type whose constructor signature is `New(baseURL string, apiKey string, opts ...Option) (*Client, error)`. `New` SHALL return a non-nil error when `baseURL` is empty, when it does not parse as an HTTPS or HTTP URL, or when `apiKey` is empty.

#### Scenario: Constructor rejects empty inputs

- **WHEN** a caller invokes `ldapclient.New("", "k")` or `ldapclient.New("https://x", "")`
- **THEN** the returned error is non-nil
- **AND** the returned `*Client` is nil

#### Scenario: Constructor accepts a valid URL and key

- **WHEN** a caller invokes `ldapclient.New("https://ldap.example.com", "abc123")`
- **THEN** the returned error is nil
- **AND** the returned `*Client` is non-nil

### Requirement: One method per operation, idiomatic Go names

The `Client` SHALL expose one method per spec operation: `Healthz`, `Readyz`, `Lookup`, `BatchLookup`, `Authenticate`. Every method SHALL accept `context.Context` as its first parameter, return a typed response struct (or a typed pointer) and an `error`. No method SHALL return `*http.Response` to the caller.

#### Scenario: Lookup signature

- **WHEN** the public API of `pkg/ldapclient` is inspected
- **THEN** `Client.Lookup` exists with signature `func (c *Client) Lookup(ctx context.Context, req LookupRequest) (*Account, error)` (or equivalent typed shape)
- **AND** no exported method has `*http.Response` in its return values

### Requirement: Functional options for optional configuration

The package SHALL export at minimum these options: `WithHTTPClient(*http.Client) Option`, `WithUserAgent(string) Option`, `WithTimeout(time.Duration) Option`. Options SHALL be applied in argument order; later options override earlier ones for the same field.

#### Scenario: WithHTTPClient overrides the default client

- **WHEN** a caller passes `WithHTTPClient(myCustomClient)` to `New`
- **THEN** every subsequent request issued by the `Client` uses `myCustomClient`

#### Scenario: WithTimeout sets a per-request deadline

- **WHEN** a caller passes `WithTimeout(2 * time.Second)`
- **AND** the server takes longer than 2 seconds to respond
- **THEN** the call returns a timeout error wrapping `context.DeadlineExceeded`

### Requirement: API key handling

The `Client` SHALL include the configured API key in the `X-API-Key` header on every outbound request. The SDK MUST NOT log the API key value at any log level. The SDK MUST NOT read the API key from environment variables or files; it accepts the key only through `New`.

#### Scenario: Every request carries the X-API-Key header

- **WHEN** a `Client` constructed with `apiKey="abc123"` issues any request via any method
- **THEN** the outgoing `http.Request.Header.Get("X-API-Key")` equals `"abc123"`

#### Scenario: SDK does not log the key

- **WHEN** the SDK source is searched for any log call referencing the configured key
- **THEN** no such call exists in `pkg/ldapclient/*.go`

### Requirement: Sentinel errors and Problem propagation

The package SHALL export at minimum these sentinel errors: `ErrBadRequest`, `ErrUnauthorized`, `ErrAccountNotFound`, `ErrTooManyRequests`, `ErrInternal`. For every documented non-2xx response, the SDK SHALL return an error that satisfies `errors.Is(err, <relevant sentinel>)` AND `errors.As(err, &pe)` where `pe` is `*ProblemError` exposing the parsed RFC 7807 fields (`Type`, `Title`, `Status`, `Detail`, `Instance`, `RequestID`).

#### Scenario: 401 response is identifiable via errors.Is

- **WHEN** the server responds `401 application/problem+json` with a valid Problem body
- **THEN** the returned error satisfies `errors.Is(err, ldapclient.ErrUnauthorized)`
- **AND** `errors.As(err, &pe)` populates `pe.RequestID` from the response body

#### Scenario: 429 response is identifiable

- **WHEN** the server responds `429` from `Authenticate`
- **THEN** `errors.Is(err, ldapclient.ErrTooManyRequests)` returns true

#### Scenario: Network failure is wrapped

- **WHEN** the underlying `http.Client.Do` returns an error (e.g. DNS failure)
- **THEN** the returned error is non-nil
- **AND** `errors.Is(err, <relevant sentinel>)` returns false (transport errors do not masquerade as protocol sentinels)
- **AND** `errors.Unwrap(err)` returns the original transport error

### Requirement: Generated code is committed and regenerable

Generated code SHALL live in `pkg/ldapclient/generated.go` (or similarly-named files) and SHALL carry a `// Code generated` header. A `//go:generate` directive at the top of `pkg/ldapclient/client.go` SHALL invoke `oapi-codegen` with the committed `tools/oapi-codegen-config.yaml`. CI SHALL fail if `git diff --exit-code pkg/ldapclient/` is non-empty after `go generate ./pkg/ldapclient/...`.

#### Scenario: go generate produces no diff against committed files

- **WHEN** CI runs `go generate ./pkg/ldapclient/... && git diff --exit-code pkg/ldapclient/`
- **THEN** both commands exit 0

#### Scenario: Generated header is present

- **WHEN** any `*.go` file in `pkg/ldapclient/` was emitted by `oapi-codegen`
- **THEN** its first comment line begins with `// Code generated`

### Requirement: SDK runtime dependencies are minimal

`pkg/ldapclient` SHALL depend at runtime only on the Go standard library plus what `oapi-codegen` requires for its emitted code (typically `github.com/oapi-codegen/runtime`). It MUST NOT pull in HTTP routers, logging libraries, or any package from the parent service's stack (e.g. `go.uber.org/zap`, `go-ldap`).

#### Scenario: Dependency graph is constrained

- **WHEN** `go list -deps -f '{{.Module.Path}}' ./pkg/ldapclient/... | sort -u` is run
- **THEN** the only non-stdlib modules listed are `github.com/<org>/ldap-service` itself and (optionally) `github.com/oapi-codegen/runtime`

### Requirement: SDK is tested against an httptest server

`pkg/ldapclient/client_test.go` SHALL contain table-driven tests for each public method that:

- Stand up an `httptest.Server` returning canned responses (happy path + each documented error status).
- Construct a `Client` pointed at the test server.
- Assert the response decodes into the typed return value, and that errors map to the correct sentinel.

Tests SHALL run as part of `go test ./pkg/ldapclient/...` on every PR (no build tag).

#### Scenario: Lookup happy path test

- **WHEN** a test stands up an httptest.Server returning `200 application/json` with a valid `Account` body
- **AND** calls `client.Lookup(ctx, req)`
- **THEN** the test asserts the returned `*Account` has the expected fields populated and the error is nil

#### Scenario: Authenticate 401 test

- **WHEN** a test stands up an httptest.Server returning `401 application/problem+json` with a valid Problem body
- **AND** calls `client.Authenticate(ctx, req)`
- **THEN** the test asserts `errors.Is(err, ldapclient.ErrUnauthorized)` is true
- **AND** `errors.As(err, &pe)` exposes the request_id from the body

### Requirement: Package documentation and runnable examples

`pkg/ldapclient/doc.go` SHALL contain a package-level doc comment summarizing usage, the URL of the canonical OpenAPI spec, the API-key handling contract, and at least one runnable `Example` per major method (`ExampleClient_Lookup`, `ExampleClient_Authenticate`).

#### Scenario: go doc shows usage

- **WHEN** a developer runs `go doc github.com/<org>/ldap-service/pkg/ldapclient`
- **THEN** the output includes a usage summary referencing `New` and `WithHTTPClient`

#### Scenario: Examples compile and run

- **WHEN** CI runs `go test -run Example ./pkg/ldapclient/...`
- **THEN** the command exits 0
- **AND** at least one `Example` is associated with `Client.Lookup` and one with `Client.Authenticate`
