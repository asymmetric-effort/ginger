package zipkin

import (
	"encoding/hex"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

// ToOTLP converts Zipkin spans to OTLP TracesData.
// Spans are grouped by service name (from localEndpoint).
func ToOTLP(spans []Span) otlp.TracesData {
	// Group spans by service name
	serviceSpans := make(map[string][]otlp.Span)
	serviceOrder := make([]string, 0)
	for i := range spans {
		svc := ""
		if spans[i].LocalEndpoint != nil {
			svc = spans[i].LocalEndpoint.ServiceName
		}
		if _, exists := serviceSpans[svc]; !exists {
			serviceOrder = append(serviceOrder, svc)
		}
		serviceSpans[svc] = append(serviceSpans[svc], spanToOTLP(&spans[i]))
	}

	var rss []otlp.ResourceSpans
	for _, svc := range serviceOrder {
		resAttrs := otlp.NewAttributes()
		if svc != "" {
			resAttrs.Set("service.name", otlp.StringValue(svc))
		}
		rss = append(rss, otlp.ResourceSpans{
			Resource:   otlp.Resource{Attributes: resAttrs},
			ScopeSpans: []otlp.ScopeSpans{{Spans: serviceSpans[svc]}},
		})
	}

	return otlp.TracesData{ResourceSpans: rss}
}

func spanToOTLP(zs *Span) otlp.Span {
	var traceID [16]byte
	var spanID [8]byte
	var parentSpanID [8]byte

	hexDecode(traceID[:], zs.TraceID)
	hexDecode(spanID[:], zs.ID)
	if zs.ParentID != "" {
		hexDecode(parentSpanID[:], zs.ParentID)
	}

	kind := otlp.SpanKindUnspecified
	switch zs.Kind {
	case KindClient:
		kind = otlp.SpanKindClient
	case KindServer:
		kind = otlp.SpanKindServer
	case KindProducer:
		kind = otlp.SpanKindProducer
	case KindConsumer:
		kind = otlp.SpanKindConsumer
	}

	attrs := otlp.NewAttributes()
	for k, v := range zs.Tags {
		attrs.Set(k, otlp.StringValue(v))
	}
	if zs.RemoteEndpoint != nil {
		if zs.RemoteEndpoint.ServiceName != "" {
			attrs.Set("peer.service", otlp.StringValue(zs.RemoteEndpoint.ServiceName))
		}
		if zs.RemoteEndpoint.IPv4 != "" {
			attrs.Set("net.peer.ip", otlp.StringValue(zs.RemoteEndpoint.IPv4))
		}
		if zs.RemoteEndpoint.Port > 0 {
			attrs.Set("net.peer.port", otlp.IntValue(int64(zs.RemoteEndpoint.Port)))
		}
	}

	var events []otlp.SpanEvent
	for _, ann := range zs.Annotations {
		events = append(events, otlp.SpanEvent{
			TimeUnixNano: ann.Timestamp * 1000,
			Name:         ann.Value,
		})
	}

	startNano := zs.Timestamp * 1000
	endNano := startNano + zs.Duration*1000

	return otlp.Span{
		TraceID:           traceID,
		SpanID:            spanID,
		ParentSpanID:      parentSpanID,
		Name:              zs.Name,
		Kind:              kind,
		StartTimeUnixNano: startNano,
		EndTimeUnixNano:   endNano,
		Attributes:        attrs,
		Events:            events,
	}
}

func hexDecode(dst []byte, src string) {
	// Support both 16-char (64-bit) and 32-char (128-bit) trace IDs
	if len(src) < len(dst)*2 {
		// Left-pad with zeros for 64-bit IDs
		offset := len(dst) - len(src)/2
		hex.Decode(dst[offset:], []byte(src))
		return
	}
	hex.Decode(dst[:], []byte(src))
}
