## ADDED Requirements

### Requirement: Pool struct (single-server connection pool)
The system SHALL implement a `Pool` struct in `internal/infra/ldap/pool.go` that manages connections to a single LDAP server. It SHALL be initialized with host, port, TLS config, bind DN, bind password, pool size, base DN, and a source label (`"internal"` or `"external"`).

#### Scenario: Pool created successfully
- **WHEN** the LDAP server is reachable and credentials are valid
- **THEN** the pool SHALL contain the configured number of ready connections

#### Scenario: LDAP server unreachable at startup
- **WHEN** the LDAP server is not reachable during pool initialization
- **THEN** `NewPool()` SHALL return an error (caller decides whether to fatal)

### Requirement: Connection liveness check
The `Pool` SHALL check connection liveness when borrowing from the pool. If a connection is dead (e.g., VPN disruption), the system SHALL discard it and create a new one.

#### Scenario: Stale connection detected
- **WHEN** a connection borrowed from the pool fails a liveness check
- **THEN** the system SHALL close it, create a new connection, bind with read-only credentials, and return the new connection

### Requirement: Pool overflow handling
The `Pool` SHALL allow creating connections beyond pool size when the pool is exhausted. Overflow connections SHALL be closed after use (not returned to pool).

#### Scenario: Pool exhausted
- **WHEN** all pooled connections are in use and a new request arrives
- **THEN** the system SHALL create a temporary connection and close it after the operation completes

### Requirement: Pool Search operation
The `Pool` SHALL implement `Search(ctx, username, attributes)` by performing an LDAP search with base=configured base DN, scope=WholeSubtree, filter=`(uid=<escaped_username>)`. The username MUST be escaped with `ldap.EscapeFilter()`. The returned `Account.Source` SHALL be set to the pool's source label.

#### Scenario: Account found
- **WHEN** searching for username `"110550001"` with attributes `["mail", "dept"]` on the internal pool
- **THEN** the result SHALL contain the account's DN, UID, requested attributes, and `Source: "internal"`

#### Scenario: Account not found
- **WHEN** searching for a username that does not exist on this server
- **THEN** the pool SHALL return `domain.ErrAccountNotFound`

### Requirement: Pool Bind operation
The `Pool` SHALL implement `Bind(ctx, dn, password)` by creating a new connection (NOT from the pool) and attempting bind with the given DN and password. The connection SHALL be closed after the bind attempt.

#### Scenario: Successful bind
- **WHEN** DN and password are correct
- **THEN** `Bind` SHALL return nil

#### Scenario: Failed bind
- **WHEN** password is incorrect
- **THEN** `Bind` SHALL return an LDAP invalid credentials error

### Requirement: Pool HealthCheck
The `Pool` SHALL implement `HealthCheck(ctx)` by borrowing a connection and performing a simple search to verify connectivity.

#### Scenario: Server healthy
- **WHEN** the LDAP server is reachable
- **THEN** `HealthCheck` SHALL return nil

#### Scenario: Server unreachable
- **WHEN** the LDAP server is unreachable
- **THEN** `HealthCheck` SHALL return an error

### Requirement: Pool Close
The `Pool` SHALL provide a `Close()` method that drains and closes all connections.

#### Scenario: Graceful shutdown
- **WHEN** `Close()` is called
- **THEN** all pooled connections SHALL be closed

### Requirement: Repository struct (fan-out orchestrator)
The system SHALL implement a `Repository` struct in `internal/infra/ldap/repository.go` that holds two `Pool` instances (internal and external) and implements `domain.LDAPRepository`. It SHALL contain the fan-out logic.

#### Scenario: Repository initialization
- **WHEN** `NewRepository(internalPool, externalPool, logger)` is called
- **THEN** the repository SHALL be ready to serve requests using both pools

### Requirement: Repository Lookup fan-out
The `Repository.Lookup` SHALL search the internal pool first. If the account is not found (`ErrAccountNotFound`), it SHALL search the external pool. If the internal pool returns a connection error, the system SHALL log the error and still try the external pool.

#### Scenario: Account found in internal LDAP
- **WHEN** username `"110550001"` exists in internal LDAP
- **THEN** `Lookup` SHALL return the account with `Source: "internal"` without querying external

#### Scenario: Account found in external LDAP
- **WHEN** username `"alumni@example.com"` does not exist in internal LDAP but exists in external
- **THEN** `Lookup` SHALL return the account with `Source: "external"`

#### Scenario: Account not found in either source
- **WHEN** a username does not exist in either LDAP server
- **THEN** `Lookup` SHALL return `domain.ErrAccountNotFound`

#### Scenario: Internal LDAP down, external healthy
- **WHEN** the internal pool returns a connection error and the account exists in external
- **THEN** `Lookup` SHALL log the internal error and return the account from external

#### Scenario: Both LDAP sources down
- **WHEN** both pools return connection errors
- **THEN** `Lookup` SHALL return `domain.ErrServiceUnavailable`

### Requirement: Repository LookupBatch fan-out
The `Repository.LookupBatch` SHALL perform individual fan-out lookups for each username (up to 50). Found accounts SHALL be returned in the accounts slice; not-found usernames SHALL be returned in the not_found slice.

#### Scenario: Mixed results across sources
- **WHEN** batch includes internal user `"110550001"` and external user `"alumni@example.com"`
- **THEN** both SHALL be returned in accounts with their respective `Source` values

### Requirement: Repository Authenticate fan-out
The `Repository.Authenticate` SHALL:
1. Search internal pool for the user's DN
2. If not found, search external pool
3. Once DN is found, call `Bind` on the **same pool** that found the user
4. Return `(true, nil)` on success, `(false, nil)` on any failure

#### Scenario: Internal user authenticates
- **WHEN** username `"110550001"` is found in internal LDAP and password is correct
- **THEN** bind SHALL be performed against the internal pool and return `(true, nil)`

#### Scenario: External user authenticates
- **WHEN** username `"alumni@example.com"` is found in external LDAP and password is correct
- **THEN** bind SHALL be performed against the external pool and return `(true, nil)`

#### Scenario: User not found in either source
- **WHEN** username does not exist in either server
- **THEN** `Authenticate` SHALL return `(false, nil)`

### Requirement: Repository HealthCheck
The `Repository.HealthCheck` SHALL check both pools. It SHALL return nil only if both pools are healthy. If either is unhealthy, it SHALL return `domain.ErrServiceUnavailable`.

#### Scenario: Both healthy
- **WHEN** both LDAP servers are reachable
- **THEN** `HealthCheck` SHALL return nil

#### Scenario: One source unhealthy
- **WHEN** internal LDAP is healthy but external is unreachable
- **THEN** `HealthCheck` SHALL return `domain.ErrServiceUnavailable`

### Requirement: Repository Close
The `Repository.Close()` SHALL close both pools.

#### Scenario: Graceful shutdown
- **WHEN** `Close()` is called
- **THEN** both internal and external pools SHALL be closed

### Requirement: Compile-time interface checks
The system SHALL include:
- `var _ domain.LDAPPool = (*Pool)(nil)`
- `var _ domain.LDAPRepository = (*Repository)(nil)`

#### Scenario: Interface compliance
- **WHEN** the code compiles
- **THEN** both `Pool` and `Repository` SHALL satisfy their respective interfaces
