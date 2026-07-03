package zipkin

import "encoding/json"

// Kind identifies the span kind in Zipkin format.
type Kind string

const (
	KindClient   Kind = "CLIENT"
	KindServer   Kind = "SERVER"
	KindProducer Kind = "PRODUCER"
	KindConsumer Kind = "CONSUMER"
)

// Endpoint describes a network context of a service.
type Endpoint struct {
	ServiceName string `json:"serviceName,omitempty"`
	IPv4        string `json:"ipv4,omitempty"`
	IPv6        string `json:"ipv6,omitempty"`
	Port        int    `json:"port,omitempty"`
}

// Annotation is a timed event.
type Annotation struct {
	Timestamp uint64 `json:"timestamp"` // microseconds since epoch
	Value     string `json:"value"`
}

// Span is a Zipkin v2 JSON span.
type Span struct {
	TraceID        string            `json:"traceId"`
	ID             string            `json:"id"`
	ParentID       string            `json:"parentId,omitempty"`
	Name           string            `json:"name,omitempty"`
	Kind           Kind              `json:"kind,omitempty"`
	Timestamp      uint64            `json:"timestamp,omitempty"` // microseconds
	Duration       uint64            `json:"duration,omitempty"`  // microseconds
	LocalEndpoint  *Endpoint         `json:"localEndpoint,omitempty"`
	RemoteEndpoint *Endpoint         `json:"remoteEndpoint,omitempty"`
	Annotations    []Annotation      `json:"annotations,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Debug          bool              `json:"debug,omitempty"`
	Shared         bool              `json:"shared,omitempty"`
}

// UnmarshalArray parses a JSON array of Zipkin spans.
func UnmarshalArray(data []byte) ([]Span, error) {
	var spans []Span
	if err := json.Unmarshal(data, &spans); err != nil {
		return nil, err
	}
	return spans, nil
}

// MarshalArray serializes a slice of spans to JSON.
func MarshalArray(spans []Span) ([]byte, error) {
	return json.Marshal(spans)
}
