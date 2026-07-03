package protobuf

import (
	"math"
	"sync"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 256)
		return &b
	},
}

// Encoder writes protobuf binary format.
type Encoder struct {
	buf []byte
}

// NewEncoder creates a new Encoder.
func NewEncoder() *Encoder {
	bp := bufPool.Get().(*[]byte)
	b := (*bp)[:0]
	return &Encoder{buf: b}
}

// Reset resets the encoder for reuse.
func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
}

// Bytes returns the encoded bytes. The returned slice is valid until
// the next call to Reset or Release.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// Release returns the buffer to the pool.
func (e *Encoder) Release() {
	bufPool.Put(&e.buf)
}

// WriteVarint writes a varint-encoded uint64.
func (e *Encoder) WriteVarint(v uint64) {
	for v >= 0x80 {
		e.buf = append(e.buf, byte(v)|0x80)
		v >>= 7
	}
	e.buf = append(e.buf, byte(v))
}

// WriteSignedVarint writes a signed int64 as a varint (two's complement).
func (e *Encoder) WriteSignedVarint(v int64) {
	e.WriteVarint(uint64(v))
}

// WriteZigzag writes a zigzag-encoded signed int64.
func (e *Encoder) WriteZigzag(v int64) {
	e.WriteVarint(uint64(v<<1) ^ uint64(v>>63))
}

// WriteFixed32 writes a 4-byte little-endian value.
func (e *Encoder) WriteFixed32(v uint32) {
	e.buf = append(e.buf,
		byte(v),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
	)
}

// WriteFixed64 writes an 8-byte little-endian value.
func (e *Encoder) WriteFixed64(v uint64) {
	e.buf = append(e.buf,
		byte(v),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
		byte(v>>32),
		byte(v>>40),
		byte(v>>48),
		byte(v>>56),
	)
}

// WriteFloat writes a float32.
func (e *Encoder) WriteFloat(v float32) {
	e.WriteFixed32(math.Float32bits(v))
}

// WriteDouble writes a float64.
func (e *Encoder) WriteDouble(v float64) {
	e.WriteFixed64(math.Float64bits(v))
}

// WriteBytes writes a length-delimited byte slice.
func (e *Encoder) WriteBytes(b []byte) {
	e.WriteVarint(uint64(len(b)))
	e.buf = append(e.buf, b...)
}

// WriteString writes a length-delimited string.
func (e *Encoder) WriteString(s string) {
	e.WriteVarint(uint64(len(s)))
	e.buf = append(e.buf, s...)
}

// WriteTag writes a field tag (field number + wire type).
func (e *Encoder) WriteTag(fieldNumber uint32, wireType WireType) {
	e.WriteVarint(MakeTag(fieldNumber, wireType))
}

// WriteTagVarint writes a tag + varint field.
func (e *Encoder) WriteTagVarint(fieldNumber uint32, v uint64) {
	e.WriteTag(fieldNumber, WireVarint)
	e.WriteVarint(v)
}

// WriteTagSignedVarint writes a tag + signed varint field.
func (e *Encoder) WriteTagSignedVarint(fieldNumber uint32, v int64) {
	e.WriteTag(fieldNumber, WireVarint)
	e.WriteSignedVarint(v)
}

// WriteTagZigzag writes a tag + zigzag-encoded field.
func (e *Encoder) WriteTagZigzag(fieldNumber uint32, v int64) {
	e.WriteTag(fieldNumber, WireVarint)
	e.WriteZigzag(v)
}

// WriteTagFixed32 writes a tag + fixed32 field.
func (e *Encoder) WriteTagFixed32(fieldNumber uint32, v uint32) {
	e.WriteTag(fieldNumber, Wire32Bit)
	e.WriteFixed32(v)
}

// WriteTagFixed64 writes a tag + fixed64 field.
func (e *Encoder) WriteTagFixed64(fieldNumber uint32, v uint64) {
	e.WriteTag(fieldNumber, Wire64Bit)
	e.WriteFixed64(v)
}

// WriteTagFloat writes a tag + float field.
func (e *Encoder) WriteTagFloat(fieldNumber uint32, v float32) {
	e.WriteTag(fieldNumber, Wire32Bit)
	e.WriteFloat(v)
}

// WriteTagDouble writes a tag + double field.
func (e *Encoder) WriteTagDouble(fieldNumber uint32, v float64) {
	e.WriteTag(fieldNumber, Wire64Bit)
	e.WriteDouble(v)
}

// WriteTagBytes writes a tag + length-delimited bytes field.
func (e *Encoder) WriteTagBytes(fieldNumber uint32, b []byte) {
	e.WriteTag(fieldNumber, WireBytes)
	e.WriteBytes(b)
}

// WriteTagString writes a tag + length-delimited string field.
func (e *Encoder) WriteTagString(fieldNumber uint32, s string) {
	e.WriteTag(fieldNumber, WireBytes)
	e.WriteString(s)
}

// WriteTagBool writes a tag + bool field.
func (e *Encoder) WriteTagBool(fieldNumber uint32, v bool) {
	val := uint64(0)
	if v {
		val = 1
	}
	e.WriteTagVarint(fieldNumber, val)
}

// WriteRaw appends raw bytes without encoding.
func (e *Encoder) WriteRaw(b []byte) {
	e.buf = append(e.buf, b...)
}

// Len returns the current length of the encoded data.
func (e *Encoder) Len() int {
	return len(e.buf)
}

// EncodeMessage encodes a nested message. It uses a temporary encoder
// to encode the message, then writes the length-delimited result.
func (e *Encoder) EncodeMessage(fieldNumber uint32, fn func(enc *Encoder)) {
	inner := NewEncoder()
	fn(inner)
	e.WriteTagBytes(fieldNumber, inner.Bytes())
	inner.Release()
}

// EncodePackedVarints writes a packed repeated varint field.
func (e *Encoder) EncodePackedVarints(fieldNumber uint32, values []uint64) {
	if len(values) == 0 {
		return
	}
	inner := NewEncoder()
	for _, v := range values {
		inner.WriteVarint(v)
	}
	e.WriteTagBytes(fieldNumber, inner.Bytes())
	inner.Release()
}

// EncodePackedFixed32 writes a packed repeated fixed32 field.
func (e *Encoder) EncodePackedFixed32(fieldNumber uint32, values []uint32) {
	if len(values) == 0 {
		return
	}
	inner := NewEncoder()
	for _, v := range values {
		inner.WriteFixed32(v)
	}
	e.WriteTagBytes(fieldNumber, inner.Bytes())
	inner.Release()
}

// EncodePackedFixed64 writes a packed repeated fixed64 field.
func (e *Encoder) EncodePackedFixed64(fieldNumber uint32, values []uint64) {
	if len(values) == 0 {
		return
	}
	inner := NewEncoder()
	for _, v := range values {
		inner.WriteFixed64(v)
	}
	e.WriteTagBytes(fieldNumber, inner.Bytes())
	inner.Release()
}
