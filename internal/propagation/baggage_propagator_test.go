package propagation

import (
	"context"
	"testing"
)

func TestBaggagePropagatorFields(t *testing.T) {
	p := BaggagePropagator{}
	fields := p.Fields()
	if len(fields) != 1 || fields[0] != "baggage" {
		t.Errorf("Fields() = %v, want [baggage]", fields)
	}
}

func TestBaggagePropagatorInjectEmpty(t *testing.T) {
	p := BaggagePropagator{}
	carrier := MapCarrier{}
	p.Inject(context.Background(), carrier)
	if got := carrier.Get("baggage"); got != "" {
		t.Errorf("expected empty baggage header, got %q", got)
	}
}

func TestBaggagePropagatorInject(t *testing.T) {
	p := BaggagePropagator{}
	b := NewBaggage()
	b, _ = b.Set("user", "alice")
	b, _ = b.Set("tenant", "acme")
	ctx := ContextWithBaggage(context.Background(), b)

	carrier := MapCarrier{}
	p.Inject(ctx, carrier)

	got := carrier.Get("baggage")
	if got == "" {
		t.Fatal("expected non-empty baggage header")
	}
	// Should contain both entries
	if !containsEntry(got, "user", "alice") {
		t.Errorf("missing user=alice in %q", got)
	}
	if !containsEntry(got, "tenant", "acme") {
		t.Errorf("missing tenant=acme in %q", got)
	}
}

func TestBaggagePropagatorInjectURLEncode(t *testing.T) {
	p := BaggagePropagator{}
	b := NewBaggage()
	b, _ = b.Set("key", "value with spaces")
	ctx := ContextWithBaggage(context.Background(), b)

	carrier := MapCarrier{}
	p.Inject(ctx, carrier)

	got := carrier.Get("baggage")
	// Value should be URL-encoded
	if got == "" {
		t.Fatal("expected non-empty baggage header")
	}
	// The raw header should NOT contain literal spaces
	for _, c := range got {
		if c == ' ' {
			t.Errorf("expected URL-encoded value, got literal space in %q", got)
			break
		}
	}
}

func TestBaggagePropagatorExtractEmpty(t *testing.T) {
	p := BaggagePropagator{}
	carrier := MapCarrier{}
	ctx := p.Extract(context.Background(), carrier)
	b := BaggageFromContext(ctx)
	if b.Len() != 0 {
		t.Errorf("expected empty baggage, got %d entries", b.Len())
	}
}

func TestBaggagePropagatorExtract(t *testing.T) {
	p := BaggagePropagator{}
	carrier := MapCarrier{
		"baggage": "user=alice,tenant=acme",
	}
	ctx := p.Extract(context.Background(), carrier)
	b := BaggageFromContext(ctx)

	if b.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", b.Len())
	}
	if got := b.Get("user"); got != "alice" {
		t.Errorf("user = %q, want alice", got)
	}
	if got := b.Get("tenant"); got != "acme" {
		t.Errorf("tenant = %q, want acme", got)
	}
}

func TestBaggagePropagatorExtractURLEncoded(t *testing.T) {
	p := BaggagePropagator{}
	carrier := MapCarrier{
		"baggage": "key=value+with+spaces",
	}
	ctx := p.Extract(context.Background(), carrier)
	b := BaggageFromContext(ctx)

	if got := b.Get("key"); got != "value with spaces" {
		t.Errorf("key = %q, want 'value with spaces'", got)
	}
}

func TestBaggagePropagatorExtractWithProperties(t *testing.T) {
	// Per W3C spec, properties after ';' are member-level metadata and should be ignored
	p := BaggagePropagator{}
	carrier := MapCarrier{
		"baggage": "user=alice;some-prop=ignored,tenant=acme",
	}
	ctx := p.Extract(context.Background(), carrier)
	b := BaggageFromContext(ctx)

	if got := b.Get("user"); got != "alice" {
		t.Errorf("user = %q, want alice", got)
	}
	if got := b.Get("tenant"); got != "acme" {
		t.Errorf("tenant = %q, want acme", got)
	}
}

func TestBaggagePropagatorExtractInvalid(t *testing.T) {
	p := BaggagePropagator{}
	// No '=' sign
	carrier := MapCarrier{
		"baggage": "invalidentry",
	}
	ctx := p.Extract(context.Background(), carrier)
	b := BaggageFromContext(ctx)
	if b.Len() != 0 {
		t.Errorf("expected empty baggage for invalid header, got %d entries", b.Len())
	}
}

func TestBaggagePropagatorRoundTrip(t *testing.T) {
	p := BaggagePropagator{}

	b := NewBaggage()
	b, _ = b.Set("user-id", "12345")
	b, _ = b.Set("region", "us-east-1")
	ctx := ContextWithBaggage(context.Background(), b)

	carrier := MapCarrier{}
	p.Inject(ctx, carrier)

	ctx2 := p.Extract(context.Background(), carrier)
	b2 := BaggageFromContext(ctx2)

	if b2.Len() != 2 {
		t.Fatalf("round-trip: expected 2 entries, got %d", b2.Len())
	}
	if got := b2.Get("user-id"); got != "12345" {
		t.Errorf("round-trip: user-id = %q, want 12345", got)
	}
	if got := b2.Get("region"); got != "us-east-1" {
		t.Errorf("round-trip: region = %q, want us-east-1", got)
	}
}

func TestBaggagePropagatorExtractWhitespace(t *testing.T) {
	p := BaggagePropagator{}
	carrier := MapCarrier{
		"baggage": " user = alice , tenant = acme ",
	}
	ctx := p.Extract(context.Background(), carrier)
	b := BaggageFromContext(ctx)

	if got := b.Get("user"); got != "alice" {
		t.Errorf("user = %q, want alice", got)
	}
	if got := b.Get("tenant"); got != "acme" {
		t.Errorf("tenant = %q, want acme", got)
	}
}

// containsEntry checks if a baggage header string contains key=value (URL-encoded).
func containsEntry(header, key, value string) bool {
	pairs := make(map[string]string)
	for _, p := range splitBaggage(header) {
		parts := splitFirst(p, '=')
		if len(parts) == 2 {
			pairs[parts[0]] = parts[1]
		}
	}
	v, ok := pairs[key]
	return ok && v == value
}

func splitBaggage(s string) []string {
	var result []string
	for _, p := range splitAll(s, ',') {
		p = trimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func splitFirst(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{trimSpace(s[:i]), trimSpace(s[i+1:])}
		}
	}
	return []string{s}
}

func splitAll(s string, sep byte) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
