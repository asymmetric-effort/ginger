package otlp

import (
	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
)

// OTLP protobuf field numbers (from opentelemetry-proto).
const (
	// TracesData
	fieldTracesDataResourceSpans = 1

	// ResourceSpans
	fieldResourceSpansResource   = 1
	fieldResourceSpansScopeSpans = 2

	// Resource
	fieldResourceAttributes = 1

	// ScopeSpans
	fieldScopeSpansScope = 1
	fieldScopeSpansSpans = 2

	// InstrumentationScope
	fieldScopeName       = 1
	fieldScopeVersion    = 2
	fieldScopeAttributes = 3

	// Span
	fieldSpanTraceID                = 1
	fieldSpanSpanID                 = 2
	fieldSpanTraceState             = 3
	fieldSpanParentSpanID           = 4
	fieldSpanName                   = 5
	fieldSpanKind                   = 6
	fieldSpanStartTimeUnixNano      = 7
	fieldSpanEndTimeUnixNano        = 8
	fieldSpanAttributes             = 9
	fieldSpanEvents                 = 11
	fieldSpanLinks                  = 12
	fieldSpanStatus                 = 13
	fieldSpanDroppedAttributesCount = 14
	fieldSpanDroppedEventsCount     = 15
	fieldSpanDroppedLinksCount      = 16

	// SpanEvent
	fieldEventTimeUnixNano           = 1
	fieldEventName                   = 2
	fieldEventAttributes             = 3
	fieldEventDroppedAttributesCount = 4

	// SpanLink
	fieldLinkTraceID                = 1
	fieldLinkSpanID                 = 2
	fieldLinkTraceState             = 3
	fieldLinkAttributes             = 4
	fieldLinkDroppedAttributesCount = 5

	// Status
	fieldStatusMessage = 2
	fieldStatusCode    = 3

	// KeyValue
	fieldKVKey   = 1
	fieldKVValue = 2

	// AnyValue
	fieldAnyValueString = 1
	fieldAnyValueBool   = 2
	fieldAnyValueInt    = 3
	fieldAnyValueDouble = 4
	fieldAnyValueArray  = 5
	fieldAnyValueKvList = 6
	fieldAnyValueBytes  = 7

	// ArrayValue
	fieldArrayValues = 1
	// KvListValue
	fieldKvListValues = 1
)

// Marshal encodes TracesData to protobuf bytes.
func Marshal(td TracesData) ([]byte, error) {
	enc := protobuf.NewEncoder()
	defer enc.Release()

	for i := range td.ResourceSpans {
		enc.EncodeMessage(fieldTracesDataResourceSpans, func(e *protobuf.Encoder) {
			marshalResourceSpans(e, &td.ResourceSpans[i])
		})
	}

	out := make([]byte, enc.Len())
	copy(out, enc.Bytes())
	return out, nil
}

func marshalResourceSpans(enc *protobuf.Encoder, rs *ResourceSpans) {
	if rs.Resource.Attributes.Len() > 0 {
		enc.EncodeMessage(fieldResourceSpansResource, func(e *protobuf.Encoder) {
			marshalAttributes(e, fieldResourceAttributes, rs.Resource.Attributes)
		})
	}
	for i := range rs.ScopeSpans {
		enc.EncodeMessage(fieldResourceSpansScopeSpans, func(e *protobuf.Encoder) {
			marshalScopeSpans(e, &rs.ScopeSpans[i])
		})
	}
}

func marshalScopeSpans(enc *protobuf.Encoder, ss *ScopeSpans) {
	if ss.Scope.Name != "" || ss.Scope.Version != "" {
		enc.EncodeMessage(fieldScopeSpansScope, func(e *protobuf.Encoder) {
			if ss.Scope.Name != "" {
				e.WriteTagString(fieldScopeName, ss.Scope.Name)
			}
			if ss.Scope.Version != "" {
				e.WriteTagString(fieldScopeVersion, ss.Scope.Version)
			}
			marshalAttributes(e, fieldScopeAttributes, ss.Scope.Attributes)
		})
	}
	for i := range ss.Spans {
		enc.EncodeMessage(fieldScopeSpansSpans, func(e *protobuf.Encoder) {
			marshalSpan(e, &ss.Spans[i])
		})
	}
}

func marshalSpan(enc *protobuf.Encoder, s *Span) {
	enc.WriteTagBytes(fieldSpanTraceID, s.TraceID[:])
	enc.WriteTagBytes(fieldSpanSpanID, s.SpanID[:])
	if s.TraceState != "" {
		enc.WriteTagString(fieldSpanTraceState, s.TraceState)
	}
	if s.ParentSpanID != [8]byte{} {
		enc.WriteTagBytes(fieldSpanParentSpanID, s.ParentSpanID[:])
	}
	enc.WriteTagString(fieldSpanName, s.Name)
	if s.Kind != SpanKindUnspecified {
		enc.WriteTagVarint(fieldSpanKind, uint64(s.Kind))
	}
	enc.WriteTagFixed64(fieldSpanStartTimeUnixNano, s.StartTimeUnixNano)
	enc.WriteTagFixed64(fieldSpanEndTimeUnixNano, s.EndTimeUnixNano)
	marshalAttributes(enc, fieldSpanAttributes, s.Attributes)
	for i := range s.Events {
		enc.EncodeMessage(fieldSpanEvents, func(e *protobuf.Encoder) {
			marshalSpanEvent(e, &s.Events[i])
		})
	}
	for i := range s.Links {
		enc.EncodeMessage(fieldSpanLinks, func(e *protobuf.Encoder) {
			marshalSpanLink(e, &s.Links[i])
		})
	}
	if s.Status.Code != StatusCodeUnset || s.Status.Message != "" {
		enc.EncodeMessage(fieldSpanStatus, func(e *protobuf.Encoder) {
			if s.Status.Message != "" {
				e.WriteTagString(fieldStatusMessage, s.Status.Message)
			}
			if s.Status.Code != StatusCodeUnset {
				e.WriteTagVarint(fieldStatusCode, uint64(s.Status.Code))
			}
		})
	}
	if s.DroppedAttributesCount > 0 {
		enc.WriteTagVarint(fieldSpanDroppedAttributesCount, uint64(s.DroppedAttributesCount))
	}
	if s.DroppedEventsCount > 0 {
		enc.WriteTagVarint(fieldSpanDroppedEventsCount, uint64(s.DroppedEventsCount))
	}
	if s.DroppedLinksCount > 0 {
		enc.WriteTagVarint(fieldSpanDroppedLinksCount, uint64(s.DroppedLinksCount))
	}
}

func marshalSpanEvent(enc *protobuf.Encoder, e *SpanEvent) {
	enc.WriteTagFixed64(fieldEventTimeUnixNano, e.TimeUnixNano)
	enc.WriteTagString(fieldEventName, e.Name)
	marshalAttributes(enc, fieldEventAttributes, e.Attributes)
	if e.DroppedAttributesCount > 0 {
		enc.WriteTagVarint(fieldEventDroppedAttributesCount, uint64(e.DroppedAttributesCount))
	}
}

func marshalSpanLink(enc *protobuf.Encoder, l *SpanLink) {
	enc.WriteTagBytes(fieldLinkTraceID, l.TraceID[:])
	enc.WriteTagBytes(fieldLinkSpanID, l.SpanID[:])
	if l.TraceState != "" {
		enc.WriteTagString(fieldLinkTraceState, l.TraceState)
	}
	marshalAttributes(enc, fieldLinkAttributes, l.Attributes)
	if l.DroppedAttributesCount > 0 {
		enc.WriteTagVarint(fieldLinkDroppedAttributesCount, uint64(l.DroppedAttributesCount))
	}
}

func marshalAttributes(enc *protobuf.Encoder, fieldNum uint32, attrs Attributes) {
	for _, kv := range attrs.Items() {
		enc.EncodeMessage(fieldNum, func(e *protobuf.Encoder) {
			e.WriteTagString(fieldKVKey, kv.Key)
			e.EncodeMessage(fieldKVValue, func(ve *protobuf.Encoder) {
				marshalAnyValue(ve, kv.Value)
			})
		})
	}
}

func marshalAnyValue(enc *protobuf.Encoder, v AnyValue) {
	switch v.Type {
	case AnyValueTypeString:
		enc.WriteTagString(fieldAnyValueString, v.Str)
	case AnyValueTypeBool:
		enc.WriteTagBool(fieldAnyValueBool, v.BoolVal)
	case AnyValueTypeInt:
		enc.WriteTagSignedVarint(fieldAnyValueInt, v.IntVal)
	case AnyValueTypeDouble:
		enc.WriteTagDouble(fieldAnyValueDouble, v.DoubleVal)
	case AnyValueTypeBytes:
		enc.WriteTagBytes(fieldAnyValueBytes, v.BytesVal)
	case AnyValueTypeArray:
		enc.EncodeMessage(fieldAnyValueArray, func(e *protobuf.Encoder) {
			for i := range v.ArrayVal {
				e.EncodeMessage(fieldArrayValues, func(ae *protobuf.Encoder) {
					marshalAnyValue(ae, v.ArrayVal[i])
				})
			}
		})
	case AnyValueTypeKvList:
		enc.EncodeMessage(fieldAnyValueKvList, func(e *protobuf.Encoder) {
			for i := range v.KvListVal {
				e.EncodeMessage(fieldKvListValues, func(ke *protobuf.Encoder) {
					ke.WriteTagString(fieldKVKey, v.KvListVal[i].Key)
					ke.EncodeMessage(fieldKVValue, func(ve *protobuf.Encoder) {
						marshalAnyValue(ve, v.KvListVal[i].Value)
					})
				})
			}
		})
	}
}
