package storage

import (
	"context"
	"testing"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

type mockBackend struct {
	reader *mockReader
	writer *mockWriter
}

func (b *mockBackend) TraceReader() TraceReader        { return b.reader }
func (b *mockBackend) TraceWriter() TraceWriter        { return b.writer }
func (b *mockBackend) DependencyReader() DependencyReader { return nil }
func (b *mockBackend) DependencyWriter() DependencyWriter { return nil }
func (b *mockBackend) SamplingStore() SamplingStore    { return nil }
func (b *mockBackend) Close() error                    { return nil }

type mockReader struct {
	traces map[TraceID]otlp.TracesData
}

func (r *mockReader) GetTrace(_ context.Context, tid TraceID) (otlp.TracesData, error) {
	td, ok := r.traces[tid]
	if !ok {
		return otlp.TracesData{}, ErrTraceNotFound
	}
	return td, nil
}

func (r *mockReader) FindTraces(_ context.Context, _ TraceQueryParameters) ([]otlp.TracesData, error) {
	return nil, nil
}
func (r *mockReader) FindTraceIDs(_ context.Context, _ TraceQueryParameters) ([]TraceID, error) {
	return nil, nil
}
func (r *mockReader) GetServices(_ context.Context) ([]string, error)                      { return nil, nil }
func (r *mockReader) GetOperations(_ context.Context, _ string) ([]Operation, error) { return nil, nil }

type mockWriter struct {
	written []otlp.TracesData
}

func (w *mockWriter) WriteSpans(_ context.Context, td otlp.TracesData) error {
	w.written = append(w.written, td)
	return nil
}

func TestArchiveTraceSuccess(t *testing.T) {
	tid := TraceID{1}
	td := otlp.TracesData{ResourceSpans: []otlp.ResourceSpans{{}}}

	primary := &mockReader{traces: map[TraceID]otlp.TracesData{tid: td}}
	archiveWriter := &mockWriter{}
	archiveBackend := &mockBackend{reader: &mockReader{traces: map[TraceID]otlp.TracesData{}}, writer: archiveWriter}

	a := NewArchiveStorage(primary, nil, archiveBackend)
	err := a.ArchiveTrace(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if len(archiveWriter.written) != 1 {
		t.Errorf("expected 1 write, got %d", len(archiveWriter.written))
	}
}

func TestArchiveTraceNotConfigured(t *testing.T) {
	a := NewArchiveStorage(nil, nil, nil)
	err := a.ArchiveTrace(context.Background(), TraceID{1})
	if err != ErrArchiveNotConfigured {
		t.Errorf("expected ErrArchiveNotConfigured, got %v", err)
	}
}

func TestArchiveTraceNotFound(t *testing.T) {
	primary := &mockReader{traces: map[TraceID]otlp.TracesData{}}
	archiveBackend := &mockBackend{writer: &mockWriter{}}

	a := NewArchiveStorage(primary, nil, archiveBackend)
	err := a.ArchiveTrace(context.Background(), TraceID{99})
	if err != ErrTraceNotFound {
		t.Errorf("expected ErrTraceNotFound, got %v", err)
	}
}

func TestGetArchivedTrace(t *testing.T) {
	tid := TraceID{1}
	td := otlp.TracesData{ResourceSpans: []otlp.ResourceSpans{{}}}
	archiveBackend := &mockBackend{
		reader: &mockReader{traces: map[TraceID]otlp.TracesData{tid: td}},
		writer: &mockWriter{},
	}

	a := NewArchiveStorage(nil, nil, archiveBackend)
	result, err := a.GetArchivedTrace(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected result")
	}
}

func TestGetArchivedTraceNotConfigured(t *testing.T) {
	a := NewArchiveStorage(nil, nil, nil)
	_, err := a.GetArchivedTrace(context.Background(), TraceID{1})
	if err != ErrArchiveNotConfigured {
		t.Errorf("expected ErrArchiveNotConfigured, got %v", err)
	}
}

func TestHasArchive(t *testing.T) {
	a := NewArchiveStorage(nil, nil, nil)
	if a.HasArchive() {
		t.Error("should not have archive")
	}
	a2 := NewArchiveStorage(nil, nil, &mockBackend{})
	if !a2.HasArchive() {
		t.Error("should have archive")
	}
}
