## Why

The LDAP lookup attribute whitelist (`domain.AllowedAttributes`) is the security boundary for what consumer services can extract from the directory through this service. Two problems with the current list, surfaced while integrating the portal-backend caller:

1. **Casing mismatch with the on-prem directory schema.** The directory returns `fullName` (camelCase) and the custom hyphenated attribute is `Alternate-Email`, not the lowercase / different-string forms (`fullname`, `alternative-mail`) currently in the map. Because `AllowedAttributes` is an exact-match Go map, callers requesting the directory's real attribute names are rejected today.
2. **Missing identity and account-status attributes that real callers need.** `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail` are native LDAP attributes used by upstream portal flows. Their absence forces consumers to either request a superset and filter, or skip the whitelist — neither is acceptable.

A third concern raised during review: credential-bearing attributes (`userPassword`, `temppassword`) MUST NOT be reachable through the lookup surface under any conditions. The current whitelist excludes them implicitly, but only one test (`userPassword blocked`) anchors that contract. This change makes the deny explicit at the spec level so it survives future whitelist edits.

## What Changes

- **Rename** `fullname` → `fullName` in `AllowedAttributes` to match directory schema casing.
- **Rename** `alternative-mail` → `Alternate-Email` to match the actual hyphenated custom attribute name.
- **Add** to `AllowedAttributes`: `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`.
- **Remove** `deptCode` from `AllowedAttributes` (no longer used by any caller; `dept` and `departmentNumber` cover the use cases).
- **Keep** `mobile` (still in use).
- **Codify the credential-attribute deny** as an explicit spec requirement: `userPassword`, `temppassword`, `userCertificate` and any other credential-bearing attributes MUST NOT appear in `AllowedAttributes`. Adding them requires a separate, dedicated change with its own threat model.
- Update `internal/domain/domain.go`, `internal/domain/domain_test.go`, `internal/usecase/lookup_test.go`, `internal/handler/lookup_test.go`, the `ldap-attribute-whitelist-extension` capability spec, and `CLAUDE.md` so the documented "custom attributes with non-standard names" line reflects `Alternate-Email`.

## Capabilities

### Modified Capabilities
- `ldap-attribute-whitelist-extension`: whitelist contents expanded; two renames; explicit credential-attribute deny.

### New Capabilities
- None.

## Impact

**Behavior-affecting (BREAKING for direct callers):**
- Callers requesting `fullname` (lowercase) or `alternative-mail` will receive `ErrAttributeNotAllowed` after this change. They MUST switch to `fullName` / `Alternate-Email`.
- Callers requesting `deptCode` will receive `ErrAttributeNotAllowed`. They MUST switch to `dept` or `departmentNumber`.

**Files touched:**
- `internal/domain/domain.go` — whitelist map contents.
- `internal/domain/domain_test.go` — table-driven cases for all renamed, added, and removed attributes; preserve the `userPassword blocked` and `temppassword blocked` negative cases.
- `internal/usecase/lookup_test.go` — fixture attribute names `fullname` → `fullName`.
- `internal/handler/lookup_test.go` — same rename in request bodies and expected response keys.
- `openspec/specs/ldap-attribute-whitelist-extension/spec.md` — updated by `openspec` sync after archive.
- `CLAUDE.md` — line 69: replace `alternative-mail` with `Alternate-Email`.

**Coordination:**
- Portal-backend (PHP) and MFA Service callers need attribute-name updates in their call sites before this change ships. Confirm with caller owners before merging.
