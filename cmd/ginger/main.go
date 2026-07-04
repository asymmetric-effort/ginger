package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asymmetric-effort/ginger/internal/config"
	"github.com/asymmetric-effort/ginger/internal/health"
	"github.com/asymmetric-effort/ginger/internal/logging"
	"github.com/asymmetric-effort/ginger/internal/pipeline"
	"github.com/asymmetric-effort/ginger/internal/protocol/jaeger"
	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
	"github.com/asymmetric-effort/ginger/internal/protocol/zipkin"
	"github.com/asymmetric-effort/ginger/internal/query"
	"github.com/asymmetric-effort/ginger/internal/sampling"
	"github.com/asymmetric-effort/ginger/internal/storage"
	"github.com/asymmetric-effort/ginger/internal/storage/memory"
	"github.com/asymmetric-effort/ginger/internal/ui"
	"github.com/asymmetric-effort/ginger/internal/version"
)

// ServerConfig holds configuration for the ginger server.
type ServerConfig struct {
	OTLPHTTPPort int    `yaml:"otlp_http_port"`
	QueryPort    int    `yaml:"query_port"`
	AdminPort    int    `yaml:"admin_port"`
	MaxTraces    int    `yaml:"max_traces"`
	MaxBodyBytes int64  `yaml:"max_body_bytes"`
	LogLevel     string `yaml:"log_level"`
	LogFormat    string `yaml:"log_format"`
	PprofEnabled bool   `yaml:"pprof_enabled"`
	SamplingFile string `yaml:"sampling_file"`
	QueueSize    int    `yaml:"queue_size"`
	BatchSize    int    `yaml:"batch_size"`
	BatchDelay   string `yaml:"batch_delay"`
	DrainTimeout string `yaml:"drain_timeout"`
}

func defaultConfig() ServerConfig {
	return ServerConfig{
		OTLPHTTPPort: 4318,
		QueryPort:    16686,
		AdminPort:    14269,
		MaxTraces:    10000,
		MaxBodyBytes: 4 * 1024 * 1024,
		LogLevel:     "info",
		LogFormat:    "json",
		PprofEnabled: false,
		QueueSize:    1024,
		BatchSize:    512,
		BatchDelay:   "200ms",
		DrainTimeout: "5s",
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.String())
		return
	}

	configPath := flag.String("config", "", "path to YAML config file")
	flag.Parse()

	cfg := defaultConfig()

	if *configPath != "" {
		if err := loadConfig(*configPath, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	// Create a context that cancels on SIGINT/SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := runWithContext(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(path string, cfg *ServerConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	return config.Decode(data, cfg)
}

// storageExporter adapts a storage.TraceWriter into a pipeline.Exporter.
type storageExporter struct {
	writer storage.TraceWriter
}

func (e *storageExporter) ExportTraces(ctx context.Context, td otlp.TracesData) error {
	return e.writer.WriteSpans(ctx, td)
}

func (e *storageExporter) Shutdown(_ context.Context) error {
	return nil
}

func runWithContext(ctx context.Context, cfg ServerConfig) error {
	// 1. Create structured logger
	logLevel := parseLogLevel(cfg.LogLevel)
	logger := logging.New(logging.Config{
		Level:  logLevel,
		Format: cfg.LogFormat,
	})
	logger.Info("starting ginger server",
		logging.String("version", version.String()),
		logging.Int("otlp_http_port", cfg.OTLPHTTPPort),
		logging.Int("query_port", cfg.QueryPort),
		logging.Int("admin_port", cfg.AdminPort),
	)

	// 2. Create storage backend (memory)
	store := memory.NewBackend(cfg.MaxTraces)
	defer store.Close()

	// 3. Create storage exporter (adapts storage.TraceWriter to pipeline.Exporter)
	exporter := &storageExporter{writer: store.TraceWriter()}

	// 4. Create batch processor
	batchDelay, _ := time.ParseDuration(cfg.BatchDelay)
	if batchDelay <= 0 {
		batchDelay = 200 * time.Millisecond
	}
	batchProc := pipeline.NewBatchProcessor(pipeline.BatchConfig{
		MaxBatchSize: cfg.BatchSize,
		SendDelay:    batchDelay,
	}, exporter)

	// 5. Create pipeline
	drainTimeout, _ := time.ParseDuration(cfg.DrainTimeout)
	if drainTimeout <= 0 {
		drainTimeout = 5 * time.Second
	}
	pipe := pipeline.New(pipeline.Config{
		QueueSize:    cfg.QueueSize,
		DrainTimeout: drainTimeout,
	})
	pipe.AddProcessor(batchProc)
	pipe.AddExporter(exporter)

	// 6. Create receivers
	otlpReceiver := otlp.NewHTTPReceiver(pipe, cfg.MaxBodyBytes)
	zipkinReceiver := zipkin.NewHTTPReceiver(pipe, cfg.MaxBodyBytes)
	jaegerReceiver := jaeger.NewThriftHTTPReceiver(pipe, cfg.MaxBodyBytes)

	// 7. Create query service and HTTP handler
	querySvc := query.NewService(store.TraceReader(), store.DependencyReader(), nil)
	queryHandler := query.NewHTTPHandler(querySvc)

	// 8. Create health handler
	healthHandler := health.NewHandler()

	// 9. Create sampling handler
	var samplingHandler http.Handler
	if cfg.SamplingFile != "" {
		provider, err := sampling.NewFileProvider(cfg.SamplingFile)
		if err != nil {
			logger.Warn("failed to load sampling strategies, using default",
				logging.String("file", cfg.SamplingFile),
				logging.NamedError("err", err),
			)
			samplingHandler = sampling.NewHandler(&defaultSamplingProvider{})
		} else {
			samplingHandler = sampling.NewHandler(provider)
		}
	} else {
		samplingHandler = sampling.NewHandler(&defaultSamplingProvider{})
	}

	// 10. Create UI handler
	uiHandler, err := ui.NewHandler()
	if err != nil {
		logger.Warn("failed to create UI handler", logging.NamedError("err", err))
	}

	// 11. Build HTTP muxes
	// Receiver mux (OTLP + Zipkin + Jaeger ingest)
	receiverMux := http.NewServeMux()
	receiverMux.Handle("/v1/traces", otlpReceiver)
	receiverMux.Handle("/api/v2/spans", zipkinReceiver)
	receiverMux.Handle("/api/traces", jaegerReceiver)

	// Query mux (API + UI)
	queryMux := http.NewServeMux()
	queryHandler.RegisterRoutes(queryMux)
	queryMux.Handle("/api/sampling", samplingHandler)
	if uiHandler != nil {
		queryMux.Handle("/", uiHandler)
	}

	// Admin mux (health + pprof)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/health/live", healthHandler.LiveHandler())
	adminMux.HandleFunc("/health/ready", healthHandler.ReadyHandler())
	if cfg.PprofEnabled {
		health.RegisterPprof(adminMux)
	}

	// 12. Create HTTP servers
	receiverServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.OTLPHTTPPort),
		Handler:      receiverMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	queryServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.QueryPort),
		Handler:      queryMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	adminServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AdminPort),
		Handler:      adminMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 13. Start pipeline
	pipeCtx, pipeCancel := context.WithCancel(context.Background())
	defer pipeCancel()

	if err := pipe.Start(pipeCtx); err != nil {
		return fmt.Errorf("start pipeline: %w", err)
	}

	// Mark service as ready
	healthHandler.SetReady(true)

	// Start HTTP servers in goroutines
	errCh := make(chan error, 3)
	go func() {
		logger.Info("receiver server listening", logging.Int("port", cfg.OTLPHTTPPort))
		if err := receiverServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("receiver server: %w", err)
		}
	}()
	go func() {
		logger.Info("query server listening", logging.Int("port", cfg.QueryPort))
		if err := queryServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("query server: %w", err)
		}
	}()
	go func() {
		logger.Info("admin server listening", logging.Int("port", cfg.AdminPort))
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("admin server: %w", err)
		}
	}()

	// 14. Wait for context cancellation (signal) or server error
	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-errCh:
		logger.Error("server error", logging.NamedError("err", err))
		return err
	}

	healthHandler.SetReady(false)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP servers
	if err := receiverServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("receiver server shutdown error", logging.NamedError("err", err))
	}
	if err := queryServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("query server shutdown error", logging.NamedError("err", err))
	}
	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("admin server shutdown error", logging.NamedError("err", err))
	}

	// Shutdown pipeline
	if err := pipe.Shutdown(shutdownCtx); err != nil {
		logger.Error("pipeline shutdown error", logging.NamedError("err", err))
	}

	logger.Info("shutdown complete")
	return nil
}

func parseLogLevel(s string) logging.Level {
	switch s {
	case "debug":
		return logging.LevelDebug
	case "warn":
		return logging.LevelWarn
	case "error":
		return logging.LevelError
	default:
		return logging.LevelInfo
	}
}

// defaultSamplingProvider returns a default probabilistic strategy.
type defaultSamplingProvider struct{}

func (d *defaultSamplingProvider) GetSamplingStrategy(_ string) (*sampling.Strategy, error) {
	return &sampling.Strategy{
		Type:                  sampling.StrategyProbabilistic,
		ProbabilisticSampling: &sampling.ProbabilisticStrategy{SamplingRate: 1.0},
	}, nil
}
