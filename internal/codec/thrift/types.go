package thrift

// TType is the Thrift type identifier.
type TType byte

const (
	TypeStop   TType = 0
	TypeBool   TType = 2
	TypeByte   TType = 3
	TypeI16    TType = 6
	TypeI32    TType = 8
	TypeI64    TType = 10
	TypeDouble TType = 4
	TypeString TType = 11
	TypeList   TType = 15
	TypeSet    TType = 14
	TypeMap    TType = 13
	TypeStruct TType = 12
)

// String returns the type name.
func (t TType) String() string {
	switch t {
	case TypeStop:
		return "stop"
	case TypeBool:
		return "bool"
	case TypeByte:
		return "byte"
	case TypeI16:
		return "i16"
	case TypeI32:
		return "i32"
	case TypeI64:
		return "i64"
	case TypeDouble:
		return "double"
	case TypeString:
		return "string"
	case TypeList:
		return "list"
	case TypeSet:
		return "set"
	case TypeMap:
		return "map"
	case TypeStruct:
		return "struct"
	default:
		return "unknown"
	}
}

// Compact protocol type IDs (different from binary protocol).
const (
	compactBoolTrue  byte = 1
	compactBoolFalse byte = 2
	compactByte      byte = 3
	compactI16       byte = 4
	compactI32       byte = 5
	compactI64       byte = 6
	compactDouble    byte = 7
	compactBinary    byte = 8
	compactList      byte = 9
	compactSet       byte = 10
	compactMap       byte = 11
	compactStruct    byte = 12
)

// compactTypeToTType converts a compact type ID to a TType.
func compactTypeToTType(ct byte) TType {
	switch ct {
	case compactBoolTrue, compactBoolFalse:
		return TypeBool
	case compactByte:
		return TypeByte
	case compactI16:
		return TypeI16
	case compactI32:
		return TypeI32
	case compactI64:
		return TypeI64
	case compactDouble:
		return TypeDouble
	case compactBinary:
		return TypeString
	case compactList:
		return TypeList
	case compactSet:
		return TypeSet
	case compactMap:
		return TypeMap
	case compactStruct:
		return TypeStruct
	default:
		return TypeStop
	}
}

// tTypeToCompact converts a TType to compact protocol type ID.
func tTypeToCompact(t TType) byte {
	switch t {
	case TypeBool:
		return compactBoolTrue // default; actual value handled separately
	case TypeByte:
		return compactByte
	case TypeI16:
		return compactI16
	case TypeI32:
		return compactI32
	case TypeI64:
		return compactI64
	case TypeDouble:
		return compactDouble
	case TypeString:
		return compactBinary
	case TypeList:
		return compactList
	case TypeSet:
		return compactSet
	case TypeMap:
		return compactMap
	case TypeStruct:
		return compactStruct
	default:
		return 0
	}
}
