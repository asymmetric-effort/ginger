package jaeger

// VType identifies the type of a Jaeger KeyValue.
type VType int32

const (
	VTypeString  VType = 0
	VTypeBool    VType = 1
	VTypeInt64   VType = 2
	VTypeFloat64 VType = 3
	VTypeBinary  VType = 4
)

// RefType identifies the type of a span reference.
type RefType int32

const (
	RefTypeChildOf     RefType = 0
	RefTypeFollowsFrom RefType = 1
)

// KeyValue is a Jaeger model key-value pair.
type KeyValue struct {
	Key      string  `json:"key"`
	VType    VType   `json:"vType"`
	VStr     string  `json:"vStr,omitempty"`
	VBool    bool    `json:"vBool,omitempty"`
	VInt64   int64   `json:"vInt64,omitempty"`
	VFloat64 float64 `json:"vFloat64,omitempty"`
	VBinary  []byte  `json:"vBinary,omitempty"`
}

// Log is a timestamped event on a span.
type Log struct {
	Timestamp uint64     `json:"timestamp"` // microseconds since epoch
	Fields    []KeyValue `json:"fields"`
}

// SpanRef is a reference to another span.
type SpanRef struct {
	TraceID [16]byte `json:"-"`
	SpanID  [8]byte  `json:"-"`
	RefType RefType  `json:"refType"`
}

// Process describes the traced service.
type Process struct {
	ServiceName string     `json:"serviceName"`
	Tags        []KeyValue `json:"tags,omitempty"`
}

// Span is a Jaeger model span.
type Span struct {
	TraceID       [16]byte   `json:"-"`
	SpanID        [8]byte    `json:"-"`
	OperationName string     `json:"operationName"`
	References    []SpanRef  `json:"references,omitempty"`
	Flags         uint32     `json:"flags,omitempty"`
	StartTime     uint64     `json:"startTime"` // microseconds since epoch
	Duration      uint64     `json:"duration"`  // microseconds
	Tags          []KeyValue `json:"tags,omitempty"`
	Logs          []Log      `json:"logs,omitempty"`
	Process       *Process   `json:"process,omitempty"`
}

// Batch is a collection of spans with a shared process.
type Batch struct {
	Process Process `json:"process"`
	Spans   []Span  `json:"spans"`
}
