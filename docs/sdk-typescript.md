# TypeScript SDK Guide

## Quick Start

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

## Manual Instrumentation

```typescript
const span = tracer.startSpan('process-order', {
  kind: SpanKind.SERVER,
});

span.setAttributes({
  'order.id': '12345',
  'order.total': 99.99,
});

span.addEvent('payment.processed', { gateway: 'stripe' });

if (error) {
  span.recordException(error);
  span.setStatus({ code: StatusCode.ERROR, message: error.message });
}

span.end();
```

## Context Propagation

```typescript
import { inject, extract } from '@asymmetric-effort/ginger/propagation';

// Inject into outgoing fetch headers
const headers = new Headers();
inject(headers);
fetch(url, { headers });

// Extract from incoming request
const context = extract(request.headers);
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| endpoint | http://localhost:4318 | OTLP HTTP endpoint |
| service | "" | Service name |
| samplingRate | 1.0 | Probability (0.0-1.0) |
