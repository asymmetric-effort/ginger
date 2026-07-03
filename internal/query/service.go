package query

import (
	"context"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

// Service provides trace query operations.
type Service struct {
	reader   storage.TraceReader
	depReader storage.DependencyReader
	adjuster *AdjusterChain
}

// NewService creates a query Service.
func NewService(reader storage.TraceReader, depReader storage.DependencyReader, adjuster *AdjusterChain) *Service {
	if adjuster == nil {
		adjuster = NewAdjusterChain()
	}
	return &Service{reader: reader, depReader: depReader, adjuster: adjuster}
}

// GetTrace retrieves and adjusts a trace by ID.
func (s *Service) GetTrace(ctx context.Context, traceID storage.TraceID) (otlp.TracesData, error) {
	td, err := s.reader.GetTrace(ctx, traceID)
	if err != nil {
		return otlp.TracesData{}, err
	}
	td, _ = s.adjuster.Adjust(td)
	return td, nil
}

// FindTraces searches for traces matching the query.
func (s *Service) FindTraces(ctx context.Context, query storage.TraceQueryParameters) ([]otlp.TracesData, error) {
	results, err := s.reader.FindTraces(ctx, query)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i], _ = s.adjuster.Adjust(results[i])
	}
	return results, nil
}

// GetServices returns all service names.
func (s *Service) GetServices(ctx context.Context) ([]string, error) {
	return s.reader.GetServices(ctx)
}

// GetOperations returns operations for a service.
func (s *Service) GetOperations(ctx context.Context, service string) ([]storage.Operation, error) {
	return s.reader.GetOperations(ctx, service)
}

// GetDependencies returns service dependencies.
func (s *Service) GetDependencies(ctx context.Context, endTime time.Time, lookback time.Duration) ([]storage.DependencyLink, error) {
	return s.depReader.GetDependencies(ctx, endTime, lookback)
}
