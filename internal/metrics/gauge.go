package metrics

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

// Gauge is a metric that can go up and down.
type Gauge struct {
	desc   Desc
	labels []string

	mu     sync.RWMutex
	values map[LabelSet]*gaugeValue
}

type gaugeValue struct {
	bits   atomic.Uint64
	labels []Label
}

// NewGauge creates a new Gauge.
func NewGauge(name, help string, labels ...string) *Gauge {
	if len(labels) > maxLabelsPerMetric {
		panic(fmt.Sprintf("too many labels (%d > %d) for gauge %s", len(labels), maxLabelsPerMetric, name))
	}
	return &Gauge{
		desc:   Desc{Name: name, Help: help, Type: MetricTypeGauge},
		labels: labels,
		values: make(map[LabelSet]*gaugeValue),
	}
}

// Set sets the gauge to the given value.
func (g *Gauge) Set(v float64, labelValues ...string) {
	gv := g.getOrCreate(labelValues)
	gv.bits.Store(math.Float64bits(v))
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc(labelValues ...string) {
	g.Add(1, labelValues...)
}

// Dec decrements the gauge by 1.
func (g *Gauge) Dec(labelValues ...string) {
	g.Add(-1, labelValues...)
}

// Add adds the given value to the gauge.
func (g *Gauge) Add(v float64, labelValues ...string) {
	gv := g.getOrCreate(labelValues)
	for {
		old := gv.bits.Load()
		newVal := math.Float64frombits(old) + v
		if gv.bits.CompareAndSwap(old, math.Float64bits(newVal)) {
			return
		}
	}
}

// Desc returns the metric descriptor.
func (g *Gauge) Desc() Desc {
	return g.desc
}

// Collect returns all current metric values.
func (g *Gauge) Collect() []Metric {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.values) == 0 {
		return []Metric{{
			Labels: nil,
			Value:  0,
			Type:   MetricTypeGauge,
		}}
	}

	metrics := make([]Metric, 0, len(g.values))
	for _, gv := range g.values {
		metrics = append(metrics, Metric{
			Labels: gv.labels,
			Value:  math.Float64frombits(gv.bits.Load()),
			Type:   MetricTypeGauge,
		})
	}
	return metrics
}

func (g *Gauge) getOrCreate(labelValues []string) *gaugeValue {
	labels := g.makeLabels(labelValues)
	key := MakeLabelSet(labels)

	g.mu.RLock()
	gv, ok := g.values[key]
	g.mu.RUnlock()
	if ok {
		return gv
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	gv, ok = g.values[key]
	if ok {
		return gv
	}
	gv = &gaugeValue{labels: labels}
	g.values[key] = gv
	return gv
}

func (g *Gauge) makeLabels(values []string) []Label {
	labels := make([]Label, len(g.labels))
	for i, name := range g.labels {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		labels[i] = Label{Name: name, Value: val}
	}
	return labels
}
