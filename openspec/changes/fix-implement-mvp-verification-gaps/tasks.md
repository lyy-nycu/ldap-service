## 1. Batch Overflow Behavior Fix

- [ ] 1.1 Add a typed/sentinel error path for batch size overflow in lookup use case
- [ ] 1.2 Map batch size overflow to 400 invalid-request in batch lookup handler
- [ ] 1.3 Ensure error message/detail remains generic and RFC 7807-compliant

## 2. Regression Test Coverage

- [ ] 2.1 Add handler test case for POST /api/v1/ldap/lookup/batch with 51 usernames
- [ ] 2.2 Assert status 400 and Problem type /problems/invalid-request in that case
- [ ] 2.3 Run focused tests for usecase and handler packages

## 3. OpenSpec Process Reconciliation

- [ ] 3.1 Reconcile implement-mvp task checklist checkboxes with implemented code state
- [ ] 3.2 Re-run openspec status/apply instructions and confirm no stale task-progress mismatch remains
- [ ] 3.3 Record verification evidence and residual accepted divergences (if any)

## 4. Validation

- [ ] 4.1 Run go build ./...
- [ ] 4.2 Run go vet ./...
- [ ] 4.3 Run go test ./... -count=1
