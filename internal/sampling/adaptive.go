package sampling

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// AdaptiveEngine tracks per-service throughput and computes probabilities.
type AdaptiveEngine struct {
	mu          sync.RWMutex
	target      float64
	window      map[string]*throughputWindow
	probabilities map[string]float64
}

type throughputWindow struct {
	count atomic.Int64
}

// NewAdaptiveEngine creates an adaptive sampling engine.
func NewAdaptiveEngine(targetQPS float64) *AdaptiveEngine {
	if targetQPS <= 0 {
		targetQPS = 1
	}
	return &AdaptiveEngine{
		target:        targetQPS,
		window:        make(map[string]*throughputWindow),
		probabilities: make(map[string]float64),
	}
}

// RecordSpan records a span observation for throughput tracking.
func (ae *AdaptiveEngine) RecordSpan(service string) {
	ae.mu.RLock()
	w, ok := ae.window[service]
	ae.mu.RUnlock()
	if !ok {
		ae.mu.Lock()
		w, ok = ae.window[service]
		if !ok {
			w = &throughputWindow{}
			ae.window[service] = w
		}
		ae.mu.Unlock()
	}
	w.count.Add(1)
}

// Compute recalculates probabilities based on observed throughput.
func (ae *AdaptiveEngine) Compute(interval time.Duration) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	secs := interval.Seconds()
	if secs <= 0 {
		secs = 1
	}

	for service, w := range ae.window {
		observed := float64(w.count.Swap(0)) / secs
		if observed <= 0 {
			continue
		}

		current, ok := ae.probabilities[service]
		if !ok {
			current = 1.0
		}

		desired := ae.target / observed
		newRate := current*0.7 + desired*0.3
		newRate = math.Max(0.0001, math.Min(1.0, newRate))
		ae.probabilities[service] = newRate
	}
}

// GetProbability returns the current sampling probability for a service.
func (ae *AdaptiveEngine) GetProbability(service string) float64 {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	if p, ok := ae.probabilities[service]; ok {
		return p
	}
	return 1.0
}

// GetAllProbabilities returns all current probabilities.
func (ae *AdaptiveEngine) GetAllProbabilities() map[string]float64 {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	out := make(map[string]float64, len(ae.probabilities))
	for k, v := range ae.probabilities {
		out[k] = v
	}
	return out
}
