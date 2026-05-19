## 1. RED — failing tests (PR 1)

- [ ] 1.1 Branch `feat/relax-userpassword-red` from `main`.
- [ ] 1.2 Extend `internal/usecase/modify_test.go` with table cases:
  - `plaintext password accepted` — `"Correct-Horse-Battery-Staple"` → no error, repo receives `userpassword` verbatim.
  - `ssha pass-through still accepted` — `"{SSHA}abc=="` → no error.
  - `argon2 pass-through accepted` — `"{ARGON2}$argon2id$..."` → no error.
  - `empty plaintext rejected` → `ErrInvalidAttrValue`.
  - `plaintext with null byte rejected` → `ErrInvalidAttrValue` (and assert returned error message does not contain `abc`/`def`).
  - `plaintext with newline rejected` → `ErrInvalidAttrValue`.
  - `plaintext at 256 bytes accepted` → no error.
  - `plaintext at 257 bytes rejected` → `ErrInvalidAttrValue`.
  - Remove (or update assertion of) the existing `userpassword missing SSHA prefix` case so it now asserts the plaintext is accepted.
- [ ] 1.3 Add `internal/handler/modify_test.go` cases:
  - plaintext payload returns 200 with `userpassword` in `modified`.
  - null-byte payload returns 400 `/problems/invalid-attr-value` and response body does not contain `abc`/`def`.
  - TLS-precondition failure surfaces as 500 `/problems/internal-error` (driven by mock returning `ErrServiceUnavailable`).
- [ ] 1.4 Add `internal/infra/ldap/repository_modify_test.go` cases:
  - `userpassword forwarded verbatim (plaintext)` — assert `Replace("userpassword", []string{"p4ssw0rd!"})`.
  - `userpassword forwarded verbatim ({SSHA})` — assert `Replace("userpassword", []string{"{SSHA}abc=="})`.
  - `non-TLS conn with userpassword → ErrServiceUnavailable, no PDU sent`.
  - `non-TLS conn with disable-only → still proceeds` (assert PDU sent).
  - `non-TLS conn with userpassword → log line "refusing to send userpassword over non-TLS ldap connection" emitted, value not in log`. Use `zaptest/observer`.
- [ ] 1.5 Add `internal/usecase/modify_test.go` log-leak regression:
  - With a zap observer, run a request whose plaintext password contains a recognizable token; assert no observed log entry's encoded form contains the token.
- [ ] 1.6 Run `go vet ./... && go test ./...` — confirm new tests fail in the expected ways (red).
- [ ] 1.7 Open PR titled `RED: relax /ldap/modify userpassword validation (tests only)`, body links to this change folder.

## 2. GREEN — implementation (PR 2)

- [ ] 2.1 Branch `feat/relax-userpassword-green` from `feat/relax-userpassword-red` (so PR 2 includes PR 1's tests).
- [ ] 2.2 `internal/domain/modify.go`:
  - Update the acceptance-criteria comment on `ModifyAttrs.UserPassword` to document plaintext + `{scheme}` pass-through, and the input guards.
  - No struct/field change (`UserPassword string` stays).
- [ ] 2.3 `internal/usecase/modify.go`:
  - Replace `strings.HasPrefix(attrs.UserPassword, "{SSHA}")` check with:
    - if value starts with `{` and matches `^\{[A-Z0-9]+\}.+` → pass-through, skip guards.
    - else (plaintext) → enforce: non-empty, ≤256 bytes, no byte in `0x00..0x1F`, no `0x7F`.
  - Use a package-level compiled regex; do not put the value in any returned error.
- [ ] 2.4 `internal/infra/ldap/pool.go`:
  - Expose a TLS check on the borrowed connection (e.g. `conn.IsTLS bool` cached at dial time, accessed via a small wrapper struct returned by `getConn`). Do not depend on `*ldap.Conn` reflection.
- [ ] 2.5 `internal/infra/ldap/repository.go` (or `pool.go`'s `Modify`):
  - Before sending `Modify` PDU, if request includes `userpassword` and `conn.IsTLS == false`, log `"refusing to send userpassword over non-TLS ldap connection"` with `subject_uid` and `source`, return `domain.ErrServiceUnavailable`.
  - Pass value byte-for-byte to `ldap.NewModifyRequest().Replace("userpassword", []string{value})`.
- [ ] 2.6 `internal/handler/modify.go`:
  - Confirm error mapping table covers the cases (no functional change expected; verify `ErrInvalidAttrValue → 400` and `ErrServiceUnavailable → 500` still wired).
- [ ] 2.7 `test/integration/modify_test.go`:
  - Add `TestModify_PlaintextUserpassword_ServerHashes`:
    1. Modify with plaintext.
    2. Re-read entry as read-only bind; assert stored value starts with `{`.
    3. Bind as user with the plaintext; assert success.
  - Keep one existing `{SSHA}` integration test case as pass-through coverage.
- [ ] 2.8 `README` §6:
  - Change example payload to plaintext.
  - Add "Caller responsibilities / password handling" note explaining plaintext-default, `{scheme}` pass-through, TLS requirement, and that ldap-service does not hash.
- [ ] 2.9 Run `go vet ./... && go test ./... -count=1` — all green.
- [ ] 2.10 Bring up `docker compose up -d --wait ldap-internal ldap-external` and run `go test ./test/integration/... -count=1 -tags=integration` (or whatever build tag the suite uses) — all green.
- [ ] 2.11 Open PR titled `GREEN: relax /ldap/modify userpassword validation (impl + integration)`, body links to this change folder and to PR 1.

## 3. Cross-repo contract sync

- [ ] 3.1 In `NYCUITSC/portal-backend`, open a PR updating `openspec/changes/infra-ldap-modify-contract/openapi-fragment.yaml`:
  - Relax `userpassword` schema: drop `pattern: ^\{SSHA\}`; update `description` to document plaintext + `{scheme}` pass-through.
- [ ] 3.2 In the same portal-backend PR, add a plaintext case to `backend-go/internal/adapter/ldap/modify_test.go` (consumer side).
- [ ] 3.3 Link the portal-backend PR from this change's tasks completion comment.

## 4. Ship & verify

- [ ] 4.1 After GREEN PR merges to `main`, confirm staging deploy succeeds (Build + Deploy Staging green in `.github/workflows/deploy.yml` run).
- [ ] 4.2 Post-deploy smoke from portal-backend side: write a plaintext password against staging, read back, bind. Capture run output for the PR comment.
- [ ] 4.3 Hold prod promotion until portal-backend's contract-sync PR is merged (so docs and runtime line up).

## 5. Archive

- [ ] 5.1 Once shipped to prod and portal-backend PR merged, run `openspec archive relax-modify-userpassword`.
