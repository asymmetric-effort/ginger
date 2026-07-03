package jaeger

import (
	"context"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
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
			spanEnc.WriteTagBytes(2, []byte{1, 2, 3, 4, 5, 6, 7, 8})                                 // span_id
			spanEnc.WriteTagString(3, "test-op")                                                       // operation_name
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
