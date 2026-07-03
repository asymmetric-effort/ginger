# API Reference

## Query API v3

### Get Trace
```
GET /api/v3/traces/{traceID}
```
Returns a single trace by 32-character hex trace ID.

**Response:** `TracesData` JSON

### Find Traces
```
GET /api/v3/traces?service=<name>&operation=<name>&limit=<n>&start=<us>&end=<us>&minDuration=<duration>&maxDuration=<duration>&tags=<key:value|...>
```

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| service | string | Filter by service name |
| operation | string | Filter by operation name |
| limit | int | Max results (default 20) |
| start | int64 | Start time (microseconds since epoch) |
| end | int64 | End time (microseconds since epoch) |
| minDuration | duration | Minimum span duration (e.g. "1ms") |
| maxDuration | duration | Maximum span duration |
| tags | string | Tag filters: `key:value\|key2:value2` or JSON |

### Get Services
```
GET /api/v3/services
```
Returns `[]string` of service names.

### Get Operations
```
GET /api/v3/operations?service=<name>
```
Returns operations for the given service.

### Get Dependencies
```
GET /api/v3/dependencies?endTs=<ms>&lookback=<ms>
```
Returns service dependency graph.

## Query API v2 (Legacy)

All v2 endpoints return the Jaeger envelope format:
```json
{"data": [...], "total": N, "limit": N, "errors": []}
```

| v3 Endpoint | v2 Endpoint |
|-------------|-------------|
| /api/v3/traces/{id} | /api/traces/{id} |
| /api/v3/traces | /api/traces |
| /api/v3/services | /api/services |
| /api/v3/operations | /api/operations |
| /api/v3/dependencies | /api/dependencies |

## Sampling API

```
GET /sampling?service=<name>
```
Returns sampling strategy for the service.

## Health Endpoints

| Endpoint | Description |
|----------|-------------|
| GET /health/live | Liveness probe (always 200) |
| GET /health/ready | Readiness probe (200 when ready, 503 otherwise) |
| GET /debug/pprof/ | Go pprof (when enabled) |
