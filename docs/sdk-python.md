# Python SDK Guide

## Quick Start

```python
from ginger.tracer import Tracer

tracer = Tracer(endpoint="http://localhost:4318", service="my-service")

with tracer.start_span("operation-name") as span:
    span.set_attribute("key", "value")
```

## Manual Instrumentation

```python
with tracer.start_span("process-order", kind=SpanKind.SERVER) as span:
    span.set_attribute("order.id", "12345")
    span.add_event("payment.processed", {"gateway": "stripe"})

    try:
        process(order)
    except Exception as e:
        span.record_exception(e)
        span.set_status(StatusCode.ERROR, str(e))
```

## Context Propagation

```python
from ginger.propagation import inject, extract

# Inject into outgoing request headers
headers = {}
inject(headers)

# Extract from incoming request headers
context = extract(request.headers)
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| endpoint | http://localhost:4318 | OTLP HTTP endpoint |
| service | "" | Service name |
| sampling_rate | 1.0 | Probability (0.0-1.0) |
