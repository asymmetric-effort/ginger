package tracer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestExporterSendsSpans(t *testing.T) {
	var mu sync.Mutex
	var received []byte
	var requestCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requestCount++
		if r.URL.Path != "/v1/traces" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected content-type: %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		received = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(srv.URL, "test-service", 1)
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "test-span")
	span.SetAttribute("key", "value")
	span.SetStatus(StatusOk, "")
	span.End()

	// batchSize=1 so Export should trigger immediate flush
	err := exp.Export(span)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requestCount != 1 {
		t.Fatalf("expected 1 request, got %d", requestCount)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	rs, ok := payload["resourceSpans"].([]interface{})
	if !ok || len(rs) == 0 {
		t.Fatal("missing resourceSpans")
	}

	rsMap := rs[0].(map[string]interface{})
	resource := rsMap["resource"].(map[string]interface{})
	attrs := resource["attributes"].([]interface{})
	if len(attrs) == 0 {
		t.Fatal("missing resource attributes")
	}
	firstAttr := attrs[0].(map[string]interface{})
	if firstAttr["key"] != "service.name" {
		t.Errorf("expected service.name key, got %v", firstAttr["key"])
	}

	ss := rsMap["scopeSpans"].([]interface{})
	scopeSpan := ss[0].(map[string]interface{})
	spans := scopeSpan["spans"].([]interface{})
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	spanMap := spans[0].(map[string]interface{})
	if spanMap["name"] != "test-span" {
		t.Errorf("span name = %v", spanMap["name"])
	}
}

func TestExporterBatching(t *testing.T) {
	var mu sync.Mutex
	var requestCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(srv.URL, "test-service", 3)
	tr := New(Config{Service: "test"})

	// Add 2 spans — should not flush yet (batchSize=3)
	for i := 0; i < 2; i++ {
		_, span := tr.Start(context.Background(), "op")
		span.End()
		if err := exp.Export(span); err != nil {
			t.Fatal(err)
		}
	}

	mu.Lock()
	count := requestCount
	mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 requests before batch full, got %d", count)
	}

	// Third span triggers flush
	_, span := tr.Start(context.Background(), "op")
	span.End()
	if err := exp.Export(span); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	count = requestCount
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 request after batch full, got %d", count)
	}
}

func TestExporterShutdownFlushesRemaining(t *testing.T) {
	var mu sync.Mutex
	var received []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		received = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(srv.URL, "test-service", 100)
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "shutdown-span")
	span.End()

	// Export but don't reach batch size
	if err := exp.Export(span); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	if received != nil {
		mu.Unlock()
		t.Fatal("should not have flushed yet")
	}
	mu.Unlock()

	// Shutdown forces flush
	if err := exp.Shutdown(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("shutdown should have flushed spans")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatal(err)
	}
	rs := payload["resourceSpans"].([]interface{})
	rsMap := rs[0].(map[string]interface{})
	ss := rsMap["scopeSpans"].([]interface{})
	scopeSpan := ss[0].(map[string]interface{})
	spans := scopeSpan["spans"].([]interface{})
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestExporterFlushEmpty(t *testing.T) {
	exp := NewOTLPHTTPExporter("http://localhost:0", "test", 10)
	// Flushing with no spans should not error
	if err := exp.Flush(); err != nil {
		t.Fatalf("flush empty: %v", err)
	}
}

func TestExporterHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(srv.URL, "test", 1)
	tr := New(Config{Service: "test"})
	_, span := tr.Start(context.Background(), "op")
	span.End()

	err := exp.Export(span)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
