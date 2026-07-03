package otlp

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
	grpcpkg "github.com/asymmetric-effort/ginger/internal/net/grpc"
)

// GRPCReceiver receives OTLP traces via gRPC TraceService.Export.
type GRPCReceiver struct {
	sink          TracesConsumer
	spansReceived atomic.Int64
	spansRejected atomic.Int64
	maxSize       int
}

// NewGRPCReceiver creates a new OTLP gRPC receiver.
func NewGRPCReceiver(sink TracesConsumer, maxSize int) *GRPCReceiver {
	if maxSize <= 0 {
		maxSize = 4 * 1024 * 1024
	}
	return &GRPCReceiver{sink: sink, maxSize: maxSize}
}

// ServiceDesc returns the gRPC service descriptor for registration.
func (r *GRPCReceiver) ServiceDesc() *grpcpkg.ServiceDesc {
	return &grpcpkg.ServiceDesc{
		ServiceName: "opentelemetry.proto.collector.trace.v1.TraceService",
		Methods: []grpcpkg.MethodDesc{{
			Name: "Export",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				return r.handleExport(ctx, reqData)
			},
		}},
	}
}

func (r *GRPCReceiver) handleExport(ctx context.Context, reqData []byte) ([]byte, error) {
	td, err := Unmarshal(reqData)
	if err != nil {
		return nil, grpcpkg.NewStatusError(3, "invalid request: "+err.Error()) // INVALID_ARGUMENT
	}

	spanCount := countSpans(td)
	if err := r.sink.ConsumeTraces(ctx, td); err != nil {
		r.spansRejected.Add(int64(spanCount))
		return nil, grpcpkg.NewStatusError(8, fmt.Sprintf("resource exhausted: %d spans rejected", spanCount)) // RESOURCE_EXHAUSTED
	}

	r.spansReceived.Add(int64(spanCount))

	// Return empty ExportTraceServiceResponse
	enc := protobuf.NewEncoder()
	defer enc.Release()
	out := make([]byte, enc.Len())
	copy(out, enc.Bytes())
	return out, nil
}

// SpansReceived returns total received span count.
func (r *GRPCReceiver) SpansReceived() int64 { return r.spansReceived.Load() }

// SpansRejected returns total rejected span count.
func (r *GRPCReceiver) SpansRejected() int64 { return r.spansRejected.Load() }
