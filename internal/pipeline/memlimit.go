package pipeline

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// MemoryLimiterConfig configures the memory limiter processor.
type MemoryLimiterConfig struct {
	HardLimitMiB  uint64
	SoftLimitMiB  uint64
	CheckInterval time.Duration
	DropRatio     float64 // fraction to drop when soft limit exceeded (0.0-1.0)
}

// MemoryLimiter drops spans when heap memory exceeds thresholds.
type MemoryLimiter struct {
	config     MemoryLimiterConfig
	softExceed atomic.Bool
	hardExceed atomic.Bool
	dropCount  atomic.Int64
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	counter    atomic.Uint64
}

// NewMemoryLimiter creates a new memory limiter processor.
func NewMemoryLimiter(config MemoryLimiterConfig) *MemoryLimiter {
	if config.CheckInterval <= 0 {
		config.CheckInterval = time.Second
	}
	if config.DropRatio <= 0 {
		config.DropRatio = 0.5
	}
	return &MemoryLimiter{config: config}
}

// Start begins memory monitoring.
func (ml *MemoryLimiter) Start(ctx context.Context) {
	ctx, ml.cancel = context.WithCancel(ctx)
	ml.wg.Add(1)
	go ml.monitor(ctx)
}

// ProcessTraces drops data if memory thresholds are exceeded.
func (ml *MemoryLimiter) ProcessTraces(_ context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	if ml.hardExceed.Load() {
		ml.dropCount.Add(1)
		return otlp.TracesData{}, ErrMemoryLimitExceeded
	}
	if ml.softExceed.Load() {
		n := ml.counter.Add(1)
		dropThreshold := uint64(1.0 / ml.config.DropRatio)
		if dropThreshold == 0 {
			dropThreshold = 2
		}
		if n%dropThreshold == 0 {
			ml.dropCount.Add(1)
			return otlp.TracesData{}, ErrMemoryLimitExceeded
		}
	}
	return td, nil
}

// Shutdown stops the memory monitor.
func (ml *MemoryLimiter) Shutdown() {
	if ml.cancel != nil {
		ml.cancel()
	}
	ml.wg.Wait()
}

// DroppedCount returns the number of dropped batches.
func (ml *MemoryLimiter) DroppedCount() int64 {
	return ml.dropCount.Load()
}

func (ml *MemoryLimiter) monitor(ctx context.Context) {
	defer ml.wg.Done()
	ticker := time.NewTicker(ml.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			heapMiB := stats.HeapInuse / (1024 * 1024)

			if ml.config.HardLimitMiB > 0 && heapMiB >= ml.config.HardLimitMiB {
				ml.hardExceed.Store(true)
				ml.softExceed.Store(true)
			} else if ml.config.SoftLimitMiB > 0 && heapMiB >= ml.config.SoftLimitMiB {
				ml.hardExceed.Store(false)
				ml.softExceed.Store(true)
			} else {
				ml.hardExceed.Store(false)
				ml.softExceed.Store(false)
			}
		}
	}
}
