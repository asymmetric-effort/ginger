# Configuration Reference

Ginger is configured via a YAML file. Environment variables can be interpolated using `${VAR}` or `${VAR:-default}` syntax.

## Server

```yaml
server:
  otlp_grpc_port: 4317
  otlp_http_port: 4318
  jaeger_grpc_port: 14250
  jaeger_thrift_http_port: 14268
  jaeger_thrift_compact_port: 6831
  jaeger_thrift_binary_port: 6832
  zipkin_port: 9411
  query_port: 16686
  admin_port: 13133
```

## Storage

```yaml
storage:
  backend: influxdb
  influxdb:
    endpoint: ${INFLUXDB_ENDPOINT:-http://localhost:8086}
    org: ${INFLUXDB_ORG:-ginger}
    bucket: ${INFLUXDB_BUCKET:-traces}
    token: ${INFLUXDB_TOKEN}
    write_timeout: 10s
    query_timeout: 30s
```

## Pipeline

```yaml
pipeline:
  queue_size: 1024
  drain_timeout: 5s
  batch:
    max_batch_size: 512
    send_delay: 200ms
  memory_limiter:
    hard_limit_mib: 1024
    soft_limit_mib: 768
    check_interval: 1s
```

## Sampling

```yaml
sampling:
  type: adaptive  # or "file"
  file:
    path: /etc/ginger/strategies.json
    reload_interval: 60s
  adaptive:
    target_samples_per_second: 1.0
    initial_sampling_rate: 1.0
```

## TLS

```yaml
tls:
  enabled: true
  cert_file: /etc/ginger/tls/cert.pem
  key_file: /etc/ginger/tls/key.pem
  ca_file: /etc/ginger/tls/ca.pem
  client_auth: require  # none, request, require
```

## Multi-Tenancy

```yaml
tenancy:
  enabled: false
  header: X-Tenant-ID
  default_tenant: ""
  allowed_tenants: []
```

## Health

```yaml
health:
  pprof_enabled: false
```
