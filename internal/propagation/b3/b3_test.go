package b3

import (
	"context"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/propagation"
)

func TestInjectExtractSingleHeader(t *testing.T) {
	tid, _ := propagation.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := propagation.SpanIDFromHex("b7ad6b7169203331")
	sc := propagation.NewSpanContext(tid, sid, propagation.FlagsSampled, propagation.TraceState{}, false)

	ctx := propagation.ContextWithSpanContext(context.Background(), sc)
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(ctx, carrier)

	val := carrier.Get("b3")
	expected := "0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-1"
	if val != expected {
		t.Errorf("b3 = %q, want %q", val, expected)
	}

	ctx2 := Propagator{}.Extract(context.Background(), carrier)
	sc2 := propagation.SpanContextFromContext(ctx2)
	if !sc2.IsValid() {
		t.Fatal("should be valid")
	}
	if sc2.TraceID() != tid {
		t.Error("trace ID mismatch")
	}
	if !sc2.IsSampled() {
		t.Error("should be sampled")
	}
}

func TestInjectNotSampled(t *testing.T) {
	tid, _ := propagation.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := propagation.SpanIDFromHex("b7ad6b7169203331")
	sc := propagation.NewSpanContext(tid, sid, 0, propagation.TraceState{}, false)

	ctx := propagation.ContextWithSpanContext(context.Background(), sc)
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(ctx, carrier)

	val := carrier.Get("b3")
	if val[len(val)-1] != '0' {
		t.Errorf("should end with 0: %q", val)
	}
}

func TestExtractMultiHeader(t *testing.T) {
	carrier := propagation.MapCarrier{
		"x-b3-traceid": "0af7651916cd43dd8448eb211c80319c",
		"x-b3-spanid":  "b7ad6b7169203331",
		"x-b3-sampled": "1",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatal("should be valid")
	}
	if !sc.IsSampled() {
		t.Error("should be sampled")
	}
}

func TestExtractMultiHeaderFlags(t *testing.T) {
	carrier := propagation.MapCarrier{
		"x-b3-traceid": "0af7651916cd43dd8448eb211c80319c",
		"x-b3-spanid":  "b7ad6b7169203331",
		"x-b3-flags":   "1", // debug = sampled
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsSampled() {
		t.Error("flags=1 should be sampled")
	}
}

func TestExtract64BitTraceID(t *testing.T) {
	carrier := propagation.MapCarrier{
		"b3": "b7ad6b7169203331-b7ad6b7169203331-1",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Error("64-bit trace ID should be valid")
	}
}

func TestExtract64BitMultiHeader(t *testing.T) {
	carrier := propagation.MapCarrier{
		"x-b3-traceid": "b7ad6b7169203331",
		"x-b3-spanid":  "b7ad6b7169203331",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Error("64-bit trace ID should be valid")
	}
}

func TestExtractSingleHeaderWithParent(t *testing.T) {
	carrier := propagation.MapCarrier{
		"b3": "0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-1-0000000000000000",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Error("should be valid with parent span ID")
	}
}

func TestExtractSingleHeaderDeny(t *testing.T) {
	carrier := propagation.MapCarrier{"b3": "0"}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("b3=0 should not produce valid context")
	}
}

func TestExtractSingleHeaderAccept(t *testing.T) {
	carrier := propagation.MapCarrier{"b3": "1"}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("b3=1 should not produce valid context (no IDs)")
	}
}

func TestExtractInvalid(t *testing.T) {
	tests := []propagation.MapCarrier{
		{},
		{"b3": "invalid"},
		{"b3": "a"}, // too short to split
		{"b3": "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-b7ad6b7169203331-1"},
		{"b3": "0af7651916cd43dd8448eb211c80319c-zzzzzzzzzzzzzzzz-1"},
		{"x-b3-traceid": "bad", "x-b3-spanid": "b7ad6b7169203331"},
		{"x-b3-traceid": "0af7651916cd43dd8448eb211c80319c", "x-b3-spanid": "bad"},
		{"x-b3-traceid": "0af7651916cd43dd8448eb211c80319c"}, // missing span
	}
	for i, carrier := range tests {
		ctx := Propagator{}.Extract(context.Background(), carrier)
		sc := propagation.SpanContextFromContext(ctx)
		if sc.IsValid() {
			t.Errorf("test %d: should be invalid", i)
		}
	}
}

func TestInjectInvalid(t *testing.T) {
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(context.Background(), carrier)
	if carrier.Get("b3") != "" {
		t.Error("should not inject for invalid context")
	}
}

func TestFields(t *testing.T) {
	if len(Propagator{}.Fields()) != 6 {
		t.Errorf("expected 6 fields, got %d", len(Propagator{}.Fields()))
	}
}

func TestExtractNotSampledSingle(t *testing.T) {
	carrier := propagation.MapCarrier{
		"b3": "0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-0",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if sc.IsSampled() {
		t.Error("should not be sampled")
	}
}
