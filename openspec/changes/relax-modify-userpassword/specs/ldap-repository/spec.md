## ADDED Requirements

### Requirement: Modify path requires TLS LDAP connection when userpassword is present

When a `Repository.Modify` call carries a non-empty `userpassword` value (plaintext or scheme-prefixed), the underlying `*ldap.Conn` borrowed from the pool SHALL be using TLS (LDAPS or completed StartTLS) at the moment the `Modify` PDU is sent.

If the connection is not TLS, the system SHALL:

- abort the modify operation before sending any PDU
- return `domain.ErrServiceUnavailable`
- emit a single zap log line at `error` level with the message `"refusing to send userpassword over non-TLS ldap connection"`, the subject `uid`, and the connection's source label (`"internal"` or `"external"`)
- NOT include the `userpassword` value, password length, or any derived value in the log line

The TLS check SHALL be performed per request, not only at pool initialization, so that future changes to the dialer cannot silently regress this invariant.

#### Scenario: TLS connection allows userpassword modify

- **WHEN** `Repository.Modify` is called with `userpassword` set and the pooled connection is TLS
- **THEN** the modify proceeds and returns the list of modified attribute names

#### Scenario: Non-TLS connection rejects userpassword modify

- **WHEN** `Repository.Modify` is called with `userpassword` set and the pooled connection is not TLS
- **THEN** the call returns `domain.ErrServiceUnavailable` and no `Modify` PDU is sent

#### Scenario: Non-TLS connection still allows non-password modify

- **WHEN** `Repository.Modify` is called with only `disable` set (no `userpassword`) and the pooled connection is not TLS
- **THEN** the modify proceeds normally (the TLS precondition only applies when `userpassword` is present)

### Requirement: userpassword value is forwarded verbatim

The system SHALL pass the `userpassword` value to the LDAP `Replace` operation byte-for-byte, with no client-side hashing, salting, normalization, base64 wrapping, or scheme tagging.

#### Scenario: Plaintext is forwarded byte-for-byte

- **WHEN** `Repository.Modify` is called with `userpassword: "p4ssw0rd!"`
- **THEN** the `ldap.NewModifyRequest().Replace("userpassword", ...)` call receives exactly `[]string{"p4ssw0rd!"}`

#### Scenario: Scheme-prefixed value is forwarded byte-for-byte

- **WHEN** `Repository.Modify` is called with `userpassword: "{SSHA}abc=="`
- **THEN** the `Replace` call receives exactly `[]string{"{SSHA}abc=="}`
