package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func makeSamplingTD(service string, n int) otlp.TracesData {
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue(service))
	spans := make([]otlp.Span, n)
	for i := range spans {
		spans[i] = otlp.Span{
			TraceID: [16]byte{byte(i + 1)}, SpanID: [8]byte{byte(i + 1)},
			Name: "op", StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
		}
	}
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource:   otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{Spans: spans}},
		}},
	}
}

func TestAdaptiveSamplingFullRate(t *testing.T) {
	as := NewAdaptiveSamplingProcessor(AdaptiveSamplingConfig{InitialSamplingRate: 1.0})
	td := makeSamplingTD("svc", 10)
	result, err := as.ProcessTraces(context.Background(), td)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, rs := range result.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	if count != 10 {
		t.Errorf("at 100%% rate, expected 10, got %d", count)
	}
}

func TestAdaptiveSamplingZeroRate(t *testing.T) {
	as := NewAdaptiveSamplingProcessor(AdaptiveSamplingConfig{InitialSamplingRate: 0.0001})
	// Force rate to 0
	as.mu.Lock()
	as.rates["svc"] = 0
	as.mu.Unlock()

	td := makeSamplingTD("svc", 10)
	result, _ := as.ProcessTraces(context.Background(), td)
	count := 0
	for _, rs := range result.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	if count != 0 {
		t.Errorf("at 0%% rate, expected 0, got %d", count)
	}
}

func TestAdaptiveSamplingDeterministic(t *testing.T) {
	as := NewAdaptiveSamplingProcessor(AdaptiveSamplingConfig{InitialSamplingRate: 0.5})
	td := makeSamplingTD("svc", 1)

	// Same trace ID should always get same decision
	r1, _ := as.ProcessTraces(context.Background(), td)
	r2, _ := as.ProcessTraces(context.Background(), td)
	c1, c2 := 0, 0
	for _, rs := range r1.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			c1 += len(ss.Spans)
		}
	}
	for _, rs := range r2.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			c2 += len(ss.Spans)
		}
	}
	if c1 != c2 {
		t.Error("deterministic sampling should give same result")
	}
}

func TestAdaptiveSamplingGetRate(t *testing.T) {
	as := NewAdaptiveSamplingProcessor(AdaptiveSamplingConfig{InitialSamplingRate: 0.5})
	if as.GetRate("unknown") != 0.5 {
		t.Error("unknown service should use initial rate")
	}
	as.mu.Lock()
	as.rates["svc"] = 0.1
	as.mu.Unlock()
	if as.GetRate("svc") != 0.1 {
		t.Error("known service should use stored rate")
	}
}

func TestAdaptiveSamplingStartShutdown(t *testing.T) {
	as := NewAdaptiveSamplingProcessor(AdaptiveSamplingConfig{AdjustInterval: 50 * time.Millisecond})
	as.Start(context.Background())
	as.ProcessTraces(context.Background(), makeSamplingTD("svc", 100))
	time.Sleep(100 * time.Millisecond)
	as.Shutdown()
}

func TestAdaptiveSamplingDefaults(t *testing.T) {
	as := NewAdaptiveSamplingProcessor(AdaptiveSamplingConfig{})
	if as.config.TargetSamplesPerSecond != 1 {
		t.Error("default target")
	}
	if as.config.InitialSamplingRate != 1.0 {
		t.Error("default rate")
	}
}

func TestShouldSample(t *testing.T) {
	tid := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	if !shouldSample(tid, 1.0) {
		t.Error("rate 1.0 should always sample")
	}
	if shouldSample(tid, 0) {
		t.Error("rate 0 should never sample")
	}
}
