## Context

The LDAP service MVP is fully implemented with 95+ unit tests, but all LDAP interactions use the `ldapConn` mock interface. The service targets two independent on-prem OpenLDAP servers (internal + external) reachable via Azure S2S VPN. Before deploying, we need to validate real LDAP protocol behavior: TLS negotiation, connection pool lifecycle, search-then-bind auth flow, and fan-out routing.

The production directory uses base DN `o=nycu` with OUs: `student`, `employee`, `retire` (internal), `alumni`, `cooperator` (external).

## Goals / Non-Goals

**Goals:**
- Reproducible local environment that mirrors the dual-source LDAP architecture
- Seed data covering all OUs with accounts that exercise lookup and auth flows
- Multi-stage Dockerfile producing a minimal `scratch`-based production image
- Integration tests validating all HTTP endpoints against real OpenLDAP
- `.env.example` documenting all required environment variables

**Non-Goals:**
- Replicating production data or schema extensions beyond MVP attributes
- CI/CD pipeline integration (can use this compose setup later, but not in scope)
- Performance/load testing
- TLS certificate management for production (local uses self-signed or plaintext for simplicity)

## Decisions

### D1: Two separate OpenLDAP containers (not one with two trees)

Production uses two independent OpenLDAP hosts with separate credentials. Using two containers (`ldap-internal`, `ldap-external`) mirrors this accurately, including independent failure modes. A single container with two suffix databases would conflate failure isolation.

### D2: `osixia/openldap` Docker image

Well-maintained, supports LDIF seed via bind-mount to `/container/service/slapd/assets/config/bootstrap/ldif/custom/`, environment-based configuration for admin DN/password, and optional TLS. No Go dependency added.

### D3: LDIF seed files split by source

Two LDIF files: `seed-internal.ldif` (student, employee, retire OUs with test accounts) and `seed-external.ldif` (alumni, cooperator OUs). Each seeded into its respective container. Test accounts use predictable passwords (`testpass123`) for integration tests.

### D4: Integration tests as a separate Go test package

Place integration tests in `test/integration/` with a `//go:build integration` build tag. They require Docker Compose to be running and are excluded from `go test ./...` by default. Run explicitly via `go test -tags=integration ./test/integration/...`.

**Alternative considered:** Using `testcontainers-go` to start containers programmatically. Rejected — adds a Go dependency not in the approved list, and Docker Compose is simpler for a two-container setup that developers also use for manual testing.

### D5: Multi-stage Dockerfile with scratch base

Stage 1: `golang:1.22-alpine` for build (CGO_DISABLED=1, static binary). Stage 2: `scratch` base with only the binary and CA certificates. Minimal attack surface per spec requirements.

### D6: Plaintext LDAP (port 389) for local development

Production uses LDAPS over VPN. For local Docker, use plaintext LDAP to avoid self-signed cert complexity. The `LDAP_USE_TLS` env var is already supported — set to `false` in `.env.example`. TLS code paths are validated in unit tests via the mock.

## Risks / Trade-offs

- **[Schema drift]** Local seed data may diverge from production schema over time → Mitigation: keep LDIF minimal (only MVP attributes), document which attributes are seeded
- **[Port conflicts]** Containers bind to host ports 3891/3892 → Mitigation: use non-standard ports, document in `.env.example`
- **[No TLS coverage]** Local runs without TLS → Mitigation: TLS code path tested in unit tests; production TLS validated during deployment smoke test
- **[Test data passwords in repo]** Seed LDIF contains plaintext test passwords → Mitigation: these are local-only test accounts, never production credentials, documented as such
