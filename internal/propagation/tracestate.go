package propagation

import (
	"errors"
	"strings"
)

const (
	traceStateMaxEntries  = 32
	traceStateMaxBytes    = 8192
	traceStateDelimiter   = ","
	traceStateKeyValueSep = "="
)

// ErrTraceStateTooLarge is returned when a trace state exceeds the maximum allowed size.
var ErrTraceStateTooLarge = errors.New("trace state exceeds maximum size")

// ErrTraceStateTooManyEntries is returned when a trace state has too many entries.
var ErrTraceStateTooManyEntries = errors.New("trace state has too many entries")

// ErrTraceStateInvalidEntry is returned when a trace state entry is malformed.
var ErrTraceStateInvalidEntry = errors.New("trace state entry is malformed")

// TraceStateEntry is a single key-value pair in a trace state.
type TraceStateEntry struct {
	Key   string
	Value string
}

// TraceState carries vendor-specific trace identification data across tracing systems.
type TraceState struct {
	entries []TraceStateEntry
}

// NewTraceState creates a TraceState from the given entries.
func NewTraceState(entries ...TraceStateEntry) (TraceState, error) {
	if len(entries) > traceStateMaxEntries {
		return TraceState{}, ErrTraceStateTooManyEntries
	}
	ts := TraceState{entries: make([]TraceStateEntry, len(entries))}
	copy(ts.entries, entries)
	if ts.byteSize() > traceStateMaxBytes {
		return TraceState{}, ErrTraceStateTooLarge
	}
	return ts, nil
}

// ParseTraceState parses a W3C tracestate header value.
func ParseTraceState(header string) (TraceState, error) {
	if header == "" {
		return TraceState{}, nil
	}
	parts := strings.Split(header, traceStateDelimiter)
	if len(parts) > traceStateMaxEntries {
		return TraceState{}, ErrTraceStateTooManyEntries
	}
	entries := make([]TraceStateEntry, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		idx := strings.Index(trimmed, traceStateKeyValueSep)
		if idx < 1 {
			return TraceState{}, ErrTraceStateInvalidEntry
		}
		entries = append(entries, TraceStateEntry{
			Key:   trimmed[:idx],
			Value: trimmed[idx+1:],
		})
	}
	ts := TraceState{entries: entries}
	if ts.byteSize() > traceStateMaxBytes {
		return TraceState{}, ErrTraceStateTooLarge
	}
	return ts, nil
}

// Get returns the value for the given key, or empty string if not found.
func (ts TraceState) Get(key string) string {
	for i := range ts.entries {
		if ts.entries[i].Key == key {
			return ts.entries[i].Value
		}
	}
	return ""
}

// Set returns a new TraceState with the key set to value. If the key already exists,
// it is moved to the front and updated. New keys are prepended.
func (ts TraceState) Set(key, value string) (TraceState, error) {
	newEntries := make([]TraceStateEntry, 0, len(ts.entries)+1)
	newEntries = append(newEntries, TraceStateEntry{Key: key, Value: value})
	for i := range ts.entries {
		if ts.entries[i].Key != key {
			newEntries = append(newEntries, ts.entries[i])
		}
	}
	if len(newEntries) > traceStateMaxEntries {
		return TraceState{}, ErrTraceStateTooManyEntries
	}
	result := TraceState{entries: newEntries}
	if result.byteSize() > traceStateMaxBytes {
		return TraceState{}, ErrTraceStateTooLarge
	}
	return result, nil
}

// Delete returns a new TraceState with the key removed.
func (ts TraceState) Delete(key string) TraceState {
	newEntries := make([]TraceStateEntry, 0, len(ts.entries))
	for i := range ts.entries {
		if ts.entries[i].Key != key {
			newEntries = append(newEntries, ts.entries[i])
		}
	}
	return TraceState{entries: newEntries}
}

// Len returns the number of entries.
func (ts TraceState) Len() int {
	return len(ts.entries)
}

// Entries returns a copy of the entries.
func (ts TraceState) Entries() []TraceStateEntry {
	out := make([]TraceStateEntry, len(ts.entries))
	copy(out, ts.entries)
	return out
}

// String returns the W3C tracestate header value.
func (ts TraceState) String() string {
	if len(ts.entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range ts.entries {
		if i > 0 {
			b.WriteString(traceStateDelimiter)
		}
		b.WriteString(e.Key)
		b.WriteString(traceStateKeyValueSep)
		b.WriteString(e.Value)
	}
	return b.String()
}

func (ts TraceState) byteSize() int {
	size := 0
	for i, e := range ts.entries {
		if i > 0 {
			size++ // delimiter
		}
		size += len(e.Key) + 1 + len(e.Value) // key=value
	}
	return size
}
