package propagation

import (
	"context"
	"testing"
)

func TestContextWithSpanContext(t *testing.T) {
	tid := TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	sid := SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	sc := NewSpanContext(tid, sid, FlagsSampled, TraceState{}, true)

	ctx := context.Background()
	ctx = ContextWithSpanContext(ctx, sc)

	got := SpanContextFromContext(ctx)
	if got.TraceID() != tid {
		t.Error("TraceID mismatch after context round-trip")
	}
	if got.SpanID() != sid {
		t.Error("SpanID mismatch after context round-trip")
	}
	if !got.IsSampled() {
		t.Error("expected sampled after round-trip")
	}
	if !got.IsRemote() {
		t.Error("expected remote after round-trip")
	}
}

func TestSpanContextFromContextEmpty(t *testing.T) {
	ctx := context.Background()
	sc := SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("empty context should return invalid SpanContext")
	}
}

func TestContextWithBaggage(t *testing.T) {
	b := NewBaggage()
	b, _ = b.Set("key", "value")

	ctx := context.Background()
	ctx = ContextWithBaggage(ctx, b)

	got := BaggageFromContext(ctx)
	if got.Get("key") != "value" {
		t.Errorf("Baggage round-trip: Get(key) = %q, want value", got.Get("key"))
	}
}

func TestBaggageFromContextEmpty(t *testing.T) {
	ctx := context.Background()
	b := BaggageFromContext(ctx)
	if b.Len() != 0 {
		t.Error("empty context should return empty Baggage")
	}
}

func TestContextBothSpanContextAndBaggage(t *testing.T) {
	tid := TraceID{1}
	sid := SpanID{1}
	sc := NewSpanContext(tid, sid, 0, TraceState{}, false)
	b := NewBaggage()
	b, _ = b.Set("k", "v")

	ctx := context.Background()
	ctx = ContextWithSpanContext(ctx, sc)
	ctx = ContextWithBaggage(ctx, b)

	gotSC := SpanContextFromContext(ctx)
	gotB := BaggageFromContext(ctx)

	if gotSC.TraceID() != tid {
		t.Error("SpanContext TraceID lost")
	}
	if gotB.Get("k") != "v" {
		t.Error("Baggage value lost")
	}
}

func TestContextOverwriteSpanContext(t *testing.T) {
	sc1 := NewSpanContext(TraceID{1}, SpanID{1}, 0, TraceState{}, false)
	sc2 := NewSpanContext(TraceID{2}, SpanID{2}, FlagsSampled, TraceState{}, true)

	ctx := context.Background()
	ctx = ContextWithSpanContext(ctx, sc1)
	ctx = ContextWithSpanContext(ctx, sc2)

	got := SpanContextFromContext(ctx)
	if got.TraceID() != (TraceID{2}) {
		t.Error("expected second SpanContext to win")
	}
	if !got.IsSampled() {
		t.Error("expected sampled from second context")
	}
}
