## 1. Batch Overflow Behavior Fix

- [x] 1.1 Add a typed/sentinel error path for batch size overflow in lookup use case
- [x] 1.2 Map batch size overflow to 400 invalid-request in batch lookup handler
- [x] 1.3 Ensure error message/detail remains generic and RFC 7807-compliant

## 2. Regression Test Coverage

- [x] 2.1 Add handler test case for POST /api/v1/ldap/lookup/batch with 51 usernames
- [x] 2.2 Assert status 400 and Problem type /problems/invalid-request in that case
- [x] 2.3 Run focused tests for usecase and handler packages

## 3. OpenSpec Process Reconciliation

- [x] 3.1 Reconcile implement-mvp task checklist checkboxes with implemented code state
- [x] 3.2 Re-run openspec status/apply instructions and confirm no stale task-progress mismatch remains
- [x] 3.3 Record verification evidence and residual accepted divergences (if any)

## 4. Validation

- [x] 4.1 Run go build ./...
- [x] 4.2 Run go vet ./...
- [x] 4.3 Run go test ./... -count=1
