package health

import (
	"context"
	"sync/atomic"
	"time"
)

// SelfTracer records internal operation metrics for ginger's own observability.
type SelfTracer struct {
	enabled       atomic.Bool
	storageWrites atomic.Int64
	storageErrors atomic.Int64
	queryCount    atomic.Int64
	pipelineFlush atomic.Int64
}

// NewSelfTracer creates a new self-tracer.
func NewSelfTracer() *SelfTracer {
	return &SelfTracer{}
}

// SetEnabled enables or disables self-tracing.
func (st *SelfTracer) SetEnabled(enabled bool) {
	st.enabled.Store(enabled)
}

// IsEnabled reports whether self-tracing is enabled.
func (st *SelfTracer) IsEnabled() bool {
	return st.enabled.Load()
}

// RecordStorageWrite records a storage write operation.
func (st *SelfTracer) RecordStorageWrite() {
	if st.enabled.Load() {
		st.storageWrites.Add(1)
	}
}

// RecordStorageError records a storage error.
func (st *SelfTracer) RecordStorageError() {
	if st.enabled.Load() {
		st.storageErrors.Add(1)
	}
}

// RecordQuery records a query operation.
func (st *SelfTracer) RecordQuery() {
	if st.enabled.Load() {
		st.queryCount.Add(1)
	}
}

// RecordPipelineFlush records a pipeline flush.
func (st *SelfTracer) RecordPipelineFlush() {
	if st.enabled.Load() {
		st.pipelineFlush.Add(1)
	}
}

// Stats returns current self-tracing statistics.
func (st *SelfTracer) Stats() SelfTracingStats {
	return SelfTracingStats{
		StorageWrites: st.storageWrites.Load(),
		StorageErrors: st.storageErrors.Load(),
		QueryCount:    st.queryCount.Load(),
		PipelineFlush: st.pipelineFlush.Load(),
	}
}

// SelfTracingStats holds self-tracing counters.
type SelfTracingStats struct {
	StorageWrites int64
	StorageErrors int64
	QueryCount    int64
	PipelineFlush int64
}

// TimedOperation measures the duration of an operation.
func TimedOperation(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// InstrumentedFunc wraps a function with duration measurement.
func InstrumentedFunc(name string, fn func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		start := time.Now()
		err := fn(ctx)
		_ = time.Since(start) // duration available for metrics
		_ = name
		return err
	}
}
