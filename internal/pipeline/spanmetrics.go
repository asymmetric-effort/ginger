package pipeline

import (
	"context"
	"time"

	"github.com/asymmetric-effort/ginger/internal/metrics"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// SpanmetricsConnector generates RED metrics from span data flowing through the pipeline.
type SpanmetricsConnector struct {
	calls    *metrics.Counter
	errors   *metrics.Counter
	duration *metrics.Histogram
}

// NewSpanmetricsConnector creates a spanmetrics connector.
func NewSpanmetricsConnector(registry *metrics.Registry) *SpanmetricsConnector {
	buckets := []float64{2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10000, 15000}
	calls := metrics.NewCounter("calls_total", "Total span calls", "service", "operation", "span_kind", "status_code")
	errs := metrics.NewCounter("errors_total", "Total error spans", "service", "operation")
	dur := metrics.NewHistogram("duration_milliseconds", "Span duration in ms", buckets, "service", "operation")

	registry.MustRegister(calls)
	registry.MustRegister(errs)
	registry.MustRegister(dur)

	return &SpanmetricsConnector{calls: calls, errors: errs, duration: dur}
}

// ProcessTraces observes spans without modifying them — read-only tap.
func (sc *SpanmetricsConnector) ProcessTraces(_ context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	for _, rs := range td.ResourceSpans {
		service := ""
		if v, ok := rs.Resource.Attributes.Get("service.name"); ok {
			service = v.Str
		}
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				sc.observeSpan(service, span)
			}
		}
	}
	return td, nil // pass through unmodified
}

func (sc *SpanmetricsConnector) observeSpan(service string, span otlp.Span) {
	kind := span.Kind.String()
	status := span.Status.Code.String()

	sc.calls.Inc(service, span.Name, kind, status)

	if span.Status.Code == otlp.StatusCodeError {
		sc.errors.Inc(service, span.Name)
	}

	durMs := float64(span.EndTimeUnixNano-span.StartTimeUnixNano) / float64(time.Millisecond)
	sc.duration.Observe(durMs, service, span.Name)
}
