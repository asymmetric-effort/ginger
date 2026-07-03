package zipkin

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

type mockSink struct {
	traces []otlp.TracesData
	err    error
}

func (s *mockSink) ConsumeTraces(_ context.Context, td otlp.TracesData) error {
	if s.err != nil {
		return s.err
	}
	s.traces = append(s.traces, td)
	return nil
}

func TestZipkinHTTPReceiverSuccess(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	body := `[{"traceId":"0102030405060708090a0b0c0d0e0f10","id":"0102030405060708","name":"test","timestamp":1000000,"duration":500000}]`

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v2/spans", bytes.NewReader([]byte(body)))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(sink.traces) != 1 {
		t.Errorf("traces = %d", len(sink.traces))
	}
	if recv.SpansReceived() != 1 {
		t.Errorf("received = %d", recv.SpansReceived())
	}
}

func TestZipkinHTTPReceiverMethodNotAllowed(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v2/spans", nil)
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", w.Code)
	}
}

func TestZipkinHTTPReceiverBadJSON(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v2/spans", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestZipkinHTTPReceiverSinkError(t *testing.T) {
	sink := &mockSink{err: fmt.Errorf("full")}
	recv := NewHTTPReceiver(sink, 0)

	body := `[{"traceId":"aa","id":"bb","name":"test"}]`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v2/spans", bytes.NewReader([]byte(body)))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d", w.Code)
	}
}

func TestZipkinHTTPReceiverBadGzip(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v2/spans", bytes.NewReader([]byte("not gzip")))
	r.Header.Set("Content-Encoding", "gzip")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}
