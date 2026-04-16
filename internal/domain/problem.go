package domain

import "fmt"

// ---------------------------------------------------------------------------
// RFC 7807 Problem Details
// ---------------------------------------------------------------------------

// Problem represents an RFC 7807 Problem Details response.
// Content-Type for responses using this struct MUST be "application/problem+json".
//
// Acceptance criteria:
//   - MUST implement the error interface (Error() string)
//   - JSON tags MUST be lowercase
//   - Detail and Instance use omitempty — they are optional fields
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Error implements the error interface for Problem.
func (p *Problem) Error() string {
	return fmt.Sprintf("%s: %s", p.Type, p.Title)
}

// ---------------------------------------------------------------------------
// Problem constructors
// ---------------------------------------------------------------------------
//
// Each constructor maps to an error type URI defined in the spec (section 3.1).
// The instance parameter should be the request ID (e.g., "req-<uuid>").

// NewInvalidRequest creates a 400 Problem for JSON parse failures or missing fields.
//
// Acceptance criteria:
//   - Type: "/problems/invalid-request"
//   - Status: 400
//   - detail parameter becomes the Detail field
func NewInvalidRequest(detail, instance string) *Problem {
	return &Problem{
		Type:     "/problems/invalid-request",
		Title:    "Invalid request",
		Status:   400,
		Detail:   detail,
		Instance: instance,
	}
}

// NewInvalidUsername creates a 400 Problem for usernames that fail validation.
//
// Acceptance criteria:
//   - Type: "/problems/invalid-username"
//   - Status: 400
func NewInvalidUsername(detail, instance string) *Problem {
	return &Problem{
		Type:     "/problems/invalid-username",
		Title:    "Invalid username",
		Status:   400,
		Detail:   detail,
		Instance: instance,
	}
}

// NewAttributeNotAllowed creates a 400 Problem for attributes outside the whitelist.
//
// Acceptance criteria:
//   - Type: "/problems/attribute-not-allowed"
//   - Status: 400
func NewAttributeNotAllowed(detail, instance string) *Problem {
	return &Problem{
		Type:     "/problems/attribute-not-allowed",
		Title:    "Attribute not allowed",
		Status:   400,
		Detail:   detail,
		Instance: instance,
	}
}

// NewUnauthorized creates a 401 Problem for missing or invalid API keys.
//
// Acceptance criteria:
//   - Type: "/problems/unauthorized"
//   - Status: 401
func NewUnauthorized(detail, instance string) *Problem {
	return &Problem{
		Type:     "/problems/unauthorized",
		Title:    "Unauthorized",
		Status:   401,
		Detail:   detail,
		Instance: instance,
	}
}

// NewAuthenticationFailed creates a 401 Problem for failed LDAP authentication.
//
// Acceptance criteria:
//   - Type: "/problems/authentication-failed"
//   - Status: 401
//   - Detail MUST always be "authentication failed" — NEVER include the real reason
//   - The detail is hardcoded, not a parameter
func NewAuthenticationFailed(instance string) *Problem {
	return &Problem{
		Type:     "/problems/authentication-failed",
		Title:    "Authentication failed",
		Status:   401,
		Detail:   "authentication failed",
		Instance: instance,
	}
}

// NewNotFound creates a 404 Problem for accounts that don't exist in either LDAP source.
//
// Acceptance criteria:
//   - Type: "/problems/not-found"
//   - Status: 404
func NewNotFound(detail, instance string) *Problem {
	return &Problem{
		Type:     "/problems/not-found",
		Title:    "Account not found",
		Status:   404,
		Detail:   detail,
		Instance: instance,
	}
}

// NewRateLimitExceeded creates a 429 Problem when authenticate rate limit is hit.
//
// Acceptance criteria:
//   - Type: "/problems/rate-limit-exceeded"
//   - Status: 429
//   - Detail MUST be "too many authentication attempts for this username, try again later"
//   - The detail is hardcoded, not a parameter
func NewRateLimitExceeded(instance string) *Problem {
	return &Problem{
		Type:     "/problems/rate-limit-exceeded",
		Title:    "Rate limit exceeded",
		Status:   429,
		Detail:   "too many authentication attempts for this username, try again later",
		Instance: instance,
	}
}

// NewInternalError creates a 500 Problem for unexpected errors.
//
// Acceptance criteria:
//   - Type: "/problems/internal-error"
//   - Status: 500
func NewInternalError(detail, instance string) *Problem {
	return &Problem{
		Type:     "/problems/internal-error",
		Title:    "Internal error",
		Status:   500,
		Detail:   detail,
		Instance: instance,
	}
}

// NewServiceUnavailable creates a 503 Problem when LDAP servers are unreachable.
//
// Acceptance criteria:
//   - Type: "/problems/service-unavailable"
//   - Status: 503
func NewServiceUnavailable(detail, instance string) *Problem {
	return &Problem{
		Type:     "/problems/service-unavailable",
		Title:    "Service unavailable",
		Status:   503,
		Detail:   detail,
		Instance: instance,
	}
}
