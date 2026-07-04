<p align="center">
  <img src="docs/img/logo.png" alt="Ginger" width="300" />
</p>

<h3 align="center">OpenTelemetry tracing toolkit for Kubernetes with post-quantum cryptography</h3>

<p align="center">
  <a href="https://github.com/asymmetric-effort/ginger/actions"><img src="https://github.com/asymmetric-effort/ginger/actions/workflows/ci.yml/badge.svg" alt="CI Status" /></a>
  <img src="https://img.shields.io/badge/go-1.26.4-blue" alt="Go Version" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License" /></a>
  <a href="https://hub.docker.com/r/asymmetriceffort/ginger"><img src="https://img.shields.io/badge/docker-asymmetriceffort%2Fginger-blue" alt="Docker Image" /></a>
</p>

---

## Overview

Ginger provides distributed tracing with full feature parity with Jaeger v2, built entirely on the
OpenTelemetry standard. All data collection and storage is encrypted using post-quantum cryptography
(PQC) meeting or exceeding NIST and CIS standards.

**Key features:**

- **OpenTelemetry Native** -- Built on OTLP with full protocol support
- **Post-Quantum Cryptography** -- X25519+ML-KEM-768 hybrid key exchange for all data in transit
- **InfluxDB Backend** -- Optimized time-series storage for trace data
- **Kubernetes First** -- Designed for high-security K8s environments with Helm chart support
- **Multi-Protocol Ingestion** -- OTLP (gRPC/HTTP), Jaeger (gRPC/Thrift), and Zipkin receivers
- **Adaptive Sampling** -- Head-based, tail-based, and remote sampling strategies
- **Multi-Tenancy** -- Tenant isolation with configurable routing
- **Client SDKs** -- Go, Python, and TypeScript libraries

---

## Quick Start

### Docker

```bash
docker run -p 4317:4317 -p 4318:4318 -p 16686:16686 \
  -e INFLUXDB_ENDPOINT=http://influxdb:8086 \
  -e INFLUXDB_TOKEN=my-token \
  asymmetriceffort/ginger --config /etc/ginger/config.yaml
```

### Helm

```bash
helm install ginger deploy/helm/ginger \
  --set storage.influxdb.endpoint=http://influxdb:8086 \
  --set storage.influxdb.token=my-token
```

### Go SDK

```go
import "github.com/asymmetric-effort/ginger/sdk/go/tracer"

tr := tracer.New(tracer.Config{
    Endpoint: "localhost:4317",
    Service:  "my-service",
})
defer tr.Shutdown()

ctx, span := tr.Start(ctx, "operation-name")
defer span.End()
```

### Python SDK

```python
from ginger.tracer import Tracer

tracer = Tracer(endpoint="http://localhost:4318", service="my-service")

with tracer.start_span("operation-name") as span:
    span.set_attribute("key", "value")
```

### TypeScript SDK

```typescript
import { Tracer } from '@asymmetric-effort/ginger';

const tracer = new Tracer({
  endpoint: 'http://localhost:4318',
  service: 'my-service',
});

const span = tracer.startSpan('operation-name');
try {
  // ... your code
} finally {
  span.end();
}
```

---

## Architecture

Ginger implements the OpenTelemetry Collector pipeline model with receivers, processors, exporters,
and extensions. The collector is stateless and horizontally scalable. The query service provides a
Jaeger-compatible UI and API backed by InfluxDB.

```
Clients (SDKs / Agents)
        |
  +-----------+-----------+
  |           |           |
OTLP gRPC  OTLP HTTP  Zipkin HTTP
 :4317      :4318       :9411
  |           |           |
  +-----+-----+-----------+
        |
   [ Pipeline ]
   Batch -> Sample -> Export
        |
   [ InfluxDB ]
        |
   [ Query API ]
    :16686
```

For detailed architecture documentation, see [docs/architecture.md](docs/architecture.md).

---

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System architecture and pipeline design |
| [Configuration](docs/configuration.md) | Configuration reference (YAML, env vars) |
| [REST API](docs/api-v1.md) | Query API v2/v3 reference |
| [gRPC Services](docs/grpc.md) | gRPC service and protobuf reference |
| [Go SDK](docs/sdk-go.md) | Go client library guide |
| [Python SDK](docs/sdk-python.md) | Python client library guide |
| [TypeScript SDK](docs/sdk-typescript.md) | TypeScript client library guide |
| [Deployment](docs/deployment.md) | Deployment, Docker, and security guide |
| [Kubernetes](docs/kubernetes.md) | Kubernetes scaling patterns |
| [Documentation Index](docs/index.md) | Full documentation index |

---

## Installation

### Go

```bash
go install github.com/asymmetric-effort/ginger/cmd/ginger@latest
```

### Python

```bash
pip install ginger-tracing
```

### TypeScript / Node.js

```bash
npm install @asymmetric-effort/ginger
```

### Docker

```bash
docker pull asymmetriceffort/ginger:latest
```

### Helm

```bash
helm repo add ginger https://asymmetric-effort.github.io/ginger
helm install ginger ginger/ginger
```

### Build from Source

```bash
go build -o bin/ginger ./cmd/ginger
```

---

## Key Features

### Post-Quantum Cryptography

All data in transit is protected using a hybrid X25519+ML-KEM-768 key exchange, providing
resistance to both classical and quantum attacks. Meets or exceeds NIST PQC and CIS benchmarks.

### Multi-Protocol Ingestion

Ginger accepts trace data over multiple protocols simultaneously:

- **OTLP gRPC** (port 4317) -- Primary ingestion endpoint
- **OTLP HTTP** (port 4318) -- HTTP/JSON and HTTP/Protobuf
- **Jaeger gRPC** (port 14250) -- Jaeger native gRPC
- **Jaeger Thrift** (ports 6831, 6832, 14268) -- Compact, binary, and HTTP Thrift
- **Zipkin** (port 9411) -- Zipkin v2 JSON

### InfluxDB Storage Backend

Trace spans are stored in InfluxDB as time-series data, enabling efficient range queries and
long-term retention with configurable TTL.

### Multi-Tenancy

Tenant-aware routing isolates trace data by tenant ID. Each tenant can have independent storage
buckets, sampling policies, and retention rules.

### Adaptive Sampling

Three sampling strategies operate independently or together:

- **Head-based** -- Probabilistic sampling at ingestion time
- **Tail-based** -- Decision deferred until the full trace is assembled
- **Remote** -- Centralized sampling configuration pushed to SDKs

### Kubernetes Native

First-class Kubernetes support with Helm charts, horizontal pod autoscaling, health probes,
and resource limit configuration. The collector is stateless for easy scaling.

---

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

---

## Development

### Prerequisites

- Go 1.26.4+
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

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, and contribution
guidelines.

## Security

See [SECURITY.md](SECURITY.md) for our security policy and vulnerability reporting instructions.

## License

[MIT](LICENSE)
