package queue

import (
	"context"
	"sync"
)

// BoundedQueue is a thread-safe, bounded queue backed by a channel.
type BoundedQueue[T any] struct {
	ch     chan T
	closed bool
	mu     sync.Mutex
}

// NewBoundedQueue creates a BoundedQueue with the given capacity.
// Capacity must be greater than zero.
func NewBoundedQueue[T any](capacity int) *BoundedQueue[T] {
	if capacity <= 0 {
		capacity = 1
	}
	return &BoundedQueue[T]{
		ch: make(chan T, capacity),
	}
}

// Enqueue attempts to add an item to the queue without blocking.
// Returns false if the queue is full or closed.
func (q *BoundedQueue[T]) Enqueue(item T) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.mu.Unlock()

	select {
	case q.ch <- item:
		return true
	default:
		return false
	}
}

// EnqueueWait adds an item to the queue, blocking until space is available
// or the context is cancelled.
func (q *BoundedQueue[T]) EnqueueWait(ctx context.Context, item T) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrQueueClosed
	}
	q.mu.Unlock()

	select {
	case q.ch <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dequeue removes and returns an item from the queue without blocking.
// Returns false if the queue is empty or closed.
func (q *BoundedQueue[T]) Dequeue() (T, bool) {
	select {
	case item, ok := <-q.ch:
		return item, ok
	default:
		var zero T
		return zero, false
	}
}

// DequeueWait removes and returns an item, blocking until one is available
// or the context is cancelled.
func (q *BoundedQueue[T]) DequeueWait(ctx context.Context) (T, error) {
	select {
	case item := <-q.ch:
		return item, nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// DrainTo drains up to len(dst) items into the provided slice, returning
// the number of items drained.
func (q *BoundedQueue[T]) DrainTo(dst []T) int {
	n := 0
	for n < len(dst) {
		select {
		case item := <-q.ch:
			dst[n] = item
			n++
		default:
			return n
		}
	}
	return n
}

// Len returns the current number of items in the queue.
func (q *BoundedQueue[T]) Len() int {
	return len(q.ch)
}

// Cap returns the capacity of the queue.
func (q *BoundedQueue[T]) Cap() int {
	return cap(q.ch)
}

// Close closes the queue. After closing, Enqueue and EnqueueWait will fail.
// Items already in the queue can still be dequeued.
func (q *BoundedQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
}
