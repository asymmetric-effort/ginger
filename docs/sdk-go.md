# Go SDK Guide

## Quick Start

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

## Manual Instrumentation

```go
ctx, span := tr.Start(ctx, "process-order",
    tracer.WithSpanKind(tracer.SpanKindServer),
)
defer span.End()

span.SetAttribute("order.id", "12345")
span.SetAttribute("order.total", 99.99)

span.AddEvent("payment.processed", map[string]string{
    "gateway": "stripe",
})

if err != nil {
    span.RecordError(err)
    span.SetStatus(tracer.StatusError, err.Error())
}
```

## Context Propagation

```go
// Inject into outgoing HTTP request
tracer.InjectHTTP(ctx, req.Header)

// Extract from incoming HTTP request
ctx = tracer.ExtractHTTP(ctx, req.Header)
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| Endpoint | localhost:4317 | OTLP gRPC endpoint |
| Service | "" | Service name |
| SamplingRate | 1.0 | Probability (0.0-1.0) |
| BatchSize | 512 | Max spans per batch |
