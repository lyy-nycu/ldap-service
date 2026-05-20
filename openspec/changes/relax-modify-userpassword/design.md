## Context

`POST /api/v1/ldap/modify` shipped in the `implement-mvp` cycle accepting
`userpassword` only when prefixed with `{SSHA}`. That requirement was copied
from the legacy PHP `api.php` handler, which itself was written before the
NYCU OpenLDAP cluster enabled the `ppolicy` overlay. With `ppolicy` now
active, client-side pre-hashing breaks both password-history enforcement
(`pwdInHistory`) and quality enforcement (`pwdCheckQuality`,
`pwdCheckModule`), because slapd cannot recompute hashes from a value it
never saw in plaintext.

ldap-service is deployed to Azure Container Apps and reaches both LDAP hosts
exclusively over LDAPS / StartTLS via the S2S VPN. The on-the-wire TLS
posture is therefore already strong; what is missing is per-request
enforcement and a relaxation of the application-layer validation.

The contract is consumer-driven and currently has one consumer
(portal-backend, repo `NYCUITSC/portal-backend`, PR #170, fragment file
`openspec/changes/infra-ldap-modify-contract/openapi-fragment.yaml`). The
MFA service is a planned future consumer.

## Goals / Non-Goals

**Goals:**

- Allow callers to send `userpassword` as plaintext over TLS, so slapd can
  hash with its configured `password-hash` directive and `ppolicy` can
  enforce history + quality.
- Keep accepting `{scheme}value` (`{SSHA}`, `{ARGON2}`, …) verbatim so the
  current portal-backend payload keeps working unchanged. Rollout is
  additive on the wire even though the spec changes.
- Guarantee plaintext passwords never leave the cluster unencrypted: refuse
  modify requests if the LDAP connection backing the request is not TLS.
- Guarantee plaintext passwords never appear in zap logs, error messages,
  problem-detail bodies, or panic stacks.
- Provide a small, well-defined set of plaintext sanity guards (length,
  null bytes, control characters) that fail fast with `invalid-attr-value`
  before reaching slapd.

**Non-Goals:**

- Implementing password hashing inside ldap-service. We delegate entirely
  to slapd's `password-hash`.
- Choosing the storage scheme. That is an OpenLDAP cn=config / slapd.conf
  concern owned by ops.
- Configuring `ppolicy` policies (history depth, min length, quality
  module). Owned by ops; documented as a precondition.
- Migrating existing stored passwords. Stored values keep whatever scheme
  they were written with; only new writes flow through the new path.
- Changing any other modify attribute (`disable`, `altemate-email`,
  `temppassword`) or any other endpoint.
- Adding a new authn flow for the operator who triggers password resets.
  That is portal-backend's concern.

## Decisions

### D1. Accept plaintext, keep `{scheme}` pass-through

**Decision**: `userpassword` accepted in two shapes:

1. **Plaintext** (default, recommended): any value whose first byte is not
   `{`. Forwarded to slapd as-is via the LDAP `Replace` op. slapd applies
   `password-hash` and `ppolicy`.
2. **Scheme pass-through**: any value matching the regex
   `^\{[A-Z0-9]+\}` (e.g. `{SSHA}…`, `{ARGON2}…`, `{CRYPT}…`,
   `{CLEARTEXT}…`). Forwarded verbatim. slapd stores as-is without
   re-hashing and without running `pwdCheckQuality`. Use only for admin
   reset / migration tooling.

**Rationale**: This is the only shape that (a) lets the current
portal-backend `{SSHA}` payload keep working during rollout, and (b) makes
plaintext the obvious and ergonomic default for new callers.

**Alternatives considered**:

- *Plaintext only, reject `{scheme}`*: cleanest spec, but breaks the
  existing portal-backend payload on day 1 and forces a synchronized
  deploy. Rejected.
- *ldap-service hashes plaintext itself*: would also break `pwdInHistory`
  (we'd use our own salt) and gives ldap-service a crypto responsibility
  it should not own. Rejected.
- *Discriminator field in the request body* (e.g. `userpassword_hashed:
  bool`): adds shape complexity to the contract for no real benefit; the
  `{scheme}` prefix is already the de-facto LDAP convention. Rejected.

### D2. TLS precondition is per-request, not just startup

**Decision**: When a modify request contains `userpassword`, the
`Repository.Modify` path asserts that the underlying `*ldap.Conn` is using
TLS before sending the modify PDU. If not, the request fails with
`ErrServiceUnavailable` → HTTP 500 and an explicit log line
`"refusing to send plaintext userpassword over non-TLS ldap connection"`
(without the password value).

**Rationale**: A startup-only check can drift (e.g. a future change adds a
non-TLS fallback for some other operation). A per-request check is cheap
(a single `conn.IsTLS()` call) and is the only invariant that actually
matters at the moment the secret leaves the process.

**Alternatives considered**:

- *Startup-only check via config validation*: simpler but loses the
  invariant under future code changes. Rejected.
- *Refuse plaintext entirely on non-TLS conns but allow `{scheme}`
  pass-through*: tempting, but `{CLEARTEXT}foo` is also plaintext. Cleaner
  to require TLS for any `userpassword` value. Adopted.

### D3. Plaintext input guards

**Decision**: For plaintext (non-`{scheme}`) values, validate:

- non-empty
- length ≤ 256 bytes
- no `\x00` (null byte)
- no `\r`, `\n`, or other C0 control characters (`< 0x20`) except `\t`?
  → reject `\t` too; passwords shouldn't contain it.

Failures → `domain.ErrInvalidAttrValue` →
`/problems/invalid-attr-value` (HTTP 400).

**Rationale**: slapd will reject some of these on its own, but doing so
inside ldap-service gives callers a clear, structured error instead of an
opaque `ConstraintViolation` from LDAP.

### D4. Error envelope: replace `userpassword-not-ssha` with `userpassword-malformed`

**Decision**: Drop the implicit "must start with `{SSHA}`" error case from
the `invalid-attr-value` family. Add a new sub-case
`userpassword-malformed` covering the D3 guards. `disable` validation is
unchanged.

The Problem Details body keeps the same `type`
(`/problems/invalid-attr-value`) — only the `detail` text changes — so
existing consumers do not need to add new `type` branches.

### D5. Coordinated contract update

**Decision**: ldap-service ships first (additive: still accepts `{SSHA}`).
Once staged, open a coordinated PR in `NYCUITSC/portal-backend` updating
`openapi-fragment.yaml`:

- `userpassword`: `description` updated, `pattern` removed (or relaxed to
  `^([^\x00-\x1f]{1,256}|\{[A-Z0-9]+\}.+)$`).
- Add a plaintext-input test case to `backend-go/internal/adapter/ldap/modify_test.go`.

portal-backend's runtime code does not need to change immediately — its
existing `{SSHA}` payload is still accepted. Migration to plaintext can
happen in a follow-up PR on its own schedule.

### D6. Integration test asserts server-side hashing

**Decision**: Extend `test/integration/modify_test.go` with a new case:

1. Modify with plaintext `userpassword: "Correct-Horse-Battery-Staple"`.
2. Re-read the entry as the read-only bind DN and assert the stored value
   starts with `{` (i.e. slapd hashed it; we don't assert the specific
   scheme, since that is an ops decision).
3. Bind as the user with the original plaintext password and assert
   success.

This proves both legs of the contract end-to-end and will fail loudly if
`password-hash` is misconfigured on the test fixture (which is the same
shape as a production misconfiguration).

## Risks / Trade-offs

- **[Risk] OpenLDAP misconfigured (`password-hash` unset)** → slapd stores
  plaintext silently.
  **Mitigation**: integration test in D6; documented precondition in the
  spec delta; staging smoke-test reads back a written password and
  asserts `{`-prefix before promoting to prod.

- **[Risk] `{CLEARTEXT}foo` pass-through stores plaintext on purpose**.
  This is an LDAP convention, not a bug, but it would defeat the purpose
  of the change.
  **Mitigation**: do not advertise `{CLEARTEXT}` in the README or examples;
  the validator still accepts any `{SCHEME}` since slapd will reject
  unknown schemes itself; we do not whitelist schemes (whitelisting in
  the application would couple us to slapd's enabled modules).

- **[Risk] Plaintext leaks into logs via a future code change**.
  **Mitigation**: regression test that asserts no log line emitted by
  the modify path contains the request's `userpassword` value. Add a zap
  observer in the test.

- **[Risk] TLS check is bypassed because pool returns a stale non-TLS conn
  during a future refactor**.
  **Mitigation**: per-request `conn.IsTLS()` assertion (D2); compile-time
  test that asserts `Pool.dial` always returns a TLS connection when
  `useTLS=true`.

- **[Trade-off] We accept `{scheme}` pass-through forever, not just during
  rollout**. This means a misbehaving caller can opt out of `ppolicy` by
  sending `{SSHA}…`. Acceptable: we trust API-key holders to be
  well-intentioned, and pass-through is genuinely useful for admin reset
  flows (where the operator wants to set a known temporary password
  without triggering quality rules).

- **[Trade-off] No discriminator field; we sniff the leading byte**. This
  is brittle if some future scheme name starts with a non-`{` character,
  but no such RFC-defined scheme exists, and slapd itself uses the same
  convention.

## Migration Plan

1. Land this proposal's code in ldap-service. Ship to staging.
2. Run the D6 integration test against the staging OpenLDAP fixture. Confirm
   stored value starts with `{` and that bind with the plaintext succeeds.
3. Promote ldap-service to prod (additive change; portal-backend payload
   unchanged).
4. Open coordinated PR in `NYCUITSC/portal-backend` updating
   `openapi-fragment.yaml` and adding a plaintext consumer test case. Land
   when reviewed.
5. (Optional, separate PR) Migrate portal-backend's caller to send plaintext
   so that NYCU's `ppolicy` history starts protecting real users.

**Rollback**: revert the ldap-service PR. The old code rejected plaintext;
any caller that has migrated to plaintext between staging and rollback
will see `400 invalid-attr-value` until they revert too. Window is short
because (1) portal-backend has not migrated yet at the point of the
ldap-service revert, and (2) we always keep one canary period in staging
before promoting.

## Open Questions

- Should we whitelist the `{scheme}` set we accept, or trust slapd? Current
  decision: trust slapd. Revisit if we find a scheme that slapd silently
  no-ops on.
- Do we want a structured audit-log entry on every successful password
  write (subject_id, operator service name from API key, timestamp; never
  the value)? Useful for incident response but out of scope for this
  change.
