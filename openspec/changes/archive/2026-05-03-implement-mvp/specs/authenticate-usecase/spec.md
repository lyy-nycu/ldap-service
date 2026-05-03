## ADDED Requirements

### Requirement: Authenticate use case
The system SHALL implement `AuthenticateUseCase.Authenticate` in `internal/usecase/authenticate.go`. It SHALL:
1. Validate username with `domain.ValidateUsername()`
2. Verify password is non-empty
3. Delegate to `domain.LDAPRepository.Authenticate()` — the repository handles fan-out (search internal first, then external, bind against whichever server found the user)
4. Return generic failure for ALL failure reasons (user not found, wrong password)

#### Scenario: Successful authentication — internal user
- **WHEN** `Authenticate` is called with internal username `"110550001"` and correct password
- **THEN** it SHALL return `(true, nil)`

#### Scenario: Successful authentication — external user
- **WHEN** `Authenticate` is called with external username `"alumni@example.com"` and correct password
- **THEN** it SHALL return `(true, nil)`

#### Scenario: Invalid username format
- **WHEN** `Authenticate` is called with username containing injection chars
- **THEN** it SHALL return `(false, domain.ErrAuthenticationFailed)` — MUST NOT reveal the reason is invalid format

#### Scenario: Empty password
- **WHEN** `Authenticate` is called with an empty password
- **THEN** it SHALL return `(false, domain.ErrAuthenticationFailed)` — MUST NOT reveal the reason is empty password

#### Scenario: Wrong password
- **WHEN** the repository returns `(false, nil)` for a valid username
- **THEN** the use case SHALL return `(false, domain.ErrAuthenticationFailed)`

#### Scenario: User not found in either source
- **WHEN** the repository returns `(false, nil)` for a nonexistent username
- **THEN** the use case SHALL return `(false, domain.ErrAuthenticationFailed)` — indistinguishable from wrong password

### Requirement: Password must not be logged
The authenticate use case MUST NOT log the password value at any log level. The password SHALL only be passed to the repository's `Authenticate` method.

#### Scenario: Log output during authentication
- **WHEN** an authentication attempt occurs (success or failure)
- **THEN** log entries SHALL contain username and result but MUST NOT contain any part of the password
