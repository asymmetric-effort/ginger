package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

type mockReceiver struct {
	sink TracesConsumer
}

func (r *mockReceiver) Start(_ context.Context, sink TracesConsumer) error {
	r.sink = sink
	return nil
}

func (r *mockReceiver) Shutdown(_ context.Context) error {
	return nil
}

type mockProcessor struct {
	fn func(otlp.TracesData) (otlp.TracesData, error)
}

func (p *mockProcessor) ProcessTraces(_ context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	if p.fn != nil {
		return p.fn(td)
	}
	return td, nil
}

type mockExporter struct {
	mu     sync.Mutex
	traces []otlp.TracesData
	closed bool
}

func (e *mockExporter) ExportTraces(_ context.Context, td otlp.TracesData) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.traces = append(e.traces, td)
	return nil
}

func (e *mockExporter) Shutdown(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	return nil
}

func (e *mockExporter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.traces)
}

func makeTestTD() otlp.TracesData {
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: "test", StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
}

func TestPipelineBasic(t *testing.T) {
	recv := &mockReceiver{}
	exp := &mockExporter{}

	p := New(Config{QueueSize: 10})
	p.AddReceiver(recv)
	p.AddExporter(exp)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Submit data through the receiver
	recv.sink.ConsumeTraces(ctx, makeTestTD())
	recv.sink.ConsumeTraces(ctx, makeTestTD())

	time.Sleep(100 * time.Millisecond)

	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	if exp.count() < 2 {
		t.Errorf("expected at least 2 exports, got %d", exp.count())
	}
	if !exp.closed {
		t.Error("exporter should be shut down")
	}
}

func TestPipelineWithProcessor(t *testing.T) {
	recv := &mockReceiver{}
	exp := &mockExporter{}
	var processed atomic.Int32

	proc := &mockProcessor{fn: func(td otlp.TracesData) (otlp.TracesData, error) {
		processed.Add(1)
		return td, nil
	}}

	p := New(Config{QueueSize: 10})
	p.AddReceiver(recv)
	p.AddProcessor(proc)
	p.AddExporter(exp)

	ctx := context.Background()
	p.Start(ctx)
	recv.sink.ConsumeTraces(ctx, makeTestTD())

	time.Sleep(100 * time.Millisecond)
	p.Shutdown(context.Background())

	if processed.Load() < 1 {
		t.Error("processor should have been called")
	}
}

func TestPipelineProcessorError(t *testing.T) {
	recv := &mockReceiver{}
	exp := &mockExporter{}

	proc := &mockProcessor{fn: func(td otlp.TracesData) (otlp.TracesData, error) {
		return td, context.Canceled // error drops the batch
	}}

	p := New(Config{QueueSize: 10})
	p.AddReceiver(recv)
	p.AddProcessor(proc)
	p.AddExporter(exp)

	ctx := context.Background()
	p.Start(ctx)
	recv.sink.ConsumeTraces(ctx, makeTestTD())

	time.Sleep(100 * time.Millisecond)
	p.Shutdown(context.Background())

	if exp.count() != 0 {
		t.Error("processor error should drop the batch")
	}
}

func TestPipelineQueueFull(t *testing.T) {
	p := New(Config{QueueSize: 1})
	// Don't start — no consumer, queue fills immediately
	p.ConsumeTraces(context.Background(), makeTestTD())
	err := p.ConsumeTraces(context.Background(), makeTestTD())
	if err == nil {
		t.Error("should error when queue is full")
	}
}

func TestPipelineMultipleExporters(t *testing.T) {
	recv := &mockReceiver{}
	exp1 := &mockExporter{}
	exp2 := &mockExporter{}

	p := New(Config{QueueSize: 10})
	p.AddReceiver(recv)
	p.AddExporter(exp1)
	p.AddExporter(exp2)

	ctx := context.Background()
	p.Start(ctx)
	recv.sink.ConsumeTraces(ctx, makeTestTD())

	time.Sleep(100 * time.Millisecond)
	p.Shutdown(context.Background())

	if exp1.count() < 1 || exp2.count() < 1 {
		t.Errorf("both exporters should receive data: exp1=%d exp2=%d", exp1.count(), exp2.count())
	}
}

func TestPipelineQueueLen(t *testing.T) {
	p := New(Config{QueueSize: 10})
	if p.QueueLen() != 0 {
		t.Error("initial queue should be empty")
	}
	p.ConsumeTraces(context.Background(), makeTestTD())
	if p.QueueLen() != 1 {
		t.Errorf("queue len = %d, want 1", p.QueueLen())
	}
}

func TestPipelineDefaultConfig(t *testing.T) {
	p := New(Config{})
	if p.config.QueueSize != 1024 {
		t.Errorf("default queue size = %d", p.config.QueueSize)
	}
	if p.config.DrainTimeout != 5*time.Second {
		t.Errorf("default drain timeout = %v", p.config.DrainTimeout)
	}
}

func TestPipelineDrain(t *testing.T) {
	exp := &mockExporter{}
	p := New(Config{QueueSize: 100})
	p.AddExporter(exp)

	ctx := context.Background()
	p.Start(ctx)

	// Fill queue
	for i := 0; i < 10; i++ {
		p.ConsumeTraces(ctx, makeTestTD())
	}

	time.Sleep(200 * time.Millisecond)
	p.Shutdown(context.Background())

	if exp.count() < 10 {
		t.Errorf("drain should process all items, got %d", exp.count())
	}
}
