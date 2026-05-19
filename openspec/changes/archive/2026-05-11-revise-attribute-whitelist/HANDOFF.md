# Handoff: revise-attribute-whitelist → Copilot

**Supervisor:** Claude Code (this handoff)
**Implementer:** GitHub Copilot in VS Code
**Reviewer (after impl):** Claude Code via `/security-review`

You (Copilot) are implementing the OpenSpec change at
`openspec/changes/revise-attribute-whitelist/`. Read **proposal.md**, **design.md**, and
**tasks.md** first. This file is the file-by-file action list — do not deviate from it.

---

## Ground rules

1. **Do not widen the trust boundary.** No case-insensitive matching, no alias map, no
   "compatibility" double-entries. If a caller used `fullname`, they get a 400 after this
   change. That is intentional (design.md, Decision 1).
2. **Do not delete the existing `userPassword blocked` test case.** Do not add `userPassword`
   or `temppassword` to `AllowedAttributes` under any circumstances. The credential-deny is
   load-bearing.
3. **Match the final whitelist exactly — 19 entries, nothing else.**
4. Touch only the files listed below. Do not refactor unrelated code.
5. Run the full verification block (Step 6) at the end and paste the output back to the
   supervisor.

---

## Final whitelist (target state)

After your edit, `AllowedAttributes` in `internal/domain/domain.go` MUST contain exactly:

```
cn
uid
sn
givenName
fullName            ← renamed from fullname
initials
dept
employeeStatus
title
ou
mobile              ← KEPT
mail
Alternate-Email     ← renamed from alternative-mail
birthday            ← NEW
departmentNumber    ← NEW
description         ← NEW
disable             ← NEW
idno                ← NEW
originEmail         ← NEW
```

NOT present: `deptCode` (removed), `fullname`, `alternative-mail`, `userPassword`, `temppassword`.

---

## File 1 — `internal/domain/domain.go`

Locate the `AllowedAttributes` declaration (currently around lines 41–56). Replace the map
literal so its keys are exactly the 19 entries above, in this grouped order:

```go
var AllowedAttributes = map[string]bool{
    // Identity — core LDAP
    "cn":               true,
    "uid":              true,
    "sn":               true,
    "givenName":        true,
    "fullName":         true,
    "initials":         true,

    // Organizational
    "dept":             true,
    "departmentNumber": true,
    "employeeStatus":   true,
    "title":            true,
    "ou":               true,

    // Contact
    "mobile":           true,
    "mail":             true,
    "Alternate-Email":  true,

    // Identity — extended (added 2026-05)
    "birthday":         true,
    "description":      true,
    "disable":          true,
    "idno":             true,
    "originEmail":      true,
}
```

Do not change anything else in this file. In particular:
- Do NOT change `ValidateAttributes` — it must remain exact-match.
- Do NOT introduce a `DeniedAttributes` set.
- Do NOT add a `normalizeAttribute` helper.

---

## File 2 — `internal/domain/domain_test.go`

Update `TestValidateAttributes`:

1. Replace the **"all allowed attributes"** case input so it lists exactly the 19 attributes
   above (any order is fine, but list ALL of them).
2. Rename the case `"fullname attribute"` to `"fullName attribute"` and change its input
   from `{"fullname"}` to `{"fullName"}`.
3. Rename the case `"hyphenated attribute"` input from `{"alternative-mail"}` to
   `{"Alternate-Email"}`.
4. Add positive cases (`wantErr: false`):
   - `"birthday attribute"` → `{"birthday"}`
   - `"departmentNumber attribute"` → `{"departmentNumber"}`
   - `"description attribute"` → `{"description"}`
   - `"disable attribute"` → `{"disable"}`
   - `"idno attribute"` → `{"idno"}`
   - `"originEmail attribute"` → `{"originEmail"}`
5. Add negative cases (`wantErr: true`) — these lock in the migration:
   - `"fullname lowercase rejected"` → `{"fullname"}`
   - `"alternative-mail old name rejected"` → `{"alternative-mail"}`
   - `"deptCode removed"` → `{"deptCode"}`
   - `"temppassword blocked"` → `{"temppassword"}`
6. Keep the existing `"userPassword blocked"`, `"objectClass blocked"`, and
   `"userCertificate blocked"` cases as-is.
7. Do NOT delete any other case.

---

## File 3 — `internal/usecase/lookup_test.go`

Around lines 61 and 114 there are cases named `"valid lookup with fullname and initials"` and
`"valid batch with fullname and initials"`.

1. Rename both case names to use `fullName`.
2. In the `attributes` slice, change `"fullname"` → `"fullName"`.
3. In the `mockResult.Attributes` map, rename the `"fullname"` key to `"fullName"`. Preserve
   the map value (`"Student User"`).

No other changes in this file.

---

## File 4 — `internal/handler/lookup_test.go`

Around lines 45 and 125 there are cases that include `"fullname"` in the JSON request body
and in the mocked `Account.Attributes` map.

1. In the request body JSON strings, replace `"fullname"` with `"fullName"`.
2. In the `Attributes` map literals, rename the `"fullname"` key to `"fullName"` (preserve
   the value).
3. Rename the case names to use `fullName`.

No other changes in this file.

---

## File 5 — `CLAUDE.md`

Find this line (currently line 69):

```
- Custom attributes with non-standard names: `alternative-mail` (hyphenated)
```

Replace with:

```
- Custom attributes with non-standard names: `Alternate-Email` (hyphenated, mixed-case)
```

No other edits to `CLAUDE.md`.

---

## Step 6 — Verify

Run these commands in order and paste the full output back to the supervisor:

```bash
go build ./...
go vet ./...
go test ./... -count=1

# Grep for residual references — each line of output should land ONLY inside a negative test case.
grep -rn 'fullname'         internal/ cmd/ test/
grep -rn 'alternative-mail' internal/ cmd/ test/
grep -rn 'deptCode'         internal/ cmd/ test/

# Confirm the credential deny is intact.
grep -n 'userPassword'  internal/domain/domain.go internal/domain/domain_test.go
grep -n 'temppassword'  internal/domain/domain.go internal/domain/domain_test.go
```

**Expected:**
- `go build`, `go vet`, `go test` all clean.
- The first three greps return only lines inside negative test cases in `domain_test.go`.
- `userPassword` appears in `domain_test.go` only (negative cases). It MUST NOT appear in
  `domain.go`. Same for `temppassword`.

If any of these fail, stop and report back — do not "fix forward."

---

## Out of scope (do NOT do these)

- Do not add OpenAPI / SDK regeneration commits — that belongs to a separate change.
- Do not edit `openspec/specs/ldap-attribute-whitelist-extension/spec.md` directly. The
  supervisor will sync it after archive.
- Do not edit any file under `internal/infra/`, `internal/middleware/`, or `cmd/`.
- Do not bump module versions or run `go mod tidy` unless `go build` requires it.
