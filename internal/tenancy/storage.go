package tenancy

import (
	"context"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

// TenantedWriter wraps a TraceWriter to inject tenant tag.
type TenantedWriter struct {
	inner storage.TraceWriter
}

// NewTenantedWriter creates a tenanted writer.
func NewTenantedWriter(inner storage.TraceWriter) *TenantedWriter {
	return &TenantedWriter{inner: inner}
}

// WriteSpans injects tenant from context into resource attributes before writing.
func (tw *TenantedWriter) WriteSpans(ctx context.Context, td otlp.TracesData) error {
	tenant, ok := TenantFromContext(ctx)
	if !ok {
		return tw.inner.WriteSpans(ctx, td)
	}
	for i := range td.ResourceSpans {
		td.ResourceSpans[i].Resource.Attributes.Set("ginger.tenant", otlp.StringValue(tenant))
	}
	return tw.inner.WriteSpans(ctx, td)
}

// TenantedReader wraps a TraceReader to filter by tenant.
type TenantedReader struct {
	inner storage.TraceReader
}

// NewTenantedReader creates a tenanted reader.
func NewTenantedReader(inner storage.TraceReader) *TenantedReader {
	return &TenantedReader{inner: inner}
}

// GetTrace retrieves a trace and verifies tenant ownership.
func (tr *TenantedReader) GetTrace(ctx context.Context, traceID storage.TraceID) (otlp.TracesData, error) {
	td, err := tr.inner.GetTrace(ctx, traceID)
	if err != nil {
		return td, err
	}
	tenant, ok := TenantFromContext(ctx)
	if !ok {
		return td, nil
	}
	if !traceBelongsToTenant(td, tenant) {
		return otlp.TracesData{}, storage.ErrTraceNotFound
	}
	return td, nil
}

// FindTraces finds traces filtered by tenant.
func (tr *TenantedReader) FindTraces(ctx context.Context, query storage.TraceQueryParameters) ([]otlp.TracesData, error) {
	results, err := tr.inner.FindTraces(ctx, query)
	if err != nil {
		return nil, err
	}
	tenant, ok := TenantFromContext(ctx)
	if !ok {
		return results, nil
	}
	var filtered []otlp.TracesData
	for _, td := range results {
		if traceBelongsToTenant(td, tenant) {
			filtered = append(filtered, td)
		}
	}
	return filtered, nil
}

// FindTraceIDs delegates to inner reader.
func (tr *TenantedReader) FindTraceIDs(ctx context.Context, query storage.TraceQueryParameters) ([]storage.TraceID, error) {
	return tr.inner.FindTraceIDs(ctx, query)
}

// GetServices delegates to inner reader.
func (tr *TenantedReader) GetServices(ctx context.Context) ([]string, error) {
	return tr.inner.GetServices(ctx)
}

// GetOperations delegates to inner reader.
func (tr *TenantedReader) GetOperations(ctx context.Context, service string) ([]storage.Operation, error) {
	return tr.inner.GetOperations(ctx, service)
}

func traceBelongsToTenant(td otlp.TracesData, tenant string) bool {
	for _, rs := range td.ResourceSpans {
		v, ok := rs.Resource.Attributes.Get("ginger.tenant")
		if ok && v.Str == tenant {
			return true
		}
	}
	return false
}
