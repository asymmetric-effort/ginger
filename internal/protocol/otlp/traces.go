package otlp

// SpanKind identifies the type of span.
type SpanKind int32

const (
	SpanKindUnspecified SpanKind = 0
	SpanKindInternal    SpanKind = 1
	SpanKindServer      SpanKind = 2
	SpanKindClient      SpanKind = 3
	SpanKindProducer    SpanKind = 4
	SpanKindConsumer    SpanKind = 5
)

// String returns the span kind name.
func (k SpanKind) String() string {
	switch k {
	case SpanKindInternal:
		return "INTERNAL"
	case SpanKindServer:
		return "SERVER"
	case SpanKindClient:
		return "CLIENT"
	case SpanKindProducer:
		return "PRODUCER"
	case SpanKindConsumer:
		return "CONSUMER"
	default:
		return "UNSPECIFIED"
	}
}

// StatusCode represents the span status.
type StatusCode int32

const (
	StatusCodeUnset StatusCode = 0
	StatusCodeOk    StatusCode = 1
	StatusCodeError StatusCode = 2
)

// String returns the status code name.
func (c StatusCode) String() string {
	switch c {
	case StatusCodeOk:
		return "OK"
	case StatusCodeError:
		return "ERROR"
	default:
		return "UNSET"
	}
}

// TracesData is the top-level container for trace data.
type TracesData struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans,omitempty"`
}

// ResourceSpans groups spans by resource.
type ResourceSpans struct {
	Resource  Resource    `json:"resource,omitempty"`
	ScopeSpans []ScopeSpans `json:"scopeSpans,omitempty"`
}

// Resource describes the entity producing telemetry.
type Resource struct {
	Attributes Attributes `json:"attributes,omitempty"`
}

// ScopeSpans groups spans by instrumentation scope.
type ScopeSpans struct {
	Scope InstrumentationScope `json:"scope,omitempty"`
	Spans []Span               `json:"spans,omitempty"`
}

// InstrumentationScope identifies the instrumentation library.
type InstrumentationScope struct {
	Name    string     `json:"name,omitempty"`
	Version string     `json:"version,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// Span represents a single operation within a trace.
type Span struct {
	TraceID                [16]byte    `json:"-"`
	SpanID                 [8]byte     `json:"-"`
	ParentSpanID           [8]byte     `json:"-"`
	TraceState             string      `json:"traceState,omitempty"`
	Name                   string      `json:"name"`
	Kind                   SpanKind    `json:"kind"`
	StartTimeUnixNano      uint64      `json:"startTimeUnixNano,string"`
	EndTimeUnixNano        uint64      `json:"endTimeUnixNano,string"`
	Attributes             Attributes  `json:"attributes,omitempty"`
	Events                 []SpanEvent `json:"events,omitempty"`
	Links                  []SpanLink  `json:"links,omitempty"`
	Status                 Status      `json:"status,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
	DroppedEventsCount     uint32      `json:"droppedEventsCount,omitempty"`
	DroppedLinksCount      uint32      `json:"droppedLinksCount,omitempty"`
}

// SpanEvent represents a timed annotation on a span.
type SpanEvent struct {
	TimeUnixNano           uint64     `json:"timeUnixNano,string"`
	Name                   string     `json:"name"`
	Attributes             Attributes `json:"attributes,omitempty"`
	DroppedAttributesCount uint32     `json:"droppedAttributesCount,omitempty"`
}

// SpanLink represents a causal relationship to another span.
type SpanLink struct {
	TraceID                [16]byte   `json:"-"`
	SpanID                 [8]byte    `json:"-"`
	TraceState             string     `json:"traceState,omitempty"`
	Attributes             Attributes `json:"attributes,omitempty"`
	DroppedAttributesCount uint32     `json:"droppedAttributesCount,omitempty"`
}

// Status represents the status of a span.
type Status struct {
	Message string     `json:"message,omitempty"`
	Code    StatusCode `json:"code,omitempty"`
}
