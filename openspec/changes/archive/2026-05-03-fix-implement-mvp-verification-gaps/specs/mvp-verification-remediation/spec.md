## ADDED Requirements

### Requirement: Batch overflow returns invalid request problem
The system SHALL return HTTP 400 with RFC 7807 Problem type /problems/invalid-request when batch lookup input contains more than 50 usernames.

#### Scenario: Batch size exceeds limit
- **WHEN** POST /api/v1/ldap/lookup/batch is called with 51 usernames
- **THEN** response status SHALL be 400
- **AND** Problem type SHALL be /problems/invalid-request

### Requirement: Batch overflow behavior is regression-tested at handler boundary
The system SHALL include handler-level automated tests that verify oversize batch requests are mapped to invalid-request responses.

#### Scenario: Handler test covers oversize batch mapping
- **WHEN** test suite executes handler tests for batch lookup
- **THEN** at least one test case SHALL assert that oversize batch requests return 400 invalid-request

### Requirement: MVP verification checklist reflects implementation status
The project SHALL keep implement-mvp task checklist entries synchronized with implemented code status before verification is marked complete.

#### Scenario: Checklist synchronization before verification
- **WHEN** verification is run for implement-mvp
- **THEN** openspec/changes/implement-mvp/tasks.md SHALL not report stale unchecked items for already-implemented tasks
