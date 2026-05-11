package domain

import (
	"errors"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid — internal usernames
		{name: "student number", input: "110550001", wantErr: false},
		{name: "employee number", input: "T1234", wantErr: false},
		{name: "dotted username", input: "john.doe", wantErr: false},
		{name: "hyphenated username", input: "john-doe", wantErr: false},
		{name: "underscore username", input: "john_doe", wantErr: false},

		// Valid — external usernames (email-style)
		{name: "email username", input: "user@example.com", wantErr: false},
		{name: "email with dots", input: "first.last@nycu.edu.tw", wantErr: false},

		// Invalid — basic
		{name: "empty string", input: "", wantErr: true},
		{name: "spaces", input: "user name", wantErr: true},
		{name: "semicolon", input: "user;drop", wantErr: true},
		{name: "too long (129 chars)", input: string(make([]byte, 129)), wantErr: true},

		// Invalid — LDAP injection attacks
		{name: "LDAP injection parentheses", input: "user)(uid=*)", wantErr: true},
		{name: "LDAP injection asterisk wildcard", input: "user*", wantErr: true},
		{name: "LDAP injection OR filter", input: "user)(|(uid=*)", wantErr: true},
		{name: "LDAP injection AND filter", input: "user)(&(uid=admin)", wantErr: true},
		{name: "LDAP injection backslash escape", input: "user\\00", wantErr: true},
		{name: "LDAP injection null byte", input: "user\x00admin", wantErr: true},
		{name: "LDAP injection DN traversal", input: "uid=admin,ou=employee,o=nycu", wantErr: true},
		{name: "LDAP injection encoded equals", input: "user%3dadmin", wantErr: true},

		// Invalid — encoding attacks
		{name: "null byte mid-string", input: "valid\x00", wantErr: true},
		{name: "tab character", input: "user\tadmin", wantErr: true},
		{name: "newline character", input: "user\nadmin", wantErr: true},
		{name: "carriage return", input: "user\radmin", wantErr: true},

		// Invalid — special characters
		{name: "hash symbol", input: "user#1", wantErr: true},
		{name: "plus sign", input: "user+tag@example.com", wantErr: true},
		{name: "angle brackets", input: "<script>alert(1)</script>", wantErr: true},
		{name: "single quotes", input: "user'OR'1'='1", wantErr: true},
		{name: "double quotes", input: `user"admin`, wantErr: true},
		{name: "exclamation NOT filter", input: "user)(!(uid=*))", wantErr: true},

		// Boundary — length
		{name: "max length (128 chars)", input: string(makeValidBytes(128)), wantErr: false},
		{name: "max length email", input: string(makeValidBytes(64)) + "@" + string(makeValidBytes(63)), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - nil error for valid usernames
			//   - ErrInvalidUsername for invalid usernames
			err := ValidateUsername(tt.input)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidUsername) {
					t.Fatalf("ValidateUsername(%q) error = %v, want ErrInvalidUsername", tt.input, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateUsername(%q) error = %v, want nil", tt.input, err)
			}
		})
	}
}

func TestValidateAttributes(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{name: "single allowed attribute", input: []string{"mail"}, wantErr: false},
		{name: "multiple allowed attributes", input: []string{"mail", "mobile", "dept"}, wantErr: false},
		{name: "all allowed attributes", input: []string{
			"cn", "uid", "sn", "givenName", "fullName", "initials", "dept", "employeeStatus",
			"title", "ou", "mobile", "mail", "Alternate-Email", "birthday", "departmentNumber",
			"description", "disable", "idno", "originEmail",
		}, wantErr: false},
		{name: "fullName attribute", input: []string{"fullName"}, wantErr: false},
		{name: "initials attribute", input: []string{"initials"}, wantErr: false},
		{name: "hyphenated attribute", input: []string{"Alternate-Email"}, wantErr: false},
		{name: "birthday attribute", input: []string{"birthday"}, wantErr: false},
		{name: "departmentNumber attribute", input: []string{"departmentNumber"}, wantErr: false},
		{name: "description attribute", input: []string{"description"}, wantErr: false},
		{name: "disable attribute", input: []string{"disable"}, wantErr: false},
		{name: "idno attribute", input: []string{"idno"}, wantErr: false},
		{name: "originEmail attribute", input: []string{"originEmail"}, wantErr: false},
		{name: "empty slice", input: []string{}, wantErr: false},

		// Invalid — sensitive attributes
		{name: "userPassword blocked", input: []string{"userPassword"}, wantErr: true},
		{name: "temppassword blocked", input: []string{"temppassword"}, wantErr: true},
		{name: "objectClass blocked", input: []string{"objectClass"}, wantErr: true},
		{name: "userCertificate blocked", input: []string{"userCertificate"}, wantErr: true},
		{name: "fullname lowercase rejected", input: []string{"fullname"}, wantErr: true},
		{name: "alternative-mail old name rejected", input: []string{"alternative-mail"}, wantErr: true},
		{name: "deptCode removed", input: []string{"deptCode"}, wantErr: true},

		// Invalid — mixed valid and invalid
		{name: "mixed valid and invalid", input: []string{"mail", "userPassword"}, wantErr: true},
		{name: "valid then sensitive last", input: []string{"mail", "mobile", "objectClass"}, wantErr: true},

		// Invalid — injection via attribute names
		{name: "wildcard attribute", input: []string{"*"}, wantErr: true},
		{name: "nonexistent attribute", input: []string{"foo"}, wantErr: true},
		{name: "empty string attribute", input: []string{""}, wantErr: true},
		{name: "attribute with spaces", input: []string{"mail address"}, wantErr: true},
		{name: "LDAP operational attribute", input: []string{"createTimestamp"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - nil error for valid attribute lists
			//   - ErrAttributeNotAllowed for any disallowed attribute
			err := ValidateAttributes(tt.input)
			if tt.wantErr {
				if !errors.Is(err, ErrAttributeNotAllowed) {
					t.Fatalf("ValidateAttributes(%v) error = %v, want ErrAttributeNotAllowed", tt.input, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateAttributes(%v) error = %v, want nil", tt.input, err)
			}
		})
	}
}

// makeValidBytes creates a byte slice of length n filled with valid username characters.
func makeValidBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return b
}
