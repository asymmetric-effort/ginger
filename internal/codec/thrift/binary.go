package thrift

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	// ErrUnderflow is returned when there is not enough data to decode.
	ErrUnderflow = errors.New("thrift: buffer underflow")
)

// BinaryEncoder writes Thrift binary protocol.
type BinaryEncoder struct {
	buf []byte
}

// NewBinaryEncoder creates a new BinaryEncoder.
func NewBinaryEncoder() *BinaryEncoder {
	return &BinaryEncoder{buf: make([]byte, 0, 256)}
}

// Reset clears the encoder.
func (e *BinaryEncoder) Reset() { e.buf = e.buf[:0] }

// Bytes returns the encoded bytes.
func (e *BinaryEncoder) Bytes() []byte { return e.buf }

// WriteBool writes a bool.
func (e *BinaryEncoder) WriteBool(v bool) {
	if v {
		e.buf = append(e.buf, 1)
	} else {
		e.buf = append(e.buf, 0)
	}
}

// WriteByte writes a single byte.
func (e *BinaryEncoder) WriteByte(v byte) {
	e.buf = append(e.buf, v)
}

// WriteI16 writes a big-endian int16.
func (e *BinaryEncoder) WriteI16(v int16) {
	e.buf = append(e.buf, byte(v>>8), byte(v))
}

// WriteI32 writes a big-endian int32.
func (e *BinaryEncoder) WriteI32(v int32) {
	e.buf = append(e.buf, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// WriteI64 writes a big-endian int64.
func (e *BinaryEncoder) WriteI64(v int64) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	e.buf = append(e.buf, b...)
}

// WriteDouble writes a float64 as big-endian IEEE 754.
func (e *BinaryEncoder) WriteDouble(v float64) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, math.Float64bits(v))
	e.buf = append(e.buf, b...)
}

// WriteString writes a length-prefixed string.
func (e *BinaryEncoder) WriteString(v string) {
	e.WriteI32(int32(len(v)))
	e.buf = append(e.buf, v...)
}

// WriteBytes writes a length-prefixed byte slice.
func (e *BinaryEncoder) WriteBytes(v []byte) {
	e.WriteI32(int32(len(v)))
	e.buf = append(e.buf, v...)
}

// WriteFieldBegin writes a field header.
func (e *BinaryEncoder) WriteFieldBegin(fieldType TType, fieldID int16) {
	e.WriteByte(byte(fieldType))
	e.WriteI16(fieldID)
}

// WriteFieldStop writes the field stop marker.
func (e *BinaryEncoder) WriteFieldStop() {
	e.WriteByte(byte(TypeStop))
}

// WriteListBegin writes a list header.
func (e *BinaryEncoder) WriteListBegin(elemType TType, size int32) {
	e.WriteByte(byte(elemType))
	e.WriteI32(size)
}

// WriteMapBegin writes a map header.
func (e *BinaryEncoder) WriteMapBegin(keyType, valueType TType, size int32) {
	e.WriteByte(byte(keyType))
	e.WriteByte(byte(valueType))
	e.WriteI32(size)
}

// BinaryDecoder reads Thrift binary protocol.
type BinaryDecoder struct {
	buf []byte
	pos int
}

// NewBinaryDecoder creates a decoder over the given bytes.
func NewBinaryDecoder(buf []byte) *BinaryDecoder {
	return &BinaryDecoder{buf: buf}
}

// Remaining returns unread bytes count.
func (d *BinaryDecoder) Remaining() int { return len(d.buf) - d.pos }

// Done reports whether all bytes consumed.
func (d *BinaryDecoder) Done() bool { return d.pos >= len(d.buf) }

// ReadBool reads a bool.
func (d *BinaryDecoder) ReadBool() (bool, error) {
	if d.pos >= len(d.buf) {
		return false, ErrUnderflow
	}
	v := d.buf[d.pos] != 0
	d.pos++
	return v, nil
}

// ReadByte reads a single byte.
func (d *BinaryDecoder) ReadByte() (byte, error) {
	if d.pos >= len(d.buf) {
		return 0, ErrUnderflow
	}
	v := d.buf[d.pos]
	d.pos++
	return v, nil
}

// ReadI16 reads a big-endian int16.
func (d *BinaryDecoder) ReadI16() (int16, error) {
	if d.pos+2 > len(d.buf) {
		return 0, ErrUnderflow
	}
	v := int16(d.buf[d.pos])<<8 | int16(d.buf[d.pos+1])
	d.pos += 2
	return v, nil
}

// ReadI32 reads a big-endian int32.
func (d *BinaryDecoder) ReadI32() (int32, error) {
	if d.pos+4 > len(d.buf) {
		return 0, ErrUnderflow
	}
	v := int32(d.buf[d.pos])<<24 | int32(d.buf[d.pos+1])<<16 |
		int32(d.buf[d.pos+2])<<8 | int32(d.buf[d.pos+3])
	d.pos += 4
	return v, nil
}

// ReadI64 reads a big-endian int64.
func (d *BinaryDecoder) ReadI64() (int64, error) {
	if d.pos+8 > len(d.buf) {
		return 0, ErrUnderflow
	}
	v := binary.BigEndian.Uint64(d.buf[d.pos : d.pos+8])
	d.pos += 8
	return int64(v), nil
}

// ReadDouble reads a big-endian IEEE 754 float64.
func (d *BinaryDecoder) ReadDouble() (float64, error) {
	if d.pos+8 > len(d.buf) {
		return 0, ErrUnderflow
	}
	v := binary.BigEndian.Uint64(d.buf[d.pos : d.pos+8])
	d.pos += 8
	return math.Float64frombits(v), nil
}

// ReadString reads a length-prefixed string.
func (d *BinaryDecoder) ReadString() (string, error) {
	length, err := d.ReadI32()
	if err != nil {
		return "", err
	}
	if d.pos+int(length) > len(d.buf) {
		return "", ErrUnderflow
	}
	s := string(d.buf[d.pos : d.pos+int(length)])
	d.pos += int(length)
	return s, nil
}

// ReadBytes reads a length-prefixed byte slice.
func (d *BinaryDecoder) ReadBytes() ([]byte, error) {
	length, err := d.ReadI32()
	if err != nil {
		return nil, err
	}
	if d.pos+int(length) > len(d.buf) {
		return nil, ErrUnderflow
	}
	b := make([]byte, length)
	copy(b, d.buf[d.pos:d.pos+int(length)])
	d.pos += int(length)
	return b, nil
}

// ReadFieldBegin reads a field header. Returns TypeStop when no more fields.
func (d *BinaryDecoder) ReadFieldBegin() (fieldType TType, fieldID int16, err error) {
	typeByte, err := d.ReadByte()
	if err != nil {
		return TypeStop, 0, err
	}
	fieldType = TType(typeByte)
	if fieldType == TypeStop {
		return TypeStop, 0, nil
	}
	fieldID, err = d.ReadI16()
	if err != nil {
		return TypeStop, 0, err
	}
	return fieldType, fieldID, nil
}

// ReadListBegin reads a list header.
func (d *BinaryDecoder) ReadListBegin() (elemType TType, size int32, err error) {
	typeByte, err := d.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	size, err = d.ReadI32()
	if err != nil {
		return 0, 0, err
	}
	return TType(typeByte), size, nil
}

// ReadMapBegin reads a map header.
func (d *BinaryDecoder) ReadMapBegin() (keyType, valueType TType, size int32, err error) {
	kt, err := d.ReadByte()
	if err != nil {
		return 0, 0, 0, err
	}
	vt, err := d.ReadByte()
	if err != nil {
		return 0, 0, 0, err
	}
	size, err = d.ReadI32()
	if err != nil {
		return 0, 0, 0, err
	}
	return TType(kt), TType(vt), size, nil
}
