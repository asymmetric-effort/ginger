package jaeger

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/codec/thrift"
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

func encodeThriftBatch() []byte {
	enc := thrift.NewBinaryEncoder()

	// Field 1: Process (struct)
	enc.WriteFieldBegin(thrift.TypeStruct, 1)
	// Process field 1: serviceName
	enc.WriteFieldBegin(thrift.TypeString, 1)
	enc.WriteString("test-service")
	enc.WriteFieldStop() // end of process struct

	// Field 2: Spans (list of struct)
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1) // 1 span

	// Span fields
	enc.WriteFieldBegin(thrift.TypeI64, 1) // traceIdLow
	enc.WriteI64(12345)
	enc.WriteFieldBegin(thrift.TypeI64, 2) // traceIdHigh
	enc.WriteI64(67890)
	enc.WriteFieldBegin(thrift.TypeI64, 3) // spanId
	enc.WriteI64(111)
	enc.WriteFieldBegin(thrift.TypeString, 5) // operationName
	enc.WriteString("GET /api")
	enc.WriteFieldBegin(thrift.TypeI64, 7) // startTime
	enc.WriteI64(1000000)
	enc.WriteFieldBegin(thrift.TypeI64, 8) // duration
	enc.WriteI64(500000)
	enc.WriteFieldStop() // end span

	enc.WriteFieldStop() // end batch

	return enc.Bytes()
}

func TestThriftHTTPReceiverSuccess(t *testing.T) {
	sink := &mockSink{}
	recv := NewThriftHTTPReceiver(sink, 0)

	body := encodeThriftBatch()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/x-thrift")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(sink.traces) != 1 {
		t.Fatalf("traces = %d", len(sink.traces))
	}
	if recv.SpansReceived() != 1 {
		t.Errorf("received = %d", recv.SpansReceived())
	}
}

func TestThriftHTTPReceiverMethodNotAllowed(t *testing.T) {
	recv := NewThriftHTTPReceiver(&mockSink{}, 0)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/traces", nil)
	recv.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", w.Code)
	}
}

func TestThriftHTTPReceiverBadBody(t *testing.T) {
	recv := NewThriftHTTPReceiver(&mockSink{}, 0)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/traces", bytes.NewReader([]byte{0xff}))
	recv.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestThriftHTTPReceiverSinkError(t *testing.T) {
	sink := &mockSink{err: fmt.Errorf("full")}
	recv := NewThriftHTTPReceiver(sink, 0)

	body := encodeThriftBatch()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/traces", bytes.NewReader(body))
	recv.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d", w.Code)
	}
}
