package tracecontext

import (
	"context"

	"github.com/asymmetric-effort/ginger/internal/propagation"
)

const (
	traceparentHeader = "traceparent"
	tracestateHeader  = "tracestate"
	version           = "00"
)

// Propagator implements W3C TraceContext propagation.
type Propagator struct{}

// Inject writes traceparent and tracestate headers to the carrier.
func (p Propagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	sc := propagation.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	// traceparent: 00-<traceID>-<spanID>-<flags>
	tp := version + "-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-" + sc.TraceFlags().String()
	carrier.Set(traceparentHeader, tp)

	// tracestate
	ts := sc.TraceStateValue()
	if ts.Len() > 0 {
		carrier.Set(tracestateHeader, ts.String())
	}
}

// Extract reads traceparent and tracestate headers from the carrier.
func (p Propagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	tp := carrier.Get(traceparentHeader)
	if tp == "" {
		return ctx
	}

	sc := parseTraceparent(tp)
	if !sc.IsValid() {
		return ctx
	}

	// Parse tracestate if present
	tsHeader := carrier.Get(tracestateHeader)
	if tsHeader != "" {
		ts, err := propagation.ParseTraceState(tsHeader)
		if err == nil {
			sc = propagation.NewSpanContext(sc.TraceID(), sc.SpanID(), sc.TraceFlags(), ts, true)
		}
	}

	return propagation.ContextWithSpanContext(ctx, sc)
}

// Fields returns the header names used by this propagator.
func (p Propagator) Fields() []string {
	return []string{traceparentHeader, tracestateHeader}
}

func parseTraceparent(tp string) propagation.SpanContext {
	// Format: version-traceID-spanID-flags
	// 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
	if len(tp) != 55 {
		return propagation.SpanContext{}
	}
	if tp[0] != '0' || tp[1] != '0' || tp[2] != '-' || tp[35] != '-' || tp[52] != '-' {
		return propagation.SpanContext{}
	}

	traceID, err := propagation.TraceIDFromHex(tp[3:35])
	if err != nil {
		return propagation.SpanContext{}
	}
	spanID, err := propagation.SpanIDFromHex(tp[36:52])
	if err != nil {
		return propagation.SpanContext{}
	}

	// Parse flags (2 hex chars)
	flagByte := hexToByte(tp[53], tp[54])
	if flagByte < 0 {
		return propagation.SpanContext{}
	}

	return propagation.NewSpanContext(traceID, spanID, propagation.TraceFlags(flagByte), propagation.TraceState{}, true)
}

func hexToByte(hi, lo byte) int16 {
	h := hexVal(hi)
	l := hexVal(lo)
	if h < 0 || l < 0 {
		return -1
	}
	return int16(h<<4 | l)
}

func hexVal(c byte) int8 {
	switch {
	case c >= '0' && c <= '9':
		return int8(c - '0')
	case c >= 'a' && c <= 'f':
		return int8(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int8(c - 'A' + 10)
	default:
		return -1
	}
}
