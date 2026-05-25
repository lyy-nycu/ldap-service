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
// Authenticate result types
// ---------------------------------------------------------------------------

// AccountState describes the post-bind status of an account. Returned in the
// 200 body of POST /api/v1/ldap/authenticate so callers (e.g. portal-backend)
// can map LDAP truth to their own HTTP policy (401 vs 403).
//
// IMPORTANT: this value is only ever populated when the bind itself
// succeeded. A bind failure produces NO result and is reported as 401 with
// the standard ErrAuthenticationFailed problem — see the no-enumeration
// invariant on AuthenticateUseCase.Authenticate.
type AccountState string

const (
	AccountStateActive             AccountState = "active"
	AccountStateDisabled           AccountState = "disabled"
	AccountStatePendingActivation  AccountState = "pending_activation"
	AccountStateLocked             AccountState = "locked"
)

// PasswordState describes the post-bind status of the user's password.
// Same emission rules as AccountState — only returned alongside a 200.
type PasswordState string

const (
	PasswordStateCurrent    PasswordState = "current"
	PasswordStateExpired    PasswordState = "expired"
	PasswordStateMustChange PasswordState = "must_change"
)

// AuthenticateResult is the success-path payload of an authenticate call.
// It is ONLY returned when the LDAP bind succeeded. The HTTP layer
// serializes the public fields as the 200 response body.
type AuthenticateResult struct {
	UID           string        `json:"user_id"`
	AccountState  AccountState  `json:"account_state"`
	PasswordState PasswordState `json:"password_state"`
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

	// Modify atomically replaces the given attributes on the subject
	// identified by username on this LDAP server.
	//
	// Acceptance criteria:
	//   - MUST first resolve subjectID → DN via the same filter Search uses
	//     (cn=<EscapeFilter(subjectID)>), scope = WholeSubtree, base = baseDN
	//   - MUST return ErrAccountNotFound if no entry matches
	//   - MUST issue ONE upstream `ldap.Modify` call (atomicity invariant)
	//   - The Modify request MUST contain exactly one `replace` op per
	//     entry in attrs, using attr.Name verbatim (no key rewriting —
	//     in particular "altemate-email" must reach LDAP as-is)
	//   - MUST use the borrowed pool connection (read-only bind is fine
	//     for modify if the pool's bindDN has write ACLs)
	//   - MUST propagate context for cancellation
	//   - MUST NOT log any attribute value (passwords pass through here)
	//   - On LDAP error: return ErrSchemaViolation if the upstream code
	//     is constraint/schema/object-class related; otherwise the raw
	//     transport error so Repository can decide on ErrServiceUnavailable
	Modify(ctx context.Context, subjectID string, attrs []ModifyAttr) error

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

	// Authenticate verifies a user's password via LDAP bind and reports the
	// account / password state of the bound user.
	//
	// Acceptance criteria:
	//   - MUST search internal pool first to find the user's DN
	//   - If not found in internal, MUST search external pool
	//   - MUST bind against the SAME pool that found the user
	//   - On successful bind: MUST return (*AuthenticateResult, nil) with
	//     AccountState and PasswordState derived from the LDAP entry's
	//     `disable`, `pwdReset` and `pwdChangedTime` attributes
	//   - MUST return (nil, nil) for ALL failure cases (not found, wrong
	//     password, both sources unavailable). The nil result is the sole
	//     failure signal — never an error that reveals the reason
	//   - MUST NOT log the password
	//   - MUST request the state attributes via Search; these attributes
	//     are NOT part of AllowedAttributes (do not leak via Lookup)
	Authenticate(ctx context.Context, username string, password string) (*AuthenticateResult, error)

	// HealthCheck verifies both LDAP sources are reachable.
	//
	// Acceptance criteria:
	//   - MUST check both internal and external pools
	//   - MUST return nil only if BOTH are healthy
	//   - MUST return ErrServiceUnavailable if either is unhealthy
	HealthCheck(ctx context.Context) error

	// Modify atomically replaces the given attributes on the subject
	// identified by subjectID, fanning out across both LDAP sources.
	//
	// Acceptance criteria:
	//   - MUST try internal pool first (search-then-modify on same pool)
	//   - On ErrAccountNotFound from internal, MUST try external pool
	//   - MUST return ErrAccountNotFound only if BOTH sources return not found
	//   - On connection error from internal, MUST log and still try external
	//   - On ErrSchemaViolation, MUST propagate it (do NOT fall over to
	//     the other pool — schema rejection is authoritative)
	//   - MUST NOT log attribute values (passwords pass through here)
	Modify(ctx context.Context, subjectID string, attrs []ModifyAttr) error
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
	// Authenticate verifies a user's password and returns the post-bind
	// account / password state.
	//
	// Acceptance criteria:
	//   - MUST validate username with ValidateUsername()
	//   - MUST verify password is non-empty
	//   - On success: MUST return (*AuthenticateResult, nil)
	//   - On ANY failure (invalid username, empty password, user not found,
	//     wrong password, repository error): MUST return
	//     (nil, ErrAuthenticationFailed)
	//   - MUST NOT reveal the specific failure reason in error or log
	//   - MUST NOT log the password
	//   - The result.AccountState / PasswordState are policy-neutral here;
	//     the HTTP layer surfaces them to the caller verbatim
	Authenticate(ctx context.Context, username string, password string) (*AuthenticateResult, error)
}
