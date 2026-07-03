package tenancy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenantFromContextEmpty(t *testing.T) {
	_, ok := TenantFromContext(context.Background())
	if ok {
		t.Error("empty context should return false")
	}
}

func TestContextWithTenant(t *testing.T) {
	ctx := ContextWithTenant(context.Background(), "tenant-1")
	tenant, ok := TenantFromContext(ctx)
	if !ok || tenant != "tenant-1" {
		t.Errorf("tenant = %q, ok = %v", tenant, ok)
	}
}

func TestExtractHTTP(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "my-tenant")

	tenant, err := e.ExtractHTTP(r)
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "my-tenant" {
		t.Errorf("tenant = %q", tenant)
	}
}

func TestExtractHTTPMissing(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})
	r := httptest.NewRequest("GET", "/", nil)

	_, err := e.ExtractHTTP(r)
	if err != ErrTenantRequired {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}
}

func TestExtractHTTPDefault(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, DefaultTenant: "default-t"})
	r := httptest.NewRequest("GET", "/", nil)

	tenant, err := e.ExtractHTTP(r)
	if err != nil {
		t.Fatal(err)
	}
	if tenant != "default-t" {
		t.Errorf("tenant = %q", tenant)
	}
}

func TestExtractHTTPInvalidName(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "invalid tenant!@#")

	_, err := e.ExtractHTTP(r)
	if err != ErrTenantInvalid {
		t.Errorf("expected ErrTenantInvalid, got %v", err)
	}
}

func TestExtractHTTPTooLong(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})
	r := httptest.NewRequest("GET", "/", nil)
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	r.Header.Set("X-Tenant-ID", string(long))

	_, err := e.ExtractHTTP(r)
	if err != ErrTenantInvalid {
		t.Errorf("expected ErrTenantInvalid for too-long name, got %v", err)
	}
}

func TestExtractHTTPAllowlist(t *testing.T) {
	e := NewExtractor(Config{
		Enabled:        true,
		Header:         "X-Tenant-ID",
		AllowedTenants: []string{"tenant-a", "tenant-b"},
	})

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "tenant-a")
	tenant, err := e.ExtractHTTP(r)
	if err != nil || tenant != "tenant-a" {
		t.Error("allowed tenant should pass")
	}

	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "tenant-c")
	_, err = e.ExtractHTTP(r)
	if err != ErrTenantNotAllowed {
		t.Errorf("expected ErrTenantNotAllowed, got %v", err)
	}
}

func TestExtractFromMetadata(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})
	md := map[string][]string{"x-tenant-id": {"my-tenant"}}

	tenant, err := e.ExtractFromMetadata(md)
	if err != nil || tenant != "my-tenant" {
		t.Errorf("tenant = %q, err = %v", tenant, err)
	}
}

func TestExtractFromMetadataMissing(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})
	_, err := e.ExtractFromMetadata(map[string][]string{})
	if err != ErrTenantRequired {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}
}

func TestExtractFromMetadataDefault(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, DefaultTenant: "def"})
	tenant, err := e.ExtractFromMetadata(map[string][]string{})
	if err != nil || tenant != "def" {
		t.Errorf("expected default, got %q", tenant)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})

	called := false
	handler := e.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := TenantFromContext(r.Context())
		if !ok || tenant != "test-tenant" {
			t.Errorf("middleware tenant = %q", tenant)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "test-tenant")
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("handler should have been called")
	}
}

func TestHTTPMiddlewareMissing(t *testing.T) {
	e := NewExtractor(Config{Enabled: true, Header: "X-Tenant-ID"})
	handler := e.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHTTPMiddlewareNotAllowed(t *testing.T) {
	e := NewExtractor(Config{
		Enabled:        true,
		Header:         "X-Tenant-ID",
		AllowedTenants: []string{"allowed"},
	})
	handler := e.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "forbidden")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Header != "X-Tenant-ID" {
		t.Errorf("default header = %q", cfg.Header)
	}
}

func TestDefaultHeaderFallback(t *testing.T) {
	e := NewExtractor(Config{Enabled: true}) // no header set
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "test")
	tenant, err := e.ExtractHTTP(r)
	if err != nil || tenant != "test" {
		t.Errorf("fallback header: %q, %v", tenant, err)
	}
}

func TestDefaultMetadataKeyFallback(t *testing.T) {
	e := NewExtractor(Config{Enabled: true}) // no header set
	md := map[string][]string{"x-tenant-id": {"test"}}
	tenant, err := e.ExtractFromMetadata(md)
	if err != nil || tenant != "test" {
		t.Errorf("fallback key: %q, %v", tenant, err)
	}
}
