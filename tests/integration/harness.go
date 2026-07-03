package integration

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/asymmetric-effort/ginger/internal/health"
	"github.com/asymmetric-effort/ginger/internal/pipeline"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/query"
	"github.com/asymmetric-effort/ginger/internal/storage"
	"github.com/asymmetric-effort/ginger/internal/storage/memory"
)

// Harness provides a test environment with in-memory ginger components.
type Harness struct {
	Backend     *memory.Backend
	Pipeline    *pipeline.Pipeline
	QuerySvc    *query.Service
	HTTPHandler *query.HTTPHandler
	Health      *health.Handler
	Server      *http.Server
	Addr        string
	t           *testing.T
}

// NewHarness creates and starts a test harness.
func NewHarness(t *testing.T) *Harness {
	t.Helper()
	backend := memory.NewBackend(10000)

	// Create pipeline with storage exporter
	p := pipeline.New(pipeline.Config{QueueSize: 100})
	exp := &storageExporter{writer: backend}
	p.AddExporter(exp)

	querySvc := query.NewService(backend, backend, nil)
	httpHandler := query.NewHTTPHandler(querySvc)
	healthHandler := health.NewHandler()
	healthHandler.SetReady(true)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)
	mux.HandleFunc("/health/live", healthHandler.LiveHandler())
	mux.HandleFunc("/health/ready", healthHandler.ReadyHandler())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)

	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	h := &Harness{
		Backend:     backend,
		Pipeline:    p,
		QuerySvc:    querySvc,
		HTTPHandler: httpHandler,
		Health:      healthHandler,
		Server:      srv,
		Addr:        ln.Addr().String(),
		t:           t,
	}

	t.Cleanup(func() {
		p.Shutdown(context.Background())
		srv.Close()
	})

	return h
}

// SendTrace submits a trace through the pipeline.
func (h *Harness) SendTrace(td otlp.TracesData) error {
	return h.Pipeline.ConsumeTraces(context.Background(), td)
}

// WaitForService polls until the service appears or timeout.
func (h *Harness) WaitForService(service string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		services, _ := h.Backend.GetServices(context.Background())
		for _, s := range services {
			if s == service {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// QueryTrace retrieves a trace by ID.
func (h *Harness) QueryTrace(traceID storage.TraceID) (otlp.TracesData, error) {
	return h.QuerySvc.GetTrace(context.Background(), traceID)
}

type storageExporter struct {
	writer storage.TraceWriter
}

func (e *storageExporter) ExportTraces(ctx context.Context, td otlp.TracesData) error {
	return e.writer.WriteSpans(ctx, td)
}

func (e *storageExporter) Shutdown(_ context.Context) error { return nil }
