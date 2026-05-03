## Why

The implement-mvp verification found a small set of gaps between expected OpenSpec behavior and current implementation/process state. These gaps block clean verification and increase the risk of regressions in batch lookup error handling.

## What Changes

- Align batch lookup overflow behavior with spec by returning 400 invalid-request (not 500) when usernames exceed the max batch size.
- Add explicit handler-level test coverage for batch overflow mapping to prevent regressions.
- Reconcile MVP packaging/task evidence by normalizing Docker build-stage version expectation and updating task checklist status to reflect completed work.
- Re-run verification checks and capture pass/fail evidence after remediation.

## Capabilities

### New Capabilities
- `mvp-verification-remediation`: Defines acceptance criteria for closing verification findings in behavior mapping, test coverage, and OpenSpec task/process completeness.

### Modified Capabilities
- None.

## Impact

- Affected code: internal/usecase, internal/handler tests, Dockerfile, OpenSpec change/task artifacts.
- APIs: no new endpoints; behavior change on existing batch overflow error response only.
- Dependencies/systems: no new third-party dependencies.
