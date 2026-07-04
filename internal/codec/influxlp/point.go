package influxlp

import "time"

// FieldValue represents an InfluxDB field value.
type FieldValue struct {
	Type   FieldType
	String string
	Int    int64
	Float  float64
	Bool   bool
	UInt   uint64
}

// FieldType identifies the type of a field value.
type FieldType byte

const (
	FieldTypeFloat FieldType = iota
	FieldTypeInt
	FieldTypeUInt
	FieldTypeBool
	FieldTypeString
)

// Point represents an InfluxDB line protocol data point.
type Point struct {
	Measurement string
	Tags        []Tag
	Fields      []Field
	Timestamp   time.Time
}

// Tag is a key-value pair for indexing.
type Tag struct {
	Key   string
	Value string
}

// Field is a key-value pair for data storage.
type Field struct {
	Key   string
	Value FieldValue
}

// StringField creates a string field.
func StringField(key, value string) Field {
	return Field{Key: key, Value: FieldValue{Type: FieldTypeString, String: value}}
}

// IntField creates an integer field.
func IntField(key string, value int64) Field {
	return Field{Key: key, Value: FieldValue{Type: FieldTypeInt, Int: value}}
}

// UIntField creates an unsigned integer field.
func UIntField(key string, value uint64) Field {
	return Field{Key: key, Value: FieldValue{Type: FieldTypeUInt, UInt: value}}
}

// FloatField creates a float field.
func FloatField(key string, value float64) Field {
	return Field{Key: key, Value: FieldValue{Type: FieldTypeFloat, Float: value}}
}

// BoolField creates a boolean field.
func BoolField(key string, value bool) Field {
	return Field{Key: key, Value: FieldValue{Type: FieldTypeBool, Bool: value}}
}
