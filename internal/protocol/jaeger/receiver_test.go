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

// ---------------------------------------------------------------------------
// skipThriftField coverage — exercise every type branch
// ---------------------------------------------------------------------------

func encodeThriftBatchWithSkipFields() []byte {
	enc := thrift.NewBinaryEncoder()

	// Field 1: Process struct (field 1=serviceName, plus unknown field 99 to skip)
	enc.WriteFieldBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeString, 1)
	enc.WriteString("svc")
	// Unknown field in process: skip a bool (TypeBool)
	enc.WriteFieldBegin(thrift.TypeBool, 99)
	enc.WriteBool(true)
	enc.WriteFieldStop()

	// Field 2: Spans list with one span that has unknown fields of every type
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1) // 1 span
	// Known span fields
	enc.WriteFieldBegin(thrift.TypeI64, 1) // traceIdLow
	enc.WriteI64(100)
	enc.WriteFieldBegin(thrift.TypeI64, 2) // traceIdHigh
	enc.WriteI64(200)
	enc.WriteFieldBegin(thrift.TypeI64, 3) // spanId
	enc.WriteI64(300)
	enc.WriteFieldBegin(thrift.TypeString, 5) // operationName
	enc.WriteString("op")
	enc.WriteFieldBegin(thrift.TypeI64, 7) // startTime
	enc.WriteI64(1000)
	enc.WriteFieldBegin(thrift.TypeI64, 8) // duration
	enc.WriteI64(500)
	// Unknown fields — exercise skip for each supported type
	enc.WriteFieldBegin(thrift.TypeByte, 20)
	enc.WriteI8(0x42)
	enc.WriteFieldBegin(thrift.TypeI16, 21)
	enc.WriteI16(1234)
	enc.WriteFieldBegin(thrift.TypeI32, 22)
	enc.WriteI32(9999)
	enc.WriteFieldBegin(thrift.TypeDouble, 23)
	enc.WriteDouble(3.14)
	enc.WriteFieldBegin(thrift.TypeString, 24)
	enc.WriteString("extra-string")
	// List field to skip
	enc.WriteFieldBegin(thrift.TypeList, 25)
	enc.WriteListBegin(thrift.TypeI32, 2)
	enc.WriteI32(1)
	enc.WriteI32(2)
	// Set field to skip
	enc.WriteFieldBegin(thrift.TypeSet, 26)
	enc.WriteListBegin(thrift.TypeI64, 1)
	enc.WriteI64(999)
	// Map field to skip
	enc.WriteFieldBegin(thrift.TypeMap, 27)
	enc.WriteMapBegin(thrift.TypeString, thrift.TypeI32, 1)
	enc.WriteString("key")
	enc.WriteI32(42)
	// Struct field to skip (nested struct)
	enc.WriteFieldBegin(thrift.TypeStruct, 28)
	enc.WriteFieldBegin(thrift.TypeString, 1)
	enc.WriteString("nested-str")
	enc.WriteFieldStop()
	// End span struct
	enc.WriteFieldStop()

	// End batch
	enc.WriteFieldStop()

	return enc.Bytes()
}

func TestDecodeThriftBatchWithSkipFields(t *testing.T) {
	data := encodeThriftBatchWithSkipFields()
	batch, err := decodeThriftBatch(data)
	if err != nil {
		t.Fatalf("decodeThriftBatch error: %v", err)
	}
	if batch.Process.ServiceName != "svc" {
		t.Errorf("serviceName = %q", batch.Process.ServiceName)
	}
	if len(batch.Spans) != 1 {
		t.Fatalf("spans = %d", len(batch.Spans))
	}
	if batch.Spans[0].OperationName != "op" {
		t.Errorf("operationName = %q", batch.Spans[0].OperationName)
	}
}

func TestDecodeThriftBatchUnknownTopField(t *testing.T) {
	// Encode a batch with an unknown top-level field (e.g., field 99 TypeI32)
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeI32, 99)
	enc.WriteI32(12345)
	enc.WriteFieldStop()
	_, err := decodeThriftBatch(enc.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeThriftProcessUnknownField(t *testing.T) {
	// Simulate a batch where process has an unknown field (field 99 TypeString)
	enc := thrift.NewBinaryEncoder()
	// field 1 = process
	enc.WriteFieldBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeString, 1)
	enc.WriteString("my-service")
	enc.WriteFieldBegin(thrift.TypeString, 99) // unknown process field
	enc.WriteString("ignored")
	enc.WriteFieldStop()
	enc.WriteFieldStop()

	batch, err := decodeThriftBatch(enc.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batch.Process.ServiceName != "my-service" {
		t.Errorf("serviceName = %q", batch.Process.ServiceName)
	}
}

func TestDecodeThriftSpanAllKnownFields(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeI64, 1)
	enc.WriteI64(0xDEADBEEF)
	enc.WriteFieldBegin(thrift.TypeI64, 2)
	enc.WriteI64(0xCAFEBABE)
	enc.WriteFieldBegin(thrift.TypeI64, 3)
	enc.WriteI64(0x1234567890ABCDEF)
	enc.WriteFieldBegin(thrift.TypeString, 5)
	enc.WriteString("my-op")
	enc.WriteFieldBegin(thrift.TypeI64, 7)
	enc.WriteI64(1700000000000000)
	enc.WriteFieldBegin(thrift.TypeI64, 8)
	enc.WriteI64(50000)
	enc.WriteFieldStop()
	enc.WriteFieldStop()

	batch, err := decodeThriftBatch(enc.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batch.Spans) != 1 {
		t.Fatalf("spans = %d", len(batch.Spans))
	}
	if batch.Spans[0].OperationName != "my-op" {
		t.Errorf("operationName = %q", batch.Spans[0].OperationName)
	}
	if batch.Spans[0].StartTime != 1700000000000000 {
		t.Errorf("startTime = %d", batch.Spans[0].StartTime)
	}
	if batch.Spans[0].Duration != 50000 {
		t.Errorf("duration = %d", batch.Spans[0].Duration)
	}
}

func TestSkipThriftFieldAllTypes(t *testing.T) {
	// Directly exercise skipThriftField for all type branches
	enc := thrift.NewBinaryEncoder()
	// TypeBool
	enc.WriteBool(true)
	// TypeByte
	enc.WriteI8(42)
	// TypeI16
	enc.WriteI16(1000)
	// TypeI32
	enc.WriteI32(100000)
	// TypeI64
	enc.WriteI64(9999999999)
	// TypeDouble
	enc.WriteDouble(2.718)
	// TypeString
	enc.WriteString("hello")
	// TypeList of I32 (2 elements)
	enc.WriteListBegin(thrift.TypeI32, 2)
	enc.WriteI32(1)
	enc.WriteI32(2)
	// TypeSet of I64 (1 element)
	enc.WriteListBegin(thrift.TypeI64, 1)
	enc.WriteI64(99)
	// TypeMap of String->I32 (1 pair)
	enc.WriteMapBegin(thrift.TypeString, thrift.TypeI32, 1)
	enc.WriteString("k")
	enc.WriteI32(7)
	// TypeStruct: one string field then stop
	enc.WriteFieldBegin(thrift.TypeString, 1)
	enc.WriteString("nested")
	enc.WriteFieldStop()

	dec := thrift.NewBinaryDecoder(enc.Bytes())
	skipThriftField(dec, thrift.TypeBool)
	skipThriftField(dec, thrift.TypeByte)
	skipThriftField(dec, thrift.TypeI16)
	skipThriftField(dec, thrift.TypeI32)
	skipThriftField(dec, thrift.TypeI64)
	skipThriftField(dec, thrift.TypeDouble)
	skipThriftField(dec, thrift.TypeString)
	skipThriftField(dec, thrift.TypeList)
	skipThriftField(dec, thrift.TypeSet)
	skipThriftField(dec, thrift.TypeMap)
	skipThriftField(dec, thrift.TypeStruct)

	if !dec.Done() {
		t.Errorf("decoder not done after all skips; remaining = %d", dec.Remaining())
	}
}

// ---------------------------------------------------------------------------
// Error path coverage for decodeThrift* functions
// ---------------------------------------------------------------------------

func TestDecodeThriftBatchReadFieldBeginError(t *testing.T) {
	// Empty data — ReadFieldBegin will fail
	_, err := decodeThriftBatch([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestDecodeThriftBatchProcess_ReadFieldBeginError(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	// field 1 = process struct, but with truncated inner content
	enc.WriteFieldBegin(thrift.TypeStruct, 1)
	// Don't write field stop — truncated
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated process struct")
	}
}

func TestDecodeThriftBatchProcess_ReadStringError(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeStruct, 1)
	// process field 1 = serviceName, but truncated string
	enc.WriteFieldBegin(thrift.TypeString, 1)
	// Write length prefix claiming 100 bytes, but no data
	enc.WriteI32(100)
	// Don't write the actual string bytes
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated serviceName string")
	}
}

func TestDecodeThriftBatchSpanList_ReadListBeginError(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	// field 2 = spans, but truncated list header
	enc.WriteFieldBegin(thrift.TypeList, 2)
	// Don't write full list header — truncated
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated span list")
	}
}

func TestDecodeThriftSpanTraceIdLow_ReadI64Error(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	// span field 1 = traceIdLow, but truncated I64
	enc.WriteFieldBegin(thrift.TypeI64, 1)
	// Write only 4 bytes instead of 8
	enc.WriteI32(12345)
	// Now the I64 read will fail (only 4 bytes available, not 8)
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated traceIdLow")
	}
}

func TestDecodeThriftSpanTraceIdHigh_ReadI64Error(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeI64, 1) // traceIdLow ok
	enc.WriteI64(1)
	enc.WriteFieldBegin(thrift.TypeI64, 2) // traceIdHigh truncated
	enc.WriteI32(999)                       // only 4 bytes
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated traceIdHigh")
	}
}

func TestDecodeThriftSpanSpanId_ReadI64Error(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeI64, 1)
	enc.WriteI64(1)
	enc.WriteFieldBegin(thrift.TypeI64, 2)
	enc.WriteI64(2)
	enc.WriteFieldBegin(thrift.TypeI64, 3) // spanId truncated
	enc.WriteI32(333)
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated spanId")
	}
}

func TestDecodeThriftSpanStartTime_ReadI64Error(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeI64, 1)
	enc.WriteI64(1)
	enc.WriteFieldBegin(thrift.TypeI64, 2)
	enc.WriteI64(2)
	enc.WriteFieldBegin(thrift.TypeI64, 3)
	enc.WriteI64(3)
	enc.WriteFieldBegin(thrift.TypeString, 5)
	enc.WriteString("op")
	enc.WriteFieldBegin(thrift.TypeI64, 7) // startTime truncated
	enc.WriteI32(111)
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated startTime")
	}
}

func TestDecodeThriftSpanDuration_ReadI64Error(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeI64, 1)
	enc.WriteI64(1)
	enc.WriteFieldBegin(thrift.TypeI64, 2)
	enc.WriteI64(2)
	enc.WriteFieldBegin(thrift.TypeI64, 3)
	enc.WriteI64(3)
	enc.WriteFieldBegin(thrift.TypeString, 5)
	enc.WriteString("op")
	enc.WriteFieldBegin(thrift.TypeI64, 7)
	enc.WriteI64(1000)
	enc.WriteFieldBegin(thrift.TypeI64, 8) // duration truncated
	enc.WriteI32(500)
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated duration")
	}
}

func TestDecodeThriftSpanOperationName_ReadStringError(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeI64, 1)
	enc.WriteI64(1)
	enc.WriteFieldBegin(thrift.TypeI64, 2)
	enc.WriteI64(2)
	enc.WriteFieldBegin(thrift.TypeI64, 3)
	enc.WriteI64(3)
	enc.WriteFieldBegin(thrift.TypeString, 5) // operationName truncated
	enc.WriteI32(100)                          // claim 100 bytes but don't write them
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated operationName")
	}
}

func TestDecodeThriftSpanReadFieldBeginError(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	// Start a span but give it no fields at all (empty struct body)
	// ReadFieldBegin on empty will fail
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated span struct")
	}
}

func TestDecodeThriftProcessReadFieldBeginError(t *testing.T) {
	enc := thrift.NewBinaryEncoder()
	enc.WriteFieldBegin(thrift.TypeStruct, 1)
	// Truncated — no content in process
	_, err := decodeThriftBatch(enc.Bytes())
	if err == nil {
		t.Error("expected error for truncated process")
	}
}

func TestThriftHTTPReceiverMaxBodyExceeded(t *testing.T) {
	recv := NewThriftHTTPReceiver(&mockSink{}, 10) // max 10 bytes
	body := encodeThriftBatch()                    // much larger than 10 bytes
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/traces", bytes.NewReader(body))
	recv.ServeHTTP(w, r)
	// Should fail with 400 because body read is limited
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
