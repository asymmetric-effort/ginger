package tenancy

import (
	"context"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
	"github.com/asymmetric-effort/ginger/internal/storage/memory"
)

func TestTenantedWriterInjectsTenant(t *testing.T) {
	backend := memory.NewBackend(100)
	tw := NewTenantedWriter(backend)

	ctx := ContextWithTenant(context.Background(), "tenant-a")
	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000}},
			}},
		}},
	}

	if err := tw.WriteSpans(ctx, td); err != nil {
		t.Fatal(err)
	}

	got, _ := backend.GetTrace(context.Background(), [16]byte{1})
	v, ok := got.ResourceSpans[0].Resource.Attributes.Get("ginger.tenant")
	if !ok || v.Str != "tenant-a" {
		t.Error("tenant not injected")
	}
}

func TestTenantedWriterNoTenant(t *testing.T) {
	backend := memory.NewBackend(100)
	tw := NewTenantedWriter(backend)

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000}},
			}},
		}},
	}

	if err := tw.WriteSpans(context.Background(), td); err != nil {
		t.Fatal(err)
	}
}

func TestTenantedReaderFilters(t *testing.T) {
	backend := memory.NewBackend(100)
	tw := NewTenantedWriter(backend)
	tr := NewTenantedReader(backend)

	// Write trace for tenant-a
	ctxA := ContextWithTenant(context.Background(), "tenant-a")
	tdA := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000}},
			}},
		}},
	}
	tw.WriteSpans(ctxA, tdA)

	// Read as tenant-a — should find
	got, err := tr.GetTrace(ctxA, [16]byte{1})
	if err != nil {
		t.Fatalf("tenant-a should find: %v", err)
	}
	if len(got.ResourceSpans) == 0 {
		t.Error("should have data")
	}

	// Read as tenant-b — should NOT find
	ctxB := ContextWithTenant(context.Background(), "tenant-b")
	_, err = tr.GetTrace(ctxB, [16]byte{1})
	if err != storage.ErrTraceNotFound {
		t.Errorf("tenant-b should not find: %v", err)
	}
}

func TestTenantedReaderFindTraces(t *testing.T) {
	backend := memory.NewBackend(100)
	tw := NewTenantedWriter(backend)
	tr := NewTenantedReader(backend)

	ctxA := ContextWithTenant(context.Background(), "tenant-a")
	ctxB := ContextWithTenant(context.Background(), "tenant-b")

	tdA := otlp.TracesData{ResourceSpans: []otlp.ResourceSpans{{
		ScopeSpans: []otlp.ScopeSpans{{
			Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
				StartTimeUnixNano: 1000, EndTimeUnixNano: 2000}},
		}},
	}}}
	tw.WriteSpans(ctxA, tdA)

	results, _ := tr.FindTraces(ctxA, storage.TraceQueryParameters{})
	if len(results) != 1 {
		t.Errorf("tenant-a should find 1, got %d", len(results))
	}

	results, _ = tr.FindTraces(ctxB, storage.TraceQueryParameters{})
	if len(results) != 0 {
		t.Errorf("tenant-b should find 0, got %d", len(results))
	}
}

func TestTenantedReaderNoTenantContext(t *testing.T) {
	backend := memory.NewBackend(100)
	tr := NewTenantedReader(backend)

	backend.WriteSpans(context.Background(), otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000}},
			}},
		}},
	})

	// No tenant in context — should return all
	got, err := tr.GetTrace(context.Background(), [16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ResourceSpans) == 0 {
		t.Error("should return data without tenant context")
	}

	results, _ := tr.FindTraces(context.Background(), storage.TraceQueryParameters{})
	if len(results) == 0 {
		t.Error("FindTraces without tenant should return all")
	}
}

func TestTenantedReaderDelegates(t *testing.T) {
	backend := memory.NewBackend(100)
	tr := NewTenantedReader(backend)

	_, err := tr.FindTraceIDs(context.Background(), storage.TraceQueryParameters{})
	if err != nil {
		t.Error(err)
	}
	_, err = tr.GetServices(context.Background())
	if err != nil {
		t.Error(err)
	}
	_, err = tr.GetOperations(context.Background(), "svc")
	if err != nil {
		t.Error(err)
	}
}

func TestTenantedReaderGetTraceError(t *testing.T) {
	backend := memory.NewBackend(100)
	tr := NewTenantedReader(backend)

	_, err := tr.GetTrace(context.Background(), [16]byte{99})
	if err != storage.ErrTraceNotFound {
		t.Errorf("expected not found, got %v", err)
	}
}
