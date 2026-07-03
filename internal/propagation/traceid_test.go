package propagation

import "testing"

func TestTraceIDIsValid(t *testing.T) {
	var zero TraceID
	if zero.IsValid() {
		t.Error("zero TraceID should not be valid")
	}

	nonZero := TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if !nonZero.IsValid() {
		t.Error("non-zero TraceID should be valid")
	}

	full := TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	if !full.IsValid() {
		t.Error("full TraceID should be valid")
	}
}

func TestTraceIDString(t *testing.T) {
	tid := TraceID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
	expected := "0123456789abcdef0123456789abcdef"
	if got := tid.String(); got != expected {
		t.Errorf("TraceID.String() = %q, want %q", got, expected)
	}
}

func TestTraceIDFromHex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "0123456789abcdef0123456789abcdef", false},
		{"valid uppercase", "0123456789ABCDEF0123456789ABCDEF", false},
		{"too short", "0123456789abcdef", true},
		{"too long", "0123456789abcdef0123456789abcdef00", true},
		{"invalid hex", "xyz3456789abcdef0123456789abcdef", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tid, err := TraceIDFromHex(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if !tid.IsValid() {
				t.Error("parsed TraceID should be valid")
			}
		})
	}
}

func TestTraceIDFromHexRoundTrip(t *testing.T) {
	const hexStr = "0123456789abcdef0123456789abcdef"
	tid, err := TraceIDFromHex(hexStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tid.String(); got != hexStr {
		t.Errorf("round trip: got %q, want %q", got, hexStr)
	}
}

func TestSpanIDIsValid(t *testing.T) {
	var zero SpanID
	if zero.IsValid() {
		t.Error("zero SpanID should not be valid")
	}

	nonZero := SpanID{0, 0, 0, 0, 0, 0, 0, 1}
	if !nonZero.IsValid() {
		t.Error("non-zero SpanID should be valid")
	}
}

func TestSpanIDString(t *testing.T) {
	sid := SpanID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
	expected := "0123456789abcdef"
	if got := sid.String(); got != expected {
		t.Errorf("SpanID.String() = %q, want %q", got, expected)
	}
}

func TestSpanIDFromHex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "0123456789abcdef", false},
		{"too short", "01234567", true},
		{"too long", "0123456789abcdef00", true},
		{"invalid hex", "xyz3456789abcdef", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sid, err := SpanIDFromHex(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if !sid.IsValid() {
				t.Error("parsed SpanID should be valid")
			}
		})
	}
}

func TestSpanIDFromHexRoundTrip(t *testing.T) {
	const hexStr = "0123456789abcdef"
	sid, err := SpanIDFromHex(hexStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := sid.String(); got != hexStr {
		t.Errorf("round trip: got %q, want %q", got, hexStr)
	}
}

func TestZeroTraceIDString(t *testing.T) {
	var tid TraceID
	expected := "00000000000000000000000000000000"
	if got := tid.String(); got != expected {
		t.Errorf("zero TraceID.String() = %q, want %q", got, expected)
	}
}

func TestZeroSpanIDString(t *testing.T) {
	var sid SpanID
	expected := "0000000000000000"
	if got := sid.String(); got != expected {
		t.Errorf("zero SpanID.String() = %q, want %q", got, expected)
	}
}
