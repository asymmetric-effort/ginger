package jaeger

import (
	"testing"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func TestBatchToOTLP(t *testing.T) {
	batch := Batch{
		Process: Process{
			ServiceName: "myservice",
			Tags:        []KeyValue{{Key: "version", VType: VTypeString, VStr: "1.0"}},
		},
		Spans: []Span{{
			TraceID:       [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			SpanID:        [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			OperationName: "GET /api",
			StartTime:     1000000, // 1s in us
			Duration:      500000,  // 0.5s in us
			Tags: []KeyValue{
				{Key: "http.method", VType: VTypeString, VStr: "GET"},
				{Key: "http.status_code", VType: VTypeInt64, VInt64: 200},
				{Key: "error", VType: VTypeBool, VBool: false},
				{Key: "latency", VType: VTypeFloat64, VFloat64: 0.5},
				{Key: "data", VType: VTypeBinary, VBinary: []byte{0x01}},
			},
			Logs: []Log{{
				Timestamp: 1250000,
				Fields: []KeyValue{
					{Key: "event", VType: VTypeString, VStr: "request.received"},
					{Key: "size", VType: VTypeInt64, VInt64: 1024},
				},
			}},
			References: []SpanRef{{
				TraceID: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				SpanID:  [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
				RefType: RefTypeChildOf,
			}},
		}},
	}

	td := BatchToOTLP(batch)

	if len(td.ResourceSpans) != 1 {
		t.Fatalf("ResourceSpans: %d", len(td.ResourceSpans))
	}
	rs := td.ResourceSpans[0]
	svc, ok := rs.Resource.Attributes.Get("service.name")
	if !ok || svc.Str != "myservice" {
		t.Error("service.name")
	}
	ver, ok := rs.Resource.Attributes.Get("version")
	if !ok || ver.Str != "1.0" {
		t.Error("version tag")
	}

	spans := rs.ScopeSpans[0].Spans
	if len(spans) != 1 {
		t.Fatalf("spans: %d", len(spans))
	}
	span := spans[0]
	if span.Name != "GET /api" {
		t.Errorf("name = %q", span.Name)
	}
	if span.StartTimeUnixNano != 1000000000 {
		t.Errorf("start = %d", span.StartTimeUnixNano)
	}
	if span.EndTimeUnixNano != 1500000000 {
		t.Errorf("end = %d", span.EndTimeUnixNano)
	}
	if span.ParentSpanID != [8]byte{8, 7, 6, 5, 4, 3, 2, 1} {
		t.Error("parent span ID")
	}
	if len(span.Events) != 1 {
		t.Fatalf("events: %d", len(span.Events))
	}
	if span.Events[0].Name != "request.received" {
		t.Error("event name")
	}
}

func TestBatchToOTLPSpanKind(t *testing.T) {
	batch := Batch{
		Process: Process{ServiceName: "svc"},
		Spans: []Span{{
			TraceID: [16]byte{1}, SpanID: [8]byte{1},
			OperationName: "test", StartTime: 1, Duration: 1,
			Tags: []KeyValue{{Key: "span.kind", VType: VTypeString, VStr: "server"}},
		}},
	}
	td := BatchToOTLP(batch)
	if td.ResourceSpans[0].ScopeSpans[0].Spans[0].Kind != otlp.SpanKindServer {
		t.Error("expected server kind")
	}
}

func TestBatchToOTLPClientKind(t *testing.T) {
	batch := Batch{
		Process: Process{ServiceName: "svc"},
		Spans: []Span{{
			TraceID: [16]byte{1}, SpanID: [8]byte{1},
			OperationName: "test", StartTime: 1, Duration: 1,
			Tags: []KeyValue{{Key: "span.kind", VType: VTypeString, VStr: "client"}},
		}},
	}
	td := BatchToOTLP(batch)
	if td.ResourceSpans[0].ScopeSpans[0].Spans[0].Kind != otlp.SpanKindClient {
		t.Error("expected client kind")
	}
}

func TestBatchToOTLPProducerConsumerKind(t *testing.T) {
	for _, kind := range []string{"producer", "consumer"} {
		batch := Batch{
			Process: Process{ServiceName: "svc"},
			Spans: []Span{{
				TraceID: [16]byte{1}, SpanID: [8]byte{1},
				OperationName: "test", StartTime: 1, Duration: 1,
				Tags: []KeyValue{{Key: "span.kind", VType: VTypeString, VStr: kind}},
			}},
		}
		td := BatchToOTLP(batch)
		span := td.ResourceSpans[0].ScopeSpans[0].Spans[0]
		if kind == "producer" && span.Kind != otlp.SpanKindProducer {
			t.Errorf("expected producer")
		}
		if kind == "consumer" && span.Kind != otlp.SpanKindConsumer {
			t.Errorf("expected consumer")
		}
	}
}

func TestOTLPToBatch(t *testing.T) {
	attrs := otlp.NewAttributes()
	attrs.Set("http.method", otlp.StringValue("GET"))

	eventAttrs := otlp.NewAttributes()
	eventAttrs.Set("size", otlp.IntValue(1024))

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource: otlp.Resource{Attributes: func() otlp.Attributes {
				a := otlp.NewAttributes()
				a.Set("service.name", otlp.StringValue("myservice"))
				a.Set("version", otlp.StringValue("2.0"))
				return a
			}()},
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID:           [16]byte{1},
					SpanID:            [8]byte{2},
					ParentSpanID:      [8]byte{3},
					Name:              "GET /api",
					Kind:              otlp.SpanKindServer,
					StartTimeUnixNano: 1000000000,
					EndTimeUnixNano:   1500000000,
					Attributes:        attrs,
					Events: []otlp.SpanEvent{{
						TimeUnixNano: 1250000000,
						Name:         "recv",
						Attributes:   eventAttrs,
					}},
				}},
			}},
		}},
	}

	batch := OTLPToBatch(td)
	if batch.Process.ServiceName != "myservice" {
		t.Errorf("service = %q", batch.Process.ServiceName)
	}
	if len(batch.Process.Tags) != 1 || batch.Process.Tags[0].Key != "version" {
		t.Errorf("tags = %v", batch.Process.Tags)
	}
	if len(batch.Spans) != 1 {
		t.Fatalf("spans: %d", len(batch.Spans))
	}
	span := batch.Spans[0]
	if span.OperationName != "GET /api" {
		t.Error("operation name")
	}
	if span.StartTime != 1000000 {
		t.Errorf("start = %d", span.StartTime)
	}
	if span.Duration != 500000 {
		t.Errorf("duration = %d", span.Duration)
	}
	if len(span.References) != 1 || span.References[0].RefType != RefTypeChildOf {
		t.Error("references")
	}
	if len(span.Logs) != 1 || span.Logs[0].Fields[0].VStr != "recv" {
		t.Error("logs")
	}
}

func TestOTLPToBatchEmpty(t *testing.T) {
	batch := OTLPToBatch(otlp.TracesData{})
	if batch.Process.ServiceName != "" {
		t.Error("empty should produce empty batch")
	}
}

func TestOTLPToBatchNoParent(t *testing.T) {
	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: "test", StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	batch := OTLPToBatch(td)
	if len(batch.Spans[0].References) != 0 {
		t.Error("no parent should mean no refs")
	}
}

func TestKvToAnyValueDefault(t *testing.T) {
	kv := KeyValue{Key: "k", VType: VType(99), VStr: "fallback"}
	av := kvToAnyValue(kv)
	if av.Type != otlp.AnyValueTypeString || av.Str != "fallback" {
		t.Error("unknown VType should default to string")
	}
}

func TestAnyValueToKVDefault(t *testing.T) {
	av := otlp.AnyValue{Type: otlp.AnyValueType(99), Str: "fallback"}
	kv := anyValueToKV("k", av)
	if kv.VType != VTypeString || kv.VStr != "fallback" {
		t.Error("unknown AnyValueType should default to string")
	}
}

func TestAnyValueToKVAllTypes(t *testing.T) {
	tests := []struct {
		name string
		av   otlp.AnyValue
		vt   VType
	}{
		{"bool", otlp.BoolValue(true), VTypeBool},
		{"int", otlp.IntValue(42), VTypeInt64},
		{"double", otlp.DoubleValue(3.14), VTypeFloat64},
		{"bytes", otlp.BytesValue([]byte{1}), VTypeBinary},
		{"string", otlp.StringValue("hi"), VTypeString},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := anyValueToKV("k", tt.av)
			if kv.VType != tt.vt {
				t.Errorf("VType = %d, want %d", kv.VType, tt.vt)
			}
		})
	}
}

func TestOTLPToBatchZeroDuration(t *testing.T) {
	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{
				Spans: []otlp.Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: "test", StartTimeUnixNano: 1000, EndTimeUnixNano: 1000,
				}},
			}},
		}},
	}
	batch := OTLPToBatch(td)
	if batch.Spans[0].Duration != 0 {
		t.Errorf("duration should be 0, got %d", batch.Spans[0].Duration)
	}
}
