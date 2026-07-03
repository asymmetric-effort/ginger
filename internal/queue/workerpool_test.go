package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolBasic(t *testing.T) {
	var count atomic.Int64

	pool := NewWorkerPool[int](4, 10, func(_ context.Context, item int) {
		count.Add(int64(item))
	})

	ctx := context.Background()
	pool.Start(ctx)

	for i := 1; i <= 10; i++ {
		if !pool.Submit(i) {
			t.Errorf("Submit(%d) failed", i)
		}
	}

	// Give workers time to process
	time.Sleep(100 * time.Millisecond)
	pool.Close()

	expected := int64(55) // sum of 1..10
	if got := count.Load(); got != expected {
		t.Errorf("processed sum = %d, want %d", got, expected)
	}
}

func TestWorkerPoolSubmitWait(t *testing.T) {
	var count atomic.Int64

	pool := NewWorkerPool[int](1, 1, func(_ context.Context, item int) {
		count.Add(1)
		time.Sleep(10 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool.Start(ctx)

	err := pool.SubmitWait(ctx, 1)
	if err != nil {
		t.Fatalf("SubmitWait failed: %v", err)
	}

	pool.Close()
	if count.Load() < 1 {
		t.Error("expected at least 1 item processed")
	}
}

func TestWorkerPoolSubmitWaitTimeout(t *testing.T) {
	// Use a BoundedQueue directly to test SubmitWait timeout
	// since the worker pool complicates timing.
	q := NewBoundedQueue[int](1)
	q.Enqueue(1) // fill it

	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := q.EnqueueWait(shortCtx, 2)
	if err == nil {
		t.Error("EnqueueWait should timeout on full queue")
	}
}

func TestWorkerPoolClose(t *testing.T) {
	var processed atomic.Int64

	pool := NewWorkerPool[int](2, 100, func(_ context.Context, _ int) {
		processed.Add(1)
	})

	ctx := context.Background()
	pool.Start(ctx)

	for i := 0; i < 50; i++ {
		pool.Submit(i)
	}

	pool.Close()

	// After Close, Submit should fail
	if pool.Submit(999) {
		t.Error("Submit after Close should fail")
	}
}

func TestWorkerPoolSubmitAfterClose(t *testing.T) {
	pool := NewWorkerPool[int](1, 1, func(_ context.Context, _ int) {})
	pool.Start(context.Background())
	pool.Close()

	if pool.Submit(1) {
		t.Error("Submit after Close should return false")
	}
}

func TestWorkerPoolZeroWorkers(t *testing.T) {
	pool := NewWorkerPool[int](0, 5, func(_ context.Context, _ int) {})
	// Should default to 1 worker
	pool.Start(context.Background())
	pool.Submit(1)
	time.Sleep(50 * time.Millisecond)
	pool.Close()
}

func TestWorkerPoolConcurrentSubmit(t *testing.T) {
	var count atomic.Int64

	pool := NewWorkerPool[int](4, 100, func(_ context.Context, _ int) {
		count.Add(1)
	})

	ctx := context.Background()
	pool.Start(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				pool.Submit(j)
			}
		}()
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)
	pool.Close()

	if count.Load() == 0 {
		t.Error("expected some items to be processed")
	}
}

func TestWorkerPoolContextCancellation(t *testing.T) {
	var count atomic.Int64

	ctx, cancel := context.WithCancel(context.Background())
	pool := NewWorkerPool[int](2, 10, func(_ context.Context, _ int) {
		count.Add(1)
		time.Sleep(50 * time.Millisecond)
	})

	pool.Start(ctx)
	for i := 0; i < 5; i++ {
		pool.Submit(i)
	}

	time.Sleep(30 * time.Millisecond)
	cancel()
	pool.Close()
	// Verify no deadlocks or panics
}
