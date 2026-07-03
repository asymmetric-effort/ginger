package metrics

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestMetricTypeString(t *testing.T) {
	tests := []struct {
		t    MetricType
		want string
	}{
		{MetricTypeCounter, "counter"},
		{MetricTypeGauge, "gauge"},
		{MetricTypeHistogram, "histogram"},
		{MetricType(99), "untyped"},
	}
	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("MetricType(%d).String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestCounterBasic(t *testing.T) {
	c := NewCounter("test_counter", "A test counter")
	c.Inc()
	c.Inc()
	c.Add(3)

	metrics := c.Collect()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 5 {
		t.Errorf("counter value = %f, want 5", metrics[0].Value)
	}
}

func TestCounterWithLabels(t *testing.T) {
	c := NewCounter("http_requests", "HTTP requests", "method", "status")
	c.Inc("GET", "200")
	c.Inc("GET", "200")
	c.Inc("POST", "201")

	metrics := c.Collect()
	if len(metrics) != 2 {
		t.Fatalf("expected 2 label combinations, got %d", len(metrics))
	}
}

func TestCounterNegativeAdd(t *testing.T) {
	c := NewCounter("test", "test")
	c.Inc()
	c.Add(-1) // should be ignored

	metrics := c.Collect()
	if metrics[0].Value != 1 {
		t.Errorf("counter should ignore negative adds, got %f", metrics[0].Value)
	}
}

func TestCounterNoLabelsInitialCollect(t *testing.T) {
	c := NewCounter("empty", "empty counter")
	metrics := c.Collect()
	if len(metrics) != 1 || metrics[0].Value != 0 {
		t.Error("empty counter should return zero value")
	}
}

func TestCounterDesc(t *testing.T) {
	c := NewCounter("test_counter", "help text")
	d := c.Desc()
	if d.Name != "test_counter" || d.Help != "help text" || d.Type != MetricTypeCounter {
		t.Errorf("unexpected desc: %+v", d)
	}
}

func TestCounterConcurrent(t *testing.T) {
	c := NewCounter("concurrent", "test", "worker")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Inc("w1")
			}
		}()
	}
	wg.Wait()
}

func TestGaugeBasic(t *testing.T) {
	g := NewGauge("temperature", "Current temperature")
	g.Set(36.5)

	metrics := g.Collect()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 36.5 {
		t.Errorf("gauge value = %f, want 36.5", metrics[0].Value)
	}
}

func TestGaugeIncDec(t *testing.T) {
	g := NewGauge("connections", "Active connections")
	g.Inc()
	g.Inc()
	g.Dec()

	metrics := g.Collect()
	if metrics[0].Value != 1 {
		t.Errorf("gauge value = %f, want 1", metrics[0].Value)
	}
}

func TestGaugeAdd(t *testing.T) {
	g := NewGauge("test", "test")
	g.Add(5)
	g.Add(-3)

	metrics := g.Collect()
	if metrics[0].Value != 2 {
		t.Errorf("gauge value = %f, want 2", metrics[0].Value)
	}
}

func TestGaugeWithLabels(t *testing.T) {
	g := NewGauge("memory", "Memory usage", "pool")
	g.Set(100, "heap")
	g.Set(50, "stack")

	metrics := g.Collect()
	if len(metrics) != 2 {
		t.Fatalf("expected 2 label combinations, got %d", len(metrics))
	}
}

func TestGaugeNoLabelsInitialCollect(t *testing.T) {
	g := NewGauge("empty", "empty gauge")
	metrics := g.Collect()
	if len(metrics) != 1 || metrics[0].Value != 0 {
		t.Error("empty gauge should return zero value")
	}
}

func TestGaugeDesc(t *testing.T) {
	g := NewGauge("test_gauge", "help")
	d := g.Desc()
	if d.Name != "test_gauge" || d.Type != MetricTypeGauge {
		t.Errorf("unexpected desc: %+v", d)
	}
}

func TestGaugeConcurrent(t *testing.T) {
	g := NewGauge("concurrent", "test")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				g.Add(1)
				g.Add(-1)
			}
		}()
	}
	wg.Wait()
}

func TestHistogramBasic(t *testing.T) {
	h := NewHistogram("request_duration", "Request duration", []float64{0.1, 0.5, 1.0, 5.0})
	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.8)
	h.Observe(3.0)
	h.Observe(10.0)

	metrics := h.Collect()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	m := metrics[0]
	if m.Count != 5 {
		t.Errorf("count = %d, want 5", m.Count)
	}
	// sum = 0.05+0.3+0.8+3.0+10.0 = 14.15
	if m.Value < 14.14 || m.Value > 14.16 {
		t.Errorf("sum = %f, want ~14.15", m.Value)
	}
	// Check bucket counts
	if len(m.Buckets) != 5 { // 4 bounds + inf
		t.Fatalf("expected 5 buckets, got %d", len(m.Buckets))
	}
	// <=0.1: 1, <=0.5: 2, <=1.0: 3, <=5.0: 4, +Inf: 5
	expected := []uint64{1, 2, 3, 4, 5}
	for i, e := range expected {
		if m.Buckets[i].Count != e {
			t.Errorf("bucket[%d] count = %d, want %d", i, m.Buckets[i].Count, e)
		}
	}
}

func TestHistogramDefaultBuckets(t *testing.T) {
	h := NewHistogram("test", "test", nil)
	h.Observe(0.001)
	metrics := h.Collect()
	if len(metrics[0].Buckets) != len(DefaultBuckets)+1 {
		t.Errorf("expected %d buckets, got %d", len(DefaultBuckets)+1, len(metrics[0].Buckets))
	}
}

func TestHistogramWithLabels(t *testing.T) {
	h := NewHistogram("latency", "Latency", []float64{0.1, 1.0}, "endpoint")
	h.Observe(0.05, "/api")
	h.Observe(0.5, "/health")

	metrics := h.Collect()
	if len(metrics) != 2 {
		t.Fatalf("expected 2 label combinations, got %d", len(metrics))
	}
}

func TestHistogramNoLabelsInitialCollect(t *testing.T) {
	h := NewHistogram("empty", "empty", []float64{1, 5, 10})
	metrics := h.Collect()
	if len(metrics) != 1 || metrics[0].Count != 0 {
		t.Error("empty histogram should return zero count")
	}
}

func TestHistogramDesc(t *testing.T) {
	h := NewHistogram("test_hist", "help", nil)
	d := h.Desc()
	if d.Name != "test_hist" || d.Type != MetricTypeHistogram {
		t.Errorf("unexpected desc: %+v", d)
	}
}

func TestHistogramConcurrent(t *testing.T) {
	h := NewHistogram("concurrent", "test", []float64{1, 5, 10})
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				h.Observe(float64(j) / 10)
			}
		}()
	}
	wg.Wait()
}

func TestRegistryRegisterAndWrite(t *testing.T) {
	r := NewRegistry()
	c := NewCounter("requests_total", "Total requests", "method")
	r.MustRegister(c)

	c.Inc("GET")
	c.Inc("GET")
	c.Inc("POST")

	var buf bytes.Buffer
	if err := r.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# HELP requests_total Total requests") {
		t.Error("missing HELP line")
	}
	if !strings.Contains(output, "# TYPE requests_total counter") {
		t.Error("missing TYPE line")
	}
	if !strings.Contains(output, "requests_total{method=\"GET\"}") {
		t.Errorf("missing counter line: %s", output)
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	r := NewRegistry()
	c1 := NewCounter("test", "test1")
	c2 := NewCounter("test", "test2")
	r.MustRegister(c1)
	err := r.Register(c2)
	if err == nil {
		t.Error("duplicate registration should fail")
	}
}

func TestRegistryMustRegisterPanics(t *testing.T) {
	r := NewRegistry()
	c1 := NewCounter("test", "test")
	r.MustRegister(c1)

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustRegister duplicate should panic")
		}
	}()
	c2 := NewCounter("test", "test")
	r.MustRegister(c2)
}

func TestRegistryHistogramOutput(t *testing.T) {
	r := NewRegistry()
	h := NewHistogram("duration_seconds", "Duration", []float64{0.1, 0.5, 1.0})
	r.MustRegister(h)

	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.8)

	var buf bytes.Buffer
	if err := r.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "duration_seconds_bucket{le=\"0.1\"}") {
		t.Errorf("missing bucket line: %s", output)
	}
	if !strings.Contains(output, "duration_seconds_sum") {
		t.Errorf("missing sum line: %s", output)
	}
	if !strings.Contains(output, "duration_seconds_count") {
		t.Errorf("missing count line: %s", output)
	}
}

func TestRegistryGaugeOutput(t *testing.T) {
	r := NewRegistry()
	g := NewGauge("goroutines", "Number of goroutines")
	r.MustRegister(g)

	g.Set(42)

	var buf bytes.Buffer
	if err := r.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# TYPE goroutines gauge") {
		t.Error("missing TYPE line")
	}
	if !strings.Contains(output, "goroutines 42") {
		t.Errorf("missing gauge value: %s", output)
	}
}

func TestLabelSetDeterministic(t *testing.T) {
	l1 := MakeLabelSet([]Label{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}})
	l2 := MakeLabelSet([]Label{{Name: "b", Value: "2"}, {Name: "a", Value: "1"}})
	if l1 != l2 {
		t.Errorf("LabelSet should be order-independent: %q != %q", l1, l2)
	}
}

func TestEscapeLabelValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `hello`},
		{`say "hi"`, `say \"hi\"`},
		{"line\nbreak", `line\nbreak`},
		{`back\slash`, `back\\slash`},
	}
	for _, tt := range tests {
		got := escapeLabelValue(tt.input)
		if got != tt.want {
			t.Errorf("escapeLabelValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCounterTooManyLabels(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic with too many labels")
		}
	}()
	labels := make([]string, maxLabelsPerMetric+1)
	NewCounter("test", "test", labels...)
}

func TestGaugeTooManyLabels(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic with too many labels")
		}
	}()
	labels := make([]string, maxLabelsPerMetric+1)
	NewGauge("test", "test", labels...)
}

func TestHistogramTooManyLabels(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic with too many labels")
		}
	}()
	labels := make([]string, maxLabelsPerMetric+1)
	NewHistogram("test", "test", nil, labels...)
}

func TestRegistryWriteToErrorOnHelp(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(NewCounter("test", "help"))
	err := r.WriteTo(&failWriter{failAfter: 0})
	if err == nil {
		t.Error("expected write error")
	}
}

func TestRegistryWriteToErrorOnType(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(NewCounter("test", "help"))
	err := r.WriteTo(&failWriter{failAfter: 1})
	if err == nil {
		t.Error("expected write error on TYPE line")
	}
}

func TestRegistryWriteToErrorOnMetric(t *testing.T) {
	r := NewRegistry()
	c := NewCounter("test", "help")
	r.MustRegister(c)
	c.Inc()
	err := r.WriteTo(&failWriter{failAfter: 2})
	if err == nil {
		t.Error("expected write error on metric line")
	}
}

type failWriter struct {
	failAfter int
	calls     int
}

func (w *failWriter) Write(p []byte) (int, error) {
	if w.calls >= w.failAfter {
		return 0, errWriteFailed
	}
	w.calls++
	return len(p), nil
}

var errWriteFailed = &writeError{}

type writeError struct{}

func (e *writeError) Error() string { return "write failed" }

func TestHistogramWithLabelsOutput(t *testing.T) {
	r := NewRegistry()
	h := NewHistogram("latency", "Latency", []float64{0.1, 1.0}, "endpoint")
	r.MustRegister(h)
	h.Observe(0.05, "/api")

	var buf bytes.Buffer
	if err := r.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `endpoint="/api"`) {
		t.Errorf("missing label in histogram output: %s", output)
	}
}

func TestCounterNoLabelsOutput(t *testing.T) {
	r := NewRegistry()
	c := NewCounter("simple_counter", "no labels")
	r.MustRegister(c)
	c.Inc()

	var buf bytes.Buffer
	if err := r.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "simple_counter 1") {
		t.Errorf("missing simple counter line: %s", output)
	}
}

func TestGaugeNoLabelsOutput(t *testing.T) {
	r := NewRegistry()
	g := NewGauge("simple_gauge", "no labels")
	r.MustRegister(g)
	g.Set(99)

	var buf bytes.Buffer
	if err := r.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "simple_gauge 99") {
		t.Errorf("missing simple gauge line: %s", output)
	}
}

func TestCounterConcurrentGetOrCreate(t *testing.T) {
	// Exercise the double-check locking path in getOrCreate
	c := NewCounter("race_test", "test", "worker")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc("same_label") // all goroutines hit same key
		}()
	}
	wg.Wait()
}

func TestGaugeConcurrentGetOrCreate(t *testing.T) {
	g := NewGauge("race_test", "test", "worker")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.Inc("same_label")
		}()
	}
	wg.Wait()
}

func TestHistogramConcurrentGetOrCreate(t *testing.T) {
	h := NewHistogram("race_test", "test", []float64{1}, "worker")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Observe(0.5, "same_label")
		}()
	}
	wg.Wait()
}

func TestCounterMissingLabelValues(t *testing.T) {
	c := NewCounter("test", "test", "method", "status")
	c.Inc("GET") // only one label value provided (two expected)
	metrics := c.Collect()
	if len(metrics) != 1 {
		t.Fatal("expected 1 metric")
	}
	if len(metrics[0].Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(metrics[0].Labels))
	}
	if metrics[0].Labels[1].Value != "" {
		t.Errorf("missing label value should default to empty, got %q", metrics[0].Labels[1].Value)
	}
}
