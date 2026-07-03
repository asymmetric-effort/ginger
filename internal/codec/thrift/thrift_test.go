package thrift

import (
	"math"
	"testing"
)

func TestTTypeString(t *testing.T) {
	tests := []struct {
		t    TType
		want string
	}{
		{TypeStop, "stop"}, {TypeBool, "bool"}, {TypeByte, "byte"},
		{TypeI16, "i16"}, {TypeI32, "i32"}, {TypeI64, "i64"},
		{TypeDouble, "double"}, {TypeString, "string"},
		{TypeList, "list"}, {TypeSet, "set"}, {TypeMap, "map"},
		{TypeStruct, "struct"}, {TType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("TType(%d).String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestCompactTypeConversions(t *testing.T) {
	types := []TType{TypeBool, TypeByte, TypeI16, TypeI32, TypeI64, TypeDouble, TypeString, TypeList, TypeSet, TypeMap, TypeStruct}
	for _, tt := range types {
		ct := tTypeToCompact(tt)
		got := compactTypeToTType(ct)
		if got != tt {
			t.Errorf("round-trip TType %v: compact=%d, back=%v", tt, ct, got)
		}
	}
	if got := compactTypeToTType(0); got != TypeStop {
		t.Errorf("unknown compact type should map to Stop, got %v", got)
	}
	if got := tTypeToCompact(TypeStop); got != 0 {
		t.Errorf("Stop should map to 0, got %d", got)
	}
}

// === Binary Protocol Tests ===

func TestBinaryBool(t *testing.T) {
	for _, v := range []bool{true, false} {
		enc := NewBinaryEncoder()
		enc.WriteBool(v)
		dec := NewBinaryDecoder(enc.Bytes())
		got, err := dec.ReadBool()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("bool round-trip: %v != %v", got, v)
		}
	}
}

func TestBinaryByte(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteByte(0xAB)
	dec := NewBinaryDecoder(enc.Bytes())
	got, err := dec.ReadByte()
	if err != nil {
		t.Fatal(err)
	}
	if got != 0xAB {
		t.Errorf("byte: got %x, want AB", got)
	}
}

func TestBinaryI16(t *testing.T) {
	for _, v := range []int16{0, 1, -1, 32767, -32768} {
		enc := NewBinaryEncoder()
		enc.WriteI16(v)
		dec := NewBinaryDecoder(enc.Bytes())
		got, err := dec.ReadI16()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("i16: %d != %d", got, v)
		}
	}
}

func TestBinaryI32(t *testing.T) {
	for _, v := range []int32{0, 1, -1, math.MaxInt32, math.MinInt32} {
		enc := NewBinaryEncoder()
		enc.WriteI32(v)
		dec := NewBinaryDecoder(enc.Bytes())
		got, err := dec.ReadI32()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("i32: %d != %d", got, v)
		}
	}
}

func TestBinaryI64(t *testing.T) {
	for _, v := range []int64{0, 1, -1, math.MaxInt64, math.MinInt64} {
		enc := NewBinaryEncoder()
		enc.WriteI64(v)
		dec := NewBinaryDecoder(enc.Bytes())
		got, err := dec.ReadI64()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("i64: %d != %d", got, v)
		}
	}
}

func TestBinaryDouble(t *testing.T) {
	for _, v := range []float64{0, 3.14, -2.718, math.MaxFloat64} {
		enc := NewBinaryEncoder()
		enc.WriteDouble(v)
		dec := NewBinaryDecoder(enc.Bytes())
		got, err := dec.ReadDouble()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("double: %f != %f", got, v)
		}
	}
}

func TestBinaryString(t *testing.T) {
	for _, v := range []string{"", "hello", "hello world"} {
		enc := NewBinaryEncoder()
		enc.WriteString(v)
		dec := NewBinaryDecoder(enc.Bytes())
		got, err := dec.ReadString()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("string: %q != %q", got, v)
		}
	}
}

func TestBinaryBytes(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteBytes([]byte{0xDE, 0xAD})
	dec := NewBinaryDecoder(enc.Bytes())
	got, err := dec.ReadBytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 0xDE || got[1] != 0xAD {
		t.Errorf("bytes: %x", got)
	}
}

func TestBinaryFieldRoundTrip(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteFieldBegin(TypeI32, 1)
	enc.WriteI32(42)
	enc.WriteFieldBegin(TypeString, 2)
	enc.WriteString("hello")
	enc.WriteFieldStop()

	dec := NewBinaryDecoder(enc.Bytes())
	ft, fid, err := dec.ReadFieldBegin()
	if err != nil || ft != TypeI32 || fid != 1 {
		t.Fatalf("field 1: type=%v id=%d err=%v", ft, fid, err)
	}
	v32, _ := dec.ReadI32()
	if v32 != 42 {
		t.Error("field 1 value")
	}

	ft, fid, err = dec.ReadFieldBegin()
	if err != nil || ft != TypeString || fid != 2 {
		t.Fatalf("field 2: type=%v id=%d err=%v", ft, fid, err)
	}
	vs, _ := dec.ReadString()
	if vs != "hello" {
		t.Error("field 2 value")
	}

	ft, _, err = dec.ReadFieldBegin()
	if err != nil || ft != TypeStop {
		t.Error("expected stop")
	}
}

func TestBinaryList(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteListBegin(TypeI32, 3)
	enc.WriteI32(1)
	enc.WriteI32(2)
	enc.WriteI32(3)

	dec := NewBinaryDecoder(enc.Bytes())
	et, size, err := dec.ReadListBegin()
	if err != nil || et != TypeI32 || size != 3 {
		t.Fatalf("list: type=%v size=%d err=%v", et, size, err)
	}
}

func TestBinaryMap(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteMapBegin(TypeString, TypeI32, 2)
	enc.WriteString("a")
	enc.WriteI32(1)
	enc.WriteString("b")
	enc.WriteI32(2)

	dec := NewBinaryDecoder(enc.Bytes())
	kt, vt, size, err := dec.ReadMapBegin()
	if err != nil || kt != TypeString || vt != TypeI32 || size != 2 {
		t.Fatalf("map: kt=%v vt=%v size=%d err=%v", kt, vt, size, err)
	}
}

func TestBinaryReset(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteI32(42)
	enc.Reset()
	if len(enc.Bytes()) != 0 {
		t.Error("expected empty after reset")
	}
}

func TestBinaryUnderflow(t *testing.T) {
	dec := NewBinaryDecoder(nil)
	if _, err := dec.ReadBool(); err != ErrUnderflow {
		t.Error("bool underflow")
	}
	if _, err := dec.ReadByte(); err != ErrUnderflow {
		t.Error("byte underflow")
	}
	if _, err := dec.ReadI16(); err != ErrUnderflow {
		t.Error("i16 underflow")
	}
	if _, err := dec.ReadI32(); err != ErrUnderflow {
		t.Error("i32 underflow")
	}
	if _, err := dec.ReadI64(); err != ErrUnderflow {
		t.Error("i64 underflow")
	}
	if _, err := dec.ReadDouble(); err != ErrUnderflow {
		t.Error("double underflow")
	}
	if _, err := dec.ReadString(); err != ErrUnderflow {
		t.Error("string underflow")
	}
	if _, err := dec.ReadBytes(); err != ErrUnderflow {
		t.Error("bytes underflow")
	}
}

func TestBinaryStringLengthUnderflow(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteI32(100) // says 100 bytes
	dec := NewBinaryDecoder(enc.Bytes())
	_, err := dec.ReadString()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
}

func TestBinaryBytesLengthUnderflow(t *testing.T) {
	enc := NewBinaryEncoder()
	enc.WriteI32(100)
	dec := NewBinaryDecoder(enc.Bytes())
	_, err := dec.ReadBytes()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
}

func TestBinaryFieldBeginUnderflow(t *testing.T) {
	// Only type byte, no field ID
	dec := NewBinaryDecoder([]byte{byte(TypeI32)})
	_, _, err := dec.ReadFieldBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for field ID")
	}
}

func TestBinaryListBeginErrors(t *testing.T) {
	dec := NewBinaryDecoder(nil)
	_, _, err := dec.ReadListBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
	dec = NewBinaryDecoder([]byte{byte(TypeI32)}) // type but no size
	_, _, err = dec.ReadListBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for size")
	}
}

func TestBinaryMapBeginErrors(t *testing.T) {
	dec := NewBinaryDecoder(nil)
	_, _, _, err := dec.ReadMapBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for key type")
	}
	dec = NewBinaryDecoder([]byte{byte(TypeString)})
	_, _, _, err = dec.ReadMapBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for value type")
	}
	dec = NewBinaryDecoder([]byte{byte(TypeString), byte(TypeI32)})
	_, _, _, err = dec.ReadMapBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for size")
	}
}

func TestBinaryDoneRemaining(t *testing.T) {
	dec := NewBinaryDecoder([]byte{1, 2})
	if dec.Done() {
		t.Error("should not be done")
	}
	if dec.Remaining() != 2 {
		t.Errorf("remaining = %d", dec.Remaining())
	}
}

// === Compact Protocol Tests ===

func TestCompactBool(t *testing.T) {
	for _, v := range []bool{true, false} {
		enc := NewCompactEncoder()
		enc.WriteBool(v)
		dec := NewCompactDecoder(enc.Bytes())
		got, err := dec.ReadBool()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("bool: %v != %v", got, v)
		}
	}
}

func TestCompactI16(t *testing.T) {
	for _, v := range []int16{0, 1, -1, 32767, -32768} {
		enc := NewCompactEncoder()
		enc.WriteI16(v)
		dec := NewCompactDecoder(enc.Bytes())
		got, err := dec.ReadI16()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("i16: %d != %d", got, v)
		}
	}
}

func TestCompactI32(t *testing.T) {
	for _, v := range []int32{0, 1, -1, math.MaxInt32, math.MinInt32} {
		enc := NewCompactEncoder()
		enc.WriteI32(v)
		dec := NewCompactDecoder(enc.Bytes())
		got, err := dec.ReadI32()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("i32: %d != %d", got, v)
		}
	}
}

func TestCompactI64(t *testing.T) {
	for _, v := range []int64{0, 1, -1, math.MaxInt64, math.MinInt64} {
		enc := NewCompactEncoder()
		enc.WriteI64(v)
		dec := NewCompactDecoder(enc.Bytes())
		got, err := dec.ReadI64()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("i64: %d != %d", got, v)
		}
	}
}

func TestCompactDouble(t *testing.T) {
	for _, v := range []float64{0, 3.14, -2.718} {
		enc := NewCompactEncoder()
		enc.WriteDouble(v)
		dec := NewCompactDecoder(enc.Bytes())
		got, err := dec.ReadDouble()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("double: %f != %f", got, v)
		}
	}
}

func TestCompactString(t *testing.T) {
	for _, v := range []string{"", "hello", "world"} {
		enc := NewCompactEncoder()
		enc.WriteString(v)
		dec := NewCompactDecoder(enc.Bytes())
		got, err := dec.ReadString()
		if err != nil {
			t.Fatal(err)
		}
		if got != v {
			t.Errorf("string: %q != %q", got, v)
		}
	}
}

func TestCompactBytes(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteBytes([]byte{0xDE, 0xAD})
	dec := NewCompactDecoder(enc.Bytes())
	got, err := dec.ReadBytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("bytes len = %d", len(got))
	}
}

func TestCompactFieldDelta(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteFieldBegin(TypeI32, 1)
	enc.WriteI32(42)
	enc.WriteFieldBegin(TypeString, 2)
	enc.WriteString("hi")
	enc.WriteFieldStop()

	dec := NewCompactDecoder(enc.Bytes())
	ft, fid, err := dec.ReadFieldBegin()
	if err != nil || ft != TypeI32 || fid != 1 {
		t.Fatalf("field 1: %v %d %v", ft, fid, err)
	}
	v32, _ := dec.ReadI32()
	if v32 != 42 {
		t.Error("field 1 value")
	}

	ft, fid, err = dec.ReadFieldBegin()
	if err != nil || ft != TypeString || fid != 2 {
		t.Fatalf("field 2: %v %d %v", ft, fid, err)
	}

	vs, _ := dec.ReadString()
	if vs != "hi" {
		t.Error("field 2 value")
	}

	ft, _, err = dec.ReadFieldBegin()
	if err != nil || ft != TypeStop {
		t.Error("expected stop")
	}
}

func TestCompactFieldLargeDelta(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteFieldBegin(TypeI32, 100) // delta > 15
	enc.WriteI32(99)
	enc.WriteFieldStop()

	dec := NewCompactDecoder(enc.Bytes())
	ft, fid, err := dec.ReadFieldBegin()
	if err != nil || ft != TypeI32 || fid != 100 {
		t.Fatalf("large delta field: %v %d %v", ft, fid, err)
	}
}

func TestCompactBoolField(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteFieldBeginBool(1, true)
	enc.WriteFieldBeginBool(2, false)
	enc.WriteFieldStop()

	dec := NewCompactDecoder(enc.Bytes())

	ft, fid, err := dec.ReadFieldBegin()
	if err != nil || ft != TypeBool || fid != 1 {
		t.Fatalf("bool field 1: %v %d %v", ft, fid, err)
	}
	bv, _ := dec.ReadBool()
	if !bv {
		t.Error("expected true")
	}

	ft, fid, err = dec.ReadFieldBegin()
	if err != nil || ft != TypeBool || fid != 2 {
		t.Fatalf("bool field 2: %v %d %v", ft, fid, err)
	}
	bv, _ = dec.ReadBool()
	if bv {
		t.Error("expected false")
	}
}

func TestCompactBoolFieldLargeDelta(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteFieldBeginBool(100, true) // delta > 15
	enc.WriteFieldStop()

	dec := NewCompactDecoder(enc.Bytes())
	ft, fid, err := dec.ReadFieldBegin()
	if err != nil || ft != TypeBool || fid != 100 {
		t.Fatalf("bool large delta: %v %d %v", ft, fid, err)
	}
}

func TestCompactList(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteListBegin(TypeI32, 3) // size <= 14
	enc.WriteI32(1)
	enc.WriteI32(2)
	enc.WriteI32(3)

	dec := NewCompactDecoder(enc.Bytes())
	et, size, err := dec.ReadListBegin()
	if err != nil || et != TypeI32 || size != 3 {
		t.Fatalf("list: %v %d %v", et, size, err)
	}
}

func TestCompactListLarge(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteListBegin(TypeI32, 20) // size > 14

	dec := NewCompactDecoder(enc.Bytes())
	et, size, err := dec.ReadListBegin()
	if err != nil || et != TypeI32 || size != 20 {
		t.Fatalf("large list: %v %d %v", et, size, err)
	}
}

func TestCompactMap(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteMapBegin(TypeString, TypeI32, 2)

	dec := NewCompactDecoder(enc.Bytes())
	kt, vt, size, err := dec.ReadMapBegin()
	if err != nil || kt != TypeString || vt != TypeI32 || size != 2 {
		t.Fatalf("map: kt=%v vt=%v size=%d err=%v", kt, vt, size, err)
	}
}

func TestCompactMapEmpty(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteMapBegin(TypeString, TypeI32, 0)

	dec := NewCompactDecoder(enc.Bytes())
	_, _, size, err := dec.ReadMapBegin()
	if err != nil || size != 0 {
		t.Fatalf("empty map: size=%d err=%v", size, err)
	}
}

func TestCompactStruct(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteStructBegin()
	enc.WriteFieldBegin(TypeI32, 1)
	enc.WriteI32(42)
	enc.WriteFieldStop()
	enc.WriteStructEnd()

	if len(enc.Bytes()) == 0 {
		t.Error("struct should produce output")
	}
}

func TestCompactReset(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteI32(42)
	enc.Reset()
	if len(enc.Bytes()) != 0 {
		t.Error("expected empty after reset")
	}
}

func TestCompactUnderflow(t *testing.T) {
	dec := NewCompactDecoder(nil)
	if _, err := dec.ReadBool(); err != ErrUnderflow {
		t.Error("bool underflow")
	}
	if _, err := dec.ReadByte(); err != ErrUnderflow {
		t.Error("byte underflow")
	}
	if _, err := dec.ReadI16(); err != ErrUnderflow {
		t.Error("i16 underflow")
	}
	if _, err := dec.ReadDouble(); err != ErrUnderflow {
		t.Error("double underflow")
	}
	if _, err := dec.ReadString(); err != ErrUnderflow {
		t.Error("string underflow")
	}
	if _, err := dec.ReadBytes(); err != ErrUnderflow {
		t.Error("bytes underflow")
	}
}

func TestCompactStringLengthUnderflow(t *testing.T) {
	enc := NewCompactEncoder()
	enc.writeVarint(100) // says 100 bytes
	dec := NewCompactDecoder(enc.Bytes())
	_, err := dec.ReadString()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
}

func TestCompactBytesLengthUnderflow(t *testing.T) {
	enc := NewCompactEncoder()
	enc.writeVarint(100)
	dec := NewCompactDecoder(enc.Bytes())
	_, err := dec.ReadBytes()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
}

func TestCompactFieldBeginUnderflow(t *testing.T) {
	dec := NewCompactDecoder(nil)
	_, _, err := dec.ReadFieldBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
}

func TestCompactFieldBeginIDUnderflow(t *testing.T) {
	// Type byte with delta=0 (need to read separate field ID) but no data
	dec := NewCompactDecoder([]byte{compactI32}) // delta=0, type=i32
	_, _, err := dec.ReadFieldBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for field ID")
	}
}

func TestCompactListBeginUnderflow(t *testing.T) {
	dec := NewCompactDecoder(nil)
	_, _, err := dec.ReadListBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
}

func TestCompactListBeginLargeSizeUnderflow(t *testing.T) {
	// Header byte with size=15 (need varint) but no varint data
	dec := NewCompactDecoder([]byte{0xf0 | compactI32})
	_, _, err := dec.ReadListBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for large list size")
	}
}

func TestCompactMapBeginUnderflow(t *testing.T) {
	dec := NewCompactDecoder(nil)
	_, _, _, err := dec.ReadMapBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow")
	}
}

func TestCompactMapBeginTypesUnderflow(t *testing.T) {
	// Size varint = 1, but no type byte
	enc := NewCompactEncoder()
	enc.writeVarint(1)
	dec := NewCompactDecoder(enc.Bytes())
	_, _, _, err := dec.ReadMapBegin()
	if err != ErrUnderflow {
		t.Error("expected underflow for map types")
	}
}

func TestCompactDoneRemaining(t *testing.T) {
	dec := NewCompactDecoder([]byte{1, 2})
	if dec.Done() {
		t.Error("should not be done")
	}
	if dec.Remaining() != 2 {
		t.Errorf("remaining = %d", dec.Remaining())
	}
}

func TestCompactStructNested(t *testing.T) {
	enc := NewCompactEncoder()
	enc.WriteFieldBegin(TypeStruct, 1)
	enc.WriteStructBegin()
	enc.WriteFieldBegin(TypeI32, 1)
	enc.WriteI32(42)
	enc.WriteFieldStop()
	enc.WriteStructEnd()
	enc.WriteFieldStop()

	dec := NewCompactDecoder(enc.Bytes())
	ft, fid, err := dec.ReadFieldBegin()
	if err != nil || ft != TypeStruct || fid != 1 {
		t.Fatalf("outer: %v %d %v", ft, fid, err)
	}
	dec.ReadStructBegin()
	ft, fid, err = dec.ReadFieldBegin()
	if err != nil || ft != TypeI32 || fid != 1 {
		t.Fatalf("inner: %v %d %v", ft, fid, err)
	}
	v, _ := dec.ReadI32()
	if v != 42 {
		t.Error("inner value")
	}
	ft, _, _ = dec.ReadFieldBegin()
	if ft != TypeStop {
		t.Error("inner stop")
	}
	dec.ReadStructEnd()
	ft, _, _ = dec.ReadFieldBegin()
	if ft != TypeStop {
		t.Error("outer stop")
	}
}
