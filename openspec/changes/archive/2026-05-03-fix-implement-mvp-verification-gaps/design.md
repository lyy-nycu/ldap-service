## Context

The implement-mvp verification identified three practical gaps: stale OpenSpec task completion tracking, batch overflow error mapping that can surface as 500 in the handler path, and missing explicit handler-level regression coverage for that mapping. The service architecture and route contracts are already stable, so remediation should be minimal and localized.

Constraints:
- Keep Clean Architecture boundaries unchanged.
- Do not introduce new dependencies.
- Preserve existing endpoint contracts except where they currently violate approved behavior.

## Goals / Non-Goals

**Goals:**
- Ensure oversize batch lookup requests are translated to 400 invalid-request at the HTTP boundary.
- Add deterministic test coverage that protects this mapping from regressions.
- Synchronize implement-mvp task checkboxes with actual completion before final verification.

**Non-Goals:**
- No new endpoints or payload shapes.
- No changes to LDAP fan-out behavior.
- No broad refactor of use case or repository structure.

## Decisions

1. Keep batch-size validation in use case, but use a typed/sentinel error for size overflow.
- Rationale: validation belongs in use case, while handlers map domain/use case errors to API problems.
- Alternative considered: move size validation into handler only. Rejected because it duplicates validation logic and weakens use case invariants.

2. Map batch-overflow error to NewInvalidRequest in batch handler.
- Rationale: aligns with existing spec scenario for >50 usernames.
- Alternative considered: map to generic internal error. Rejected as non-compliant and user-hostile.

3. Add a dedicated handler test case for 51 usernames.
- Rationale: proves external API behavior at the boundary regardless of use case implementation detail.
- Alternative considered: rely on use case unit tests only. Rejected because handler error mapping is the regression point.

4. Treat OpenSpec task synchronization as a required verification step.
- Rationale: verification quality depends on accurate artifact state, not only passing code/tests.
- Alternative considered: leave task list stale and document exceptions. Rejected because it causes recurring false-critical findings.

## Risks / Trade-offs

- [Risk] Introducing a new error value may require updating multiple switch statements. → Mitigation: centralize mapping in handler and add explicit tests.
- [Risk] Updating many checklist boxes can be error-prone. → Mitigation: reconcile tasks against concrete evidence (build, tests, existing files) in one pass.
- [Trade-off] Minimal remediation keeps velocity high but does not redesign OpenSpec process mechanics. → Mitigation: include checklist synchronization rule in this change tasks.

## Migration Plan

1. Update use case/handler error path for batch overflow.
2. Add/adjust handler test for oversize batch request.
3. Run go build ./..., go vet ./..., go test ./... -count=1.
4. Reconcile implement-mvp tasks.md checkboxes to completed state based on evidence.
5. Re-run verification and confirm no critical finding remains for stale checklist or batch mapping.

Rollback:
- Revert remediation commit if unexpected API behavior appears; no data migration is involved.

## Open Questions

- Should batch-overflow use a new domain sentinel error or reuse an existing invalid-request classification helper?
- Should Docker build-stage version drift be normalized in implement-mvp or tracked as accepted post-MVP divergence?
