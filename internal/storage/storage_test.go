package storage

import "testing"

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("missing")
	if ok {
		t.Error("should not find missing backend")
	}

	r.Register("test", nil)
	_, ok = r.Get("test")
	if !ok {
		t.Error("should find registered backend")
	}
}

func TestRegistryClose(t *testing.T) {
	r := NewRegistry()
	err := r.Close()
	if err != nil {
		t.Errorf("close empty: %v", err)
	}
}
