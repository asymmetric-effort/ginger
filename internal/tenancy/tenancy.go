package tenancy

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
)

var (
	// ErrTenantRequired is returned when no tenant header is present.
	ErrTenantRequired = errors.New("tenant header required")
	// ErrTenantNotAllowed is returned when the tenant is not in the allowlist.
	ErrTenantNotAllowed = errors.New("tenant not allowed")
	// ErrTenantInvalid is returned when the tenant name is malformed.
	ErrTenantInvalid = errors.New("tenant name invalid: must be alphanumeric+hyphen, max 64 chars")
)

var tenantNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

type tenantContextKey struct{}

// Config configures multi-tenancy.
type Config struct {
	Enabled        bool
	Header         string
	DefaultTenant  string
	AllowedTenants []string
}

// DefaultConfig returns a Config with defaults.
func DefaultConfig() Config {
	return Config{
		Header: "X-Tenant-ID",
	}
}

// TenantFromContext extracts the tenant from the context.
func TenantFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tenantContextKey{}).(string)
	return t, ok
}

// ContextWithTenant returns a context with the tenant set.
func ContextWithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenant)
}

// Extractor extracts and validates tenant from requests.
type Extractor struct {
	config   Config
	allowed  map[string]struct{}
}

// NewExtractor creates a new tenant Extractor.
func NewExtractor(config Config) *Extractor {
	e := &Extractor{config: config}
	if len(config.AllowedTenants) > 0 {
		e.allowed = make(map[string]struct{}, len(config.AllowedTenants))
		for _, t := range config.AllowedTenants {
			e.allowed[t] = struct{}{}
		}
	}
	return e
}

// ExtractHTTP extracts the tenant from an HTTP request header.
func (e *Extractor) ExtractHTTP(r *http.Request) (string, error) {
	header := e.config.Header
	if header == "" {
		header = "X-Tenant-ID"
	}
	tenant := r.Header.Get(header)
	if tenant == "" {
		if e.config.DefaultTenant != "" {
			return e.config.DefaultTenant, nil
		}
		return "", ErrTenantRequired
	}
	return e.validate(tenant)
}

// ExtractFromMetadata extracts the tenant from gRPC-style metadata.
func (e *Extractor) ExtractFromMetadata(md map[string][]string) (string, error) {
	key := strings.ToLower(e.config.Header)
	if key == "" {
		key = "x-tenant-id"
	}
	vals, ok := md[key]
	if !ok || len(vals) == 0 {
		if e.config.DefaultTenant != "" {
			return e.config.DefaultTenant, nil
		}
		return "", ErrTenantRequired
	}
	return e.validate(vals[0])
}

func (e *Extractor) validate(tenant string) (string, error) {
	if !tenantNameRE.MatchString(tenant) {
		return "", ErrTenantInvalid
	}
	if e.allowed != nil {
		if _, ok := e.allowed[tenant]; !ok {
			return "", ErrTenantNotAllowed
		}
	}
	return tenant, nil
}

// HTTPMiddleware returns middleware that extracts the tenant and adds it to the context.
func (e *Extractor) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, err := e.ExtractHTTP(r)
		if err != nil {
			if errors.Is(err, ErrTenantNotAllowed) {
				http.Error(w, err.Error(), http.StatusForbidden)
			} else {
				http.Error(w, err.Error(), http.StatusUnauthorized)
			}
			return
		}
		ctx := ContextWithTenant(r.Context(), tenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
