package integration

import (
	"context"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/storage"
)

func TestQueryGetTrace(t *testing.T) {
	h := NewHarness(t)
	tid := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	h.SendTrace(makeTestTD(tid, "query-svc", "GET /api"))
	time.Sleep(100 * time.Millisecond)

	got, err := h.QuerySvc.GetTrace(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ResourceSpans) == 0 {
		t.Error("should find trace")
	}
}

func TestQueryFindTracesByService(t *testing.T) {
	h := NewHarness(t)
	for i := byte(1); i <= 3; i++ {
		h.SendTrace(makeTestTD([16]byte{i}, "find-svc", "op"))
	}
	h.SendTrace(makeTestTD([16]byte{99}, "other-svc", "op"))
	time.Sleep(200 * time.Millisecond)

	traces, err := h.QuerySvc.FindTraces(context.Background(), storage.TraceQueryParameters{
		ServiceName: "find-svc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 3 {
		t.Errorf("expected 3, got %d", len(traces))
	}
}

func TestQueryGetServices(t *testing.T) {
	h := NewHarness(t)
	h.SendTrace(makeTestTD([16]byte{1}, "svc-a", "op"))
	h.SendTrace(makeTestTD([16]byte{2}, "svc-b", "op"))
	time.Sleep(200 * time.Millisecond)

	services, err := h.QuerySvc.GetServices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 2 {
		t.Errorf("expected 2 services, got %d", len(services))
	}
}

func TestQueryGetOperations(t *testing.T) {
	h := NewHarness(t)
	h.SendTrace(makeTestTD([16]byte{1}, "ops-svc", "GET /api"))
	h.SendTrace(makeTestTD([16]byte{2}, "ops-svc", "POST /api"))
	time.Sleep(200 * time.Millisecond)

	ops, err := h.QuerySvc.GetOperations(context.Background(), "ops-svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Errorf("expected 2 ops, got %d", len(ops))
	}
}

func TestQueryGetDependencies(t *testing.T) {
	h := NewHarness(t)
	ctx := context.Background()
	h.Backend.WriteLinks(ctx, time.Now(), []storage.DependencyLink{
		{Parent: "frontend", Child: "backend", CallCount: 100},
	})

	deps, err := h.QuerySvc.GetDependencies(ctx, time.Now().Add(time.Minute), 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Errorf("expected 1 dep, got %d", len(deps))
	}
}
