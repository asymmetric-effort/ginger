package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

func makeTestTD(traceID [16]byte, service, operation string) otlp.TracesData {
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue(service))
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: traceID, SpanID: [8]byte{1}, Name: operation,
					Kind:              otlp.SpanKindServer,
					StartTimeUnixNano: uint64(time.Now().UnixNano()),
					EndTimeUnixNano:   uint64(time.Now().Add(100 * time.Millisecond).UnixNano()),
				}},
			}},
		}},
	}
}

func TestHarnessBasic(t *testing.T) {
	h := NewHarness(t)

	tid := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	td := makeTestTD(tid, "test-svc", "GET /api")

	if err := h.SendTrace(td); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	got, err := h.QueryTrace(tid)
	if err != nil {
		t.Fatalf("QueryTrace: %v", err)
	}
	if len(got.ResourceSpans) == 0 {
		t.Error("expected trace data")
	}
}

func TestHarnessWaitForService(t *testing.T) {
	h := NewHarness(t)

	td := makeTestTD([16]byte{1}, "my-service", "op")
	h.SendTrace(td)
	time.Sleep(100 * time.Millisecond)

	if !h.WaitForService("my-service", 2*time.Second) {
		t.Error("service should appear")
	}

	if h.WaitForService("nonexistent", 200*time.Millisecond) {
		t.Error("should not find nonexistent service")
	}
}

func TestHarnessHTTPEndpoints(t *testing.T) {
	h := NewHarness(t)

	// Health live
	resp, err := http.Get("http://" + h.Addr + "/health/live")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("live: %d", resp.StatusCode)
	}

	// Health ready
	resp, err = http.Get("http://" + h.Addr + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("ready: %d", resp.StatusCode)
	}

	// Services endpoint
	tid := [16]byte{2}
	h.SendTrace(makeTestTD(tid, "api-svc", "op"))
	time.Sleep(100 * time.Millisecond)

	resp, err = http.Get("http://" + h.Addr + "/api/v3/services")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var services []string
	json.Unmarshal(body, &services)
	if len(services) == 0 {
		t.Error("should have services")
	}
}

func TestHarnessMultipleTraces(t *testing.T) {
	h := NewHarness(t)

	for i := byte(1); i <= 5; i++ {
		h.SendTrace(makeTestTD([16]byte{i}, "svc", "op"))
	}
	time.Sleep(200 * time.Millisecond)

	traces, err := h.QuerySvc.FindTraces(context.Background(), storage.TraceQueryParameters{NumTraces: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 5 {
		t.Errorf("expected 5, got %d", len(traces))
	}
}
