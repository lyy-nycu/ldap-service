## Context

`domain.AllowedAttributes` (in `internal/domain/domain.go`) is the single chokepoint that decides which LDAP attributes can be (a) requested via `POST /api/v1/ldap/lookup{,/batch}` and (b) returned to the caller. `domain.ValidateAttributes` rejects anything outside the map with `ErrAttributeNotAllowed`, which handlers translate to `/problems/attribute-not-allowed` (RFC 7807).

The map is exact-match — Go's `map[string]bool` is case-sensitive and does not treat hyphenation/camelCase as equivalent. LDAP itself is case-insensitive for attribute *names* on the wire, but our validation layer is not, so the strings in the map MUST match what callers send literally.

The previous `add-fullname-initials-allowed-attributes` change added these attributes using the directory's pre-2026 lowercase casing. The current directory schema (post-2026-04 sync) uses `fullName` and `Alternate-Email`. Calls using the documented attribute names from the directory thus fail today.

## Goals / Non-Goals

**Goals:**
- Bring `AllowedAttributes` into alignment with the actual on-prem LDAP schema (`fullName`, `Alternate-Email`).
- Expand the whitelist to include identity and account-status attributes (`birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`) that production callers require.
- Promote the implicit credential-attribute deny into an explicit spec requirement so it cannot drift.

**Non-Goals:**
- No per-caller / per-API-key ACL.
- No attribute-alias translation layer (we do not transparently rewrite `fullname` → `fullName`).
- No new endpoints; lookup surface unchanged.
- No change to authentication flow, rate limiting, or pool behavior.
- No introduction of `userPassword` or `temppassword` access via this or any future lookup surface — those would require a separate, dedicated, audited mechanism, not a whitelist entry.

## Decisions

### Decision 1: Use directory-native casing, do not normalize in the application layer
**Rationale:** The whitelist is a security control; making it case-insensitive or adding an alias map would (a) widen the matching surface, (b) hide schema drift, and (c) couple the service to a translation table that has to be kept in lockstep with the directory. Reject inputs that don't match the directory exactly — callers updating their call sites is a one-time cost.
**Alternatives considered:**
- Make `ValidateAttributes` case-insensitive: rejected — silently widens the trust boundary and complicates audit.
- Add a `fullname` → `fullName` alias map: rejected — same drift risk and obscures the actual contract.

### Decision 2: Codify the credential-attribute deny as a spec requirement, not just a test
**Rationale:** The supervisor / Copilot loop will edit `AllowedAttributes` many more times. A single negative test (`userPassword blocked`) can be deleted by an over-eager edit. A capability-spec requirement that names the forbidden attributes and the reason makes the constraint impossible to remove without an explicit OpenSpec change that documents the threat model.
**Alternatives considered:**
- Rely on existing tests only: rejected — fragile under refactors.
- Move the deny check to a separate `DeniedAttributes` set in code: deferred — adds complexity for a list of three entries; revisit if the deny list grows.

### Decision 3: Treat this as breaking for the existing callers and coordinate before merge
**Rationale:** Renaming `fullname` → `fullName`, `alternative-mail` → `Alternate-Email`, and removing `deptCode` will return `400 attribute-not-allowed` to any caller still using the old names. The service has two known callers (portal-backend, mfa-service); both are in-house. A coordinated cutover is cheaper than running a compatibility layer indefinitely.
**Alternatives considered:**
- Ship a deprecation window with both names allowed: rejected — same drift / audit risk as Decision 1, and we have full caller visibility.

### Decision 4: Keep `mobile`, drop `deptCode`
**Rationale:** Caller audit (portal-backend + mfa-service) shows `mobile` still in use for security-alert delivery, while `deptCode` has no remaining consumer — `dept` (display name) and `departmentNumber` (numeric ID) cover the use cases.
**Alternatives considered:**
- Drop both: rejected — `mobile` is referenced by `SendTotpApi` in portal-backend.

## Risks / Trade-offs

- **Breaking change for direct callers using old attribute names.**
  Mitigation: confirm caller readiness before merge; coordinate cutover. Concrete callers to verify: `portal-backend` (PHP `LdapService`) and `mfa-service` consumers of `/api/v1/ldap/lookup`.
- **Risk that the renamed attributes are not actually what the directory returns.**
  Mitigation: verify directly against the live directory (one `ldapsearch` against the internal pool with `-LLL` for raw attribute names) before merging. Document the verification result in tasks.md.
- **Credential-attribute deny requirement could be misread as an exhaustive list.**
  Mitigation: phrase the spec requirement as "any credential-bearing attribute, including but not limited to … MUST NOT appear" so future entries (e.g. `sambaNTPassword`) are covered by the same rule without re-amendment.

## Migration Plan

1. Verify directory schema names with a read-only `ldapsearch` from the internal pool's read account.
2. Update `AllowedAttributes` map contents in `internal/domain/domain.go`.
3. Update all tests that reference renamed/removed attribute strings.
4. Update `CLAUDE.md` line 69 to reflect `Alternate-Email`.
5. Run `go build ./... && go vet ./... && go test ./... -count=1`.
6. Coordinate caller updates (portal-backend, mfa-service) — they must switch attribute names in the same release window.
7. Archive the OpenSpec change; sync the capability spec.

**Rollback strategy:**
Revert the commit. The whitelist is a single-file change in `internal/domain/domain.go`; behavior reverts instantly. Callers that already migrated to new names will fail until they revert too — accept this and document in the release notes.

## Open Questions

- Confirm that `disable` is the exact attribute name returned by the directory (vs `nsAccountLock`, `pwdAccountLockedTime`, or a custom attribute). The developer MUST verify before Copilot edits.
- Confirm `originEmail` casing — directory may use `originEmail` or `originemail`. Verify via `ldapsearch`.

## Assumptions

- The on-prem LDAP schema returns attributes named exactly: `fullName`, `Alternate-Email`, `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`. This MUST be verified (see Open Questions).
- No caller currently depends on `deptCode`. (Validated via grep of portal-backend and mfa-service.)
- This change does NOT introduce or open the door to credential-attribute access. Any future request to expose `userPassword` / `temppassword` is a separate, dedicated change with its own threat model and audit story.
