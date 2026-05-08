## ADDED Requirements

### Requirement: Spec file location and format

The repository SHALL contain exactly one OpenAPI document at `api/openapi.yaml`. The document MUST declare `openapi: 3.1.0` (or a later 3.1.x patch version). It MUST be valid YAML and parseable by `oapi-codegen v2` and Spectral without warnings other than the documented exceptions in Q1 of design.

#### Scenario: Spec file exists at the canonical path

- **WHEN** a tool reads `api/openapi.yaml`
- **THEN** the file exists, is non-empty, and parses as YAML
- **AND** the top-level `openapi` field equals `3.1.0` or matches the regex `^3\.1\.\d+$`

#### Scenario: Spec passes Spectral lint

- **WHEN** CI runs `spectral lint api/openapi.yaml --ruleset spectral:oas --fail-severity error`
- **THEN** the command exits 0

### Requirement: Coverage of every public endpoint

The spec SHALL describe every route registered in `internal/handler/router.go`, with operation IDs in lowerCamelCase: `healthz`, `readyz`, `lookup`, `batchLookup`, `authenticate`. Each operation MUST list its HTTP method, path, request body schema (where applicable), all documented response statuses, and a one-line `summary` plus a multi-line `description`.

#### Scenario: All five operations are present

- **WHEN** the spec is parsed
- **THEN** the set of `(method, path)` pairs equals exactly: `GET /healthz`, `GET /readyz`, `POST /api/v1/ldap/lookup`, `POST /api/v1/ldap/lookup/batch`, `POST /api/v1/ldap/authenticate`
- **AND** every operation has a non-empty `summary` and `description`
- **AND** every operation has an `operationId` matching the listed names

### Requirement: Schemas mirror internal/domain types

Every request and response schema in the spec SHALL correspond to a Go type in `internal/domain/`. The schema name SHALL match the Go type name exactly (e.g. `Account`, `Problem`). Field names SHALL be the JSON tags from those Go structs. New fields added to a domain type are a breaking spec change unless marked `nullable` and added to neither `required` nor any prior 1.x examples.

#### Scenario: Account schema matches the Go struct

- **WHEN** a reader compares the `components.schemas.Account` definition to `internal/domain/domain.go` `type Account struct { ... }`
- **THEN** every JSON-tagged field on the struct appears as a property on the schema
- **AND** no schema property exists that does not correspond to a struct field

#### Scenario: Problem schema matches RFC 7807 + the Go struct

- **WHEN** a reader compares `components.schemas.Problem` to `internal/domain/problem.go` `type Problem struct { ... }`
- **THEN** the schema includes properties `type`, `title`, `status`, `detail`, `instance`, `request_id`
- **AND** `status` is `integer`, all others are `string` or `string|null`

### Requirement: API key security scheme is defined and applied

The spec SHALL define an `apiKey` security scheme of type `apiKey`, in `header`, named `X-API-Key`. Every operation EXCEPT `healthz` and `readyz` SHALL list `[{ apiKey: [] }]` in its `security` array. The two health endpoints SHALL set `security: []` explicitly, denoting public access.

#### Scenario: Protected operations require the API key

- **WHEN** the spec is parsed
- **THEN** `lookup`, `batchLookup`, and `authenticate` each have `security: [{ apiKey: [] }]`

#### Scenario: Health operations are explicitly public

- **WHEN** the spec is parsed
- **THEN** `healthz` and `readyz` each have `security: []` (an empty array, not absent)

### Requirement: Error response uses application/problem+json with the Problem schema

Every operation that can fail SHALL document the relevant 4xx and 5xx responses as `application/problem+json` with the `Problem` schema. Documented statuses SHALL include at least: `400 Bad Request` (lookup, batchLookup, authenticate), `401 Unauthorized` (lookup, batchLookup, authenticate), `429 Too Many Requests` (authenticate), `500 Internal Server Error` (all non-health operations).

#### Scenario: Authenticate documents 401 and 429

- **WHEN** the spec is parsed
- **THEN** the `authenticate` operation has both a `401` and a `429` response
- **AND** each response's `content` declares `application/problem+json` with `schema: { $ref: '#/components/schemas/Problem' }`

### Requirement: Rate-limit headers are documented

Operations rate-limited at the middleware (`authenticate` per CLAUDE.md "5 attempts per minute") SHALL declare `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `Retry-After` response headers on at least the `429` response.

#### Scenario: Authenticate's 429 response declares rate-limit headers

- **WHEN** the `authenticate` operation's `429` response is inspected
- **THEN** its `headers` map contains `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `Retry-After`
- **AND** each is described and typed (`integer` or `string`)

### Requirement: Versioning policy

The spec's `info.version` SHALL be a SemVer 2.0.0 string. Initial release is `0.1.0`. Any change to a request schema, response schema (other than adding optional fields), error contract, or removed endpoint SHALL bump the major version once `info.version >= 1.0.0`. Pre-1.0, breaking changes are permitted but MUST be called out in `CHANGELOG.md`.

#### Scenario: Version is a valid semver

- **WHEN** the spec is parsed
- **THEN** `info.version` matches the SemVer regex
- **AND** the version is `0.1.0` at the time this change lands

### Requirement: Service identity and contact

The spec's `info` block SHALL include `title` (`LDAP Service API`), `description` (one paragraph), `version`, and `contact` (with at least an email). `servers` SHALL list at least one `https://` URL placeholder for the prod deployment with a `description`.

#### Scenario: Info block is fully populated

- **WHEN** the spec is parsed
- **THEN** `info.title`, `info.description`, `info.version`, and `info.contact.email` are all non-empty
- **AND** `servers` has at least one entry whose `url` starts with `https://`

### Requirement: Contract test verifies spec matches handler behavior

The repository SHALL contain a build-tagged contract test (`//go:build contract`) that, for each operation in the spec, exercises the running server via the SDK and asserts both happy-path and at least one documented error path. CI SHALL run this test on any PR that modifies `api/openapi.yaml`, `internal/handler/`, or `pkg/ldapclient/`.

#### Scenario: Contract test fails when handler diverges from spec

- **WHEN** a developer changes a handler to return `{ "uid": ... }` instead of `{ "username": ... }` (matching the existing struct tag) without updating the spec
- **THEN** the `contract` test fails because the generated SDK cannot decode the response into the documented schema

#### Scenario: CI invokes the contract test on relevant PRs

- **WHEN** a PR modifies any file under `api/` or `internal/handler/` or `pkg/ldapclient/`
- **THEN** the CI job that runs `go test -tags=contract ./pkg/ldapclient/...` is required and must pass
