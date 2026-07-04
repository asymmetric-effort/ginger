package zipkin

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalArray(t *testing.T) {
	input := `[{
		"traceId": "0af7651916cd43dd8448eb211c80319c",
		"id": "b7ad6b7169203331",
		"name": "test",
		"kind": "SERVER",
		"timestamp": 1000000,
		"duration": 500000,
		"localEndpoint": {"serviceName": "svc1", "ipv4": "10.0.0.1", "port": 8080},
		"tags": {"http.method": "GET"},
		"annotations": [{"timestamp": 1250000, "value": "event1"}]
	}]`

	spans, err := UnmarshalArray([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]
	if s.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("traceId = %q", s.TraceID)
	}
	if s.Name != "test" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Kind != KindServer {
		t.Errorf("kind = %q", s.Kind)
	}
	if s.LocalEndpoint.ServiceName != "svc1" {
		t.Error("endpoint")
	}
}

func TestUnmarshalArrayInvalid(t *testing.T) {
	_, err := UnmarshalArray([]byte("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMarshalArray(t *testing.T) {
	spans := []Span{{
		TraceID: "abc123",
		ID:      "def456",
		Name:    "test",
		Kind:    KindClient,
	}}
	data, err := MarshalArray(spans)
	if err != nil {
		t.Fatal(err)
	}
	var parsed []map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed[0]["kind"] != "CLIENT" {
		t.Error("kind")
	}
}

func TestToOTLP(t *testing.T) {
	spans := []Span{{
		TraceID:       "0af7651916cd43dd8448eb211c80319c",
		ID:            "b7ad6b7169203331",
		ParentID:      "0000000000000001",
		Name:          "test-op",
		Kind:          KindServer,
		Timestamp:     1000000,
		Duration:      500000,
		LocalEndpoint: &Endpoint{ServiceName: "svc1"},
		RemoteEndpoint: &Endpoint{
			ServiceName: "upstream",
			IPv4:        "10.0.0.2",
			Port:        9090,
		},
		Tags:        map[string]string{"http.method": "GET"},
		Annotations: []Annotation{{Timestamp: 1250000, Value: "event1"}},
	}}

	td := ToOTLP(spans)
	if len(td.ResourceSpans) != 1 {
		t.Fatalf("ResourceSpans: %d", len(td.ResourceSpans))
	}
	rs := td.ResourceSpans[0]
	svc, ok := rs.Resource.Attributes.Get("service.name")
	if !ok || svc.Str != "svc1" {
		t.Error("service.name")
	}

	otlpSpan := rs.ScopeSpans[0].Spans[0]
	if otlpSpan.Name != "test-op" {
		t.Error("name")
	}
	if otlpSpan.StartTimeUnixNano != 1000000000 {
		t.Errorf("start = %d", otlpSpan.StartTimeUnixNano)
	}
	if otlpSpan.EndTimeUnixNano != 1500000000 {
		t.Errorf("end = %d", otlpSpan.EndTimeUnixNano)
	}

	// Check remote endpoint attrs
	peer, ok := otlpSpan.Attributes.Get("peer.service")
	if !ok || peer.Str != "upstream" {
		t.Error("peer.service")
	}
	ip, ok := otlpSpan.Attributes.Get("net.peer.ip")
	if !ok || ip.Str != "10.0.0.2" {
		t.Error("net.peer.ip")
	}
	port, ok := otlpSpan.Attributes.Get("net.peer.port")
	if !ok || port.IntVal != 9090 {
		t.Error("net.peer.port")
	}

	if len(otlpSpan.Events) != 1 || otlpSpan.Events[0].Name != "event1" {
		t.Error("events")
	}
}

func TestToOTLPGroupByService(t *testing.T) {
	spans := []Span{
		{TraceID: "aa", ID: "11", Name: "s1", LocalEndpoint: &Endpoint{ServiceName: "svcA"}},
		{TraceID: "aa", ID: "22", Name: "s2", LocalEndpoint: &Endpoint{ServiceName: "svcB"}},
		{TraceID: "aa", ID: "33", Name: "s3", LocalEndpoint: &Endpoint{ServiceName: "svcA"}},
	}
	td := ToOTLP(spans)
	if len(td.ResourceSpans) != 2 {
		t.Fatalf("expected 2 ResourceSpans (2 services), got %d", len(td.ResourceSpans))
	}
}

func TestToOTLPNoEndpoint(t *testing.T) {
	spans := []Span{{TraceID: "aa", ID: "11", Name: "orphan"}}
	td := ToOTLP(spans)
	if len(td.ResourceSpans) != 1 {
		t.Fatal("should still create ResourceSpans")
	}
}

func TestToOTLPNoRemoteEndpoint(t *testing.T) {
	spans := []Span{{
		TraceID:       "aa",
		ID:            "11",
		Name:          "test",
		LocalEndpoint: &Endpoint{ServiceName: "svc"},
	}}
	td := ToOTLP(spans)
	span := td.ResourceSpans[0].ScopeSpans[0].Spans[0]
	_, ok := span.Attributes.Get("peer.service")
	if ok {
		t.Error("should not have peer.service without remote endpoint")
	}
}

func TestToOTLPAllKinds(t *testing.T) {
	kinds := []Kind{KindClient, KindServer, KindProducer, KindConsumer, ""}
	for _, k := range kinds {
		spans := []Span{{
			TraceID: "aa", ID: "11", Name: "test", Kind: k,
			LocalEndpoint: &Endpoint{ServiceName: "svc"},
		}}
		td := ToOTLP(spans)
		if len(td.ResourceSpans) != 1 {
			t.Error("should have ResourceSpans")
		}
	}
}

func TestToOTLP64BitTraceID(t *testing.T) {
	spans := []Span{{
		TraceID:       "b7ad6b7169203331", // 64-bit
		ID:            "b7ad6b7169203331",
		Name:          "test",
		LocalEndpoint: &Endpoint{ServiceName: "svc"},
	}}
	td := ToOTLP(spans)
	span := td.ResourceSpans[0].ScopeSpans[0].Spans[0]
	// First 8 bytes should be zero (padded)
	for i := 0; i < 8; i++ {
		if span.TraceID[i] != 0 {
			t.Errorf("byte %d should be 0", i)
		}
	}
	// Last 8 bytes should be non-zero
	nonZero := false
	for i := 8; i < 16; i++ {
		if span.TraceID[i] != 0 {
			nonZero = true
		}
	}
	if !nonZero {
		t.Error("last 8 bytes should be non-zero")
	}
}

func TestSpanModel(t *testing.T) {
	s := Span{
		TraceID:  "abc",
		ID:       "def",
		ParentID: "ghi",
		Name:     "test",
		Debug:    true,
		Shared:   true,
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var s2 Span
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatal(err)
	}
	if s2.Debug != true || s2.Shared != true {
		t.Error("debug/shared not preserved")
	}
}
