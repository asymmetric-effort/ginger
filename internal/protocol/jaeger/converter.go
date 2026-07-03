package jaeger

import (
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// BatchToOTLP converts a Jaeger Batch to OTLP TracesData.
func BatchToOTLP(batch Batch) otlp.TracesData {
	resAttrs := otlp.NewAttributes()
	resAttrs.Set("service.name", otlp.StringValue(batch.Process.ServiceName))
	for _, tag := range batch.Process.Tags {
		resAttrs.Set(tag.Key, kvToAnyValue(tag))
	}

	var spans []otlp.Span
	for i := range batch.Spans {
		spans = append(spans, spanToOTLP(&batch.Spans[i]))
	}

	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource:   otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{Spans: spans}},
		}},
	}
}

// OTLPToBatch converts OTLP TracesData to a Jaeger Batch.
// If the TracesData has multiple ResourceSpans, only the first is converted.
func OTLPToBatch(td otlp.TracesData) Batch {
	if len(td.ResourceSpans) == 0 {
		return Batch{}
	}
	rs := td.ResourceSpans[0]

	proc := Process{}
	svcName, ok := rs.Resource.Attributes.Get("service.name")
	if ok {
		proc.ServiceName = svcName.Str
	}
	for _, kv := range rs.Resource.Attributes.Items() {
		if kv.Key == "service.name" {
			continue
		}
		proc.Tags = append(proc.Tags, anyValueToKV(kv.Key, kv.Value))
	}

	var spans []Span
	for _, ss := range rs.ScopeSpans {
		for i := range ss.Spans {
			spans = append(spans, spanFromOTLP(&ss.Spans[i]))
		}
	}

	return Batch{Process: proc, Spans: spans}
}

func spanToOTLP(s *Span) otlp.Span {
	attrs := otlp.NewAttributes()
	for _, tag := range s.Tags {
		attrs.Set(tag.Key, kvToAnyValue(tag))
	}

	var events []otlp.SpanEvent
	for _, log := range s.Logs {
		evAttrs := otlp.NewAttributes()
		name := ""
		for _, f := range log.Fields {
			if f.Key == "event" && f.VType == VTypeString {
				name = f.VStr
			} else {
				evAttrs.Set(f.Key, kvToAnyValue(f))
			}
		}
		events = append(events, otlp.SpanEvent{
			TimeUnixNano: log.Timestamp * 1000, // us → ns
			Name:         name,
			Attributes:   evAttrs,
		})
	}

	var parentSpanID [8]byte
	for _, ref := range s.References {
		if ref.RefType == RefTypeChildOf {
			parentSpanID = ref.SpanID
			break
		}
	}

	kind := otlp.SpanKindUnspecified
	if spanKindVal, ok := attrs.Get("span.kind"); ok {
		switch spanKindVal.Str {
		case "server":
			kind = otlp.SpanKindServer
		case "client":
			kind = otlp.SpanKindClient
		case "producer":
			kind = otlp.SpanKindProducer
		case "consumer":
			kind = otlp.SpanKindConsumer
		}
	}

	return otlp.Span{
		TraceID:           s.TraceID,
		SpanID:            s.SpanID,
		ParentSpanID:      parentSpanID,
		Name:              s.OperationName,
		Kind:              kind,
		StartTimeUnixNano: s.StartTime * 1000,
		EndTimeUnixNano:   (s.StartTime + s.Duration) * 1000,
		Attributes:        attrs,
		Events:            events,
	}
}

func spanFromOTLP(s *otlp.Span) Span {
	var tags []KeyValue
	for _, kv := range s.Attributes.Items() {
		tags = append(tags, anyValueToKV(kv.Key, kv.Value))
	}

	var logs []Log
	for _, ev := range s.Events {
		fields := []KeyValue{{Key: "event", VType: VTypeString, VStr: ev.Name}}
		for _, kv := range ev.Attributes.Items() {
			fields = append(fields, anyValueToKV(kv.Key, kv.Value))
		}
		logs = append(logs, Log{
			Timestamp: ev.TimeUnixNano / 1000,
			Fields:    fields,
		})
	}

	var refs []SpanRef
	if s.ParentSpanID != [8]byte{} {
		refs = append(refs, SpanRef{
			TraceID: s.TraceID,
			SpanID:  s.ParentSpanID,
			RefType: RefTypeChildOf,
		})
	}

	duration := uint64(0)
	if s.EndTimeUnixNano > s.StartTimeUnixNano {
		duration = (s.EndTimeUnixNano - s.StartTimeUnixNano) / 1000
	}

	return Span{
		TraceID:       s.TraceID,
		SpanID:        s.SpanID,
		OperationName: s.Name,
		References:    refs,
		StartTime:     s.StartTimeUnixNano / 1000,
		Duration:      duration,
		Tags:          tags,
		Logs:          logs,
	}
}

func kvToAnyValue(kv KeyValue) otlp.AnyValue {
	switch kv.VType {
	case VTypeString:
		return otlp.StringValue(kv.VStr)
	case VTypeBool:
		return otlp.BoolValue(kv.VBool)
	case VTypeInt64:
		return otlp.IntValue(kv.VInt64)
	case VTypeFloat64:
		return otlp.DoubleValue(kv.VFloat64)
	case VTypeBinary:
		return otlp.BytesValue(kv.VBinary)
	default:
		return otlp.StringValue(kv.VStr)
	}
}

func anyValueToKV(key string, v otlp.AnyValue) KeyValue {
	switch v.Type {
	case otlp.AnyValueTypeString:
		return KeyValue{Key: key, VType: VTypeString, VStr: v.Str}
	case otlp.AnyValueTypeBool:
		return KeyValue{Key: key, VType: VTypeBool, VBool: v.BoolVal}
	case otlp.AnyValueTypeInt:
		return KeyValue{Key: key, VType: VTypeInt64, VInt64: v.IntVal}
	case otlp.AnyValueTypeDouble:
		return KeyValue{Key: key, VType: VTypeFloat64, VFloat64: v.DoubleVal}
	case otlp.AnyValueTypeBytes:
		return KeyValue{Key: key, VType: VTypeBinary, VBinary: v.BytesVal}
	default:
		return KeyValue{Key: key, VType: VTypeString, VStr: v.Str}
	}
}
