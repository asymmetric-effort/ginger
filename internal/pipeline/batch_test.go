package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

type collectExporter struct {
	mu     sync.Mutex
	traces []otlp.TracesData
}

func (e *collectExporter) ExportTraces(_ context.Context, td otlp.TracesData) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.traces = append(e.traces, td)
	return nil
}

func (e *collectExporter) Shutdown(_ context.Context) error { return nil }

func (e *collectExporter) totalSpans() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	total := 0
	for _, td := range e.traces {
		for _, rs := range td.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				total += len(ss.Spans)
			}
		}
	}
	return total
}

func makeTDWithSpans(n int) otlp.TracesData {
	spans := make([]otlp.Span, n)
	for i := range spans {
		spans[i] = otlp.Span{
			TraceID: [16]byte{byte(i + 1)}, SpanID: [8]byte{byte(i + 1)},
			Name: "op", StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
		}
	}
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{Spans: spans}},
		}},
	}
}

func TestBatchProcessorFlushOnSize(t *testing.T) {
	exp := &collectExporter{}
	bp := NewBatchProcessor(BatchConfig{MaxBatchSize: 5, SendDelay: time.Hour}, exp)

	ctx := context.Background()
	// Send exactly 5 spans — should trigger immediate flush
	bp.ProcessTraces(ctx, makeTDWithSpans(5))

	if exp.totalSpans() != 5 {
		t.Errorf("expected 5 spans flushed, got %d", exp.totalSpans())
	}
}

func TestBatchProcessorFlushOnTimer(t *testing.T) {
	exp := &collectExporter{}
	bp := NewBatchProcessor(BatchConfig{MaxBatchSize: 100, SendDelay: 50 * time.Millisecond}, exp)

	ctx := context.Background()
	bp.ProcessTraces(ctx, makeTDWithSpans(3))

	// Should not be flushed immediately
	if exp.totalSpans() != 0 {
		t.Error("should not flush before timer")
	}

	time.Sleep(150 * time.Millisecond)

	if exp.totalSpans() != 3 {
		t.Errorf("expected 3 spans after timer, got %d", exp.totalSpans())
	}
}

func TestBatchProcessorShutdownFlushes(t *testing.T) {
	exp := &collectExporter{}
	bp := NewBatchProcessor(BatchConfig{MaxBatchSize: 100, SendDelay: time.Hour}, exp)

	ctx := context.Background()
	bp.ProcessTraces(ctx, makeTDWithSpans(3))

	if exp.totalSpans() != 0 {
		t.Error("should not flush before shutdown")
	}

	bp.Shutdown(ctx)

	if exp.totalSpans() != 3 {
		t.Errorf("shutdown should flush remaining, got %d", exp.totalSpans())
	}
}

func TestBatchProcessorMultipleBatches(t *testing.T) {
	exp := &collectExporter{}
	bp := NewBatchProcessor(BatchConfig{MaxBatchSize: 3, SendDelay: time.Hour}, exp)

	ctx := context.Background()
	// Send 7 spans — should produce 2 full batches + 1 partial
	bp.ProcessTraces(ctx, makeTDWithSpans(7))

	// 2 batches of 3 flushed, 1 remaining
	if exp.totalSpans() != 6 {
		t.Errorf("expected 6 flushed, got %d", exp.totalSpans())
	}

	bp.Shutdown(ctx)

	if exp.totalSpans() != 7 {
		t.Errorf("after shutdown, expected 7 total, got %d", exp.totalSpans())
	}
}

func TestBatchProcessorDefaults(t *testing.T) {
	exp := &collectExporter{}
	bp := NewBatchProcessor(BatchConfig{}, exp)
	if bp.config.MaxBatchSize != 512 {
		t.Errorf("default MaxBatchSize = %d", bp.config.MaxBatchSize)
	}
	if bp.config.SendDelay != 200*time.Millisecond {
		t.Errorf("default SendDelay = %v", bp.config.SendDelay)
	}
}

func TestBatchProcessorFlushEmpty(t *testing.T) {
	exp := &collectExporter{}
	bp := NewBatchProcessor(BatchConfig{MaxBatchSize: 10}, exp)
	bp.Flush(context.Background()) // should not panic
	if exp.totalSpans() != 0 {
		t.Error("flushing empty should produce nothing")
	}
}
