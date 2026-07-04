package influxdb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/asymmetric-effort/ginger/internal/storage"
)

// TraceReader reads traces from InfluxDB.
type TraceReader struct {
	client *Client
	bucket string
}

// NewTraceReader creates a new InfluxDB trace reader.
func NewTraceReader(client *Client, bucket string) *TraceReader {
	return &TraceReader{client: client, bucket: bucket}
}

// GetServices returns all service names from InfluxDB.
func (tr *TraceReader) GetServices(ctx context.Context) ([]string, error) {
	flux := fmt.Sprintf(`import "influxdata/influxdb/schema"
schema.tagValues(bucket: "%s", tag: "service")`, tr.bucket)

	records, err := tr.client.Query(ctx, flux)
	if err != nil {
		return nil, err
	}

	var services []string
	for _, r := range records {
		if v, ok := r.Values["_value"]; ok && v != "" {
			services = append(services, v)
		}
	}
	return services, nil
}

// GetOperations returns operations for a service.
func (tr *TraceReader) GetOperations(ctx context.Context, service string) ([]storage.Operation, error) {
	flux := fmt.Sprintf(`from(bucket: "%s")
  |> range(start: -24h)
  |> filter(fn: (r) => r._measurement == "traces" and r.service == "%s")
  |> keep(columns: ["operation", "span_kind"])
  |> distinct(column: "operation")`, tr.bucket, escapeFlux(service))

	records, err := tr.client.Query(ctx, flux)
	if err != nil {
		return nil, err
	}

	var ops []storage.Operation
	seen := make(map[string]struct{})
	for _, r := range records {
		op := r.Values["operation"]
		if op == "" {
			continue
		}
		if _, exists := seen[op]; exists {
			continue
		}
		seen[op] = struct{}{}
		ops = append(ops, storage.Operation{
			Name:     op,
			SpanKind: r.Values["span_kind"],
		})
	}
	return ops, nil
}

// BuildFindTracesQuery builds a Flux query for FindTraces.
func BuildFindTracesQuery(bucket string, query storage.TraceQueryParameters) string {
	var b strings.Builder
	fmt.Fprintf(&b, `from(bucket: "%s")`, bucket)

	// Time range
	if !query.StartTimeMin.IsZero() {
		fmt.Fprintf(&b, `  |> range(start: %s`, query.StartTimeMin.UTC().Format(time.RFC3339))
		if !query.StartTimeMax.IsZero() {
			fmt.Fprintf(&b, `, stop: %s`, query.StartTimeMax.UTC().Format(time.RFC3339))
		}
		b.WriteString(")")
	} else {
		b.WriteString(`  |> range(start: -24h)`)
	}

	b.WriteString(`  |> filter(fn: (r) => r._measurement == "traces")`)

	if query.ServiceName != "" {
		fmt.Fprintf(&b, `  |> filter(fn: (r) => r.service == "%s")`, escapeFlux(query.ServiceName))
	}
	if query.OperationName != "" {
		fmt.Fprintf(&b, `  |> filter(fn: (r) => r.operation == "%s")`, escapeFlux(query.OperationName))
	}

	limit := query.NumTraces
	if limit <= 0 {
		limit = 20
	}
	fmt.Fprintf(&b, `  |> limit(n: %d)`, limit)

	return b.String()
}

func escapeFlux(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func timeFromNano(ns uint64) time.Time {
	return time.Unix(0, int64(ns))
}
