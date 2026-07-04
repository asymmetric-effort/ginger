package otlp

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestUnmarshalUnknownFieldsInSubMessages tests that unknown fields inside nested
// messages are silently skipped by all unmarshal* functions.
func TestUnmarshalUnknownFieldsInSubMessages(t *testing.T) {
	// We build raw protobuf bytes manually so we can inject unknown fields
	// at every nested level.
	//
	// Field numbers for TracesData:
	//   1 = ResourceSpans (WireBytes)
	// Field numbers for ResourceSpans:
	//   1 = Resource (WireBytes)
	//   2 = ScopeSpans (WireBytes)
	// Field numbers for Resource:
	//   1 = Attributes (WireBytes = KeyValue)
	// Field numbers for ScopeSpans:
	//   1 = Scope (WireBytes)
	//   2 = Spans (WireBytes)
	// etc.

	// Unknown varint tag (field 99, wire type 0, value 42): 0xF8 0x06 0x2A
	unknownField := []byte{0xF8, 0x06, 0x2A}

	// Build a KeyValue message: field1(string "k") + field2(AnyValue{field1="v"})
	// then append an unknown field.
	// AnyValue: field 1 (string) = "v" → tag 0x0A, len 1, 'v'
	anyValueMsg := []byte{0x0A, 0x01, 'v'}
	anyValueMsg = append(anyValueMsg, unknownField...) // unknown in AnyValue

	// KeyValue: field1="k", field2=anyValueMsg
	kvMsg := buildLenDelim(0x0A, []byte("k"))                  // field 1, string "k"
	kvMsg = append(kvMsg, buildLenDelim(0x12, anyValueMsg)...) // field 2, embedded AnyValue
	kvMsg = append(kvMsg, unknownField...)                     // unknown in KeyValue

	// Resource: field1=kvMsg
	resourceMsg := buildLenDelim(0x0A, kvMsg)
	resourceMsg = append(resourceMsg, unknownField...) // unknown in Resource

	// Span message with unknown field
	traceID := make([]byte, 16)
	traceID[0] = 1
	spanID := make([]byte, 8)
	spanID[0] = 1

	spanMsg := buildLenDelim(0x0A, traceID)                                // field 1 = TraceID
	spanMsg = append(spanMsg, buildLenDelim(0x12, spanID)...)              // field 2 = SpanID
	spanMsg = append(spanMsg, buildLenDelim(0x2A, []byte("test-span"))...) // field 5 = Name
	// StartTimeUnixNano (field 7, fixed64 = 0x39): value 1000
	spanMsg = append(spanMsg, encodeFixed64Tag(0x39, 1000)...)
	// EndTimeUnixNano (field 8, fixed64 = 0x41): value 2000
	spanMsg = append(spanMsg, encodeFixed64Tag(0x41, 2000)...)
	spanMsg = append(spanMsg, unknownField...) // unknown in Span

	// SpanEvent with unknown field
	eventMsg := encodeFixed64Tag(0x09, 1500) // field 1, fixed64
	eventMsg = append(eventMsg, buildLenDelim(0x12, []byte("ev"))...)
	eventMsg = append(eventMsg, unknownField...)
	spanMsg = append(spanMsg, buildLenDelim(0x5A, eventMsg)...) // field 11 = Events

	// SpanLink with unknown field
	linkTraceID := make([]byte, 16)
	linkTraceID[0] = 2
	linkSpanID := make([]byte, 8)
	linkSpanID[0] = 2
	linkMsg := buildLenDelim(0x0A, linkTraceID)
	linkMsg = append(linkMsg, buildLenDelim(0x12, linkSpanID)...)
	linkMsg = append(linkMsg, unknownField...)
	spanMsg = append(spanMsg, buildLenDelim(0x62, linkMsg)...) // field 12 = Links

	// Status with unknown field
	statusMsg := buildLenDelim(0x12, []byte("ok")) // field 2 = message
	statusMsg = append(statusMsg, unknownField...)
	spanMsg = append(spanMsg, buildLenDelim(0x6A, statusMsg)...) // field 13 = Status

	// InstrumentationScope with unknown field
	scopeMsg := buildLenDelim(0x0A, []byte("mylib"))                   // field 1 = name
	scopeMsg = append(scopeMsg, buildLenDelim(0x12, []byte("1.0"))...) // field 2 = version
	scopeMsg = append(scopeMsg, unknownField...)

	// ScopeSpans: field1=scope, field2=span
	scopeSpansMsg := buildLenDelim(0x0A, scopeMsg)
	scopeSpansMsg = append(scopeSpansMsg, buildLenDelim(0x12, spanMsg)...)
	scopeSpansMsg = append(scopeSpansMsg, unknownField...) // unknown in ScopeSpans

	// ResourceSpans: field1=resource, field2=scopeSpans
	rsMsg := buildLenDelim(0x0A, resourceMsg)
	rsMsg = append(rsMsg, buildLenDelim(0x12, scopeSpansMsg)...)
	rsMsg = append(rsMsg, unknownField...) // unknown in ResourceSpans

	// TracesData: field1=resourceSpans
	tdMsg := buildLenDelim(0x0A, rsMsg)
	tdMsg = append(tdMsg, unknownField...) // unknown in TracesData

	td, err := Unmarshal(tdMsg)
	if err != nil {
		t.Fatalf("Unmarshal with unknown fields: %v", err)
	}
	if len(td.ResourceSpans) != 1 {
		t.Fatalf("expected 1 ResourceSpan, got %d", len(td.ResourceSpans))
	}
	rs := td.ResourceSpans[0]
	if v, ok := rs.Resource.Attributes.Get("k"); !ok || v.Str != "v" {
		t.Error("resource attribute not parsed")
	}
	if len(rs.ScopeSpans) != 1 {
		t.Fatalf("expected 1 ScopeSpans, got %d", len(rs.ScopeSpans))
	}
	ss := rs.ScopeSpans[0]
	if ss.Scope.Name != "mylib" {
		t.Errorf("scope name = %q", ss.Scope.Name)
	}
	if len(ss.Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ss.Spans))
	}
	if ss.Spans[0].Name != "test-span" {
		t.Errorf("span name = %q", ss.Spans[0].Name)
	}
	if len(ss.Spans[0].Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(ss.Spans[0].Events))
	}
	if len(ss.Spans[0].Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(ss.Spans[0].Links))
	}
}

// TestUnmarshalUnknownFieldsInArrayValue tests unknown fields in ArrayValue and KvListValue messages.
func TestUnmarshalUnknownFieldsInArrayValue(t *testing.T) {
	// Build TracesData with an attribute that is an array value with unknown fields
	// AnyValue with type array: field 5 (WireBytes) → ArrayValue
	// ArrayValue: field 1 = AnyValue items
	unknownField := []byte{0xF8, 0x06, 0x2A}

	// inner AnyValue item (string "x")
	innerAV := []byte{0x0A, 0x01, 'x'}

	// ArrayValue message: field 1 = innerAV, then unknown
	arrayValMsg := buildLenDelim(0x0A, innerAV)
	arrayValMsg = append(arrayValMsg, unknownField...) // unknown in ArrayValue

	// AnyValue: field 5 = arrayValMsg
	anyValueMsg := buildLenDelim(0x2A, arrayValMsg)
	anyValueMsg = append(anyValueMsg, unknownField...) // unknown in AnyValue (this hits the top-level default)

	// KeyValue: field 1 = "arr", field 2 = anyValueMsg
	kvMsg := buildLenDelim(0x0A, []byte("arr"))
	kvMsg = append(kvMsg, buildLenDelim(0x12, anyValueMsg)...)

	// KvListValue: field 1 = KvList with KV entries, then unknown
	// Build a KvListValue AnyValue (field 6)
	kvListInnerKV := buildLenDelim(0x0A, []byte("k2"))
	kvListInnerKV = append(kvListInnerKV, buildLenDelim(0x12, []byte{0x0A, 0x01, 'z'})...)

	kvListMsg := buildLenDelim(0x0A, kvListInnerKV)
	kvListMsg = append(kvListMsg, unknownField...) // unknown in KvListValue

	anyValueKvList := buildLenDelim(0x32, kvListMsg) // field 6 = KvList

	kvMsg2 := buildLenDelim(0x0A, []byte("kvlist"))
	kvMsg2 = append(kvMsg2, buildLenDelim(0x12, anyValueKvList)...)

	// Span with 2 attributes: the array one and the kvlist one
	traceID := make([]byte, 16)
	traceID[0] = 5
	spanID := make([]byte, 8)
	spanID[0] = 5

	spanMsg := buildLenDelim(0x0A, traceID)
	spanMsg = append(spanMsg, buildLenDelim(0x12, spanID)...)
	spanMsg = append(spanMsg, buildLenDelim(0x2A, []byte("test"))...)
	spanMsg = append(spanMsg, encodeFixed64Tag(0x39, 1)...)
	spanMsg = append(spanMsg, encodeFixed64Tag(0x41, 2)...)
	spanMsg = append(spanMsg, buildLenDelim(0x4A, kvMsg)...)  // field 9 = attribute (arr)
	spanMsg = append(spanMsg, buildLenDelim(0x4A, kvMsg2)...) // field 9 = attribute (kvlist)

	scopeSpansMsg := buildLenDelim(0x12, spanMsg) // field 2 = span
	rsMsg := buildLenDelim(0x12, scopeSpansMsg)   // field 2 = scopeSpans
	tdMsg := buildLenDelim(0x0A, rsMsg)           // field 1 = resourceSpans

	td, err := Unmarshal(tdMsg)
	if err != nil {
		t.Fatalf("Unmarshal array/kvlist with unknown fields: %v", err)
	}
	if len(td.ResourceSpans) != 1 {
		t.Fatal("expected 1 resource span")
	}
	span := td.ResourceSpans[0].ScopeSpans[0].Spans[0]
	v, ok := span.Attributes.Get("arr")
	if !ok {
		t.Error("arr attribute not found")
	} else if v.Type != AnyValueTypeArray || len(v.ArrayVal) != 1 {
		t.Errorf("array attr type=%v len=%d", v.Type, len(v.ArrayVal))
	}
	v2, ok2 := span.Attributes.Get("kvlist")
	if !ok2 {
		t.Error("kvlist attribute not found")
	} else if v2.Type != AnyValueTypeKvList || len(v2.KvListVal) != 1 {
		t.Errorf("kvlist attr type=%v len=%d", v2.Type, len(v2.KvListVal))
	}
}

// buildLenDelim builds a length-delimited field: tag byte(s) + varint(len) + payload.
// tag should be a pre-encoded tag byte (e.g. 0x0A for field 1, wire type 2).
func buildLenDelim(tag byte, payload []byte) []byte {
	result := []byte{tag}
	result = appendVarint(result, uint64(len(payload)))
	return append(result, payload...)
}

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func encodeFixed64Tag(tag byte, v uint64) []byte {
	return []byte{
		tag,
		byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24),
		byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56),
	}
}

// TestUnmarshalSkipFieldErrors tests that SkipField errors propagate correctly
// for each nested unmarshal function by injecting invalid wire type bytes.
func TestUnmarshalSkipFieldErrors(t *testing.T) {
	// Each test crafts a valid outer envelope but with an invalid-wire-type field
	// inside a sub-message, so the relevant unmarshal* function returns an error.

	// Invalid wire type field: field 99, wire type 3 (invalid)
	// Tag = 99<<3 | 3 = 795 → varint: 0xFB 0x06
	invalidWireTag := []byte{0xFB, 0x06}

	// --- Test unmarshalResourceSpans SkipField error ---
	// ResourceSpans message with invalid wire type at end
	rsInvalid := buildLenDelim(0x12, []byte{}) // empty ScopeSpans (field 2)
	rsInvalid = append(rsInvalid, invalidWireTag...)
	tdInvalid1 := buildLenDelim(0x0A, rsInvalid)
	if _, err := Unmarshal(tdInvalid1); err == nil {
		t.Error("unmarshalResourceSpans: expected error from invalid wire type")
	}

	// --- Test unmarshalResource SkipField error ---
	// Resource message with invalid wire type
	resourceInvalid := invalidWireTag
	rsWithBadResource := buildLenDelim(0x0A, resourceInvalid) // field 1 = Resource
	tdInvalid2 := buildLenDelim(0x0A, rsWithBadResource)
	if _, err := Unmarshal(tdInvalid2); err == nil {
		t.Error("unmarshalResource: expected error from invalid wire type")
	}

	// --- Test unmarshalScopeSpans SkipField error ---
	// ScopeSpans message with invalid wire type
	ssInvalid := invalidWireTag
	rsWithBadSS := buildLenDelim(0x12, ssInvalid) // field 2 = ScopeSpans
	tdInvalid3 := buildLenDelim(0x0A, rsWithBadSS)
	if _, err := Unmarshal(tdInvalid3); err == nil {
		t.Error("unmarshalScopeSpans: expected error from invalid wire type")
	}

	// --- Test unmarshalScope SkipField error ---
	// Scope/InstrumentationScope message with invalid wire type
	scopeInvalid := invalidWireTag
	ssWithBadScope := buildLenDelim(0x0A, scopeInvalid) // field 1 = Scope
	rsWithBadScope := buildLenDelim(0x12, ssWithBadScope)
	tdInvalid4 := buildLenDelim(0x0A, rsWithBadScope)
	if _, err := Unmarshal(tdInvalid4); err == nil {
		t.Error("unmarshalScope: expected error from invalid wire type")
	}

	// --- Test unmarshalSpan SkipField error ---
	traceID := make([]byte, 16)
	traceID[0] = 1
	spanID := make([]byte, 8)
	spanID[0] = 1
	spanInvalid := buildLenDelim(0x0A, traceID)
	spanInvalid = append(spanInvalid, buildLenDelim(0x12, spanID)...)
	spanInvalid = append(spanInvalid, buildLenDelim(0x2A, []byte("s"))...)
	spanInvalid = append(spanInvalid, encodeFixed64Tag(0x39, 1)...)
	spanInvalid = append(spanInvalid, encodeFixed64Tag(0x41, 2)...)
	spanInvalid = append(spanInvalid, invalidWireTag...)
	ssWithBadSpan := buildLenDelim(0x12, spanInvalid) // field 2 = Span
	rsForSpan := buildLenDelim(0x12, ssWithBadSpan)
	tdInvalid5 := buildLenDelim(0x0A, rsForSpan)
	if _, err := Unmarshal(tdInvalid5); err == nil {
		t.Error("unmarshalSpan: expected error from invalid wire type")
	}

	// --- Test unmarshalSpanEvent SkipField error ---
	eventInvalid := encodeFixed64Tag(0x09, 100) // valid TimeUnixNano
	eventInvalid = append(eventInvalid, buildLenDelim(0x12, []byte("ev"))...)
	eventInvalid = append(eventInvalid, invalidWireTag...)
	spanWithBadEvent := buildLenDelim(0x0A, traceID)
	spanWithBadEvent = append(spanWithBadEvent, buildLenDelim(0x12, spanID)...)
	spanWithBadEvent = append(spanWithBadEvent, buildLenDelim(0x2A, []byte("s"))...)
	spanWithBadEvent = append(spanWithBadEvent, encodeFixed64Tag(0x39, 1)...)
	spanWithBadEvent = append(spanWithBadEvent, encodeFixed64Tag(0x41, 2)...)
	spanWithBadEvent = append(spanWithBadEvent, buildLenDelim(0x5A, eventInvalid)...) // field 11 = Events
	ssWithBadEvent := buildLenDelim(0x12, spanWithBadEvent)
	rsForEvent := buildLenDelim(0x12, ssWithBadEvent)
	tdInvalid6 := buildLenDelim(0x0A, rsForEvent)
	if _, err := Unmarshal(tdInvalid6); err == nil {
		t.Error("unmarshalSpanEvent: expected error from invalid wire type")
	}

	// --- Test unmarshalSpanLink SkipField error ---
	linkTraceID := make([]byte, 16)
	linkTraceID[0] = 2
	linkSpanID := make([]byte, 8)
	linkSpanID[0] = 2
	linkInvalid := buildLenDelim(0x0A, linkTraceID)
	linkInvalid = append(linkInvalid, buildLenDelim(0x12, linkSpanID)...)
	linkInvalid = append(linkInvalid, invalidWireTag...)
	spanWithBadLink := buildLenDelim(0x0A, traceID)
	spanWithBadLink = append(spanWithBadLink, buildLenDelim(0x12, spanID)...)
	spanWithBadLink = append(spanWithBadLink, buildLenDelim(0x2A, []byte("s"))...)
	spanWithBadLink = append(spanWithBadLink, encodeFixed64Tag(0x39, 1)...)
	spanWithBadLink = append(spanWithBadLink, encodeFixed64Tag(0x41, 2)...)
	spanWithBadLink = append(spanWithBadLink, buildLenDelim(0x62, linkInvalid)...) // field 12 = Links
	ssWithBadLink := buildLenDelim(0x12, spanWithBadLink)
	rsForLink := buildLenDelim(0x12, ssWithBadLink)
	tdInvalid7 := buildLenDelim(0x0A, rsForLink)
	if _, err := Unmarshal(tdInvalid7); err == nil {
		t.Error("unmarshalSpanLink: expected error from invalid wire type")
	}

	// --- Test unmarshalStatus SkipField error ---
	statusInvalid := buildLenDelim(0x12, []byte("ok")) // valid message field
	statusInvalid = append(statusInvalid, invalidWireTag...)
	spanWithBadStatus := buildLenDelim(0x0A, traceID)
	spanWithBadStatus = append(spanWithBadStatus, buildLenDelim(0x12, spanID)...)
	spanWithBadStatus = append(spanWithBadStatus, buildLenDelim(0x2A, []byte("s"))...)
	spanWithBadStatus = append(spanWithBadStatus, encodeFixed64Tag(0x39, 1)...)
	spanWithBadStatus = append(spanWithBadStatus, encodeFixed64Tag(0x41, 2)...)
	spanWithBadStatus = append(spanWithBadStatus, buildLenDelim(0x6A, statusInvalid)...) // field 13 = Status
	ssWithBadStatus := buildLenDelim(0x12, spanWithBadStatus)
	rsForStatus := buildLenDelim(0x12, ssWithBadStatus)
	tdInvalid8 := buildLenDelim(0x0A, rsForStatus)
	if _, err := Unmarshal(tdInvalid8); err == nil {
		t.Error("unmarshalStatus: expected error from invalid wire type")
	}

	// --- Test unmarshalKeyValue SkipField error ---
	kvInvalid := buildLenDelim(0x0A, []byte("k")) // valid key
	kvInvalid = append(kvInvalid, invalidWireTag...)
	resourceWithBadKV := buildLenDelim(0x0A, kvInvalid) // field 1 = Attribute (KeyValue)
	rsForKV := buildLenDelim(0x0A, resourceWithBadKV)
	tdInvalid9 := buildLenDelim(0x0A, rsForKV)
	if _, err := Unmarshal(tdInvalid9); err == nil {
		t.Error("unmarshalKeyValue: expected error from invalid wire type")
	}

	// --- Test unmarshalAnyValue SkipField error ---
	anyValInvalid := buildLenDelim(0x0A, []byte("str")) // field 1 = string value
	anyValInvalid = append(anyValInvalid, invalidWireTag...)
	kvWithBadAnyVal := buildLenDelim(0x0A, []byte("k"))
	kvWithBadAnyVal = append(kvWithBadAnyVal, buildLenDelim(0x12, anyValInvalid)...) // field 2 = AnyValue
	resourceWithBadAnyVal := buildLenDelim(0x0A, kvWithBadAnyVal)
	rsForAnyVal := buildLenDelim(0x0A, resourceWithBadAnyVal)
	tdInvalid10 := buildLenDelim(0x0A, rsForAnyVal)
	if _, err := Unmarshal(tdInvalid10); err == nil {
		t.Error("unmarshalAnyValue: expected error from invalid wire type")
	}

	// --- Test unmarshalArrayValue SkipField error ---
	arrayValInvalid := buildLenDelim(0x0A, []byte{0x0A, 0x01, 'x'}) // valid AV
	arrayValInvalid = append(arrayValInvalid, invalidWireTag...)
	anyValArray := buildLenDelim(0x2A, arrayValInvalid) // field 5 = array
	kvWithArray := buildLenDelim(0x0A, []byte("k"))
	kvWithArray = append(kvWithArray, buildLenDelim(0x12, anyValArray)...)
	resourceForArray := buildLenDelim(0x0A, kvWithArray)
	rsForArray := buildLenDelim(0x0A, resourceForArray)
	tdInvalidArray := buildLenDelim(0x0A, rsForArray)
	if _, err := Unmarshal(tdInvalidArray); err == nil {
		t.Error("unmarshalArrayValue: expected error from invalid wire type")
	}

	// --- Test unmarshalKvListValue SkipField error ---
	kvListInnerKV := buildLenDelim(0x0A, []byte("k"))
	kvListInnerKV = append(kvListInnerKV, buildLenDelim(0x12, []byte{0x0A, 0x01, 'v'})...)
	kvListValInvalid := buildLenDelim(0x0A, kvListInnerKV) // valid kv
	kvListValInvalid = append(kvListValInvalid, invalidWireTag...)
	anyValKvList := buildLenDelim(0x32, kvListValInvalid) // field 6 = kvlist
	kvWithKvList := buildLenDelim(0x0A, []byte("k"))
	kvWithKvList = append(kvWithKvList, buildLenDelim(0x12, anyValKvList)...)
	resourceForKvList := buildLenDelim(0x0A, kvWithKvList)
	rsForKvList := buildLenDelim(0x0A, resourceForKvList)
	tdInvalidKvList := buildLenDelim(0x0A, rsForKvList)
	if _, err := Unmarshal(tdInvalidKvList); err == nil {
		t.Error("unmarshalKvListValue: expected error from invalid wire type")
	}
}

// TestUnmarshalReadMessageErrors tests error propagation when ReadMessage fails
// (truncated sub-message data).
func TestUnmarshalReadMessageErrors(t *testing.T) {
	// A tag followed by a length saying there are N bytes, but fewer bytes follow
	// This causes ReadMessage to fail.

	// Truncated ResourceSpans sub-message: field 1 (ResourceSpans), wire type 2,
	// claim length 10 but provide only 2 bytes
	truncatedRS := []byte{0x0A, 0x0A, 0x01, 0x02} // field 1, len=10, but only 2 bytes
	if _, err := Unmarshal(truncatedRS); err == nil {
		t.Error("Unmarshal: expected error on truncated ResourceSpans")
	}

	// Truncated Resource sub-message inside ResourceSpans
	truncatedResource := []byte{0x0A, 0x0A, 0x01, 0x02} // field 1 (Resource), len=10, 2 bytes
	rsForResource := buildLenDelim(0x0A, truncatedResource)
	if _, err := Unmarshal(rsForResource); err == nil {
		t.Error("unmarshalResourceSpans: expected error on truncated Resource")
	}

	// Truncated ScopeSpans inside ResourceSpans
	truncatedSS := []byte{0x12, 0x0A, 0x01, 0x02} // field 2 (ScopeSpans), len=10, 2 bytes
	rsForSS := buildLenDelim(0x0A, truncatedSS)
	if _, err := Unmarshal(rsForSS); err == nil {
		t.Error("unmarshalResourceSpans: expected error on truncated ScopeSpans")
	}

	// Truncated Scope inside ScopeSpans
	// ScopeSpans message: field 1 (Scope), len=10, 2 bytes → ReadMessage will fail
	truncatedScopeInSS := []byte{0x0A, 0x0A, 0x01, 0x02} // tag 0x0A, len=10, 2 bytes
	// ScopeSpans embedded in ResourceSpans as field 2 (tag 0x12)
	rsMsgForScope := buildLenDelim(0x12, truncatedScopeInSS) // RS message with field 2 = SS
	tdForScope := buildLenDelim(0x0A, rsMsgForScope)         // TD with field 1 = RS
	if _, err := Unmarshal(tdForScope); err == nil {
		t.Error("unmarshalScopeSpans: expected error on truncated Scope")
	}

	// Truncated Span inside ScopeSpans
	// ScopeSpans message: field 2 (Span), len=10, 2 bytes → ReadMessage will fail
	truncatedSpanInSS := []byte{0x12, 0x0A, 0x01, 0x02}    // tag 0x12, len=10, 2 bytes
	rsMsgForSpan := buildLenDelim(0x12, truncatedSpanInSS) // RS message with field 2 = SS
	tdForSpan := buildLenDelim(0x0A, rsMsgForSpan)         // TD with field 1 = RS
	if _, err := Unmarshal(tdForSpan); err == nil {
		t.Error("unmarshalScopeSpans: expected error on truncated Span")
	}

	// Truncated Attribute (KeyValue) inside Resource
	truncatedKV := []byte{0x0A, 0x0A, 0x01, 0x02} // field 1 (Attribute), len=10, 2 bytes
	resourceForKV := buildLenDelim(0x0A, truncatedKV)
	rsForKV := buildLenDelim(0x0A, resourceForKV)
	tdForKV := buildLenDelim(0x0A, rsForKV)
	if _, err := Unmarshal(tdForKV); err == nil {
		t.Error("unmarshalResource: expected error on truncated Attribute")
	}

	// Truncated AnyValue inside KeyValue (which is inside Resource)
	truncatedAV := []byte{0x12, 0x0A, 0x01, 0x02} // field 2 (AnyValue), len=10, 2 bytes
	kvWithTruncAV := buildLenDelim(0x0A, []byte("k"))
	kvWithTruncAV = append(kvWithTruncAV, truncatedAV...)
	resourceForAV := buildLenDelim(0x0A, kvWithTruncAV)
	rsForAV := buildLenDelim(0x0A, resourceForAV)
	tdForAV := buildLenDelim(0x0A, rsForAV)
	if _, err := Unmarshal(tdForAV); err == nil {
		t.Error("unmarshalKeyValue: expected error on truncated AnyValue")
	}

	traceID := make([]byte, 16)
	traceID[0] = 1
	spanID := make([]byte, 8)
	spanID[0] = 1

	// Truncated Attributes inside Span
	truncatedSpanAttr := []byte{0x4A, 0x0A, 0x01, 0x02} // field 9 (Attr), len=10, 2 bytes
	spanWithTruncAttr := buildLenDelim(0x0A, traceID)
	spanWithTruncAttr = append(spanWithTruncAttr, buildLenDelim(0x12, spanID)...)
	spanWithTruncAttr = append(spanWithTruncAttr, buildLenDelim(0x2A, []byte("s"))...)
	spanWithTruncAttr = append(spanWithTruncAttr, encodeFixed64Tag(0x39, 1)...)
	spanWithTruncAttr = append(spanWithTruncAttr, encodeFixed64Tag(0x41, 2)...)
	spanWithTruncAttr = append(spanWithTruncAttr, truncatedSpanAttr...)
	ssForSpanAttr := buildLenDelim(0x12, spanWithTruncAttr)
	rsForSpanAttr := buildLenDelim(0x12, ssForSpanAttr)
	tdForSpanAttr := buildLenDelim(0x0A, rsForSpanAttr)
	if _, err := Unmarshal(tdForSpanAttr); err == nil {
		t.Error("unmarshalSpan: expected error on truncated Attributes")
	}

	// Truncated Event inside Span
	truncatedEvent := []byte{0x5A, 0x0A, 0x01, 0x02} // field 11 (Event), len=10, 2 bytes
	spanWithTruncEvent := buildLenDelim(0x0A, traceID)
	spanWithTruncEvent = append(spanWithTruncEvent, buildLenDelim(0x12, spanID)...)
	spanWithTruncEvent = append(spanWithTruncEvent, buildLenDelim(0x2A, []byte("s"))...)
	spanWithTruncEvent = append(spanWithTruncEvent, encodeFixed64Tag(0x39, 1)...)
	spanWithTruncEvent = append(spanWithTruncEvent, encodeFixed64Tag(0x41, 2)...)
	spanWithTruncEvent = append(spanWithTruncEvent, truncatedEvent...)
	ssForEvent := buildLenDelim(0x12, spanWithTruncEvent)
	rsForEvent := buildLenDelim(0x12, ssForEvent)
	tdForEvent := buildLenDelim(0x0A, rsForEvent)
	if _, err := Unmarshal(tdForEvent); err == nil {
		t.Error("unmarshalSpan: expected error on truncated Event")
	}

	// Truncated Link inside Span
	truncatedLink := []byte{0x62, 0x0A, 0x01, 0x02} // field 12 (Link), len=10, 2 bytes
	spanWithTruncLink := buildLenDelim(0x0A, traceID)
	spanWithTruncLink = append(spanWithTruncLink, buildLenDelim(0x12, spanID)...)
	spanWithTruncLink = append(spanWithTruncLink, buildLenDelim(0x2A, []byte("s"))...)
	spanWithTruncLink = append(spanWithTruncLink, encodeFixed64Tag(0x39, 1)...)
	spanWithTruncLink = append(spanWithTruncLink, encodeFixed64Tag(0x41, 2)...)
	spanWithTruncLink = append(spanWithTruncLink, truncatedLink...)
	ssForLink := buildLenDelim(0x12, spanWithTruncLink)
	rsForLink := buildLenDelim(0x12, ssForLink)
	tdForLink := buildLenDelim(0x0A, rsForLink)
	if _, err := Unmarshal(tdForLink); err == nil {
		t.Error("unmarshalSpan: expected error on truncated Link")
	}

	// Truncated Status inside Span
	truncatedStatus := []byte{0x6A, 0x0A, 0x01, 0x02} // field 13 (Status), len=10, 2 bytes
	spanWithTruncStatus := buildLenDelim(0x0A, traceID)
	spanWithTruncStatus = append(spanWithTruncStatus, buildLenDelim(0x12, spanID)...)
	spanWithTruncStatus = append(spanWithTruncStatus, buildLenDelim(0x2A, []byte("s"))...)
	spanWithTruncStatus = append(spanWithTruncStatus, encodeFixed64Tag(0x39, 1)...)
	spanWithTruncStatus = append(spanWithTruncStatus, encodeFixed64Tag(0x41, 2)...)
	spanWithTruncStatus = append(spanWithTruncStatus, truncatedStatus...)
	ssForStatus := buildLenDelim(0x12, spanWithTruncStatus)
	rsForStatus := buildLenDelim(0x12, ssForStatus)
	tdForStatus := buildLenDelim(0x0A, rsForStatus)
	if _, err := Unmarshal(tdForStatus); err == nil {
		t.Error("unmarshalSpan: expected error on truncated Status")
	}

	// Truncated Event Attribute inside SpanEvent
	truncatedEventAttr := []byte{0x1A, 0x0A, 0x01, 0x02} // field 3 (Attr), len=10, 2 bytes
	eventWithTruncAttr := encodeFixed64Tag(0x09, 100)
	eventWithTruncAttr = append(eventWithTruncAttr, buildLenDelim(0x12, []byte("ev"))...)
	eventWithTruncAttr = append(eventWithTruncAttr, truncatedEventAttr...)
	spanWithEvent := buildLenDelim(0x0A, traceID)
	spanWithEvent = append(spanWithEvent, buildLenDelim(0x12, spanID)...)
	spanWithEvent = append(spanWithEvent, buildLenDelim(0x2A, []byte("s"))...)
	spanWithEvent = append(spanWithEvent, encodeFixed64Tag(0x39, 1)...)
	spanWithEvent = append(spanWithEvent, encodeFixed64Tag(0x41, 2)...)
	spanWithEvent = append(spanWithEvent, buildLenDelim(0x5A, eventWithTruncAttr)...)
	ssForEvAttr := buildLenDelim(0x12, spanWithEvent)
	rsForEvAttr := buildLenDelim(0x12, ssForEvAttr)
	tdForEvAttr := buildLenDelim(0x0A, rsForEvAttr)
	if _, err := Unmarshal(tdForEvAttr); err == nil {
		t.Error("unmarshalSpanEvent: expected error on truncated Attribute")
	}

	// Truncated Link Attribute inside SpanLink
	truncatedLinkAttr := []byte{0x22, 0x0A, 0x01, 0x02} // field 4 (Attr), len=10, 2 bytes
	linkWithTruncAttr := buildLenDelim(0x0A, traceID)
	linkWithTruncAttr = append(linkWithTruncAttr, buildLenDelim(0x12, spanID)...)
	linkWithTruncAttr = append(linkWithTruncAttr, truncatedLinkAttr...)
	spanWithLink := buildLenDelim(0x0A, traceID)
	spanWithLink = append(spanWithLink, buildLenDelim(0x12, spanID)...)
	spanWithLink = append(spanWithLink, buildLenDelim(0x2A, []byte("s"))...)
	spanWithLink = append(spanWithLink, encodeFixed64Tag(0x39, 1)...)
	spanWithLink = append(spanWithLink, encodeFixed64Tag(0x41, 2)...)
	spanWithLink = append(spanWithLink, buildLenDelim(0x62, linkWithTruncAttr)...)
	ssForLinkAttr := buildLenDelim(0x12, spanWithLink)
	rsForLinkAttr := buildLenDelim(0x12, ssForLinkAttr)
	tdForLinkAttr := buildLenDelim(0x0A, rsForLinkAttr)
	if _, err := Unmarshal(tdForLinkAttr); err == nil {
		t.Error("unmarshalSpanLink: expected error on truncated Attribute")
	}

	// Truncated AnyValue inside ArrayValue
	truncatedArrayAV := []byte{0x0A, 0x0A, 0x01, 0x02} // field 1 (Values), len=10, 2 bytes
	arrayValWithTrunc := truncatedArrayAV
	anyValWithTruncArray := buildLenDelim(0x2A, arrayValWithTrunc) // field 5 = array
	kvForArray := buildLenDelim(0x0A, []byte("k"))
	kvForArray = append(kvForArray, buildLenDelim(0x12, anyValWithTruncArray)...)
	resourceForArray2 := buildLenDelim(0x0A, kvForArray)
	rsForArray2 := buildLenDelim(0x0A, resourceForArray2)
	tdForArray2 := buildLenDelim(0x0A, rsForArray2)
	if _, err := Unmarshal(tdForArray2); err == nil {
		t.Error("unmarshalArrayValue: expected error on truncated AnyValue")
	}

	// Truncated KV inside KvListValue
	truncatedKvListKV := []byte{0x0A, 0x0A, 0x01, 0x02} // field 1 (Values), len=10, 2 bytes
	kvListValWithTrunc := truncatedKvListKV
	anyValWithTruncKvList := buildLenDelim(0x32, kvListValWithTrunc) // field 6 = kvlist
	kvForKvList := buildLenDelim(0x0A, []byte("k"))
	kvForKvList = append(kvForKvList, buildLenDelim(0x12, anyValWithTruncKvList)...)
	resourceForKvList2 := buildLenDelim(0x0A, kvForKvList)
	rsForKvList2 := buildLenDelim(0x0A, resourceForKvList2)
	tdForKvList2 := buildLenDelim(0x0A, rsForKvList2)
	if _, err := Unmarshal(tdForKvList2); err == nil {
		t.Error("unmarshalKvListValue: expected error on truncated KV")
	}

	// Truncated Array AnyValue (in outer any value with type array)
	truncatedAnyValForArray := []byte{0x0A, 0x0A, 0x01, 0x02}
	anyValTruncInArray := buildLenDelim(0x2A, truncatedAnyValForArray)
	kvForTruncArray := buildLenDelim(0x0A, []byte("k"))
	kvForTruncArray = append(kvForTruncArray, buildLenDelim(0x12, anyValTruncInArray)...)
	resourceForTruncArray := buildLenDelim(0x0A, kvForTruncArray)
	rsForTruncArray := buildLenDelim(0x0A, resourceForTruncArray)
	tdForTruncArray := buildLenDelim(0x0A, rsForTruncArray)
	if _, err := Unmarshal(tdForTruncArray); err == nil {
		t.Error("unmarshalAnyValue: expected error on truncated Array sub-message")
	}

	// Truncated KvList (in outer any value with type kvlist)
	truncatedAnyValForKvList := []byte{0x0A, 0x0A, 0x01, 0x02}
	anyValTruncInKvList := buildLenDelim(0x32, truncatedAnyValForKvList)
	kvForTruncKvList := buildLenDelim(0x0A, []byte("k"))
	kvForTruncKvList = append(kvForTruncKvList, buildLenDelim(0x12, anyValTruncInKvList)...)
	resourceForTruncKvList := buildLenDelim(0x0A, kvForTruncKvList)
	rsForTruncKvList := buildLenDelim(0x0A, resourceForTruncKvList)
	tdForTruncKvList := buildLenDelim(0x0A, rsForTruncKvList)
	if _, err := Unmarshal(tdForTruncKvList); err == nil {
		t.Error("unmarshalAnyValue: expected error on truncated KvList sub-message")
	}

	// Truncated Scope Attribute inside InstrumentationScope
	truncatedScopeAttr := []byte{0x1A, 0x0A, 0x01, 0x02} // field 3 (Attr), len=10, 2 bytes
	scopeWithTruncAttr := buildLenDelim(0x0A, []byte("lib"))
	scopeWithTruncAttr = append(scopeWithTruncAttr, truncatedScopeAttr...)
	ssForScopeAttr := buildLenDelim(0x0A, scopeWithTruncAttr)
	rsForScopeAttr := buildLenDelim(0x12, ssForScopeAttr)
	tdForScopeAttr := buildLenDelim(0x0A, rsForScopeAttr)
	if _, err := Unmarshal(tdForScopeAttr); err == nil {
		t.Error("unmarshalScope: expected error on truncated Attribute")
	}

	// Truncated inner unmarshalResourceSpans call (unmarshal of sub returns error)
	// This is hard to trigger directly without corrupt inner data...
	// Instead test a ReadField error inside unmarshalResourceSpans
	// by having a truncated field tag at the end
	rsWithTruncTag := buildLenDelim(0x12, []byte{}) // valid empty ScopeSpans
	rsWithTruncTag = append(rsWithTruncTag, 0xFF)   // incomplete multi-byte varint
	tdWithTruncRS := buildLenDelim(0x0A, rsWithTruncTag)
	if _, err := Unmarshal(tdWithTruncRS); err == nil {
		t.Error("unmarshalResourceSpans: expected error on truncated field tag")
	}
}

// TestUnmarshalSubMessageErrors tests error propagation when inner unmarshal functions fail.
// This covers the "if err != nil { return } after sub-unmarshal call" branches.
func TestUnmarshalSubMessageErrors(t *testing.T) {
	// Invalid wire type tag for use inside sub-messages
	// field 99, wire type 3 (invalid): 99<<3|3 = 795 → varint 0xFB 0x06
	invalidWireTag := []byte{0xFB, 0x06}

	// --- unmarshalResource: unmarshalKeyValue error ---
	// KV message that causes unmarshalKeyValue to fail:
	// use a valid KV tag but with invalid wire type for the value
	badKVMsg := buildLenDelim(0x0A, []byte("key"))       // valid key
	badKVMsg = append(badKVMsg, invalidWireTag...)       // invalid field in KV → unmarshalKeyValue errors
	resourceWithBadKV := buildLenDelim(0x0A, badKVMsg)   // Resource field 1 = KV
	rsForBadKV := buildLenDelim(0x0A, resourceWithBadKV) // RS field 1 = Resource
	tdForBadKV := buildLenDelim(0x0A, rsForBadKV)        // TD field 1 = RS
	if _, err := Unmarshal(tdForBadKV); err == nil {
		t.Error("unmarshalResource: expected error from unmarshalKeyValue failing")
	}

	// --- unmarshalScope: unmarshalKeyValue error for scope attributes ---
	// Scope attribute KV is field 3 (0x1A), use invalid content
	badScopeKVMsg := buildLenDelim(0x0A, []byte("key")) // valid key
	badScopeKVMsg = append(badScopeKVMsg, invalidWireTag...)
	scopeWithBadAttr := buildLenDelim(0x1A, badScopeKVMsg)      // Scope field 3 = Attr KV
	ssForBadScopeAttr := buildLenDelim(0x0A, scopeWithBadAttr)  // ScopeSpans field 1 = Scope
	rsForBadScopeAttr := buildLenDelim(0x12, ssForBadScopeAttr) // RS field 2 = SS
	tdForBadScopeAttr := buildLenDelim(0x0A, rsForBadScopeAttr) // TD field 1 = RS
	if _, err := Unmarshal(tdForBadScopeAttr); err == nil {
		t.Error("unmarshalScope: expected error from unmarshalKeyValue failing")
	}

	traceID := make([]byte, 16)
	traceID[0] = 1
	spanID := make([]byte, 8)
	spanID[0] = 1

	// --- unmarshalSpan: unmarshalKeyValue error for span attributes ---
	badSpanKVMsg := buildLenDelim(0x0A, []byte("key"))
	badSpanKVMsg = append(badSpanKVMsg, invalidWireTag...)
	spanWithBadAttr := buildLenDelim(0x0A, traceID)
	spanWithBadAttr = append(spanWithBadAttr, buildLenDelim(0x12, spanID)...)
	spanWithBadAttr = append(spanWithBadAttr, buildLenDelim(0x2A, []byte("s"))...)
	spanWithBadAttr = append(spanWithBadAttr, encodeFixed64Tag(0x39, 1)...)
	spanWithBadAttr = append(spanWithBadAttr, encodeFixed64Tag(0x41, 2)...)
	spanWithBadAttr = append(spanWithBadAttr, buildLenDelim(0x4A, badSpanKVMsg)...) // field 9 = Attr
	ssForBadSpanAttr := buildLenDelim(0x12, spanWithBadAttr)
	rsForBadSpanAttr := buildLenDelim(0x12, ssForBadSpanAttr)
	tdForBadSpanAttr := buildLenDelim(0x0A, rsForBadSpanAttr)
	if _, err := Unmarshal(tdForBadSpanAttr); err == nil {
		t.Error("unmarshalSpan: expected error from unmarshalKeyValue in span attributes")
	}

	// --- unmarshalSpan: unmarshalSpanEvent error ---
	badEventMsg := encodeFixed64Tag(0x09, 100)
	badEventMsg = append(badEventMsg, invalidWireTag...) // invalid field in event
	spanWithBadEvent := buildLenDelim(0x0A, traceID)
	spanWithBadEvent = append(spanWithBadEvent, buildLenDelim(0x12, spanID)...)
	spanWithBadEvent = append(spanWithBadEvent, buildLenDelim(0x2A, []byte("s"))...)
	spanWithBadEvent = append(spanWithBadEvent, encodeFixed64Tag(0x39, 1)...)
	spanWithBadEvent = append(spanWithBadEvent, encodeFixed64Tag(0x41, 2)...)
	spanWithBadEvent = append(spanWithBadEvent, buildLenDelim(0x5A, badEventMsg)...) // field 11 = Event
	ssForBadEvent := buildLenDelim(0x12, spanWithBadEvent)
	rsForBadEvent := buildLenDelim(0x12, ssForBadEvent)
	tdForBadEvent := buildLenDelim(0x0A, rsForBadEvent)
	if _, err := Unmarshal(tdForBadEvent); err == nil {
		t.Error("unmarshalSpan: expected error from unmarshalSpanEvent failing")
	}

	// --- unmarshalSpan: unmarshalSpanLink error ---
	linkTID := make([]byte, 16)
	linkTID[0] = 2
	linkSID := make([]byte, 8)
	linkSID[0] = 2
	badLinkMsg := buildLenDelim(0x0A, linkTID)
	badLinkMsg = append(badLinkMsg, buildLenDelim(0x12, linkSID)...)
	badLinkMsg = append(badLinkMsg, invalidWireTag...) // invalid field in link
	spanWithBadLink := buildLenDelim(0x0A, traceID)
	spanWithBadLink = append(spanWithBadLink, buildLenDelim(0x12, spanID)...)
	spanWithBadLink = append(spanWithBadLink, buildLenDelim(0x2A, []byte("s"))...)
	spanWithBadLink = append(spanWithBadLink, encodeFixed64Tag(0x39, 1)...)
	spanWithBadLink = append(spanWithBadLink, encodeFixed64Tag(0x41, 2)...)
	spanWithBadLink = append(spanWithBadLink, buildLenDelim(0x62, badLinkMsg)...) // field 12 = Link
	ssForBadLink := buildLenDelim(0x12, spanWithBadLink)
	rsForBadLink := buildLenDelim(0x12, ssForBadLink)
	tdForBadLink := buildLenDelim(0x0A, rsForBadLink)
	if _, err := Unmarshal(tdForBadLink); err == nil {
		t.Error("unmarshalSpan: expected error from unmarshalSpanLink failing")
	}

	// --- unmarshalSpan: unmarshalStatus error ---
	badStatusMsg := buildLenDelim(0x12, []byte("ok"))
	badStatusMsg = append(badStatusMsg, invalidWireTag...) // invalid field in status
	spanWithBadStatus := buildLenDelim(0x0A, traceID)
	spanWithBadStatus = append(spanWithBadStatus, buildLenDelim(0x12, spanID)...)
	spanWithBadStatus = append(spanWithBadStatus, buildLenDelim(0x2A, []byte("s"))...)
	spanWithBadStatus = append(spanWithBadStatus, encodeFixed64Tag(0x39, 1)...)
	spanWithBadStatus = append(spanWithBadStatus, encodeFixed64Tag(0x41, 2)...)
	spanWithBadStatus = append(spanWithBadStatus, buildLenDelim(0x6A, badStatusMsg)...) // field 13 = Status
	ssForBadStatus := buildLenDelim(0x12, spanWithBadStatus)
	rsForBadStatus := buildLenDelim(0x12, ssForBadStatus)
	tdForBadStatus := buildLenDelim(0x0A, rsForBadStatus)
	if _, err := Unmarshal(tdForBadStatus); err == nil {
		t.Error("unmarshalSpan: expected error from unmarshalStatus failing")
	}

	// --- unmarshalSpanEvent: unmarshalKeyValue error ---
	badEvKVMsg := buildLenDelim(0x0A, []byte("key"))
	badEvKVMsg = append(badEvKVMsg, invalidWireTag...)
	eventWithBadAttr := encodeFixed64Tag(0x09, 100)
	eventWithBadAttr = append(eventWithBadAttr, buildLenDelim(0x12, []byte("ev"))...)
	eventWithBadAttr = append(eventWithBadAttr, buildLenDelim(0x1A, badEvKVMsg)...) // field 3 = Attr
	spanForEvKV := buildLenDelim(0x0A, traceID)
	spanForEvKV = append(spanForEvKV, buildLenDelim(0x12, spanID)...)
	spanForEvKV = append(spanForEvKV, buildLenDelim(0x2A, []byte("s"))...)
	spanForEvKV = append(spanForEvKV, encodeFixed64Tag(0x39, 1)...)
	spanForEvKV = append(spanForEvKV, encodeFixed64Tag(0x41, 2)...)
	spanForEvKV = append(spanForEvKV, buildLenDelim(0x5A, eventWithBadAttr)...)
	ssForEvKV := buildLenDelim(0x12, spanForEvKV)
	rsForEvKV := buildLenDelim(0x12, ssForEvKV)
	tdForEvKV := buildLenDelim(0x0A, rsForEvKV)
	if _, err := Unmarshal(tdForEvKV); err == nil {
		t.Error("unmarshalSpanEvent: expected error from unmarshalKeyValue failing")
	}

	// --- unmarshalSpanLink: unmarshalKeyValue error ---
	badLinkKVMsg := buildLenDelim(0x0A, []byte("key"))
	badLinkKVMsg = append(badLinkKVMsg, invalidWireTag...)
	linkWithBadAttr := buildLenDelim(0x0A, linkTID)
	linkWithBadAttr = append(linkWithBadAttr, buildLenDelim(0x12, linkSID)...)
	linkWithBadAttr = append(linkWithBadAttr, buildLenDelim(0x22, badLinkKVMsg)...) // field 4 = Attr
	spanForLinkKV := buildLenDelim(0x0A, traceID)
	spanForLinkKV = append(spanForLinkKV, buildLenDelim(0x12, spanID)...)
	spanForLinkKV = append(spanForLinkKV, buildLenDelim(0x2A, []byte("s"))...)
	spanForLinkKV = append(spanForLinkKV, encodeFixed64Tag(0x39, 1)...)
	spanForLinkKV = append(spanForLinkKV, encodeFixed64Tag(0x41, 2)...)
	spanForLinkKV = append(spanForLinkKV, buildLenDelim(0x62, linkWithBadAttr)...)
	ssForLinkKV := buildLenDelim(0x12, spanForLinkKV)
	rsForLinkKV := buildLenDelim(0x12, ssForLinkKV)
	tdForLinkKV := buildLenDelim(0x0A, rsForLinkKV)
	if _, err := Unmarshal(tdForLinkKV); err == nil {
		t.Error("unmarshalSpanLink: expected error from unmarshalKeyValue failing")
	}

	// --- unmarshalAnyValue: unmarshalArrayValue error ---
	badArrayMsg := buildLenDelim(0x0A, []byte{0x0A, 0x01, 'x'}) // valid AV item
	badArrayMsg = append(badArrayMsg, invalidWireTag...)        // invalid field in ArrayValue
	anyValBadArray := buildLenDelim(0x2A, badArrayMsg)          // AnyValue field 5 = ArrayValue
	kvForBadArray := buildLenDelim(0x0A, []byte("k"))
	kvForBadArray = append(kvForBadArray, buildLenDelim(0x12, anyValBadArray)...)
	resourceForBadArray := buildLenDelim(0x0A, kvForBadArray)
	rsForBadArray := buildLenDelim(0x0A, resourceForBadArray)
	tdForBadArray := buildLenDelim(0x0A, rsForBadArray)
	if _, err := Unmarshal(tdForBadArray); err == nil {
		t.Error("unmarshalAnyValue: expected error from unmarshalArrayValue failing")
	}

	// --- unmarshalAnyValue: unmarshalKvListValue error ---
	innerKV := buildLenDelim(0x0A, []byte("k"))
	innerKV = append(innerKV, buildLenDelim(0x12, []byte{0x0A, 0x01, 'v'})...)
	badKvListMsg := buildLenDelim(0x0A, innerKV)
	badKvListMsg = append(badKvListMsg, invalidWireTag...) // invalid field in KvListValue
	anyValBadKvList := buildLenDelim(0x32, badKvListMsg)   // AnyValue field 6 = KvListValue
	kvForBadKvList := buildLenDelim(0x0A, []byte("k"))
	kvForBadKvList = append(kvForBadKvList, buildLenDelim(0x12, anyValBadKvList)...)
	resourceForBadKvList := buildLenDelim(0x0A, kvForBadKvList)
	rsForBadKvList := buildLenDelim(0x0A, resourceForBadKvList)
	tdForBadKvList := buildLenDelim(0x0A, rsForBadKvList)
	if _, err := Unmarshal(tdForBadKvList); err == nil {
		t.Error("unmarshalAnyValue: expected error from unmarshalKvListValue failing")
	}

	// --- unmarshalArrayValue: unmarshalAnyValue error ---
	badInnerAV := []byte{0x0A, 0x0A, 0x01, 0x02}      // truncated AnyValue string
	arrayWithBadAV := buildLenDelim(0x0A, badInnerAV) // ArrayValue field 1 = bad AV
	anyValForBadAV := buildLenDelim(0x2A, arrayWithBadAV)
	kvForBadAV := buildLenDelim(0x0A, []byte("k"))
	kvForBadAV = append(kvForBadAV, buildLenDelim(0x12, anyValForBadAV)...)
	resourceForBadAV := buildLenDelim(0x0A, kvForBadAV)
	rsForBadAV := buildLenDelim(0x0A, resourceForBadAV)
	tdForBadAV := buildLenDelim(0x0A, rsForBadAV)
	if _, err := Unmarshal(tdForBadAV); err == nil {
		t.Error("unmarshalArrayValue: expected error from unmarshalAnyValue failing")
	}

	// --- unmarshalKvListValue: unmarshalKeyValue error ---
	badInnerKVInList := buildLenDelim(0x0A, []byte("k"))
	badInnerKVInList = append(badInnerKVInList, invalidWireTag...)
	kvListWithBadKV := buildLenDelim(0x0A, badInnerKVInList) // KvListValue field 1 = bad KV
	anyValForBadKV := buildLenDelim(0x32, kvListWithBadKV)
	kvForBadKVInList := buildLenDelim(0x0A, []byte("k"))
	kvForBadKVInList = append(kvForBadKVInList, buildLenDelim(0x12, anyValForBadKV)...)
	resourceForBadKVInList := buildLenDelim(0x0A, kvForBadKVInList)
	rsForBadKVInList := buildLenDelim(0x0A, resourceForBadKVInList)
	tdForBadKVInList := buildLenDelim(0x0A, rsForBadKVInList)
	if _, err := Unmarshal(tdForBadKVInList); err == nil {
		t.Error("unmarshalKvListValue: expected error from unmarshalKeyValue failing")
	}

	// --- unmarshalResourceSpans: unmarshalResource error ---
	badResourceMsg := invalidWireTag                           // Resource with invalid wire type → unmarshalResource errors
	rsWithBadResource := buildLenDelim(0x0A, badResourceMsg)   // RS field 1 = bad Resource
	tdForBadResource := buildLenDelim(0x0A, rsWithBadResource) // TD field 1 = RS
	if _, err := Unmarshal(tdForBadResource); err == nil {
		t.Error("unmarshalResourceSpans: expected error from unmarshalResource failing")
	}

	// --- unmarshalResourceSpans: unmarshalScopeSpans error ---
	badSSContent := invalidWireTag                   // ScopeSpans with invalid wire type
	rsWithBadSS := buildLenDelim(0x12, badSSContent) // RS field 2 = bad SS
	tdForBadSS := buildLenDelim(0x0A, rsWithBadSS)   // TD field 1 = RS
	if _, err := Unmarshal(tdForBadSS); err == nil {
		t.Error("unmarshalResourceSpans: expected error from unmarshalScopeSpans failing")
	}

	// --- unmarshalScopeSpans: unmarshalScope error ---
	badScopeContent := invalidWireTag                      // Scope with invalid wire type
	ssWithBadScope := buildLenDelim(0x0A, badScopeContent) // SS field 1 = bad Scope
	rsForBadScope := buildLenDelim(0x12, ssWithBadScope)   // RS field 2 = SS
	tdForBadScope := buildLenDelim(0x0A, rsForBadScope)    // TD field 1 = RS
	if _, err := Unmarshal(tdForBadScope); err == nil {
		t.Error("unmarshalScopeSpans: expected error from unmarshalScope failing")
	}

	// --- unmarshalScopeSpans: unmarshalSpan error ---
	badSpanContent := buildLenDelim(0x0A, traceID) // valid TraceID
	badSpanContent = append(badSpanContent, buildLenDelim(0x12, spanID)...)
	badSpanContent = append(badSpanContent, invalidWireTag...) // invalid field in span
	ssWithBadSpan := buildLenDelim(0x12, badSpanContent)       // SS field 2 = bad Span
	rsForBadSpan := buildLenDelim(0x12, ssWithBadSpan)         // RS field 2 = SS
	tdForBadSpan := buildLenDelim(0x0A, rsForBadSpan)          // TD field 1 = RS
	if _, err := Unmarshal(tdForBadSpan); err == nil {
		t.Error("unmarshalScopeSpans: expected error from unmarshalSpan failing")
	}

	// --- Unmarshal: unmarshalResourceSpans error ---
	badRSContent := invalidWireTag                   // RS with invalid wire type → unmarshalResourceSpans errors
	tdWithBadRS := buildLenDelim(0x0A, badRSContent) // TD field 1 = bad RS
	if _, err := Unmarshal(tdWithBadRS); err == nil {
		t.Error("Unmarshal: expected error from unmarshalResourceSpans failing")
	}
}

// TestUnmarshalReadFieldErrors tests that truncated field tags within sub-messages
// cause ReadField to fail and propagate errors correctly.
// This covers the "fn, wt, err := dec.ReadField(); if err != nil { return }" branches
// that appear in every unmarshal function.
func TestUnmarshalReadFieldErrors(t *testing.T) {
	// A single 0xFF byte as content for a sub-message — it starts a multi-byte varint
	// but has no continuation, causing ReadField → ReadVarint → ErrBufferUnderflow.
	truncTag := []byte{0xFF}

	// --- Unmarshal: ReadField error in TracesData decoder ---
	// Valid RS field first, then truncated tag at top level
	validRS := buildLenDelim(0x0A, []byte{}) // field 1, empty RS
	tdTruncTag := append(validRS, truncTag...)
	if _, err := Unmarshal(tdTruncTag); err == nil {
		t.Error("Unmarshal: expected ReadField error from truncated tag")
	}

	// --- unmarshalResource: ReadField error ---
	resourceTruncTag := append([]byte{}, truncTag...)
	rsTruncResource := buildLenDelim(0x0A, resourceTruncTag) // RS field 1 = Resource with trunc tag
	tdTruncResource := buildLenDelim(0x0A, rsTruncResource)
	if _, err := Unmarshal(tdTruncResource); err == nil {
		t.Error("unmarshalResource: expected ReadField error from truncated tag")
	}

	// --- unmarshalScopeSpans: ReadField error ---
	ssTruncTag := append([]byte{}, truncTag...)
	rsTruncSS := buildLenDelim(0x12, ssTruncTag) // RS field 2 = ScopeSpans with trunc tag
	tdTruncSS := buildLenDelim(0x0A, rsTruncSS)
	if _, err := Unmarshal(tdTruncSS); err == nil {
		t.Error("unmarshalScopeSpans: expected ReadField error from truncated tag")
	}

	// --- unmarshalScope: ReadField error ---
	scopeTruncTag := append([]byte{}, truncTag...)
	ssTruncScope := buildLenDelim(0x0A, scopeTruncTag) // SS field 1 = Scope with trunc tag
	rsTruncScope := buildLenDelim(0x12, ssTruncScope)  // RS field 2 = SS
	tdTruncScope := buildLenDelim(0x0A, rsTruncScope)
	if _, err := Unmarshal(tdTruncScope); err == nil {
		t.Error("unmarshalScope: expected ReadField error from truncated tag")
	}

	traceID := make([]byte, 16)
	traceID[0] = 1
	spanID := make([]byte, 8)
	spanID[0] = 1

	// --- unmarshalSpan: ReadField error ---
	spanTruncTag := buildLenDelim(0x0A, traceID)
	spanTruncTag = append(spanTruncTag, truncTag...)
	ssTruncSpan := buildLenDelim(0x12, spanTruncTag) // SS field 2 = Span with trunc
	rsTruncSpan := buildLenDelim(0x12, ssTruncSpan)
	tdTruncSpan := buildLenDelim(0x0A, rsTruncSpan)
	if _, err := Unmarshal(tdTruncSpan); err == nil {
		t.Error("unmarshalSpan: expected ReadField error from truncated tag")
	}

	// --- unmarshalSpanEvent: ReadField error ---
	eventTruncTag := append([]byte{}, truncTag...)
	spanWithEventTrunc := buildLenDelim(0x0A, traceID)
	spanWithEventTrunc = append(spanWithEventTrunc, buildLenDelim(0x12, spanID)...)
	spanWithEventTrunc = append(spanWithEventTrunc, buildLenDelim(0x2A, []byte("s"))...)
	spanWithEventTrunc = append(spanWithEventTrunc, encodeFixed64Tag(0x39, 1)...)
	spanWithEventTrunc = append(spanWithEventTrunc, encodeFixed64Tag(0x41, 2)...)
	spanWithEventTrunc = append(spanWithEventTrunc, buildLenDelim(0x5A, eventTruncTag)...)
	ssEvTrunc := buildLenDelim(0x12, spanWithEventTrunc)
	rsEvTrunc := buildLenDelim(0x12, ssEvTrunc)
	tdEvTrunc := buildLenDelim(0x0A, rsEvTrunc)
	if _, err := Unmarshal(tdEvTrunc); err == nil {
		t.Error("unmarshalSpanEvent: expected ReadField error from truncated tag")
	}

	// --- unmarshalSpanLink: ReadField error ---
	linkTruncTag := append([]byte{}, truncTag...)
	spanWithLinkTrunc := buildLenDelim(0x0A, traceID)
	spanWithLinkTrunc = append(spanWithLinkTrunc, buildLenDelim(0x12, spanID)...)
	spanWithLinkTrunc = append(spanWithLinkTrunc, buildLenDelim(0x2A, []byte("s"))...)
	spanWithLinkTrunc = append(spanWithLinkTrunc, encodeFixed64Tag(0x39, 1)...)
	spanWithLinkTrunc = append(spanWithLinkTrunc, encodeFixed64Tag(0x41, 2)...)
	spanWithLinkTrunc = append(spanWithLinkTrunc, buildLenDelim(0x62, linkTruncTag)...)
	ssLinkTrunc := buildLenDelim(0x12, spanWithLinkTrunc)
	rsLinkTrunc := buildLenDelim(0x12, ssLinkTrunc)
	tdLinkTrunc := buildLenDelim(0x0A, rsLinkTrunc)
	if _, err := Unmarshal(tdLinkTrunc); err == nil {
		t.Error("unmarshalSpanLink: expected ReadField error from truncated tag")
	}

	// --- unmarshalStatus: ReadField error ---
	statusTruncTag := append([]byte{}, truncTag...)
	spanWithStatusTrunc := buildLenDelim(0x0A, traceID)
	spanWithStatusTrunc = append(spanWithStatusTrunc, buildLenDelim(0x12, spanID)...)
	spanWithStatusTrunc = append(spanWithStatusTrunc, buildLenDelim(0x2A, []byte("s"))...)
	spanWithStatusTrunc = append(spanWithStatusTrunc, encodeFixed64Tag(0x39, 1)...)
	spanWithStatusTrunc = append(spanWithStatusTrunc, encodeFixed64Tag(0x41, 2)...)
	spanWithStatusTrunc = append(spanWithStatusTrunc, buildLenDelim(0x6A, statusTruncTag)...)
	ssStatusTrunc := buildLenDelim(0x12, spanWithStatusTrunc)
	rsStatusTrunc := buildLenDelim(0x12, ssStatusTrunc)
	tdStatusTrunc := buildLenDelim(0x0A, rsStatusTrunc)
	if _, err := Unmarshal(tdStatusTrunc); err == nil {
		t.Error("unmarshalStatus: expected ReadField error from truncated tag")
	}

	// --- unmarshalKeyValue: ReadField error ---
	kvTruncTag := append([]byte{}, truncTag...)
	// Put KV in Resource attributes
	resourceKVTrunc := buildLenDelim(0x0A, kvTruncTag) // Resource field 1 = bad KV
	rsKVTrunc := buildLenDelim(0x0A, resourceKVTrunc)
	tdKVTrunc := buildLenDelim(0x0A, rsKVTrunc)
	if _, err := Unmarshal(tdKVTrunc); err == nil {
		t.Error("unmarshalKeyValue: expected ReadField error from truncated tag")
	}

	// --- unmarshalAnyValue: ReadField error ---
	anyValTruncTag := []byte{0x0A, 0x01, 's'} // valid string first
	anyValTruncTag = append(anyValTruncTag, truncTag...)
	kvWithAVTrunc := buildLenDelim(0x0A, []byte("k"))
	kvWithAVTrunc = append(kvWithAVTrunc, buildLenDelim(0x12, anyValTruncTag)...)
	resourceAVTrunc := buildLenDelim(0x0A, kvWithAVTrunc)
	rsAVTrunc := buildLenDelim(0x0A, resourceAVTrunc)
	tdAVTrunc := buildLenDelim(0x0A, rsAVTrunc)
	if _, err := Unmarshal(tdAVTrunc); err == nil {
		t.Error("unmarshalAnyValue: expected ReadField error from truncated tag")
	}

	// --- unmarshalArrayValue: ReadField error ---
	arrayTruncTag := append([]byte{}, truncTag...)
	anyValWithArrayTrunc := buildLenDelim(0x2A, arrayTruncTag) // AnyValue field 5 = truncated array
	kvForArrayTrunc := buildLenDelim(0x0A, []byte("k"))
	kvForArrayTrunc = append(kvForArrayTrunc, buildLenDelim(0x12, anyValWithArrayTrunc)...)
	resourceArrayTrunc := buildLenDelim(0x0A, kvForArrayTrunc)
	rsArrayTrunc := buildLenDelim(0x0A, resourceArrayTrunc)
	tdArrayTrunc := buildLenDelim(0x0A, rsArrayTrunc)
	if _, err := Unmarshal(tdArrayTrunc); err == nil {
		t.Error("unmarshalArrayValue: expected ReadField error from truncated tag")
	}

	// --- unmarshalKvListValue: ReadField error ---
	kvListTruncTag := append([]byte{}, truncTag...)
	anyValWithKvListTrunc := buildLenDelim(0x32, kvListTruncTag) // AnyValue field 6 = truncated kvlist
	kvForKvListTrunc := buildLenDelim(0x0A, []byte("k"))
	kvForKvListTrunc = append(kvForKvListTrunc, buildLenDelim(0x12, anyValWithKvListTrunc)...)
	resourceKvListTrunc := buildLenDelim(0x0A, kvForKvListTrunc)
	rsKvListTrunc := buildLenDelim(0x0A, resourceKvListTrunc)
	tdKvListTrunc := buildLenDelim(0x0A, rsKvListTrunc)
	if _, err := Unmarshal(tdKvListTrunc); err == nil {
		t.Error("unmarshalKvListValue: expected ReadField error from truncated tag")
	}
}

// TestUnmarshalSpecificErrorPaths targets specific uncovered error branches by line number.
func TestUnmarshalSpecificErrorPaths(t *testing.T) {
	// line 30-32: Unmarshal SkipField error
	// TracesData with an invalid-wire-type unknown field
	invalidWireTag := []byte{0xFB, 0x06} // field 99, wire type 3 (invalid)
	if _, err := Unmarshal(invalidWireTag); err == nil {
		t.Error("Unmarshal: expected SkipField error for invalid wire type at TD level")
	}

	// line 85-87: unmarshalResource ReadMessage error
	// Need: RS content has valid Resource field tag with length X, but Resource content
	// has fewer than X bytes
	// Resource content = Attributes field tag + length=10 + 2 bytes (truncated)
	attrTrunc := []byte{0x0A, 0x0A, 0x01, 0x02} // valid attr tag, len=10, 2 bytes
	// This is the Resource content (4 bytes), but Attributes ReadMessage wants 10
	// RS content = field 1 (Resource) + length = 4 + attrTrunc
	rsContent85 := buildLenDelim(0x0A, attrTrunc) // field 1 (Resource), len=4, content=attrTrunc
	tdMsg85 := buildLenDelim(0x0A, rsContent85)   // field 1 (RS), len=len(rsContent85)
	if _, err := Unmarshal(tdMsg85); err == nil {
		t.Error("unmarshalResource line 85: expected ReadMessage error for truncated Attributes")
	}

	traceID := make([]byte, 16)
	traceID[0] = 1

	// line 181-183: unmarshalSpan ReadBytes error for TraceID
	// Need a Span message where TraceID field (field 1) has length > available bytes
	spanTraceIDTrunc := []byte{0x0A, 0x10, 0x01, 0x02}   // field 1, len=16, only 2 bytes
	ssMsgForTID := buildLenDelim(0x12, spanTraceIDTrunc) // SS field 2 = span
	rsMsgForTID := buildLenDelim(0x12, ssMsgForTID)      // RS field 2 = SS
	tdForTID := buildLenDelim(0x0A, rsMsgForTID)
	if _, err := Unmarshal(tdForTID); err == nil {
		t.Error("unmarshalSpan line 181: expected ReadBytes error for truncated TraceID")
	}

	// line 187-189: unmarshalSpan ReadBytes error for SpanID
	spanSpanIDTrunc := buildLenDelim(0x0A, traceID)                        // valid TraceID (16 bytes)
	spanSpanIDTrunc = append(spanSpanIDTrunc, []byte{0x12, 0x08, 0x01}...) // SpanID field 2, len=8, only 1 byte
	ssMsgForSID := buildLenDelim(0x12, spanSpanIDTrunc)
	rsMsgForSID := buildLenDelim(0x12, ssMsgForSID)
	tdForSID := buildLenDelim(0x0A, rsMsgForSID)
	if _, err := Unmarshal(tdForSID); err == nil {
		t.Error("unmarshalSpan line 187: expected ReadBytes error for truncated SpanID")
	}

	spanID := make([]byte, 8)
	spanID[0] = 1
	parentSpanID := make([]byte, 8)
	parentSpanID[0] = 2

	// line 195-197: unmarshalSpan ReadBytes error for ParentSpanID
	spanParentIDTrunc := buildLenDelim(0x0A, traceID)
	spanParentIDTrunc = append(spanParentIDTrunc, buildLenDelim(0x12, spanID)...)
	spanParentIDTrunc = append(spanParentIDTrunc, []byte{0x22, 0x08, 0x01, 0x02}...) // ParentSpanID field 4, len=8, 2 bytes
	ssMsgForPSID := buildLenDelim(0x12, spanParentIDTrunc)
	rsMsgForPSID := buildLenDelim(0x12, ssMsgForPSID)
	tdForPSID := buildLenDelim(0x0A, rsMsgForPSID)
	if _, err := Unmarshal(tdForPSID); err == nil {
		t.Error("unmarshalSpan line 195: expected ReadBytes error for truncated ParentSpanID")
	}

	// line 203-205: unmarshalSpan ReadVarint error for Kind
	spanKindTrunc := buildLenDelim(0x0A, traceID)
	spanKindTrunc = append(spanKindTrunc, buildLenDelim(0x12, spanID)...)
	spanKindTrunc = append(spanKindTrunc, buildLenDelim(0x2A, []byte("s"))...)
	spanKindTrunc = append(spanKindTrunc, []byte{0x30, 0xFF}...) // Kind field 6, truncated varint
	ssMsgForKind := buildLenDelim(0x12, spanKindTrunc)
	rsMsgForKind := buildLenDelim(0x12, ssMsgForKind)
	tdForKind := buildLenDelim(0x0A, rsMsgForKind)
	if _, err := Unmarshal(tdForKind); err == nil {
		t.Error("unmarshalSpan line 203: expected ReadVarint error for truncated Kind")
	}

	// line 253-255: unmarshalSpan ReadVarint error for DroppedAttributesCount
	spanDropAttrTrunc := buildLenDelim(0x0A, traceID)
	spanDropAttrTrunc = append(spanDropAttrTrunc, buildLenDelim(0x12, spanID)...)
	spanDropAttrTrunc = append(spanDropAttrTrunc, buildLenDelim(0x2A, []byte("s"))...)
	spanDropAttrTrunc = append(spanDropAttrTrunc, encodeFixed64Tag(0x39, 1)...)
	spanDropAttrTrunc = append(spanDropAttrTrunc, encodeFixed64Tag(0x41, 2)...)
	spanDropAttrTrunc = append(spanDropAttrTrunc, []byte{0x70, 0xFF}...) // DroppedAttr field 14, truncated varint
	ssMsgDropAttr := buildLenDelim(0x12, spanDropAttrTrunc)
	rsMsgDropAttr := buildLenDelim(0x12, ssMsgDropAttr)
	tdDropAttr := buildLenDelim(0x0A, rsMsgDropAttr)
	if _, err := Unmarshal(tdDropAttr); err == nil {
		t.Error("unmarshalSpan line 253: expected ReadVarint error for truncated DroppedAttributesCount")
	}

	// line 259-261: unmarshalSpan ReadVarint error for DroppedEventsCount
	spanDropEvTrunc := buildLenDelim(0x0A, traceID)
	spanDropEvTrunc = append(spanDropEvTrunc, buildLenDelim(0x12, spanID)...)
	spanDropEvTrunc = append(spanDropEvTrunc, buildLenDelim(0x2A, []byte("s"))...)
	spanDropEvTrunc = append(spanDropEvTrunc, encodeFixed64Tag(0x39, 1)...)
	spanDropEvTrunc = append(spanDropEvTrunc, encodeFixed64Tag(0x41, 2)...)
	spanDropEvTrunc = append(spanDropEvTrunc, []byte{0x78, 0xFF}...) // DroppedEvents field 15, truncated varint
	ssMsgDropEv := buildLenDelim(0x12, spanDropEvTrunc)
	rsMsgDropEv := buildLenDelim(0x12, ssMsgDropEv)
	tdDropEv := buildLenDelim(0x0A, rsMsgDropEv)
	if _, err := Unmarshal(tdDropEv); err == nil {
		t.Error("unmarshalSpan line 259: expected ReadVarint error for truncated DroppedEventsCount")
	}

	// line 265-267: unmarshalSpan ReadVarint error for DroppedLinksCount
	// DroppedLinksCount is field 16 → tag = 16<<3|0 = 128 = varint 0x80 0x01
	spanDropLinkTrunc := buildLenDelim(0x0A, traceID)
	spanDropLinkTrunc = append(spanDropLinkTrunc, buildLenDelim(0x12, spanID)...)
	spanDropLinkTrunc = append(spanDropLinkTrunc, buildLenDelim(0x2A, []byte("s"))...)
	spanDropLinkTrunc = append(spanDropLinkTrunc, encodeFixed64Tag(0x39, 1)...)
	spanDropLinkTrunc = append(spanDropLinkTrunc, encodeFixed64Tag(0x41, 2)...)
	spanDropLinkTrunc = append(spanDropLinkTrunc, []byte{0x80, 0x01, 0xFF}...) // field 16 tag + truncated varint value
	ssMsgDropLink := buildLenDelim(0x12, spanDropLinkTrunc)
	rsMsgDropLink := buildLenDelim(0x12, ssMsgDropLink)
	tdDropLink := buildLenDelim(0x0A, rsMsgDropLink)
	if _, err := Unmarshal(tdDropLink); err == nil {
		t.Error("unmarshalSpan line 265: expected ReadVarint error for truncated DroppedLinksCount")
	}

	// line 303-305: unmarshalSpanEvent ReadVarint error for DroppedAttributesCount
	// Event DroppedAttributesCount = field 4, tag = 4<<3|0 = 32 = 0x20
	eventDropTrunc := encodeFixed64Tag(0x09, 100)
	eventDropTrunc = append(eventDropTrunc, buildLenDelim(0x12, []byte("ev"))...)
	eventDropTrunc = append(eventDropTrunc, []byte{0x20, 0xFF}...) // field 4, truncated varint
	spanForEventDrop := buildLenDelim(0x0A, traceID)
	spanForEventDrop = append(spanForEventDrop, buildLenDelim(0x12, spanID)...)
	spanForEventDrop = append(spanForEventDrop, buildLenDelim(0x2A, []byte("s"))...)
	spanForEventDrop = append(spanForEventDrop, encodeFixed64Tag(0x39, 1)...)
	spanForEventDrop = append(spanForEventDrop, encodeFixed64Tag(0x41, 2)...)
	spanForEventDrop = append(spanForEventDrop, buildLenDelim(0x5A, eventDropTrunc)...)
	ssEventDrop := buildLenDelim(0x12, spanForEventDrop)
	rsEventDrop := buildLenDelim(0x12, ssEventDrop)
	tdEventDrop := buildLenDelim(0x0A, rsEventDrop)
	if _, err := Unmarshal(tdEventDrop); err == nil {
		t.Error("unmarshalSpanEvent line 303: expected ReadVarint error for truncated DroppedAttributesCount")
	}

	// line 327-329: unmarshalSpanLink ReadBytes error for TraceID
	// Link TraceID = field 1 (tag 0x0A)
	linkTraceIDTrunc := []byte{0x0A, 0x10, 0x01, 0x02} // field 1, len=16, 2 bytes
	spanForLinkTID := buildLenDelim(0x0A, traceID)
	spanForLinkTID = append(spanForLinkTID, buildLenDelim(0x12, spanID)...)
	spanForLinkTID = append(spanForLinkTID, buildLenDelim(0x2A, []byte("s"))...)
	spanForLinkTID = append(spanForLinkTID, encodeFixed64Tag(0x39, 1)...)
	spanForLinkTID = append(spanForLinkTID, encodeFixed64Tag(0x41, 2)...)
	spanForLinkTID = append(spanForLinkTID, buildLenDelim(0x62, linkTraceIDTrunc)...) // Link field 12
	ssLinkTID := buildLenDelim(0x12, spanForLinkTID)
	rsLinkTID := buildLenDelim(0x12, ssLinkTID)
	tdLinkTID := buildLenDelim(0x0A, rsLinkTID)
	if _, err := Unmarshal(tdLinkTID); err == nil {
		t.Error("unmarshalSpanLink line 327: expected ReadBytes error for truncated Link TraceID")
	}

	// line 333-335: unmarshalSpanLink ReadBytes error for SpanID
	linkTID := make([]byte, 16)
	linkTID[0] = 2
	linkSpanIDTrunc := buildLenDelim(0x0A, linkTID)
	linkSpanIDTrunc = append(linkSpanIDTrunc, []byte{0x12, 0x08, 0x01}...) // SpanID field 2, len=8, 1 byte
	spanForLinkSID := buildLenDelim(0x0A, traceID)
	spanForLinkSID = append(spanForLinkSID, buildLenDelim(0x12, spanID)...)
	spanForLinkSID = append(spanForLinkSID, buildLenDelim(0x2A, []byte("s"))...)
	spanForLinkSID = append(spanForLinkSID, encodeFixed64Tag(0x39, 1)...)
	spanForLinkSID = append(spanForLinkSID, encodeFixed64Tag(0x41, 2)...)
	spanForLinkSID = append(spanForLinkSID, buildLenDelim(0x62, linkSpanIDTrunc)...)
	ssLinkSID := buildLenDelim(0x12, spanForLinkSID)
	rsLinkSID := buildLenDelim(0x12, ssLinkSID)
	tdLinkSID := buildLenDelim(0x0A, rsLinkSID)
	if _, err := Unmarshal(tdLinkSID); err == nil {
		t.Error("unmarshalSpanLink line 333: expected ReadBytes error for truncated Link SpanID")
	}

	// line 351-353: unmarshalSpanLink ReadVarint error for DroppedAttributesCount
	// Link DroppedAttributesCount = field 5, tag = 5<<3|0 = 40 = 0x28
	linkSID := make([]byte, 8)
	linkSID[0] = 2
	linkDropTrunc := buildLenDelim(0x0A, linkTID)
	linkDropTrunc = append(linkDropTrunc, buildLenDelim(0x12, linkSID)...)
	linkDropTrunc = append(linkDropTrunc, []byte{0x28, 0xFF}...) // field 5, truncated varint
	spanForLinkDrop := buildLenDelim(0x0A, traceID)
	spanForLinkDrop = append(spanForLinkDrop, buildLenDelim(0x12, spanID)...)
	spanForLinkDrop = append(spanForLinkDrop, buildLenDelim(0x2A, []byte("s"))...)
	spanForLinkDrop = append(spanForLinkDrop, encodeFixed64Tag(0x39, 1)...)
	spanForLinkDrop = append(spanForLinkDrop, encodeFixed64Tag(0x41, 2)...)
	spanForLinkDrop = append(spanForLinkDrop, buildLenDelim(0x62, linkDropTrunc)...)
	ssLinkDrop := buildLenDelim(0x12, spanForLinkDrop)
	rsLinkDrop := buildLenDelim(0x12, ssLinkDrop)
	tdLinkDrop := buildLenDelim(0x0A, rsLinkDrop)
	if _, err := Unmarshal(tdLinkDrop); err == nil {
		t.Error("unmarshalSpanLink line 351: expected ReadVarint error for truncated Link DroppedAttributesCount")
	}

	// line 377-379: unmarshalStatus ReadVarint error for Code
	// Status Code = field 3, tag = 3<<3|0 = 24 = 0x18
	statusCodeTrunc := buildLenDelim(0x12, []byte("ok"))             // valid message
	statusCodeTrunc = append(statusCodeTrunc, []byte{0x18, 0xFF}...) // field 3, truncated varint
	spanForStatusCode := buildLenDelim(0x0A, traceID)
	spanForStatusCode = append(spanForStatusCode, buildLenDelim(0x12, spanID)...)
	spanForStatusCode = append(spanForStatusCode, buildLenDelim(0x2A, []byte("s"))...)
	spanForStatusCode = append(spanForStatusCode, encodeFixed64Tag(0x39, 1)...)
	spanForStatusCode = append(spanForStatusCode, encodeFixed64Tag(0x41, 2)...)
	spanForStatusCode = append(spanForStatusCode, buildLenDelim(0x6A, statusCodeTrunc)...)
	ssStatusCode := buildLenDelim(0x12, spanForStatusCode)
	rsStatusCode := buildLenDelim(0x12, ssStatusCode)
	tdStatusCode := buildLenDelim(0x0A, rsStatusCode)
	if _, err := Unmarshal(tdStatusCode); err == nil {
		t.Error("unmarshalStatus line 377: expected ReadVarint error for truncated StatusCode")
	}

	// line 447-449: unmarshalAnyValue ReadMessage error for ArrayValue
	// AnyValue field 5 (ArrayValue) with truncated length
	anyValArrayTrunc := []byte{0x2A, 0x0A, 0x01, 0x02} // field 5, len=10, 2 bytes
	kvForArrayTrunc := buildLenDelim(0x0A, []byte("k"))
	kvForArrayTrunc = append(kvForArrayTrunc, buildLenDelim(0x12, anyValArrayTrunc)...)
	resourceArrayTrunc := buildLenDelim(0x0A, kvForArrayTrunc)
	rsArrayTrunc := buildLenDelim(0x0A, resourceArrayTrunc)
	tdArrayTrunc := buildLenDelim(0x0A, rsArrayTrunc)
	if _, err := Unmarshal(tdArrayTrunc); err == nil {
		t.Error("unmarshalAnyValue line 447: expected ReadMessage error for truncated ArrayValue")
	}

	// line 458-460: unmarshalAnyValue ReadMessage error for KvListValue
	// AnyValue field 6 (KvList) with truncated length
	anyValKvListTrunc := []byte{0x32, 0x0A, 0x01, 0x02} // field 6, len=10, 2 bytes
	kvForKvListTrunc := buildLenDelim(0x0A, []byte("k"))
	kvForKvListTrunc = append(kvForKvListTrunc, buildLenDelim(0x12, anyValKvListTrunc)...)
	resourceKvListTrunc := buildLenDelim(0x0A, kvForKvListTrunc)
	rsKvListTrunc := buildLenDelim(0x0A, resourceKvListTrunc)
	tdKvListTrunc := buildLenDelim(0x0A, rsKvListTrunc)
	if _, err := Unmarshal(tdKvListTrunc); err == nil {
		t.Error("unmarshalAnyValue line 458: expected ReadMessage error for truncated KvListValue")
	}
}

// TestHTTPReceiverGzipSuccess tests a valid gzip-encoded request (covers gzip reader path).
func TestHTTPReceiverGzipSuccess(t *testing.T) {
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	body, _ := json.Marshal(td)

	// Compress the body with gzip
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write(body)
	gw.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", &buf)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Encoding", "gzip")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("gzip success: status = %d, body = %s", w.Code, w.Body.String())
	}
	if recv.SpansReceived() != 1 {
		t.Errorf("gzip success: received = %d", recv.SpansReceived())
	}
}

// TestHTTPReceiverReadBodyError tests that a body read error returns 400.
func TestHTTPReceiverReadBodyError(t *testing.T) {
	// Use a valid gzip stream but set maxBodyBytes to tiny value to cause a read error
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 1) // maxBodyBytes=1 to trigger max bytes reader error

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	body, _ := json.Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHTTPReceiverProtobufAlias(t *testing.T) {
	// Test the "application/protobuf" content-type alias path
	sink := &mockSink{}
	recv := NewHTTPReceiver(sink, 0)

	td := TracesData{
		ResourceSpans: []ResourceSpans{{
			ScopeSpans: []ScopeSpans{{
				Spans: []Span{{
					TraceID: [16]byte{1}, SpanID: [8]byte{1}, Name: "test",
					StartTimeUnixNano: 1000, EndTimeUnixNano: 2000,
				}},
			}},
		}},
	}
	body, _ := Marshal(td)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/protobuf") // the alias path
	recv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if recv.SpansReceived() != 1 {
		t.Errorf("received = %d", recv.SpansReceived())
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
