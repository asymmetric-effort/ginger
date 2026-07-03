package jaeger

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/asymmetric-effort/ginger/internal/codec/thrift"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// TracesConsumer receives trace data.
type TracesConsumer interface {
	ConsumeTraces(ctx context.Context, td otlp.TracesData) error
}

// ThriftHTTPReceiver receives Jaeger spans via HTTP POST /api/traces (Thrift Binary).
type ThriftHTTPReceiver struct {
	sink          TracesConsumer
	spansReceived atomic.Int64
	maxBodyBytes  int64
}

// NewThriftHTTPReceiver creates a new Jaeger Thrift HTTP receiver.
func NewThriftHTTPReceiver(sink TracesConsumer, maxBodyBytes int64) *ThriftHTTPReceiver {
	if maxBodyBytes <= 0 {
		maxBodyBytes = 4 * 1024 * 1024
	}
	return &ThriftHTTPReceiver{sink: sink, maxBodyBytes: maxBodyBytes}
}

// ServeHTTP handles POST /api/traces.
func (r *ThriftHTTPReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body := http.MaxBytesReader(w, req.Body, r.maxBodyBytes)
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	batch, err := decodeThriftBatch(data)
	if err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}

	td := BatchToOTLP(batch)
	if err := r.sink.ConsumeTraces(req.Context(), td); err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	r.spansReceived.Add(int64(len(batch.Spans)))
	w.WriteHeader(http.StatusAccepted)
}

// SpansReceived returns the total spans received.
func (r *ThriftHTTPReceiver) SpansReceived() int64 {
	return r.spansReceived.Load()
}

// decodeThriftBatch decodes a Thrift Binary encoded Jaeger Batch.
func decodeThriftBatch(data []byte) (Batch, error) {
	dec := thrift.NewBinaryDecoder(data)
	var batch Batch

	// Read struct fields
	for {
		fieldType, fieldID, err := dec.ReadFieldBegin()
		if err != nil {
			return batch, err
		}
		if fieldType == thrift.TypeStop {
			break
		}
		switch fieldID {
		case 1: // process
			batch.Process, err = decodeThriftProcess(dec)
		case 2: // spans (list)
			_, size, err := dec.ReadListBegin()
			if err != nil {
				return batch, err
			}
			batch.Spans = make([]Span, 0, size)
			for i := int32(0); i < size; i++ {
				span, err := decodeThriftSpan(dec)
				if err != nil {
					return batch, err
				}
				batch.Spans = append(batch.Spans, span)
			}
		default:
			skipThriftField(dec, fieldType)
		}
		if err != nil {
			return batch, err
		}
	}
	return batch, nil
}

func decodeThriftProcess(dec *thrift.BinaryDecoder) (Process, error) {
	var proc Process
	for {
		ft, fid, err := dec.ReadFieldBegin()
		if err != nil {
			return proc, err
		}
		if ft == thrift.TypeStop {
			break
		}
		switch fid {
		case 1: // serviceName
			proc.ServiceName, err = dec.ReadString()
		default:
			skipThriftField(dec, ft)
		}
		if err != nil {
			return proc, err
		}
	}
	return proc, nil
}

func decodeThriftSpan(dec *thrift.BinaryDecoder) (Span, error) {
	var span Span
	for {
		ft, fid, err := dec.ReadFieldBegin()
		if err != nil {
			return span, err
		}
		if ft == thrift.TypeStop {
			break
		}
		switch fid {
		case 1: // traceIdLow
			v, err := dec.ReadI64()
			if err != nil {
				return span, err
			}
			for i := 0; i < 8; i++ {
				span.TraceID[15-i] = byte(v >> (uint(i) * 8))
			}
		case 2: // traceIdHigh
			v, err := dec.ReadI64()
			if err != nil {
				return span, err
			}
			for i := 0; i < 8; i++ {
				span.TraceID[7-i] = byte(v >> (uint(i) * 8))
			}
		case 3: // spanId
			v, err := dec.ReadI64()
			if err != nil {
				return span, err
			}
			for i := 0; i < 8; i++ {
				span.SpanID[7-i] = byte(v >> (uint(i) * 8))
			}
		case 5: // operationName
			span.OperationName, err = dec.ReadString()
		case 7: // startTime
			v, err := dec.ReadI64()
			if err != nil {
				return span, err
			}
			span.StartTime = uint64(v)
		case 8: // duration
			v, err := dec.ReadI64()
			if err != nil {
				return span, err
			}
			span.Duration = uint64(v)
		default:
			skipThriftField(dec, ft)
		}
		if err != nil {
			return span, err
		}
	}
	return span, nil
}

func skipThriftField(dec *thrift.BinaryDecoder, ft thrift.TType) {
	switch ft {
	case thrift.TypeBool, thrift.TypeByte:
		dec.ReadByte()
	case thrift.TypeI16:
		dec.ReadI16()
	case thrift.TypeI32:
		dec.ReadI32()
	case thrift.TypeI64:
		dec.ReadI64()
	case thrift.TypeDouble:
		dec.ReadDouble()
	case thrift.TypeString:
		dec.ReadString()
	case thrift.TypeList:
		elemType, size, _ := dec.ReadListBegin()
		for i := int32(0); i < size; i++ {
			skipThriftField(dec, elemType)
		}
	case thrift.TypeMap:
		kt, vt, size, _ := dec.ReadMapBegin()
		for i := int32(0); i < size; i++ {
			skipThriftField(dec, kt)
			skipThriftField(dec, vt)
		}
	case thrift.TypeStruct:
		for {
			sft, _, _ := dec.ReadFieldBegin()
			if sft == thrift.TypeStop {
				break
			}
			skipThriftField(dec, sft)
		}
	case thrift.TypeSet:
		elemType, size, _ := dec.ReadListBegin()
		for i := int32(0); i < size; i++ {
			skipThriftField(dec, elemType)
		}
	}
}
