## Why

The current `POST /api/v1/ldap/modify` contract requires the caller to send
`userpassword` already pre-hashed with the `{SSHA}` prefix. This was inherited
verbatim from the legacy PHP behaviour and locked into the consumer-driven
contract in `infra-ldap-modify-contract` without revisiting OpenLDAP's password
policy interactions.

In practice this breaks two OpenLDAP `ppolicy` features we rely on:

- **`pwdInHistory` (password reuse prevention)** — history comparison requires
  the server to hash the candidate with the same scheme/salt strategy as the
  stored history entries. Client-side hashing with a fresh random salt every
  time makes every new password look unique, so history is effectively
  disabled.
- **`pwdCheckQuality` / `pwdCheckModule` / `pwdMinLength`** — quality checks
  can only inspect plaintext. With pre-hashed input the server cannot enforce
  them at all.

It also pushes hashing crypto into every caller (today portal-backend; tomorrow
MFA service, admin tools, etc.), each of which must reimplement salt
generation correctly. That is the wrong layer for that responsibility.

## What Changes

- **BREAKING (contract)**: relax the `userpassword` validation rule on
  `POST /api/v1/ldap/modify` from "MUST start with `{SSHA}`" to "plaintext, OR
  a value beginning with a recognized `{scheme}` prefix for pass-through".
  - Plaintext is the recommended path. OpenLDAP hashes per its configured
    `password-hash` directive, so `ppolicy` (history, quality) works.
  - `{SSHA}…` / `{ARGON2}…` / `{CRYPT}…` / `{SHA}…` / `{MD5}…` / `{CLEARTEXT}…`
    continue to be accepted verbatim (pass-through) so the current
    portal-backend payload keeps working unchanged during rollout. This makes
    the change **additive at the wire level** even though it is breaking at the
    spec level.
- **Hard precondition**: refuse to start (or refuse the request) when the LDAP
  connection used for modify is not TLS. Plaintext passwords must never leave
  the cluster in the clear.
- Add explicit plaintext-input guards: reject empty string, null bytes,
  bare control characters, and values exceeding a sane max length (e.g. 256
  bytes) to fail fast before reaching slapd.
- Confirm zap log output never contains the `userpassword` value (today
  request bodies are not logged; add a regression test).
- Update the consumer-driven OpenAPI fragment in
  `NYCUITSC/portal-backend` (`openspec/changes/infra-ldap-modify-contract/openapi-fragment.yaml`)
  in a coordinated PR so the published contract matches.
- Update README §6 and the modify example to show plaintext and explain when
  `{scheme}` pass-through is appropriate (admin reset flows).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `domain-types`: relax the `ModifyAttrs.userpassword` validation rule and
  document accepted shapes (plaintext vs. `{scheme}` pass-through).
- `lookup-usecase`: n/a — listed only to avoid confusion; not modified. (The
  modify use case lives next to it but this proposal does not touch lookup.)
- `ldap-repository`: document that `Modify` forwards `userpassword` verbatim
  and that the LDAP connection MUST be TLS when the request contains
  `userpassword`.
- `http-handlers`: update the `/api/v1/ldap/modify` request-validation
  contract and the error-mapping table (the existing
  `invalid-attr-value/userpassword-not-ssha` problem is removed; new
  `invalid-attr-value/userpassword-malformed` covers null bytes / control
  chars / oversize).
- `ldap-attribute-whitelist-extension`: clarify that the four modify write
  attributes (`disable`, `userpassword`, `altemate-email`, `temppassword`) are
  unaffected at the whitelist level — only the per-value validation for
  `userpassword` changes.

## Impact

- **Code**: `internal/usecase/modify.go` (drop `{SSHA}` requirement, add
  plaintext guards), `internal/domain/modify.go` (validation comment + error
  set), `internal/infra/ldap/pool.go` / `repository.go` (TLS precondition for
  modify), `internal/handler/modify.go` (error mapping), tests across all
  three layers, `test/integration/modify_test.go` (plaintext path → bind with
  same plaintext succeeds).
- **Contract / cross-repo**: coordinated PR in `NYCUITSC/portal-backend` to
  update `openapi-fragment.yaml` and the consumer-side `modify_test.go` table
  (add plaintext case, keep one `{SSHA}` pass-through case).
- **Ops / OpenLDAP**: both internal and external servers must have
  `password-hash {SSHA}` (or the agreed scheme) configured and the `ppolicy`
  overlay enabled with the desired `pwdInHistory` / `pwdCheckQuality`. This
  is a precondition, not a code change here, but the rollout plan must
  verify it per environment.
- **Security posture**: net improvement — plaintext leaves the caller over
  TLS to ldap-service, then over TLS to slapd; storage remains hashed by
  slapd; ppolicy now actually enforces history and quality. Risk: if any
  environment is misconfigured (LDAP not TLS, or `password-hash` unset),
  behaviour silently degrades. Mitigated by startup TLS assertion + an
  integration test that reads back the stored value and asserts it begins
  with `{` (i.e. server hashed it).
- **Callers**: portal-backend can stop computing SSHA itself. No immediate
  code change required (its current `{SSHA}` payload still works), but the
  recommended migration is to send plaintext to enable ppolicy.
- **Deployment order**: ldap-service ships first (additive: still accepts
  `{SSHA}`), then portal-backend contract update, then portal-backend caller
  migrates to plaintext.
