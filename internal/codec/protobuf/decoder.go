package protobuf

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrBufferUnderflow is returned when there is not enough data to decode.
	ErrBufferUnderflow = errors.New("protobuf: buffer underflow")
	// ErrVarintOverflow is returned when a varint exceeds 10 bytes.
	ErrVarintOverflow = errors.New("protobuf: varint overflow")
	// ErrInvalidWireType is returned for an unrecognized wire type.
	ErrInvalidWireType = errors.New("protobuf: invalid wire type")
)

// Decoder reads protobuf binary format.
type Decoder struct {
	buf []byte
	pos int
}

// NewDecoder creates a decoder over the given bytes.
func NewDecoder(buf []byte) *Decoder {
	return &Decoder{buf: buf}
}

// Remaining returns the number of unread bytes.
func (d *Decoder) Remaining() int {
	return len(d.buf) - d.pos
}

// Done reports whether all bytes have been consumed.
func (d *Decoder) Done() bool {
	return d.pos >= len(d.buf)
}

// ReadVarint reads a varint-encoded uint64.
func (d *Decoder) ReadVarint() (uint64, error) {
	var val uint64
	var shift uint
	for i := 0; i < 10; i++ {
		if d.pos >= len(d.buf) {
			return 0, ErrBufferUnderflow
		}
		b := d.buf[d.pos]
		d.pos++
		val |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return val, nil
		}
		shift += 7
	}
	return 0, ErrVarintOverflow
}

// ReadSignedVarint reads a signed varint (two's complement).
func (d *Decoder) ReadSignedVarint() (int64, error) {
	v, err := d.ReadVarint()
	if err != nil {
		return 0, err
	}
	return int64(v), nil
}

// ReadZigzag reads a zigzag-decoded signed int64.
func (d *Decoder) ReadZigzag() (int64, error) {
	v, err := d.ReadVarint()
	if err != nil {
		return 0, err
	}
	return int64(v>>1) ^ -int64(v&1), nil
}

// ReadFixed32 reads a 4-byte little-endian uint32.
func (d *Decoder) ReadFixed32() (uint32, error) {
	if d.pos+4 > len(d.buf) {
		return 0, ErrBufferUnderflow
	}
	v := uint32(d.buf[d.pos]) |
		uint32(d.buf[d.pos+1])<<8 |
		uint32(d.buf[d.pos+2])<<16 |
		uint32(d.buf[d.pos+3])<<24
	d.pos += 4
	return v, nil
}

// ReadFixed64 reads an 8-byte little-endian uint64.
func (d *Decoder) ReadFixed64() (uint64, error) {
	if d.pos+8 > len(d.buf) {
		return 0, ErrBufferUnderflow
	}
	v := uint64(d.buf[d.pos]) |
		uint64(d.buf[d.pos+1])<<8 |
		uint64(d.buf[d.pos+2])<<16 |
		uint64(d.buf[d.pos+3])<<24 |
		uint64(d.buf[d.pos+4])<<32 |
		uint64(d.buf[d.pos+5])<<40 |
		uint64(d.buf[d.pos+6])<<48 |
		uint64(d.buf[d.pos+7])<<56
	d.pos += 8
	return v, nil
}

// ReadFloat reads a float32.
func (d *Decoder) ReadFloat() (float32, error) {
	v, err := d.ReadFixed32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(v), nil
}

// ReadDouble reads a float64.
func (d *Decoder) ReadDouble() (float64, error) {
	v, err := d.ReadFixed64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(v), nil
}

// ReadBytes reads a length-delimited byte slice.
func (d *Decoder) ReadBytes() ([]byte, error) {
	length, err := d.ReadVarint()
	if err != nil {
		return nil, err
	}
	if d.pos+int(length) > len(d.buf) {
		return nil, ErrBufferUnderflow
	}
	b := make([]byte, length)
	copy(b, d.buf[d.pos:d.pos+int(length)])
	d.pos += int(length)
	return b, nil
}

// ReadString reads a length-delimited string.
func (d *Decoder) ReadString() (string, error) {
	b, err := d.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadBool reads a varint and returns it as a bool.
func (d *Decoder) ReadBool() (bool, error) {
	v, err := d.ReadVarint()
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

// ReadField reads a field tag and returns the field number and wire type.
func (d *Decoder) ReadField() (fieldNumber uint32, wireType WireType, err error) {
	tag, err := d.ReadVarint()
	if err != nil {
		return 0, 0, err
	}
	fieldNumber, wireType = ParseTag(tag)
	return fieldNumber, wireType, nil
}

// SkipField skips over a field value based on its wire type.
func (d *Decoder) SkipField(wireType WireType) error {
	switch wireType {
	case WireVarint:
		_, err := d.ReadVarint()
		return err
	case Wire64Bit:
		if d.pos+8 > len(d.buf) {
			return ErrBufferUnderflow
		}
		d.pos += 8
		return nil
	case WireBytes:
		length, err := d.ReadVarint()
		if err != nil {
			return err
		}
		if d.pos+int(length) > len(d.buf) {
			return ErrBufferUnderflow
		}
		d.pos += int(length)
		return nil
	case Wire32Bit:
		if d.pos+4 > len(d.buf) {
			return ErrBufferUnderflow
		}
		d.pos += 4
		return nil
	default:
		return fmt.Errorf("%w: %d", ErrInvalidWireType, wireType)
	}
}

// ReadMessage reads a length-delimited submessage and returns a Decoder over it.
func (d *Decoder) ReadMessage() (*Decoder, error) {
	b, err := d.ReadBytes()
	if err != nil {
		return nil, err
	}
	return NewDecoder(b), nil
}

// ReadPackedVarints reads a packed repeated varint field.
func (d *Decoder) ReadPackedVarints() ([]uint64, error) {
	sub, err := d.ReadMessage()
	if err != nil {
		return nil, err
	}
	var values []uint64
	for !sub.Done() {
		v, err := sub.ReadVarint()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

// ReadPackedFixed32 reads a packed repeated fixed32 field.
func (d *Decoder) ReadPackedFixed32() ([]uint32, error) {
	sub, err := d.ReadMessage()
	if err != nil {
		return nil, err
	}
	var values []uint32
	for !sub.Done() {
		v, err := sub.ReadFixed32()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

// ReadPackedFixed64 reads a packed repeated fixed64 field.
func (d *Decoder) ReadPackedFixed64() ([]uint64, error) {
	sub, err := d.ReadMessage()
	if err != nil {
		return nil, err
	}
	var values []uint64
	for !sub.Done() {
		v, err := sub.ReadFixed64()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}
