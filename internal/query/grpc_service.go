package query

import (
	"context"
	"encoding/hex"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
	grpcpkg "github.com/asymmetric-effort/ginger/internal/net/grpc"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

// GRPCHandler serves the Query API v3 gRPC service.
type GRPCHandler struct {
	service *Service
}

// NewGRPCHandler creates a gRPC query handler.
func NewGRPCHandler(service *Service) *GRPCHandler {
	return &GRPCHandler{service: service}
}

// ServiceDesc returns the gRPC service descriptor.
func (h *GRPCHandler) ServiceDesc() *grpcpkg.ServiceDesc {
	return &grpcpkg.ServiceDesc{
		ServiceName: "jaeger.api_v3.QueryService",
		Methods: []grpcpkg.MethodDesc{
			{Name: "GetTrace", UnaryHandler: h.handleGetTrace},
			{Name: "GetServices", UnaryHandler: h.handleGetServices},
			{Name: "GetOperations", UnaryHandler: h.handleGetOperations},
		},
	}
}

func (h *GRPCHandler) handleGetTrace(ctx context.Context, reqData []byte) ([]byte, error) {
	// Parse GetTraceRequest: field 1 = trace_id (string hex)
	dec := protobuf.NewDecoder(reqData)
	var traceIDHex string
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return nil, grpcpkg.NewStatusError(3, "invalid request")
		}
		if fn == 1 {
			traceIDHex, _ = dec.ReadString()
		} else {
			dec.SkipField(wt)
		}
	}

	if traceIDHex == "" {
		return nil, grpcpkg.NewStatusError(3, "trace_id required")
	}

	var traceID storage.TraceID
	b, err := hex.DecodeString(traceIDHex)
	if err != nil || len(b) != 16 {
		return nil, grpcpkg.NewStatusError(3, "invalid trace_id")
	}
	copy(traceID[:], b)

	td, err := h.service.GetTrace(ctx, traceID)
	if err != nil {
		if err == storage.ErrTraceNotFound {
			return nil, grpcpkg.NewStatusError(5, "trace not found")
		}
		return nil, grpcpkg.NewStatusError(13, err.Error())
	}

	respData, err := otlp.Marshal(td)
	if err != nil {
		return nil, grpcpkg.NewStatusError(13, "marshal error")
	}
	return respData, nil
}

func (h *GRPCHandler) handleGetServices(ctx context.Context, _ []byte) ([]byte, error) {
	services, err := h.service.GetServices(ctx)
	if err != nil {
		return nil, grpcpkg.NewStatusError(13, err.Error())
	}

	enc := protobuf.NewEncoder()
	defer enc.Release()
	for _, svc := range services {
		enc.WriteTagString(1, svc) // field 1 = services (repeated string)
	}
	out := make([]byte, enc.Len())
	copy(out, enc.Bytes())
	return out, nil
}

func (h *GRPCHandler) handleGetOperations(ctx context.Context, reqData []byte) ([]byte, error) {
	dec := protobuf.NewDecoder(reqData)
	var service string
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return nil, grpcpkg.NewStatusError(3, "invalid request")
		}
		if fn == 1 {
			service, _ = dec.ReadString()
		} else {
			dec.SkipField(wt)
		}
	}

	ops, err := h.service.GetOperations(ctx, service)
	if err != nil {
		return nil, grpcpkg.NewStatusError(13, err.Error())
	}

	enc := protobuf.NewEncoder()
	defer enc.Release()
	for _, op := range ops {
		enc.EncodeMessage(1, func(e *protobuf.Encoder) {
			e.WriteTagString(1, op.Name)
			if op.SpanKind != "" {
				e.WriteTagString(2, op.SpanKind)
			}
		})
	}
	out := make([]byte, enc.Len())
	copy(out, enc.Bytes())
	return out, nil
}
