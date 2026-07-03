package storage

import (
	"context"
	"errors"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

var (
	// ErrTraceNotFound is returned when a trace is not found.
	ErrTraceNotFound = errors.New("trace not found")
	// ErrArchiveNotConfigured is returned when archive storage is not configured.
	ErrArchiveNotConfigured = errors.New("archive storage not configured")
)

// TraceID is a 16-byte trace identifier.
type TraceID = [16]byte

// Operation describes a span operation.
type Operation struct {
	Name     string
	SpanKind string
}

// TraceQueryParameters defines search criteria for finding traces.
type TraceQueryParameters struct {
	ServiceName   string
	OperationName string
	Tags          map[string]string
	StartTimeMin  time.Time
	StartTimeMax  time.Time
	DurationMin   time.Duration
	DurationMax   time.Duration
	NumTraces     int
}

// DependencyLink describes a dependency between two services.
type DependencyLink struct {
	Parent    string
	Child     string
	CallCount uint64
}

// Throughput holds throughput data for adaptive sampling.
type Throughput struct {
	Service       string
	Operation     string
	Count         int64
	Probabilities float64
}

// ServiceOperationProbabilities maps service→operation→probability.
type ServiceOperationProbabilities map[string]map[string]float64

// TraceWriter writes spans to storage.
type TraceWriter interface {
	WriteSpans(ctx context.Context, td otlp.TracesData) error
}

// TraceReader reads spans from storage.
type TraceReader interface {
	GetTrace(ctx context.Context, traceID TraceID) (otlp.TracesData, error)
	FindTraces(ctx context.Context, query TraceQueryParameters) ([]otlp.TracesData, error)
	FindTraceIDs(ctx context.Context, query TraceQueryParameters) ([]TraceID, error)
	GetServices(ctx context.Context) ([]string, error)
	GetOperations(ctx context.Context, service string) ([]Operation, error)
}

// DependencyWriter writes dependency links.
type DependencyWriter interface {
	WriteLinks(ctx context.Context, ts time.Time, deps []DependencyLink) error
}

// DependencyReader reads dependency links.
type DependencyReader interface {
	GetDependencies(ctx context.Context, endTime time.Time, lookback time.Duration) ([]DependencyLink, error)
}

// SamplingStore persists adaptive sampling data.
type SamplingStore interface {
	InsertThroughput(ctx context.Context, throughput []Throughput) error
	InsertProbabilities(ctx context.Context, hostname string, probs ServiceOperationProbabilities) error
	GetThroughput(ctx context.Context, start, end time.Time) ([]Throughput, error)
	GetLatestProbabilities(ctx context.Context) (ServiceOperationProbabilities, error)
}

// StorageBackend is a factory for creating storage components.
type StorageBackend interface {
	TraceReader() TraceReader
	TraceWriter() TraceWriter
	DependencyReader() DependencyReader
	DependencyWriter() DependencyWriter
	SamplingStore() SamplingStore
	Close() error
}

// Registry holds named storage backends.
type Registry struct {
	backends map[string]StorageBackend
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{backends: make(map[string]StorageBackend)}
}

// Register adds a backend to the registry.
func (r *Registry) Register(name string, backend StorageBackend) {
	r.backends[name] = backend
}

// Get returns a backend by name.
func (r *Registry) Get(name string) (StorageBackend, bool) {
	b, ok := r.backends[name]
	return b, ok
}

// Close closes all registered backends.
func (r *Registry) Close() error {
	var firstErr error
	for _, b := range r.backends {
		if err := b.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
