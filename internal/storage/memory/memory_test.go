package memory

import (
	"context"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

func makeSpan(traceID [16]byte, spanID [8]byte, name string, service string, startNs, endNs uint64) otlp.TracesData {
	resAttrs := otlp.NewAttributes()
	if service != "" {
		resAttrs.Set("service.name", otlp.StringValue(service))
	}
	spanAttrs := otlp.NewAttributes()
	spanAttrs.Set("test.key", otlp.StringValue("test.value"))

	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID:           traceID,
					SpanID:            spanID,
					Name:              name,
					Kind:              otlp.SpanKindServer,
					StartTimeUnixNano: startNs,
					EndTimeUnixNano:   endNs,
					Attributes:        spanAttrs,
				}},
			}},
		}},
	}
}

func TestWriteAndGetTrace(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()
	tid := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	td := makeSpan(tid, [8]byte{1}, "op1", "svc1", 1000, 2000)
	if err := b.WriteSpans(ctx, td); err != nil {
		t.Fatal(err)
	}

	got, err := b.GetTrace(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ResourceSpans) != 1 {
		t.Fatalf("ResourceSpans: %d", len(got.ResourceSpans))
	}
	if got.ResourceSpans[0].ScopeSpans[0].Spans[0].Name != "op1" {
		t.Error("span name mismatch")
	}
}

func TestGetTraceNotFound(t *testing.T) {
	b := NewBackend(100)
	_, err := b.GetTrace(context.Background(), [16]byte{99})
	if err != storage.ErrTraceNotFound {
		t.Errorf("expected ErrTraceNotFound, got %v", err)
	}
}

func TestWriteMergesSpans(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()
	tid := [16]byte{1}

	b.WriteSpans(ctx, makeSpan(tid, [8]byte{1}, "op1", "svc1", 1000, 2000))
	b.WriteSpans(ctx, makeSpan(tid, [8]byte{2}, "op2", "svc1", 1500, 2500))

	got, _ := b.GetTrace(ctx, tid)
	spans := got.ResourceSpans[0].ScopeSpans[0].Spans
	if len(spans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(spans))
	}
}

func TestEviction(t *testing.T) {
	b := NewBackend(3)
	ctx := context.Background()

	for i := byte(1); i <= 4; i++ {
		b.WriteSpans(ctx, makeSpan([16]byte{i}, [8]byte{i}, "op", "svc", 1000, 2000))
	}

	// First trace should be evicted
	_, err := b.GetTrace(ctx, [16]byte{1})
	if err != storage.ErrTraceNotFound {
		t.Error("oldest trace should be evicted")
	}
	// Fourth should exist
	_, err = b.GetTrace(ctx, [16]byte{4})
	if err != nil {
		t.Error("newest trace should exist")
	}
}

func TestFindTracesByService(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op1", "svcA", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{2}, [8]byte{2}, "op2", "svcB", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{3}, [8]byte{3}, "op3", "svcA", 1000, 2000))

	results, err := b.FindTraces(ctx, storage.TraceQueryParameters{ServiceName: "svcA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 traces, got %d", len(results))
	}
}

func TestFindTracesByOperation(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "GET /api", "svc", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{2}, [8]byte{2}, "POST /api", "svc", 1000, 2000))

	results, err := b.FindTraces(ctx, storage.TraceQueryParameters{OperationName: "GET /api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestFindTracesByTags(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op", "svc", 1000, 2000))

	results, err := b.FindTraces(ctx, storage.TraceQueryParameters{
		Tags: map[string]string{"test.key": "test.value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}

	results, err = b.FindTraces(ctx, storage.TraceQueryParameters{
		Tags: map[string]string{"test.key": "wrong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected 0 for wrong tag value")
	}
}

func TestFindTracesByTimeRange(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	now := time.Now()
	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op", "svc",
		uint64(now.UnixNano()), uint64(now.Add(time.Second).UnixNano())))

	results, _ := b.FindTraces(ctx, storage.TraceQueryParameters{
		StartTimeMin: now.Add(-time.Minute),
		StartTimeMax: now.Add(time.Minute),
	})
	if len(results) != 1 {
		t.Errorf("expected 1 in range, got %d", len(results))
	}

	results, _ = b.FindTraces(ctx, storage.TraceQueryParameters{
		StartTimeMin: now.Add(time.Hour),
	})
	if len(results) != 0 {
		t.Error("expected 0 out of range")
	}
}

func TestFindTracesByDuration(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	// 1 second duration
	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op", "svc", 1000000000, 2000000000))

	results, _ := b.FindTraces(ctx, storage.TraceQueryParameters{
		DurationMin: 500 * time.Millisecond,
		DurationMax: 2 * time.Second,
	})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}

	results, _ = b.FindTraces(ctx, storage.TraceQueryParameters{
		DurationMin: 5 * time.Second,
	})
	if len(results) != 0 {
		t.Error("expected 0 for min > actual")
	}
}

func TestFindTracesLimit(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	for i := byte(1); i <= 10; i++ {
		b.WriteSpans(ctx, makeSpan([16]byte{i}, [8]byte{i}, "op", "svc", 1000, 2000))
	}

	results, _ := b.FindTraces(ctx, storage.TraceQueryParameters{NumTraces: 3})
	if len(results) != 3 {
		t.Errorf("expected 3, got %d", len(results))
	}
}

func TestFindTraceIDs(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op", "svc", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{2}, [8]byte{2}, "op", "svc", 1000, 2000))

	ids, err := b.FindTraceIDs(ctx, storage.TraceQueryParameters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2, got %d", len(ids))
	}
}

func TestGetServices(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op", "svcA", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{2}, [8]byte{2}, "op", "svcB", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{3}, [8]byte{3}, "op", "svcA", 1000, 2000))

	services, err := b.GetServices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 2 {
		t.Errorf("expected 2, got %d", len(services))
	}
}

func TestGetServicesEmpty(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op", "", 1000, 2000))

	services, _ := b.GetServices(ctx)
	if len(services) != 0 {
		t.Errorf("expected 0 for empty service, got %d", len(services))
	}
}

func TestGetOperations(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "GET /api", "svc", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{2}, [8]byte{2}, "POST /api", "svc", 1000, 2000))
	b.WriteSpans(ctx, makeSpan([16]byte{3}, [8]byte{3}, "GET /other", "other", 1000, 2000))

	ops, err := b.GetOperations(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Errorf("expected 2, got %d", len(ops))
	}
}

func TestDependencies(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()
	now := time.Now()

	deps := []storage.DependencyLink{
		{Parent: "svcA", Child: "svcB", CallCount: 10},
		{Parent: "svcB", Child: "svcC", CallCount: 5},
	}
	b.WriteLinks(ctx, now, deps)

	got, err := b.GetDependencies(ctx, now.Add(time.Minute), 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestDependenciesMerge(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()
	now := time.Now()

	b.WriteLinks(ctx, now, []storage.DependencyLink{{Parent: "a", Child: "b", CallCount: 5}})
	b.WriteLinks(ctx, now, []storage.DependencyLink{{Parent: "a", Child: "b", CallCount: 3}})

	got, _ := b.GetDependencies(ctx, now.Add(time.Minute), 2*time.Minute)
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].CallCount != 8 {
		t.Errorf("call count = %d, want 8", got[0].CallCount)
	}
}

func TestDependenciesOutOfRange(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteLinks(ctx, time.Now().Add(-time.Hour), []storage.DependencyLink{{Parent: "a", Child: "b", CallCount: 1}})

	got, _ := b.GetDependencies(ctx, time.Now(), time.Minute) // lookback 1 min
	if len(got) != 0 {
		t.Error("should not find deps outside range")
	}
}

func TestSamplingStore(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	err := b.InsertThroughput(ctx, []storage.Throughput{{Service: "svc", Operation: "op", Count: 100}})
	if err != nil {
		t.Fatal(err)
	}

	throughput, err := b.GetThroughput(ctx, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(throughput) != 1 {
		t.Errorf("expected 1, got %d", len(throughput))
	}

	probs := storage.ServiceOperationProbabilities{
		"svc": {"op": 0.5},
	}
	err = b.InsertProbabilities(ctx, "host1", probs)
	if err != nil {
		t.Fatal(err)
	}

	got, err := b.GetLatestProbabilities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got["svc"]["op"] != 0.5 {
		t.Error("probabilities mismatch")
	}
}

func TestClear(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()

	b.WriteSpans(ctx, makeSpan([16]byte{1}, [8]byte{1}, "op", "svc", 1000, 2000))
	b.Clear()

	_, err := b.GetTrace(ctx, [16]byte{1})
	if err != storage.ErrTraceNotFound {
		t.Error("Clear should remove all traces")
	}
}

func TestBackendInterfaces(t *testing.T) {
	b := NewBackend(100)
	// Verify all interface methods are accessible
	_ = b.TraceReader()
	_ = b.TraceWriter()
	_ = b.DependencyReader()
	_ = b.DependencyWriter()
	_ = b.SamplingStore()
	b.Close()
}

func TestDefaultMaxTraces(t *testing.T) {
	b := NewBackend(0)
	if b.maxTraces != defaultMaxTraces {
		t.Errorf("default = %d, want %d", b.maxTraces, defaultMaxTraces)
	}
}

func TestFindTracesDefaultLimit(t *testing.T) {
	b := NewBackend(100)
	ctx := context.Background()
	for i := byte(1); i <= 25; i++ {
		b.WriteSpans(ctx, makeSpan([16]byte{i}, [8]byte{i}, "op", "svc", 1000, 2000))
	}
	results, _ := b.FindTraces(ctx, storage.TraceQueryParameters{})
	if len(results) != 20 {
		t.Errorf("default limit: got %d, want 20", len(results))
	}
}
