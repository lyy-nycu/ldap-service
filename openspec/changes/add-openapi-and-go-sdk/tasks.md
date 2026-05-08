## 1. OpenAPI specification

- [ ] 1.1 Create `api/` directory and add `api/openapi.yaml` with `openapi: 3.1.0`
- [ ] 1.2 Author `info` block: `title="LDAP Service API"`, `version="0.1.0"`, `description`, `contact.email`
- [ ] 1.3 Author `servers` array with one `https://` placeholder + description
- [ ] 1.4 Define `components.securitySchemes.apiKey` (type=apiKey, in=header, name=X-API-Key)
- [ ] 1.5 Define `components.schemas.Account` mirroring `internal/domain/domain.go` `Account` struct (every JSON-tagged field)
- [ ] 1.6 Define `components.schemas.Problem` mirroring `internal/domain/problem.go` (`type`, `title`, `status`, `detail`, `instance`, `request_id`)
- [ ] 1.7 Define request schemas: `LookupRequest`, `BatchLookupRequest`, `AuthenticateRequest` (match handler decode targets)
- [ ] 1.8 Define `BatchLookupResponse` (and any other response wrapper types the handlers actually return)
- [ ] 1.9 Author operation `healthz` (`GET /healthz`) — `security: []`, response `200`
- [ ] 1.10 Author operation `readyz` (`GET /readyz`) — `security: []`, responses `200`, `503`
- [ ] 1.11 Author operation `lookup` (`POST /api/v1/ldap/lookup`) — `security: [{apiKey: []}]`, responses `200`, `400`, `401`, `404`, `500`
- [ ] 1.12 Author operation `batchLookup` (`POST /api/v1/ldap/lookup/batch`) — same security, responses `200`, `400`, `401`, `413` (batch overflow), `500`
- [ ] 1.13 Author operation `authenticate` (`POST /api/v1/ldap/authenticate`) — same security, responses `200`, `400`, `401`, `429`, `500`
- [ ] 1.14 Add `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After` headers on `authenticate`'s `429`
- [ ] 1.15 Add a `CHANGELOG.md` (or section in README) documenting the `0.1.0` initial release and the pre-1.0 breaking-change policy

## 2. Codegen toolchain

- [ ] 2.1 Add `tools/tools.go` with a build tag of `//go:build tools` and a blank import of `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`
- [ ] 2.2 Pin the codegen version in `go.mod` (run `go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen` or via `tools.go`); commit `go.sum`
- [ ] 2.3 Create `tools/oapi-codegen-config.yaml` configuring: `package: ldapclient`, `output: pkg/ldapclient/generated.go`, generate types + client, no server stubs
- [ ] 2.4 Verify `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config tools/oapi-codegen-config.yaml api/openapi.yaml` produces a clean file

## 3. Hand-authored SDK surface

- [ ] 3.1 Create `pkg/ldapclient/client.go` with:
  - `//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config ../../tools/oapi-codegen-config.yaml ../../api/openapi.yaml`
  - `type Client struct { ... }` wrapping the generated raw client
  - `func New(baseURL, apiKey string, opts ...Option) (*Client, error)` validating both args
  - Public methods `Healthz`, `Readyz`, `Lookup`, `BatchLookup`, `Authenticate` — each `(ctx, req) → (*TypedResp, error)`
  - Internal helper that injects `X-API-Key` on every request
- [ ] 3.2 Create `pkg/ldapclient/options.go` with `Option` type and `WithHTTPClient`, `WithUserAgent`, `WithTimeout`
- [ ] 3.3 Create `pkg/ldapclient/errors.go` with:
  - Sentinels: `ErrBadRequest`, `ErrUnauthorized`, `ErrAccountNotFound`, `ErrTooManyRequests`, `ErrInternal`
  - `type ProblemError struct { Type, Title, Detail, Instance, RequestID string; Status int }`
  - `(*ProblemError) Error() string`, `Unwrap() error`, `Is(target error) bool` mapping `Status` → sentinel
  - Internal helper `parseProblem(resp *http.Response) error` that reads `application/problem+json` and returns the wrapped error
- [ ] 3.4 Create `pkg/ldapclient/doc.go` with package-level doc, OpenAPI URL reference, API-key handling note, and runnable `ExampleClient_Lookup` + `ExampleClient_Authenticate`
- [ ] 3.5 Run `go generate ./pkg/ldapclient/...` and commit `pkg/ldapclient/generated.go`

## 4. SDK tests

- [ ] 4.1 Create `pkg/ldapclient/client_test.go` (no build tag) with table-driven tests:
  - Constructor rejects empty `baseURL` and empty `apiKey`
  - Each method's happy path against `httptest.Server` returning canned 2xx body
  - Each method's documented error status returns the matching sentinel via `errors.Is`
  - `errors.As` exposes `*ProblemError` with `RequestID` populated from response body
  - `WithHTTPClient` injects a custom client (assert via test transport)
  - `WithTimeout` produces a `context.DeadlineExceeded`-wrapping error on slow server
  - `X-API-Key` header is present on every outgoing request
  - SDK never logs the API key (assert via grep over `pkg/ldapclient/*.go`)
- [ ] 4.2 Create `pkg/ldapclient/contract_test.go` with `//go:build contract` that:
  - Boots `cmd/server` against an in-memory LDAP fake (reuse infra from `openspec/specs/integration-test-env` if available; otherwise stub)
  - Calls every SDK method against the live server
  - Asserts decoded responses are non-zero for happy paths and that errors carry the documented sentinel for at least one error path per method

## 5. AGENTS.md and README

- [ ] 5.1 Create `AGENTS.md` at repo root with sections:
  - "What this service does" (3 lines)
  - "Authentication" (`X-API-Key` header, no other schemes)
  - "Contract" (link to `api/openapi.yaml`, note that it is the source of truth)
  - "Calling examples": (a) curl, (b) Go via `pkg/ldapclient`, (c) Go raw `net/http` for non-Go clients reading by example
  - "Errors" (link to `Problem` schema and to `pkg/ldapclient/errors.go` sentinel list)
  - "Versioning" (semver policy from spec, current version is `0.1.0`)
- [ ] 5.2 Update `README.md`:
  - Add a top-of-file note: "API contract: see [`api/openapi.yaml`](api/openapi.yaml). Go SDK: [`pkg/ldapclient`](pkg/ldapclient)."
  - Add a "Calling this service" section with a 5-line Go SDK example and the `go get github.com/<org>/ldap-service/pkg/ldapclient` install command

## 6. CI integration (extends `.github/workflows/ci.yml` from `add-aca-deploy-pipeline`)

- [ ] 6.1 Add a `openapi-lint` job: install Spectral, run `spectral lint api/openapi.yaml --ruleset spectral:oas --fail-severity error`
- [ ] 6.2 Add a `sdk-stale` job: run `go generate ./pkg/ldapclient/...` then `git diff --exit-code pkg/ldapclient/`
- [ ] 6.3 Add a `contract` job (path-filtered to PRs touching `api/`, `internal/handler/`, or `pkg/ldapclient/`): `go test -tags=contract ./pkg/ldapclient/...`
- [ ] 6.4 Update branch protection to require `openapi-lint`, `sdk-stale`, and `contract` (when triggered) before merge

## 7. Verification

- [ ] 7.1 `spectral lint api/openapi.yaml` exits 0 locally
- [ ] 7.2 `go build ./pkg/ldapclient/...` succeeds with zero errors
- [ ] 7.3 `go test ./pkg/ldapclient/...` passes (non-contract tests)
- [ ] 7.4 `go test -tags=contract ./pkg/ldapclient/...` passes against a locally booted server
- [ ] 7.5 `go list -deps ./pkg/ldapclient/... | grep -E '<module>/internal'` returns no matches
- [ ] 7.6 `go doc github.com/<org>/ldap-service/pkg/ldapclient` shows a useful summary
- [ ] 7.7 Sanity check: `grep -rE '(zap|logger|fmt\.Print).*[aA]pi[_-]?[kK]ey' pkg/ldapclient/` returns no matches (key never logged)
- [ ] 7.8 Manual check: a fresh consumer (e.g. a scratch `go.mod` in `/tmp`) can `go get github.com/<org>/ldap-service/pkg/ldapclient` and call `Lookup` against a running server
