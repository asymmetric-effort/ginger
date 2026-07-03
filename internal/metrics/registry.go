package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

const maxLabelsPerMetric = 20

// MetricType identifies the type of a metric.
type MetricType int

const (
	// MetricTypeCounter is a monotonically increasing value.
	MetricTypeCounter MetricType = iota
	// MetricTypeGauge is a value that can go up and down.
	MetricTypeGauge
	// MetricTypeHistogram is a distribution of observed values.
	MetricTypeHistogram
)

func (t MetricType) String() string {
	switch t {
	case MetricTypeCounter:
		return "counter"
	case MetricTypeGauge:
		return "gauge"
	case MetricTypeHistogram:
		return "histogram"
	default:
		return "untyped"
	}
}

// Registry holds all registered metric collectors.
type Registry struct {
	mu      sync.RWMutex
	metrics map[string]Collector
	order   []string
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		metrics: make(map[string]Collector),
	}
}

// Register adds a collector to the registry. Returns an error if the name is already registered.
func (r *Registry) Register(c Collector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := c.Desc().Name
	if _, exists := r.metrics[name]; exists {
		return fmt.Errorf("metric %q already registered", name)
	}
	r.metrics[name] = c
	r.order = append(r.order, name)
	return nil
}

// MustRegister registers a collector and panics on error.
func (r *Registry) MustRegister(c Collector) {
	if err := r.Register(c); err != nil {
		panic(err)
	}
}

// WriteTo writes all metrics in Prometheus text exposition format to the writer.
func (r *Registry) WriteTo(w io.Writer) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range r.order {
		c := r.metrics[name]
		desc := c.Desc()

		// TYPE line
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n", desc.Name, desc.Help); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "# TYPE %s %s\n", desc.Name, desc.Type.String()); err != nil {
			return err
		}

		// Metric lines
		for _, m := range c.Collect() {
			line := formatMetric(desc.Name, m)
			if _, err := io.WriteString(w, line); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatMetric(name string, m Metric) string {
	var b strings.Builder
	switch m.Type {
	case MetricTypeHistogram:
		for _, bucket := range m.Buckets {
			b.WriteString(name)
			b.WriteString("_bucket{")
			writeLabelsWithExtra(&b, m.Labels, "le", formatFloat(bucket.UpperBound))
			b.WriteString("} ")
			b.WriteString(formatUint(bucket.Count))
			b.WriteByte('\n')
		}
		b.WriteString(name)
		b.WriteString("_sum")
		writeLabels(&b, m.Labels)
		b.WriteByte(' ')
		b.WriteString(formatFloat(m.Value))
		b.WriteByte('\n')
		b.WriteString(name)
		b.WriteString("_count")
		writeLabels(&b, m.Labels)
		b.WriteByte(' ')
		b.WriteString(formatUint(m.Count))
		b.WriteByte('\n')
	default:
		b.WriteString(name)
		writeLabels(&b, m.Labels)
		b.WriteByte(' ')
		b.WriteString(formatFloat(m.Value))
		b.WriteByte('\n')
	}
	return b.String()
}

func writeLabels(b *strings.Builder, labels []Label) {
	if len(labels) == 0 {
		return
	}
	b.WriteByte('{')
	for i, l := range labels {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(l.Name)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(l.Value))
		b.WriteByte('"')
	}
	b.WriteByte('}')
}

func writeLabelsWithExtra(b *strings.Builder, labels []Label, extraKey, extraVal string) {
	for _, l := range labels {
		b.WriteString(l.Name)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(l.Value))
		b.WriteString(`",`)
	}
	b.WriteString(extraKey)
	b.WriteString(`="`)
	b.WriteString(extraVal)
	b.WriteByte('"')
}

func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}

func formatUint(u uint64) string {
	return fmt.Sprintf("%d", u)
}

// Label is a name-value pair for metric labeling.
type Label struct {
	Name  string
	Value string
}

// LabelSet is a sorted set of labels used as a map key.
type LabelSet string

// MakeLabelSet creates a LabelSet from labels (sorted by name).
func MakeLabelSet(labels []Label) LabelSet {
	sorted := make([]Label, len(labels))
	copy(sorted, labels)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	var b strings.Builder
	for i, l := range sorted {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteString(l.Value)
	}
	return LabelSet(b.String())
}

// Desc describes a metric.
type Desc struct {
	Name string
	Help string
	Type MetricType
}

// Metric is a single measurement with optional labels and histogram buckets.
type Metric struct {
	Labels  []Label
	Value   float64
	Count   uint64
	Type    MetricType
	Buckets []HistogramBucket
}

// HistogramBucket is a cumulative histogram bucket.
type HistogramBucket struct {
	UpperBound float64
	Count      uint64
}

// Collector is the interface for metric collectors.
type Collector interface {
	Desc() Desc
	Collect() []Metric
}
