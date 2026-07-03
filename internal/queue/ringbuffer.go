package queue

import "sync"

// RingBuffer is a fixed-size buffer that overwrites the oldest entry when full.
type RingBuffer[T any] struct {
	mu    sync.Mutex
	buf   []T
	head  int
	count int
}

// NewRingBuffer creates a RingBuffer with the given capacity.
// Capacity must be greater than zero.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer[T]{
		buf: make([]T, capacity),
	}
}

// Put adds an item to the ring buffer. If the buffer is full,
// the oldest item is overwritten.
func (r *RingBuffer[T]) Put(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	writeIdx := (r.head + r.count) % len(r.buf)
	if r.count == len(r.buf) {
		// Buffer is full — overwrite oldest, advance head
		r.buf[r.head] = item
		r.head = (r.head + 1) % len(r.buf)
	} else {
		r.buf[writeIdx] = item
		r.count++
	}
}

// Get removes and returns the oldest item from the buffer.
// Returns false if the buffer is empty.
func (r *RingBuffer[T]) Get() (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		var zero T
		return zero, false
	}

	item := r.buf[r.head]
	var zero T
	r.buf[r.head] = zero // clear reference for GC
	r.head = (r.head + 1) % len(r.buf)
	r.count--
	return item, true
}

// Len returns the number of items in the buffer.
func (r *RingBuffer[T]) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

// Cap returns the capacity of the buffer.
func (r *RingBuffer[T]) Cap() int {
	return len(r.buf)
}
