## ADDED Requirements

### Requirement: Lookup attribute whitelist supports fullname and initials
The system MUST treat `fullname` and `initials` as allowed LDAP lookup attributes for both single lookup and batch lookup requests.

#### Scenario: Single lookup requests fullname and initials
- **WHEN** a caller invokes `POST /api/v1/ldap/lookup` with attributes including `fullname` and `initials`
- **THEN** request validation MUST accept these attributes as allowed
- **AND** the lookup flow MUST pass these attributes to repository search
- **AND** the system MUST treat these as native LDAP attributes, not aliases of other fields

#### Scenario: Batch lookup requests fullname and initials
- **WHEN** a caller invokes `POST /api/v1/ldap/lookup/batch` with attributes including `fullname` and `initials`
- **THEN** request validation MUST accept these attributes as allowed
- **AND** the batch lookup flow MUST process these attributes using the same whitelist rules as existing attributes

### Requirement: Fullname and initials do not require attribute alias mapping
The system MUST NOT introduce runtime mapping from `displayName` (or other attributes) into `fullname` or `initials` for this change.

#### Scenario: Directory schema provides native fullname and initials
- **WHEN** lookup is executed against current on-prem LDAP sources
- **THEN** attribute retrieval MUST rely on native `fullname` and `initials` attributes
- **AND** no alias translation layer is required in application code

### Requirement: Existing attribute restrictions remain enforced
The system MUST continue rejecting non-whitelisted attributes after adding `fullname` and `initials`.

#### Scenario: Sensitive or unknown attribute is requested
- **WHEN** a caller requests an attribute outside the updated whitelist
- **THEN** validation MUST return `ErrAttributeNotAllowed`
- **AND** handlers MUST map that domain error to `/problems/attribute-not-allowed`

### Requirement: Documentation and tests reflect expanded whitelist
The system MUST keep documentation and tests aligned with the updated whitelist to prevent regressions and integration ambiguity.

#### Scenario: Whitelist documentation and tests are reviewed
- **WHEN** project documentation and tests are updated for this change
- **THEN** `fullname` and `initials` MUST appear in allowed-attribute references
- **AND** table-driven tests MUST include positive coverage for these new attributes
