# Architecture

## System Overview

```
                    ┌─────────────┐
                    │   Clients   │
                    │ (SDKs/Agents)│
                    └──────┬──────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
    ┌───────▼───┐  ┌───────▼───┐  ┌───────▼───┐
    │OTLP gRPC  │  │OTLP HTTP  │  │Zipkin HTTP│
    │  :4317    │  │  :4318    │  │  :9411    │
    └───────┬───┘  └───────┬───┘  └───────┬───┘
            │              │              │
            └──────────────┼──────────────┘
                           │
                    ┌──────▼──────┐
                    │  Pipeline   │
                    │ (Processors)│
                    │ Batch/Filter│
                    │ Sampling    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  InfluxDB   │
                    │  Storage    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Query     │
                    │  Service    │
                    │ :16686      │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │     UI      │
                    └─────────────┘
```

## Components

### Receivers
Accept trace data via OTLP (gRPC/HTTP), Jaeger (gRPC/Thrift/UDP), and Zipkin protocols. All data is converted to OTLP internal format.

### Pipeline
Bounded queue between receivers and exporters. Processors include batch (size/time flushing), memory limiter, filter (include/exclude), attributes (insert/update/delete), and sampling (adaptive head-based, policy-based tail).

### Storage
InfluxDB v2 backend with line protocol encoding. Schema uses tags for indexed fields (service, operation, trace_id) and fields for data (duration, attributes JSON). In-memory backend for development.

### Query Service
REST API (v3 and legacy v2) with trace adjusters (clock skew, dedup, IP normalization, sort). Serves the web UI.

### Security
TLS 1.3 with hybrid PQC key exchange (X25519+ML-KEM-768). mTLS between components. Bearer token, API key, and basic auth middleware.

### Multi-Tenancy
HTTP header-based tenant extraction with context propagation. Storage isolation via tenant tag injection and query filtering.

## Data Flow

1. Client SDK sends spans via OTLP gRPC to receiver
2. Receiver converts to internal OTLP model, submits to pipeline queue
3. Pipeline processors: batch accumulation → filter → sampling
4. Exporter writes to InfluxDB via line protocol
5. Query service reads from InfluxDB, applies adjusters, serves API
6. UI fetches from query API and renders trace views
