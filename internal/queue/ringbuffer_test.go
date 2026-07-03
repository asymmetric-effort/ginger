package queue

import (
	"sync"
	"testing"
)

func TestRingBufferPutGet(t *testing.T) {
	r := NewRingBuffer[int](3)

	r.Put(1)
	r.Put(2)
	r.Put(3)

	item, ok := r.Get()
	if !ok || item != 1 {
		t.Errorf("Get() = (%d, %v), want (1, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != 2 {
		t.Errorf("Get() = (%d, %v), want (2, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != 3 {
		t.Errorf("Get() = (%d, %v), want (3, true)", item, ok)
	}
	_, ok = r.Get()
	if ok {
		t.Error("Get from empty ring buffer should return false")
	}
}

func TestRingBufferOverwrite(t *testing.T) {
	r := NewRingBuffer[int](3)

	r.Put(1)
	r.Put(2)
	r.Put(3)
	r.Put(4) // overwrites 1

	item, ok := r.Get()
	if !ok || item != 2 {
		t.Errorf("after overwrite, Get() = (%d, %v), want (2, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != 3 {
		t.Errorf("Get() = (%d, %v), want (3, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != 4 {
		t.Errorf("Get() = (%d, %v), want (4, true)", item, ok)
	}
}

func TestRingBufferOverwriteMultiple(t *testing.T) {
	r := NewRingBuffer[int](2)

	r.Put(1)
	r.Put(2)
	r.Put(3) // overwrites 1
	r.Put(4) // overwrites 2

	item, ok := r.Get()
	if !ok || item != 3 {
		t.Errorf("Get() = (%d, %v), want (3, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != 4 {
		t.Errorf("Get() = (%d, %v), want (4, true)", item, ok)
	}
}

func TestRingBufferEmpty(t *testing.T) {
	r := NewRingBuffer[int](5)
	_, ok := r.Get()
	if ok {
		t.Error("Get from empty buffer should return false")
	}
}

func TestRingBufferLenCap(t *testing.T) {
	r := NewRingBuffer[int](5)
	if r.Cap() != 5 {
		t.Errorf("Cap() = %d, want 5", r.Cap())
	}
	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0", r.Len())
	}

	r.Put(1)
	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1", r.Len())
	}

	r.Put(2)
	r.Put(3)
	r.Put(4)
	r.Put(5)
	if r.Len() != 5 {
		t.Errorf("Len() = %d, want 5", r.Len())
	}

	r.Put(6) // overwrite
	if r.Len() != 5 {
		t.Errorf("Len() after overwrite = %d, want 5", r.Len())
	}

	r.Get()
	if r.Len() != 4 {
		t.Errorf("Len() after Get = %d, want 4", r.Len())
	}
}

func TestRingBufferZeroCapacity(t *testing.T) {
	r := NewRingBuffer[int](0)
	if r.Cap() != 1 {
		t.Errorf("zero capacity should default to 1, got %d", r.Cap())
	}
}

func TestRingBufferNegativeCapacity(t *testing.T) {
	r := NewRingBuffer[int](-3)
	if r.Cap() != 1 {
		t.Errorf("negative capacity should default to 1, got %d", r.Cap())
	}
}

func TestRingBufferSizeOne(t *testing.T) {
	r := NewRingBuffer[int](1)
	r.Put(1)
	r.Put(2) // overwrites 1

	item, ok := r.Get()
	if !ok || item != 2 {
		t.Errorf("Get() = (%d, %v), want (2, true)", item, ok)
	}
	_, ok = r.Get()
	if ok {
		t.Error("second Get should return false")
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	r := NewRingBuffer[int](3)

	// Fill and partially drain multiple times to exercise wrap-around
	r.Put(1)
	r.Put(2)
	r.Put(3)
	r.Get() // removes 1
	r.Get() // removes 2

	r.Put(4)
	r.Put(5)

	item, ok := r.Get()
	if !ok || item != 3 {
		t.Errorf("Get() = (%d, %v), want (3, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != 4 {
		t.Errorf("Get() = (%d, %v), want (4, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != 5 {
		t.Errorf("Get() = (%d, %v), want (5, true)", item, ok)
	}
}

func TestRingBufferConcurrent(t *testing.T) {
	r := NewRingBuffer[int](100)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				r.Put(j)
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				r.Get()
			}
		}()
	}

	wg.Wait()
	// Verify no panics or data races
}

func TestRingBufferStringType(t *testing.T) {
	r := NewRingBuffer[string](2)
	r.Put("hello")
	r.Put("world")

	item, ok := r.Get()
	if !ok || item != "hello" {
		t.Errorf("Get() = (%q, %v), want (hello, true)", item, ok)
	}
	item, ok = r.Get()
	if !ok || item != "world" {
		t.Errorf("Get() = (%q, %v), want (world, true)", item, ok)
	}
}
