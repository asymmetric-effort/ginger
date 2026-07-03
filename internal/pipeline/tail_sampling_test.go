package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func TestAlwaysSamplePolicy(t *testing.T) {
	p := AlwaysSamplePolicy{}
	if !p.ShouldSample(nil) {
		t.Error("should always sample")
	}
}

func TestErrorStatusPolicy(t *testing.T) {
	p := ErrorStatusPolicy{}
	if p.ShouldSample([]otlp.Span{{Status: otlp.Status{Code: otlp.StatusCodeOk}}}) {
		t.Error("should not sample OK spans")
	}
	if !p.ShouldSample([]otlp.Span{{Status: otlp.Status{Code: otlp.StatusCodeError}}}) {
		t.Error("should sample error spans")
	}
}

func TestLatencyPolicy(t *testing.T) {
	p := LatencyPolicy{MinDuration: time.Second}
	fast := []otlp.Span{{StartTimeUnixNano: 1000000000, EndTimeUnixNano: 1500000000}} // 0.5s
	slow := []otlp.Span{{StartTimeUnixNano: 1000000000, EndTimeUnixNano: 3000000000}} // 2s
	if p.ShouldSample(fast) {
		t.Error("should not sample fast spans")
	}
	if !p.ShouldSample(slow) {
		t.Error("should sample slow spans")
	}
}

func TestSpanCountPolicy(t *testing.T) {
	p := SpanCountPolicy{MinSpans: 3}
	if p.ShouldSample([]otlp.Span{{}, {}}) {
		t.Error("2 spans < 3")
	}
	if !p.ShouldSample([]otlp.Span{{}, {}, {}}) {
		t.Error("3 spans >= 3")
	}
}

func TestCompositePolicyOr(t *testing.T) {
	p := CompositePolicy{
		Policies: []TailSamplingPolicy{
			ErrorStatusPolicy{},
			SpanCountPolicy{MinSpans: 5},
		},
		UseAnd: false,
	}
	// Error span — matches first policy
	if !p.ShouldSample([]otlp.Span{{Status: otlp.Status{Code: otlp.StatusCodeError}}}) {
		t.Error("OR: should match error policy")
	}
}

func TestCompositePolicyAnd(t *testing.T) {
	p := CompositePolicy{
		Policies: []TailSamplingPolicy{
			ErrorStatusPolicy{},
			SpanCountPolicy{MinSpans: 2},
		},
		UseAnd: true,
	}
	// Error but only 1 span — fails span count
	if p.ShouldSample([]otlp.Span{{Status: otlp.Status{Code: otlp.StatusCodeError}}}) {
		t.Error("AND: should require both")
	}
	// Error and 2 spans — passes both
	if !p.ShouldSample([]otlp.Span{
		{Status: otlp.Status{Code: otlp.StatusCodeError}},
		{Status: otlp.Status{Code: otlp.StatusCodeOk}},
	}) {
		t.Error("AND: should pass both")
	}
}

func TestTailSamplingProcessorBasic(t *testing.T) {
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: 50 * time.Millisecond,
		Policies:     []TailSamplingPolicy{AlwaysSamplePolicy{}},
	}, exp)

	ctx := context.Background()
	ts.Start(ctx)

	td := makeSamplingTD("svc", 1)
	ts.ProcessTraces(ctx, td)

	if ts.BufferedTraces() != 1 {
		t.Errorf("buffered = %d", ts.BufferedTraces())
	}

	// Wait for decision
	time.Sleep(200 * time.Millisecond)

	ts.Shutdown(ctx)

	if exp.totalSpans() < 1 {
		t.Errorf("expected at least 1 exported span, got %d", exp.totalSpans())
	}
}

func TestTailSamplingProcessorFilters(t *testing.T) {
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: 50 * time.Millisecond,
		Policies:     []TailSamplingPolicy{ErrorStatusPolicy{}},
	}, exp)

	ctx := context.Background()
	ts.Start(ctx)

	// OK span — should NOT be sampled
	td := otlp.TracesData{ResourceSpans: []otlp.ResourceSpans{{
		ScopeSpans: []otlp.ScopeSpans{{Spans: []otlp.Span{{
			TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "ok",
			Status: otlp.Status{Code: otlp.StatusCodeOk},
		}}}},
	}}}
	ts.ProcessTraces(ctx, td)

	time.Sleep(200 * time.Millisecond)
	ts.Shutdown(ctx)

	if exp.totalSpans() != 0 {
		t.Errorf("OK trace should not be sampled, got %d spans", exp.totalSpans())
	}
}

func TestTailSamplingProcessorEviction(t *testing.T) {
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: time.Hour, // never decide during test
		MaxTraces:    2,
	}, exp)

	ctx := context.Background()

	for i := byte(1); i <= 3; i++ {
		ts.ProcessTraces(ctx, makeSamplingTD("svc", 1))
	}

	// Only 2 should remain (oldest evicted)
	if ts.BufferedTraces() > 2 {
		t.Errorf("buffered = %d, max 2", ts.BufferedTraces())
	}
}

func TestTailSamplingDefaults(t *testing.T) {
	ts := NewTailSamplingProcessor(TailSamplingConfig{}, &collectExporter{})
	if ts.config.DecisionWait != 30*time.Second {
		t.Error("default DecisionWait")
	}
	if ts.config.MaxTraces != 10000 {
		t.Error("default MaxTraces")
	}
}

// collectExporter is defined in batch_test.go — reuse via same package.
// If not accessible, define a local version:
var _ Exporter = (*collectExporter)(nil)

func init() {
	// Ensure collectExporter satisfies Exporter
	var _ sync.Locker = &sync.Mutex{}
}
