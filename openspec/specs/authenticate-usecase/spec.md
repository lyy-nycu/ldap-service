# authenticate-usecase Specification

## Purpose
TBD - created by archiving change implement-mvp. Update Purpose after archive.
## Requirements
### Requirement: Authenticate use case
The system SHALL implement `AuthenticateUseCase.Authenticate` in `internal/usecase/authenticate.go`. It SHALL:
1. Validate username with `domain.ValidateUsername()`
2. Verify password is non-empty
3. Delegate to `domain.LDAPRepository.Authenticate()` — the repository handles fan-out (search internal first, then external, bind against whichever server found the user)
4. Return generic failure (`(nil, domain.ErrAuthenticationFailed)`) for ALL failure reasons (user not found, wrong password, service unavailable, malformed input)
5. On success return `(*domain.AuthenticateResult, nil)` where the result carries `UID`, `AccountState`, and `PasswordState`. The use case SHALL NOT make policy decisions based on those state values — callers map them to HTTP semantics.

#### Scenario: Successful authentication — internal user
- **WHEN** `Authenticate` is called with internal username `"110550001"` and correct password
- **THEN** it SHALL return a non-nil `*AuthenticateResult` and `nil` error

#### Scenario: Successful authentication — external user
- **WHEN** `Authenticate` is called with external username `"alumni@example.com"` and correct password
- **THEN** it SHALL return a non-nil `*AuthenticateResult` and `nil` error

#### Scenario: Invalid username format
- **WHEN** `Authenticate` is called with username containing injection chars
- **THEN** it SHALL return `(nil, domain.ErrAuthenticationFailed)` — MUST NOT reveal the reason is invalid format

#### Scenario: Empty password
- **WHEN** `Authenticate` is called with an empty password
- **THEN** it SHALL return `(nil, domain.ErrAuthenticationFailed)` — MUST NOT reveal the reason is empty password

#### Scenario: Wrong password
- **WHEN** the repository returns `(nil, nil)` for a valid username
- **THEN** the use case SHALL return `(nil, domain.ErrAuthenticationFailed)`

#### Scenario: User not found in either source
- **WHEN** the repository returns `(nil, nil)` for a nonexistent username
- **THEN** the use case SHALL return `(nil, domain.ErrAuthenticationFailed)` — indistinguishable from wrong password

#### Scenario: Account/password state propagation
- **WHEN** the repository returns a result with `AccountState=pending_activation` or `PasswordState=expired`/`must_change`
- **THEN** the use case SHALL pass the result through unchanged — bind succeeded, so this is a 200, not 401

### Requirement: Account and password state mapping
The system SHALL derive `account_state` and `password_state` in `internal/infra/ldap/repository.go` from the matched LDAP entry's attributes after a successful bind. The mapping SHALL be:

| LDAP attribute | Derived value |
|---|---|
| `disable=1` | `account_state=pending_activation` |
| `disable=0` or absent | `account_state=active` |
| `pwdReset=TRUE` (case-insensitive) | `password_state=must_change` |
| `pwdChangedTime` older than configured `LDAP_PASSWORD_MAX_AGE` | `password_state=expired` |
| Otherwise | `password_state=current` |

`pwdReset=TRUE` SHALL take precedence over time-based expiry. Malformed `pwdChangedTime` values SHALL fail open as `current` and be logged as a warning. When `LDAP_PASSWORD_MAX_AGE` is zero or unset, the time-based expiry check SHALL be disabled.

The state attributes (`pwdReset`, `pwdChangedTime`) MUST NOT be added to `domain.AllowedAttributes`, so they cannot be returned via the public Lookup API.

### Requirement: Password must not be logged
The authenticate use case MUST NOT log the password value at any log level. The password SHALL only be passed to the repository's `Authenticate` method.

#### Scenario: Log output during authentication
- **WHEN** an authentication attempt occurs (success or failure)
- **THEN** log entries SHALL contain username and result but MUST NOT contain any part of the password

