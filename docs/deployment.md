# Deployment Guide

## Docker

```bash
docker build -t ginger .
docker run -p 4317:4317 -p 4318:4318 -p 16686:16686 \
  -e INFLUXDB_ENDPOINT=http://influxdb:8086 \
  -e INFLUXDB_TOKEN=my-token \
  ginger --config /etc/ginger/config.yaml
```

## Kubernetes

See `deploy/kubernetes/` for plain manifests or `deploy/helm/ginger/` for the Helm chart.

```bash
helm install ginger deploy/helm/ginger \
  --set storage.influxdb.endpoint=http://influxdb:8086 \
  --set storage.influxdb.token=my-token
```

## Security Hardening

### PQC TLS Certificate Generation

```bash
# Generate ECDSA P-256 key pair (ML-DSA pending Go stdlib support)
openssl ecparam -genkey -name prime256v1 -out server.key
openssl req -new -x509 -key server.key -out server.crt -days 365
```

### mTLS Setup

```yaml
tls:
  enabled: true
  cert_file: /etc/ginger/tls/server.crt
  key_file: /etc/ginger/tls/server.key
  ca_file: /etc/ginger/tls/ca.crt
  client_auth: require
```

### API Key Configuration

```yaml
auth:
  api_keys:
    - ${GINGER_API_KEY}
```

## CIS Benchmark Checklist

- [x] Non-root container user (UID 10001)
- [x] Read-only root filesystem
- [x] No shell in runtime image (distroless)
- [x] Resource limits set
- [x] TLS 1.3 minimum
- [x] No default credentials
- [x] Network policies applied
