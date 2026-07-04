package protobuf

import (
	"math"
	"testing"
)

func TestWireTypeString(t *testing.T) {
	tests := []struct {
		w    WireType
		want string
	}{
		{WireVarint, "varint"},
		{Wire64Bit, "64-bit"},
		{WireBytes, "bytes"},
		{Wire32Bit, "32-bit"},
		{WireType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.w.String(); got != tt.want {
			t.Errorf("WireType(%d).String() = %q, want %q", tt.w, got, tt.want)
		}
	}
}

func TestMakeParseTag(t *testing.T) {
	tests := []struct {
		field uint32
		wire  WireType
	}{
		{1, WireVarint},
		{2, WireBytes},
		{15, Wire64Bit},
		{16, Wire32Bit},
		{100, WireVarint},
	}
	for _, tt := range tests {
		tag := MakeTag(tt.field, tt.wire)
		field, wire := ParseTag(tag)
		if field != tt.field || wire != tt.wire {
			t.Errorf("MakeTag(%d, %d) round-trip: got (%d, %d)", tt.field, tt.wire, field, wire)
		}
	}
}

func TestVarintRoundTrip(t *testing.T) {
	values := []uint64{0, 1, 127, 128, 255, 256, 16383, 16384, 1<<32 - 1, 1<<63 - 1, math.MaxUint64}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteVarint(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadVarint()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadVarint(%d) error: %v", v, err)
		}
		if got != v {
			t.Errorf("varint round-trip: wrote %d, got %d", v, got)
		}
		if !dec.Done() {
			t.Errorf("decoder should be done after reading varint %d", v)
		}
	}
}

func TestSignedVarintRoundTrip(t *testing.T) {
	values := []int64{0, 1, -1, 127, -128, math.MaxInt64, math.MinInt64}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteSignedVarint(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadSignedVarint()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadSignedVarint(%d) error: %v", v, err)
		}
		if got != v {
			t.Errorf("signed varint round-trip: wrote %d, got %d", v, got)
		}
	}
}

func TestZigzagRoundTrip(t *testing.T) {
	values := []int64{0, 1, -1, 127, -128, 300, -300, math.MaxInt64, math.MinInt64}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteZigzag(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadZigzag()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadZigzag(%d) error: %v", v, err)
		}
		if got != v {
			t.Errorf("zigzag round-trip: wrote %d, got %d", v, got)
		}
	}
}

func TestFixed32RoundTrip(t *testing.T) {
	values := []uint32{0, 1, 255, 256, math.MaxUint32}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteFixed32(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadFixed32()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadFixed32(%d) error: %v", v, err)
		}
		if got != v {
			t.Errorf("fixed32 round-trip: wrote %d, got %d", v, got)
		}
	}
}

func TestFixed64RoundTrip(t *testing.T) {
	values := []uint64{0, 1, math.MaxUint32, math.MaxUint64}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteFixed64(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadFixed64()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadFixed64(%d) error: %v", v, err)
		}
		if got != v {
			t.Errorf("fixed64 round-trip: wrote %d, got %d", v, got)
		}
	}
}

func TestFloatRoundTrip(t *testing.T) {
	values := []float32{0, 1.5, -3.14, math.MaxFloat32, math.SmallestNonzeroFloat32}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteFloat(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadFloat()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadFloat(%f) error: %v", v, err)
		}
		if got != v {
			t.Errorf("float round-trip: wrote %f, got %f", v, got)
		}
	}
}

func TestDoubleRoundTrip(t *testing.T) {
	values := []float64{0, 1.5, -3.14, math.MaxFloat64, math.SmallestNonzeroFloat64}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteDouble(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadDouble()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadDouble(%f) error: %v", v, err)
		}
		if got != v {
			t.Errorf("double round-trip: wrote %f, got %f", v, got)
		}
	}
}

func TestBytesRoundTrip(t *testing.T) {
	values := [][]byte{{}, {0x01}, {0x00, 0xff, 0x80}, make([]byte, 300)}
	for i, v := range values {
		enc := NewEncoder()
		enc.WriteBytes(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadBytes()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadBytes[%d] error: %v", i, err)
		}
		if len(got) != len(v) {
			t.Errorf("bytes round-trip[%d]: len wrote %d, got %d", i, len(v), len(got))
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	values := []string{"", "hello", "hello world", string(make([]byte, 300))}
	for _, v := range values {
		enc := NewEncoder()
		enc.WriteString(v)
		dec := NewDecoder(enc.Bytes())
		got, err := dec.ReadString()
		enc.Release()
		if err != nil {
			t.Fatalf("ReadString(%q) error: %v", v, err)
		}
		if got != v {
			t.Errorf("string round-trip: wrote %q, got %q", v, got)
		}
	}
}

func TestBoolRoundTrip(t *testing.T) {
	for _, v := range []bool{true, false} {
		enc := NewEncoder()
		enc.WriteTagBool(1, v)
		dec := NewDecoder(enc.Bytes())
		fn, wt, err := dec.ReadField()
		if err != nil {
			t.Fatal(err)
		}
		if fn != 1 || wt != WireVarint {
			t.Errorf("tag: field=%d wire=%d", fn, wt)
		}
		got, err := dec.ReadBool()
		if err != nil {
			t.Fatal(err)
		}
		enc.Release()
		if got != v {
			t.Errorf("bool round-trip: wrote %v, got %v", v, got)
		}
	}
}

func TestTaggedFieldsRoundTrip(t *testing.T) {
	enc := NewEncoder()
	enc.WriteTagVarint(1, 42)
	enc.WriteTagSignedVarint(2, -100)
	enc.WriteTagZigzag(3, -200)
	enc.WriteTagFixed32(4, 12345)
	enc.WriteTagFixed64(5, 67890)
	enc.WriteTagFloat(6, 3.14)
	enc.WriteTagDouble(7, 2.718)
	enc.WriteTagBytes(8, []byte{0xde, 0xad})
	enc.WriteTagString(9, "hello")
	enc.WriteTagBool(10, true)

	dec := NewDecoder(enc.Bytes())
	enc.Release()

	// Field 1: varint
	fn, wt, err := dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(1))
	assertEqual(t, wt, WireVarint)
	v, err := dec.ReadVarint()
	assertNoErr(t, err)
	assertEqual(t, v, uint64(42))

	// Field 2: signed varint
	fn, _, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(2))
	sv, err := dec.ReadSignedVarint()
	assertNoErr(t, err)
	assertEqual(t, sv, int64(-100))

	// Field 3: zigzag
	fn, _, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(3))
	zv, err := dec.ReadZigzag()
	assertNoErr(t, err)
	assertEqual(t, zv, int64(-200))

	// Field 4: fixed32
	fn, wt, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(4))
	assertEqual(t, wt, Wire32Bit)
	f32, err := dec.ReadFixed32()
	assertNoErr(t, err)
	assertEqual(t, f32, uint32(12345))

	// Field 5: fixed64
	fn, wt, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(5))
	assertEqual(t, wt, Wire64Bit)
	f64, err := dec.ReadFixed64()
	assertNoErr(t, err)
	assertEqual(t, f64, uint64(67890))

	// Field 6: float
	fn, _, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(6))
	fl, err := dec.ReadFloat()
	assertNoErr(t, err)
	if math.Abs(float64(fl)-3.14) > 0.01 {
		t.Errorf("float = %f, want ~3.14", fl)
	}

	// Field 7: double
	fn, _, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(7))
	dbl, err := dec.ReadDouble()
	assertNoErr(t, err)
	if math.Abs(dbl-2.718) > 0.001 {
		t.Errorf("double = %f, want ~2.718", dbl)
	}

	// Field 8: bytes
	fn, _, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(8))
	bs, err := dec.ReadBytes()
	assertNoErr(t, err)
	if len(bs) != 2 || bs[0] != 0xde || bs[1] != 0xad {
		t.Errorf("bytes = %x", bs)
	}

	// Field 9: string
	fn, _, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(9))
	str, err := dec.ReadString()
	assertNoErr(t, err)
	assertEqual(t, str, "hello")

	// Field 10: bool
	fn, _, err = dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(10))
	bv, err := dec.ReadBool()
	assertNoErr(t, err)
	if !bv {
		t.Error("expected true")
	}

	if !dec.Done() {
		t.Errorf("expected done, remaining=%d", dec.Remaining())
	}
}

func TestNestedMessage(t *testing.T) {
	enc := NewEncoder()
	enc.EncodeMessage(1, func(inner *Encoder) {
		inner.WriteTagVarint(1, 42)
		inner.WriteTagString(2, "nested")
	})

	dec := NewDecoder(enc.Bytes())
	enc.Release()

	fn, wt, err := dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(1))
	assertEqual(t, wt, WireBytes)

	sub, err := dec.ReadMessage()
	assertNoErr(t, err)

	fn, _, err = sub.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(1))
	v, err := sub.ReadVarint()
	assertNoErr(t, err)
	assertEqual(t, v, uint64(42))

	fn, _, err = sub.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(2))
	s, err := sub.ReadString()
	assertNoErr(t, err)
	assertEqual(t, s, "nested")
}

func TestPackedVarints(t *testing.T) {
	enc := NewEncoder()
	enc.EncodePackedVarints(1, []uint64{1, 2, 3, 128, 256})

	dec := NewDecoder(enc.Bytes())
	enc.Release()

	fn, wt, err := dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(1))
	assertEqual(t, wt, WireBytes)

	values, err := dec.ReadPackedVarints()
	assertNoErr(t, err)
	expected := []uint64{1, 2, 3, 128, 256}
	if len(values) != len(expected) {
		t.Fatalf("packed varints: got %d values, want %d", len(values), len(expected))
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("packed varint[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestPackedFixed32(t *testing.T) {
	enc := NewEncoder()
	enc.EncodePackedFixed32(1, []uint32{10, 20, 30})

	dec := NewDecoder(enc.Bytes())
	enc.Release()

	dec.ReadField()
	values, err := dec.ReadPackedFixed32()
	assertNoErr(t, err)
	if len(values) != 3 || values[0] != 10 || values[1] != 20 || values[2] != 30 {
		t.Errorf("packed fixed32 = %v", values)
	}
}

func TestPackedFixed64(t *testing.T) {
	enc := NewEncoder()
	enc.EncodePackedFixed64(1, []uint64{100, 200, 300})

	dec := NewDecoder(enc.Bytes())
	enc.Release()

	dec.ReadField()
	values, err := dec.ReadPackedFixed64()
	assertNoErr(t, err)
	if len(values) != 3 || values[0] != 100 {
		t.Errorf("packed fixed64 = %v", values)
	}
}

func TestPackedEmpty(t *testing.T) {
	enc := NewEncoder()
	enc.EncodePackedVarints(1, nil)
	enc.EncodePackedFixed32(2, nil)
	enc.EncodePackedFixed64(3, nil)
	if enc.Len() != 0 {
		t.Error("empty packed should produce no output")
	}
	enc.Release()
}

func TestSkipField(t *testing.T) {
	enc := NewEncoder()
	enc.WriteTagVarint(1, 42)
	enc.WriteTagFixed64(2, 12345)
	enc.WriteTagBytes(3, []byte("skip me"))
	enc.WriteTagFixed32(4, 99)
	enc.WriteTagString(5, "last")

	dec := NewDecoder(enc.Bytes())
	enc.Release()

	// Skip fields 1-4, read field 5
	for i := 0; i < 4; i++ {
		_, wt, err := dec.ReadField()
		assertNoErr(t, err)
		err = dec.SkipField(wt)
		assertNoErr(t, err)
	}

	fn, _, err := dec.ReadField()
	assertNoErr(t, err)
	assertEqual(t, fn, uint32(5))
	s, err := dec.ReadString()
	assertNoErr(t, err)
	assertEqual(t, s, "last")
}

func TestSkipFieldInvalidWireType(t *testing.T) {
	err := NewDecoder(nil).SkipField(WireType(7))
	if err == nil {
		t.Error("expected error for invalid wire type")
	}
}

func TestDecoderUnderflow(t *testing.T) {
	dec := NewDecoder(nil)
	if _, err := dec.ReadVarint(); err != ErrBufferUnderflow {
		t.Error("expected underflow on empty")
	}
	if _, err := dec.ReadFixed32(); err != ErrBufferUnderflow {
		t.Error("expected underflow for fixed32")
	}
	if _, err := dec.ReadFixed64(); err != ErrBufferUnderflow {
		t.Error("expected underflow for fixed64")
	}
	if _, err := dec.ReadFloat(); err != ErrBufferUnderflow {
		t.Error("expected underflow for float")
	}
	if _, err := dec.ReadDouble(); err != ErrBufferUnderflow {
		t.Error("expected underflow for double")
	}
	if _, err := dec.ReadBytes(); err != ErrBufferUnderflow {
		t.Error("expected underflow for bytes")
	}
	if _, err := dec.ReadString(); err != ErrBufferUnderflow {
		t.Error("expected underflow for string")
	}
	if _, err := dec.ReadBool(); err != ErrBufferUnderflow {
		t.Error("expected underflow for bool")
	}
}

func TestDecoderBytesUnderflow(t *testing.T) {
	// Length says 100 bytes but only 2 available
	enc := NewEncoder()
	enc.WriteVarint(100)
	enc.WriteFixed32(0) // only 4 bytes, not 100
	dec := NewDecoder(enc.Bytes())
	enc.Release()
	_, err := dec.ReadBytes()
	if err != ErrBufferUnderflow {
		t.Errorf("expected underflow, got %v", err)
	}
}

func TestSkipFieldUnderflow(t *testing.T) {
	dec := NewDecoder(nil)
	if err := dec.SkipField(WireVarint); err != ErrBufferUnderflow {
		t.Error("skip varint underflow")
	}
	if err := dec.SkipField(Wire64Bit); err != ErrBufferUnderflow {
		t.Error("skip 64-bit underflow")
	}
	if err := dec.SkipField(Wire32Bit); err != ErrBufferUnderflow {
		t.Error("skip 32-bit underflow")
	}
	if err := dec.SkipField(WireBytes); err != ErrBufferUnderflow {
		t.Error("skip bytes underflow")
	}
}

func TestSkipBytesFieldLengthUnderflow(t *testing.T) {
	// Valid length varint but not enough data
	enc := NewEncoder()
	enc.WriteVarint(100) // says 100 bytes follow
	dec := NewDecoder(enc.Bytes())
	enc.Release()
	err := dec.SkipField(WireBytes)
	if err != ErrBufferUnderflow {
		t.Errorf("expected underflow, got %v", err)
	}
}

func TestVarintOverflow(t *testing.T) {
	// 11 bytes of 0x80 (continuation) — exceeds max 10 bytes
	buf := make([]byte, 11)
	for i := range buf {
		buf[i] = 0x80
	}
	dec := NewDecoder(buf)
	_, err := dec.ReadVarint()
	if err != ErrVarintOverflow {
		t.Errorf("expected varint overflow, got %v", err)
	}
}

func TestEncoderReset(t *testing.T) {
	enc := NewEncoder()
	enc.WriteVarint(42)
	if enc.Len() == 0 {
		t.Error("expected non-zero length")
	}
	enc.Reset()
	if enc.Len() != 0 {
		t.Error("expected zero length after reset")
	}
	enc.Release()
}

func TestEncoderWriteRaw(t *testing.T) {
	enc := NewEncoder()
	enc.WriteRaw([]byte{0x01, 0x02, 0x03})
	if enc.Len() != 3 {
		t.Errorf("WriteRaw: len = %d, want 3", enc.Len())
	}
	enc.Release()
}

func TestDecoderRemaining(t *testing.T) {
	dec := NewDecoder([]byte{0x01, 0x02, 0x03})
	if dec.Remaining() != 3 {
		t.Errorf("Remaining() = %d, want 3", dec.Remaining())
	}
	dec.ReadVarint()
	if dec.Remaining() != 2 {
		t.Errorf("Remaining() = %d, want 2", dec.Remaining())
	}
}

func TestReadSignedVarintError(t *testing.T) {
	dec := NewDecoder(nil)
	_, err := dec.ReadSignedVarint()
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadZigzagError(t *testing.T) {
	dec := NewDecoder(nil)
	_, err := dec.ReadZigzag()
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadFieldError(t *testing.T) {
	dec := NewDecoder(nil)
	_, _, err := dec.ReadField()
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadMessageError(t *testing.T) {
	dec := NewDecoder(nil)
	_, err := dec.ReadMessage()
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadPackedVarintsError(t *testing.T) {
	dec := NewDecoder(nil)
	_, err := dec.ReadPackedVarints()
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadPackedVarintsInnerError(t *testing.T) {
	// Create a packed field with a truncated varint inside
	enc := NewEncoder()
	enc.WriteVarint(2)               // length = 2
	enc.WriteRaw([]byte{0x80, 0x80}) // two continuation bytes, no terminator (but exactly 2 bytes)
	dec := NewDecoder(enc.Bytes())
	enc.Release()
	// This will read the sub-message (2 bytes), then try to decode varints inside
	_, err := dec.ReadPackedVarints()
	// The inner decoder reads 0x80, 0x80 — two continuation bytes, then hits EOF
	if err == nil {
		t.Error("expected error for malformed inner varint")
	}
}

func TestReadPackedFixed32Error(t *testing.T) {
	dec := NewDecoder(nil)
	_, err := dec.ReadPackedFixed32()
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadPackedFixed32InnerError(t *testing.T) {
	// Packed field with 3 bytes (not divisible by 4)
	enc := NewEncoder()
	enc.WriteVarint(3)
	enc.WriteRaw([]byte{0x01, 0x02, 0x03})
	dec := NewDecoder(enc.Bytes())
	enc.Release()
	_, err := dec.ReadPackedFixed32()
	if err == nil {
		t.Error("expected error for truncated fixed32")
	}
}

func TestReadPackedFixed64Error(t *testing.T) {
	dec := NewDecoder(nil)
	_, err := dec.ReadPackedFixed64()
	if err == nil {
		t.Error("expected error")
	}
}

func TestReadPackedFixed64InnerError(t *testing.T) {
	enc := NewEncoder()
	enc.WriteVarint(3)
	enc.WriteRaw([]byte{0x01, 0x02, 0x03})
	dec := NewDecoder(enc.Bytes())
	enc.Release()
	_, err := dec.ReadPackedFixed64()
	if err == nil {
		t.Error("expected error for truncated fixed64")
	}
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
