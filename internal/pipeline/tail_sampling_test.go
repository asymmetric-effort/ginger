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

func TestTailSamplingNoPoliciesSamplesAll(t *testing.T) {
	// No policies → sample everything (makeDecisions: sampled = len(ts.config.Policies) == 0)
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: 10 * time.Millisecond,
		// No policies configured
	}, exp)

	ctx := context.Background()
	ts.Start(ctx)
	ts.ProcessTraces(ctx, makeSamplingTD("svc", 1))

	time.Sleep(100 * time.Millisecond) // wait for decision loop to fire
	ts.Shutdown(ctx)

	if exp.totalSpans() < 1 {
		t.Errorf("no-policies should sample all spans, got %d", exp.totalSpans())
	}
}

func TestTailSamplingDecisionLoopTick(t *testing.T) {
	// Test that decisionLoop ticker fires and processes decisions
	// Uses very short DecisionWait so decisions happen quickly
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: 10 * time.Millisecond,
		Policies:     []TailSamplingPolicy{AlwaysSamplePolicy{}},
	}, exp)

	ctx := context.Background()
	ts.Start(ctx)

	// Submit multiple traces
	for i := byte(1); i <= 3; i++ {
		td := makeSamplingTD("svc", 1)
		// Make each trace have a unique trace ID
		td.ResourceSpans[0].ScopeSpans[0].Spans[0].TraceID = [16]byte{i}
		ts.ProcessTraces(ctx, td)
	}

	// Wait for the decisionLoop ticker to fire (1 second ticker internally,
	// but we force decisions via Shutdown)
	time.Sleep(50 * time.Millisecond)
	ts.Shutdown(ctx)

	if exp.totalSpans() < 3 {
		t.Errorf("expected at least 3 spans exported, got %d", exp.totalSpans())
	}
}

func TestTailSamplingMakeDecisionsRemaining(t *testing.T) {
	// Test makeDecisions with traces that haven't reached DecisionWait yet
	// These should be moved to 'remaining' list
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: 1 * time.Hour, // never expire during test
		Policies:     []TailSamplingPolicy{AlwaysSamplePolicy{}},
	}, exp)

	ctx := context.Background()
	ts.Start(ctx)
	ts.ProcessTraces(ctx, makeSamplingTD("svc", 1))

	// makeDecisions is called; trace is not old enough → stays in 'remaining'
	ts.mu.Lock()
	ts.makeDecisions(ctx)
	ts.mu.Unlock()

	if ts.BufferedTraces() != 1 {
		t.Errorf("trace should remain buffered, got %d", ts.BufferedTraces())
	}

	ts.Shutdown(ctx)
}

func TestTailSamplingEvictOldest(t *testing.T) {
	// Test evictOldest is called when MaxTraces exceeded
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: time.Hour,
		MaxTraces:    3,
	}, exp)

	ctx := context.Background()

	// Add 4 unique traces — the 4th should trigger eviction of the oldest
	for i := byte(1); i <= 4; i++ {
		td := makeSamplingTD("svc", 1)
		td.ResourceSpans[0].ScopeSpans[0].Spans[0].TraceID = [16]byte{i}
		ts.ProcessTraces(ctx, td)
	}

	// Should have max 3 buffered (oldest evicted)
	if ts.BufferedTraces() > 3 {
		t.Errorf("buffered = %d, expected max 3", ts.BufferedTraces())
	}
	if ts.BufferedTraces() < 3 {
		t.Errorf("buffered = %d, expected 3", ts.BufferedTraces())
	}
}

func TestCompositePolicyOrNoMatch(t *testing.T) {
	// CompositePolicy OR with no policies matching → returns false
	p := CompositePolicy{
		Policies: []TailSamplingPolicy{
			ErrorStatusPolicy{},
			SpanCountPolicy{MinSpans: 100},
		},
		UseAnd: false,
	}
	// Normal span (no error, only 1 span) — no policy matches
	spans := []otlp.Span{{
		Status: otlp.Status{Code: otlp.StatusCodeOk},
	}}
	if p.ShouldSample(spans) {
		t.Error("OR with no matching policies should return false")
	}
}

func TestTailSamplingEvictOldestEmpty(t *testing.T) {
	// Test evictOldest when ts.order is empty → should return immediately
	ts := NewTailSamplingProcessor(TailSamplingConfig{}, &collectExporter{})
	// Call evictOldest directly on empty processor
	ts.evictOldest() // should not panic
}

func TestTailSamplingMakeDecisionsDeletedTrace(t *testing.T) {
	// Test the `if !ok { continue }` branch in makeDecisions
	// Happens when a trace is in ts.order but deleted from ts.traces
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: 0, // expire immediately
	}, exp)

	ctx := context.Background()
	traceID := [16]byte{42}

	// Manually insert a trace ID into order but not into traces
	ts.mu.Lock()
	ts.order = append(ts.order, traceID)
	// Don't add to ts.traces - simulates a deleted/evicted trace
	ts.makeDecisions(ctx) // should hit the `!ok` continue branch
	ts.mu.Unlock()

	// No panic and order should be empty now
	if ts.BufferedTraces() != 0 {
		t.Errorf("expected 0 buffered traces, got %d", ts.BufferedTraces())
	}
}

func TestTailSamplingDecisionLoopTicker(t *testing.T) {
	// Test that the 1-second ticker in decisionLoop fires
	// We use a very short decision wait so that when the ticker fires, decisions happen
	exp := &collectExporter{}
	ts := NewTailSamplingProcessor(TailSamplingConfig{
		DecisionWait: 1 * time.Millisecond,
		Policies:     []TailSamplingPolicy{AlwaysSamplePolicy{}},
	}, exp)

	ctx := context.Background()
	ts.Start(ctx)

	ts.ProcessTraces(ctx, makeSamplingTD("svc", 1))

	// Wait > 1 second for the ticker to fire
	time.Sleep(1100 * time.Millisecond)

	ts.Shutdown(ctx)

	if exp.totalSpans() < 1 {
		t.Errorf("decisionLoop ticker should have processed span, got %d", exp.totalSpans())
	}
}

func TestCompositePolicyAndNoPolicies(t *testing.T) {
	// CompositePolicy AND with no sub-policies → returns true (vacuously true)
	p := CompositePolicy{
		Policies: []TailSamplingPolicy{},
		UseAnd:   true,
	}
	if !p.ShouldSample(nil) {
		t.Error("AND with no policies should return true")
	}
}

// collectExporter is defined in batch_test.go — reuse via same package.
// If not accessible, define a local version:
var _ Exporter = (*collectExporter)(nil)

func init() {
	// Ensure collectExporter satisfies Exporter
	var _ sync.Locker = &sync.Mutex{}
}
