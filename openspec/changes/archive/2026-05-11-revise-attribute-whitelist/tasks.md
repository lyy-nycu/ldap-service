## 0. Pre-flight (developer, before delegating to Copilot)

- [ ] 0.1 Run `ldapsearch` against the internal pool's read account and confirm the directory returns these exact attribute names: `fullName`, `Alternate-Email`, `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`. Record the verification in a commit-message footer or a comment on this file.
  - Verification attempt (local docker internal LDAP):
    - Command: `docker compose exec -T ldap-internal ldapsearch -x -H ldap://localhost:389 -D "cn=admin,dc=nycu" -w "adminpass" -b "dc=nycu" "(uid=110550001)" dn uid fullName Alternate-Email birthday departmentNumber description disable idno originEmail`
    - Result: only `uid` returned; requested attributes were not present in local fixture data.
    - Follow-up: on-prem internal LDAP verification still required before checking this item.
- [ ] 0.2 Confirm with portal-backend and mfa-service owners that they are ready to cut over to the new attribute names in the same release window.

## 1. Update domain whitelist (`internal/domain/domain.go`)

- [x] 1.1 In `AllowedAttributes`, **rename** the key `"fullname"` to `"fullName"`.
- [x] 1.2 In `AllowedAttributes`, **rename** the key `"alternative-mail"` to `"Alternate-Email"`.
- [x] 1.3 In `AllowedAttributes`, **remove** the key `"deptCode"`.
- [x] 1.4 In `AllowedAttributes`, **add** the keys: `"birthday"`, `"departmentNumber"`, `"description"`, `"disable"`, `"idno"`, `"originEmail"`. Preserve existing alphabetical/grouped ordering convention.
- [x] 1.5 Verify the final map is exactly these 19 entries and nothing else: `cn`, `uid`, `sn`, `givenName`, `fullName`, `initials`, `dept`, `employeeStatus`, `title`, `ou`, `mobile`, `mail`, `Alternate-Email`, `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`.
- [x] 1.6 Do NOT change `ValidateAttributes` behavior. Do NOT add case-insensitive matching or alias mapping. Do NOT add a `DeniedAttributes` set — the credential-deny rule is enforced solely by `userPassword` / `temppassword` / `userCertificate` being absent from `AllowedAttributes`.

## 2. Update domain tests (`internal/domain/domain_test.go`)

- [x] 2.1 Update the "all allowed attributes" case to list exactly the new 19 attributes.
- [x] 2.2 Rename the `"fullname attribute"` case to `"fullName attribute"` and update its input.
- [x] 2.3 Rename the `"hyphenated attribute"` case input from `"alternative-mail"` to `"Alternate-Email"`.
- [x] 2.4 Add positive cases for each new attribute: `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`.
- [x] 2.5 Add **negative cases** to lock in the credential-attribute deny:
  - `"temppassword blocked"` → input `[]string{"temppassword"}`, `wantErr: true`
  - `"userCertificate blocked"` is already present; keep it.
  - `"userPassword blocked"` is already present; keep it.
  - Add `"fullname lowercase rejected"` → input `[]string{"fullname"}`, `wantErr: true` (locks Decision 1 — no case-insensitive matching).
  - Add `"alternative-mail old name rejected"` → input `[]string{"alternative-mail"}`, `wantErr: true`.
  - Add `"deptCode removed"` → input `[]string{"deptCode"}`, `wantErr: true`.
- [x] 2.6 Do NOT delete any existing negative case.

## 3. Update usecase tests (`internal/usecase/lookup_test.go`)

- [x] 3.1 In every test case that uses `"fullname"` in `attributes`, replace with `"fullName"`. Also update the corresponding `mockResult.Attributes` map key and value reference.
- [x] 3.2 Verify both single-lookup and batch-lookup table-driven tests are updated (line ~61 and ~114 in the current file).

## 4. Update handler tests (`internal/handler/lookup_test.go`)

- [x] 4.1 In every request body JSON containing `"fullname"`, replace with `"fullName"`.
- [x] 4.2 In every expected response/mock `Attributes` map, rename the `"fullname"` key to `"fullName"`.
- [x] 4.3 Verify both lookup and batch lookup test tables (line ~45 and ~125 in the current file).

## 5. Update CLAUDE.md

- [x] 5.1 On the "Custom attributes with non-standard names" line (currently line 69), replace `alternative-mail` with `Alternate-Email`.

## 6. Verify

- [x] 6.1 `go build ./...`
- [x] 6.2 `go vet ./...`
- [x] 6.3 `go test ./... -count=1`
- [x] 6.4 Grep for any remaining occurrences of the renamed/removed strings — they should appear ONLY inside negative test cases:
  - `grep -rn 'fullname' internal/ cmd/ test/` — expect only the new negative test case
  - `grep -rn 'alternative-mail' internal/ cmd/ test/` — expect only the new negative test case
  - `grep -rn 'deptCode' internal/ cmd/ test/` — expect only the new negative test case

## 7. Spec sync (supervisor, after Copilot implementation)

- [x] 7.1 Run `openspec validate revise-attribute-whitelist` (or the equivalent skill).
- [ ] 7.2 Archive the change; sync the `ldap-attribute-whitelist-extension` capability spec.
