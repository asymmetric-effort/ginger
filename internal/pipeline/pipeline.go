package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/queue"
)

// TracesConsumer receives trace data from a receiver.
type TracesConsumer interface {
	ConsumeTraces(ctx context.Context, td otlp.TracesData) error
}

// Receiver ingests trace data from external sources.
type Receiver interface {
	Start(ctx context.Context, sink TracesConsumer) error
	Shutdown(ctx context.Context) error
}

// Processor transforms trace data passing through the pipeline.
type Processor interface {
	ProcessTraces(ctx context.Context, td otlp.TracesData) (otlp.TracesData, error)
}

// Exporter sends trace data to a storage backend or external destination.
type Exporter interface {
	ExportTraces(ctx context.Context, td otlp.TracesData) error
	Shutdown(ctx context.Context) error
}

// Config configures a pipeline.
type Config struct {
	QueueSize    int
	DrainTimeout time.Duration
}

// Pipeline wires Receivers → Processors → Exporters with bounded queues.
type Pipeline struct {
	config     Config
	receivers  []Receiver
	processors []Processor
	exporters  []Exporter
	queue      *queue.BoundedQueue[otlp.TracesData]
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// New creates a new Pipeline.
func New(config Config) *Pipeline {
	if config.QueueSize <= 0 {
		config.QueueSize = 1024
	}
	if config.DrainTimeout <= 0 {
		config.DrainTimeout = 5 * time.Second
	}
	return &Pipeline{
		config: config,
		queue:  queue.NewBoundedQueue[otlp.TracesData](config.QueueSize),
	}
}

// AddReceiver adds a receiver to the pipeline.
func (p *Pipeline) AddReceiver(r Receiver) {
	p.receivers = append(p.receivers, r)
}

// AddProcessor adds a processor to the chain.
func (p *Pipeline) AddProcessor(proc Processor) {
	p.processors = append(p.processors, proc)
}

// AddExporter adds an exporter.
func (p *Pipeline) AddExporter(exp Exporter) {
	p.exporters = append(p.exporters, exp)
}

// Start starts the pipeline: exporters first, then processors, then receivers.
func (p *Pipeline) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)

	// Start the consumer goroutine that processes the queue
	p.wg.Add(1)
	go p.consumeLoop(ctx)

	// Start receivers (they call ConsumeTraces on the pipeline)
	for _, r := range p.receivers {
		if err := r.Start(ctx, p); err != nil {
			return fmt.Errorf("start receiver: %w", err)
		}
	}

	return nil
}

// Shutdown stops the pipeline in reverse order: receivers, then drain, then exporters.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	// Stop receivers first
	for _, r := range p.receivers {
		if err := r.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown receiver: %w", err)
		}
	}

	// Close the queue and wait for consumer to drain
	p.queue.Close()
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()

	// Shutdown exporters
	for _, exp := range p.exporters {
		if err := exp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown exporter: %w", err)
		}
	}

	return nil
}

// ConsumeTraces implements TracesConsumer. Called by receivers to submit data.
func (p *Pipeline) ConsumeTraces(_ context.Context, td otlp.TracesData) error {
	if !p.queue.Enqueue(td) {
		return fmt.Errorf("pipeline queue full")
	}
	return nil
}

// QueueLen returns the current queue length.
func (p *Pipeline) QueueLen() int {
	return p.queue.Len()
}

func (p *Pipeline) consumeLoop(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			p.drain(ctx)
			return
		default:
		}
		td, ok := p.queue.Dequeue()
		if !ok {
			// Queue empty — short sleep to avoid busy-wait, then retry
			select {
			case <-ctx.Done():
				p.drain(ctx)
				return
			case <-time.After(5 * time.Millisecond):
				continue
			}
		}
		p.processAndExport(ctx, td)
	}
}

func (p *Pipeline) drain(ctx context.Context) {
	// Drain any remaining items from the queue
	for {
		td, ok := p.queue.Dequeue()
		if !ok {
			return
		}
		p.processAndExport(ctx, td)
	}
}

func (p *Pipeline) processAndExport(ctx context.Context, td otlp.TracesData) {
	var err error
	for _, proc := range p.processors {
		td, err = proc.ProcessTraces(ctx, td)
		if err != nil {
			return // drop on processor error
		}
	}

	// Fan-out to all exporters
	for _, exp := range p.exporters {
		exp.ExportTraces(ctx, td)
	}
}
