package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// BearerTokenMiddleware validates Bearer tokens.
func BearerTokenMiddleware(validate func(token string) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				w.Header().Set("WWW-Authenticate", `Bearer realm="ginger"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			token := authHeader[7:]
			if !validate(token) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="ginger"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// APIKeyMiddleware validates API keys from the X-API-Key header.
func APIKeyMiddleware(validKeys []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				http.Error(w, "API key required", http.StatusUnauthorized)
				return
			}
			valid := false
			for _, vk := range validKeys {
				if subtle.ConstantTimeCompare([]byte(key), []byte(vk)) == 1 {
					valid = true
					break
				}
			}
			if !valid {
				http.Error(w, "invalid API key", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// BasicAuthMiddleware provides HTTP Basic authentication.
// credentials is a map of username→password.
func BasicAuthMiddleware(credentials map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="ginger"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			expected, exists := credentials[user]
			if !exists || subtle.ConstantTimeCompare([]byte(pass), []byte(expected)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="ginger"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
