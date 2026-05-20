## ADDED Requirements

### Requirement: Modify handler error mapping for userpassword

The `POST /api/v1/ldap/modify` handler SHALL map use-case errors to RFC 7807 Problem Details responses as follows for `userpassword`-related failures:

| Use-case error | HTTP status | Problem `type` |
| --- | --- | --- |
| `ErrInvalidAttrValue` (plaintext guard failure) | 400 | `/problems/invalid-attr-value` |
| `ErrServiceUnavailable` (TLS precondition failed or both LDAP sources down) | 500 | `/problems/internal-error` |

The Problem Details `detail` field SHALL NOT contain the submitted `userpassword` value or any prefix, suffix, or length of it.

#### Scenario: Plaintext null byte returns 400 invalid-attr-value

- **WHEN** a modify request contains `"userpassword": "abc\u0000def"`
- **THEN** the response is HTTP 400 with `type: "/problems/invalid-attr-value"` and the `detail` does not contain the substrings `abc` or `def`

#### Scenario: Non-TLS LDAP backend returns 500 internal-error

- **WHEN** the modify use case returns `ErrServiceUnavailable` due to the TLS precondition
- **THEN** the response is HTTP 500 with `type: "/problems/internal-error"`

### Requirement: Modify handler accepts both plaintext and scheme-prefixed userpassword

The `POST /api/v1/ldap/modify` handler SHALL no longer reject requests on the basis that `userpassword` lacks a `{SSHA}` prefix. It SHALL forward the value to the use case for validation per the `domain-types` capability and propagate any resulting error per the table above.

#### Scenario: Plaintext userpassword is accepted

- **WHEN** a well-formed request contains `"userpassword": "Correct-Horse-Battery-Staple"`
- **THEN** the handler returns HTTP 200 with `{"modified": ["userpassword"]}` (or whatever subset was requested)

#### Scenario: SSHA-prefixed userpassword is still accepted

- **WHEN** a well-formed request contains `"userpassword": "{SSHA}abc=="`
- **THEN** the handler returns HTTP 200 with `userpassword` included in `modified`
