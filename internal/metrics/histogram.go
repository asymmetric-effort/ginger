package metrics

import (
	"fmt"
	"math"
	"sync"
)

// DefaultBuckets are the default histogram bucket boundaries.
var DefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// Histogram tracks the distribution of observed values.
type Histogram struct {
	desc   Desc
	labels []string
	bounds []float64

	mu     sync.RWMutex
	values map[LabelSet]*histogramValue
}

type histogramValue struct {
	mu     sync.Mutex
	bounds []float64
	counts []uint64 // one per bucket + overflow
	sum    float64
	count  uint64
	labels []Label
}

// NewHistogram creates a new Histogram with the given bucket boundaries.
// If buckets is nil, DefaultBuckets is used.
func NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	if len(labels) > maxLabelsPerMetric {
		panic(fmt.Sprintf("too many labels (%d > %d) for histogram %s", len(labels), maxLabelsPerMetric, name))
	}
	if buckets == nil {
		buckets = DefaultBuckets
	}
	return &Histogram{
		desc:   Desc{Name: name, Help: help, Type: MetricTypeHistogram},
		labels: labels,
		bounds: buckets,
		values: make(map[LabelSet]*histogramValue),
	}
}

// Observe records a value in the histogram.
func (h *Histogram) Observe(v float64, labelValues ...string) {
	hv := h.getOrCreate(labelValues)
	hv.mu.Lock()
	defer hv.mu.Unlock()

	hv.sum += v
	hv.count++
	for i, bound := range hv.bounds {
		if v <= bound {
			hv.counts[i]++
		}
	}
	// +Inf bucket (last element)
	hv.counts[len(hv.bounds)]++
}

// Desc returns the metric descriptor.
func (h *Histogram) Desc() Desc {
	return h.desc
}

// Collect returns all current metric values.
func (h *Histogram) Collect() []Metric {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.values) == 0 {
		buckets := make([]HistogramBucket, len(h.bounds)+1)
		for i, b := range h.bounds {
			buckets[i] = HistogramBucket{UpperBound: b, Count: 0}
		}
		buckets[len(h.bounds)] = HistogramBucket{UpperBound: math.Inf(1), Count: 0}
		return []Metric{{
			Labels:  nil,
			Value:   0,
			Count:   0,
			Type:    MetricTypeHistogram,
			Buckets: buckets,
		}}
	}

	metrics := make([]Metric, 0, len(h.values))
	for _, hv := range h.values {
		hv.mu.Lock()
		buckets := make([]HistogramBucket, len(hv.bounds)+1)
		for i, b := range hv.bounds {
			buckets[i] = HistogramBucket{UpperBound: b, Count: hv.counts[i]}
		}
		buckets[len(hv.bounds)] = HistogramBucket{UpperBound: math.Inf(1), Count: hv.counts[len(hv.bounds)]}
		m := Metric{
			Labels:  hv.labels,
			Value:   hv.sum,
			Count:   hv.count,
			Type:    MetricTypeHistogram,
			Buckets: buckets,
		}
		hv.mu.Unlock()
		metrics = append(metrics, m)
	}
	return metrics
}

func (h *Histogram) getOrCreate(labelValues []string) *histogramValue {
	labels := h.makeLabels(labelValues)
	key := MakeLabelSet(labels)

	h.mu.RLock()
	hv, ok := h.values[key]
	h.mu.RUnlock()
	if ok {
		return hv
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	hv, ok = h.values[key]
	if ok {
		return hv
	}
	hv = &histogramValue{
		bounds: h.bounds,
		counts: make([]uint64, len(h.bounds)+1),
		labels: labels,
	}
	h.values[key] = hv
	return hv
}

func (h *Histogram) makeLabels(values []string) []Label {
	labels := make([]Label, len(h.labels))
	for i, name := range h.labels {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		labels[i] = Label{Name: name, Value: val}
	}
	return labels
}
