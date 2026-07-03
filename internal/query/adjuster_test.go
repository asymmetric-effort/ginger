package query

import (
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func makeTD(spans ...otlp.Span) otlp.TracesData {
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			ScopeSpans: []otlp.ScopeSpans{{Spans: spans}},
		}},
	}
}

func getSpans(td otlp.TracesData) []otlp.Span {
	if len(td.ResourceSpans) == 0 || len(td.ResourceSpans[0].ScopeSpans) == 0 {
		return nil
	}
	return td.ResourceSpans[0].ScopeSpans[0].Spans
}

func TestSpanSorter(t *testing.T) {
	td := makeTD(
		otlp.Span{SpanID: [8]byte{3}, StartTimeUnixNano: 3000},
		otlp.Span{SpanID: [8]byte{1}, StartTimeUnixNano: 1000},
		otlp.Span{SpanID: [8]byte{2}, StartTimeUnixNano: 2000},
	)
	result, warnings := SpanSorter{}.Adjust(td)
	if len(warnings) != 0 {
		t.Error("sorter should produce no warnings")
	}
	spans := getSpans(result)
	if spans[0].StartTimeUnixNano != 1000 || spans[1].StartTimeUnixNano != 2000 || spans[2].StartTimeUnixNano != 3000 {
		t.Error("spans not sorted")
	}
}

func TestSpanDeduplicator(t *testing.T) {
	td := makeTD(
		otlp.Span{SpanID: [8]byte{1}, Name: "a", StartTimeUnixNano: 1000},
		otlp.Span{SpanID: [8]byte{1}, Name: "a-dup", StartTimeUnixNano: 1000},
		otlp.Span{SpanID: [8]byte{2}, Name: "b", StartTimeUnixNano: 2000},
	)
	result, warnings := SpanDeduplicator{}.Adjust(td)
	spans := getSpans(result)
	if len(spans) != 2 {
		t.Errorf("expected 2 unique spans, got %d", len(spans))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestSpanDeduplicatorKeepsBetter(t *testing.T) {
	attrs := otlp.NewAttributes()
	attrs.Set("key", otlp.StringValue("val"))
	td := makeTD(
		otlp.Span{SpanID: [8]byte{1}, Name: "a"},
		otlp.Span{SpanID: [8]byte{1}, Name: "a-better", Attributes: attrs},
	)
	result, _ := SpanDeduplicator{}.Adjust(td)
	spans := getSpans(result)
	if spans[0].Attributes.Len() != 1 {
		t.Error("should keep span with more attributes")
	}
}

func TestClockSkewAdjuster(t *testing.T) {
	parent := otlp.Span{
		SpanID:            [8]byte{1},
		StartTimeUnixNano: 2000,
		EndTimeUnixNano:   5000,
	}
	child := otlp.Span{
		SpanID:            [8]byte{2},
		ParentSpanID:      [8]byte{1},
		StartTimeUnixNano: 1000, // before parent!
		EndTimeUnixNano:   3000,
	}
	td := makeTD(parent, child)

	result, warnings := (ClockSkewAdjuster{MaxAdjust: time.Second}).Adjust(td)
	spans := getSpans(result)

	if spans[1].StartTimeUnixNano != 2000 {
		t.Errorf("child start = %d, want 2000", spans[1].StartTimeUnixNano)
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestClockSkewAdjusterMaxExceeded(t *testing.T) {
	parent := otlp.Span{
		SpanID:            [8]byte{1},
		StartTimeUnixNano: 5000000000, // 5s
	}
	child := otlp.Span{
		SpanID:            [8]byte{2},
		ParentSpanID:      [8]byte{1},
		StartTimeUnixNano: 1000000000, // 1s — 4s before parent
		EndTimeUnixNano:   3000000000,
	}
	td := makeTD(parent, child)

	result, warnings := (ClockSkewAdjuster{MaxAdjust: time.Second}).Adjust(td)
	spans := getSpans(result)

	// Should NOT adjust (diff > MaxAdjust)
	if spans[1].StartTimeUnixNano != 1000000000 {
		t.Error("should not adjust when diff exceeds max")
	}
	if len(warnings) != 0 {
		t.Error("should have no warnings")
	}
}

func TestClockSkewAdjusterNoParent(t *testing.T) {
	span := otlp.Span{SpanID: [8]byte{1}, StartTimeUnixNano: 1000}
	td := makeTD(span)
	result, warnings := (ClockSkewAdjuster{}).Adjust(td)
	spans := getSpans(result)
	if spans[0].StartTimeUnixNano != 1000 {
		t.Error("no parent — should not adjust")
	}
	if len(warnings) != 0 {
		t.Error("no warnings expected")
	}
}

func TestClockSkewAdjusterMissingParent(t *testing.T) {
	span := otlp.Span{SpanID: [8]byte{1}, ParentSpanID: [8]byte{99}, StartTimeUnixNano: 1000}
	td := makeTD(span)
	_, warnings := (ClockSkewAdjuster{}).Adjust(td)
	if len(warnings) != 0 {
		t.Error("missing parent — no warning")
	}
}

func TestClockSkewAdjusterNoSkew(t *testing.T) {
	parent := otlp.Span{SpanID: [8]byte{1}, StartTimeUnixNano: 1000}
	child := otlp.Span{SpanID: [8]byte{2}, ParentSpanID: [8]byte{1}, StartTimeUnixNano: 2000}
	td := makeTD(parent, child)
	_, warnings := (ClockSkewAdjuster{}).Adjust(td)
	if len(warnings) != 0 {
		t.Error("no skew — no warning")
	}
}

func TestIPAttributeNormalizer(t *testing.T) {
	attrs := otlp.NewAttributes()
	attrs.Set("net.peer.ip", otlp.BytesValue([]byte{10, 0, 0, 1}))
	span := otlp.Span{SpanID: [8]byte{1}, Attributes: attrs}
	td := makeTD(span)

	result, _ := IPAttributeNormalizer{}.Adjust(td)
	spans := getSpans(result)
	v, _ := spans[0].Attributes.Get("net.peer.ip")
	if v.Str != "10.0.0.1" {
		t.Errorf("IP = %q, want 10.0.0.1", v.Str)
	}
}

func TestIPAttributeNormalizerNoIP(t *testing.T) {
	span := otlp.Span{SpanID: [8]byte{1}}
	td := makeTD(span)
	_, warnings := IPAttributeNormalizer{}.Adjust(td)
	if len(warnings) != 0 {
		t.Error("no IP attrs — no warnings")
	}
}

func TestIPAttributeNormalizerStringIP(t *testing.T) {
	attrs := otlp.NewAttributes()
	attrs.Set("net.peer.ip", otlp.StringValue("10.0.0.1"))
	span := otlp.Span{SpanID: [8]byte{1}, Attributes: attrs}
	td := makeTD(span)

	result, _ := IPAttributeNormalizer{}.Adjust(td)
	spans := getSpans(result)
	v, _ := spans[0].Attributes.Get("net.peer.ip")
	// String IP should remain unchanged
	if v.Str != "10.0.0.1" {
		t.Errorf("string IP changed: %q", v.Str)
	}
}

func TestAdjusterChain(t *testing.T) {
	chain := NewAdjusterChain(SpanSorter{}, SpanDeduplicator{})
	td := makeTD(
		otlp.Span{SpanID: [8]byte{2}, StartTimeUnixNano: 2000},
		otlp.Span{SpanID: [8]byte{1}, StartTimeUnixNano: 1000},
		otlp.Span{SpanID: [8]byte{1}, StartTimeUnixNano: 1000}, // dup
	)
	result, warnings := chain.Adjust(td)
	spans := getSpans(result)
	if len(spans) != 2 {
		t.Errorf("expected 2, got %d", len(spans))
	}
	if spans[0].StartTimeUnixNano != 1000 {
		t.Error("should be sorted")
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %d", len(warnings))
	}
}
