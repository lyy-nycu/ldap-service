## Context

The LDAP service currently has clear endpoint implementations in handler packages, but the top-level project README provides almost no endpoint-level guidance. New consumers must read source code to understand endpoint purpose, request/response shapes, authentication requirements, and expected failure behavior.

This change adds a focused service guide in the README to document what each function and endpoint means for callers, aligned to existing behavior and security constraints.

## Goals / Non-Goals

**Goals:**
- Provide a concise service guide covering all public endpoints.
- Explain endpoint meaning, not just path/method.
- Include practical request/response examples for integration.
- Document security expectations and common behavior notes.

**Non-Goals:**
- Changing runtime behavior of handlers, middleware, or use cases.
- Adding new endpoints.
- Revising architecture or dependencies.

## Decisions

### Decision 1: Use top-level README as the canonical service guide
Rationale: Consumers discover project usage through README first, so placing endpoint and function explanations there minimizes friction.
Alternatives considered:
- New docs folder file: rejected for now to avoid fragmentation for a small change.
- Inline comments only: rejected because comments are not discoverable by external consumers.

### Decision 2: Organize docs by endpoint purpose and integration workflow
Rationale: Readers need both conceptual meaning and executable examples. Endpoint-centric sections plus quick-start commands support both.
Alternatives considered:
- Pure schema table only: insufficient for behavior semantics.
- Narrative-only docs: too hard to scan during implementation.

### Decision 3: Mirror security semantics already enforced by code
Rationale: Documentation must reflect code behavior exactly, especially around API key protection and generic authentication failure messaging.
Alternatives considered:
- Omit security details: rejected due to integration and support risk.

## Risks / Trade-offs

- [Docs drift from implementation over time] -> Mitigate by linking endpoint claims directly to handler/router behavior and keeping scope focused on currently implemented behavior.
- [Examples may become stale with API evolution] -> Mitigate by keeping examples minimal and tied to stable fields already used by tests and handlers.

## Migration Plan

1. Add service guide sections to README.
2. Validate examples and descriptions against current handlers and router.
3. Run build, vet, and tests to ensure no behavioral changes were introduced.
4. Merge as documentation-only update.

Rollback strategy:
- Revert README section if inaccuracies are discovered; no runtime rollback required.

## Open Questions

- Should this guide remain in README long term, or later be split into docs/api.md when content grows?
