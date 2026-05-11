package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// ---------------------------------------------------------------------------
// Source constants
// ---------------------------------------------------------------------------

// SourceInternal identifies accounts from the internal LDAP server
// (student, employee, retire).
const SourceInternal = "internal"

// SourceExternal identifies accounts from the external LDAP server
// (cooperator, alumni).
const SourceExternal = "external"

// ---------------------------------------------------------------------------
// Entities
// ---------------------------------------------------------------------------

// Account represents a user account retrieved from LDAP.
type Account struct {
	DN         string            `json:"dn"`
	UID        string            `json:"uid"`
	Attributes map[string]string `json:"attributes"`
	Source     string            `json:"source"` // SourceInternal or SourceExternal
}

// ---------------------------------------------------------------------------
// Attribute whitelist
// ---------------------------------------------------------------------------

// AllowedAttributes defines the only LDAP attributes that may be queried
// through the API. Any attribute not in this set MUST be rejected with
// ErrAttributeNotAllowed.
var AllowedAttributes = map[string]bool{
	"cn":               true,
	"uid":              true,
	"sn":               true,
	"givenName":        true,
	"fullName":         true,
	"initials":         true,
	"dept":             true,
	"employeeStatus":   true,
	"title":            true,
	"ou":               true,
	"mobile":           true,
	"mail":             true,
	"Alternate-Email":  true,
	"birthday":         true,
	"departmentNumber": true,
	"description":      true,
	"disable":          true,
	"idno":             true,
	"originEmail":      true,
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// usernameRegex matches internal usernames (110550001, T1234) and
// external email-style usernames (user@example.com).
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._@-]{1,128}$`)

// ValidateUsername checks if a username is valid for LDAP lookup.
//
// Acceptance criteria:
//   - MUST match regex ^[a-zA-Z0-9._@-]{1,128}$
//   - MUST return ErrInvalidUsername on failure
//   - Supports internal usernames (110550001, T1234) and external emails (user@example.com)
//   - This is a pure validation function — no LDAP call, no side effects
func ValidateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return ErrInvalidUsername
	}

	return nil
}

// ValidateAttributes checks that every requested attribute is in AllowedAttributes.
//
// Acceptance criteria:
//   - MUST check each attribute against AllowedAttributes
//   - MUST return ErrAttributeNotAllowed on first disallowed attribute
//   - MUST include the offending attribute name in the error message
//   - Empty slice is valid (returns nil)
func ValidateAttributes(attributes []string) error {
	for _, attr := range attributes {
		if !AllowedAttributes[attr] {
			return fmt.Errorf("%w: %s", ErrAttributeNotAllowed, attr)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrInvalidUsername      = errors.New("invalid username")
	ErrBatchSizeExceeded    = errors.New("batch size exceeded")
	ErrAttributeNotAllowed  = errors.New("attribute not allowed")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrServiceUnavailable   = errors.New("service unavailable")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
)

// ---------------------------------------------------------------------------
// Repository interfaces
// ---------------------------------------------------------------------------

// LDAPPool represents a connection pool to a single LDAP server.
// Each server (internal, external) gets its own Pool instance.
// Pool does NOT contain fan-out logic — that belongs in Repository.
type LDAPPool interface {
	// Search finds an account by username on this LDAP server.
	//
	// Acceptance criteria:
	//   - MUST use ldap.EscapeFilter() on username — never string concatenation
	//   - MUST search with base=baseDN, scope=WholeSubtree
	//   - MUST set Account.Source to the pool's source label (SourceInternal or SourceExternal)
	//   - MUST return ErrAccountNotFound if no entry matches
	//   - MUST only fetch the requested attributes (pre-validated by caller)
	//   - MUST propagate context for cancellation
	Search(ctx context.Context, username string, attributes []string) (*Account, error)

	// Bind attempts authentication with the given DN and password.
	//
	// Acceptance criteria:
	//   - MUST create a NEW connection for the bind — do NOT use a pooled connection
	//   - MUST close the connection after the bind attempt
	//   - MUST return nil on successful bind
	//   - MUST return an error on failed bind (wrong password, unreachable, etc.)
	//   - MUST NOT log the password
	Bind(ctx context.Context, dn string, password string) error

	// HealthCheck verifies that this LDAP server is reachable.
	//
	// Acceptance criteria:
	//   - MUST borrow a connection from the pool
	//   - MUST perform a simple search to verify connectivity
	//   - MUST return nil if healthy, error if unreachable
	HealthCheck(ctx context.Context) error

	// Close drains and closes all connections in this pool.
	//
	// Acceptance criteria:
	//   - MUST close all pooled connections
	//   - MUST be safe to call during graceful shutdown
	Close() error
}

// LDAPRepository is the aggregate interface over both LDAP sources.
// It handles fan-out internally — callers do not need to know about
// multiple sources.
type LDAPRepository interface {
	// Lookup finds an account by username across both LDAP sources.
	//
	// Acceptance criteria:
	//   - MUST search internal pool first
	//   - On ErrAccountNotFound from internal, MUST search external pool
	//   - On connection error from internal, MUST log the error and still try external
	//   - MUST return ErrAccountNotFound only if BOTH sources return not found
	//   - MUST return ErrServiceUnavailable if both sources have connection errors
	Lookup(ctx context.Context, username string, attributes []string) (*Account, error)

	// LookupBatch finds multiple accounts across both LDAP sources.
	//
	// Acceptance criteria:
	//   - MUST perform fan-out lookup for each username individually
	//   - Found accounts go in the first return slice
	//   - Not-found usernames go in the second return slice ([]string)
	//   - MUST NOT fail the entire batch if some usernames are not found
	LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*Account, []string, error)

	// Authenticate verifies a user's password via LDAP bind.
	//
	// Acceptance criteria:
	//   - MUST search internal pool first to find the user's DN
	//   - If not found in internal, MUST search external pool
	//   - MUST bind against the SAME pool that found the user
	//   - MUST return (true, nil) on successful bind
	//   - MUST return (false, nil) for all failures (not found, wrong password)
	//   - MUST NOT return (false, error) that reveals the failure reason to callers
	//   - MUST NOT log the password
	Authenticate(ctx context.Context, username string, password string) (bool, error)

	// HealthCheck verifies both LDAP sources are reachable.
	//
	// Acceptance criteria:
	//   - MUST check both internal and external pools
	//   - MUST return nil only if BOTH are healthy
	//   - MUST return ErrServiceUnavailable if either is unhealthy
	HealthCheck(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// Use case interfaces
// ---------------------------------------------------------------------------

// LookupUseCase defines the business logic for account lookups.
// Handlers depend on this interface — never on the concrete implementation.
type LookupUseCase interface {
	// Lookup finds a single account by username.
	//
	// Acceptance criteria:
	//   - MUST validate username with ValidateUsername() before calling repository
	//   - MUST validate attributes with ValidateAttributes() before calling repository
	//   - MUST propagate context for cancellation
	//   - Fan-out is handled by the repository — use case is unaware of multiple sources
	Lookup(ctx context.Context, username string, attributes []string) (*Account, error)

	// LookupBatch finds multiple accounts by username.
	//
	// Acceptance criteria:
	//   - MUST validate batch size: max 50 usernames
	//   - MUST validate each username with ValidateUsername()
	//   - MUST validate attributes with ValidateAttributes()
	//   - MUST return error on first invalid username (fail fast, do not call repository)
	LookupBatch(ctx context.Context, usernames []string, attributes []string) ([]*Account, []string, error)
}

// AuthenticateUseCase defines the business logic for LDAP authentication.
// Handlers depend on this interface — never on the concrete implementation.
type AuthenticateUseCase interface {
	// Authenticate verifies a user's password.
	//
	// Acceptance criteria:
	//   - MUST validate username with ValidateUsername()
	//   - MUST verify password is non-empty
	//   - MUST return (false, ErrAuthenticationFailed) for ALL failure reasons:
	//     invalid username, empty password, user not found, wrong password
	//   - MUST NOT reveal the specific failure reason in error or log
	//   - MUST NOT log the password
	Authenticate(ctx context.Context, username string, password string) (bool, error)
}
