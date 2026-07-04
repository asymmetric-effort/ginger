package jaeger

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
	"github.com/asymmetric-effort/ginger/internal/codec/thrift"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func TestGRPCReceiverServiceDesc(t *testing.T) {
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	desc := recv.ServiceDesc()
	if desc.ServiceName != "jaeger.api_v2.CollectorService" {
		t.Errorf("service = %q", desc.ServiceName)
	}
	if len(desc.Methods) != 1 || desc.Methods[0].Name != "PostSpans" {
		t.Error("methods")
	}
}

func TestGRPCReceiverPostSpans(t *testing.T) {
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)

	// Build a minimal protobuf PostSpansRequest
	// field 1 = batch (message)
	enc := protobuf.NewEncoder()
	enc.EncodeMessage(1, func(batchEnc *protobuf.Encoder) {
		// field 1 = process
		batchEnc.EncodeMessage(1, func(procEnc *protobuf.Encoder) {
			procEnc.WriteTagString(1, "test-service")
		})
		// field 2 = spans (repeated message)
		batchEnc.EncodeMessage(2, func(spanEnc *protobuf.Encoder) {
			spanEnc.WriteTagBytes(1, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}) // trace_id
			spanEnc.WriteTagBytes(2, []byte{1, 2, 3, 4, 5, 6, 7, 8})                                // span_id
			spanEnc.WriteTagString(3, "test-op")                                                    // operation_name
		})
	})
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	desc := recv.ServiceDesc()
	handler := desc.Methods[0].UnaryHandler

	resp, err := handler(context.Background(), reqData)
	if err != nil {
		t.Fatalf("PostSpans error: %v", err)
	}
	if resp == nil {
		t.Error("response should not be nil")
	}
	if len(sink.traces) != 1 {
		t.Errorf("traces = %d", len(sink.traces))
	}
	if recv.SpansReceived() != 1 {
		t.Errorf("received = %d", recv.SpansReceived())
	}
}

func TestGRPCReceiverInvalidData(t *testing.T) {
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	desc := recv.ServiceDesc()
	handler := desc.Methods[0].UnaryHandler

	_, err := handler(context.Background(), []byte{0xff, 0xff, 0xff})
	if err == nil {
		t.Error("should error on invalid data")
	}
}

func TestGRPCReceiverSinkError(t *testing.T) {
	sink := &mockSink{err: context.Canceled}
	recv := NewGRPCReceiver(sink)

	enc := protobuf.NewEncoder()
	enc.EncodeMessage(1, func(batchEnc *protobuf.Encoder) {
		batchEnc.EncodeMessage(2, func(spanEnc *protobuf.Encoder) {
			spanEnc.WriteTagBytes(1, make([]byte, 16))
			spanEnc.WriteTagBytes(2, make([]byte, 8))
			spanEnc.WriteTagString(3, "op")
		})
	})
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	desc := recv.ServiceDesc()
	_, err := desc.Methods[0].UnaryHandler(context.Background(), reqData)
	if err == nil {
		t.Error("should error when sink fails")
	}
}

func TestUDPReceiverNewDefaults(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", true, 0)
	if r.workerCount != 4 {
		t.Errorf("default workers = %d", r.workerCount)
	}
	if r.compact != true {
		t.Error("should be compact")
	}
}

// mockSink already defined in receiver_test.go for this package

// Ensure the types satisfy the interface
var _ TracesConsumer = (*mockSink)(nil)
var _ otlp.TracesConsumer = (*mockSink)(nil)

// ---------------------------------------------------------------------------
// unmarshalProtobufSpan branch coverage
// ---------------------------------------------------------------------------

// buildSpanReq wraps a span encoder function into a full PostSpansRequest.
func buildSpanReq(spanFn func(spanEnc *protobuf.Encoder)) []byte {
	enc := protobuf.NewEncoder()
	enc.EncodeMessage(1, func(batchEnc *protobuf.Encoder) {
		batchEnc.EncodeMessage(2, spanFn)
	})
	b := make([]byte, enc.Len())
	copy(b, enc.Bytes())
	enc.Release()
	return b
}

func TestUnmarshalProtobufSpanTraceID(t *testing.T) {
	// field 1 = trace_id bytes (16 bytes)
	traceID := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.WriteTagBytes(1, traceID)
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sink.traces) != 1 {
		t.Fatal("expected 1 trace")
	}
}

func TestUnmarshalProtobufSpanSpanID(t *testing.T) {
	// field 2 = span_id bytes (8 bytes)
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.WriteTagBytes(2, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufSpanOperationName(t *testing.T) {
	// field 3 = operation_name string
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.WriteTagString(3, "my-operation")
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sink.traces) != 1 {
		t.Fatal("expected 1 trace")
	}
	spans := sink.traces[0].ResourceSpans[0].ScopeSpans[0].Spans
	if spans[0].Name != "my-operation" {
		t.Errorf("operation name = %q", spans[0].Name)
	}
}

func TestUnmarshalProtobufSpanStartTime(t *testing.T) {
	// field 5 = start_time (google.protobuf.Timestamp), field 1 = seconds
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.EncodeMessage(5, func(inner *protobuf.Encoder) {
			inner.WriteTagVarint(1, 1000) // 1000 seconds
		})
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufSpanStartTimeSkipUnknown(t *testing.T) {
	// field 5 with an unknown sub-field (field 99) that gets skipped
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.EncodeMessage(5, func(inner *protobuf.Encoder) {
			inner.WriteTagVarint(99, 42) // unknown field, should be skipped
		})
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufSpanDuration(t *testing.T) {
	// field 6 = duration (google.protobuf.Duration), field 1 = seconds
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.EncodeMessage(6, func(inner *protobuf.Encoder) {
			inner.WriteTagVarint(1, 5) // 5 seconds
		})
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufSpanDurationSkipUnknown(t *testing.T) {
	// field 6 with unknown sub-field skipped
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.EncodeMessage(6, func(inner *protobuf.Encoder) {
			inner.WriteTagVarint(99, 0)
		})
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufSpanUnknownField(t *testing.T) {
	// field 99 = unknown, should be skipped via default branch
	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.WriteTagVarint(99, 12345)
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufSpanAllFields(t *testing.T) {
	// Exercise all span fields together
	traceID := make([]byte, 16)
	for i := range traceID {
		traceID[i] = byte(i + 1)
	}
	spanID := make([]byte, 8)
	for i := range spanID {
		spanID[i] = byte(i + 1)
	}

	data := buildSpanReq(func(e *protobuf.Encoder) {
		e.WriteTagBytes(1, traceID)
		e.WriteTagBytes(2, spanID)
		e.WriteTagString(3, "full-span")
		e.EncodeMessage(5, func(inner *protobuf.Encoder) {
			inner.WriteTagVarint(1, 1609459200) // 2021-01-01 00:00:00
		})
		e.EncodeMessage(6, func(inner *protobuf.Encoder) {
			inner.WriteTagVarint(1, 1) // 1 second
		})
		e.WriteTagVarint(99, 0) // unknown, skipped
	})
	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sink.traces) != 1 {
		t.Fatal("expected 1 trace")
	}
}

func TestUnmarshalProtobufBatchProcessSubReadFieldError(t *testing.T) {
	// batch->process submessage contains truncated varint
	// Build using encoder so lengths are correct
	enc := protobuf.NewEncoder()
	enc.EncodeMessage(1, func(batchEnc *protobuf.Encoder) {
		// process field with truncated content: write tag but not value
		// We write a "bytes" field 1 claiming 1 byte content = {0x80} (truncated varint)
		batchEnc.WriteTagBytes(1, []byte{0x80}) // process body = truncated varint
	})
	data := make([]byte, enc.Len())
	copy(data, enc.Bytes())
	enc.Release()

	_, err := unmarshalProtobufBatch(data)
	if err == nil {
		t.Error("expected error for truncated process sub-field")
	}
}

func TestUnmarshalProtobufBatchProcessServiceNameReadStringError(t *testing.T) {
	// process submessage has field 1 (serviceName) with truncated string
	// {0x0A, 0x32} = field 1 wireBytes, claims 50-byte string but data ends
	enc := protobuf.NewEncoder()
	enc.EncodeMessage(1, func(batchEnc *protobuf.Encoder) {
		batchEnc.WriteTagBytes(1, []byte{0x0A, 0x32}) // process body: field 1 wireBytes, len=50
	})
	data := make([]byte, enc.Len())
	copy(data, enc.Bytes())
	enc.Release()

	_, err := unmarshalProtobufBatch(data)
	if err == nil {
		t.Error("expected error for truncated process serviceName string")
	}
}

func TestUnmarshalProtobufBatchOuterReadFieldError(t *testing.T) {
	// Truncated varint in outer decoder — ReadField will fail
	data := []byte{0x80} // incomplete varint (MSB set, no continuation)
	_, err := unmarshalProtobufBatch(data)
	if err == nil {
		t.Error("expected error for truncated varint")
	}
}

func TestUnmarshalProtobufBatchField1ReadMessageError(t *testing.T) {
	// field 1 (batch) tag + truncated length byte — ReadMessage will fail
	// Tag for field 1, wire type 2 (bytes): 0x0A
	// Then a length varint claiming 100 bytes but payload is empty
	data := []byte{0x0A, 0x64} // tag=0x0A (field 1, wireBytes), length=100 but no data
	_, err := unmarshalProtobufBatch(data)
	if err == nil {
		t.Error("expected error for truncated batch message")
	}
}

func TestUnmarshalProtobufBatchInnerReadFieldError(t *testing.T) {
	// field 1 batch submessage with truncated inner varint
	// batch submessage contains just 0x80 (truncated varint)
	inner := []byte{0x80} // truncated
	data := make([]byte, 0, 10)
	data = append(data, 0x0A)             // tag field 1, wireBytes
	data = append(data, byte(len(inner))) // length
	data = append(data, inner...)
	_, err := unmarshalProtobufBatch(data)
	if err == nil {
		t.Error("expected error for truncated inner field")
	}
}

func TestUnmarshalProtobufBatchProcessReadMessageError(t *testing.T) {
	// batch field 1 (process) with truncated message
	// inner byte: tag for process (field 1, wireBytes = 0x0A), then length 50 but no data
	inner := []byte{0x0A, 0x32} // field 1 wireBytes, length=50
	data := make([]byte, 0, 10)
	data = append(data, 0x0A)
	data = append(data, byte(len(inner)))
	data = append(data, inner...)
	_, err := unmarshalProtobufBatch(data)
	if err == nil {
		t.Error("expected error for truncated process message")
	}
}

func TestUnmarshalProtobufBatchSpanReadMessageError(t *testing.T) {
	// batch field 2 (span) with truncated message
	// inner: tag for span (field 2, wireBytes = 0x12), length 50 but no data
	inner := []byte{0x12, 0x32} // field 2 wireBytes, length=50
	data := make([]byte, 0, 10)
	data = append(data, 0x0A)
	data = append(data, byte(len(inner)))
	data = append(data, inner...)
	_, err := unmarshalProtobufBatch(data)
	if err == nil {
		t.Error("expected error for truncated span message")
	}
}

func TestUnmarshalProtobufSpanReadFieldError(t *testing.T) {
	// span decoder with truncated varint — ReadField fails
	dec := protobuf.NewDecoder([]byte{0x80}) // truncated
	_, err := unmarshalProtobufSpan(dec)
	if err == nil {
		t.Error("expected error for truncated span field")
	}
}

func TestUnmarshalProtobufSpanTraceIDReadBytesError(t *testing.T) {
	// field 1 (traceID), wireBytes, but truncated bytes
	// tag = 0x0A (field 1, wireBytes), length = 50 (but no data)
	dec := protobuf.NewDecoder([]byte{0x0A, 0x32}) // length=50 but empty
	_, err := unmarshalProtobufSpan(dec)
	if err == nil {
		t.Error("expected error for truncated traceID bytes")
	}
}

func TestUnmarshalProtobufSpanSpanIDReadBytesError(t *testing.T) {
	// field 2 (spanID), wireBytes, truncated
	// tag = 0x12 (field 2, wireBytes), length = 50 but no data
	dec := protobuf.NewDecoder([]byte{0x12, 0x32})
	_, err := unmarshalProtobufSpan(dec)
	if err == nil {
		t.Error("expected error for truncated spanID bytes")
	}
}

func TestUnmarshalProtobufSpanOperationNameTruncated(t *testing.T) {
	// field 3 (operationName), wireBytes, truncated — ReadString fails but err is not checked inline
	// The function exits loop on Done(), returning whatever was set. Test that it doesn't panic.
	// tag = 0x1A (field 3, wireBytes), length = 50 but no data
	dec := protobuf.NewDecoder([]byte{0x1A, 0x32})
	span, _ := unmarshalProtobufSpan(dec)
	// Operation name will be empty since read failed
	if span.OperationName != "" {
		t.Errorf("expected empty operationName on truncated read, got %q", span.OperationName)
	}
}

func TestUnmarshalProtobufSpanStartTimeReadMessageError(t *testing.T) {
	// field 5 (startTime), wireBytes, truncated
	// tag = 0x2A (field 5, wireBytes), length = 50 but no data
	dec := protobuf.NewDecoder([]byte{0x2A, 0x32})
	_, err := unmarshalProtobufSpan(dec)
	if err == nil {
		t.Error("expected error for truncated startTime message")
	}
}

func TestUnmarshalProtobufSpanDurationReadMessageError(t *testing.T) {
	// field 6 (duration), wireBytes, truncated
	// tag = 0x32 (field 6, wireBytes), length = 50 but no data
	dec := protobuf.NewDecoder([]byte{0x32, 0x32})
	_, err := unmarshalProtobufSpan(dec)
	if err == nil {
		t.Error("expected error for truncated duration message")
	}
}

func TestUnmarshalProtobufBatchUnknownOuterField(t *testing.T) {
	// outer field 99 = unknown, should be skipped
	enc := protobuf.NewEncoder()
	enc.WriteTagVarint(99, 0)
	data := make([]byte, enc.Len())
	copy(data, enc.Bytes())
	enc.Release()

	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufBatchProcessUnknownField(t *testing.T) {
	// process with an unknown field (field 99) that gets skipped
	enc := protobuf.NewEncoder()
	enc.EncodeMessage(1, func(batchEnc *protobuf.Encoder) {
		batchEnc.EncodeMessage(1, func(procEnc *protobuf.Encoder) {
			procEnc.WriteTagString(1, "my-svc")
			procEnc.WriteTagVarint(99, 42) // unknown, skipped
		})
	})
	data := make([]byte, enc.Len())
	copy(data, enc.Bytes())
	enc.Release()

	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalProtobufBatchUnknownBatchField(t *testing.T) {
	// batch with unknown sub-field 99 that gets skipped
	enc := protobuf.NewEncoder()
	enc.EncodeMessage(1, func(batchEnc *protobuf.Encoder) {
		batchEnc.WriteTagVarint(99, 0) // unknown batch field
	})
	data := make([]byte, enc.Len())
	copy(data, enc.Bytes())
	enc.Release()

	sink := &mockSink{}
	recv := NewGRPCReceiver(sink)
	_, err := recv.ServiceDesc().Methods[0].UnaryHandler(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// UDP receiver lifecycle
// ---------------------------------------------------------------------------

func TestUDPReceiverStartShutdown(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", false, 2)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	addr := r.Addr()
	if addr == "" {
		t.Error("Addr() should return non-empty after Start")
	}

	if err := r.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestUDPReceiverStartInvalidAddr(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "invalid-addr-!!!:99999", false, 1)
	err := r.Start(context.Background())
	if err == nil {
		t.Error("expected error for invalid address")
		r.Shutdown(context.Background())
	}
}

func TestUDPReceiverCounters(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", false, 1)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Shutdown(ctx)

	if r.Received() != 0 {
		t.Errorf("Received = %d, want 0", r.Received())
	}
	if r.Dropped() != 0 {
		t.Errorf("Dropped = %d, want 0", r.Dropped())
	}
}

func TestUDPReceiverAddrBeforeStart(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:6832", false, 1)
	// Before Start, Addr returns the configured addr
	if r.Addr() != "127.0.0.1:6832" {
		t.Errorf("Addr = %q, want 127.0.0.1:6832", r.Addr())
	}
}

func TestUDPReceiverShutdownNilState(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", true, 1)
	// Shutdown without Start should be a no-op
	if err := r.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown without Start: %v", err)
	}
}

func TestUDPReceiverStartListenError(t *testing.T) {
	// Bind a port first, then try to bind the same port to trigger ListenUDP error
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skip("cannot bind initial port:", err)
	}
	addr := conn.LocalAddr().String()
	defer conn.Close()

	sink := &mockSink{}
	r := NewUDPReceiver(sink, addr, false, 1)
	err = r.Start(context.Background())
	if err == nil {
		t.Error("expected ListenUDP error when port already bound")
		r.Shutdown(context.Background())
	}
}

func TestUDPReceiverCompactMode(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", true, 1)
	if !r.compact {
		t.Error("expected compact=true")
	}
	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Shutdown(ctx)
}

func TestUDPReceiverReadLoopConnError(t *testing.T) {
	// Trigger the ReadFromUDP error path while context is NOT done.
	// We set a read deadline on the connection so ReadFromUDP returns an error
	// while ctx is still active, hitting the "default: dropped.Add(1)" branch.
	// Since it then loops back and the context check at the top is non-blocking,
	// the loop continues. We then cancel to clean up.
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", false, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Set a very short read deadline so ReadFromUDP returns a timeout error
	// while ctx is still active. This hits the drop path.
	r.conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))

	// Wait for the timeout to fire and the drop to be counted
	time.Sleep(100 * time.Millisecond)

	// After the deadline fires, ReadFromUDP keeps returning errors. Cancel to exit.
	cancel()
	r.Shutdown(context.Background())
}

func TestUDPReceiverReadLoopBadData(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", false, 1)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Shutdown(ctx)

	// Send invalid thrift data — should cause a drop
	addr := r.Addr()
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	conn.Write([]byte{0xff, 0xff, 0xff, 0xff}) // invalid thrift

	// Give readLoop time to process
	time.Sleep(50 * time.Millisecond)

	if r.Dropped() == 0 {
		t.Error("expected dropped > 0 for invalid data")
	}
}

func TestUDPReceiverReadLoopValidData(t *testing.T) {
	sink := &mockSink{}
	r := NewUDPReceiver(sink, "127.0.0.1:0", false, 2)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Shutdown(ctx)

	addr := r.Addr()
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send valid thrift batch with one span
	enc := thriftEncoder()
	conn.Write(enc)

	// Give readLoop time to process
	time.Sleep(100 * time.Millisecond)

	if r.Received() == 0 {
		t.Error("expected received > 0 for valid data")
	}
}

func TestUDPReceiverReadLoopSinkError(t *testing.T) {
	sink := &mockSink{err: context.Canceled}
	r := NewUDPReceiver(sink, "127.0.0.1:0", false, 1)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Shutdown(ctx)

	addr := r.Addr()
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	conn.Write(thriftEncoder())
	time.Sleep(100 * time.Millisecond)

	if r.Dropped() == 0 {
		t.Error("expected dropped > 0 when sink errors")
	}
}

// thriftEncoder builds a minimal valid Thrift batch for UDP tests.
func thriftEncoder() []byte {
	enc := thrift.NewBinaryEncoder()
	// field 2 = spans list with 1 span
	enc.WriteFieldBegin(thrift.TypeList, 2)
	enc.WriteListBegin(thrift.TypeStruct, 1)
	enc.WriteFieldBegin(thrift.TypeI64, 1)
	enc.WriteI64(1111)
	enc.WriteFieldBegin(thrift.TypeI64, 2)
	enc.WriteI64(2222)
	enc.WriteFieldBegin(thrift.TypeI64, 3)
	enc.WriteI64(3333)
	enc.WriteFieldBegin(thrift.TypeString, 5)
	enc.WriteString("udp-op")
	enc.WriteFieldBegin(thrift.TypeI64, 7)
	enc.WriteI64(1000)
	enc.WriteFieldBegin(thrift.TypeI64, 8)
	enc.WriteI64(100)
	enc.WriteFieldStop()
	enc.WriteFieldStop()
	return enc.Bytes()
}
