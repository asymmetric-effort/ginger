package propagation

import (
	"context"
	"net/url"
	"strings"
)

const baggageHeader = "baggage"

// BaggagePropagator implements the W3C Baggage propagation format.
// It injects and extracts the "baggage" header as defined by the
// W3C Baggage specification (https://www.w3.org/TR/baggage/).
type BaggagePropagator struct{}

// Inject writes the baggage header into the carrier.
// Format: key1=value1,key2=value2 (values are URL-encoded).
func (p BaggagePropagator) Inject(ctx context.Context, carrier TextMapCarrier) {
	b := BaggageFromContext(ctx)
	if b.Len() == 0 {
		return
	}

	keys := b.Keys()
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := b.Get(key)
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
	}
	carrier.Set(baggageHeader, strings.Join(parts, ","))
}

// Extract parses the baggage header from the carrier and returns
// a new context with the extracted Baggage.
func (p BaggagePropagator) Extract(ctx context.Context, carrier TextMapCarrier) context.Context {
	header := carrier.Get(baggageHeader)
	if header == "" {
		return ctx
	}

	b := NewBaggage()
	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split on first '=' only; properties after ';' are ignored per spec.
		kv := strings.SplitN(pair, ";", 2)
		entry := strings.TrimSpace(kv[0])

		eqIdx := strings.IndexByte(entry, '=')
		if eqIdx < 0 {
			continue
		}

		key, err := url.QueryUnescape(strings.TrimSpace(entry[:eqIdx]))
		if err != nil || key == "" {
			continue
		}
		value, err := url.QueryUnescape(strings.TrimSpace(entry[eqIdx+1:]))
		if err != nil {
			continue
		}

		b, _ = b.Set(key, value)
	}

	if b.Len() == 0 {
		return ctx
	}

	return ContextWithBaggage(ctx, b)
}

// Fields returns the header names used by this propagator.
func (p BaggagePropagator) Fields() []string {
	return []string{baggageHeader}
}
