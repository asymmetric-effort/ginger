package influxdb

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/asymmetric-effort/ginger/internal/codec/influxlp"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// TraceWriter writes spans to InfluxDB.
type TraceWriter struct {
	client *Client
}

// NewTraceWriter creates a new InfluxDB trace writer.
func NewTraceWriter(client *Client) *TraceWriter {
	return &TraceWriter{client: client}
}

// WriteSpans writes spans to InfluxDB.
func (tw *TraceWriter) WriteSpans(ctx context.Context, td otlp.TracesData) error {
	var points []influxlp.Point

	for _, rs := range td.ResourceSpans {
		service := ""
		if v, ok := rs.Resource.Attributes.Get("service.name"); ok {
			service = v.Str
		}

		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				p := spanToPoint(span, service, rs.Resource, ss.Scope)
				points = append(points, p)
			}
		}
	}

	if len(points) == 0 {
		return nil
	}

	return tw.client.Write(ctx, points)
}

func spanToPoint(span otlp.Span, service string, res otlp.Resource, scope otlp.InstrumentationScope) influxlp.Point {
	traceID := hex.EncodeToString(span.TraceID[:])
	spanID := hex.EncodeToString(span.SpanID[:])
	parentSpanID := hex.EncodeToString(span.ParentSpanID[:])

	tags := []influxlp.Tag{
		{Key: "service", Value: service},
		{Key: "operation", Value: span.Name},
		{Key: "trace_id", Value: traceID},
		{Key: "span_id", Value: spanID},
		{Key: "parent_span_id", Value: parentSpanID},
		{Key: "span_kind", Value: span.Kind.String()},
		{Key: "status_code", Value: span.Status.Code.String()},
	}

	duration := int64(0)
	if span.EndTimeUnixNano > span.StartTimeUnixNano {
		duration = int64(span.EndTimeUnixNano-span.StartTimeUnixNano) / 1000 // ns → us
	}

	attrsJSON, _ := json.Marshal(span.Attributes.Items())
	eventsJSON, _ := json.Marshal(span.Events)
	linksJSON, _ := json.Marshal(span.Links)
	resJSON, _ := json.Marshal(res.Attributes.Items())

	fields := []influxlp.Field{
		influxlp.IntField("duration_us", duration),
		influxlp.StringField("span_name", span.Name),
		influxlp.StringField("attributes", string(attrsJSON)),
		influxlp.StringField("events", string(eventsJSON)),
		influxlp.StringField("links", string(linksJSON)),
		influxlp.StringField("resource", string(resJSON)),
	}

	if scope.Name != "" {
		fields = append(fields, influxlp.StringField("scope_name", scope.Name))
	}

	return influxlp.Point{
		Measurement: "traces",
		Tags:        tags,
		Fields:      fields,
		Timestamp:   timeFromNano(span.StartTimeUnixNano),
	}
}
