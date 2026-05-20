## ADDED Requirements

### Requirement: ModifyAttrs userpassword accepts plaintext or scheme-prefixed value

The `ModifyAttrs.UserPassword` field on `POST /api/v1/ldap/modify` SHALL accept either:

1. **Plaintext**: any value whose first byte is not `{`. The system SHALL forward the value verbatim to OpenLDAP so that slapd applies its configured `password-hash` directive and `ppolicy` overlay.
2. **Scheme pass-through**: any value matching the regular expression `^\{[A-Z0-9]+\}.+`. The system SHALL forward the value verbatim without modification. Scheme pass-through exists to support admin-reset and migration tooling that intentionally bypasses `ppolicy`.

The system SHALL NOT itself hash, salt, normalize, or transform `userpassword` in any other way.

#### Scenario: Plaintext password is accepted

- **WHEN** a modify request body contains `"userpassword": "Correct-Horse-Battery-Staple"`
- **THEN** validation passes and the value is forwarded to the LDAP `Replace` operation byte-for-byte

#### Scenario: SSHA pass-through is accepted

- **WHEN** a modify request body contains `"userpassword": "{SSHA}abcdef=="`
- **THEN** validation passes and the value is forwarded to the LDAP `Replace` operation byte-for-byte

#### Scenario: ARGON2 pass-through is accepted

- **WHEN** a modify request body contains `"userpassword": "{ARGON2}$argon2id$v=19$..."`
- **THEN** validation passes and the value is forwarded byte-for-byte

### Requirement: ModifyAttrs userpassword plaintext input guards

When `userpassword` is plaintext (i.e. does not begin with `{`), the system SHALL reject the request with `domain.ErrInvalidAttrValue` if any of the following hold:

- value is the empty string
- value length exceeds 256 bytes
- value contains any byte in `0x00`–`0x1F` (C0 control characters, including `\t`, `\n`, `\r`)
- value contains the byte `0x7F` (DEL)

The system SHALL NOT include the offending value in the error message, in any log line, or in the RFC 7807 `detail` field.

#### Scenario: Empty plaintext rejected

- **WHEN** the request body contains `"userpassword": ""`
- **THEN** validation returns `domain.ErrInvalidAttrValue`

#### Scenario: Plaintext with null byte rejected

- **WHEN** the request body contains `"userpassword": "abc\u0000def"`
- **THEN** validation returns `domain.ErrInvalidAttrValue` and no log line contains `abc` or `def`

#### Scenario: Plaintext with newline rejected

- **WHEN** the request body contains `"userpassword": "line1\nline2"`
- **THEN** validation returns `domain.ErrInvalidAttrValue`

#### Scenario: Plaintext over 256 bytes rejected

- **WHEN** the request body contains a 257-byte plaintext `userpassword`
- **THEN** validation returns `domain.ErrInvalidAttrValue`

#### Scenario: Plaintext at exactly 256 bytes accepted

- **WHEN** the request body contains a 256-byte plaintext `userpassword` with no control characters
- **THEN** validation passes

### Requirement: Removal of the {SSHA}-only requirement

The previous validation rule that required `userpassword` to begin with `{SSHA}` and rejected all other shapes SHALL no longer be enforced. Specifically, the system SHALL NOT return `domain.ErrInvalidAttrValue` solely because the value lacks the `{SSHA}` prefix.

#### Scenario: A previously-rejected plaintext value is now accepted

- **WHEN** the request body contains `"userpassword": "plaintext-value"` (which the previous contract rejected)
- **THEN** validation passes
