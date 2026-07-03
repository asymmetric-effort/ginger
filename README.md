# Ginger

An OpenTelemetry tracing toolkit for Kubernetes, focused on high-security
environments with post-quantum cryptography.

## Overview

Ginger provides distributed tracing with 100% feature parity with Jaeger v2,
built on the OpenTelemetry standard. All data collection and storage is
encrypted using post-quantum cryptography (PQC) meeting or exceeding NIST
and CIS standards.

### Key Features

- **OpenTelemetry Native**: Built on OTLP with full protocol support
- **Post-Quantum Cryptography**: All data encrypted with PQC algorithms
- **InfluxDB Backend**: Optimized time-series storage for trace data
- **Kubernetes First**: Designed for high-security K8s environments
- **Multi-Protocol Ingestion**: OTLP, Jaeger, and Zipkin protocols
- **Adaptive Sampling**: Head-based, tail-based, and remote sampling
- **Multi-Tenancy**: Tenant isolation with configurable routing
- **Client SDKs**: Go, Python, and TypeScript libraries

### Architecture

Ginger implements the OpenTelemetry Collector pipeline model:

- **Receivers**: OTLP (gRPC/HTTP), Jaeger (gRPC/Thrift), Zipkin
- **Processors**: Batch, sampling, memory limiter, attributes
- **Exporters**: InfluxDB storage, Kafka, OTLP forwarding
- **Extensions**: Query service, health checks, sampling configuration

## Quick Start

### Build

```bash
go build -o bin/ginger ./cmd/ginger
```

### Run

```bash
./bin/ginger --config config.yaml
```

### Docker

```bash
docker build -t ginger .
docker run ginger
```

### Kubernetes

```bash
helm install ginger deploy/helm/ginger
```

## Project Structure

```
cmd/ginger/          # Main binary entry point
internal/
  collector/         # Span collection and ingestion
  query/             # Query service and API
  storage/influxdb/  # InfluxDB storage backend
  sampling/          # Adaptive, tail, and remote sampling
  crypto/pqc/        # Post-quantum cryptography
  config/            # Configuration management
  health/            # Health check endpoints
  metrics/           # Internal metrics and monitoring
  pipeline/          # OTel Collector pipeline orchestration
  protocol/          # Protocol implementations (OTLP, Jaeger, Zipkin)
  ui/                # Web UI
  version/           # Version information
pkg/api/v1/          # Public API definitions
sdk/
  go/                # Go client library
  python/            # Python client library
  typescript/        # TypeScript client library
tests/
  unit/              # Unit tests
  integration/       # Integration tests
  e2e/               # End-to-end tests
deploy/
  kubernetes/        # Kubernetes manifests
  helm/              # Helm chart
docs/                # Documentation
```

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

### Prerequisites

- Go 1.26.3+
- Docker
- Kubernetes cluster (for e2e tests)

### Git Hooks

```bash
ln -sf ../../git-hooks/pre-commit .git/hooks/pre-commit
ln -sf ../../git-hooks/pre-push .git/hooks/pre-push
```

### Testing

```bash
# Unit tests
go test ./...

# With coverage
go test -race -coverprofile=coverage.txt -covermode=atomic ./...

# Integration tests
go test -tags=integration ./tests/integration/...

# E2E tests
go test -tags=e2e ./tests/e2e/...
```

## Security

See [SECURITY.md](SECURITY.md) for our security policy and vulnerability
reporting instructions.

## License

[MIT](LICENSE)
