package propagation

import (
	"encoding/hex"
	"errors"
)

const (
	traceIDSize = 16
	spanIDSize  = 8
)

// ErrInvalidTraceID is returned when a trace ID is malformed.
var ErrInvalidTraceID = errors.New("invalid trace ID")

// ErrInvalidSpanID is returned when a span ID is malformed.
var ErrInvalidSpanID = errors.New("invalid span ID")

// TraceID is a 128-bit identifier for a trace.
type TraceID [traceIDSize]byte

// IsValid reports whether the trace ID is non-zero.
func (t TraceID) IsValid() bool {
	for _, b := range t {
		if b != 0 {
			return true
		}
	}
	return false
}

// String returns the hex-encoded representation of the trace ID.
func (t TraceID) String() string {
	return hex.EncodeToString(t[:])
}

// TraceIDFromHex parses a hex-encoded trace ID string.
func TraceIDFromHex(s string) (TraceID, error) {
	var tid TraceID
	if len(s) != traceIDSize*2 {
		return tid, ErrInvalidTraceID
	}
	_, err := hex.Decode(tid[:], []byte(s))
	if err != nil {
		return tid, ErrInvalidTraceID
	}
	return tid, nil
}

// SpanID is a 64-bit identifier for a span within a trace.
type SpanID [spanIDSize]byte

// IsValid reports whether the span ID is non-zero.
func (s SpanID) IsValid() bool {
	for _, b := range s {
		if b != 0 {
			return true
		}
	}
	return false
}

// String returns the hex-encoded representation of the span ID.
func (s SpanID) String() string {
	return hex.EncodeToString(s[:])
}

// SpanIDFromHex parses a hex-encoded span ID string.
func SpanIDFromHex(str string) (SpanID, error) {
	var sid SpanID
	if len(str) != spanIDSize*2 {
		return sid, ErrInvalidSpanID
	}
	_, err := hex.Decode(sid[:], []byte(str))
	if err != nil {
		return sid, ErrInvalidSpanID
	}
	return sid, nil
}
