package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// BatchConfig configures the batch processor.
type BatchConfig struct {
	MaxBatchSize int
	SendDelay    time.Duration
	MaxQueueSize int
}

// BatchProcessor accumulates spans and flushes them in batches.
type BatchProcessor struct {
	config BatchConfig
	next   Exporter
	mu     sync.Mutex
	batch  []otlp.Span
	res    otlp.Resource
	scope  otlp.InstrumentationScope
	timer  *time.Timer
}

// NewBatchProcessor creates a batch processor that forwards to the given exporter.
func NewBatchProcessor(config BatchConfig, next Exporter) *BatchProcessor {
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 512
	}
	if config.SendDelay <= 0 {
		config.SendDelay = 200 * time.Millisecond
	}
	return &BatchProcessor{
		config: config,
		next:   next,
		batch:  make([]otlp.Span, 0, config.MaxBatchSize),
	}
}

// ProcessTraces accumulates spans. Flushes when batch is full or timer fires.
func (bp *BatchProcessor) ProcessTraces(ctx context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	bp.mu.Lock()
	for _, rs := range td.ResourceSpans {
		bp.res = rs.Resource
		for _, ss := range rs.ScopeSpans {
			bp.scope = ss.Scope
			for _, span := range ss.Spans {
				bp.batch = append(bp.batch, span)
				if len(bp.batch) >= bp.config.MaxBatchSize {
					bp.flushLocked(ctx)
				}
			}
		}
	}
	if len(bp.batch) > 0 && bp.timer == nil {
		bp.timer = time.AfterFunc(bp.config.SendDelay, func() {
			bp.mu.Lock()
			defer bp.mu.Unlock()
			bp.flushLocked(context.Background())
		})
	}
	bp.mu.Unlock()

	return otlp.TracesData{}, nil
}

// Flush forces a flush of the current batch.
func (bp *BatchProcessor) Flush(ctx context.Context) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.flushLocked(ctx)
}

func (bp *BatchProcessor) flushLocked(ctx context.Context) {
	if len(bp.batch) == 0 {
		return
	}
	if bp.timer != nil {
		bp.timer.Stop()
		bp.timer = nil
	}
	spans := make([]otlp.Span, len(bp.batch))
	copy(spans, bp.batch)
	bp.batch = bp.batch[:0]

	td := otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource:   bp.res,
			ScopeSpans: []otlp.ScopeSpans{{Scope: bp.scope, Spans: spans}},
		}},
	}
	bp.next.ExportTraces(ctx, td)
}

// Shutdown flushes remaining spans and shuts down.
func (bp *BatchProcessor) Shutdown(ctx context.Context) error {
	bp.Flush(ctx)
	return nil
}
