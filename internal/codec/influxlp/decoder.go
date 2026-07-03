package influxlp

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Decoder decodes InfluxDB line protocol data.
type Decoder struct {
	Precision Precision
}

// NewDecoder creates a new Decoder with nanosecond precision.
func NewDecoder() *Decoder {
	return &Decoder{Precision: PrecisionNanosecond}
}

// Decode parses line protocol data into Points.
func (d *Decoder) Decode(data []byte) ([]Point, error) {
	lines := strings.Split(string(data), "\n")
	var points []Point
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p, err := d.decodeLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		points = append(points, p)
	}
	return points, nil
}

func (d *Decoder) decodeLine(line string) (Point, error) {
	var p Point

	// Split into: measurement[,tags] fields [timestamp]
	// First space separates measurement+tags from fields
	// Tags are comma-separated after measurement (no space)
	// Fields are comma-separated after the first space
	// Timestamp is after the second space (optional)

	measurementEnd := findUnescaped(line, ' ', 0)
	if measurementEnd < 0 {
		return p, fmt.Errorf("missing fields")
	}

	measurementPart := line[:measurementEnd]
	rest := line[measurementEnd+1:]

	// Parse measurement and tags
	tagStart := findUnescaped(measurementPart, ',', 0)
	if tagStart < 0 {
		p.Measurement = unescapeMeasurement(measurementPart)
	} else {
		p.Measurement = unescapeMeasurement(measurementPart[:tagStart])
		tagStr := measurementPart[tagStart+1:]
		tags, err := parseTags(tagStr)
		if err != nil {
			return p, err
		}
		p.Tags = tags
	}

	// Split fields from optional timestamp
	fieldEnd := findUnescaped(rest, ' ', 0)
	var fieldStr, tsStr string
	if fieldEnd < 0 {
		fieldStr = rest
	} else {
		fieldStr = rest[:fieldEnd]
		tsStr = strings.TrimSpace(rest[fieldEnd+1:])
	}

	fields, err := parseFields(fieldStr)
	if err != nil {
		return p, err
	}
	p.Fields = fields

	if tsStr != "" {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			return p, fmt.Errorf("invalid timestamp: %w", err)
		}
		p.Timestamp = d.parseTimestamp(ts)
	}

	return p, nil
}

func (d *Decoder) parseTimestamp(v int64) time.Time {
	switch d.Precision {
	case PrecisionSecond:
		return time.Unix(v, 0).UTC()
	case PrecisionMillisecond:
		return time.UnixMilli(v).UTC()
	case PrecisionMicrosecond:
		return time.UnixMicro(v).UTC()
	default:
		return time.Unix(0, v).UTC()
	}
}

func parseTags(s string) ([]Tag, error) {
	var tags []Tag
	parts := splitUnescaped(s, ',')
	for _, part := range parts {
		eqIdx := findUnescaped(part, '=', 0)
		if eqIdx < 0 {
			return nil, fmt.Errorf("invalid tag: %q", part)
		}
		tags = append(tags, Tag{
			Key:   unescapeTagKey(part[:eqIdx]),
			Value: unescapeTagValue(part[eqIdx+1:]),
		})
	}
	return tags, nil
}

func parseFields(s string) ([]Field, error) {
	var fields []Field
	parts := splitUnescaped(s, ',')
	for _, part := range parts {
		eqIdx := findUnescaped(part, '=', 0)
		if eqIdx < 0 {
			return nil, fmt.Errorf("invalid field: %q", part)
		}
		key := unescapeFieldKey(part[:eqIdx])
		valStr := part[eqIdx+1:]
		val, err := parseFieldValue(valStr)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
		fields = append(fields, Field{Key: key, Value: val})
	}
	return fields, nil
}

func parseFieldValue(s string) (FieldValue, error) {
	if len(s) == 0 {
		return FieldValue{}, fmt.Errorf("empty field value")
	}

	// String value: "..."
	if s[0] == '"' {
		if len(s) < 2 || s[len(s)-1] != '"' {
			return FieldValue{}, fmt.Errorf("unterminated string")
		}
		return FieldValue{Type: FieldTypeString, String: unescapeFieldStringValue(s[1 : len(s)-1])}, nil
	}

	// Boolean
	if s == "t" || s == "T" || s == "true" || s == "True" || s == "TRUE" {
		return FieldValue{Type: FieldTypeBool, Bool: true}, nil
	}
	if s == "f" || s == "F" || s == "false" || s == "False" || s == "FALSE" {
		return FieldValue{Type: FieldTypeBool, Bool: false}, nil
	}

	// Integer (ends with 'i')
	if s[len(s)-1] == 'i' {
		v, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
		if err != nil {
			return FieldValue{}, fmt.Errorf("invalid integer: %w", err)
		}
		return FieldValue{Type: FieldTypeInt, Int: v}, nil
	}

	// Unsigned integer (ends with 'u')
	if s[len(s)-1] == 'u' {
		v, err := strconv.ParseUint(s[:len(s)-1], 10, 64)
		if err != nil {
			return FieldValue{}, fmt.Errorf("invalid unsigned integer: %w", err)
		}
		return FieldValue{Type: FieldTypeUInt, UInt: v}, nil
	}

	// Float (default)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return FieldValue{}, fmt.Errorf("invalid float: %w", err)
	}
	return FieldValue{Type: FieldTypeFloat, Float: v}, nil
}

// findUnescaped finds the first unescaped occurrence of ch starting at pos.
func findUnescaped(s string, ch byte, pos int) int {
	inQuote := false
	for i := pos; i < len(s); i++ {
		if s[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			if s[i] == '\\' && i+1 < len(s) {
				i++ // skip escaped char inside quotes
			}
			continue
		}
		if s[i] == '\\' && i+1 < len(s) {
			i++ // skip escaped char
			continue
		}
		if s[i] == ch {
			return i
		}
	}
	return -1
}

// splitUnescaped splits on unescaped occurrences of sep.
func splitUnescaped(s string, sep byte) []string {
	var parts []string
	start := 0
	for {
		idx := findUnescaped(s, sep, start)
		if idx < 0 {
			parts = append(parts, s[start:])
			break
		}
		parts = append(parts, s[start:idx])
		start = idx + 1
	}
	return parts
}

func unescapeMeasurement(s string) string {
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\ `, " ")
	return s
}

func unescapeTagKey(s string) string {
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\=`, "=")
	s = strings.ReplaceAll(s, `\ `, " ")
	return s
}

func unescapeTagValue(s string) string {
	return unescapeTagKey(s)
}

func unescapeFieldKey(s string) string {
	return unescapeTagKey(s)
}

func unescapeFieldStringValue(s string) string {
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}
