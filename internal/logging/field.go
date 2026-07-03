package logging

import "time"

// FieldType identifies the type of a Field value.
type FieldType uint8

const (
	// FieldTypeString is a string value.
	FieldTypeString FieldType = iota
	// FieldTypeInt64 is an int64 value.
	FieldTypeInt64
	// FieldTypeFloat64 is a float64 value.
	FieldTypeFloat64
	// FieldTypeBool is a bool value.
	FieldTypeBool
	// FieldTypeError is an error value.
	FieldTypeError
	// FieldTypeDuration is a time.Duration value.
	FieldTypeDuration
	// FieldTypeTime is a time.Time value.
	FieldTypeTime
)

// Field is a tagged union representing a key-value pair for structured logging.
// No interface{} — the value is stored in typed fields selected by Type.
type Field struct {
	Key       string
	Type      FieldType
	Str       string
	Int       int64
	Float     float64
	Bool      bool
	Err       error
	Duration  time.Duration
	TimeValue time.Time
}

// String creates a string field.
func String(key, val string) Field {
	return Field{Key: key, Type: FieldTypeString, Str: val}
}

// Int64 creates an int64 field.
func Int64(key string, val int64) Field {
	return Field{Key: key, Type: FieldTypeInt64, Int: val}
}

// Int creates an int field (stored as int64).
func Int(key string, val int) Field {
	return Field{Key: key, Type: FieldTypeInt64, Int: int64(val)}
}

// Float64 creates a float64 field.
func Float64(key string, val float64) Field {
	return Field{Key: key, Type: FieldTypeFloat64, Float: val}
}

// Bool creates a bool field.
func Bool(key string, val bool) Field {
	return Field{Key: key, Type: FieldTypeBool, Bool: val}
}

// Error creates an error field.
func Error(err error) Field {
	return Field{Key: "error", Type: FieldTypeError, Err: err}
}

// NamedError creates an error field with a custom key.
func NamedError(key string, err error) Field {
	return Field{Key: key, Type: FieldTypeError, Err: err}
}

// Duration creates a duration field.
func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Type: FieldTypeDuration, Duration: val}
}

// Time creates a time field.
func Time(key string, val time.Time) Field {
	return Field{Key: key, Type: FieldTypeTime, TimeValue: val}
}
