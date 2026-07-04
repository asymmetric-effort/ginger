# ginger-tracing

Zero-dependency Python SDK for [Ginger](https://github.com/asymmetric-effort/ginger) — an OpenTelemetry tracing toolkit for Kubernetes with post-quantum cryptography.

## Installation

```bash
pip install ginger-tracing
```

## Quick Start

```python
from ginger.tracer import Tracer

tracer = Tracer(endpoint="http://localhost:4318", service="my-service")

with tracer.start_span("operation-name") as span:
    span.set_attribute("key", "value")
```

## Features

- Zero third-party dependencies (stdlib only)
- OTLP HTTP JSON export via `urllib.request`
- Context manager span lifecycle
- Full type hints (mypy strict compatible)
- W3C TraceContext propagation

## License

MIT
