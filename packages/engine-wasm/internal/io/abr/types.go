package abr

import (
	"errors"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/descriptor"
)

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

// Descriptor, Item, Value and EnumValue are the shared Action Descriptor model
// from internal/io/descriptor. They are aliases rather than distinct types so
// that ABR callers keep working with the same values the PSD side parses, and
// so that moving the parser out did not become a churn of every consumer.
type (
	Descriptor = descriptor.Descriptor
	Item       = descriptor.Item
	Value      = descriptor.Value
	EnumValue  = descriptor.EnumValue
)
