package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

// ctxKeyServiceName is the context key for the authenticated service name.
type ctxKeyServiceName struct{}

// ServiceNameFromContext extracts the service name from context.
func ServiceNameFromContext(ctx context.Context) string {
	if name, ok := ctx.Value(ctxKeyServiceName{}).(string); ok {
		return name
	}
	return ""
}

// APIKey is a middleware that validates the X-Api-Key header.
//
// Acceptance criteria:
//   - MUST read the X-Api-Key header
//   - MUST compare against each configured key using crypto/subtle.ConstantTimeCompare()
//     — NEVER use == for key comparison
//   - If key matches: set service name in context via ctxKeyServiceName, call next
//   - If key is missing: respond with domain.NewUnauthorized, do NOT log the key
//   - If key is invalid: respond with domain.NewUnauthorized, log warning with remote IP
//     — MUST NOT log the key value itself
//   - MUST use handler/response.go helpers to write RFC 7807 responses
//     (import cycle note: pass a response writer function or use domain.Problem directly)
func APIKey(keys map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := strings.TrimSpace(r.Header.Get("X-Api-Key"))
			if apiKey == "" {
				respondUnauthorized(w, domain.NewUnauthorized("missing api key", ""))
				return
			}

			var serviceName string
			for configuredKey, name := range keys {
				if subtle.ConstantTimeCompare([]byte(apiKey), []byte(configuredKey)) == 1 {
					serviceName = name
					break
				}
			}

			if serviceName == "" {
				zap.L().Warn("invalid api key", zap.String("remote_ip", r.RemoteAddr))
				respondUnauthorized(w, domain.NewUnauthorized("invalid api key", ""))
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyServiceName{}, serviceName)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondUnauthorized(w http.ResponseWriter, p *domain.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
