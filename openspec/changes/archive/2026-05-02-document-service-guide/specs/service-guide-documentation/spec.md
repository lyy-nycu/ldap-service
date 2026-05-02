## ADDED Requirements

### Requirement: Service guide explains endpoint purpose and function meaning
The documentation SHALL describe the meaning and intended use of each public endpoint, including liveness, readiness, lookup, batch lookup, and authenticate.

#### Scenario: Reader needs endpoint purpose
- **WHEN** a developer opens the service guide
- **THEN** the guide MUST explain what each endpoint is for in plain language
- **AND** the mapping between endpoint path and function meaning MUST be explicit

### Requirement: Service guide defines request and response contracts
The documentation SHALL include request and response examples for each API endpoint so integrators can implement clients without reading source code.

#### Scenario: Reader implements client calls
- **WHEN** a developer follows the endpoint sections in the guide
- **THEN** they MUST find request body examples for POST endpoints
- **AND** they MUST find representative success and error response examples

### Requirement: Service guide documents security and usage expectations
The documentation SHALL define API key requirements, endpoint-level protection behavior, and authentication failure semantics.

#### Scenario: Reader configures secure integration
- **WHEN** a developer reads the security section
- **THEN** the guide MUST state that `/api/v1/ldap/*` endpoints require `X-Api-Key`
- **AND** the guide MUST state that `/healthz` and `/readyz` do not require API keys
- **AND** the guide MUST state that authentication failures are generic and do not reveal user existence

### Requirement: Service guide includes quick-start validation steps
The documentation SHALL provide executable examples that allow consumers to quickly verify successful connectivity and endpoint behavior.

#### Scenario: Reader validates integration quickly
- **WHEN** a developer follows quick-start examples
- **THEN** they MUST be able to call health and API endpoints with sample commands
- **AND** they MUST be able to recognize expected outputs from those examples
