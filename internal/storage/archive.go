package storage

import "context"

// ArchiveStorage wraps a primary and archive backend.
type ArchiveStorage struct {
	primary TraceReader
	writer  TraceWriter
	archive StorageBackend
}

// NewArchiveStorage creates archive storage. If archive is nil, archive operations return ErrArchiveNotConfigured.
func NewArchiveStorage(primary TraceReader, writer TraceWriter, archive StorageBackend) *ArchiveStorage {
	return &ArchiveStorage{primary: primary, writer: writer, archive: archive}
}

// ArchiveTrace copies a trace from primary to archive storage.
func (a *ArchiveStorage) ArchiveTrace(ctx context.Context, traceID TraceID) error {
	if a.archive == nil {
		return ErrArchiveNotConfigured
	}
	td, err := a.primary.GetTrace(ctx, traceID)
	if err != nil {
		return err
	}
	return a.archive.TraceWriter().WriteSpans(ctx, td)
}

// GetArchivedTrace retrieves a trace from archive storage.
func (a *ArchiveStorage) GetArchivedTrace(ctx context.Context, traceID TraceID) (interface{}, error) {
	if a.archive == nil {
		return nil, ErrArchiveNotConfigured
	}
	return a.archive.TraceReader().GetTrace(ctx, traceID)
}

// HasArchive reports whether archive storage is configured.
func (a *ArchiveStorage) HasArchive() bool {
	return a.archive != nil
}
