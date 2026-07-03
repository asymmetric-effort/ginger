package otlp

import (
	"encoding/json"
	"testing"
)

func TestSpanKindString(t *testing.T) {
	tests := []struct {
		k    SpanKind
		want string
	}{
		{SpanKindUnspecified, "UNSPECIFIED"},
		{SpanKindInternal, "INTERNAL"},
		{SpanKindServer, "SERVER"},
		{SpanKindClient, "CLIENT"},
		{SpanKindProducer, "PRODUCER"},
		{SpanKindConsumer, "CONSUMER"},
	}
	for _, tt := range tests {
		if got := tt.k.String(); got != tt.want {
			t.Errorf("SpanKind(%d).String() = %q, want %q", tt.k, got, tt.want)
		}
	}
}

func TestStatusCodeString(t *testing.T) {
	tests := []struct {
		c    StatusCode
		want string
	}{
		{StatusCodeUnset, "UNSET"},
		{StatusCodeOk, "OK"},
		{StatusCodeError, "ERROR"},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("StatusCode(%d).String() = %q, want %q", tt.c, got, tt.want)
		}
	}
}

func TestAnyValueConstructors(t *testing.T) {
	sv := StringValue("hello")
	if sv.Type != AnyValueTypeString || sv.Str != "hello" {
		t.Error("StringValue")
	}
	bv := BoolValue(true)
	if bv.Type != AnyValueTypeBool || !bv.BoolVal {
		t.Error("BoolValue")
	}
	iv := IntValue(42)
	if iv.Type != AnyValueTypeInt || iv.IntVal != 42 {
		t.Error("IntValue")
	}
	dv := DoubleValue(3.14)
	if dv.Type != AnyValueTypeDouble || dv.DoubleVal != 3.14 {
		t.Error("DoubleValue")
	}
	byv := BytesValue([]byte{0xDE, 0xAD})
	if byv.Type != AnyValueTypeBytes || len(byv.BytesVal) != 2 {
		t.Error("BytesValue")
	}
	av := ArrayValue([]AnyValue{StringValue("a"), IntValue(1)})
	if av.Type != AnyValueTypeArray || len(av.ArrayVal) != 2 {
		t.Error("ArrayValue")
	}
	kvv := KvListValue([]KeyValue{{Key: "k", Value: StringValue("v")}})
	if kvv.Type != AnyValueTypeKvList || len(kvv.KvListVal) != 1 {
		t.Error("KvListValue")
	}
}

func TestAttributes(t *testing.T) {
	attrs := NewAttributes()
	if attrs.Len() != 0 {
		t.Error("new attrs should be empty")
	}

	attrs.Set("key1", StringValue("val1"))
	attrs.Set("key2", IntValue(42))
	if attrs.Len() != 2 {
		t.Errorf("Len() = %d", attrs.Len())
	}

	v, ok := attrs.Get("key1")
	if !ok || v.Str != "val1" {
		t.Error("Get key1")
	}

	_, ok = attrs.Get("missing")
	if ok {
		t.Error("Get missing should return false")
	}

	// Update existing
	attrs.Set("key1", StringValue("updated"))
	if attrs.Len() != 2 {
		t.Error("update should not add")
	}
	v, _ = attrs.Get("key1")
	if v.Str != "updated" {
		t.Error("update value")
	}

	items := attrs.Items()
	if len(items) != 2 {
		t.Error("Items")
	}
}

func TestNewAttributesFromSlice(t *testing.T) {
	kvs := []KeyValue{
		{Key: "a", Value: StringValue("1")},
		{Key: "b", Value: IntValue(2)},
	}
	attrs := NewAttributesFromSlice(kvs)
	if attrs.Len() != 2 {
		t.Errorf("Len() = %d", attrs.Len())
	}
	// Verify it's a copy
	kvs[0].Key = "modified"
	v, ok := attrs.Get("a")
	if !ok || v.Str != "1" {
		t.Error("should be a copy")
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	td := makeTestTracesData()

	data, err := Marshal(td)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty marshal output")
	}

	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Verify structure
	if len(td2.ResourceSpans) != 1 {
		t.Fatalf("ResourceSpans: %d", len(td2.ResourceSpans))
	}
	rs := td2.ResourceSpans[0]
	v, ok := rs.Resource.Attributes.Get("service.name")
	if !ok || v.Str != "test-service" {
		t.Error("resource attribute")
	}

	if len(rs.ScopeSpans) != 1 {
		t.Fatalf("ScopeSpans: %d", len(rs.ScopeSpans))
	}
	ss := rs.ScopeSpans[0]
	if ss.Scope.Name != "ginger" {
		t.Errorf("scope name = %q", ss.Scope.Name)
	}
	if ss.Scope.Version != "0.0.1" {
		t.Errorf("scope version = %q", ss.Scope.Version)
	}

	if len(ss.Spans) != 1 {
		t.Fatalf("Spans: %d", len(ss.Spans))
	}
	span := ss.Spans[0]
	if span.Name != "test-span" {
		t.Errorf("span name = %q", span.Name)
	}
	if span.Kind != SpanKindServer {
		t.Errorf("span kind = %v", span.Kind)
	}
	if span.TraceID != [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16} {
		t.Error("trace ID mismatch")
	}
	if span.SpanID != [8]byte{1, 2, 3, 4, 5, 6, 7, 8} {
		t.Error("span ID mismatch")
	}
	if span.ParentSpanID != [8]byte{8, 7, 6, 5, 4, 3, 2, 1} {
		t.Error("parent span ID mismatch")
	}
	if span.TraceState != "vendor=val" {
		t.Errorf("trace state = %q", span.TraceState)
	}
	if span.StartTimeUnixNano != 1000000000 {
		t.Error("start time")
	}
	if span.EndTimeUnixNano != 2000000000 {
		t.Error("end time")
	}
	if span.Status.Code != StatusCodeOk {
		t.Error("status code")
	}
	if span.Status.Message != "success" {
		t.Error("status message")
	}

	// Attributes
	av, ok := span.Attributes.Get("http.method")
	if !ok || av.Str != "GET" {
		t.Error("span attribute")
	}

	// Events
	if len(span.Events) != 1 {
		t.Fatalf("events: %d", len(span.Events))
	}
	if span.Events[0].Name != "request.start" {
		t.Error("event name")
	}

	// Links
	if len(span.Links) != 1 {
		t.Fatalf("links: %d", len(span.Links))
	}
	if span.Links[0].TraceState != "link=state" {
		t.Error("link trace state")
	}
}

func TestMarshalUnmarshalEmptyTracesData(t *testing.T) {
	td := TracesData{}
	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(td2.ResourceSpans) != 0 {
		t.Error("empty should stay empty")
	}
}

func TestMarshalUnmarshalAllValueTypes(t *testing.T) {
	attrs := NewAttributes()
	attrs.Set("str", StringValue("hello"))
	attrs.Set("bool", BoolValue(true))
	attrs.Set("int", IntValue(-42))
	attrs.Set("double", DoubleValue(3.14))
	attrs.Set("bytes", BytesValue([]byte{0x01, 0x02}))
	attrs.Set("array", ArrayValue([]AnyValue{StringValue("a"), IntValue(1)}))
	attrs.Set("kvlist", KvListValue([]KeyValue{{Key: "nested", Value: BoolValue(false)}}))

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID:           [16]byte{1},
					SpanID:            [8]byte{1},
					Name:              "test",
					StartTimeUnixNano: 1,
					EndTimeUnixNano:   2,
					Attributes:        attrs,
				}},
			}},
		}},
	}

	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	span := td2.ResourceSpans[0].ScopeSpans[0].Spans[0]

	checkStr, _ := span.Attributes.Get("str")
	if checkStr.Str != "hello" {
		t.Error("string attr")
	}
	checkBool, _ := span.Attributes.Get("bool")
	if !checkBool.BoolVal {
		t.Error("bool attr")
	}
	checkInt, _ := span.Attributes.Get("int")
	if checkInt.IntVal != -42 {
		t.Error("int attr")
	}
	checkDouble, _ := span.Attributes.Get("double")
	if checkDouble.DoubleVal != 3.14 {
		t.Error("double attr")
	}
	checkBytes, _ := span.Attributes.Get("bytes")
	if len(checkBytes.BytesVal) != 2 {
		t.Error("bytes attr")
	}
	checkArray, _ := span.Attributes.Get("array")
	if len(checkArray.ArrayVal) != 2 {
		t.Error("array attr")
	}
	checkKvList, _ := span.Attributes.Get("kvlist")
	if len(checkKvList.KvListVal) != 1 {
		t.Error("kvlist attr")
	}
}

func TestMarshalDroppedCounts(t *testing.T) {
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID:                [16]byte{1},
					SpanID:                 [8]byte{1},
					Name:                   "test",
					StartTimeUnixNano:      1,
					EndTimeUnixNano:        2,
					DroppedAttributesCount: 5,
					DroppedEventsCount:     3,
					DroppedLinksCount:      1,
					Events: []SpanEvent{{
						Name:                   "ev",
						TimeUnixNano:           100,
						DroppedAttributesCount: 2,
					}},
					Links: []SpanLink{{
						TraceID:                [16]byte{2},
						SpanID:                 [8]byte{2},
						DroppedAttributesCount: 4,
					}},
				}},
			}},
		}},
	}

	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	span := td2.ResourceSpans[0].ScopeSpans[0].Spans[0]
	if span.DroppedAttributesCount != 5 {
		t.Errorf("DroppedAttributesCount = %d", span.DroppedAttributesCount)
	}
	if span.DroppedEventsCount != 3 {
		t.Errorf("DroppedEventsCount = %d", span.DroppedEventsCount)
	}
	if span.DroppedLinksCount != 1 {
		t.Errorf("DroppedLinksCount = %d", span.DroppedLinksCount)
	}
	if span.Events[0].DroppedAttributesCount != 2 {
		t.Error("event dropped count")
	}
	if span.Links[0].DroppedAttributesCount != 4 {
		t.Error("link dropped count")
	}
}

func TestAnyValueMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		val  AnyValue
		want string
	}{
		{"string", StringValue("hi"), `{"stringValue":"hi"}`},
		{"bool", BoolValue(true), `{"boolValue":true}`},
		{"int", IntValue(42), `{"intValue":"42"}`},
		{"double", DoubleValue(1.5), `{"doubleValue":1.5}`},
		{"bytes", BytesValue([]byte{0x01}), `{"bytesValue":"AQ=="}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.val)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Errorf("JSON = %s, want %s", data, tt.want)
			}
		})
	}
}

func TestAnyValueMarshalJSONArray(t *testing.T) {
	v := ArrayValue([]AnyValue{StringValue("a"), IntValue(1)})
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("empty JSON")
	}
}

func TestAnyValueMarshalJSONKvList(t *testing.T) {
	v := KvListValue([]KeyValue{{Key: "k", Value: StringValue("v")}})
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("empty JSON")
	}
}

func TestAnyValueMarshalJSONDefault(t *testing.T) {
	v := AnyValue{Type: AnyValueType(99)}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "null" {
		t.Errorf("unknown type JSON = %s", data)
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		v    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{42, "42"},
		{-9223372036854775808, "-9223372036854775808"},
		{9223372036854775807, "9223372036854775807"},
	}
	for _, tt := range tests {
		if got := formatInt64(tt.v); got != tt.want {
			t.Errorf("formatInt64(%d) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestUnmarshalInvalidData(t *testing.T) {
	_, err := Unmarshal([]byte{0xff, 0xff})
	if err == nil {
		t.Error("should error on garbage data")
	}
}

func TestMarshalNoResourceAttributes(t *testing.T) {
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID:           [16]byte{1},
					SpanID:            [8]byte{1},
					Name:              "test",
					StartTimeUnixNano: 1,
					EndTimeUnixNano:   2,
				}},
			}},
		}},
	}
	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if td2.ResourceSpans[0].Resource.Attributes.Len() != 0 {
		t.Error("should have no resource attributes")
	}
}

func TestMarshalNoScope(t *testing.T) {
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID:           [16]byte{1},
					SpanID:            [8]byte{1},
					Name:              "test",
					StartTimeUnixNano: 1,
					EndTimeUnixNano:   2,
				}},
			}},
		}},
	}
	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if td2.ResourceSpans[0].ScopeSpans[0].Scope.Name != "" {
		t.Error("should have empty scope")
	}
}

func TestMarshalUnmarshalScopeAttributes(t *testing.T) {
	scopeAttrs := NewAttributes()
	scopeAttrs.Set("lib.key", StringValue("lib.val"))

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Scope: InstrumentationScope{
					Name:       "mylib",
					Version:    "1.0",
					Attributes: scopeAttrs,
				},
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: "test", StartTimeUnixNano: 1, EndTimeUnixNano: 2,
				}},
			}},
		}},
	}

	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	scope := td2.ResourceSpans[0].ScopeSpans[0].Scope
	v, ok := scope.Attributes.Get("lib.key")
	if !ok || v.Str != "lib.val" {
		t.Error("scope attribute not round-tripped")
	}
}

func TestUnmarshalWithUnknownFields(t *testing.T) {
	// Marshal a valid TracesData, then append an unknown field
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: "test", StartTimeUnixNano: 1, EndTimeUnixNano: 2,
				}},
			}},
		}},
	}
	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}

	// Append unknown varint field (field 99, wire type 0, value 42)
	augmented := make([]byte, 0, len(data)+3)
	augmented = append(augmented, data...)
	// field 99 << 3 | 0 = 792 = varint 0xF8 0x06
	augmented = append(augmented, 0xF8, 0x06, 42)

	td2, err := Unmarshal(augmented)
	if err != nil {
		t.Fatalf("should skip unknown fields: %v", err)
	}
	if len(td2.ResourceSpans) != 1 {
		t.Error("should still have 1 ResourceSpans")
	}
}

func TestMarshalStatusCodeOnly(t *testing.T) {
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: "test", StartTimeUnixNano: 1, EndTimeUnixNano: 2,
					Status: Status{Code: StatusCodeError},
				}},
			}},
		}},
	}
	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if td2.ResourceSpans[0].ScopeSpans[0].Spans[0].Status.Code != StatusCodeError {
		t.Error("status code not preserved")
	}
}

func TestMarshalStatusMessageOnly(t *testing.T) {
	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1},
					Name: "test", StartTimeUnixNano: 1, EndTimeUnixNano: 2,
					Status: Status{Message: "oops"},
				}},
			}},
		}},
	}
	data, err := Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if td2.ResourceSpans[0].ScopeSpans[0].Spans[0].Status.Message != "oops" {
		t.Error("status message not preserved")
	}
}

func TestBytesValueCopy(t *testing.T) {
	orig := []byte{1, 2, 3}
	v := BytesValue(orig)
	orig[0] = 99
	if v.BytesVal[0] != 1 {
		t.Error("BytesValue should copy input")
	}
}

func makeTestTracesData() TracesData {
	resAttrs := NewAttributes()
	resAttrs.Set("service.name", StringValue("test-service"))

	spanAttrs := NewAttributes()
	spanAttrs.Set("http.method", StringValue("GET"))
	spanAttrs.Set("http.status_code", IntValue(200))

	eventAttrs := NewAttributes()
	eventAttrs.Set("event.key", StringValue("event-val"))

	linkAttrs := NewAttributes()
	linkAttrs.Set("link.key", BoolValue(true))

	return TracesData{
		ResourceSpans: []ResourceSpans{{
			Resource: Resource{Attributes: resAttrs},
			ScopeSpans: []ScopeSpans{{
				Scope: InstrumentationScope{Name: "ginger", Version: "0.0.1"},
				Spans: []Span{{
					TraceID:           [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
					SpanID:            [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
					ParentSpanID:      [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
					TraceState:        "vendor=val",
					Name:              "test-span",
					Kind:              SpanKindServer,
					StartTimeUnixNano: 1000000000,
					EndTimeUnixNano:   2000000000,
					Attributes:        spanAttrs,
					Events: []SpanEvent{{
						TimeUnixNano: 1500000000,
						Name:         "request.start",
						Attributes:   eventAttrs,
					}},
					Links: []SpanLink{{
						TraceID:    [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						SpanID:     [8]byte{3, 3, 3, 3, 3, 3, 3, 3},
						TraceState: "link=state",
						Attributes: linkAttrs,
					}},
					Status: Status{Code: StatusCodeOk, Message: "success"},
				}},
			}},
		}},
	}
}
