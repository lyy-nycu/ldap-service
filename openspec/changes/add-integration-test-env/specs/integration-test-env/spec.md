## ADDED Requirements

### Requirement: Docker Compose dual-LDAP environment
The system SHALL provide a Docker Compose configuration that starts two independent OpenLDAP containers (`ldap-internal` and `ldap-external`) and one service container, forming a complete local development and test environment.

#### Scenario: Start full stack
- **WHEN** developer runs `docker compose up`
- **THEN** two OpenLDAP containers start with seeded data and the service container starts and connects to both

#### Scenario: Independent failure simulation
- **WHEN** the internal LDAP container is stopped while the external remains running
- **THEN** the service returns results from the external source for accounts that exist there and returns service unavailable for internal-only accounts

### Requirement: LDIF seed data matches production directory structure
The seed data SHALL use base DN `o=nycu` with organizational units: `student`, `employee`, `retire` (internal server) and `alumni`, `cooperator` (external server). Each OU SHALL contain at least one test account with uid, mail, and displayName attributes.

#### Scenario: Internal server seed data
- **WHEN** the internal LDAP container starts
- **THEN** it contains OUs `student`, `employee`, `retire` under `o=nycu`, each with at least one account entry that has `uid`, `mail`, and `displayName` attributes

#### Scenario: External server seed data
- **WHEN** the external LDAP container starts
- **THEN** it contains OUs `alumni`, `cooperator` under `o=nycu`, each with at least one account entry that has `uid`, `mail`, and `displayName` attributes

#### Scenario: Accounts have authenticatable passwords
- **WHEN** a test account exists in the seed data
- **THEN** it has a `userPassword` attribute set to a known test value so integration tests can verify authentication

### Requirement: Multi-stage Dockerfile with scratch base
The Dockerfile SHALL use a multi-stage build: Go build stage producing a statically-linked binary, and a final `scratch` stage containing only the binary and CA certificates.

#### Scenario: Build produces minimal image
- **WHEN** the Dockerfile is built
- **THEN** the final image uses `scratch` as its base and contains only the service binary and `/etc/ssl/certs/ca-certificates.crt`

#### Scenario: Static binary compilation
- **WHEN** the Go binary is compiled in the build stage
- **THEN** CGO is disabled (`CGO_ENABLED=0`) and the binary is statically linked

### Requirement: Environment variable example file
The project SHALL include a `.env.example` file documenting all required environment variables with safe placeholder values.

#### Scenario: Developer copies env file
- **WHEN** a developer copies `.env.example` to `.env` and starts Docker Compose
- **THEN** the service connects to the local LDAP containers using the example values without modification

### Requirement: Integration test suite with build tag
Integration tests SHALL be placed in `test/integration/` with a `//go:build integration` build tag. They SHALL NOT run during `go test ./...`.

#### Scenario: Default test run excludes integration tests
- **WHEN** developer runs `go test ./...`
- **THEN** integration tests in `test/integration/` are not executed

#### Scenario: Explicit integration test run
- **WHEN** developer runs `go test -tags=integration ./test/integration/...`
- **THEN** integration tests execute against running Docker Compose environment

### Requirement: Integration tests cover all HTTP endpoints
The integration test suite SHALL exercise every HTTP endpoint defined in the router.

#### Scenario: Health check endpoints
- **WHEN** integration tests run against the live service
- **THEN** GET `/healthz` returns 200 with `{"status":"ok"}` and GET `/readyz` returns 200 with `{"status":"ready"}`

#### Scenario: Single lookup success
- **WHEN** integration test sends POST `/api/v1/ldap/lookup` with a valid API key, a seeded username, and allowed attributes
- **THEN** the response is 200 with the account's DN, UID, source, and requested attributes

#### Scenario: Batch lookup with mixed results
- **WHEN** integration test sends POST `/api/v1/ldap/lookup/batch` with one existing and one non-existing username
- **THEN** the response is 200 with the found account in `accounts` and the missing username in `not_found`

#### Scenario: Authentication success
- **WHEN** integration test sends POST `/api/v1/ldap/authenticate` with a seeded username and correct password
- **THEN** the response is 200 with `{"authenticated":true}`

#### Scenario: Authentication failure
- **WHEN** integration test sends POST `/api/v1/ldap/authenticate` with a seeded username and wrong password
- **THEN** the response is 401 with a generic authentication failed message (RFC 7807)

#### Scenario: Fan-out lookup finds external account
- **WHEN** integration test sends a lookup for a username that exists only in the external LDAP
- **THEN** the response is 200 with `source` set to `"external"`

#### Scenario: API key required
- **WHEN** integration test sends a request to `/api/v1/ldap/lookup` without an API key header
- **THEN** the response is 401

#### Scenario: Rate limiting on authenticate
- **WHEN** integration test sends more than the configured rate limit of authentication requests for the same username
- **THEN** subsequent requests receive 429 Too Many Requests
