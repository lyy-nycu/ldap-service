## ADDED Requirements

### Requirement: LDAPSourceConfig sub-struct
The system SHALL define a `LDAPSourceConfig` struct with fields: `Host` (string), `Port` (int), `UseTLS` (bool), `BindDN` (string), `BindPW` (string), `ConnPoolSize` (int).

#### Scenario: Two independent source configs
- **WHEN** `Config` is loaded
- **THEN** it SHALL contain two `LDAPSourceConfig` instances: `Internal` and `External`, each with independent values

### Requirement: Config struct and loader
The system SHALL define a `Config` struct in `internal/infra/config/config.go` with:
- `Port` (string, default `"8080"`)
- `LDAPBaseDN` (string, required — shared across both sources)
- `Internal` (`LDAPSourceConfig` — internal LDAP server)
- `External` (`LDAPSourceConfig` — external LDAP server)
- `APIKeys` (map[string]string)
- `AuthRateLimit` (int, default 5)
- `AuthRateCleanupMin` (int, default 10)

The system SHALL provide a `Load() (*Config, error)` function that reads from `os.Getenv()`.

#### Scenario: All required variables present
- **WHEN** `LDAP_BASE_DN`, `LDAP_INTERNAL_HOST`, `LDAP_INTERNAL_BIND_DN`, `LDAP_INTERNAL_BIND_PW`, `LDAP_EXTERNAL_HOST`, `LDAP_EXTERNAL_BIND_DN`, `LDAP_EXTERNAL_BIND_PW`, and `API_KEYS` are set
- **THEN** `Load()` SHALL return a valid `Config` with all values populated

#### Scenario: Missing required variable
- **WHEN** `LDAP_INTERNAL_HOST` is not set
- **THEN** `Load()` SHALL return an error naming the missing variable

#### Scenario: Default values applied
- **WHEN** `PORT` is not set
- **THEN** `Config.Port` SHALL be `"8080"`
- **WHEN** `LDAP_INTERNAL_PORT` is not set
- **THEN** `Config.Internal.Port` SHALL be `636`
- **WHEN** `LDAP_INTERNAL_USE_TLS` is not set
- **THEN** `Config.Internal.UseTLS` SHALL be `true`
- **WHEN** `LDAP_INTERNAL_CONN_POOL_SIZE` is not set
- **THEN** `Config.Internal.ConnPoolSize` SHALL be `10`
- **WHEN** `LDAP_EXTERNAL_PORT` is not set
- **THEN** `Config.External.Port` SHALL be `636`
- **WHEN** `LDAP_EXTERNAL_CONN_POOL_SIZE` is not set
- **THEN** `Config.External.ConnPoolSize` SHALL be `5`
- **WHEN** `AUTH_RATE_LIMIT` is not set
- **THEN** `Config.AuthRateLimit` SHALL be `5`

### Requirement: Environment variable naming convention
The system SHALL use these prefixed env var names:

| Variable | Required | Default | Source |
|---|---|---|---|
| `LDAP_BASE_DN` | Yes | — | Shared |
| `LDAP_INTERNAL_HOST` | Yes | — | Internal |
| `LDAP_INTERNAL_PORT` | No | `636` | Internal |
| `LDAP_INTERNAL_USE_TLS` | No | `true` | Internal |
| `LDAP_INTERNAL_BIND_DN` | Yes | — | Internal |
| `LDAP_INTERNAL_BIND_PW` | Yes | — | Internal |
| `LDAP_INTERNAL_CONN_POOL_SIZE` | No | `10` | Internal |
| `LDAP_EXTERNAL_HOST` | Yes | — | External |
| `LDAP_EXTERNAL_PORT` | No | `636` | External |
| `LDAP_EXTERNAL_USE_TLS` | No | `true` | External |
| `LDAP_EXTERNAL_BIND_DN` | Yes | — | External |
| `LDAP_EXTERNAL_BIND_PW` | Yes | — | External |
| `LDAP_EXTERNAL_CONN_POOL_SIZE` | No | `5` | External |

#### Scenario: Internal and external configs are independent
- **WHEN** `LDAP_INTERNAL_HOST=ldap1.nycu.edu.tw` and `LDAP_EXTERNAL_HOST=ldap2.nycu.edu.tw`
- **THEN** `Config.Internal.Host` SHALL be `"ldap1.nycu.edu.tw"` and `Config.External.Host` SHALL be `"ldap2.nycu.edu.tw"`

### Requirement: API Keys parsing
The system SHALL parse `API_KEYS` env var in format `key1:name1,key2:name2` into a map of key→name pairs.

#### Scenario: Multiple API keys
- **WHEN** `API_KEYS` is `"abc123:portal,def456:mfa"`
- **THEN** the parsed result SHALL contain two entries: `"abc123"→"portal"` and `"def456"→"mfa"`

#### Scenario: Invalid format
- **WHEN** `API_KEYS` is `"invalidformat"`
- **THEN** `Load()` SHALL return an error describing the expected format
