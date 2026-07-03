package sampling

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

// StrategyType identifies the sampling strategy type.
type StrategyType string

const (
	StrategyProbabilistic StrategyType = "PROBABILISTIC"
	StrategyRateLimiting  StrategyType = "RATE_LIMITING"
)

// Strategy describes a sampling strategy for a service.
type Strategy struct {
	Type                    StrategyType `json:"strategyType"`
	ProbabilisticSampling   *ProbabilisticStrategy `json:"probabilisticSampling,omitempty"`
	RateLimitingSampling    *RateLimitingStrategy  `json:"rateLimitingSampling,omitempty"`
	OperationSampling       *PerOperationStrategies `json:"operationSampling,omitempty"`
}

// ProbabilisticStrategy samples at a fixed rate.
type ProbabilisticStrategy struct {
	SamplingRate float64 `json:"samplingRate"`
}

// RateLimitingStrategy limits traces per second.
type RateLimitingStrategy struct {
	MaxTracesPerSecond int32 `json:"maxTracesPerSecond"`
}

// PerOperationStrategies provides per-operation sampling.
type PerOperationStrategies struct {
	DefaultSamplingProbability float64                `json:"defaultSamplingProbability"`
	PerOperationStrategies     []OperationStrategy    `json:"perOperationStrategies,omitempty"`
}

// OperationStrategy is per-operation probabilistic sampling.
type OperationStrategy struct {
	Operation            string                 `json:"operation"`
	ProbabilisticSampling ProbabilisticStrategy `json:"probabilisticSampling"`
}

// StrategyProvider returns sampling strategies for services.
type StrategyProvider interface {
	GetSamplingStrategy(service string) (*Strategy, error)
}

// FileProvider loads strategies from a JSON file.
type FileProvider struct {
	mu         sync.RWMutex
	strategies *strategiesFile
	filePath   string
}

type strategiesFile struct {
	DefaultStrategy    *Strategy         `json:"default_strategy"`
	ServiceStrategies  []serviceStrategy `json:"service_strategies"`
}

type serviceStrategy struct {
	Service  string  `json:"service"`
	Type     string  `json:"type"`
	Param    float64 `json:"param"`
}

// NewFileProvider creates a FileProvider from the given JSON file.
func NewFileProvider(path string) (*FileProvider, error) {
	fp := &FileProvider{filePath: path}
	if err := fp.load(); err != nil {
		return nil, err
	}
	return fp, nil
}

// GetSamplingStrategy returns the strategy for a service.
func (fp *FileProvider) GetSamplingStrategy(service string) (*Strategy, error) {
	fp.mu.RLock()
	defer fp.mu.RUnlock()

	for _, ss := range fp.strategies.ServiceStrategies {
		if ss.Service == service {
			return serviceStrategyToStrategy(ss), nil
		}
	}

	if fp.strategies.DefaultStrategy != nil {
		return fp.strategies.DefaultStrategy, nil
	}

	return &Strategy{
		Type: StrategyProbabilistic,
		ProbabilisticSampling: &ProbabilisticStrategy{SamplingRate: 1.0},
	}, nil
}

// Reload reloads the strategies file. Retains old config on error.
func (fp *FileProvider) Reload() error {
	return fp.load()
}

// WatchFile polls the file for changes at the given interval.
func (fp *FileProvider) WatchFile(interval time.Duration, done <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			fp.load() // silently reload; errors keep old config
		}
	}
}

func (fp *FileProvider) load() error {
	data, err := os.ReadFile(fp.filePath)
	if err != nil {
		return err
	}
	var sf strategiesFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return err
	}
	fp.mu.Lock()
	fp.strategies = &sf
	fp.mu.Unlock()
	return nil
}

func serviceStrategyToStrategy(ss serviceStrategy) *Strategy {
	switch ss.Type {
	case "rate_limiting":
		return &Strategy{
			Type: StrategyRateLimiting,
			RateLimitingSampling: &RateLimitingStrategy{MaxTracesPerSecond: int32(ss.Param)},
		}
	default: // probabilistic
		return &Strategy{
			Type: StrategyProbabilistic,
			ProbabilisticSampling: &ProbabilisticStrategy{SamplingRate: ss.Param},
		}
	}
}

// Handler serves the remote sampling HTTP endpoint.
type Handler struct {
	provider StrategyProvider
}

// NewHandler creates a sampling HTTP handler.
func NewHandler(provider StrategyProvider) *Handler {
	return &Handler{provider: provider}
}

// ServeHTTP handles GET /sampling?service=<name>.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service == "" {
		http.Error(w, "service parameter required", http.StatusBadRequest)
		return
	}

	strategy, err := h.provider.GetSamplingStrategy(service)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(strategy)
}
