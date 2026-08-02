package abr

import "errors"

var (
	// ErrMalformed identifies structurally invalid or truncated input.
	ErrMalformed = errors.New("malformed ABR data")
	// ErrUnsupported identifies a well-formed feature outside the compatibility matrix.
	ErrUnsupported = errors.New("unsupported ABR feature")
	// ErrLimit identifies input rejected by a configured resource limit.
	ErrLimit = errors.New("ABR resource limit exceeded")
)

// Limits caps parser work and allocations. Zero-valued fields are replaced by
// the corresponding values from DefaultLimits.
type Limits struct {
	MaxFileBytes       int
	MaxSectionBytes    int
	MaxRecords         int
	MaxDimension       int
	MaxPixels          uint64
	MaxDescriptorDepth int
	MaxDescriptorItems int
	MaxStringBytes     int
	MaxDataBytes       int
}

// DefaultLimits returns conservative limits suitable for untrusted imports.
func DefaultLimits() Limits {
	return Limits{
		MaxFileBytes:       128 << 20,
		MaxSectionBytes:    96 << 20,
		MaxRecords:         4096,
		MaxDimension:       16384,
		MaxPixels:          64 << 20,
		MaxDescriptorDepth: 32,
		MaxDescriptorItems: 100_000,
		MaxStringBytes:     1 << 20,
		MaxDataBytes:       32 << 20,
	}
}

// Library is returned only after every recognized section has parsed
// successfully. Unknown tagged sections are retained by key but not copied.
type Library struct {
	Version     uint16
	Subversion  uint16
	Sampled     []SampledBrush
	Descriptors []Descriptor
	Sections    []Section
}

// Section identifies a top-level 8BIM block.
type Section struct {
	Key    string
	Length uint32
}

// SampledBrush is an 8-bit sampled tip. Pixels are stored row-major exactly as
// encoded by the ABR file; consumers decide whether the values represent
// grayscale or inverse alpha.
type SampledBrush struct {
	Key         string
	Top         int32
	Left        int32
	Width       int
	Height      int
	Depth       uint16
	Compression uint8
	Pixels      []byte
}

// Descriptor is an Adobe Action Descriptor. It is the representation used by
// v6/v7 ABR files for computed brushes and preset metadata.
type Descriptor struct {
	Name    string
	ClassID string
	Items   []Item
}

// Item is a named value in a Descriptor.
type Item struct {
	Key   string
	Value Value
}

// Value is a typed Action Descriptor value. Only the field corresponding to
// Type is populated.
type Value struct {
	Type    string
	Bool    bool
	Integer int32
	Float   float64
	String  string
	Unit    string
	Enum    EnumValue
	ClassID string
	Object  *Descriptor
	List    []Value
	Data    []byte
}

// EnumValue stores an Action Descriptor enumeration type and value.
type EnumValue struct {
	Type  string
	Value string
}
