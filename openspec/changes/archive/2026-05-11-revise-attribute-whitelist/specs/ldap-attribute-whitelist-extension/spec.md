## MODIFIED Requirements

### Requirement: Lookup attribute whitelist supports fullname and initials
The system MUST treat `fullName` (camelCase, matching directory schema) and `initials` as allowed LDAP lookup attributes for both single lookup and batch lookup requests. The legacy lowercase form `fullname` MUST be rejected.

#### Scenario: Single lookup requests fullName and initials
- **WHEN** a caller invokes `POST /api/v1/ldap/lookup` with attributes including `fullName` and `initials`
- **THEN** request validation MUST accept these attributes as allowed
- **AND** the lookup flow MUST pass these attributes to repository search
- **AND** the system MUST treat these as native LDAP attributes, not aliases of other fields

#### Scenario: Batch lookup requests fullName and initials
- **WHEN** a caller invokes `POST /api/v1/ldap/lookup/batch` with attributes including `fullName` and `initials`
- **THEN** request validation MUST accept these attributes as allowed
- **AND** the batch lookup flow MUST process these attributes using the same whitelist rules as existing attributes

#### Scenario: Lowercase `fullname` is rejected
- **WHEN** a caller requests the attribute `fullname` (lowercase) on either lookup endpoint
- **THEN** validation MUST return `ErrAttributeNotAllowed`
- **AND** handlers MUST map that domain error to `/problems/attribute-not-allowed`

### Requirement: Fullname and initials do not require attribute alias mapping
The system MUST NOT introduce runtime mapping from `displayName` (or any other attribute) into `fullName` or `initials`. Whitelist matching MUST remain exact-string and case-sensitive.

#### Scenario: Directory schema provides native fullName and initials
- **WHEN** lookup is executed against current on-prem LDAP sources
- **THEN** attribute retrieval MUST rely on native `fullName` and `initials` attributes
- **AND** no alias translation layer is required in application code

#### Scenario: Validation is case-sensitive
- **WHEN** a caller requests an attribute whose casing differs from the whitelist entry
- **THEN** validation MUST return `ErrAttributeNotAllowed`

## ADDED Requirements

### Requirement: Lookup attribute whitelist supports the Alternate-Email custom attribute
The system MUST treat `Alternate-Email` (the directory's hyphenated, mixed-case custom attribute name) as an allowed lookup attribute. The legacy form `alternative-mail` MUST be rejected.

#### Scenario: Single lookup requests Alternate-Email
- **WHEN** a caller invokes `POST /api/v1/ldap/lookup` with attributes including `Alternate-Email`
- **THEN** request validation MUST accept the attribute as allowed
- **AND** the lookup flow MUST pass the attribute to repository search unchanged

#### Scenario: Legacy `alternative-mail` form is rejected
- **WHEN** a caller requests the attribute `alternative-mail`
- **THEN** validation MUST return `ErrAttributeNotAllowed`

### Requirement: Lookup attribute whitelist supports identity and account-status attributes
The system MUST treat the following as allowed lookup attributes, in addition to those previously whitelisted: `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`.

#### Scenario: Identity attributes are accepted
- **WHEN** a caller invokes `POST /api/v1/ldap/lookup` with any subset of `birthday`, `departmentNumber`, `description`, `disable`, `idno`, `originEmail`
- **THEN** request validation MUST accept the attributes as allowed
- **AND** the lookup flow MUST pass the attributes to repository search

#### Scenario: Batch lookup with identity attributes
- **WHEN** a caller invokes `POST /api/v1/ldap/lookup/batch` with any subset of the same attributes
- **THEN** request validation MUST accept the attributes as allowed
- **AND** the batch flow MUST apply the same whitelist rules as for existing attributes

### Requirement: Credential-bearing attributes MUST NOT be reachable through lookup
The system MUST NOT include any credential-bearing or authentication-secret attribute in `AllowedAttributes`. This includes, but is not limited to: `userPassword`, `temppassword`, `userCertificate`, `sambaNTPassword`, `sambaLMPassword`, `krbPrincipalKey`. Adding any such attribute to the whitelist requires a separate, dedicated change with its own threat model and audit story; the lookup whitelist is NOT the appropriate surface for credential retrieval under any circumstances.

#### Scenario: userPassword and temppassword are rejected
- **WHEN** a caller requests `userPassword` or `temppassword` on either lookup endpoint
- **THEN** validation MUST return `ErrAttributeNotAllowed`
- **AND** handlers MUST map that domain error to `/problems/attribute-not-allowed`
- **AND** the rejected attribute name MUST NOT be logged at info level or below in a form that would reveal client intent in production log streams (the existing error wrapping is acceptable; do not add extra logging)

#### Scenario: Whitelist change review includes credential-deny audit
- **WHEN** any future OpenSpec change modifies `AllowedAttributes`
- **THEN** the change author MUST verify that no credential-bearing attribute has been introduced
- **AND** the negative tests for `userPassword` and `temppassword` MUST remain present in `internal/domain/domain_test.go`

