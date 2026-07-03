package memory

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

const defaultMaxTraces = 10000

// Backend is an in-memory implementation of all storage interfaces.
type Backend struct {
	mu        sync.RWMutex
	traces    map[storage.TraceID]*storedTrace
	order     []storage.TraceID
	maxTraces int

	deps     []storedDep
	sampling storedSampling
}

type storedTrace struct {
	data      otlp.TracesData
	timestamp time.Time
}

type storedDep struct {
	ts   time.Time
	deps []storage.DependencyLink
}

type storedSampling struct {
	throughput    []storage.Throughput
	probabilities storage.ServiceOperationProbabilities
}

// NewBackend creates a new in-memory backend.
func NewBackend(maxTraces int) *Backend {
	if maxTraces <= 0 {
		maxTraces = defaultMaxTraces
	}
	return &Backend{
		traces:    make(map[storage.TraceID]*storedTrace),
		maxTraces: maxTraces,
	}
}

// TraceReader returns the trace reader.
func (b *Backend) TraceReader() storage.TraceReader { return b }

// TraceWriter returns the trace writer.
func (b *Backend) TraceWriter() storage.TraceWriter { return b }

// DependencyReader returns the dependency reader.
func (b *Backend) DependencyReader() storage.DependencyReader { return b }

// DependencyWriter returns the dependency writer.
func (b *Backend) DependencyWriter() storage.DependencyWriter { return b }

// SamplingStore returns the sampling store.
func (b *Backend) SamplingStore() storage.SamplingStore { return b }

// Close is a no-op for in-memory backend.
func (b *Backend) Close() error { return nil }

// Clear removes all stored data.
func (b *Backend) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.traces = make(map[storage.TraceID]*storedTrace)
	b.order = nil
	b.deps = nil
	b.sampling = storedSampling{}
}

// === TraceWriter ===

// WriteSpans stores spans grouped by trace ID.
func (b *Backend) WriteSpans(_ context.Context, td otlp.TracesData) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, rs := range td.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				tid := span.TraceID
				existing, ok := b.traces[tid]
				if !ok {
					// Evict oldest if at capacity
					if len(b.traces) >= b.maxTraces {
						b.evictOldest()
					}
					b.traces[tid] = &storedTrace{
						data:      wrapSpan(rs.Resource, ss.Scope, span),
						timestamp: time.Now(),
					}
					b.order = append(b.order, tid)
				} else {
					// Merge span into existing trace
					mergeSpan(&existing.data, rs.Resource, ss.Scope, span)
				}
			}
		}
	}
	return nil
}

// === TraceReader ===

// GetTrace retrieves a trace by ID.
func (b *Backend) GetTrace(_ context.Context, traceID storage.TraceID) (otlp.TracesData, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	st, ok := b.traces[traceID]
	if !ok {
		return otlp.TracesData{}, storage.ErrTraceNotFound
	}
	return st.data, nil
}

// FindTraces searches for traces matching the query.
func (b *Backend) FindTraces(_ context.Context, query storage.TraceQueryParameters) ([]otlp.TracesData, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var results []otlp.TracesData
	limit := query.NumTraces
	if limit <= 0 {
		limit = 20
	}

	for _, tid := range b.order {
		st := b.traces[tid]
		if matchesQuery(st, query) {
			results = append(results, st.data)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// FindTraceIDs returns trace IDs matching the query.
func (b *Backend) FindTraceIDs(_ context.Context, query storage.TraceQueryParameters) ([]storage.TraceID, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var ids []storage.TraceID
	limit := query.NumTraces
	if limit <= 0 {
		limit = 20
	}

	for _, tid := range b.order {
		st := b.traces[tid]
		if matchesQuery(st, query) {
			ids = append(ids, tid)
			if len(ids) >= limit {
				break
			}
		}
	}
	return ids, nil
}

// GetServices returns all service names.
func (b *Backend) GetServices(_ context.Context) ([]string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	seen := make(map[string]struct{})
	for _, st := range b.traces {
		for _, rs := range st.data.ResourceSpans {
			svc := getServiceName(rs.Resource)
			if svc != "" {
				seen[svc] = struct{}{}
			}
		}
	}

	services := make([]string, 0, len(seen))
	for s := range seen {
		services = append(services, s)
	}
	return services, nil
}

// GetOperations returns operations for a service.
func (b *Backend) GetOperations(_ context.Context, service string) ([]storage.Operation, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	seen := make(map[string]struct{})
	var ops []storage.Operation

	for _, st := range b.traces {
		for _, rs := range st.data.ResourceSpans {
			if getServiceName(rs.Resource) != service {
				continue
			}
			for _, ss := range rs.ScopeSpans {
				for _, span := range ss.Spans {
					if _, exists := seen[span.Name]; !exists {
						seen[span.Name] = struct{}{}
						ops = append(ops, storage.Operation{
							Name:     span.Name,
							SpanKind: span.Kind.String(),
						})
					}
				}
			}
		}
	}
	return ops, nil
}

// === DependencyWriter ===

// WriteLinks stores dependency links.
func (b *Backend) WriteLinks(_ context.Context, ts time.Time, deps []storage.DependencyLink) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deps = append(b.deps, storedDep{ts: ts, deps: deps})
	return nil
}

// === DependencyReader ===

// GetDependencies returns dependencies within the time window.
func (b *Backend) GetDependencies(_ context.Context, endTime time.Time, lookback time.Duration) ([]storage.DependencyLink, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	startTime := endTime.Add(-lookback)
	merged := make(map[string]*storage.DependencyLink)

	for _, sd := range b.deps {
		if sd.ts.Before(startTime) || sd.ts.After(endTime) {
			continue
		}
		for _, d := range sd.deps {
			key := d.Parent + "→" + d.Child
			if existing, ok := merged[key]; ok {
				existing.CallCount += d.CallCount
			} else {
				cp := d
				merged[key] = &cp
			}
		}
	}

	result := make([]storage.DependencyLink, 0, len(merged))
	for _, d := range merged {
		result = append(result, *d)
	}
	return result, nil
}

// === SamplingStore ===

// InsertThroughput stores throughput data.
func (b *Backend) InsertThroughput(_ context.Context, throughput []storage.Throughput) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sampling.throughput = append(b.sampling.throughput, throughput...)
	return nil
}

// InsertProbabilities stores sampling probabilities.
func (b *Backend) InsertProbabilities(_ context.Context, _ string, probs storage.ServiceOperationProbabilities) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sampling.probabilities = probs
	return nil
}

// GetThroughput returns throughput data within the time range.
func (b *Backend) GetThroughput(_ context.Context, _, _ time.Time) ([]storage.Throughput, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]storage.Throughput, len(b.sampling.throughput))
	copy(out, b.sampling.throughput)
	return out, nil
}

// GetLatestProbabilities returns the latest sampling probabilities.
func (b *Backend) GetLatestProbabilities(_ context.Context) (storage.ServiceOperationProbabilities, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sampling.probabilities, nil
}

// === Helpers ===

func (b *Backend) evictOldest() {
	if len(b.order) == 0 {
		return
	}
	oldest := b.order[0]
	b.order = b.order[1:]
	delete(b.traces, oldest)
}

func wrapSpan(res otlp.Resource, scope otlp.InstrumentationScope, span otlp.Span) otlp.TracesData {
	return otlp.TracesData{
		ResourceSpans: []otlp.ResourceSpans{{
			Resource:   res,
			ScopeSpans: []otlp.ScopeSpans{{Scope: scope, Spans: []otlp.Span{span}}},
		}},
	}
}

func mergeSpan(td *otlp.TracesData, res otlp.Resource, scope otlp.InstrumentationScope, span otlp.Span) {
	// Simple merge: append to first ResourceSpans/ScopeSpans
	if len(td.ResourceSpans) == 0 {
		td.ResourceSpans = append(td.ResourceSpans, otlp.ResourceSpans{Resource: res})
	}
	rs := &td.ResourceSpans[0]
	if len(rs.ScopeSpans) == 0 {
		rs.ScopeSpans = append(rs.ScopeSpans, otlp.ScopeSpans{Scope: scope})
	}
	rs.ScopeSpans[0].Spans = append(rs.ScopeSpans[0].Spans, span)
}

func getServiceName(res otlp.Resource) string {
	v, ok := res.Attributes.Get("service.name")
	if !ok {
		return ""
	}
	return v.Str
}

func matchesQuery(st *storedTrace, query storage.TraceQueryParameters) bool {
	if query.ServiceName != "" {
		found := false
		for _, rs := range st.data.ResourceSpans {
			if getServiceName(rs.Resource) == query.ServiceName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if query.OperationName != "" {
		found := false
		for _, rs := range st.data.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, span := range ss.Spans {
					if span.Name == query.OperationName {
						found = true
						break
					}
				}
			}
		}
		if !found {
			return false
		}
	}

	if len(query.Tags) > 0 {
		if !matchesTags(st, query.Tags) {
			return false
		}
	}

	if !query.StartTimeMin.IsZero() || !query.StartTimeMax.IsZero() {
		if !matchesTimeRange(st, query.StartTimeMin, query.StartTimeMax) {
			return false
		}
	}

	if query.DurationMin > 0 || query.DurationMax > 0 {
		if !matchesDuration(st, query.DurationMin, query.DurationMax) {
			return false
		}
	}

	return true
}

func matchesTags(st *storedTrace, tags map[string]string) bool {
	for _, rs := range st.data.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				allMatch := true
				for k, v := range tags {
					av, ok := span.Attributes.Get(k)
					if !ok || !matchesTagValue(av, v) {
						allMatch = false
						break
					}
				}
				if allMatch {
					return true
				}
			}
		}
	}
	return false
}

func matchesTagValue(av otlp.AnyValue, expected string) bool {
	switch av.Type {
	case otlp.AnyValueTypeString:
		return av.Str == expected
	default:
		// Marshal to JSON for comparison
		data, _ := json.Marshal(av)
		return strings.Contains(string(data), expected)
	}
}

func matchesTimeRange(st *storedTrace, min, max time.Time) bool {
	for _, rs := range st.data.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				spanTime := time.Unix(0, int64(span.StartTimeUnixNano))
				if !min.IsZero() && spanTime.Before(min) {
					continue
				}
				if !max.IsZero() && spanTime.After(max) {
					continue
				}
				return true
			}
		}
	}
	return false
}

func matchesDuration(st *storedTrace, min, max time.Duration) bool {
	for _, rs := range st.data.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				dur := time.Duration(span.EndTimeUnixNano-span.StartTimeUnixNano) * time.Nanosecond
				if min > 0 && dur < min {
					continue
				}
				if max > 0 && dur > max {
					continue
				}
				return true
			}
		}
	}
	return false
}
