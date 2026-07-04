package influxlp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Precision controls the timestamp precision.
type Precision int

const (
	PrecisionNanosecond Precision = iota
	PrecisionMicrosecond
	PrecisionMillisecond
	PrecisionSecond
)

// Encoder encodes Points to InfluxDB line protocol format.
type Encoder struct {
	Precision Precision
}

// NewEncoder creates a new Encoder with nanosecond precision.
func NewEncoder() *Encoder {
	return &Encoder{Precision: PrecisionNanosecond}
}

// Encode encodes a single point to line protocol.
func (e *Encoder) Encode(p Point) ([]byte, error) {
	if p.Measurement == "" {
		return nil, fmt.Errorf("measurement name is required")
	}
	if len(p.Fields) == 0 {
		return nil, fmt.Errorf("at least one field is required")
	}

	var b strings.Builder
	b.Grow(128)

	// Measurement (escaped)
	b.WriteString(escapeMeasurement(p.Measurement))

	// Tags (sorted by key, comma-separated, no space before)
	if len(p.Tags) > 0 {
		sorted := make([]Tag, len(p.Tags))
		copy(sorted, p.Tags)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })
		for _, tag := range sorted {
			b.WriteByte(',')
			b.WriteString(escapeTagKey(tag.Key))
			b.WriteByte('=')
			b.WriteString(escapeTagValue(tag.Value))
		}
	}

	// Space before fields
	b.WriteByte(' ')

	// Fields (sorted by key for determinism)
	sortedFields := make([]Field, len(p.Fields))
	copy(sortedFields, p.Fields)
	sort.Slice(sortedFields, func(i, j int) bool { return sortedFields[i].Key < sortedFields[j].Key })
	for i, f := range sortedFields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(escapeFieldKey(f.Key))
		b.WriteByte('=')
		writeFieldValue(&b, f.Value)
	}

	// Timestamp
	if !p.Timestamp.IsZero() {
		b.WriteByte(' ')
		b.WriteString(strconv.FormatInt(e.timestampValue(p.Timestamp), 10))
	}

	return []byte(b.String()), nil
}

// EncodeMulti encodes multiple points, each on its own line.
func (e *Encoder) EncodeMulti(points []Point) ([]byte, error) {
	var b strings.Builder
	for i, p := range points {
		line, err := e.Encode(p)
		if err != nil {
			return nil, fmt.Errorf("point %d: %w", i, err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func (e *Encoder) timestampValue(t time.Time) int64 {
	switch e.Precision {
	case PrecisionSecond:
		return t.Unix()
	case PrecisionMillisecond:
		return t.UnixMilli()
	case PrecisionMicrosecond:
		return t.UnixMicro()
	default:
		return t.UnixNano()
	}
}

func writeFieldValue(b *strings.Builder, v FieldValue) {
	switch v.Type {
	case FieldTypeFloat:
		b.WriteString(strconv.FormatFloat(v.Float, 'f', -1, 64))
	case FieldTypeInt:
		b.WriteString(strconv.FormatInt(v.Int, 10))
		b.WriteByte('i')
	case FieldTypeUInt:
		b.WriteString(strconv.FormatUint(v.UInt, 10))
		b.WriteByte('u')
	case FieldTypeBool:
		if v.Bool {
			b.WriteByte('t')
		} else {
			b.WriteByte('f')
		}
	case FieldTypeString:
		b.WriteByte('"')
		b.WriteString(escapeFieldStringValue(v.String))
		b.WriteByte('"')
	}
}

// Escaping rules per InfluxDB line protocol spec.

func escapeMeasurement(s string) string {
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, " ", `\ `)
	return s
}

func escapeTagKey(s string) string {
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "=", `\=`)
	s = strings.ReplaceAll(s, " ", `\ `)
	return s
}

func escapeTagValue(s string) string {
	return escapeTagKey(s) // same rules
}

func escapeFieldKey(s string) string {
	return escapeTagKey(s) // same rules
}

func escapeFieldStringValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
