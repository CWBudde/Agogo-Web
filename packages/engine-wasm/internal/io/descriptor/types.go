package descriptor

import (
	"errors"
	"strings"
)

var (
	// ErrMalformed identifies structurally invalid or truncated input.
	ErrMalformed = errors.New("malformed descriptor data")
	// ErrUnsupported identifies a well-formed value type this package does not
	// implement. See the package doc for why these are errors and not skips.
	ErrUnsupported = errors.New("unsupported descriptor feature")
	// ErrLimit identifies input rejected by a configured resource limit.
	ErrLimit = errors.New("descriptor resource limit exceeded")
)

// Value type 4CCs, so call sites compare against a constant rather than a
// literal that a typo would silently turn into a never-matching branch.
const (
	TypeObject       = "Objc"
	TypeGlobalObject = "GlbO"
	TypeList         = "VlLs"
	TypeDouble       = "doub"
	TypeUnitFloat    = "UntF"
	TypeText         = "TEXT"
	TypeEnum         = "enum"
	TypeInteger      = "long"
	TypeBool         = "bool"
	TypeClass        = "type"
	TypeGlobalClass  = "GlbC"
	TypeClassAlt     = "Clss"
	TypeAlias        = "alis"
	TypePath         = "Pth "
	TypeRawData      = "tdta"
)

// Unit 4CCs carried by a UntF value.
const (
	UnitPercent     = "#Prc"
	UnitAngle       = "#Ang"
	UnitPixels      = "#Pxl"
	UnitDistance    = "#Rlt"
	UnitDensity     = "#Rsl"
	UnitMillimeters = "#Mlm"
	UnitPoints      = "#Pnt"
	UnitNone        = "#Nne"
)

// Limits caps parser work and allocations. Zero-valued fields are replaced by
// the corresponding values from DefaultLimits.
type Limits struct {
	MaxDepth       int
	MaxItems       int
	MaxStringBytes int
	MaxDataBytes   int
}

// DefaultLimits returns conservative limits suitable for untrusted input.
func DefaultLimits() Limits {
	return Limits{
		MaxDepth:       32,
		MaxItems:       100_000,
		MaxStringBytes: 1 << 20,
		MaxDataBytes:   32 << 20,
	}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxDepth <= 0 {
		l.MaxDepth = d.MaxDepth
	}
	if l.MaxItems <= 0 {
		l.MaxItems = d.MaxItems
	}
	if l.MaxStringBytes <= 0 {
		l.MaxStringBytes = d.MaxStringBytes
	}
	if l.MaxDataBytes <= 0 {
		l.MaxDataBytes = d.MaxDataBytes
	}
	return l
}

// Descriptor is an Adobe Action Descriptor: a named, classed, ordered list of
// key/value items.
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

// Value is a typed Action Descriptor value. Only the fields corresponding to
// Type are populated.
type Value struct {
	Type    string
	Bool    bool
	Integer int32
	Float   float64
	String  string
	Unit    string
	Enum    EnumValue
	ClassID string
	// Name is the unicode name that precedes the class ID of a type/GlbC/Clss
	// value. It is usually empty, and it is separate from ClassID because the
	// format carries both: reading only the class ID desynchronises the stream.
	Name   string
	Object *Descriptor
	List   []Value
	Data   []byte
}

// EnumValue stores an Action Descriptor enumeration type and value.
type EnumValue struct {
	Type  string
	Value string
}

// Text returns a TEXT value without the trailing NUL Photoshop writes. Use it
// wherever the string reaches a user or the layer model; use Value.String when
// the bytes must survive a round trip unchanged.
func (v Value) Text() string {
	return strings.TrimSuffix(v.String, "\x00")
}

// Get returns the first item with the given key.
func (d Descriptor) Get(key string) (Value, bool) {
	for i := range d.Items {
		if d.Items[i].Key == key {
			return d.Items[i].Value, true
		}
	}
	return Value{}, false
}

// Object returns the nested descriptor stored under key.
func (d Descriptor) Object(key string) (Descriptor, bool) {
	v, ok := d.Get(key)
	if !ok || v.Object == nil {
		return Descriptor{}, false
	}
	return *v.Object, true
}

// UnitFloat returns the value of a UntF item only when it carries the expected
// unit, so a caller cannot silently read an angle as a percentage.
func (d Descriptor) UnitFloat(key, unit string) (float64, bool) {
	v, ok := d.Get(key)
	if !ok || v.Type != TypeUnitFloat || v.Unit != unit {
		return 0, false
	}
	return v.Float, true
}

// Bool returns the value of a bool item, and fallback when it is absent.
func (d Descriptor) Bool(key string, fallback bool) bool {
	v, ok := d.Get(key)
	if !ok || v.Type != TypeBool {
		return fallback
	}
	return v.Bool
}

// Enum returns the enumeration value stored under key.
func (d Descriptor) Enum(key string) (EnumValue, bool) {
	v, ok := d.Get(key)
	if !ok || v.Type != TypeEnum {
		return EnumValue{}, false
	}
	return v.Enum, true
}
