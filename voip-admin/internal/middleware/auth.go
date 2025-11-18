package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// FreeSWITCH Basic Auth credentials
	FreeSwitchUser     string
	FreeSwitchPassword string

	// API Key for admin API
	APIKeys []string
}

// BasicAuth middleware for FreeSWITCH XML_CURL endpoints
func BasicAuth(config *AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()

			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="FreeSWITCH"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Constant-time comparison to prevent timing attacks
			userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(config.FreeSwitchUser))
			passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(config.FreeSwitchPassword))

			if userMatch != 1 || passMatch != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="FreeSWITCH"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// APIKeyAuth middleware for admin REST API
func APIKeyAuth(config *AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key from header
			apiKey := r.Header.Get("X-API-Key")

			// Also check Authorization header with Bearer scheme
			if apiKey == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					apiKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if apiKey == "" {
				http.Error(w, "Missing API key", http.StatusUnauthorized)
				return
			}

			// Validate API key (constant-time comparison)
			valid := false
			for _, validKey := range config.APIKeys {
				if subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1 {
					valid = true
					break
				}
			}

			if !valid {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AllowPublic middleware that allows public access (for health checks)
func AllowPublic(next http.Handler) http.Handler {
	return next
}
