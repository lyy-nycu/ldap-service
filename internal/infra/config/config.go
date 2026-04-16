package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Config types
// ---------------------------------------------------------------------------

// LDAPSourceConfig holds connection settings for a single LDAP server.
type LDAPSourceConfig struct {
	Host         string
	Port         int
	UseTLS       bool
	BindDN       string
	BindPW       string
	ConnPoolSize int
}

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port               string
	LDAPBaseDN         string
	Internal           LDAPSourceConfig
	External           LDAPSourceConfig
	APIKeys            map[string]string // key → service name
	AuthRateLimit      int
	AuthRateCleanupMin int
}

// ---------------------------------------------------------------------------
// Loader
// ---------------------------------------------------------------------------

// Load reads configuration from environment variables and returns a validated Config.
//
// Acceptance criteria:
//   - MUST read all env vars listed in .env.example
//   - MUST return error naming the specific missing variable if a required var is not set
//   - Required vars: LDAP_BASE_DN, LDAP_INTERNAL_HOST, LDAP_INTERNAL_BIND_DN,
//     LDAP_INTERNAL_BIND_PW, LDAP_EXTERNAL_HOST, LDAP_EXTERNAL_BIND_DN,
//     LDAP_EXTERNAL_BIND_PW, API_KEYS
//   - Defaults: PORT="8080", LDAP_INTERNAL_PORT=636, LDAP_INTERNAL_USE_TLS=true,
//     LDAP_INTERNAL_CONN_POOL_SIZE=10, LDAP_EXTERNAL_PORT=636, LDAP_EXTERNAL_USE_TLS=true,
//     LDAP_EXTERNAL_CONN_POOL_SIZE=5, AUTH_RATE_LIMIT=5, AUTH_RATE_CLEANUP_MIN=10
//   - MUST parse API_KEYS in format "key1:name1,key2:name2" into map[string]string
//   - MUST return error if API_KEYS format is invalid
//   - MUST convert string port/pool size values to int
func Load() (*Config, error) {
	requiredVars := []string{
		"LDAP_BASE_DN",
		"LDAP_INTERNAL_HOST",
		"LDAP_INTERNAL_BIND_DN",
		"LDAP_INTERNAL_BIND_PW",
		"LDAP_EXTERNAL_HOST",
		"LDAP_EXTERNAL_BIND_DN",
		"LDAP_EXTERNAL_BIND_PW",
		"API_KEYS",
	}

	for _, name := range requiredVars {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			return nil, fmt.Errorf("missing required env var: %s", name)
		}
	}

	parseInt := func(name string, defaultValue int) (int, error) {
		raw := strings.TrimSpace(os.Getenv(name))
		if raw == "" {
			return defaultValue, nil
		}

		v, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("invalid integer value for %s", name)
		}

		return v, nil
	}

	parseBool := func(name string, defaultValue bool) (bool, error) {
		raw := strings.TrimSpace(os.Getenv(name))
		if raw == "" {
			return defaultValue, nil
		}

		v, err := strconv.ParseBool(raw)
		if err != nil {
			return false, fmt.Errorf("invalid boolean value for %s", name)
		}

		return v, nil
	}

	parseAPIKeys := func(raw string) (map[string]string, error) {
		result := make(map[string]string)
		entries := strings.Split(raw, ",")
		for _, entry := range entries {
			item := strings.TrimSpace(entry)
			parts := strings.Split(item, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid api_keys format")
			}

			key := strings.TrimSpace(parts[0])
			service := strings.TrimSpace(parts[1])
			if key == "" || service == "" {
				return nil, fmt.Errorf("invalid api_keys format")
			}

			result[key] = service
		}

		if len(result) == 0 {
			return nil, fmt.Errorf("invalid api_keys format")
		}

		return result, nil
	}

	internalPort, err := parseInt("LDAP_INTERNAL_PORT", 636)
	if err != nil {
		return nil, err
	}

	internalUseTLS, err := parseBool("LDAP_INTERNAL_USE_TLS", true)
	if err != nil {
		return nil, err
	}

	internalPoolSize, err := parseInt("LDAP_INTERNAL_CONN_POOL_SIZE", 10)
	if err != nil {
		return nil, err
	}

	externalPort, err := parseInt("LDAP_EXTERNAL_PORT", 636)
	if err != nil {
		return nil, err
	}

	externalUseTLS, err := parseBool("LDAP_EXTERNAL_USE_TLS", true)
	if err != nil {
		return nil, err
	}

	externalPoolSize, err := parseInt("LDAP_EXTERNAL_CONN_POOL_SIZE", 5)
	if err != nil {
		return nil, err
	}

	authRateLimit, err := parseInt("AUTH_RATE_LIMIT", 5)
	if err != nil {
		return nil, err
	}

	authRateCleanupMin, err := parseInt("AUTH_RATE_CLEANUP_MIN", 10)
	if err != nil {
		return nil, err
	}

	apiKeys, err := parseAPIKeys(os.Getenv("API_KEYS"))
	if err != nil {
		return nil, err
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	cfg := &Config{
		Port:       port,
		LDAPBaseDN: strings.TrimSpace(os.Getenv("LDAP_BASE_DN")),
		Internal: LDAPSourceConfig{
			Host:         strings.TrimSpace(os.Getenv("LDAP_INTERNAL_HOST")),
			Port:         internalPort,
			UseTLS:       internalUseTLS,
			BindDN:       strings.TrimSpace(os.Getenv("LDAP_INTERNAL_BIND_DN")),
			BindPW:       strings.TrimSpace(os.Getenv("LDAP_INTERNAL_BIND_PW")),
			ConnPoolSize: internalPoolSize,
		},
		External: LDAPSourceConfig{
			Host:         strings.TrimSpace(os.Getenv("LDAP_EXTERNAL_HOST")),
			Port:         externalPort,
			UseTLS:       externalUseTLS,
			BindDN:       strings.TrimSpace(os.Getenv("LDAP_EXTERNAL_BIND_DN")),
			BindPW:       strings.TrimSpace(os.Getenv("LDAP_EXTERNAL_BIND_PW")),
			ConnPoolSize: externalPoolSize,
		},
		APIKeys:            apiKeys,
		AuthRateLimit:      authRateLimit,
		AuthRateCleanupMin: authRateCleanupMin,
	}

	return cfg, nil
}
