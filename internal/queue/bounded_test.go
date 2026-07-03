package queue

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBoundedQueueEnqueue(t *testing.T) {
	q := NewBoundedQueue[int](3)
	if !q.Enqueue(1) {
		t.Error("enqueue to empty queue should succeed")
	}
	if !q.Enqueue(2) {
		t.Error("enqueue to non-full queue should succeed")
	}
	if !q.Enqueue(3) {
		t.Error("enqueue to last slot should succeed")
	}
	if q.Enqueue(4) {
		t.Error("enqueue to full queue should fail")
	}
}

func TestBoundedQueueDequeue(t *testing.T) {
	q := NewBoundedQueue[int](3)
	q.Enqueue(10)
	q.Enqueue(20)

	item, ok := q.Dequeue()
	if !ok || item != 10 {
		t.Errorf("Dequeue() = (%d, %v), want (10, true)", item, ok)
	}
	item, ok = q.Dequeue()
	if !ok || item != 20 {
		t.Errorf("Dequeue() = (%d, %v), want (20, true)", item, ok)
	}
	_, ok = q.Dequeue()
	if ok {
		t.Error("Dequeue from empty queue should return false")
	}
}

func TestBoundedQueueEnqueueWait(t *testing.T) {
	q := NewBoundedQueue[int](1)
	q.Enqueue(1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := q.EnqueueWait(ctx, 2)
	if err == nil {
		t.Error("EnqueueWait on full queue with timeout should fail")
	}
}

func TestBoundedQueueEnqueueWaitSuccess(t *testing.T) {
	q := NewBoundedQueue[int](1)
	q.Enqueue(1)

	go func() {
		time.Sleep(10 * time.Millisecond)
		q.Dequeue()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := q.EnqueueWait(ctx, 2)
	if err != nil {
		t.Errorf("EnqueueWait should succeed after space opens: %v", err)
	}
}

func TestBoundedQueueDequeueWait(t *testing.T) {
	q := NewBoundedQueue[int](1)

	go func() {
		time.Sleep(10 * time.Millisecond)
		q.Enqueue(42)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	item, err := q.DequeueWait(ctx)
	if err != nil {
		t.Fatalf("DequeueWait should succeed: %v", err)
	}
	if item != 42 {
		t.Errorf("DequeueWait = %d, want 42", item)
	}
}

func TestBoundedQueueDequeueWaitTimeout(t *testing.T) {
	q := NewBoundedQueue[int](1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := q.DequeueWait(ctx)
	if err == nil {
		t.Error("DequeueWait on empty queue with timeout should fail")
	}
}

func TestBoundedQueueDrainTo(t *testing.T) {
	q := NewBoundedQueue[int](10)
	for i := 0; i < 5; i++ {
		q.Enqueue(i)
	}

	dst := make([]int, 10)
	n := q.DrainTo(dst)
	if n != 5 {
		t.Errorf("DrainTo returned %d, want 5", n)
	}
	for i := 0; i < 5; i++ {
		if dst[i] != i {
			t.Errorf("dst[%d] = %d, want %d", i, dst[i], i)
		}
	}
	if q.Len() != 0 {
		t.Errorf("queue should be empty after drain, len=%d", q.Len())
	}
}

func TestBoundedQueueDrainToPartial(t *testing.T) {
	q := NewBoundedQueue[int](10)
	for i := 0; i < 5; i++ {
		q.Enqueue(i)
	}
	dst := make([]int, 3) // smaller than queue contents
	n := q.DrainTo(dst)
	if n != 3 {
		t.Errorf("DrainTo returned %d, want 3", n)
	}
	if q.Len() != 2 {
		t.Errorf("queue should have 2 remaining, got %d", q.Len())
	}
}

func TestBoundedQueueDrainToEmpty(t *testing.T) {
	q := NewBoundedQueue[int](5)
	dst := make([]int, 5)
	n := q.DrainTo(dst)
	if n != 0 {
		t.Errorf("DrainTo on empty queue returned %d, want 0", n)
	}
}

func TestBoundedQueueLenCap(t *testing.T) {
	q := NewBoundedQueue[int](5)
	if q.Cap() != 5 {
		t.Errorf("Cap() = %d, want 5", q.Cap())
	}
	if q.Len() != 0 {
		t.Errorf("Len() = %d, want 0", q.Len())
	}
	q.Enqueue(1)
	if q.Len() != 1 {
		t.Errorf("Len() = %d, want 1", q.Len())
	}
}

func TestBoundedQueueClose(t *testing.T) {
	q := NewBoundedQueue[int](5)
	q.Enqueue(1)
	q.Close()

	if q.Enqueue(2) {
		t.Error("Enqueue after Close should fail")
	}

	err := q.EnqueueWait(context.Background(), 3)
	if err != ErrQueueClosed {
		t.Errorf("EnqueueWait after Close should return ErrQueueClosed, got %v", err)
	}

	// Should still be able to dequeue existing items
	item, ok := q.Dequeue()
	if !ok || item != 1 {
		t.Errorf("Dequeue after Close = (%d, %v), want (1, true)", item, ok)
	}
}

func TestBoundedQueueDoubleClose(t *testing.T) {
	q := NewBoundedQueue[int](1)
	q.Close()
	q.Close() // should not panic
}

func TestBoundedQueueZeroCapacity(t *testing.T) {
	q := NewBoundedQueue[int](0)
	if q.Cap() != 1 {
		t.Errorf("zero capacity should default to 1, got %d", q.Cap())
	}
}

func TestBoundedQueueNegativeCapacity(t *testing.T) {
	q := NewBoundedQueue[int](-5)
	if q.Cap() != 1 {
		t.Errorf("negative capacity should default to 1, got %d", q.Cap())
	}
}

func TestBoundedQueueConcurrent(t *testing.T) {
	q := NewBoundedQueue[int](100)
	var wg sync.WaitGroup

	// Producers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				q.Enqueue(j)
			}
		}()
	}

	// Consumers
	consumed := make(chan int, 1000)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				item, ok := q.Dequeue()
				if ok {
					consumed <- item
				}
			}
		}()
	}

	wg.Wait()
	close(consumed)
	// Just verify no panics or deadlocks occurred
}
