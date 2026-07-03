package tracecontext

import (
	"context"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/propagation"
)

func TestInjectExtractRoundTrip(t *testing.T) {
	tid, _ := propagation.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := propagation.SpanIDFromHex("b7ad6b7169203331")
	sc := propagation.NewSpanContext(tid, sid, propagation.FlagsSampled, propagation.TraceState{}, false)

	ctx := propagation.ContextWithSpanContext(context.Background(), sc)
	carrier := propagation.MapCarrier{}

	p := Propagator{}
	p.Inject(ctx, carrier)

	expected := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if carrier.Get("traceparent") != expected {
		t.Errorf("traceparent = %q, want %q", carrier.Get("traceparent"), expected)
	}

	ctx2 := p.Extract(context.Background(), carrier)
	sc2 := propagation.SpanContextFromContext(ctx2)
	if !sc2.IsValid() {
		t.Fatal("extracted context should be valid")
	}
	if sc2.TraceID() != tid {
		t.Errorf("TraceID = %s", sc2.TraceID())
	}
	if sc2.SpanID() != sid {
		t.Errorf("SpanID = %s", sc2.SpanID())
	}
	if !sc2.IsSampled() {
		t.Error("expected sampled")
	}
	if !sc2.IsRemote() {
		t.Error("expected remote")
	}
}

func TestInjectNotSampled(t *testing.T) {
	tid, _ := propagation.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := propagation.SpanIDFromHex("b7ad6b7169203331")
	sc := propagation.NewSpanContext(tid, sid, 0, propagation.TraceState{}, false)

	ctx := propagation.ContextWithSpanContext(context.Background(), sc)
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(ctx, carrier)

	if tp := carrier.Get("traceparent"); tp[53:] != "00" {
		t.Errorf("flags should be 00, got %q", tp[53:])
	}
}

func TestInjectWithTraceState(t *testing.T) {
	tid, _ := propagation.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := propagation.SpanIDFromHex("b7ad6b7169203331")
	ts, _ := propagation.NewTraceState(propagation.TraceStateEntry{Key: "vendor", Value: "val"})
	sc := propagation.NewSpanContext(tid, sid, propagation.FlagsSampled, ts, false)

	ctx := propagation.ContextWithSpanContext(context.Background(), sc)
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(ctx, carrier)

	if carrier.Get("tracestate") != "vendor=val" {
		t.Errorf("tracestate = %q", carrier.Get("tracestate"))
	}
}

func TestExtractWithTraceState(t *testing.T) {
	carrier := propagation.MapCarrier{
		"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		"tracestate":  "vendor=val",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if sc.TraceStateValue().Get("vendor") != "val" {
		t.Error("tracestate not extracted")
	}
}

func TestExtractInvalidTraceState(t *testing.T) {
	carrier := propagation.MapCarrier{
		"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		"tracestate":  "invalid_no_equals",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	// Should still have valid span context, just no tracestate
	if !sc.IsValid() {
		t.Error("should be valid despite bad tracestate")
	}
}

func TestInjectInvalid(t *testing.T) {
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(context.Background(), carrier) // empty context
	if carrier.Get("traceparent") != "" {
		t.Error("should not inject for invalid context")
	}
}

func TestExtractEmpty(t *testing.T) {
	ctx := Propagator{}.Extract(context.Background(), propagation.MapCarrier{})
	sc := propagation.SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("should return invalid for empty carrier")
	}
}

func TestExtractMalformed(t *testing.T) {
	tests := []string{
		"invalid",
		"00-short-short-00",
		"01-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", // wrong version format
		"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-zz", // bad flags
		"00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-b7ad6b7169203331-01", // bad trace ID
		"00-0af7651916cd43dd8448eb211c80319c-zzzzzzzzzzzzzzzz-01", // bad span ID
	}
	for _, tp := range tests {
		carrier := propagation.MapCarrier{"traceparent": tp}
		ctx := Propagator{}.Extract(context.Background(), carrier)
		sc := propagation.SpanContextFromContext(ctx)
		if sc.IsValid() {
			t.Errorf("should be invalid for %q", tp)
		}
	}
}

func TestFields(t *testing.T) {
	fields := Propagator{}.Fields()
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestHexValCases(t *testing.T) {
	// Uppercase hex in flags
	carrier := propagation.MapCarrier{
		"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-0A",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Error("should handle uppercase hex flags")
	}
}
