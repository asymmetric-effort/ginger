# gRPC Service Reference

## TraceService

Service name: `opentelemetry.proto.collector.trace.v1.TraceService`

### Methods

#### Export (Unary)

Receives a batch of OTLP trace data.

```
rpc Export(ExportTraceServiceRequest) returns (ExportTraceServiceResponse)
```

**ExportTraceServiceRequest** protobuf fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| resource_spans | 1 | repeated ResourceSpans | Batch of trace data |

**ResourceSpans** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| resource | 1 | Resource | Entity producing telemetry |
| scope_spans | 2 | repeated ScopeSpans | Spans grouped by scope |

**Resource** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| attributes | 1 | repeated KeyValue | Resource attributes |

**ScopeSpans** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| scope | 1 | InstrumentationScope | Library identity |
| spans | 2 | repeated Span | Span records |

**InstrumentationScope** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| name | 1 | string | Library name |
| version | 2 | string | Library version |
| attributes | 3 | repeated KeyValue | Scope attributes |

**Span** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| trace_id | 1 | bytes (16) | Trace identifier |
| span_id | 2 | bytes (8) | Span identifier |
| trace_state | 3 | string | W3C tracestate |
| parent_span_id | 4 | bytes (8) | Parent span ID |
| name | 5 | string | Operation name |
| kind | 6 | SpanKind enum | Span kind (0-5) |
| start_time_unix_nano | 7 | fixed64 | Start time |
| end_time_unix_nano | 8 | fixed64 | End time |
| attributes | 9 | repeated KeyValue | Span attributes |
| events | 11 | repeated SpanEvent | Timed annotations |
| links | 12 | repeated SpanLink | Causal links |
| status | 13 | Status | Span status |
| dropped_attributes_count | 14 | uint32 | Dropped attributes |
| dropped_events_count | 15 | uint32 | Dropped events |
| dropped_links_count | 16 | uint32 | Dropped links |

**SpanEvent** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| time_unix_nano | 1 | fixed64 | Event timestamp |
| name | 2 | string | Event name |
| attributes | 3 | repeated KeyValue | Event attributes |
| dropped_attributes_count | 4 | uint32 | Dropped attributes |

**SpanLink** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| trace_id | 1 | bytes (16) | Linked trace ID |
| span_id | 2 | bytes (8) | Linked span ID |
| trace_state | 3 | string | W3C tracestate |
| attributes | 4 | repeated KeyValue | Link attributes |
| dropped_attributes_count | 5 | uint32 | Dropped attributes |

**Status** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| message | 2 | string | Status description |
| code | 3 | StatusCode enum | 0=UNSET, 1=OK, 2=ERROR |

**KeyValue** fields:

| Field | Number | Type | Description |
|-------|--------|------|-------------|
| key | 1 | string | Attribute key |
| value | 2 | AnyValue | Attribute value |

**AnyValue** fields (oneof):

| Field | Number | Type |
|-------|--------|------|
| string_value | 1 | string |
| bool_value | 2 | bool |
| int_value | 3 | int64 |
| double_value | 4 | double |
| array_value | 5 | ArrayValue |
| kvlist_value | 6 | KeyValueList |
| bytes_value | 7 | bytes |

**SpanKind** enum values:

| Value | Name |
|-------|------|
| 0 | UNSPECIFIED |
| 1 | INTERNAL |
| 2 | SERVER |
| 3 | CLIENT |
| 4 | PRODUCER |
| 5 | CONSUMER |

### QueryService

The Ginger query API is exposed as HTTP REST endpoints rather than gRPC streaming.
See [api-v1.md](api-v1.md) for the full REST query reference (v2 and v3 APIs).

The following query methods are available:

| Method | HTTP Endpoint | Description |
|--------|--------------|-------------|
| GetTrace | GET /api/v3/traces/{traceID} | Retrieve a single trace |
| FindTraces | GET /api/v3/traces?... | Search traces by filters |
| GetServices | GET /api/v3/services | List service names |
| GetOperations | GET /api/v3/operations?service=... | List operations for a service |
| GetDependencies | GET /api/v3/dependencies?... | Service dependency graph |

## Error Codes

Ginger uses standard gRPC status codes:

| Code | Name | Description |
|------|------|-------------|
| 0 | OK | Success |
| 3 | INVALID_ARGUMENT | Malformed request (e.g., bad protobuf) |
| 8 | RESOURCE_EXHAUSTED | Pipeline queue full, spans rejected |
| 12 | UNIMPLEMENTED | Unknown method path |
| 13 | INTERNAL | Unexpected server error |

## Wire Format

gRPC messages use HTTP/2 with `application/grpc+proto` content type.
Each message is framed as:

```
1 byte  - compressed flag (0 = uncompressed, 1 = gzip)
4 bytes - message length (big-endian uint32)
N bytes - protobuf payload
```

Responses include gRPC status in HTTP trailers:
- `Grpc-Status`: numeric status code
- `Grpc-Message`: human-readable error description

Timeouts are conveyed via the `Grpc-Timeout` header using the format `<value><unit>`
where unit is one of: `n` (nanoseconds), `u` (microseconds), `m` (milliseconds),
`S` (seconds), `M` (minutes), `H` (hours).
