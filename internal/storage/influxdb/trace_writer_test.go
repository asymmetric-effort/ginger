package influxdb

import (
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

func TestSpanToPoint(t *testing.T) {
	attrs := otlp.NewAttributes()
	attrs.Set("http.method", otlp.StringValue("GET"))

	span := otlp.Span{
		TraceID:           [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:            [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		Name:              "GET /api",
		Kind:              otlp.SpanKindServer,
		StartTimeUnixNano: 1000000000,
		EndTimeUnixNano:   2000000000,
		Attributes:        attrs,
		Status:            otlp.Status{Code: otlp.StatusCodeOk},
	}

	res := otlp.Resource{}
	scope := otlp.InstrumentationScope{Name: "ginger"}

	p := spanToPoint(span, "test-svc", res, scope)

	if p.Measurement != "traces" {
		t.Errorf("measurement = %q", p.Measurement)
	}
	if len(p.Tags) != 7 {
		t.Errorf("tags = %d", len(p.Tags))
	}

	// Check service tag
	found := false
	for _, tag := range p.Tags {
		if tag.Key == "service" && tag.Value == "test-svc" {
			found = true
		}
	}
	if !found {
		t.Error("missing service tag")
	}

	// Check duration field
	for _, f := range p.Fields {
		if f.Key == "duration_us" && f.Value.Int != 1000000 {
			t.Errorf("duration_us = %d, want 1000000", f.Value.Int)
		}
	}
}

func TestSpanToPointNoScope(t *testing.T) {
	span := otlp.Span{
		TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
		StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
	}
	p := spanToPoint(span, "svc", otlp.Resource{}, otlp.InstrumentationScope{})
	// Should not have scope_name field
	for _, f := range p.Fields {
		if f.Key == "scope_name" {
			t.Error("should not have scope_name when empty")
		}
	}
}

func TestBuildFindTracesQuery(t *testing.T) {
	q := BuildFindTracesQuery("mybucket", storage.TraceQueryParameters{
		ServiceName: "svc",
		NumTraces:   10,
	})
	if q == "" {
		t.Error("empty query")
	}
	if !strings.Contains(q, `"mybucket"`) {
		t.Error("missing bucket")
	}
	if !strings.Contains(q, `service == "svc"`) {
		t.Error("missing service filter")
	}
	if !strings.Contains(q, "limit(n: 10)") {
		t.Error("missing limit")
	}
}

func TestBuildFindTracesQueryWithTime(t *testing.T) {
	now := time.Now()
	q := BuildFindTracesQuery("b", storage.TraceQueryParameters{
		StartTimeMin: now.Add(-time.Hour),
		StartTimeMax: now,
	})
	if !strings.Contains(q, "range(start:") {
		t.Error("missing time range")
	}
}

func TestBuildFindTracesQueryDefaults(t *testing.T) {
	q := BuildFindTracesQuery("b", storage.TraceQueryParameters{})
	if !strings.Contains(q, "limit(n: 20)") {
		t.Error("default limit should be 20")
	}
}

func TestEscapeFlux(t *testing.T) {
	if escapeFlux(`test"val`) != `test\"val` {
		t.Error("should escape quotes")
	}
}

func TestTimeFromNano(t *testing.T) {
	ts := timeFromNano(1000000000)
	if ts.Unix() != 1 {
		t.Errorf("time = %v", ts)
	}
}
