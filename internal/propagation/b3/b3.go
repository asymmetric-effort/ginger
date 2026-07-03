package b3

import (
	"context"
	"strings"

	"github.com/asymmetric-effort/ginger/internal/propagation"
)

const (
	singleHeader = "b3"
	traceHeader  = "x-b3-traceid"
	spanHeader   = "x-b3-spanid"
	parentHeader = "x-b3-parentspanid"
	sampledHdr   = "x-b3-sampled"
	flagsHeader  = "x-b3-flags"
)

// Propagator implements B3 propagation (single-header and multi-header).
type Propagator struct{}

// Inject writes B3 single-header format.
func (p Propagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	sampled := "0"
	if sc.IsSampled() {
		sampled = "1"
	}

	// Single-header: {traceID}-{spanID}-{sampled}
	val := sc.TraceID().String() + "-" + sc.SpanID().String() + "-" + sampled
	carrier.Set(singleHeader, val)
}

// Extract reads B3 headers, trying single-header first then multi-header.
func (p Propagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	// Try single-header first
	single := carrier.Get(singleHeader)
	if single != "" {
		sc := parseSingleHeader(single)
		if sc.IsValid() {
			return propagation.ContextWithSpanContext(ctx, sc)
		}
	}

	// Fall back to multi-header
	sc := parseMultiHeader(carrier)
	if sc.IsValid() {
		return propagation.ContextWithSpanContext(ctx, sc)
	}

	return ctx
}

// Fields returns the header names used by this propagator.
func (p Propagator) Fields() []string {
	return []string{singleHeader, traceHeader, spanHeader, parentHeader, sampledHdr, flagsHeader}
}

func parseSingleHeader(val string) propagation.SpanContext {
	// Formats:
	// {traceID}-{spanID}-{sampled}
	// {traceID}-{spanID}-{sampled}-{parentSpanID}
	// 0 (deny)
	// 1 (accept, no IDs)
	if val == "0" || val == "1" {
		return propagation.SpanContext{} // no trace context, just sampling decision
	}

	parts := strings.Split(val, "-")
	if len(parts) < 2 {
		return propagation.SpanContext{}
	}

	traceIDStr := parts[0]
	spanIDStr := parts[1]

	// Support 64-bit trace IDs
	if len(traceIDStr) == 16 {
		traceIDStr = "0000000000000000" + traceIDStr
	}

	traceID, err := propagation.TraceIDFromHex(traceIDStr)
	if err != nil {
		return propagation.SpanContext{}
	}
	spanID, err := propagation.SpanIDFromHex(spanIDStr)
	if err != nil {
		return propagation.SpanContext{}
	}

	var flags propagation.TraceFlags
	if len(parts) >= 3 && parts[2] == "1" {
		flags = flags.WithSampled(true)
	}

	return propagation.NewSpanContext(traceID, spanID, flags, propagation.TraceState{}, true)
}

func parseMultiHeader(carrier propagation.TextMapCarrier) propagation.SpanContext {
	traceIDStr := carrier.Get(traceHeader)
	spanIDStr := carrier.Get(spanHeader)
	if traceIDStr == "" || spanIDStr == "" {
		return propagation.SpanContext{}
	}

	// Support 64-bit trace IDs
	if len(traceIDStr) == 16 {
		traceIDStr = "0000000000000000" + traceIDStr
	}

	traceID, err := propagation.TraceIDFromHex(traceIDStr)
	if err != nil {
		return propagation.SpanContext{}
	}
	spanID, err := propagation.SpanIDFromHex(spanIDStr)
	if err != nil {
		return propagation.SpanContext{}
	}

	var flags propagation.TraceFlags
	sampled := carrier.Get(sampledHdr)
	flagsVal := carrier.Get(flagsHeader)
	if sampled == "1" || flagsVal == "1" {
		flags = flags.WithSampled(true)
	}

	return propagation.NewSpanContext(traceID, spanID, flags, propagation.TraceState{}, true)
}
