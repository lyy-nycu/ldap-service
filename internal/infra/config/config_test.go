package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		want        *Config // nil means "just check error"
		wantErr     bool
		errContains string
	}{
		{
			name: "all required vars present with defaults",
			envVars: map[string]string{
				"LDAP_BASE_DN":          "o=nycu",
				"LDAP_INTERNAL_HOST":    "ldap1.nycu.edu.tw",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly,o=nycu",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_HOST":    "ldap2.nycu.edu.tw",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly,o=nycu",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              "key1:portal,key2:mfa",
			},
			wantErr: false,
			// Acceptance criteria for defaults:
			//   Port="8080", Internal.Port=636, Internal.UseTLS=true,
			//   Internal.ConnPoolSize=10, External.Port=636, External.UseTLS=true,
			//   External.ConnPoolSize=5, AuthRateLimit=5, AuthRateCleanupMin=10
		},
		{
			name: "missing LDAP_BASE_DN",
			envVars: map[string]string{
				"LDAP_INTERNAL_HOST":    "ldap1",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_HOST":    "ldap2",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              "key1:portal",
			},
			wantErr: true,
		},
		{
			name: "missing LDAP_EXTERNAL_HOST",
			envVars: map[string]string{
				"LDAP_BASE_DN":          "o=nycu",
				"LDAP_INTERNAL_HOST":    "ldap1",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              "key1:portal",
			},
			wantErr: true,
		},
		{
			name: "invalid API_KEYS format",
			envVars: map[string]string{
				"LDAP_BASE_DN":          "o=nycu",
				"LDAP_INTERNAL_HOST":    "ldap1",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_HOST":    "ldap2",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              "invalidformat",
			},
			wantErr:     true,
			errContains: "invalid api_keys format",
		},
		{
			name: "API_KEYS empty key",
			envVars: map[string]string{
				"LDAP_BASE_DN":          "o=nycu",
				"LDAP_INTERNAL_HOST":    "ldap1",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_HOST":    "ldap2",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              ":portal",
			},
			wantErr:     true,
			errContains: "invalid api_keys format",
		},
		{
			name: "API_KEYS empty service value",
			envVars: map[string]string{
				"LDAP_BASE_DN":          "o=nycu",
				"LDAP_INTERNAL_HOST":    "ldap1",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_HOST":    "ldap2",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              "key1:",
			},
			wantErr:     true,
			errContains: "invalid api_keys format",
		},
		{
			name: "invalid LDAP_INTERNAL_PORT integer",
			envVars: map[string]string{
				"LDAP_BASE_DN":          "o=nycu",
				"LDAP_INTERNAL_HOST":    "ldap1",
				"LDAP_INTERNAL_PORT":    "not-a-number",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_HOST":    "ldap2",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              "key1:portal",
			},
			wantErr:     true,
			errContains: "invalid integer value for LDAP_INTERNAL_PORT",
		},
		{
			name: "invalid AUTH_RATE_LIMIT integer",
			envVars: map[string]string{
				"LDAP_BASE_DN":          "o=nycu",
				"LDAP_INTERNAL_HOST":    "ldap1",
				"LDAP_INTERNAL_BIND_DN": "cn=readonly",
				"LDAP_INTERNAL_BIND_PW": "secret1",
				"LDAP_EXTERNAL_HOST":    "ldap2",
				"LDAP_EXTERNAL_BIND_DN": "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW": "secret2",
				"API_KEYS":              "key1:portal",
				"AUTH_RATE_LIMIT":       "NaN",
			},
			wantErr:     true,
			errContains: "invalid integer value for AUTH_RATE_LIMIT",
		},
		{
			name: "custom pool sizes and rate limit",
			envVars: map[string]string{
				"LDAP_BASE_DN":                 "o=nycu",
				"LDAP_INTERNAL_HOST":           "ldap1",
				"LDAP_INTERNAL_BIND_DN":        "cn=readonly",
				"LDAP_INTERNAL_BIND_PW":        "secret1",
				"LDAP_INTERNAL_CONN_POOL_SIZE": "20",
				"LDAP_EXTERNAL_HOST":           "ldap2",
				"LDAP_EXTERNAL_BIND_DN":        "cn=readonly",
				"LDAP_EXTERNAL_BIND_PW":        "secret2",
				"LDAP_EXTERNAL_CONN_POOL_SIZE": "3",
				"API_KEYS":                     "key1:portal",
				"AUTH_RATE_LIMIT":              "10",
				"AUTH_RATE_CLEANUP_MIN":        "5",
			},
			wantErr: false,
			// Acceptance criteria:
			//   Internal.ConnPoolSize=20, External.ConnPoolSize=3,
			//   AuthRateLimit=10, AuthRateCleanupMin=5
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Acceptance criteria:
			//   - Set env vars from tt.envVars using t.Setenv (auto-cleaned up)
			//   - Call Load()
			//   - If wantErr: assert error is non-nil
			//   - If !wantErr: assert error is nil, validate Config fields match expected defaults/values
			allVars := []string{
				"PORT",
				"LDAP_BASE_DN",
				"LDAP_INTERNAL_HOST",
				"LDAP_INTERNAL_PORT",
				"LDAP_INTERNAL_USE_TLS",
				"LDAP_INTERNAL_BIND_DN",
				"LDAP_INTERNAL_BIND_PW",
				"LDAP_INTERNAL_CONN_POOL_SIZE",
				"LDAP_EXTERNAL_HOST",
				"LDAP_EXTERNAL_PORT",
				"LDAP_EXTERNAL_USE_TLS",
				"LDAP_EXTERNAL_BIND_DN",
				"LDAP_EXTERNAL_BIND_PW",
				"LDAP_EXTERNAL_CONN_POOL_SIZE",
				"API_KEYS",
				"AUTH_RATE_LIMIT",
				"AUTH_RATE_CLEANUP_MIN",
			}

			for _, k := range allVars {
				t.Setenv(k, "")
			}
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() error = nil, want non-nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("Load() error = %q, want to contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}

			if cfg == nil {
				t.Fatal("Load() returned nil config")
			}

			if cfg.LDAPBaseDN != tt.envVars["LDAP_BASE_DN"] {
				t.Fatalf("LDAPBaseDN = %q, want %q", cfg.LDAPBaseDN, tt.envVars["LDAP_BASE_DN"])
			}
			if cfg.Internal.Host != tt.envVars["LDAP_INTERNAL_HOST"] {
				t.Fatalf("Internal.Host = %q, want %q", cfg.Internal.Host, tt.envVars["LDAP_INTERNAL_HOST"])
			}
			if cfg.Internal.BindDN != tt.envVars["LDAP_INTERNAL_BIND_DN"] {
				t.Fatalf("Internal.BindDN = %q, want %q", cfg.Internal.BindDN, tt.envVars["LDAP_INTERNAL_BIND_DN"])
			}
			if cfg.Internal.BindPW != tt.envVars["LDAP_INTERNAL_BIND_PW"] {
				t.Fatalf("Internal.BindPW = %q, want %q", cfg.Internal.BindPW, tt.envVars["LDAP_INTERNAL_BIND_PW"])
			}
			if cfg.External.Host != tt.envVars["LDAP_EXTERNAL_HOST"] {
				t.Fatalf("External.Host = %q, want %q", cfg.External.Host, tt.envVars["LDAP_EXTERNAL_HOST"])
			}
			if cfg.External.BindDN != tt.envVars["LDAP_EXTERNAL_BIND_DN"] {
				t.Fatalf("External.BindDN = %q, want %q", cfg.External.BindDN, tt.envVars["LDAP_EXTERNAL_BIND_DN"])
			}
			if cfg.External.BindPW != tt.envVars["LDAP_EXTERNAL_BIND_PW"] {
				t.Fatalf("External.BindPW = %q, want %q", cfg.External.BindPW, tt.envVars["LDAP_EXTERNAL_BIND_PW"])
			}

			switch tt.name {
			case "all required vars present with defaults":
				if cfg.Port != "8080" {
					t.Fatalf("Port = %q, want %q", cfg.Port, "8080")
				}
				if cfg.Internal.Port != 636 || !cfg.Internal.UseTLS || cfg.Internal.ConnPoolSize != 10 {
					t.Fatalf("internal defaults mismatch: got port=%d useTLS=%v pool=%d", cfg.Internal.Port, cfg.Internal.UseTLS, cfg.Internal.ConnPoolSize)
				}
				if cfg.External.Port != 636 || !cfg.External.UseTLS || cfg.External.ConnPoolSize != 5 {
					t.Fatalf("external defaults mismatch: got port=%d useTLS=%v pool=%d", cfg.External.Port, cfg.External.UseTLS, cfg.External.ConnPoolSize)
				}
				if cfg.AuthRateLimit != 5 || cfg.AuthRateCleanupMin != 10 {
					t.Fatalf("rate defaults mismatch: got limit=%d cleanup=%d", cfg.AuthRateLimit, cfg.AuthRateCleanupMin)
				}
				if len(cfg.APIKeys) != 2 || cfg.APIKeys["key1"] != "portal" || cfg.APIKeys["key2"] != "mfa" {
					t.Fatalf("APIKeys mismatch: got %#v", cfg.APIKeys)
				}

			case "custom pool sizes and rate limit":
				if cfg.Internal.ConnPoolSize != 20 || cfg.External.ConnPoolSize != 3 {
					t.Fatalf("pool sizes mismatch: got internal=%d external=%d", cfg.Internal.ConnPoolSize, cfg.External.ConnPoolSize)
				}
				if cfg.AuthRateLimit != 10 || cfg.AuthRateCleanupMin != 5 {
					t.Fatalf("rate settings mismatch: got limit=%d cleanup=%d", cfg.AuthRateLimit, cfg.AuthRateCleanupMin)
				}
				if cfg.APIKeys["key1"] != "portal" {
					t.Fatalf("APIKeys[key1] = %q, want %q", cfg.APIKeys["key1"], "portal")
				}
			}
		})
	}
}
