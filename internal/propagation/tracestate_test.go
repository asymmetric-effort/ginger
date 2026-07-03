package propagation

import "testing"

func TestTraceStateEmpty(t *testing.T) {
	ts := TraceState{}
	if ts.Len() != 0 {
		t.Error("empty TraceState should have length 0")
	}
	if got := ts.String(); got != "" {
		t.Errorf("empty TraceState.String() = %q, want empty", got)
	}
	if got := ts.Get("key"); got != "" {
		t.Errorf("Get on empty should return empty, got %q", got)
	}
}

func TestNewTraceState(t *testing.T) {
	ts, err := NewTraceState(
		TraceStateEntry{Key: "vendor1", Value: "val1"},
		TraceStateEntry{Key: "vendor2", Value: "val2"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Len() != 2 {
		t.Errorf("expected length 2, got %d", ts.Len())
	}
	if got := ts.Get("vendor1"); got != "val1" {
		t.Errorf("Get(vendor1) = %q, want val1", got)
	}
	if got := ts.Get("vendor2"); got != "val2" {
		t.Errorf("Get(vendor2) = %q, want val2", got)
	}
}

func TestNewTraceStateTooManyEntries(t *testing.T) {
	entries := make([]TraceStateEntry, traceStateMaxEntries+1)
	for i := range entries {
		entries[i] = TraceStateEntry{Key: "k", Value: "v"}
	}
	_, err := NewTraceState(entries...)
	if err != ErrTraceStateTooManyEntries {
		t.Errorf("expected ErrTraceStateTooManyEntries, got %v", err)
	}
}

func TestParseTraceState(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantErr  bool
		wantStr  string
	}{
		{"empty", "", 0, false, ""},
		{"single", "vendor=value", 1, false, "vendor=value"},
		{"multiple", "v1=a,v2=b,v3=c", 3, false, "v1=a,v2=b,v3=c"},
		{"with spaces", " v1=a , v2=b ", 2, false, "v1=a,v2=b"},
		{"trailing comma", "v1=a,", 1, false, "v1=a"},
		{"no equals", "invalid", 0, true, ""},
		{"empty key", "=value", 0, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := ParseTraceState(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ts.Len() != tt.wantLen {
				t.Errorf("Len() = %d, want %d", ts.Len(), tt.wantLen)
			}
			if got := ts.String(); got != tt.wantStr {
				t.Errorf("String() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestTraceStateSet(t *testing.T) {
	ts, _ := NewTraceState(
		TraceStateEntry{Key: "a", Value: "1"},
		TraceStateEntry{Key: "b", Value: "2"},
	)

	// Update existing key — should move to front
	ts2, err := ts.Set("b", "3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts2.Len() != 2 {
		t.Errorf("expected length 2, got %d", ts2.Len())
	}
	entries := ts2.Entries()
	if entries[0].Key != "b" || entries[0].Value != "3" {
		t.Errorf("expected b=3 at front, got %s=%s", entries[0].Key, entries[0].Value)
	}
	if entries[1].Key != "a" || entries[1].Value != "1" {
		t.Errorf("expected a=1 at second, got %s=%s", entries[1].Key, entries[1].Value)
	}

	// Add new key
	ts3, err := ts.Set("c", "4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts3.Len() != 3 {
		t.Errorf("expected length 3, got %d", ts3.Len())
	}
	if got := ts3.Get("c"); got != "4" {
		t.Errorf("Get(c) = %q, want 4", got)
	}
}

func TestTraceStateDelete(t *testing.T) {
	ts, _ := NewTraceState(
		TraceStateEntry{Key: "a", Value: "1"},
		TraceStateEntry{Key: "b", Value: "2"},
		TraceStateEntry{Key: "c", Value: "3"},
	)

	ts2 := ts.Delete("b")
	if ts2.Len() != 2 {
		t.Errorf("expected length 2, got %d", ts2.Len())
	}
	if got := ts2.Get("b"); got != "" {
		t.Errorf("deleted key should return empty, got %q", got)
	}
	if got := ts2.Get("a"); got != "1" {
		t.Errorf("Get(a) = %q, want 1", got)
	}

	// Delete non-existent key
	ts3 := ts.Delete("nonexistent")
	if ts3.Len() != 3 {
		t.Errorf("delete nonexistent should not change length")
	}
}

func TestTraceStateEntries(t *testing.T) {
	ts, _ := NewTraceState(
		TraceStateEntry{Key: "a", Value: "1"},
		TraceStateEntry{Key: "b", Value: "2"},
	)
	entries := ts.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Verify it's a copy
	entries[0].Key = "modified"
	if ts.Get("modified") != "" {
		t.Error("Entries should return a copy")
	}
}

func TestNewTraceStateTooLarge(t *testing.T) {
	largeVal := make([]byte, baggageMaxBytes)
	for i := range largeVal {
		largeVal[i] = 'x'
	}
	_, err := NewTraceState(TraceStateEntry{Key: "k", Value: string(largeVal)})
	if err != ErrTraceStateTooLarge {
		t.Errorf("expected ErrTraceStateTooLarge, got %v", err)
	}
}

func TestParseTraceStateTooMany(t *testing.T) {
	// Build a header with more than 32 entries
	parts := make([]string, traceStateMaxEntries+1)
	for i := range parts {
		parts[i] = "k=v"
	}
	header := ""
	for i, p := range parts {
		if i > 0 {
			header += ","
		}
		header += p
	}
	_, err := ParseTraceState(header)
	if err != ErrTraceStateTooManyEntries {
		t.Errorf("expected ErrTraceStateTooManyEntries, got %v", err)
	}
}

func TestParseTraceStateTooLarge(t *testing.T) {
	largeVal := make([]byte, traceStateMaxBytes)
	for i := range largeVal {
		largeVal[i] = 'x'
	}
	_, err := ParseTraceState("k=" + string(largeVal))
	if err != ErrTraceStateTooLarge {
		t.Errorf("expected ErrTraceStateTooLarge, got %v", err)
	}
}

func TestTraceStateSetTooMany(t *testing.T) {
	entries := make([]TraceStateEntry, traceStateMaxEntries)
	for i := range entries {
		entries[i] = TraceStateEntry{Key: string(rune('a' + i%26)), Value: "v"}
	}
	ts, err := NewTraceState(entries...)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Adding a new (non-duplicate) key should exceed max
	_, err = ts.Set("newkey_unique", "val")
	if err != ErrTraceStateTooManyEntries {
		t.Errorf("expected ErrTraceStateTooManyEntries, got %v", err)
	}
}

func TestTraceStateSetTooLarge(t *testing.T) {
	ts, _ := NewTraceState(TraceStateEntry{Key: "a", Value: "1"})
	largeVal := make([]byte, traceStateMaxBytes)
	for i := range largeVal {
		largeVal[i] = 'x'
	}
	_, err := ts.Set("big", string(largeVal))
	if err != ErrTraceStateTooLarge {
		t.Errorf("expected ErrTraceStateTooLarge, got %v", err)
	}
}

func TestTraceStateGetMissing(t *testing.T) {
	ts, _ := NewTraceState(TraceStateEntry{Key: "a", Value: "1"})
	if got := ts.Get("missing"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}
}
