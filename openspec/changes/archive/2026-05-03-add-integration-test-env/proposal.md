## Why

The MVP implementation is complete with full unit test coverage, but all LDAP interactions are mocked via the `ldapConn` interface. Before deploying to Azure Container Apps, we need to validate the service end-to-end against real OpenLDAP servers — connection pooling, TLS handshakes, search-then-bind authentication, filter escaping, and dual-source fan-out all need verification against real LDAP protocol behavior. Deploying untested against real LDAP risks repeating the kind of multi-day outage documented in the project's known pitfalls.

## What Changes

- Add Docker Compose environment with two independent OpenLDAP containers simulating internal and external sources
- Add LDIF seed data matching the `o=nycu` directory structure (student, employee, alumni, cooperator, retire OUs)
- Add multi-stage Dockerfile for the service using `scratch` base image
- Add `.env.example` for local development configuration
- Add integration test suite that starts the full stack and exercises all HTTP endpoints against real LDAP

## Capabilities

### New Capabilities
- `integration-test-env`: Docker Compose environment with dual OpenLDAP containers, LDIF seed data, service Dockerfile, and integration test suite for end-to-end validation

### Modified Capabilities

_(none — no spec-level requirement changes, this is infrastructure and test tooling)_

## Impact

- New files: `Dockerfile`, `docker-compose.yml`, `.env.example`, LDIF seed files, integration test files
- New dev dependency: Docker + Docker Compose (not a Go dependency)
- `.gitignore` may need updates for local `.env` and Docker volumes
- CI pipeline can later use this compose setup for automated integration testing
