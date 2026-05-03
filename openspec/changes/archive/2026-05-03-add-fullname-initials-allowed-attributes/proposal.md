## Why

Some consumers need human-readable identity fields for LDAP lookup responses, but the current whitelist does not include dedicated `fullname` and `initials` attributes. Adding these attributes to the allowed lookup set avoids ad hoc field mapping outside the service and keeps caller behavior consistent with existing whitelist validation.

## What Changes

- Extend the LDAP lookup attribute whitelist to accept `fullname` and `initials`.
- Treat `fullname` and `initials` as native on-prem LDAP attributes (no alias mapping from other fields).
- Ensure lookup and batch lookup flows validate and pass these attributes through like existing allowed fields.
- Update tests and documentation so the new allowed attributes are covered in validation and endpoint examples.

## Capabilities

### New Capabilities
- `ldap-attribute-whitelist-extension`: allow additional identity attributes (`fullname`, `initials`) in lookup attribute validation and retrieval.

### Modified Capabilities
- None.

## Impact

- `internal/domain/domain.go`: update `AllowedAttributes` whitelist and validation expectations.
- Lookup path tests in `internal/domain/` and `internal/handler/`: include new allowed attributes in coverage.
- Service documentation: reflect expanded allowed attribute list.
