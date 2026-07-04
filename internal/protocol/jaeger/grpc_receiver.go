package jaeger

import (
	"context"
	"sync/atomic"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
	grpcpkg "github.com/asymmetric-effort/ginger/internal/net/grpc"
)

// GRPCReceiver receives Jaeger spans via gRPC CollectorService.PostSpans.
type GRPCReceiver struct {
	sink          TracesConsumer
	spansReceived atomic.Int64
}

// NewGRPCReceiver creates a Jaeger gRPC receiver.
func NewGRPCReceiver(sink TracesConsumer) *GRPCReceiver {
	return &GRPCReceiver{sink: sink}
}

// ServiceDesc returns the gRPC service descriptor.
func (r *GRPCReceiver) ServiceDesc() *grpcpkg.ServiceDesc {
	return &grpcpkg.ServiceDesc{
		ServiceName: "jaeger.api_v2.CollectorService",
		Methods: []grpcpkg.MethodDesc{{
			Name: "PostSpans",
			UnaryHandler: func(ctx context.Context, reqData []byte) ([]byte, error) {
				return r.handlePostSpans(ctx, reqData)
			},
		}},
	}
}

func (r *GRPCReceiver) handlePostSpans(ctx context.Context, reqData []byte) ([]byte, error) {
	batch, err := unmarshalProtobufBatch(reqData)
	if err != nil {
		return nil, grpcpkg.NewStatusError(3, "invalid request: "+err.Error())
	}

	td := BatchToOTLP(batch)
	if err := r.sink.ConsumeTraces(ctx, td); err != nil {
		return nil, grpcpkg.NewStatusError(8, "resource exhausted")
	}

	r.spansReceived.Add(int64(len(batch.Spans)))

	enc := protobuf.NewEncoder()
	defer enc.Release()
	out := make([]byte, enc.Len())
	return out, nil
}

// SpansReceived returns total received span count.
func (r *GRPCReceiver) SpansReceived() int64 { return r.spansReceived.Load() }

// unmarshalProtobufBatch decodes a Jaeger Protobuf PostSpansRequest.
func unmarshalProtobufBatch(data []byte) (Batch, error) {
	dec := protobuf.NewDecoder(data)
	var batch Batch

	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return batch, err
		}
		switch fn {
		case 1: // batch
			sub, err := dec.ReadMessage()
			if err != nil {
				return batch, err
			}
			for !sub.Done() {
				bfn, bwt, err := sub.ReadField()
				if err != nil {
					return batch, err
				}
				switch bfn {
				case 1: // process
					psub, err := sub.ReadMessage()
					if err != nil {
						return batch, err
					}
					for !psub.Done() {
						pfn, pwt, err := psub.ReadField()
						if err != nil {
							return batch, err
						}
						if pfn == 1 {
							batch.Process.ServiceName, err = psub.ReadString()
							if err != nil {
								return batch, err
							}
						} else {
							psub.SkipField(pwt)
						}
					}
				case 2: // spans (repeated)
					ssub, err := sub.ReadMessage()
					if err != nil {
						return batch, err
					}
					span, err := unmarshalProtobufSpan(ssub)
					if err != nil {
						return batch, err
					}
					batch.Spans = append(batch.Spans, span)
				default:
					sub.SkipField(bwt)
				}
			}
		default:
			dec.SkipField(wt)
		}
	}
	return batch, nil
}

func unmarshalProtobufSpan(dec *protobuf.Decoder) (Span, error) {
	var span Span
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return span, err
		}
		switch fn {
		case 1: // trace_id (bytes)
			b, err := dec.ReadBytes()
			if err != nil {
				return span, err
			}
			copy(span.TraceID[:], b)
		case 2: // span_id (bytes)
			b, err := dec.ReadBytes()
			if err != nil {
				return span, err
			}
			copy(span.SpanID[:], b)
		case 3: // operation_name
			span.OperationName, _ = dec.ReadString()
		case 5: // start_time (google.protobuf.Timestamp)
			sub, err := dec.ReadMessage()
			if err != nil {
				return span, err
			}
			for !sub.Done() {
				sfn, swt, _ := sub.ReadField()
				if sfn == 1 {
					secs, _ := sub.ReadVarint()
					span.StartTime = secs * 1000000 // seconds → microseconds
				} else {
					sub.SkipField(swt)
				}
			}
		case 6: // duration (google.protobuf.Duration)
			sub, err := dec.ReadMessage()
			if err != nil {
				return span, err
			}
			for !sub.Done() {
				sfn, swt, _ := sub.ReadField()
				if sfn == 1 {
					secs, _ := sub.ReadVarint()
					span.Duration = secs * 1000000
				} else {
					sub.SkipField(swt)
				}
			}
		default:
			dec.SkipField(wt)
			if err != nil {
				return span, err
			}
		}
	}
	return span, nil
}
