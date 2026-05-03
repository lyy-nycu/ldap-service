## ADDED Requirements

### Requirement: JSON response helper
The system SHALL implement a `RespondJSON(w http.ResponseWriter, status int, body any)` function in `internal/handler/response.go` that sets `Content-Type: application/json` and writes the JSON-encoded body.

#### Scenario: Successful JSON response
- **WHEN** `RespondJSON` is called with status 200 and a struct
- **THEN** the response SHALL have `Content-Type: application/json` header and the JSON-encoded body

### Requirement: Problem response helper
The system SHALL implement a `RespondProblem(w http.ResponseWriter, p *domain.Problem)` function that sets `Content-Type: application/problem+json` and writes the Problem as JSON with the Problem's status code.

#### Scenario: Problem response
- **WHEN** `RespondProblem` is called with a 400 Problem
- **THEN** the response SHALL have status 400, `Content-Type: application/problem+json`, and the Problem JSON body

### Requirement: Liveness handler (GET /healthz)
The system SHALL respond with `200 OK` and `{"status":"ok"}`. No LDAP check.

#### Scenario: Liveness probe
- **WHEN** `GET /healthz` is called
- **THEN** the response SHALL be 200 with body `{"status":"ok"}`

### Requirement: Readiness handler (GET /readyz)
The system SHALL call `LDAPRepository.HealthCheck()` which checks both LDAP sources. It SHALL respond with `200 {"status":"ready"}` if both sources are healthy, or `503` with RFC 7807 Problem if either is unhealthy.

#### Scenario: Both LDAP sources healthy
- **WHEN** `GET /readyz` is called and both internal and external LDAP are reachable
- **THEN** the response SHALL be 200 with body `{"status":"ready"}`

#### Scenario: One LDAP source unhealthy
- **WHEN** `GET /readyz` is called and external LDAP is unreachable
- **THEN** the response SHALL be 503 with Problem type `/problems/service-unavailable`

#### Scenario: Both LDAP sources unhealthy
- **WHEN** `GET /readyz` is called and both LDAP servers are unreachable
- **THEN** the response SHALL be 503 with Problem type `/problems/service-unavailable`

### Requirement: Lookup handler (POST /api/v1/ldap/lookup)
The system SHALL parse the JSON request body with fields `username` (string, required) and `attributes` ([]string, required). It SHALL call `LookupUseCase.Lookup()` and respond with `200` containing `dn`, `uid`, `source`, and `attributes` fields.

#### Scenario: Successful lookup
- **WHEN** valid request with existing username
- **THEN** response SHALL be 200 with `{"dn":"...","uid":"...","source":"internal"|"external","attributes":{...}}`

#### Scenario: Invalid JSON body
- **WHEN** request body is not valid JSON
- **THEN** response SHALL be 400 with Problem type `/problems/invalid-request`

#### Scenario: Missing required field
- **WHEN** request body lacks `username` field
- **THEN** response SHALL be 400 with Problem type `/problems/invalid-request`

#### Scenario: Account not found
- **WHEN** use case returns `domain.ErrAccountNotFound`
- **THEN** response SHALL be 404 with Problem type `/problems/not-found`

### Requirement: Batch lookup handler (POST /api/v1/ldap/lookup/batch)
The system SHALL parse the JSON request body with fields `usernames` ([]string, required) and `attributes` ([]string, required). It SHALL call `LookupUseCase.LookupBatch()` and respond with `200` containing `accounts` array (each with `source` field) and `not_found` array.

#### Scenario: Successful batch lookup
- **WHEN** valid request with mix of existing and nonexistent usernames across both LDAP sources
- **THEN** response SHALL be 200 with `{"accounts":[...],"not_found":[...]}`

#### Scenario: Batch size exceeded
- **WHEN** `usernames` array has more than 50 entries
- **THEN** response SHALL be 400 with Problem type `/problems/invalid-request`

### Requirement: Authenticate handler (POST /api/v1/ldap/authenticate)
The system SHALL parse the JSON request body with fields `username` (string, required) and `password` (string, required). It SHALL call `AuthenticateUseCase.Authenticate()` and respond with `200 {"authenticated":true}` on success or `401` with Problem type `/problems/authentication-failed` on failure.

#### Scenario: Successful authentication
- **WHEN** valid credentials are provided (either internal or external user)
- **THEN** response SHALL be 200 with `{"authenticated":true}`

#### Scenario: Failed authentication
- **WHEN** authentication fails for any reason
- **THEN** response SHALL be 401 with Problem type `/problems/authentication-failed` and detail `"authentication failed"`

#### Scenario: Rate limited
- **WHEN** the rate limit middleware has already rejected the request
- **THEN** the handler SHALL NOT be reached (middleware handles 429 response)

### Requirement: Method not allowed
All handlers SHALL only accept their specified HTTP method. Other methods SHALL receive `405 Method Not Allowed`.

#### Scenario: GET on lookup endpoint
- **WHEN** `GET /api/v1/ldap/lookup` is called
- **THEN** the response SHALL be 405
