package logging

import (
	"strconv"
	"time"
)

type encoder interface {
	Encode(level Level, msg string, fields []Field) []byte
}

// jsonEncoder writes JSON-formatted log entries directly to a byte slice
// without intermediate map allocations.
type jsonEncoder struct{}

func (e *jsonEncoder) Encode(level Level, msg string, fields []Field) []byte {
	buf := make([]byte, 0, 256)
	buf = append(buf, '{')

	buf = appendJSONKey(buf, "level")
	buf = appendJSONString(buf, level.String())

	buf = append(buf, ',')
	buf = appendJSONKey(buf, "ts")
	buf = appendJSONString(buf, time.Now().UTC().Format(time.RFC3339Nano))

	buf = append(buf, ',')
	buf = appendJSONKey(buf, "msg")
	buf = appendJSONString(buf, msg)

	for _, f := range fields {
		buf = append(buf, ',')
		buf = appendJSONKey(buf, f.Key)
		buf = appendJSONFieldValue(buf, f)
	}

	buf = append(buf, '}')
	return buf
}

func appendJSONKey(buf []byte, key string) []byte {
	buf = append(buf, '"')
	buf = appendEscapedJSON(buf, key)
	buf = append(buf, '"', ':')
	return buf
}

func appendJSONString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	buf = appendEscapedJSON(buf, s)
	buf = append(buf, '"')
	return buf
}

func appendEscapedJSON(buf []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		default:
			if c < 0x20 {
				buf = append(buf, '\\', 'u', '0', '0')
				buf = append(buf, hexDigit(c>>4), hexDigit(c&0x0f))
			} else {
				buf = append(buf, c)
			}
		}
	}
	return buf
}

func hexDigit(b byte) byte {
	const digits = "0123456789abcdef"
	return digits[b]
}

func appendJSONFieldValue(buf []byte, f Field) []byte {
	switch f.Type {
	case FieldTypeString:
		buf = appendJSONString(buf, f.Str)
	case FieldTypeInt64:
		buf = strconv.AppendInt(buf, f.Int, 10)
	case FieldTypeFloat64:
		buf = strconv.AppendFloat(buf, f.Float, 'f', -1, 64)
	case FieldTypeBool:
		buf = strconv.AppendBool(buf, f.Bool)
	case FieldTypeError:
		if f.Err != nil {
			buf = appendJSONString(buf, f.Err.Error())
		} else {
			buf = append(buf, 'n', 'u', 'l', 'l')
		}
	case FieldTypeDuration:
		buf = appendJSONString(buf, f.Duration.String())
	case FieldTypeTime:
		buf = appendJSONString(buf, f.TimeValue.UTC().Format(time.RFC3339Nano))
	default:
		buf = append(buf, 'n', 'u', 'l', 'l')
	}
	return buf
}

// textEncoder writes human-readable log entries.
type textEncoder struct{}

func (e *textEncoder) Encode(level Level, msg string, fields []Field) []byte {
	buf := make([]byte, 0, 256)

	buf = append(buf, time.Now().UTC().Format("15:04:05.000")...)
	buf = append(buf, '\t')
	buf = append(buf, level.String()...)
	buf = append(buf, '\t')
	buf = append(buf, msg...)

	for _, f := range fields {
		buf = append(buf, '\t')
		buf = append(buf, f.Key...)
		buf = append(buf, '=')
		buf = appendTextFieldValue(buf, f)
	}

	return buf
}

func appendTextFieldValue(buf []byte, f Field) []byte {
	switch f.Type {
	case FieldTypeString:
		buf = append(buf, f.Str...)
	case FieldTypeInt64:
		buf = strconv.AppendInt(buf, f.Int, 10)
	case FieldTypeFloat64:
		buf = strconv.AppendFloat(buf, f.Float, 'f', -1, 64)
	case FieldTypeBool:
		buf = strconv.AppendBool(buf, f.Bool)
	case FieldTypeError:
		if f.Err != nil {
			buf = append(buf, f.Err.Error()...)
		} else {
			buf = append(buf, "<nil>"...)
		}
	case FieldTypeDuration:
		buf = append(buf, f.Duration.String()...)
	case FieldTypeTime:
		buf = append(buf, f.TimeValue.UTC().Format(time.RFC3339Nano)...)
	default:
		buf = append(buf, "<unknown>"...)
	}
	return buf
}
