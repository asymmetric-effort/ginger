package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// AdaptiveSamplingConfig configures the adaptive sampling processor.
type AdaptiveSamplingConfig struct {
	TargetSamplesPerSecond float64
	InitialSamplingRate    float64
	AdjustInterval         time.Duration
}

// AdaptiveSamplingProcessor adjusts sampling rate based on throughput.
type AdaptiveSamplingProcessor struct {
	config   AdaptiveSamplingConfig
	mu       sync.RWMutex
	rates    map[string]float64 // service → probability
	counters map[string]*atomic.Int64
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewAdaptiveSamplingProcessor creates an adaptive head-based sampler.
func NewAdaptiveSamplingProcessor(config AdaptiveSamplingConfig) *AdaptiveSamplingProcessor {
	if config.TargetSamplesPerSecond <= 0 {
		config.TargetSamplesPerSecond = 1
	}
	if config.InitialSamplingRate <= 0 || config.InitialSamplingRate > 1 {
		config.InitialSamplingRate = 1.0
	}
	if config.AdjustInterval <= 0 {
		config.AdjustInterval = 10 * time.Second
	}
	return &AdaptiveSamplingProcessor{
		config:   config,
		rates:    make(map[string]float64),
		counters: make(map[string]*atomic.Int64),
	}
}

// Start begins the adjustment loop.
func (as *AdaptiveSamplingProcessor) Start(ctx context.Context) {
	ctx, as.cancel = context.WithCancel(ctx)
	as.wg.Add(1)
	go as.adjustLoop(ctx)
}

// ProcessTraces samples spans based on per-service probability.
func (as *AdaptiveSamplingProcessor) ProcessTraces(_ context.Context, td otlp.TracesData) (otlp.TracesData, error) {
	var result otlp.TracesData
	for _, rs := range td.ResourceSpans {
		service := ""
		if v, ok := rs.Resource.Attributes.Get("service.name"); ok {
			service = v.Str
		}

		// Track throughput
		as.trackThroughput(service, rs)

		as.mu.RLock()
		rate, ok := as.rates[service]
		as.mu.RUnlock()
		if !ok {
			rate = as.config.InitialSamplingRate
		}

		var filteredSS []otlp.ScopeSpans
		for _, ss := range rs.ScopeSpans {
			var kept []otlp.Span
			for _, span := range ss.Spans {
				if shouldSample(span.TraceID, rate) {
					kept = append(kept, span)
				}
			}
			if len(kept) > 0 {
				filteredSS = append(filteredSS, otlp.ScopeSpans{Scope: ss.Scope, Spans: kept})
			}
		}
		if len(filteredSS) > 0 {
			result.ResourceSpans = append(result.ResourceSpans, otlp.ResourceSpans{
				Resource:   rs.Resource,
				ScopeSpans: filteredSS,
			})
		}
	}
	return result, nil
}

// GetRate returns the current sampling rate for a service.
func (as *AdaptiveSamplingProcessor) GetRate(service string) float64 {
	as.mu.RLock()
	defer as.mu.RUnlock()
	if r, ok := as.rates[service]; ok {
		return r
	}
	return as.config.InitialSamplingRate
}

// Shutdown stops the adjustment loop.
func (as *AdaptiveSamplingProcessor) Shutdown() {
	if as.cancel != nil {
		as.cancel()
	}
	as.wg.Wait()
}

func (as *AdaptiveSamplingProcessor) trackThroughput(service string, rs otlp.ResourceSpans) {
	count := int64(0)
	for _, ss := range rs.ScopeSpans {
		count += int64(len(ss.Spans))
	}
	as.mu.Lock()
	c, ok := as.counters[service]
	if !ok {
		c = &atomic.Int64{}
		as.counters[service] = c
	}
	as.mu.Unlock()
	c.Add(count)
}

func (as *AdaptiveSamplingProcessor) adjustLoop(ctx context.Context) {
	defer as.wg.Done()
	ticker := time.NewTicker(as.config.AdjustInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			as.adjust()
		}
	}
}

func (as *AdaptiveSamplingProcessor) adjust() {
	as.mu.Lock()
	defer as.mu.Unlock()

	interval := as.config.AdjustInterval.Seconds()
	target := as.config.TargetSamplesPerSecond

	for service, counter := range as.counters {
		observed := float64(counter.Swap(0)) / interval
		if observed <= 0 {
			continue
		}

		currentRate, ok := as.rates[service]
		if !ok {
			currentRate = as.config.InitialSamplingRate
		}

		// Simple proportional control: adjust rate to hit target QPS
		desiredRate := target / observed
		// Blend with current rate to avoid oscillation
		newRate := currentRate*0.7 + desiredRate*0.3
		newRate = math.Max(0.0001, math.Min(1.0, newRate))
		as.rates[service] = newRate
	}
}

// shouldSample deterministically decides based on trace ID hash.
func shouldSample(traceID [16]byte, rate float64) bool {
	if rate >= 1.0 {
		return true
	}
	if rate <= 0 {
		return false
	}
	h := sha256.Sum256(traceID[:])
	v := binary.BigEndian.Uint64(h[:8])
	threshold := uint64(rate * float64(math.MaxUint64))
	return v < threshold
}
