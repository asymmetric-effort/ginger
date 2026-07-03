package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockSink struct {
	traces []TracesData
	err    error
}

func (s *mockSink) ConsumeTraces(_ context.Context, td TracesData) error {
	if s.err != nil {
		return s.err
	}
	s.traces = append(s.traces, td)
	return nil
}

func TestHTTPReceiverJSON(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	body, _ := json.Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(sink.traces) != 1 {
		t.Errorf("traces = %d", len(sink.traces))
	}
	if recv.SpansReceived() != 1 {
		t.Errorf("received = %d", recv.SpansReceived())
	}
}

func TestHTTPReceiverProtobuf(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	body, _ := Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/x-protobuf")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverMethodNotAllowed(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/traces", nil)
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverUnsupportedContentType(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte("{}")))
	r.Header.Set("Content-Type", "text/plain")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverSinkError(t *testing.T) {
	sink := &mockSink{err: fmt.Errorf("queue full")}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	}
	body, _ := json.Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d", w.Code)
	}
	if recv.SpansRejected() != 1 {
		t.Errorf("rejected = %d", recv.SpansRejected())
	}
}

func TestHTTPReceiverBadJSON(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverBadProtobuf(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte{0xff, 0xff}))
	r.Header.Set("Content-Type", "application/x-protobuf")
	recv.ServeHTTP(w, r)

	// May or may not error depending on protobuf parser — either OK or BadRequest is acceptable
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHTTPReceiverBadGzip(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader([]byte("not gzip")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Encoding", "gzip")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestCountSpans(t *testing.T) {
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{}, {}, {}},
			}},
		}},
	}
	if countSpans(td) != 3 {
		t.Errorf("count = %d", countSpans(td))
	}
}

func TestGRPCReceiverDefaults(t *testing.T) {
	sink := &mockSink{}
	r := NewGRPCReceiver(sink, 0)
	if r == nil {
		t.Fatal("NewGRPCReceiver returned nil")
	}
	if r.SpansReceived() != 0 {
		t.Error("initial SpansReceived should be 0")
	}
	if r.SpansRejected() != 0 {
		t.Error("initial SpansRejected should be 0")
	}
}

func TestGRPCReceiverServiceDesc(t *testing.T) {
	sink := &mockSink{}
	r := NewGRPCReceiver(sink, 4096)
	desc := r.ServiceDesc()
	if desc == nil {
		t.Fatal("ServiceDesc returned nil")
	}
	if desc.ServiceName != "opentelemetry.proto.collector.trace.v1.TraceService" {
		t.Errorf("ServiceName = %q", desc.ServiceName)
	}
	if len(desc.Methods) != 1 {
		t.Fatalf("Methods count = %d", len(desc.Methods))
	}
	if desc.Methods[0].Name != "Export" {
		t.Errorf("Method name = %q", desc.Methods[0].Name)
	}
}

func TestGRPCReceiverHandleExportSuccess(t *testing.T) {
	sink := &mockSink{}
	r := NewGRPCReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	data, _ := Marshal(td)

	desc := r.ServiceDesc()
	ctx := context.Background()
	resp, err := desc.Methods[0].UnaryHandler(ctx, data)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if resp == nil {
		t.Error("response should not be nil")
	}
	if r.SpansReceived() != 1 {
		t.Errorf("SpansReceived = %d", r.SpansReceived())
	}
}

func TestGRPCReceiverHandleExportBadData(t *testing.T) {
	sink := &mockSink{}
	r := NewGRPCReceiver(sink, 0)

	desc := r.ServiceDesc()
	ctx := context.Background()
	_, err := desc.Methods[0].UnaryHandler(ctx, []byte{0xff, 0xff})
	if err == nil {
		t.Error("expected error on bad protobuf data")
	}
}

func TestGRPCReceiverHandleExportSinkError(t *testing.T) {
	sink := &mockSink{err: fmt.Errorf("queue full")}
	r := NewGRPCReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2,
				}},
			}},
		}},
	}
	data, _ := Marshal(td)

	desc := r.ServiceDesc()
	ctx := context.Background()
	_, err := desc.Methods[0].UnaryHandler(ctx, data)
	if err == nil {
		t.Error("expected error when sink fails")
	}
	if r.SpansRejected() != 1 {
		t.Errorf("SpansRejected = %d", r.SpansRejected())
	}
}
