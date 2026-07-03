package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Counter is a monotonically increasing metric.
type Counter struct {
	desc   Desc
	labels []string

	mu     sync.RWMutex
	values map[LabelSet]*counterValue
}

type counterValue struct {
	val    atomic.Int64 // stored as fixed-point (multiply by 1000 for 3 decimal places)
	labels []Label
}

// NewCounter creates a new Counter.
func NewCounter(name, help string, labels ...string) *Counter {
	if len(labels) > maxLabelsPerMetric {
		panic(fmt.Sprintf("too many labels (%d > %d) for counter %s", len(labels), maxLabelsPerMetric, name))
	}
	return &Counter{
		desc:   Desc{Name: name, Help: help, Type: MetricTypeCounter},
		labels: labels,
		values: make(map[LabelSet]*counterValue),
	}
}

// Inc increments the counter by 1 for the given label values.
func (c *Counter) Inc(labelValues ...string) {
	c.Add(1, labelValues...)
}

// Add adds the given value to the counter.
func (c *Counter) Add(v float64, labelValues ...string) {
	if v < 0 {
		return // counters can only increase
	}
	cv := c.getOrCreate(labelValues)
	cv.val.Add(int64(v * 1000))
}

// Desc returns the metric descriptor.
func (c *Counter) Desc() Desc {
	return c.desc
}

// Collect returns all current metric values.
func (c *Counter) Collect() []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.values) == 0 {
		return []Metric{{
			Labels: nil,
			Value:  0,
			Type:   MetricTypeCounter,
		}}
	}

	metrics := make([]Metric, 0, len(c.values))
	for _, cv := range c.values {
		metrics = append(metrics, Metric{
			Labels: cv.labels,
			Value:  float64(cv.val.Load()) / 1000,
			Type:   MetricTypeCounter,
		})
	}
	return metrics
}

func (c *Counter) getOrCreate(labelValues []string) *counterValue {
	labels := c.makeLabels(labelValues)
	key := MakeLabelSet(labels)

	c.mu.RLock()
	cv, ok := c.values[key]
	c.mu.RUnlock()
	if ok {
		return cv
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	cv, ok = c.values[key]
	if ok {
		return cv
	}
	cv = &counterValue{labels: labels}
	c.values[key] = cv
	return cv
}

func (c *Counter) makeLabels(values []string) []Label {
	labels := make([]Label, len(c.labels))
	for i, name := range c.labels {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		labels[i] = Label{Name: name, Value: val}
	}
	return labels
}
