package health

import (
	"context"
	"testing"
	"time"
)

func TestSelfTracerDisabled(t *testing.T) {
	st := NewSelfTracer()
	st.RecordStorageWrite()
	st.RecordStorageError()
	st.RecordQuery()
	st.RecordPipelineFlush()

	stats := st.Stats()
	if stats.StorageWrites != 0 {
		t.Error("disabled should not record")
	}
}

func TestSelfTracerEnabled(t *testing.T) {
	st := NewSelfTracer()
	st.SetEnabled(true)

	st.RecordStorageWrite()
	st.RecordStorageWrite()
	st.RecordStorageError()
	st.RecordQuery()
	st.RecordPipelineFlush()

	stats := st.Stats()
	if stats.StorageWrites != 2 {
		t.Errorf("writes = %d", stats.StorageWrites)
	}
	if stats.StorageErrors != 1 {
		t.Errorf("errors = %d", stats.StorageErrors)
	}
	if stats.QueryCount != 1 {
		t.Errorf("queries = %d", stats.QueryCount)
	}
	if stats.PipelineFlush != 1 {
		t.Errorf("flushes = %d", stats.PipelineFlush)
	}
}

func TestSelfTracerToggle(t *testing.T) {
	st := NewSelfTracer()
	if st.IsEnabled() {
		t.Error("should start disabled")
	}
	st.SetEnabled(true)
	if !st.IsEnabled() {
		t.Error("should be enabled")
	}
}

func TestTimedOperation(t *testing.T) {
	d := TimedOperation(func() {
		time.Sleep(10 * time.Millisecond)
	})
	if d < 5*time.Millisecond {
		t.Errorf("duration too short: %v", d)
	}
}

func TestInstrumentedFunc(t *testing.T) {
	called := false
	fn := InstrumentedFunc("test", func(_ context.Context) error {
		called = true
		return nil
	})
	err := fn(context.Background())
	if err != nil || !called {
		t.Error("should call wrapped function")
	}
}
