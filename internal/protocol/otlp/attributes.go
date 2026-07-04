package otlp

import "encoding/json"

// AnyValueType identifies the type of an AnyValue.
type AnyValueType byte

const (
	AnyValueTypeString AnyValueType = iota
	AnyValueTypeBool
	AnyValueTypeInt
	AnyValueTypeDouble
	AnyValueTypeBytes
	AnyValueTypeArray
	AnyValueTypeKvList
)

// AnyValue is a tagged union for attribute values. No interface{}.
type AnyValue struct {
	Type      AnyValueType
	Str       string
	BoolVal   bool
	IntVal    int64
	DoubleVal float64
	BytesVal  []byte
	ArrayVal  []AnyValue
	KvListVal []KeyValue
}

// StringValue creates a string AnyValue.
func StringValue(s string) AnyValue {
	return AnyValue{Type: AnyValueTypeString, Str: s}
}

// BoolValue creates a bool AnyValue.
func BoolValue(b bool) AnyValue {
	return AnyValue{Type: AnyValueTypeBool, BoolVal: b}
}

// IntValue creates an int64 AnyValue.
func IntValue(v int64) AnyValue {
	return AnyValue{Type: AnyValueTypeInt, IntVal: v}
}

// DoubleValue creates a float64 AnyValue.
func DoubleValue(v float64) AnyValue {
	return AnyValue{Type: AnyValueTypeDouble, DoubleVal: v}
}

// BytesValue creates a bytes AnyValue.
func BytesValue(b []byte) AnyValue {
	cp := make([]byte, len(b))
	copy(cp, b)
	return AnyValue{Type: AnyValueTypeBytes, BytesVal: cp}
}

// ArrayValue creates an array AnyValue.
func ArrayValue(items []AnyValue) AnyValue {
	return AnyValue{Type: AnyValueTypeArray, ArrayVal: items}
}

// KvListValue creates a key-value list AnyValue.
func KvListValue(kvs []KeyValue) AnyValue {
	return AnyValue{Type: AnyValueTypeKvList, KvListVal: kvs}
}

// MarshalJSON implements JSON serialization for AnyValue.
func (v AnyValue) MarshalJSON() ([]byte, error) {
	switch v.Type {
	case AnyValueTypeString:
		return json.Marshal(map[string]string{"stringValue": v.Str})
	case AnyValueTypeBool:
		return json.Marshal(map[string]bool{"boolValue": v.BoolVal})
	case AnyValueTypeInt:
		return json.Marshal(map[string]string{"intValue": formatInt64(v.IntVal)})
	case AnyValueTypeDouble:
		return json.Marshal(map[string]float64{"doubleValue": v.DoubleVal})
	case AnyValueTypeBytes:
		return json.Marshal(map[string][]byte{"bytesValue": v.BytesVal})
	case AnyValueTypeArray:
		return json.Marshal(map[string]interface{}{"arrayValue": map[string][]AnyValue{"values": v.ArrayVal}})
	case AnyValueTypeKvList:
		return json.Marshal(map[string]interface{}{"kvlistValue": map[string][]KeyValue{"values": v.KvListVal}})
	default:
		return []byte("null"), nil
	}
}

// KeyValue is an attribute key-value pair.
type KeyValue struct {
	Key   string   `json:"key"`
	Value AnyValue `json:"value"`
}

// Attributes is an ordered list of key-value pairs.
type Attributes struct {
	items []KeyValue
}

// NewAttributes creates an empty Attributes.
func NewAttributes() Attributes {
	return Attributes{}
}

// NewAttributesFromSlice creates Attributes from a slice of KeyValue.
func NewAttributesFromSlice(kvs []KeyValue) Attributes {
	items := make([]KeyValue, len(kvs))
	copy(items, kvs)
	return Attributes{items: items}
}

// Set adds or updates a key-value pair.
func (a *Attributes) Set(key string, value AnyValue) {
	for i := range a.items {
		if a.items[i].Key == key {
			a.items[i].Value = value
			return
		}
	}
	a.items = append(a.items, KeyValue{Key: key, Value: value})
}

// Get returns the value for the given key and whether it was found.
func (a Attributes) Get(key string) (AnyValue, bool) {
	for i := range a.items {
		if a.items[i].Key == key {
			return a.items[i].Value, true
		}
	}
	return AnyValue{}, false
}

// Len returns the number of attributes.
func (a Attributes) Len() int {
	return len(a.items)
}

// Items returns a copy of the key-value pairs.
func (a Attributes) Items() []KeyValue {
	out := make([]KeyValue, len(a.items))
	copy(out, a.items)
	return out
}

func formatInt64(v int64) string {
	buf := make([]byte, 0, 20)
	if v < 0 {
		buf = append(buf, '-')
		v = -v
		if v < 0 { // MinInt64
			return "-9223372036854775808"
		}
	}
	if v == 0 {
		return "0"
	}
	start := len(buf)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	// Reverse digits
	for i, j := start, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
