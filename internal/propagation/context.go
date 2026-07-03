package propagation

import "context"

type contextKeyType int

const (
	spanContextKey contextKeyType = iota
	baggageKey
)

// ContextWithSpanContext returns a new context with the given SpanContext.
func ContextWithSpanContext(ctx context.Context, sc SpanContext) context.Context {
	return context.WithValue(ctx, spanContextKey, sc)
}

// SpanContextFromContext extracts the SpanContext from the context.
// Returns an empty SpanContext if none is set.
func SpanContextFromContext(ctx context.Context) SpanContext {
	sc, ok := ctx.Value(spanContextKey).(SpanContext)
	if !ok {
		return SpanContext{}
	}
	return sc
}

// ContextWithBaggage returns a new context with the given Baggage.
func ContextWithBaggage(ctx context.Context, b Baggage) context.Context {
	return context.WithValue(ctx, baggageKey, b)
}

// BaggageFromContext extracts the Baggage from the context.
// Returns an empty Baggage if none is set.
func BaggageFromContext(ctx context.Context) Baggage {
	b, ok := ctx.Value(baggageKey).(Baggage)
	if !ok {
		return NewBaggage()
	}
	return b
}
