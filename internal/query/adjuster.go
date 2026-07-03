package query

import (
	"net"
	"sort"
	"time"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// Warning describes a non-fatal issue found during trace adjustment.
type Warning struct {
	SpanID  [8]byte
	Message string
}

// Adjuster transforms a trace before serving to clients.
type Adjuster interface {
	Adjust(td otlp.TracesData) (otlp.TracesData, []Warning)
}

// AdjusterChain applies adjusters in order, collecting all warnings.
type AdjusterChain struct {
	adjusters []Adjuster
}

// NewAdjusterChain creates an adjuster chain.
func NewAdjusterChain(adjusters ...Adjuster) *AdjusterChain {
	return &AdjusterChain{adjusters: adjusters}
}

// Adjust applies all adjusters in order.
func (c *AdjusterChain) Adjust(td otlp.TracesData) (otlp.TracesData, []Warning) {
	var allWarnings []Warning
	for _, adj := range c.adjusters {
		var warnings []Warning
		td, warnings = adj.Adjust(td)
		allWarnings = append(allWarnings, warnings...)
	}
	return td, allWarnings
}

// SpanSorter sorts spans within a trace by start time.
type SpanSorter struct{}

// Adjust sorts spans by start time.
func (s SpanSorter) Adjust(td otlp.TracesData) (otlp.TracesData, []Warning) {
	for ri := range td.ResourceSpans {
		for si := range td.ResourceSpans[ri].ScopeSpans {
			spans := td.ResourceSpans[ri].ScopeSpans[si].Spans
			sort.Slice(spans, func(i, j int) bool {
				return spans[i].StartTimeUnixNano < spans[j].StartTimeUnixNano
			})
		}
	}
	return td, nil
}

// SpanDeduplicator removes duplicate spans by SpanID.
type SpanDeduplicator struct{}

// Adjust removes duplicate spans.
func (d SpanDeduplicator) Adjust(td otlp.TracesData) (otlp.TracesData, []Warning) {
	var warnings []Warning
	for ri := range td.ResourceSpans {
		for si := range td.ResourceSpans[ri].ScopeSpans {
			spans := td.ResourceSpans[ri].ScopeSpans[si].Spans
			seen := make(map[[8]byte]int)
			var unique []otlp.Span
			for _, span := range spans {
				if idx, exists := seen[span.SpanID]; exists {
					// Keep the one with more events/attributes
					existing := unique[idx]
					if len(span.Events) > len(existing.Events) ||
						span.Attributes.Len() > existing.Attributes.Len() {
						unique[idx] = span
					}
					warnings = append(warnings, Warning{
						SpanID:  span.SpanID,
						Message: "duplicate span removed",
					})
				} else {
					seen[span.SpanID] = len(unique)
					unique = append(unique, span)
				}
			}
			td.ResourceSpans[ri].ScopeSpans[si].Spans = unique
		}
	}
	return td, warnings
}

// ClockSkewAdjuster adjusts child span timestamps when they precede their parent.
type ClockSkewAdjuster struct {
	MaxAdjust time.Duration
}

// Adjust corrects clock skew.
func (a ClockSkewAdjuster) Adjust(td otlp.TracesData) (otlp.TracesData, []Warning) {
	var warnings []Warning
	// Build parent map
	spanMap := make(map[[8]byte]*otlp.Span)
	for ri := range td.ResourceSpans {
		for si := range td.ResourceSpans[ri].ScopeSpans {
			for spi := range td.ResourceSpans[ri].ScopeSpans[si].Spans {
				span := &td.ResourceSpans[ri].ScopeSpans[si].Spans[spi]
				spanMap[span.SpanID] = span
			}
		}
	}

	for ri := range td.ResourceSpans {
		for si := range td.ResourceSpans[ri].ScopeSpans {
			for spi := range td.ResourceSpans[ri].ScopeSpans[si].Spans {
				span := &td.ResourceSpans[ri].ScopeSpans[si].Spans[spi]
				if span.ParentSpanID == [8]byte{} {
					continue
				}
				parent, exists := spanMap[span.ParentSpanID]
				if !exists {
					continue
				}
				if span.StartTimeUnixNano < parent.StartTimeUnixNano {
					diff := parent.StartTimeUnixNano - span.StartTimeUnixNano
					if a.MaxAdjust > 0 && time.Duration(diff) > a.MaxAdjust {
						continue
					}
					span.StartTimeUnixNano = parent.StartTimeUnixNano
					span.EndTimeUnixNano += diff
					warnings = append(warnings, Warning{
						SpanID:  span.SpanID,
						Message: "clock skew adjusted",
					})
				}
			}
		}
	}
	return td, warnings
}

// IPAttributeNormalizer converts numeric IP attributes to dotted-decimal strings.
type IPAttributeNormalizer struct{}

// Adjust normalizes IP attributes.
func (n IPAttributeNormalizer) Adjust(td otlp.TracesData) (otlp.TracesData, []Warning) {
	ipKeys := []string{"net.peer.ip", "net.host.ip", "net.sock.peer.addr", "net.sock.host.addr"}
	for ri := range td.ResourceSpans {
		for si := range td.ResourceSpans[ri].ScopeSpans {
			for spi := range td.ResourceSpans[ri].ScopeSpans[si].Spans {
				span := &td.ResourceSpans[ri].ScopeSpans[si].Spans[spi]
				for _, key := range ipKeys {
					v, ok := span.Attributes.Get(key)
					if !ok {
						continue
					}
					if v.Type == otlp.AnyValueTypeBytes && len(v.BytesVal) == 4 {
						ip := net.IPv4(v.BytesVal[0], v.BytesVal[1], v.BytesVal[2], v.BytesVal[3])
						span.Attributes.Set(key, otlp.StringValue(ip.String()))
					}
				}
			}
		}
	}
	return td, nil
}
