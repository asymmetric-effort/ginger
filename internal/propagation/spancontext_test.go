package propagation

import "testing"

func TestSpanContextIsValid(t *testing.T) {
	validTID := TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	validSID := SpanID{1, 2, 3, 4, 5, 6, 7, 8}

	tests := []struct {
		name    string
		traceID TraceID
		spanID  SpanID
		valid   bool
	}{
		{"valid", validTID, validSID, true},
		{"zero trace", TraceID{}, validSID, false},
		{"zero span", validTID, SpanID{}, false},
		{"both zero", TraceID{}, SpanID{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSpanContext(tt.traceID, tt.spanID, 0, TraceState{}, false)
			if got := sc.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestSpanContextAccessors(t *testing.T) {
	tid := TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	sid := SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	flags := FlagsSampled
	ts, _ := NewTraceState(TraceStateEntry{Key: "vendor", Value: "val"})

	sc := NewSpanContext(tid, sid, flags, ts, true)

	if sc.TraceID() != tid {
		t.Error("TraceID mismatch")
	}
	if sc.SpanID() != sid {
		t.Error("SpanID mismatch")
	}
	if sc.TraceFlags() != flags {
		t.Error("TraceFlags mismatch")
	}
	if sc.TraceStateValue().Len() != 1 {
		t.Error("TraceState mismatch")
	}
	if !sc.IsRemote() {
		t.Error("expected remote=true")
	}
	if !sc.IsSampled() {
		t.Error("expected sampled=true")
	}
}

func TestSpanContextWithRemote(t *testing.T) {
	sc := NewSpanContext(TraceID{1}, SpanID{1}, 0, TraceState{}, false)
	if sc.IsRemote() {
		t.Error("expected remote=false initially")
	}
	sc2 := sc.WithRemote(true)
	if !sc2.IsRemote() {
		t.Error("expected remote=true after WithRemote")
	}
	if sc.IsRemote() {
		t.Error("original should not be modified")
	}
}

func TestSpanContextNotSampled(t *testing.T) {
	sc := NewSpanContext(TraceID{1}, SpanID{1}, 0, TraceState{}, false)
	if sc.IsSampled() {
		t.Error("expected not sampled")
	}
}

func TestTraceFlagsString(t *testing.T) {
	tests := []struct {
		flags    TraceFlags
		expected string
	}{
		{0x00, "00"},
		{0x01, "01"},
		{0x0f, "0f"},
		{0xff, "ff"},
		{0xab, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.flags.String(); got != tt.expected {
				t.Errorf("TraceFlags(%d).String() = %q, want %q", tt.flags, got, tt.expected)
			}
		})
	}
}

func TestTraceFlagsIsSampled(t *testing.T) {
	if !(FlagsSampled).IsSampled() {
		t.Error("FlagsSampled should be sampled")
	}
	if TraceFlags(0).IsSampled() {
		t.Error("zero flags should not be sampled")
	}
	if !TraceFlags(0x03).IsSampled() {
		t.Error("0x03 should be sampled (bit 0 set)")
	}
}

func TestTraceFlagsWithSampled(t *testing.T) {
	f := TraceFlags(0)
	f = f.WithSampled(true)
	if !f.IsSampled() {
		t.Error("expected sampled after WithSampled(true)")
	}
	f = f.WithSampled(false)
	if f.IsSampled() {
		t.Error("expected not sampled after WithSampled(false)")
	}
}

func TestEmptySpanContextIsInvalid(t *testing.T) {
	var sc SpanContext
	if sc.IsValid() {
		t.Error("empty SpanContext should not be valid")
	}
	if sc.IsSampled() {
		t.Error("empty SpanContext should not be sampled")
	}
	if sc.IsRemote() {
		t.Error("empty SpanContext should not be remote")
	}
}
