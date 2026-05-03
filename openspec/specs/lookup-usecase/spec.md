# lookup-usecase Specification

## Purpose
TBD - created by archiving change implement-mvp. Update Purpose after archive.
## Requirements
### Requirement: Single lookup use case
The system SHALL implement `LookupUseCase.Lookup` in `internal/usecase/lookup.go`. It SHALL:
1. Validate username with `domain.ValidateUsername()`
2. Validate all requested attributes against `domain.AllowedAttributes`
3. Delegate to `domain.LDAPRepository.Lookup()` — the repository handles fan-out across sources transparently
4. Propagate context for cancellation

#### Scenario: Valid lookup — internal user
- **WHEN** `Lookup` is called with valid username `"110550001"` and attributes `["mail"]`
- **THEN** it SHALL return the account from the repository (fan-out is transparent to use case)

#### Scenario: Valid lookup — external user
- **WHEN** `Lookup` is called with valid username `"alumni@example.com"` and attributes `["mail"]`
- **THEN** it SHALL return the account from the repository

#### Scenario: Invalid username
- **WHEN** `Lookup` is called with username containing injection chars `"user)(uid=*)"`
- **THEN** it SHALL return `domain.ErrInvalidUsername` without calling the repository

#### Scenario: Disallowed attribute
- **WHEN** `Lookup` is called with attributes `["mail", "userPassword"]`
- **THEN** it SHALL return `domain.ErrAttributeNotAllowed` without calling the repository

### Requirement: Batch lookup use case
The system SHALL implement `LookupUseCase.LookupBatch` in `internal/usecase/lookup.go`. It SHALL:
1. Validate batch size (max 50 usernames)
2. Validate each username with `domain.ValidateUsername()`
3. Validate all requested attributes against `domain.AllowedAttributes`
4. Delegate to `domain.LDAPRepository.LookupBatch()` — fan-out is transparent

#### Scenario: Valid batch lookup with mixed sources
- **WHEN** `LookupBatch` is called with `["110550001", "alumni@example.com"]` and valid attributes
- **THEN** it SHALL return accounts from both sources (the repository handles fan-out)

#### Scenario: Batch exceeds limit
- **WHEN** `LookupBatch` is called with 51 usernames
- **THEN** it SHALL return an error indicating the batch size limit of 50

#### Scenario: One invalid username in batch
- **WHEN** `LookupBatch` is called with `["valid1", "user)(evil", "valid2"]`
- **THEN** it SHALL return `domain.ErrInvalidUsername` without calling the repository

