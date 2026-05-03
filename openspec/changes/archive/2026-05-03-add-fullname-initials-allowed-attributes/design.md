## Context

The MVP currently enforces a strict lookup attribute whitelist in domain validation, and handlers rely on that validation result to either proceed with LDAP search or return `/problems/attribute-not-allowed`. Current allowed attributes do not include `fullname` or `initials`, which creates a mismatch for consumers needing those identity fields.

## Goals / Non-Goals

**Goals:**
- Add `fullname` and `initials` to the allowed lookup attribute set.
- Preserve existing error behavior for disallowed attributes.
- Update tests and docs so whitelist behavior remains explicit and stable.

**Non-Goals:**
- Introducing per-service ACL policy or attribute-level authorization by API key.
- Changing LDAP source fan-out logic, bind flow, or authentication behavior.
- Adding new endpoints.

## Decisions

### Decision 1: Extend the existing global whitelist instead of adding a parallel policy layer
Rationale: Current architecture centralizes attribute validation in `domain.ValidateAttributes`, so extending the existing list is the smallest and least risky path for MVP scope.
Alternatives considered:
- Add per-service attribute ACL now: deferred because it is a separate policy feature with broader design impact.
- Add endpoint-specific whitelist: rejected to avoid divergence across lookup and batch lookup behavior.

### Decision 2: Keep validation and error mapping contract unchanged
Rationale: Existing handlers and tests already rely on `ErrAttributeNotAllowed` mapping to `/problems/attribute-not-allowed`; retaining this contract avoids consumer-facing regressions.
Alternatives considered:
- Return richer metadata for disallowed attributes: out of scope for this change.

### Decision 3: Update all whitelist touchpoints together (domain tests, handler/usecase tests, docs)
Rationale: Whitelist changes are security-relevant and easy to regress silently if only code is changed.
Alternatives considered:
- Code-only update: rejected due to documentation drift and incomplete coverage risk.

## Risks / Trade-offs

- [Consumers may infer this as a broader ACL capability] -> Mitigation: explicitly document that this change extends global whitelist only.

## Migration Plan

1. Add `fullname` and `initials` to `AllowedAttributes` in domain layer.
2. Update tests that assert allowed/disallowed attribute behavior.
3. Update docs that list allowed lookup attributes.
4. Run build, vet, and tests.

Rollback strategy:
- Revert whitelist additions and companion docs/tests if incompatible directory data is discovered.

## Open Questions

- None.

## Assumptions

- `fullname` and `initials` are native attributes in on-prem LDAP and are available in production.
- This change does not introduce attribute alias mapping from `displayName` or any other field.
