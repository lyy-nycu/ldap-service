## 1. LDIF Seed Data

- [x] 1.1 Create `testdata/seed-internal.ldif` — base DN `o=nycu`, OUs (`student`, `employee`, `retire`), at least one account per OU with `uid`, `mail`, `displayName`, `userPassword` (plaintext `testpass123`)
- [x] 1.2 Create `testdata/seed-external.ldif` — base DN `o=nycu`, OUs (`alumni`, `cooperator`), at least one account per OU with same attributes

## 2. Docker Configuration

- [x] 2.1 Create `Dockerfile` — multi-stage build: `golang:1.22-alpine` build stage with `CGO_ENABLED=0`, `scratch` final stage with binary + CA certs
- [x] 2.2 Create `docker-compose.yml` — three services: `ldap-internal` (osixia/openldap, port 3891, mounts seed-internal.ldif), `ldap-external` (osixia/openldap, port 3892, mounts seed-external.ldif), `service` (builds from Dockerfile, depends on both LDAP containers, loads .env)
- [x] 2.3 Create `.env.example` — all required env vars with values pointing to local Docker containers (hosts=ldap-internal/ldap-external, ports=389, TLS=false, bind DNs, API keys, rate limit settings)

## 3. Git and Project Hygiene

- [x] 3.1 Update `.gitignore` — ensure `.env`, Docker volumes, and any local data directories are excluded
- [x] 3.2 Verify `docker compose up` starts all three containers and the service logs successful LDAP pool initialization

## 4. Integration Test Suite

- [x] 4.1 Create `test/integration/main_test.go` — `TestMain` setup with `//go:build integration` tag, HTTP client helper, base URL and API key from env vars
- [x] 4.2 Create `test/integration/health_test.go` — test GET `/healthz` returns 200 + `{"status":"ok"}`, test GET `/readyz` returns 200 + `{"status":"ready"}`
- [x] 4.3 Create `test/integration/lookup_test.go` — test single lookup for internal account (verify source=internal), test single lookup for external-only account (verify source=external, fan-out), test batch lookup with mixed found/not-found
- [x] 4.4 Create `test/integration/authenticate_test.go` — test successful auth with correct password, test failed auth with wrong password (verify generic 401 response), test auth without API key returns 401
- [x] 4.5 Create `test/integration/ratelimit_test.go` — test that N+1 auth requests for same username returns 429

## 5. Validation

- [x] 5.1 Run `go test ./... -count=1` to confirm integration tests are excluded from default test run
- [x] 5.2 Run `docker compose up -d` then `go test -tags=integration ./test/integration/... -count=1 -v` to confirm all integration tests pass
- [x] 5.3 Run `docker compose down` and verify clean teardown
