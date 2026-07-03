package propagation

// TraceFlags contains flags associated with a trace.
type TraceFlags byte

const (
	// FlagsSampled indicates the trace is sampled.
	FlagsSampled TraceFlags = 0x01
)

// IsSampled reports whether the sampled flag is set.
func (f TraceFlags) IsSampled() bool {
	return f&FlagsSampled == FlagsSampled
}

// WithSampled returns a copy of the flags with the sampled bit set or cleared.
func (f TraceFlags) WithSampled(sampled bool) TraceFlags {
	if sampled {
		return f | FlagsSampled
	}
	return f &^ FlagsSampled
}

// String returns the hex-encoded representation of the flags.
func (f TraceFlags) String() string {
	const hexDigits = "0123456789abcdef"
	return string([]byte{hexDigits[f>>4], hexDigits[f&0x0f]})
}

// SpanContext contains the state that propagates across process boundaries.
type SpanContext struct {
	traceID    TraceID
	spanID     SpanID
	traceFlags TraceFlags
	traceState TraceState
	isRemote   bool
}

// NewSpanContext creates a SpanContext from the given fields.
func NewSpanContext(traceID TraceID, spanID SpanID, flags TraceFlags, state TraceState, remote bool) SpanContext {
	return SpanContext{
		traceID:    traceID,
		spanID:     spanID,
		traceFlags: flags,
		traceState: state,
		isRemote:   remote,
	}
}

// TraceID returns the trace ID.
func (sc SpanContext) TraceID() TraceID {
	return sc.traceID
}

// SpanID returns the span ID.
func (sc SpanContext) SpanID() SpanID {
	return sc.spanID
}

// TraceFlags returns the trace flags.
func (sc SpanContext) TraceFlags() TraceFlags {
	return sc.traceFlags
}

// TraceStateValue returns the trace state.
func (sc SpanContext) TraceStateValue() TraceState {
	return sc.traceState
}

// IsRemote reports whether this context was received from a remote parent.
func (sc SpanContext) IsRemote() bool {
	return sc.isRemote
}

// IsValid reports whether the span context has a valid trace ID and span ID.
func (sc SpanContext) IsValid() bool {
	return sc.traceID.IsValid() && sc.spanID.IsValid()
}

// IsSampled reports whether the sampled flag is set.
func (sc SpanContext) IsSampled() bool {
	return sc.traceFlags.IsSampled()
}

// WithRemote returns a copy of the SpanContext with the remote flag set.
func (sc SpanContext) WithRemote(remote bool) SpanContext {
	sc.isRemote = remote
	return sc
}
