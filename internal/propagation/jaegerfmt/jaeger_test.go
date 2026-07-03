package jaegerfmt

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
	Propagator{}.Inject(ctx, carrier)

	val := carrier.Get("uber-trace-id")
	if val == "" {
		t.Fatal("uber-trace-id not set")
	}
	expected := "0af7651916cd43dd8448eb211c80319c:b7ad6b7169203331:0:1"
	if val != expected {
		t.Errorf("uber-trace-id = %q, want %q", val, expected)
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

	val := carrier.Get("uber-trace-id")
	if val[len(val)-1] != '0' {
		t.Errorf("flags should be 0, got %q", val)
	}
}

func TestInjectDebugFlag(t *testing.T) {
	tid, _ := propagation.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := propagation.SpanIDFromHex("b7ad6b7169203331")
	sc := propagation.NewSpanContext(tid, sid, propagation.TraceFlags(0x03), propagation.TraceState{}, false) // sampled + debug

	ctx := propagation.ContextWithSpanContext(context.Background(), sc)
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(ctx, carrier)

	val := carrier.Get("uber-trace-id")
	if val[len(val)-1] != '3' {
		t.Errorf("debug flags should be 3, got %q", val)
	}
}

func TestInjectWithBaggage(t *testing.T) {
	tid, _ := propagation.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := propagation.SpanIDFromHex("b7ad6b7169203331")
	sc := propagation.NewSpanContext(tid, sid, propagation.FlagsSampled, propagation.TraceState{}, false)

	ctx := propagation.ContextWithSpanContext(context.Background(), sc)
	b := propagation.NewBaggage()
	b, _ = b.Set("user", "test value")
	ctx = propagation.ContextWithBaggage(ctx, b)

	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(ctx, carrier)

	if carrier.Get("uberctx-user") != "test+value" {
		t.Errorf("baggage = %q", carrier.Get("uberctx-user"))
	}
}

func TestExtractWithBaggage(t *testing.T) {
	carrier := propagation.MapCarrier{
		"uber-trace-id": "0af7651916cd43dd8448eb211c80319c:b7ad6b7169203331:0:1",
		"uberctx-user":  "test+value",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	b := propagation.BaggageFromContext(ctx)
	if b.Get("user") != "test value" {
		t.Errorf("baggage user = %q", b.Get("user"))
	}
}

func TestExtract64BitTraceID(t *testing.T) {
	carrier := propagation.MapCarrier{
		"uber-trace-id": "b7ad6b7169203331:b7ad6b7169203331:0:1",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Error("64-bit trace ID should be valid")
	}
}

func TestExtractInvalid(t *testing.T) {
	tests := []string{
		"",
		"invalid",
		"a:b:c", // too few parts
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz:b7ad6b7169203331:0:1", // bad trace ID
		"0af7651916cd43dd8448eb211c80319c:zzzzzzzzzzzzzzzz:0:1", // bad span ID
	}
	for _, val := range tests {
		carrier := propagation.MapCarrier{"uber-trace-id": val}
		ctx := Propagator{}.Extract(context.Background(), carrier)
		sc := propagation.SpanContextFromContext(ctx)
		if val != "" && sc.IsValid() {
			t.Errorf("should be invalid for %q", val)
		}
	}
}

func TestInjectInvalid(t *testing.T) {
	carrier := propagation.MapCarrier{}
	Propagator{}.Inject(context.Background(), carrier)
	if carrier.Get("uber-trace-id") != "" {
		t.Error("should not inject for invalid context")
	}
}

func TestFields(t *testing.T) {
	if len(Propagator{}.Fields()) != 1 {
		t.Error("expected 1 field")
	}
}

func TestExtractDebugFlag(t *testing.T) {
	carrier := propagation.MapCarrier{
		"uber-trace-id": "0af7651916cd43dd8448eb211c80319c:b7ad6b7169203331:0:3",
	}
	ctx := Propagator{}.Extract(context.Background(), carrier)
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsSampled() {
		t.Error("debug flag 3 should be sampled")
	}
}
