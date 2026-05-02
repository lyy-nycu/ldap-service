## Why

This service currently has a minimal top-level README, which makes it hard for new integrators to quickly understand what functions it provides and how each endpoint should be used. A concise, accurate service guide will reduce onboarding time and integration mistakes.

## What Changes

- Add a service guide section describing the business purpose of each endpoint.
- Document request and response formats for health, lookup, batch lookup, and authenticate APIs.
- Document security and behavior expectations, including API key requirements and generic authentication failure behavior.
- Add quick start examples so consumers can verify integration rapidly.

## Capabilities

### New Capabilities
- `service-guide-documentation`: provide authoritative human-readable documentation for service functions, endpoint meaning, and usage examples.

### Modified Capabilities
- None.

## Impact

- `README`: add complete service guide content.
- API consumers: clearer integration contract for endpoint semantics.
- Operations/support: faster triage and reduced confusion around endpoint behavior.
