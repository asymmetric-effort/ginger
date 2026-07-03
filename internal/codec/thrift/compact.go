package thrift

import "math"

// CompactEncoder writes Thrift compact protocol.
type CompactEncoder struct {
	buf       []byte
	lastField []int16 // stack of last field IDs for nested structs
}

// NewCompactEncoder creates a new CompactEncoder.
func NewCompactEncoder() *CompactEncoder {
	return &CompactEncoder{buf: make([]byte, 0, 256), lastField: []int16{0}}
}

// Reset clears the encoder.
func (e *CompactEncoder) Reset() {
	e.buf = e.buf[:0]
	e.lastField = e.lastField[:1]
	e.lastField[0] = 0
}

// Bytes returns the encoded bytes.
func (e *CompactEncoder) Bytes() []byte { return e.buf }

// WriteBool writes a bool.
func (e *CompactEncoder) WriteBool(v bool) {
	if v {
		e.buf = append(e.buf, 1)
	} else {
		e.buf = append(e.buf, 0)
	}
}

// WriteByte writes a single byte.
func (e *CompactEncoder) WriteByte(v byte) {
	e.buf = append(e.buf, v)
}

// WriteI16 writes a zigzag-varint int16.
func (e *CompactEncoder) WriteI16(v int16) {
	e.writeZigzag(int64(v))
}

// WriteI32 writes a zigzag-varint int32.
func (e *CompactEncoder) WriteI32(v int32) {
	e.writeZigzag(int64(v))
}

// WriteI64 writes a zigzag-varint int64.
func (e *CompactEncoder) WriteI64(v int64) {
	e.writeZigzag(v)
}

// WriteDouble writes a float64 as little-endian IEEE 754.
func (e *CompactEncoder) WriteDouble(v float64) {
	bits := math.Float64bits(v)
	e.buf = append(e.buf,
		byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24),
		byte(bits>>32), byte(bits>>40), byte(bits>>48), byte(bits>>56),
	)
}

// WriteString writes a varint-length-prefixed string.
func (e *CompactEncoder) WriteString(v string) {
	e.writeVarint(uint64(len(v)))
	e.buf = append(e.buf, v...)
}

// WriteBytes writes a varint-length-prefixed byte slice.
func (e *CompactEncoder) WriteBytes(v []byte) {
	e.writeVarint(uint64(len(v)))
	e.buf = append(e.buf, v...)
}

// WriteFieldBegin writes a field header using delta encoding.
func (e *CompactEncoder) WriteFieldBegin(fieldType TType, fieldID int16) {
	ct := tTypeToCompact(fieldType)
	delta := fieldID - e.lastField[len(e.lastField)-1]
	if delta > 0 && delta <= 15 {
		e.buf = append(e.buf, byte(delta<<4)|ct)
	} else {
		e.buf = append(e.buf, ct)
		e.WriteI16(fieldID)
	}
	e.lastField[len(e.lastField)-1] = fieldID
}

// WriteFieldBeginBool writes a bool field with the value embedded in the type.
func (e *CompactEncoder) WriteFieldBeginBool(fieldID int16, v bool) {
	ct := compactBoolFalse
	if v {
		ct = compactBoolTrue
	}
	delta := fieldID - e.lastField[len(e.lastField)-1]
	if delta > 0 && delta <= 15 {
		e.buf = append(e.buf, byte(delta<<4)|ct)
	} else {
		e.buf = append(e.buf, ct)
		e.WriteI16(fieldID)
	}
	e.lastField[len(e.lastField)-1] = fieldID
}

// WriteFieldStop writes the field stop marker.
func (e *CompactEncoder) WriteFieldStop() {
	e.buf = append(e.buf, 0)
}

// WriteListBegin writes a list header.
func (e *CompactEncoder) WriteListBegin(elemType TType, size int32) {
	ct := tTypeToCompact(elemType)
	if size <= 14 {
		e.buf = append(e.buf, byte(size<<4)|ct)
	} else {
		e.buf = append(e.buf, 0xf0|ct)
		e.writeVarint(uint64(size))
	}
}

// WriteMapBegin writes a map header.
func (e *CompactEncoder) WriteMapBegin(keyType, valueType TType, size int32) {
	if size == 0 {
		e.buf = append(e.buf, 0)
		return
	}
	e.writeVarint(uint64(size))
	e.buf = append(e.buf, tTypeToCompact(keyType)<<4|tTypeToCompact(valueType))
}

// WriteStructBegin pushes a new field ID context.
func (e *CompactEncoder) WriteStructBegin() {
	e.lastField = append(e.lastField, 0)
}

// WriteStructEnd pops the field ID context.
func (e *CompactEncoder) WriteStructEnd() {
	e.lastField = e.lastField[:len(e.lastField)-1]
}

func (e *CompactEncoder) writeVarint(v uint64) {
	for v >= 0x80 {
		e.buf = append(e.buf, byte(v)|0x80)
		v >>= 7
	}
	e.buf = append(e.buf, byte(v))
}

func (e *CompactEncoder) writeZigzag(v int64) {
	e.writeVarint(uint64(v<<1) ^ uint64(v>>63))
}

// CompactDecoder reads Thrift compact protocol.
type CompactDecoder struct {
	buf       []byte
	pos       int
	lastField []int16
	boolValue int8 // -1: no pending bool, 0: false, 1: true
}

// NewCompactDecoder creates a decoder over the given bytes.
func NewCompactDecoder(buf []byte) *CompactDecoder {
	return &CompactDecoder{buf: buf, lastField: []int16{0}, boolValue: -1}
}

// Remaining returns unread bytes count.
func (d *CompactDecoder) Remaining() int { return len(d.buf) - d.pos }

// Done reports whether all bytes consumed.
func (d *CompactDecoder) Done() bool { return d.pos >= len(d.buf) }

// ReadBool reads a bool (or uses the pending bool value from a field header).
func (d *CompactDecoder) ReadBool() (bool, error) {
	if d.boolValue >= 0 {
		v := d.boolValue == 1
		d.boolValue = -1
		return v, nil
	}
	if d.pos >= len(d.buf) {
		return false, ErrUnderflow
	}
	v := d.buf[d.pos] != 0
	d.pos++
	return v, nil
}

// ReadByte reads a single byte.
func (d *CompactDecoder) ReadByte() (byte, error) {
	if d.pos >= len(d.buf) {
		return 0, ErrUnderflow
	}
	v := d.buf[d.pos]
	d.pos++
	return v, nil
}

// ReadI16 reads a zigzag-varint int16.
func (d *CompactDecoder) ReadI16() (int16, error) {
	v, err := d.readZigzag()
	return int16(v), err
}

// ReadI32 reads a zigzag-varint int32.
func (d *CompactDecoder) ReadI32() (int32, error) {
	v, err := d.readZigzag()
	return int32(v), err
}

// ReadI64 reads a zigzag-varint int64.
func (d *CompactDecoder) ReadI64() (int64, error) {
	return d.readZigzag()
}

// ReadDouble reads a little-endian IEEE 754 float64.
func (d *CompactDecoder) ReadDouble() (float64, error) {
	if d.pos+8 > len(d.buf) {
		return 0, ErrUnderflow
	}
	bits := uint64(d.buf[d.pos]) | uint64(d.buf[d.pos+1])<<8 |
		uint64(d.buf[d.pos+2])<<16 | uint64(d.buf[d.pos+3])<<24 |
		uint64(d.buf[d.pos+4])<<32 | uint64(d.buf[d.pos+5])<<40 |
		uint64(d.buf[d.pos+6])<<48 | uint64(d.buf[d.pos+7])<<56
	d.pos += 8
	return math.Float64frombits(bits), nil
}

// ReadString reads a varint-length-prefixed string.
func (d *CompactDecoder) ReadString() (string, error) {
	length, err := d.readVarint()
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

// ReadBytes reads a varint-length-prefixed byte slice.
func (d *CompactDecoder) ReadBytes() ([]byte, error) {
	length, err := d.readVarint()
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

// ReadFieldBegin reads a field header.
func (d *CompactDecoder) ReadFieldBegin() (fieldType TType, fieldID int16, err error) {
	typeByte, err := d.ReadByte()
	if err != nil {
		return TypeStop, 0, err
	}
	if typeByte == 0 {
		return TypeStop, 0, nil
	}

	compactType := typeByte & 0x0f
	delta := int16(typeByte >> 4)

	if delta != 0 {
		fieldID = d.lastField[len(d.lastField)-1] + delta
	} else {
		fieldID, err = d.ReadI16()
		if err != nil {
			return TypeStop, 0, err
		}
	}
	d.lastField[len(d.lastField)-1] = fieldID

	fieldType = compactTypeToTType(compactType)
	if compactType == compactBoolTrue {
		d.boolValue = 1
	} else if compactType == compactBoolFalse {
		d.boolValue = 0
	}

	return fieldType, fieldID, nil
}

// ReadListBegin reads a list header.
func (d *CompactDecoder) ReadListBegin() (elemType TType, size int32, err error) {
	header, err := d.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	size = int32(header >> 4)
	if size == 15 {
		v, err := d.readVarint()
		if err != nil {
			return 0, 0, err
		}
		size = int32(v)
	}
	return compactTypeToTType(header & 0x0f), size, nil
}

// ReadMapBegin reads a map header.
func (d *CompactDecoder) ReadMapBegin() (keyType, valueType TType, size int32, err error) {
	v, err := d.readVarint()
	if err != nil {
		return 0, 0, 0, err
	}
	size = int32(v)
	if size == 0 {
		return TypeStop, TypeStop, 0, nil
	}
	types, err := d.ReadByte()
	if err != nil {
		return 0, 0, 0, err
	}
	return compactTypeToTType(types >> 4), compactTypeToTType(types & 0x0f), size, nil
}

// ReadStructBegin pushes a new field ID context.
func (d *CompactDecoder) ReadStructBegin() {
	d.lastField = append(d.lastField, 0)
}

// ReadStructEnd pops the field ID context.
func (d *CompactDecoder) ReadStructEnd() {
	d.lastField = d.lastField[:len(d.lastField)-1]
}

func (d *CompactDecoder) readVarint() (uint64, error) {
	var val uint64
	var shift uint
	for i := 0; i < 10; i++ {
		if d.pos >= len(d.buf) {
			return 0, ErrUnderflow
		}
		b := d.buf[d.pos]
		d.pos++
		val |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return val, nil
		}
		shift += 7
	}
	return 0, ErrUnderflow
}

func (d *CompactDecoder) readZigzag() (int64, error) {
	v, err := d.readVarint()
	if err != nil {
		return 0, err
	}
	return int64(v>>1) ^ -int64(v&1), nil
}
