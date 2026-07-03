package propagation

import "context"

// TextMapCarrier is the interface for propagation carriers that store key-value string pairs.
type TextMapCarrier interface {
	// Get returns the value for a key, or empty string if not found.
	Get(key string) string

	// Set stores a key-value pair.
	Set(key string, value string)

	// Keys returns all keys in the carrier.
	Keys() []string
}

// TextMapPropagator is the interface for injecting and extracting span context
// across process boundaries using text-based carriers.
type TextMapPropagator interface {
	// Inject sets values from the context into the carrier.
	Inject(ctx context.Context, carrier TextMapCarrier)

	// Extract reads values from the carrier into the context.
	Extract(ctx context.Context, carrier TextMapCarrier) context.Context

	// Fields returns the set of header/key names this propagator uses.
	Fields() []string
}

// CompositeTextMapPropagator chains multiple propagators. On Extract, each
// propagator is called in order, each receiving the context returned by the
// previous one. On Inject, each propagator injects its headers.
type CompositeTextMapPropagator struct {
	propagators []TextMapPropagator
}

// NewCompositeTextMapPropagator creates a CompositeTextMapPropagator from the given propagators.
func NewCompositeTextMapPropagator(propagators ...TextMapPropagator) CompositeTextMapPropagator {
	p := make([]TextMapPropagator, len(propagators))
	copy(p, propagators)
	return CompositeTextMapPropagator{propagators: p}
}

// Inject calls Inject on each propagator in order.
func (c CompositeTextMapPropagator) Inject(ctx context.Context, carrier TextMapCarrier) {
	for _, p := range c.propagators {
		p.Inject(ctx, carrier)
	}
}

// Extract calls Extract on each propagator in order, threading the context through.
func (c CompositeTextMapPropagator) Extract(ctx context.Context, carrier TextMapCarrier) context.Context {
	for _, p := range c.propagators {
		ctx = p.Extract(ctx, carrier)
	}
	return ctx
}

// Fields returns the union of all propagator fields.
func (c CompositeTextMapPropagator) Fields() []string {
	seen := make(map[string]struct{})
	var fields []string
	for _, p := range c.propagators {
		for _, f := range p.Fields() {
			if _, exists := seen[f]; !exists {
				seen[f] = struct{}{}
				fields = append(fields, f)
			}
		}
	}
	return fields
}

// MapCarrier is a TextMapCarrier backed by a map for convenience and testing.
type MapCarrier map[string]string

// Get returns the value for a key.
func (c MapCarrier) Get(key string) string {
	return c[key]
}

// Set stores a key-value pair.
func (c MapCarrier) Set(key string, value string) {
	c[key] = value
}

// Keys returns all keys.
func (c MapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
