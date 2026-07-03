package query

import (
	"context"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage/memory"
)

func TestGRPCHandlerGetTrace(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)

	// Seed data
	tid := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("test-svc"))
	backend.WriteSpans(context.Background(), otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: tid, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000}},
			}},
		}},
	})

	desc := h.ServiceDesc()
	if desc.ServiceName != "jaeger.api_v3.QueryService" {
		t.Errorf("service = %q", desc.ServiceName)
	}

	// Build GetTraceRequest
	enc := protobuf.NewEncoder()
	enc.WriteTagString(1, "0102030405060708090a0b0c0d0e0f10")
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	resp, err := desc.Methods[0].UnaryHandler(context.Background(), reqData)
	if err != nil {
		t.Fatalf("GetTrace error: %v", err)
	}
	if len(resp) == 0 {
		t.Error("empty response")
	}
}

func TestGRPCHandlerGetTraceNotFound(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)

	enc := protobuf.NewEncoder()
	enc.WriteTagString(1, "ffffffffffffffffffffffffffffffff")
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	desc := h.ServiceDesc()
	_, err := desc.Methods[0].UnaryHandler(context.Background(), reqData)
	if err == nil {
		t.Error("should error for not found")
	}
}

func TestGRPCHandlerGetTraceEmptyID(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)

	desc := h.ServiceDesc()
	_, err := desc.Methods[0].UnaryHandler(context.Background(), []byte{})
	if err == nil {
		t.Error("should error for empty ID")
	}
}

func TestGRPCHandlerGetTraceInvalidID(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)

	enc := protobuf.NewEncoder()
	enc.WriteTagString(1, "invalid-hex")
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	desc := h.ServiceDesc()
	_, err := desc.Methods[0].UnaryHandler(context.Background(), reqData)
	if err == nil {
		t.Error("should error for invalid hex ID")
	}
}

func TestGRPCHandlerGetServices(t *testing.T) {
	backend := memory.NewBackend(100)
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("grpc-svc"))
	backend.WriteSpans(context.Background(), otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "op",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	})

	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	// GetServices is method index 1
	resp, err := desc.Methods[1].UnaryHandler(context.Background(), []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) == 0 {
		t.Error("should return services")
	}
}

func TestGRPCHandlerGetOperations(t *testing.T) {
	backend := memory.NewBackend(100)
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue("op-svc"))
	backend.WriteSpans(context.Background(), otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "GET /api",
					StartTimeUnixNano: 1, EndTimeUnixNano: 2}},
			}},
		}},
	})

	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	enc := protobuf.NewEncoder()
	enc.WriteTagString(1, "op-svc")
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	resp, err := desc.Methods[2].UnaryHandler(context.Background(), reqData)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) == 0 {
		t.Error("should return operations")
	}
}

func TestGRPCHandlerGetTraceInvalidRequest(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	// Send truncated/invalid protobuf to trigger ReadField error
	_, err := desc.Methods[0].UnaryHandler(context.Background(), []byte{0xff, 0xff})
	if err == nil {
		t.Error("should error for invalid protobuf request")
	}
}

func TestGRPCHandlerGetTraceUnknownField(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	// Encode field number 2 (unknown to GetTrace) to trigger SkipField path
	enc := protobuf.NewEncoder()
	enc.WriteTagString(2, "unknown")
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	// This should fail because no trace_id provided (field 1)
	_, err := desc.Methods[0].UnaryHandler(context.Background(), reqData)
	if err == nil {
		t.Error("should error: trace_id required")
	}
}

func TestGRPCHandlerGetServicesError(t *testing.T) {
	svc := NewService(&errorReader{}, &errorReader{}, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	_, err := desc.Methods[1].UnaryHandler(context.Background(), []byte{})
	if err == nil {
		t.Error("should error when GetServices fails")
	}
}

func TestGRPCHandlerGetOperationsError(t *testing.T) {
	svc := NewService(&errorReader{}, &errorReader{}, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	enc := protobuf.NewEncoder()
	enc.WriteTagString(1, "svc")
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	_, err := desc.Methods[2].UnaryHandler(context.Background(), reqData)
	if err == nil {
		t.Error("should error when GetOperations fails")
	}
}

func TestGRPCHandlerGetOperationsInvalidRequest(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	// Send truncated/invalid protobuf to trigger ReadField error in GetOperations
	_, err := desc.Methods[2].UnaryHandler(context.Background(), []byte{0xff, 0xff})
	if err == nil {
		t.Error("should error for invalid protobuf request in GetOperations")
	}
}

func TestGRPCHandlerGetOperationsUnknownField(t *testing.T) {
	backend := memory.NewBackend(100)
	svc := NewService(backend, backend, nil)
	h := NewGRPCHandler(svc)
	desc := h.ServiceDesc()

	// Encode field number 2 (unknown to GetOperations) to trigger SkipField path
	enc := protobuf.NewEncoder()
	enc.WriteTagString(2, "unknown")
	reqData := make([]byte, enc.Len())
	copy(reqData, enc.Bytes())
	enc.Release()

	// This should succeed with empty result (no service filter = all ops)
	_, err := desc.Methods[2].UnaryHandler(context.Background(), reqData)
	if err != nil {
		t.Errorf("should succeed with unknown field (ops for all services): %v", err)
	}
}
