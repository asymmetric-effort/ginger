package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// TailSamplingPolicy decides whether to sample a complete trace.
type TailSamplingPolicy interface {
	ShouldSample(spans []otlp.Span) bool
}

// AlwaysSamplePolicy always samples.
type AlwaysSamplePolicy struct{}

// ShouldSample always returns true.
func (p AlwaysSamplePolicy) ShouldSample(_ []otlp.Span) bool { return true }

// ErrorStatusPolicy samples traces with error spans.
type ErrorStatusPolicy struct{}

// ShouldSample returns true if any span has error status.
func (p ErrorStatusPolicy) ShouldSample(spans []otlp.Span) bool {
	for _, s := range spans {
		if s.Status.Code == otlp.StatusCodeError {
			return true
		}
	}
	return false
}

// LatencyPolicy samples traces exceeding a duration threshold.
type LatencyPolicy struct {
	MinDuration time.Duration
}

// ShouldSample returns true if any span exceeds the threshold.
func (p LatencyPolicy) ShouldSample(spans []otlp.Span) bool {
	for _, s := range spans {
		dur := time.Duration(s.EndTimeUnixNano-s.StartTimeUnixNano) * time.Nanosecond
		if dur >= p.MinDuration {
			return true
		}
	}
	return false
}

// SpanCountPolicy samples traces with more than N spans.
type SpanCountPolicy struct {
	MinSpans int
}

// ShouldSample returns true if the trace has enough spans.
func (p SpanCountPolicy) ShouldSample(spans []otlp.Span) bool {
	return len(spans) >= p.MinSpans
}

// CompositePolicy combines policies with AND or OR logic.
type CompositePolicy struct {
	Policies []TailSamplingPolicy
	UseAnd   bool // true=AND, false=OR
}

// ShouldSample evaluates all sub-policies.
func (p CompositePolicy) ShouldSample(spans []otlp.Span) bool {
	if p.UseAnd {
		for _, sub := range p.Policies {
			if !sub.ShouldSample(spans) {
				return false
			}
		}
		return true
	}
	for _, sub := range p.Policies {
		if sub.ShouldSample(spans) {
			return true
		}
	}
	return false
}

// TailSamplingConfig configures the tail sampling processor.
type TailSamplingConfig struct {
	DecisionWait time.Duration
	MaxTraces    int
	Policies     []TailSamplingPolicy
}

// TailSamplingProcessor buffers complete traces before sampling.
type TailSamplingProcessor struct {
	config   TailSamplingConfig
	exporter Exporter
	mu       sync.Mutex
	traces   map[[16]byte]*traceBuffer
	order    [][16]byte
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

type traceBuffer struct {
	spans    []otlp.Span
	resource otlp.Resource
	scope    otlp.InstrumentationScope
	firstSeen time.Time
}

// NewTailSamplingProcessor creates a tail sampling processor.
func NewTailSamplingProcessor(config TailSamplingConfig, exporter Exporter) *TailSamplingProcessor {
	if config.DecisionWait <= 0 {
		config.DecisionWait = 30 * time.Second
	}
	if config.MaxTraces <= 0 {
		config.MaxTraces = 10000
	}
	return &TailSamplingProcessor{
		config:   config,
		exporter: exporter,
		traces:   make(map[[16]byte]*traceBuffer),
	}
}

// Start begins the decision timer.
func (ts *TailSamplingProcessor) Start(ctx context.Context) {
	ctx, ts.cancel = context.WithCancel(ctx)
	ts.wg.Add(1)
	go ts.decisionLoop(ctx)
}

// ProcessTraces buffers spans for later decision.
func (ts *TailSamplingProcessor) ProcessTraces(_ context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for _, rs := range td.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				tb, ok := ts.traces[span.TraceID]
				if !ok {
					if len(ts.traces) >= ts.config.MaxTraces {
						ts.evictOldest()
					}
					tb = &traceBuffer{
						resource:  rs.Resource,
						scope:     ss.Scope,
						firstSeen: time.Now(),
					}
					ts.traces[span.TraceID] = tb
					ts.order = append(ts.order, span.TraceID)
				}
				tb.spans = append(tb.spans, span)
			}
		}
	}
	return otlp.TracesData{}, nil // consumed, not forwarded yet
}

// Shutdown flushes and makes final decisions.
func (ts *TailSamplingProcessor) Shutdown(ctx context.Context) {
	if ts.cancel != nil {
		ts.cancel()
	}
	ts.wg.Wait()
	// Make final decisions on remaining traces
	ts.mu.Lock()
	ts.makeDecisions(ctx)
	ts.mu.Unlock()
}

// BufferedTraces returns the number of buffered traces.
func (ts *TailSamplingProcessor) BufferedTraces() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.traces)
}

func (ts *TailSamplingProcessor) decisionLoop(ctx context.Context) {
	defer ts.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ts.mu.Lock()
			ts.makeDecisions(ctx)
			ts.mu.Unlock()
		}
	}
}

func (ts *TailSamplingProcessor) makeDecisions(ctx context.Context) {
	now := time.Now()
	var remaining [][16]byte

	for _, tid := range ts.order {
		tb, ok := ts.traces[tid]
		if !ok {
			continue
		}
		if now.Sub(tb.firstSeen) < ts.config.DecisionWait {
			remaining = append(remaining, tid)
			continue
		}

		// Apply policies
		sampled := len(ts.config.Policies) == 0 // no policies = sample all
		for _, policy := range ts.config.Policies {
			if policy.ShouldSample(tb.spans) {
				sampled = true
				break
			}
		}

		if sampled {
			td := otlp.TracesData{
				ResourceSpans: []otlp.ResourceSpans{{
					Resource:   tb.resource,
					ScopeSpans: []otlp.ScopeSpans{{Scope: tb.scope, Spans: tb.spans}},
				}},
			}
			ts.exporter.ExportTraces(ctx, td)
		}

		delete(ts.traces, tid)
	}

	ts.order = remaining
}

func (ts *TailSamplingProcessor) evictOldest() {
	if len(ts.order) == 0 {
		return
	}
	oldest := ts.order[0]
	ts.order = ts.order[1:]
	delete(ts.traces, oldest)
}
