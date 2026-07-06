# Security-First Design

Ginger is built for high-security Kubernetes environments where tracing
infrastructure is part of the trusted computing base. This document describes
the security controls embedded in ginger's design, their alignment with
industry standards, and how operators can verify compliance.

## Design Principles

1. **Zero third-party dependencies.** The Go binary has no external module
   dependencies. Every component -- codecs, protocols, HTTP/gRPC servers,
   storage clients, cryptography -- is implemented from scratch or uses Go's
   standard library. This eliminates supply chain risk from transitive
   dependencies entirely.

2. **Post-quantum cryptography by default.** All TLS connections negotiate
   hybrid X25519+ML-KEM-768 key exchange (NIST FIPS 203). See
   [Post-Quantum Cryptography](post-quantum-cryptography.md) for details.

3. **Principle of least privilege.** The container runs as a non-root user,
   with a read-only root filesystem, minimal capabilities, and network
   policies that restrict egress to only the storage backend.

4. **Defense in depth.** Authentication, tenant isolation, input validation,
   and encryption each operate independently. A failure in one layer does
   not compromise the others.

## CIS Kubernetes Benchmark Alignment

The following table maps ginger's controls to the CIS Kubernetes Benchmark
v1.8 recommendations:

| CIS Control | Requirement | Ginger Implementation |
|-------------|-------------|----------------------|
| 5.1.1 | Ensure that the cluster-admin role is only used where required | Ginger's ServiceAccount has no cluster-admin binding. RBAC is scoped to the minimum permissions needed for health probes. |
| 5.1.3 | Minimize wildcard use in Roles and ClusterRoles | No wildcard permissions in ginger's ClusterRole. |
| 5.2.1 | Minimize the admission of privileged containers | `securityContext.runAsNonRoot: true`, `readOnlyRootFilesystem: true`. No privileged flag. |
| 5.2.2 | Minimize the admission of containers wishing to share the host process ID namespace | `hostPID: false` (default). Not set in deployment manifests. |
| 5.2.6 | Minimize the admission of root containers | Container runs as UID 10001, GID 10001. Enforced in both Dockerfile (`USER nonroot:nonroot`) and Kubernetes (`runAsUser: 10001`). |
| 5.2.7 | Minimize the admission of containers with added capabilities | No capabilities added. The distroless base image has no shell, package manager, or unnecessary utilities. |
| 5.4.1 | Prefer using secrets as files over secrets as environment variables | TLS certificates and InfluxDB tokens are mounted as files via Kubernetes Secrets, not passed as environment variables. Config supports `${ENV_VAR}` interpolation for cases where env vars are unavoidable. |
| 5.7.1 | Create administrative boundaries between resources using namespaces | Ginger deploys into its own namespace (`ginger`) with NetworkPolicy isolation. |
| 5.7.2 | Ensure that the seccomp profile is set to docker/default or runtime/default | The default seccomp profile applies. No custom seccomp override. |

## Container Hardening

### Dockerfile Standards

Ginger follows the [Docker Standards](https://coding-standards.asymmetric-effort.com/docker-standards):

```dockerfile
# Builder stage
FROM ubuntu:26.04 AS builder
# Install build deps, compile Go binary

# Runtime stage
FROM gcr.io/distroless/base AS runtime
COPY --from=builder /ginger /ginger
USER nonroot:nonroot
ENTRYPOINT ["/ginger"]
```

| Control | Implementation |
|---------|---------------|
| Base image | `gcr.io/distroless/base` -- no shell, no package manager, no utilities |
| Non-root user | `USER nonroot:nonroot` (UID 65534) in Dockerfile, `runAsUser: 10001` in Kubernetes |
| Read-only filesystem | `readOnlyRootFilesystem: true` in pod security context |
| No build tools in runtime | Multi-stage build; builder tools stay in the builder stage |
| Explicit version tags | `ubuntu:26.04` and distroless pinned; no `:latest` tags |
| Image scanning | Dependabot monitors the Dockerfile for known CVE updates |

### What the Runtime Image Contains

The final runtime image contains exactly one file: the statically linked
`ginger` binary (`CGO_ENABLED=0`). There is no:

- Shell (`/bin/sh`, `/bin/bash`)
- Package manager (`apt`, `apk`)
- Compiler or interpreter
- Debugging tools
- Temporary directory with write access

An attacker who gains code execution inside the container cannot install
tools, download payloads, or escalate privileges through shell access.

## Network Security

### NetworkPolicy

The Kubernetes manifests include a NetworkPolicy that restricts traffic:

```yaml
# deploy/kubernetes/networkpolicy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ginger
spec:
  podSelector:
    matchLabels:
      app: ginger
  policyTypes: [Ingress, Egress]
  ingress:
    - ports:
        - port: 4317   # OTLP gRPC
        - port: 4318   # OTLP HTTP
        - port: 16686  # Query
        - port: 13133  # Health
        - port: 9411   # Zipkin
        - port: 6831   # Jaeger Compact (UDP)
          protocol: UDP
        - port: 6832   # Jaeger Binary (UDP)
          protocol: UDP
  egress:
    - ports:
        - port: 8086   # InfluxDB
          protocol: TCP
    - ports:
        - port: 53     # DNS
          protocol: UDP
        - port: 53
          protocol: TCP
```

**Ingress**: Only trace ingest, query, and health check ports are open.
Administrative ports (pprof) are not exposed by default.

**Egress**: The only permitted outbound connection is to InfluxDB (port 8086)
and DNS. The container cannot reach the internet, other services, or the
Kubernetes API server.

### Encryption in Transit

All TCP connections use TLS 1.3 with PQC. See
[Post-Quantum Cryptography](post-quantum-cryptography.md). TLS 1.2 and
earlier are rejected by setting `MinVersion: tls.VersionTLS13`.

Legacy cipher suites (RC4, 3DES, MD5-based MACs) are not available in
TLS 1.3 and are never negotiated.

### Encryption at Rest

Data at rest in InfluxDB is encrypted using InfluxDB's native encryption
(AES-256). Ginger does not store data locally -- the read-only filesystem
prevents any local writes. The in-memory storage backend exists only for
development and testing; it is not used in production deployments.

## Authentication

Ginger supports three authentication mechanisms, all implemented in
`internal/auth/`:

### Bearer Token Authentication

```yaml
auth:
  type: bearer
  validate: <external validator function>
```

The `BearerTokenMiddleware` extracts the `Authorization: Bearer <token>`
header and validates it against a configurable validation function. Invalid
or missing tokens receive HTTP 401 with a `WWW-Authenticate: Bearer` header.

### API Key Authentication

```yaml
auth:
  type: apikey
  keys:
    - ${GINGER_API_KEY_1}
    - ${GINGER_API_KEY_2}
```

The `APIKeyMiddleware` reads the `X-API-Key` header and compares it against
a list of valid keys using **constant-time comparison**
(`crypto/subtle.ConstantTimeCompare`). This prevents timing attacks that
could leak key material through response time differences.

### Basic Authentication

The `BasicAuthMiddleware` validates HTTP Basic credentials using
constant-time comparison. Passwords are compared directly (not hashed)
because the middleware is intended for internal/admin endpoints, not
user-facing authentication. For user-facing auth, use bearer tokens with
an external identity provider.

### Mutual TLS (mTLS)

For component-to-component communication, mutual TLS provides the strongest
authentication. Both client and server present certificates signed by a
trusted CA:

```go
cfg := pqc.NewMutualTLSConfig(caPool, serverCert)
// Client must present a valid certificate to connect
```

See [Post-Quantum Cryptography](post-quantum-cryptography.md) for details
on the mTLS configuration.

## Secrets Management

Ginger follows the project's hard rule: **no secrets in the code repository.**

- TLS certificates and keys are loaded from file paths, not embedded in the
  binary or configuration. In Kubernetes, these are mounted from Secrets.
- The InfluxDB token is loaded via environment variable interpolation
  (`${INFLUXDB_TOKEN}`) or file mount, never hardcoded.
- API keys are injected via environment variables referencing Kubernetes
  Secrets.
- The pre-commit hook scans for patterns matching secrets, credentials,
  and API keys. Commits containing suspected secrets are rejected.
- CI/CD authentication uses OIDC (GitHub Actions to npm, PyPI) with no
  long-lived tokens stored as repository secrets.

## Input Validation

### Trace Data Validation

All receivers validate incoming data before processing:

- **Max body size**: Configurable (default 4MB). Requests exceeding the limit
  receive HTTP 413 or gRPC RESOURCE_EXHAUSTED.
- **Content-Type enforcement**: Only `application/x-protobuf`,
  `application/json`, and `application/x-thrift` are accepted. Other content
  types receive HTTP 415.
- **Protobuf decoding**: The custom protobuf decoder validates wire types and
  field numbers. Malformed messages are rejected, not partially parsed.
- **Gzip decompression**: Gzip-encoded bodies are decompressed with bounded
  readers to prevent decompression bombs.

### Tenant Name Validation

Tenant identifiers are validated against `^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`.
See [Multi-Tenancy](multi-tenancy.md).

### Query Parameter Validation

Query API endpoints validate all parameters (time ranges, duration formats,
numeric limits) and return HTTP 400 with descriptive errors for invalid input.
No query parameter is passed directly to storage queries -- all Flux queries
are built programmatically with parameterized values to prevent injection.

## Supply Chain Security

### Zero Runtime Dependencies

The Go binary has **zero entries** in `go.sum`. Every component is
implemented using only Go's standard library:

- HTTP/gRPC servers: `net/http`, `crypto/tls`
- Protobuf codec: custom implementation in `internal/codec/protobuf`
- Thrift codec: custom implementation in `internal/codec/thrift`
- InfluxDB client: `net/http` with custom line protocol encoder
- YAML parser: custom implementation in `internal/config`
- Structured logging: custom implementation in `internal/logging`
- Prometheus metrics: custom implementation in `internal/metrics`
- Cryptography: Go stdlib `crypto/tls`, `crypto/mlkem`, `crypto/ecdsa`

This eliminates the entire class of supply chain attacks that exploit
transitive dependencies (typosquatting, dependency confusion, compromised
maintainer accounts).

### CI/CD Supply Chain

- All GitHub Actions are from `asymmetric-effort/actions` (internal) or
  pinned to specific commit SHAs (e.g., `github/codeql-action@3cf0a529...`).
  No tag-based action references.
- Dependabot monitors the Dockerfile base image and GitHub Actions for
  known vulnerabilities.
- CodeQL runs on every push to main and weekly, scanning for common
  vulnerability patterns (XSS, injection, path traversal).
- The supply chain audit job in CI verifies that no unauthorized external
  actions are used.

### SDK Dependencies

The client SDKs (Go, Python, TypeScript) also have zero runtime dependencies:

| SDK | Runtime Dependencies | Implementation |
|-----|---------------------|---------------|
| Go | 0 | `net/http`, `encoding/json` (stdlib) |
| Python | 0 | `urllib.request`, `json`, `threading` (stdlib) |
| TypeScript | 0 | `fetch` API (built into Node.js 18+) |

## Vulnerability Management

Ginger follows the vulnerability SLAs defined in the project's
[Security Policy](https://github.com/asymmetric-effort/ginger/blob/main/SECURITY.md):

| Severity | Patch SLA |
|----------|-----------|
| Critical | 24 hours |
| High | 72 hours |
| Medium | 30 days |
| Low | 90 days |

Vulnerabilities are reported via the process described in `SECURITY.md`.

## Compliance Summary

| Framework | Coverage |
|-----------|----------|
| CIS Kubernetes Benchmark | Container hardening, RBAC, network policies, non-root execution |
| NIST PQC (FIPS 203) | Hybrid X25519+ML-KEM-768 key exchange |
| OWASP | Input validation, output encoding, secrets management, authentication |
| SOC 2 | Audit logging, access controls, encryption in transit and at rest |
