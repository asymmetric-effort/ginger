package propagation

import "errors"

const baggageMaxBytes = 8192

// ErrBaggageTooLarge is returned when baggage exceeds the maximum allowed size.
var ErrBaggageTooLarge = errors.New("baggage exceeds maximum size of 8192 bytes")

// Baggage is an ordered key-value string map for cross-cutting concerns.
type Baggage struct {
	entries []baggageEntry
}

type baggageEntry struct {
	key   string
	value string
}

// NewBaggage creates an empty Baggage.
func NewBaggage() Baggage {
	return Baggage{}
}

// Get returns the value for the given key, or empty string if not found.
func (b Baggage) Get(key string) string {
	for i := range b.entries {
		if b.entries[i].key == key {
			return b.entries[i].value
		}
	}
	return ""
}

// Set returns a new Baggage with the key set to value. If the key exists, its value is updated.
func (b Baggage) Set(key, value string) (Baggage, error) {
	newEntries := make([]baggageEntry, 0, len(b.entries)+1)
	found := false
	for i := range b.entries {
		if b.entries[i].key == key {
			newEntries = append(newEntries, baggageEntry{key: key, value: value})
			found = true
		} else {
			newEntries = append(newEntries, b.entries[i])
		}
	}
	if !found {
		newEntries = append(newEntries, baggageEntry{key: key, value: value})
	}
	result := Baggage{entries: newEntries}
	if result.byteSize() > baggageMaxBytes {
		return Baggage{}, ErrBaggageTooLarge
	}
	return result, nil
}

// Delete returns a new Baggage with the key removed.
func (b Baggage) Delete(key string) Baggage {
	newEntries := make([]baggageEntry, 0, len(b.entries))
	for i := range b.entries {
		if b.entries[i].key != key {
			newEntries = append(newEntries, b.entries[i])
		}
	}
	return Baggage{entries: newEntries}
}

// Len returns the number of entries.
func (b Baggage) Len() int {
	return len(b.entries)
}

// Keys returns all keys in insertion order.
func (b Baggage) Keys() []string {
	keys := make([]string, len(b.entries))
	for i := range b.entries {
		keys[i] = b.entries[i].key
	}
	return keys
}

func (b Baggage) byteSize() int {
	size := 0
	for _, e := range b.entries {
		size += len(e.key) + len(e.value)
	}
	return size
}
