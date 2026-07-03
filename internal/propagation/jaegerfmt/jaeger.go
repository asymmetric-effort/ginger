package jaegerfmt

import (
	"context"
	"net/url"
	"strings"

	"github.com/asymmetric-effort/ginger/internal/propagation"
)

const (
	traceHeader   = "uber-trace-id"
	baggagePrefix = "uberctx-"
)

// Propagator implements Jaeger propagation format.
type Propagator struct{}

// Inject writes uber-trace-id and baggage headers.
func (p Propagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	flags := "0"
	if sc.IsSampled() {
		flags = "1"
	}
	if sc.TraceFlags()&0x02 != 0 {
		flags = "3" // debug + sampled
	}

	// uber-trace-id: {traceID}:{spanID}:{parentSpanID}:{flags}
	val := sc.TraceID().String() + ":" + sc.SpanID().String() + ":0:" + flags
	carrier.Set(traceHeader, val)

	// Baggage
	baggage := propagation.BaggageFromContext(ctx)
	for _, key := range baggage.Keys() {
		carrier.Set(baggagePrefix+key, url.QueryEscape(baggage.Get(key)))
	}
}

// Extract reads uber-trace-id and baggage headers.
func (p Propagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	val := carrier.Get(traceHeader)
	if val == "" {
		return ctx
	}

	sc := parseJaegerHeader(val)
	if !sc.IsValid() {
		return ctx
	}

	ctx = propagation.ContextWithSpanContext(ctx, sc)

	// Extract baggage
	baggage := propagation.NewBaggage()
	for _, key := range carrier.Keys() {
		lk := strings.ToLower(key)
		if strings.HasPrefix(lk, baggagePrefix) {
			baggageKey := lk[len(baggagePrefix):]
			val, err := url.QueryUnescape(carrier.Get(key))
			if err != nil {
				val = carrier.Get(key)
			}
			baggage, _ = baggage.Set(baggageKey, val)
		}
	}
	if baggage.Len() > 0 {
		ctx = propagation.ContextWithBaggage(ctx, baggage)
	}

	return ctx
}

// Fields returns the header names used by this propagator.
func (p Propagator) Fields() []string {
	return []string{traceHeader}
}

func parseJaegerHeader(val string) propagation.SpanContext {
	parts := strings.Split(val, ":")
	if len(parts) != 4 {
		return propagation.SpanContext{}
	}

	traceIDStr := parts[0]
	spanIDStr := parts[1]
	// parts[2] is parentSpanID — we don't need it for context
	flagsStr := parts[3]

	// Pad trace ID to 32 chars if it's 16 (64-bit)
	if len(traceIDStr) == 16 {
		traceIDStr = "0000000000000000" + traceIDStr
	}

	traceID, err := propagation.TraceIDFromHex(traceIDStr)
	if err != nil {
		return propagation.SpanContext{}
	}

	// Pad span ID to 16 chars if needed
	for len(spanIDStr) < 16 {
		spanIDStr = "0" + spanIDStr
	}

	spanID, err := propagation.SpanIDFromHex(spanIDStr)
	if err != nil {
		return propagation.SpanContext{}
	}

	var flags propagation.TraceFlags
	if flagsStr == "1" || flagsStr == "3" {
		flags = flags.WithSampled(true)
	}

	return propagation.NewSpanContext(traceID, spanID, flags, propagation.TraceState{}, true)
}
