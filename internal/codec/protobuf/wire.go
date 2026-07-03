package protobuf

// WireType identifies the protobuf wire type.
type WireType uint8

const (
	// WireVarint is wire type 0 (int32, int64, uint32, uint64, sint32, sint64, bool, enum).
	WireVarint WireType = 0
	// Wire64Bit is wire type 1 (fixed64, sfixed64, double).
	Wire64Bit WireType = 1
	// WireBytes is wire type 2 (string, bytes, embedded messages, packed repeated fields).
	WireBytes WireType = 2
	// Wire32Bit is wire type 5 (fixed32, sfixed32, float).
	Wire32Bit WireType = 5
)

// String returns the wire type name.
func (w WireType) String() string {
	switch w {
	case WireVarint:
		return "varint"
	case Wire64Bit:
		return "64-bit"
	case WireBytes:
		return "bytes"
	case Wire32Bit:
		return "32-bit"
	default:
		return "unknown"
	}
}

// MakeTag creates a protobuf field tag from field number and wire type.
func MakeTag(fieldNumber uint32, wireType WireType) uint64 {
	return uint64(fieldNumber)<<3 | uint64(wireType)
}

// ParseTag extracts field number and wire type from a tag.
func ParseTag(tag uint64) (fieldNumber uint32, wireType WireType) {
	return uint32(tag >> 3), WireType(tag & 0x07)
}
