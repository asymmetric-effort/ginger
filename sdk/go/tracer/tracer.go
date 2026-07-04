package tracer

import (
	"context"
	"crypto/rand"
	"sync"
	"time"
)

// Config configures the tracer.
type Config struct {
	Endpoint     string
	Service      string
	SamplingRate float64
	BatchSize    int
}

// SpanKind identifies the span kind.
type SpanKind int

const (
	SpanKindInternal SpanKind = 1
	SpanKindServer   SpanKind = 2
	SpanKindClient   SpanKind = 3
	SpanKindProducer SpanKind = 4
	SpanKindConsumer SpanKind = 5
)

// StatusCode represents span status.
type StatusCode int

const (
	StatusUnset StatusCode = 0
	StatusOk    StatusCode = 1
	StatusError StatusCode = 2
)

// Tracer creates spans.
type Tracer struct {
	config   Config
	mu       sync.Mutex
	spans    []*Span
	exporter *OTLPHTTPExporter
}

// New creates a new Tracer.
func New(config Config) *Tracer {
	if config.SamplingRate <= 0 {
		config.SamplingRate = 1.0
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 512
	}
	t := &Tracer{config: config}
	if config.Endpoint != "" {
		t.exporter = NewOTLPHTTPExporter(config.Endpoint, config.Service, config.BatchSize)
	}
	return t
}

type spanContextKey struct{}

// Start creates a new span and returns a context containing it.
func (t *Tracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span) {
	var traceID [16]byte
	var spanID [8]byte
	rand.Read(traceID[:])
	rand.Read(spanID[:])

	s := &Span{
		traceID:   traceID,
		spanID:    spanID,
		name:      name,
		kind:      SpanKindInternal,
		startTime: time.Now(),
		attrs:     make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(s)
	}

	t.mu.Lock()
	t.spans = append(t.spans, s)
	t.mu.Unlock()

	return context.WithValue(ctx, spanContextKey{}, s), s
}

// Shutdown flushes remaining spans to the OTLP endpoint.
func (t *Tracer) Shutdown() {
	if t.exporter == nil {
		return
	}
	t.mu.Lock()
	spans := t.spans
	t.spans = nil
	t.mu.Unlock()
	if len(spans) > 0 {
		_ = t.exporter.Export(spans...)
	}
	_ = t.exporter.Shutdown()
}

// SpanOption configures a span.
type SpanOption func(*Span)

// WithSpanKind sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption {
	return func(s *Span) { s.kind = kind }
}

// Span represents a single operation.
type Span struct {
	traceID   [16]byte
	spanID    [8]byte
	name      string
	kind      SpanKind
	startTime time.Time
	endTime   time.Time
	status    StatusCode
	statusMsg string
	attrs     map[string]interface{}
	events    []Event
	ended     bool
	mu        sync.Mutex
}

// Event is a timed annotation.
type Event struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]string
}

// End marks the span as complete.
func (s *Span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ended = true
	s.endTime = time.Now()
}

// SetAttribute sets a span attribute.
func (s *Span) SetAttribute(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs[key] = value
}

// RecordError records an error on the span.
func (s *Span) RecordError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, Event{
		Name:      "exception",
		Timestamp: time.Now(),
		Attributes: map[string]string{
			"exception.message": err.Error(),
		},
	})
}

// AddEvent adds a timed event.
func (s *Span) AddEvent(name string, attrs map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, Event{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	})
}

// SetStatus sets the span status.
func (s *Span) SetStatus(code StatusCode, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = code
	s.statusMsg = message
}
