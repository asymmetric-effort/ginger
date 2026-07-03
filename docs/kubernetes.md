# Horizontal Scaling Patterns

## Collector Scaling

The ginger collector is stateless — multiple replicas can run behind a Kubernetes Service for horizontal scaling.

**Limitation**: Tail sampling requires trace fragments from the same trace to reach the same collector instance. Use consistent hashing by TraceID via a headless Service with client-side routing, or use head-based sampling instead.

```yaml
# Multi-replica deployment
replicas: 3
```

## Query Service Scaling

The query service is fully stateless. Any replica can serve any query since all state is in InfluxDB. Scale freely.

## InfluxDB Connection Pooling

Each ginger pod maintains up to 10 idle connections to InfluxDB. For N replicas:
- Recommended: N × 10 max connections at InfluxDB
- Monitor InfluxDB connection limits when scaling beyond 10 replicas

## Readiness Probes

Zero-downtime rolling updates require proper readiness probes:

```yaml
readinessProbe:
  httpGet:
    path: /health/ready
    port: 13133
  initialDelaySeconds: 5
  periodSeconds: 5
```

The readiness probe checks storage connectivity and pipeline queue capacity before accepting traffic.
