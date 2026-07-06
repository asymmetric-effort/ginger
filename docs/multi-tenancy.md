# Multi-Tenancy

Ginger supports multi-tenancy, allowing a single deployment to serve multiple
independent teams, customers, or environments while maintaining strict data
isolation between them. Each tenant's trace data is tagged, filtered, and
segregated so that one tenant cannot see, query, or interfere with another
tenant's data.

## How It Works

Multi-tenancy in ginger operates at three layers:

1. **Request extraction** -- identifying the tenant from incoming requests
2. **Storage tagging** -- labeling every span with its tenant identifier
3. **Query filtering** -- restricting query results to the requesting tenant

### Tenant Identification

Every inbound request must declare which tenant it belongs to. Ginger extracts
the tenant identity from a configurable HTTP header (default: `X-Tenant-ID`)
on REST/OTLP HTTP requests, or from gRPC metadata on gRPC requests.

```
POST /v1/traces HTTP/1.1
Host: ginger.example.com
X-Tenant-ID: acme-corp
Content-Type: application/x-protobuf

<OTLP trace data>
```

The tenant is extracted before any processing occurs and propagated through
the Go context for the lifetime of the request. Every downstream component
(pipeline, storage, query) can access the tenant via `tenancy.TenantFromContext(ctx)`.

### Storage Isolation

When multi-tenancy is enabled, the `TenantedWriter` wraps the storage writer
and injects a `ginger.tenant` resource attribute into every span before it
reaches the storage backend:

```
ginger.tenant = "acme-corp"
```

In InfluxDB, this becomes a tag on every measurement, enabling efficient
per-tenant queries. The tenant tag is indexed, so filtering by tenant adds
negligible overhead to queries regardless of total data volume.

### Query Isolation

The `TenantedReader` wraps the storage reader and enforces tenant boundaries
on every query:

- **GetTrace**: retrieves the trace, then verifies the `ginger.tenant`
  attribute matches the requesting tenant. Returns "trace not found" if it
  belongs to a different tenant.
- **FindTraces**: retrieves matching traces, then filters out any that
  belong to other tenants before returning results.

A tenant cannot discover that another tenant's traces exist. Cross-tenant
queries return empty results, not permission errors, to prevent information
leakage about other tenants' activity.

## Configuration

Multi-tenancy is disabled by default. Enable it in `config.yaml`:

```yaml
tenancy:
  enabled: true
  header: X-Tenant-ID
  default_tenant: ""
  allowed_tenants: []
```

### Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable multi-tenancy. When disabled, all requests are treated as belonging to a single implicit tenant. |
| `header` | string | `X-Tenant-ID` | HTTP header name used to identify the tenant. Also used as the gRPC metadata key (lowercased). |
| `default_tenant` | string | `""` (empty) | Tenant ID assigned to requests that arrive without a tenant header. If empty, requests without a tenant header are rejected with HTTP 401. |
| `allowed_tenants` | []string | `[]` (empty) | Allowlist of permitted tenant IDs. If empty, any valid tenant name is accepted. If non-empty, requests from tenants not on the list are rejected with HTTP 403. |

### Open Mode vs Allowlist Mode

**Open mode** (`allowed_tenants: []`): Any request with a valid tenant name
is accepted. Useful when tenants self-register or when the set of tenants is
dynamic.

```yaml
tenancy:
  enabled: true
  header: X-Tenant-ID
  # No allowlist -- accept any valid tenant name
```

**Allowlist mode** (`allowed_tenants: ["team-a", "team-b"]`): Only requests
from explicitly listed tenants are accepted. Requests from unlisted tenants
receive HTTP 403 Forbidden.

```yaml
tenancy:
  enabled: true
  header: X-Tenant-ID
  allowed_tenants:
    - production
    - staging
    - dev-team-alpha
    - dev-team-beta
```

## Tenant Name Validation

Tenant names must match the pattern `^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`:

- Must start with an alphanumeric character
- May contain alphanumeric characters and hyphens
- Maximum 64 characters
- No spaces, underscores, dots, or special characters

Invalid tenant names are rejected with HTTP 401 and the error message
`tenant name invalid: must be alphanumeric+hyphen, max 64 chars`.

Examples of valid tenant names:
- `acme-corp`
- `team-alpha`
- `env-production`
- `customer-12345`

Examples of invalid tenant names:
- `acme corp` (space)
- `_internal` (starts with underscore)
- `team.alpha` (dot)
- `` (empty)

## Usage Examples

### Example 1: SaaS Platform with Customer Isolation

A SaaS provider runs a single ginger deployment serving multiple customers.
Each customer's application sends traces with their customer ID:

```yaml
# ginger config
tenancy:
  enabled: true
  header: X-Tenant-ID
  allowed_tenants:
    - customer-acme
    - customer-globex
    - customer-initech
```

Customer Acme's application sends traces:

```bash
curl -X POST https://ginger.example.com/v1/traces \
  -H "X-Tenant-ID: customer-acme" \
  -H "Content-Type: application/json" \
  -d '{"resourceSpans": [...]}'
```

When Acme queries for their traces, they only see their own data:

```bash
curl https://ginger.example.com/api/v3/services \
  -H "X-Tenant-ID: customer-acme"
# Returns: ["acme-frontend", "acme-api", "acme-payments"]
# Does NOT return Globex or Initech services
```

If Acme attempts to retrieve a trace that belongs to Globex:

```bash
curl https://ginger.example.com/api/v3/traces/abc123 \
  -H "X-Tenant-ID: customer-acme"
# Returns: 404 Not Found (even though the trace exists for customer-globex)
```

### Example 2: Environment Separation

A team uses a single ginger deployment for all environments, separating
production and staging traces:

```yaml
tenancy:
  enabled: true
  header: X-Environment
  allowed_tenants:
    - production
    - staging
    - development
```

The production deployment sends traces with `X-Environment: production`:

```bash
# Production Kubernetes deployment environment variable
env:
  - name: GINGER_TENANT
    value: "production"
```

The application's OTLP exporter includes the header:

```go
// Go SDK example
req.Header.Set("X-Environment", os.Getenv("GINGER_TENANT"))
```

Production queries only return production traces, even though staging and
development traces coexist in the same InfluxDB instance.

### Example 3: Default Tenant for Migration

During migration from a non-tenanted deployment, use `default_tenant` to
assign a tenant to legacy applications that do not yet send a tenant header:

```yaml
tenancy:
  enabled: true
  header: X-Tenant-ID
  default_tenant: legacy
  allowed_tenants:
    - legacy
    - team-platform
    - team-payments
```

Applications that have been updated send `X-Tenant-ID: team-platform`.
Legacy applications that have not been updated are automatically assigned
to the `legacy` tenant.

### Example 4: Open Registration

An internal platform allows any team to self-register by choosing a tenant
name. No allowlist is configured:

```yaml
tenancy:
  enabled: true
  header: X-Team-ID
```

Any request with a valid team name is accepted:

```bash
# Team Alpha starts sending traces
curl -X POST https://ginger.internal/v1/traces \
  -H "X-Team-ID: team-alpha" \
  -d '...'

# Team Beta starts independently, no configuration change needed
curl -X POST https://ginger.internal/v1/traces \
  -H "X-Team-ID: team-beta" \
  -d '...'
```

## HTTP Middleware

When multi-tenancy is enabled, the tenant extraction middleware is applied
to all inbound HTTP handlers. The middleware:

1. Reads the tenant header from the request
2. Validates the tenant name format
3. Checks the allowlist (if configured)
4. Injects the tenant into the request context
5. Passes the request to the next handler

If extraction fails, the middleware short-circuits the request:

| Condition | HTTP Status | Error |
|-----------|-------------|-------|
| Missing header, no default tenant | 401 Unauthorized | `tenant header required` |
| Invalid tenant name format | 401 Unauthorized | `tenant name invalid: must be alphanumeric+hyphen, max 64 chars` |
| Tenant not in allowlist | 403 Forbidden | `tenant not allowed` |

## gRPC Metadata

For gRPC receivers (OTLP gRPC, Jaeger gRPC), the tenant is extracted from
gRPC metadata using the same header name, lowercased per gRPC convention:

```
# HTTP header
X-Tenant-ID: acme-corp

# Equivalent gRPC metadata key
x-tenant-id: acme-corp
```

## Storage Schema

When multi-tenancy is enabled, every span stored in InfluxDB includes the
tenant as a tag:

```
traces,service=frontend,tenant=acme-corp,trace_id=abc123 duration_us=1500i ...
```

The `tenant` tag is indexed, so queries filtered by tenant use the tag index
and do not scan other tenants' data. This provides both isolation and
performance: a query for tenant A's traces does not touch tenant B's data
at the storage level.

## Security Considerations

- Tenant identity is determined solely by the request header. In production,
  combine multi-tenancy with authentication (bearer token, mTLS, or API key)
  to prevent clients from impersonating other tenants.
- The tenant header should be set by a trusted component (API gateway, service
  mesh sidecar, or the application itself) rather than by end users directly.
- Cross-tenant data leakage is prevented at the query layer. Even if a
  malicious client guesses a valid trace ID belonging to another tenant, the
  query returns "not found" rather than the trace data.
- The allowlist provides defense-in-depth: even if an attacker can set
  arbitrary headers, they cannot create traces for tenants not on the list.
