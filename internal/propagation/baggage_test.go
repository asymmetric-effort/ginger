package propagation

import "testing"

func TestBaggageEmpty(t *testing.T) {
	b := NewBaggage()
	if b.Len() != 0 {
		t.Error("empty Baggage should have length 0")
	}
	if got := b.Get("key"); got != "" {
		t.Errorf("Get on empty should return empty, got %q", got)
	}
	keys := b.Keys()
	if len(keys) != 0 {
		t.Error("empty Baggage should have no keys")
	}
}

func TestBaggageSetAndGet(t *testing.T) {
	b := NewBaggage()
	b, err := b.Set("key1", "val1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err = b.Set("key2", "val2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.Len() != 2 {
		t.Errorf("expected length 2, got %d", b.Len())
	}
	if got := b.Get("key1"); got != "val1" {
		t.Errorf("Get(key1) = %q, want val1", got)
	}
	if got := b.Get("key2"); got != "val2" {
		t.Errorf("Get(key2) = %q, want val2", got)
	}
}

func TestBaggageSetOverwrite(t *testing.T) {
	b := NewBaggage()
	b, _ = b.Set("key", "old")
	b, _ = b.Set("key", "new")

	if b.Len() != 1 {
		t.Errorf("expected length 1 after overwrite, got %d", b.Len())
	}
	if got := b.Get("key"); got != "new" {
		t.Errorf("Get(key) = %q, want new", got)
	}
}

func TestBaggageDelete(t *testing.T) {
	b := NewBaggage()
	b, _ = b.Set("a", "1")
	b, _ = b.Set("b", "2")
	b, _ = b.Set("c", "3")

	b = b.Delete("b")
	if b.Len() != 2 {
		t.Errorf("expected length 2, got %d", b.Len())
	}
	if got := b.Get("b"); got != "" {
		t.Errorf("deleted key should return empty, got %q", got)
	}
	if got := b.Get("a"); got != "1" {
		t.Errorf("Get(a) = %q, want 1", got)
	}
	if got := b.Get("c"); got != "3" {
		t.Errorf("Get(c) = %q, want 3", got)
	}
}

func TestBaggageDeleteNonexistent(t *testing.T) {
	b := NewBaggage()
	b, _ = b.Set("a", "1")
	b2 := b.Delete("nonexistent")
	if b2.Len() != 1 {
		t.Error("delete nonexistent should not change length")
	}
}

func TestBaggageKeys(t *testing.T) {
	b := NewBaggage()
	b, _ = b.Set("x", "1")
	b, _ = b.Set("y", "2")
	b, _ = b.Set("z", "3")

	keys := b.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "x" || keys[1] != "y" || keys[2] != "z" {
		t.Errorf("unexpected key order: %v", keys)
	}
}

func TestBaggageGetMissing(t *testing.T) {
	b := NewBaggage()
	b, _ = b.Set("a", "1")
	if got := b.Get("missing"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}
}

func TestBaggageTooLarge(t *testing.T) {
	b := NewBaggage()
	// Build a value close to the 8192 byte limit then exceed it
	largeVal := make([]byte, baggageMaxBytes)
	for i := range largeVal {
		largeVal[i] = 'x'
	}
	_, err := b.Set("k", string(largeVal))
	if err != ErrBaggageTooLarge {
		t.Errorf("expected ErrBaggageTooLarge, got %v", err)
	}
}

func TestBaggageImmutability(t *testing.T) {
	b1 := NewBaggage()
	b1, _ = b1.Set("key", "val1")
	b2, _ := b1.Set("key", "val2")

	if b1.Get("key") != "val1" {
		t.Error("original baggage should not be modified")
	}
	if b2.Get("key") != "val2" {
		t.Error("new baggage should have updated value")
	}
}
