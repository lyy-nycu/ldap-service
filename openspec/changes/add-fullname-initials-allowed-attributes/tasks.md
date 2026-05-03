## 1. Update Domain Whitelist

- [ ] 1.1 Add `fullname` and `initials` to `AllowedAttributes` in `internal/domain/domain.go`
- [ ] 1.2 Ensure `ValidateAttributes` behavior remains unchanged for non-whitelisted attributes
- [ ] 1.3 Keep lookup behavior as native-attribute retrieval only (do not add `displayName` alias mapping)

## 2. Update Test Coverage

- [ ] 2.1 Extend domain attribute validation tests to include positive cases for `fullname` and `initials`
- [ ] 2.2 Add or update handler/usecase tests to confirm lookup and batch lookup accept `fullname` and `initials`

## 3. Update Documentation

- [ ] 3.1 Add `fullname` and `initials` to allowed-attribute documentation in README/OpenSpec docs where applicable
- [ ] 3.2 Clarify that this change extends global whitelist only (no per-service ACL change)

## 4. Verify

- [ ] 4.1 Run `go build ./...`
- [ ] 4.2 Run `go vet ./...`
- [ ] 4.3 Run `go test ./... -count=1`
