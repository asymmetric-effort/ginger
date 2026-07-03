package propagation

import (
	"context"
	"testing"
)

// mockPropagator is a test propagator that sets/gets a specific header.
type mockPropagator struct {
	headerName string
	headerVal  string
}

func (m *mockPropagator) Inject(_ context.Context, carrier TextMapCarrier) {
	carrier.Set(m.headerName, m.headerVal)
}

func (m *mockPropagator) Extract(ctx context.Context, carrier TextMapCarrier) context.Context {
	val := carrier.Get(m.headerName)
	if val == "" {
		return ctx
	}
	// Store in SpanContext with a deterministic trace ID based on the value
	tid := TraceID{val[0]}
	sc := NewSpanContext(tid, SpanID{1}, 0, TraceState{}, true)
	return ContextWithSpanContext(ctx, sc)
}

func (m *mockPropagator) Fields() []string {
	return []string{m.headerName}
}

func TestMapCarrier(t *testing.T) {
	c := MapCarrier{}
	c.Set("key1", "val1")
	c.Set("key2", "val2")

	if got := c.Get("key1"); got != "val1" {
		t.Errorf("Get(key1) = %q, want val1", got)
	}
	if got := c.Get("key2"); got != "val2" {
		t.Errorf("Get(key2) = %q, want val2", got)
	}
	if got := c.Get("missing"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestMapCarrierOverwrite(t *testing.T) {
	c := MapCarrier{}
	c.Set("key", "old")
	c.Set("key", "new")
	if got := c.Get("key"); got != "new" {
		t.Errorf("Get(key) = %q, want new", got)
	}
}

func TestCompositeTextMapPropagatorInject(t *testing.T) {
	p1 := &mockPropagator{headerName: "h1", headerVal: "v1"}
	p2 := &mockPropagator{headerName: "h2", headerVal: "v2"}
	comp := NewCompositeTextMapPropagator(p1, p2)

	carrier := MapCarrier{}
	comp.Inject(context.Background(), carrier)

	if got := carrier.Get("h1"); got != "v1" {
		t.Errorf("carrier.Get(h1) = %q, want v1", got)
	}
	if got := carrier.Get("h2"); got != "v2" {
		t.Errorf("carrier.Get(h2) = %q, want v2", got)
	}
}

func TestCompositeTextMapPropagatorExtract(t *testing.T) {
	p1 := &mockPropagator{headerName: "h1", headerVal: "v1"}
	p2 := &mockPropagator{headerName: "h2", headerVal: "v2"}
	comp := NewCompositeTextMapPropagator(p1, p2)

	carrier := MapCarrier{"h1": "A", "h2": "B"}
	ctx := comp.Extract(context.Background(), carrier)

	// Last propagator wins (p2 overwrites context from p1)
	sc := SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Error("expected valid SpanContext after extract")
	}
	if sc.TraceID()[0] != 'B' {
		t.Errorf("expected trace from p2 (B), got %v", sc.TraceID()[0])
	}
}

func TestCompositeTextMapPropagatorFields(t *testing.T) {
	p1 := &mockPropagator{headerName: "h1"}
	p2 := &mockPropagator{headerName: "h2"}
	p3 := &mockPropagator{headerName: "h1"} // duplicate
	comp := NewCompositeTextMapPropagator(p1, p2, p3)

	fields := comp.Fields()
	if len(fields) != 2 {
		t.Errorf("expected 2 unique fields, got %d: %v", len(fields), fields)
	}
}

func TestCompositeTextMapPropagatorEmpty(t *testing.T) {
	comp := NewCompositeTextMapPropagator()
	carrier := MapCarrier{}
	comp.Inject(context.Background(), carrier)
	if len(carrier.Keys()) != 0 {
		t.Error("empty composite should inject nothing")
	}

	ctx := comp.Extract(context.Background(), carrier)
	sc := SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("empty composite should extract nothing")
	}

	fields := comp.Fields()
	if len(fields) != 0 {
		t.Error("empty composite should have no fields")
	}
}

func TestCompositeTextMapPropagatorExtractMissingHeader(t *testing.T) {
	p1 := &mockPropagator{headerName: "h1", headerVal: "v1"}
	comp := NewCompositeTextMapPropagator(p1)

	carrier := MapCarrier{} // no headers
	ctx := comp.Extract(context.Background(), carrier)
	sc := SpanContextFromContext(ctx)
	if sc.IsValid() {
		t.Error("extract with missing headers should return invalid context")
	}
}
