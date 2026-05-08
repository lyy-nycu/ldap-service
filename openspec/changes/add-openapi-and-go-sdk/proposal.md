## Why

`ldap-service` exposes five HTTP endpoints (`/healthz`, `/readyz`, `/api/v1/ldap/lookup`, `/api/v1/ldap/lookup/batch`, `/api/v1/ldap/authenticate`) but ships **no machine-readable contract** and **no client library**. Today every caller — the PHP monolith, the upcoming `mfa-service`, future internal tools, and any AI agent helping a developer integrate — must read Go handler source or guess from `README.md` examples. As the strangler-fig migration progresses and the consumer count grows, this drift will surface as production incidents (wrong field names, missed error codes, ignored rate limits).

A shared, machine-readable contract — OpenAPI 3.1 — solves both problems at once:

- **For agents** (Claude, Copilot, Cursor, future tools): a single URL/file they can fetch and reason over. No per-agent skill files to maintain.
- **For human developers**: a generated, typed Go SDK eliminates hand-rolled HTTP plumbing, enforces auth header use, and surfaces all error types as Go errors.

Authoring per-agent skill packs is rejected as the primary integration vector: it scales N (consumers) × M (agent flavors) and ages out as new agents appear. OpenAPI scales O(1).

## What Changes

- Add **`api/openapi.yaml`** — OpenAPI 3.1 spec covering all current endpoints, request/response schemas (matching `internal/domain` types), the `X-API-Key` security scheme, the RFC 7807 problem+json error response, and rate-limit headers.
- Add a **generated Go SDK** at `pkg/ldapclient/` produced by `oapi-codegen` from `api/openapi.yaml`. Public API: a `Client` type with one method per endpoint, typed request/response structs, and a sentinel-error mapping for the documented error codes.
- Add **`AGENTS.md`** at repo root — a single page consumed by Claude, Copilot, Cursor, and any AGENTS.md-aware agent. Points to `api/openapi.yaml`, lists the auth scheme, and shows three calling examples (curl, Go via SDK, Go raw `net/http`).
- Add a **CI lint job** that runs Spectral on `api/openapi.yaml` and fails the PR on lint errors.
- Add a **CI generate-and-diff job** that regenerates the SDK and fails the PR if the committed SDK is stale relative to the spec.
- Document the **versioning policy**: semantic versioning on the OpenAPI `info.version` field; SDK module path includes a major-version suffix (`/v1`) once we ship v1.0.0.
- Add an OpenAPI link to `README.md` and a one-paragraph "Calling this service" block with the SDK install command.

Out of scope for this change:

- Publishing the SDK as a separate Go module repo (decision deferred — see open question 1; for MVP it lives in this repo at `pkg/ldapclient`).
- SDKs in other languages (PHP, TypeScript). Add later when a non-Go consumer materializes.
- Changing any handler code, request/response shape, or error format. This change is **descriptive**, not prescriptive — it documents what already exists.
- Auto-publishing the OpenAPI spec to a docs site (Swagger UI / Redoc). Can follow later as a static GitHub Pages job.

## Capabilities

### New Capabilities

- `openapi-contract`: Defines what `api/openapi.yaml` must contain — coverage of every public endpoint, the security scheme, the standard error response, schema fidelity to `internal/domain`, versioning rules, and how the spec is kept in sync with handler code.
- `go-client-sdk`: Defines the public surface and behavior of `pkg/ldapclient` — the `Client` type, constructor options (HTTP client injection, base URL, API key), per-endpoint methods, error mapping (sentinel errors for documented status codes), and the regenerate-from-spec workflow.

### Modified Capabilities

_None — this change adds two new capabilities; no existing requirement changes._

## Impact

- **New files**:
  - `api/openapi.yaml` (hand-authored, ~300 lines)
  - `pkg/ldapclient/client.go` and supporting generated files (output of `oapi-codegen`)
  - `pkg/ldapclient/doc.go` (hand-authored package doc + examples)
  - `pkg/ldapclient/errors.go` (hand-authored sentinel error mapping)
  - `pkg/ldapclient/client_test.go` (round-trip tests against `httptest.Server`)
  - `AGENTS.md`
  - `tools/oapi-codegen-config.yaml`
  - `.github/workflows/ci.yml` additions: `openapi-lint` job, `sdk-stale` job (additive to the file produced by `add-aca-deploy-pipeline`)
- **README updates**: link to OpenAPI, "Calling this service" block, SDK install command.
- **No changes** to `internal/`, `cmd/`, or handler behavior. The spec describes existing behavior; if the spec and handler disagree, the handler is correct and the spec must be fixed in this same change.
- **New dev-time dependency**: `github.com/oapi-codegen/oapi-codegen/v2` (as a Go tool, pinned via `tools.go`). Not added to runtime deps.
- **CI runtime added**: ~30s for Spectral lint + SDK regen-and-diff.
- **Future SDK consumers** can `go get github.com/<org>/ldap-service/pkg/ldapclient@<tag>`. Consumer-side this is a normal Go module dependency.
