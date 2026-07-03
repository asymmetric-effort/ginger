package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestBearerTokenValid(t *testing.T) {
	mw := BearerTokenMiddleware(func(token string) bool { return token == "valid-token" })
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer valid-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestBearerTokenInvalid(t *testing.T) {
	mw := BearerTokenMiddleware(func(token string) bool { return token == "valid" })
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer invalid")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("missing WWW-Authenticate")
	}
}

func TestBearerTokenMissing(t *testing.T) {
	mw := BearerTokenMiddleware(func(token string) bool { return true })
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBearerTokenWrongScheme(t *testing.T) {
	mw := BearerTokenMiddleware(func(token string) bool { return true })
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAPIKeyValid(t *testing.T) {
	mw := APIKeyMiddleware([]string{"key-1", "key-2"})
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "key-2")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestAPIKeyInvalid(t *testing.T) {
	mw := APIKeyMiddleware([]string{"key-1"})
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "wrong-key")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAPIKeyMissing(t *testing.T) {
	mw := APIKeyMiddleware([]string{"key-1"})
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBasicAuthValid(t *testing.T) {
	mw := BasicAuthMiddleware(map[string]string{"admin": "secret"})
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.SetBasicAuth("admin", "secret")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestBasicAuthWrongPassword(t *testing.T) {
	mw := BasicAuthMiddleware(map[string]string{"admin": "secret"})
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.SetBasicAuth("admin", "wrong")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBasicAuthWrongUser(t *testing.T) {
	mw := BasicAuthMiddleware(map[string]string{"admin": "secret"})
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.SetBasicAuth("nobody", "secret")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBasicAuthMissing(t *testing.T) {
	mw := BasicAuthMiddleware(map[string]string{"admin": "secret"})
	handler := mw(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("missing WWW-Authenticate")
	}
}
