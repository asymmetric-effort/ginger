package queue

import (
	"context"
	"sync"
)

// WorkerFunc is the function type that workers execute.
type WorkerFunc[T any] func(ctx context.Context, item T)

// WorkerPool processes items from a bounded job queue using a fixed number of workers.
type WorkerPool[T any] struct {
	jobs    *BoundedQueue[T]
	fn      WorkerFunc[T]
	workers int
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

// NewWorkerPool creates a WorkerPool with the given number of workers,
// job queue capacity, and worker function.
func NewWorkerPool[T any](workers, queueCapacity int, fn WorkerFunc[T]) *WorkerPool[T] {
	if workers <= 0 {
		workers = 1
	}
	return &WorkerPool[T]{
		jobs:    NewBoundedQueue[T](queueCapacity),
		fn:      fn,
		workers: workers,
	}
}

// Start launches the worker goroutines. Must only be called once.
func (p *WorkerPool[T]) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

// Submit adds a job to the pool's queue without blocking.
// Returns false if the queue is full or the pool is closed.
func (p *WorkerPool[T]) Submit(item T) bool {
	return p.jobs.Enqueue(item)
}

// SubmitWait adds a job to the pool's queue, blocking until space is available
// or the context is cancelled.
func (p *WorkerPool[T]) SubmitWait(ctx context.Context, item T) error {
	return p.jobs.EnqueueWait(ctx, item)
}

// Close signals workers to stop, drains remaining jobs, and waits for
// all workers to finish.
func (p *WorkerPool[T]) Close() {
	p.jobs.Close()
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

func (p *WorkerPool[T]) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case item, ok := <-p.jobs.ch:
			if !ok {
				return
			}
			p.fn(ctx, item)
		case <-ctx.Done():
			// Drain remaining items before exiting
			for {
				select {
				case item, ok := <-p.jobs.ch:
					if !ok {
						return
					}
					p.fn(ctx, item)
				default:
					return
				}
			}
		}
	}
}
