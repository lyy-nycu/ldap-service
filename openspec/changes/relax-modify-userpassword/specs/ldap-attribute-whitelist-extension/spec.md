## ADDED Requirements

### Requirement: Modify write-attribute set is unaffected

The set of write-target attributes accepted by `POST /api/v1/ldap/modify` (`disable`, `userpassword`, `altemate-email`, `temppassword`) SHALL be unchanged by this proposal. Only the per-value validation rule for `userpassword` changes; the attribute itself remains writable, and the misspelling of `altemate-email` remains intentional and unchanged.

#### Scenario: Modify still accepts the four documented attributes

- **WHEN** a modify request body contains any non-empty subset of `disable`, `userpassword`, `altemate-email`, `temppassword`
- **THEN** validation considers the attribute names valid (subject to per-value rules in `domain-types`)

#### Scenario: Modify still rejects attributes outside the write set

- **WHEN** a modify request body contains an attribute key not in the four-attribute write set (e.g. `mail`, `cn`)
- **THEN** the request is rejected as `invalid-request`
