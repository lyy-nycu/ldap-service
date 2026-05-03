# domain-types Specification

## Purpose
TBD - created by archiving change implement-mvp. Update Purpose after archive.
## Requirements
### Requirement: Account entity
The system SHALL define an `Account` struct in `internal/domain/domain.go` with fields: `DN` (string), `UID` (string), `Attributes` (map[string]string), `Source` (string — `"internal"` or `"external"`).

#### Scenario: Account struct is usable by all layers
- **WHEN** a lookup or authenticate operation completes successfully
- **THEN** the result SHALL be represented as an `Account` value with DN, UID, requested attributes, and source populated

#### Scenario: Account from internal LDAP
- **WHEN** a student account is found on the internal LDAP server
- **THEN** `Account.Source` SHALL be `"internal"`

#### Scenario: Account from external LDAP
- **WHEN** an alumni account is found on the external LDAP server
- **THEN** `Account.Source` SHALL be `"external"`

### Requirement: LDAP source constants
The system SHALL define constants `SourceInternal = "internal"` and `SourceExternal = "external"` in `internal/domain/domain.go`.

#### Scenario: Source constants used consistently
- **WHEN** any code sets `Account.Source`
- **THEN** it SHALL use the domain constants, not string literals

### Requirement: Attribute whitelist
The system SHALL define an `AllowedAttributes` set in `internal/domain/domain.go` containing exactly: `cn`, `uid`, `sn`, `givenName`, `dept`, `deptCode`, `employeeStatus`, `title`, `ou`, `mobile`, `mail`, `alternative-mail`.

#### Scenario: Validate allowed attribute
- **WHEN** a caller requests attributes `["mail", "mobile"]`
- **THEN** validation SHALL pass because both are in the whitelist

#### Scenario: Reject disallowed attribute
- **WHEN** a caller requests attribute `"userPassword"`
- **THEN** validation SHALL return `ErrAttributeNotAllowed` with the offending attribute name

### Requirement: Username validation
The system SHALL define a `ValidateUsername(string) error` function that accepts usernames matching `^[a-zA-Z0-9._@-]{1,128}$` and rejects all others with `ErrInvalidUsername`. The regex includes `@` to support email-style usernames used by external LDAP users (cooperators, alumni). Max length is 128 to accommodate email addresses.

#### Scenario: Valid internal username (student number)
- **WHEN** input is `"110550001"`
- **THEN** `ValidateUsername` SHALL return nil

#### Scenario: Valid internal username (employee number)
- **WHEN** input is `"T1234"`
- **THEN** `ValidateUsername` SHALL return nil

#### Scenario: Valid external username (email)
- **WHEN** input is `"user@example.com"`
- **THEN** `ValidateUsername` SHALL return nil

#### Scenario: Invalid username with injection characters
- **WHEN** input is `"user)(uid=*)"`
- **THEN** `ValidateUsername` SHALL return `ErrInvalidUsername`

#### Scenario: Empty username
- **WHEN** input is `""`
- **THEN** `ValidateUsername` SHALL return `ErrInvalidUsername`

### Requirement: Domain error types
The system SHALL define sentinel errors: `ErrAccountNotFound`, `ErrInvalidUsername`, `ErrAttributeNotAllowed`, `ErrAuthenticationFailed`, `ErrServiceUnavailable`, `ErrRateLimitExceeded`.

#### Scenario: Error types are distinct
- **WHEN** any domain function returns an error
- **THEN** callers SHALL be able to distinguish error types using `errors.Is()`

### Requirement: LDAPRepository interface
The system SHALL define a `LDAPRepository` interface in `internal/domain/domain.go` with methods:
- `Lookup(ctx context.Context, username string, attributes []string) (*Account, error)`
- `LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*Account, []string, error)`
- `Authenticate(ctx context.Context, username string, password string) (bool, error)`
- `HealthCheck(ctx context.Context) error`

The `LDAPRepository` interface represents the **aggregate** of both LDAP sources. Implementations SHALL handle fan-out internally — callers do not need to know about multiple sources.

#### Scenario: Interface is implementable
- **WHEN** `infra/ldap/repository.go` implements `LDAPRepository`
- **THEN** it SHALL satisfy the interface at compile time via `var _ domain.LDAPRepository = (*Repository)(nil)`

### Requirement: LDAPPool interface
The system SHALL define a `LDAPPool` interface in `internal/domain/domain.go` with methods:
- `Search(ctx context.Context, username string, attributes []string) (*Account, error)`
- `Bind(ctx context.Context, dn string, password string) error`
- `HealthCheck(ctx context.Context) error`
- `Close() error`

This interface represents a single LDAP server's connection pool. The `Repository` struct orchestrates two `LDAPPool` instances.

#### Scenario: LDAPPool interface is implementable
- **WHEN** `infra/ldap/pool.go` implements `LDAPPool`
- **THEN** it SHALL satisfy the interface at compile time

### Requirement: UseCase interfaces
The system SHALL define `LookupUseCase` and `AuthenticateUseCase` interfaces in `internal/domain/domain.go`.

`LookupUseCase`:
- `Lookup(ctx context.Context, username string, attributes []string) (*Account, error)`
- `LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*Account, []string, error)`

`AuthenticateUseCase`:
- `Authenticate(ctx context.Context, username string, password string) (bool, error)`

#### Scenario: UseCase interfaces decouple handlers from implementation
- **WHEN** handlers depend on use case interfaces
- **THEN** they SHALL not import the `usecase` or `infra` packages directly

### Requirement: RFC 7807 Problem Details
The system SHALL define a `Problem` struct in `internal/domain/problem.go` with fields: `Type` (string), `Title` (string), `Status` (int), `Detail` (string, optional), `Instance` (string, optional). JSON tags SHALL use lowercase field names. The struct SHALL implement the `error` interface.

The system SHALL define constructor functions for each error type URI:
- `NewInvalidRequest(detail, instance string) *Problem`
- `NewInvalidUsername(detail, instance string) *Problem`
- `NewAttributeNotAllowed(detail, instance string) *Problem`
- `NewUnauthorized(detail, instance string) *Problem`
- `NewAuthenticationFailed(instance string) *Problem`
- `NewNotFound(detail, instance string) *Problem`
- `NewServiceUnavailable(detail, instance string) *Problem`
- `NewInternalError(detail, instance string) *Problem`
- `NewRateLimitExceeded(instance string) *Problem`

#### Scenario: Problem serializes to RFC 7807 JSON
- **WHEN** a `Problem{Type: "/problems/invalid-username", Title: "Invalid username", Status: 400}` is marshaled to JSON
- **THEN** the output SHALL match `{"type":"/problems/invalid-username","title":"Invalid username","status":400}`

#### Scenario: AuthenticationFailed has fixed detail
- **WHEN** `NewAuthenticationFailed` is called
- **THEN** the detail SHALL always be `"authentication failed"` regardless of the actual failure reason

