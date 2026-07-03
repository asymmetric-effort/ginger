package pipeline

import (
	"context"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/metrics"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func TestSpanmetricsConnector(t *testing.T) {
	reg := metrics.NewRegistry()
	sc := NewSpanmetricsConnector(reg)

	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("frontend"))

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{
					{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "GET /api",
						Kind: otlp.SpanKindServer, Status: otlp.Status{Code: otlp.StatusCodeOk},
						StartTimeUnixNano: 1000000000, EndTimeUnixNano: 1100000000},
					{TraceID: [16]byte{2}, SpanID: [8]byte{2}, Name: "POST /api",
						Kind: otlp.SpanKindServer, Status: otlp.Status{Code: otlp.StatusCodeError},
						StartTimeUnixNano: 1000000000, EndTimeUnixNano: 1500000000},
				},
			}},
		}},
	}

	result, err := sc.ProcessTraces(context.Background(), td)
	if err != nil {
		t.Fatal(err)
	}

	// Verify pass-through
	count := 0
	for _, rs := range result.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	if count != 2 {
		t.Errorf("should pass through all spans, got %d", count)
	}
}

func TestSpanmetricsNoService(t *testing.T) {
	reg := metrics.NewRegistry()
	sc := NewSpanmetricsConnector(reg)

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}

	_, err := sc.ProcessTraces(context.Background(), td)
	if err != nil {
		t.Fatal(err)
	}
}
