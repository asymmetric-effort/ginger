package otlp

import (
	"fmt"

	"github.com/asymmetric-effort/ginger/internal/codec/protobuf"
)

// Unmarshal decodes protobuf bytes into TracesData.
func Unmarshal(data []byte) (TracesData, error) {
	var td TracesData
	dec := protobuf.NewDecoder(data)
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return td, fmt.Errorf("TracesData: %w", err)
		}
		switch fn {
		case fieldTracesDataResourceSpans:
			sub, err := dec.ReadMessage()
			if err != nil {
				return td, err
			}
			rs, err := unmarshalResourceSpans(sub)
			if err != nil {
				return td, err
			}
			td.ResourceSpans = append(td.ResourceSpans, rs)
		default:
			if err := dec.SkipField(wt); err != nil {
				return td, err
			}
		}
	}
	return td, nil
}

func unmarshalResourceSpans(dec *protobuf.Decoder) (ResourceSpans, error) {
	var rs ResourceSpans
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return rs, err
		}
		switch fn {
		case fieldResourceSpansResource:
			sub, err := dec.ReadMessage()
			if err != nil {
				return rs, err
			}
			res, err := unmarshalResource(sub)
			if err != nil {
				return rs, err
			}
			rs.Resource = res
		case fieldResourceSpansScopeSpans:
			sub, err := dec.ReadMessage()
			if err != nil {
				return rs, err
			}
			ss, err := unmarshalScopeSpans(sub)
			if err != nil {
				return rs, err
			}
			rs.ScopeSpans = append(rs.ScopeSpans, ss)
		default:
			if err := dec.SkipField(wt); err != nil {
				return rs, err
			}
		}
	}
	return rs, nil
}

func unmarshalResource(dec *protobuf.Decoder) (Resource, error) {
	var r Resource
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return r, err
		}
		switch fn {
		case fieldResourceAttributes:
			sub, err := dec.ReadMessage()
			if err != nil {
				return r, err
			}
			kv, err := unmarshalKeyValue(sub)
			if err != nil {
				return r, err
			}
			r.Attributes.Set(kv.Key, kv.Value)
		default:
			if err := dec.SkipField(wt); err != nil {
				return r, err
			}
		}
	}
	return r, nil
}

func unmarshalScopeSpans(dec *protobuf.Decoder) (ScopeSpans, error) {
	var ss ScopeSpans
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return ss, err
		}
		switch fn {
		case fieldScopeSpansScope:
			sub, err := dec.ReadMessage()
			if err != nil {
				return ss, err
			}
			scope, err := unmarshalScope(sub)
			if err != nil {
				return ss, err
			}
			ss.Scope = scope
		case fieldScopeSpansSpans:
			sub, err := dec.ReadMessage()
			if err != nil {
				return ss, err
			}
			span, err := unmarshalSpan(sub)
			if err != nil {
				return ss, err
			}
			ss.Spans = append(ss.Spans, span)
		default:
			if err := dec.SkipField(wt); err != nil {
				return ss, err
			}
		}
	}
	return ss, nil
}

func unmarshalScope(dec *protobuf.Decoder) (InstrumentationScope, error) {
	var s InstrumentationScope
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return s, err
		}
		switch fn {
		case fieldScopeName:
			s.Name, err = dec.ReadString()
		case fieldScopeVersion:
			s.Version, err = dec.ReadString()
		case fieldScopeAttributes:
			sub, err := dec.ReadMessage()
			if err != nil {
				return s, err
			}
			kv, err := unmarshalKeyValue(sub)
			if err != nil {
				return s, err
			}
			s.Attributes.Set(kv.Key, kv.Value)
		default:
			err = dec.SkipField(wt)
		}
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

func unmarshalSpan(dec *protobuf.Decoder) (Span, error) {
	var s Span
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return s, err
		}
		switch fn {
		case fieldSpanTraceID:
			b, err := dec.ReadBytes()
			if err != nil {
				return s, err
			}
			copy(s.TraceID[:], b)
		case fieldSpanSpanID:
			b, err := dec.ReadBytes()
			if err != nil {
				return s, err
			}
			copy(s.SpanID[:], b)
		case fieldSpanTraceState:
			s.TraceState, err = dec.ReadString()
		case fieldSpanParentSpanID:
			b, err := dec.ReadBytes()
			if err != nil {
				return s, err
			}
			copy(s.ParentSpanID[:], b)
		case fieldSpanName:
			s.Name, err = dec.ReadString()
		case fieldSpanKind:
			v, err := dec.ReadVarint()
			if err != nil {
				return s, err
			}
			s.Kind = SpanKind(v)
		case fieldSpanStartTimeUnixNano:
			s.StartTimeUnixNano, err = dec.ReadFixed64()
		case fieldSpanEndTimeUnixNano:
			s.EndTimeUnixNano, err = dec.ReadFixed64()
		case fieldSpanAttributes:
			sub, err := dec.ReadMessage()
			if err != nil {
				return s, err
			}
			kv, err := unmarshalKeyValue(sub)
			if err != nil {
				return s, err
			}
			s.Attributes.Set(kv.Key, kv.Value)
		case fieldSpanEvents:
			sub, err := dec.ReadMessage()
			if err != nil {
				return s, err
			}
			ev, err := unmarshalSpanEvent(sub)
			if err != nil {
				return s, err
			}
			s.Events = append(s.Events, ev)
		case fieldSpanLinks:
			sub, err := dec.ReadMessage()
			if err != nil {
				return s, err
			}
			link, err := unmarshalSpanLink(sub)
			if err != nil {
				return s, err
			}
			s.Links = append(s.Links, link)
		case fieldSpanStatus:
			sub, err := dec.ReadMessage()
			if err != nil {
				return s, err
			}
			status, err := unmarshalStatus(sub)
			if err != nil {
				return s, err
			}
			s.Status = status
		case fieldSpanDroppedAttributesCount:
			v, err := dec.ReadVarint()
			if err != nil {
				return s, err
			}
			s.DroppedAttributesCount = uint32(v)
		case fieldSpanDroppedEventsCount:
			v, err := dec.ReadVarint()
			if err != nil {
				return s, err
			}
			s.DroppedEventsCount = uint32(v)
		case fieldSpanDroppedLinksCount:
			v, err := dec.ReadVarint()
			if err != nil {
				return s, err
			}
			s.DroppedLinksCount = uint32(v)
		default:
			err = dec.SkipField(wt)
		}
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

func unmarshalSpanEvent(dec *protobuf.Decoder) (SpanEvent, error) {
	var e SpanEvent
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return e, err
		}
		switch fn {
		case fieldEventTimeUnixNano:
			e.TimeUnixNano, err = dec.ReadFixed64()
		case fieldEventName:
			e.Name, err = dec.ReadString()
		case fieldEventAttributes:
			sub, err := dec.ReadMessage()
			if err != nil {
				return e, err
			}
			kv, err := unmarshalKeyValue(sub)
			if err != nil {
				return e, err
			}
			e.Attributes.Set(kv.Key, kv.Value)
		case fieldEventDroppedAttributesCount:
			v, err := dec.ReadVarint()
			if err != nil {
				return e, err
			}
			e.DroppedAttributesCount = uint32(v)
		default:
			err = dec.SkipField(wt)
		}
		if err != nil {
			return e, err
		}
	}
	return e, nil
}

func unmarshalSpanLink(dec *protobuf.Decoder) (SpanLink, error) {
	var l SpanLink
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return l, err
		}
		switch fn {
		case fieldLinkTraceID:
			b, err := dec.ReadBytes()
			if err != nil {
				return l, err
			}
			copy(l.TraceID[:], b)
		case fieldLinkSpanID:
			b, err := dec.ReadBytes()
			if err != nil {
				return l, err
			}
			copy(l.SpanID[:], b)
		case fieldLinkTraceState:
			l.TraceState, err = dec.ReadString()
		case fieldLinkAttributes:
			sub, err := dec.ReadMessage()
			if err != nil {
				return l, err
			}
			kv, err := unmarshalKeyValue(sub)
			if err != nil {
				return l, err
			}
			l.Attributes.Set(kv.Key, kv.Value)
		case fieldLinkDroppedAttributesCount:
			v, err := dec.ReadVarint()
			if err != nil {
				return l, err
			}
			l.DroppedAttributesCount = uint32(v)
		default:
			err = dec.SkipField(wt)
		}
		if err != nil {
			return l, err
		}
	}
	return l, nil
}

func unmarshalStatus(dec *protobuf.Decoder) (Status, error) {
	var s Status
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return s, err
		}
		switch fn {
		case fieldStatusMessage:
			s.Message, err = dec.ReadString()
		case fieldStatusCode:
			v, err := dec.ReadVarint()
			if err != nil {
				return s, err
			}
			s.Code = StatusCode(v)
		default:
			err = dec.SkipField(wt)
		}
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

func unmarshalKeyValue(dec *protobuf.Decoder) (KeyValue, error) {
	var kv KeyValue
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return kv, err
		}
		switch fn {
		case fieldKVKey:
			kv.Key, err = dec.ReadString()
		case fieldKVValue:
			sub, err := dec.ReadMessage()
			if err != nil {
				return kv, err
			}
			av, err := unmarshalAnyValue(sub)
			if err != nil {
				return kv, err
			}
			kv.Value = av
		default:
			err = dec.SkipField(wt)
		}
		if err != nil {
			return kv, err
		}
	}
	return kv, nil
}

func unmarshalAnyValue(dec *protobuf.Decoder) (AnyValue, error) {
	var v AnyValue
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return v, err
		}
		switch fn {
		case fieldAnyValueString:
			v.Type = AnyValueTypeString
			v.Str, err = dec.ReadString()
		case fieldAnyValueBool:
			v.Type = AnyValueTypeBool
			v.BoolVal, err = dec.ReadBool()
		case fieldAnyValueInt:
			v.Type = AnyValueTypeInt
			v.IntVal, err = dec.ReadSignedVarint()
		case fieldAnyValueDouble:
			v.Type = AnyValueTypeDouble
			v.DoubleVal, err = dec.ReadDouble()
		case fieldAnyValueBytes:
			v.Type = AnyValueTypeBytes
			v.BytesVal, err = dec.ReadBytes()
		case fieldAnyValueArray:
			v.Type = AnyValueTypeArray
			sub, err := dec.ReadMessage()
			if err != nil {
				return v, err
			}
			arr, err := unmarshalArrayValue(sub)
			if err != nil {
				return v, err
			}
			v.ArrayVal = arr
		case fieldAnyValueKvList:
			v.Type = AnyValueTypeKvList
			sub, err := dec.ReadMessage()
			if err != nil {
				return v, err
			}
			kvs, err := unmarshalKvListValue(sub)
			if err != nil {
				return v, err
			}
			v.KvListVal = kvs
		default:
			err = dec.SkipField(wt)
		}
		if err != nil {
			return v, err
		}
	}
	return v, nil
}

func unmarshalArrayValue(dec *protobuf.Decoder) ([]AnyValue, error) {
	var values []AnyValue
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return nil, err
		}
		switch fn {
		case fieldArrayValues:
			sub, err := dec.ReadMessage()
			if err != nil {
				return nil, err
			}
			av, err := unmarshalAnyValue(sub)
			if err != nil {
				return nil, err
			}
			values = append(values, av)
		default:
			if err := dec.SkipField(wt); err != nil {
				return nil, err
			}
		}
	}
	return values, nil
}

func unmarshalKvListValue(dec *protobuf.Decoder) ([]KeyValue, error) {
	var kvs []KeyValue
	for !dec.Done() {
		fn, wt, err := dec.ReadField()
		if err != nil {
			return nil, err
		}
		switch fn {
		case fieldKvListValues:
			sub, err := dec.ReadMessage()
			if err != nil {
				return nil, err
			}
			kv, err := unmarshalKeyValue(sub)
			if err != nil {
				return nil, err
			}
			kvs = append(kvs, kv)
		default:
			if err := dec.SkipField(wt); err != nil {
				return nil, err
			}
		}
	}
	return kvs, nil
}
