package descriptor

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// minItemBytes is the smallest a key/value item can be: a 4-byte key length
// plus at least one key character, or the 8-byte length==0 form, plus the
// 4-byte value type. The guard stays conservative and does not count the
// payload - the smallest is "bool" at one byte - so a future zero-payload
// value type cannot turn this into a false rejection.
const minItemBytes = 9

// minListItemBytes is the same bound for a VlLs entry, which carries no key.
const minListItemBytes = 4

type reader struct {
	b   []byte
	off int
}

func (r *reader) remaining() int { return len(r.b) - r.off }

func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || n > r.remaining() {
		return nil, fmt.Errorf("%w: need %d bytes at offset %d, have %d", ErrMalformed, n, r.off, r.remaining())
	}
	b := r.b[r.off : r.off+n]
	r.off += n
	return b, nil
}

func (r *reader) u8() (uint8, error) {
	b, err := r.take(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (r *reader) u32() (uint32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

func (r *reader) i32() (int32, error) {
	v, err := r.u32()
	return int32(v), err
}

func (r *reader) f64() (float64, error) {
	b, err := r.take(8)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
}

func (r *reader) fourCC() (string, error) {
	b, err := r.take(4)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *reader) unicodeString(maxBytes int) (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	bytes64 := uint64(n) * 2
	if bytes64 > uint64(maxBytes) {
		return "", fmt.Errorf("%w: Unicode string is %d bytes", ErrLimit, bytes64)
	}
	b, err := r.take(int(bytes64))
	if err != nil {
		return "", err
	}
	u16 := make([]uint16, n)
	for i := range u16 {
		u16[i] = binary.BigEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u16)), nil
}

// lengthAndKey reads the format's four-character-or-counted identifier: a
// length, then either four raw bytes when the length is zero or that many bytes
// otherwise. It backs class IDs, item keys and both halves of an enum.
func (r *reader) lengthAndKey(maxBytes int) (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return r.fourCC()
	}
	if uint64(n) > uint64(maxBytes) {
		return "", fmt.Errorf("%w: identifier is %d bytes", ErrLimit, n)
	}
	b, err := r.take(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type state struct {
	limits Limits
	items  int
}

// Parse reads one descriptor body - name, class ID, item count, items - from
// the front of data and reports how many bytes it consumed. Trailing bytes are
// the caller's business: a TySh block carries a warp descriptor and four int32s
// after the text descriptor, while an ABR section is exactly one descriptor.
func Parse(data []byte, limits Limits) (Descriptor, int, error) {
	r := reader{b: data}
	s := state{limits: limits.withDefaults()}
	d, err := s.descriptor(&r, 0)
	if err != nil {
		return Descriptor{}, 0, err
	}
	return d, r.off, nil
}

// ParseAll is Parse plus the requirement that the whole input is consumed.
func ParseAll(data []byte, limits Limits) (Descriptor, error) {
	d, n, err := Parse(data, limits)
	if err != nil {
		return Descriptor{}, err
	}
	if n != len(data) {
		return Descriptor{}, fmt.Errorf("%w: %d trailing descriptor bytes", ErrMalformed, len(data)-n)
	}
	return d, nil
}

// ParseVersioned reads a u32 version, which must be 16, then one descriptor
// body, and requires the whole input to be consumed. This is the shape of an
// ABR "desc" section.
func ParseVersioned(data []byte, limits Limits) (Descriptor, error) {
	if len(data) < 4 {
		return Descriptor{}, fmt.Errorf("%w: descriptor section is %d bytes", ErrMalformed, len(data))
	}
	version := binary.BigEndian.Uint32(data)
	if version != 16 {
		return Descriptor{}, fmt.Errorf("%w: descriptor version %d", ErrUnsupported, version)
	}
	return ParseAll(data[4:], limits)
}

func (s *state) descriptor(r *reader, depth int) (Descriptor, error) {
	if depth >= s.limits.MaxDepth {
		return Descriptor{}, fmt.Errorf("%w: descriptor nesting exceeds %d", ErrLimit, s.limits.MaxDepth)
	}
	name, err := r.unicodeString(s.limits.MaxStringBytes)
	if err != nil {
		return Descriptor{}, err
	}
	classID, err := r.lengthAndKey(s.limits.MaxStringBytes)
	if err != nil {
		return Descriptor{}, err
	}
	count, err := r.u32()
	if err != nil {
		return Descriptor{}, err
	}
	if err := s.checkCount(r, count, minItemBytes); err != nil {
		return Descriptor{}, err
	}
	d := Descriptor{Name: name, ClassID: classID, Items: make([]Item, 0, count)}
	for range count {
		key, err := r.lengthAndKey(s.limits.MaxStringBytes)
		if err != nil {
			return Descriptor{}, err
		}
		value, err := s.value(r, depth)
		if err != nil {
			return Descriptor{}, fmt.Errorf("descriptor item %q: %w", key, err)
		}
		d.Items = append(d.Items, Item{Key: key, Value: value})
	}
	return d, nil
}

// checkCount applies both bounds a declared count needs: the remaining input,
// then the cumulative budget across the whole descriptor. The first is what
// stops a four-byte count reserving the whole budget before a byte of item data
// has been read - measured, that is a 20 MB allocation from a 12-byte payload.
func (s *state) checkCount(r *reader, count uint32, minBytes int) error {
	// Remaining input is checked first, and it is the tighter of the two: a
	// count the file cannot possibly contain is a structural impossibility of
	// the descriptor itself, while MaxItems is a policy cap on legitimate but
	// unreasonable input. Checking the budget first would report a hostile
	// four-byte count as "over budget", which says nothing useful.
	if uint64(count) > uint64(r.remaining()/minBytes) {
		return fmt.Errorf("%w: item count %d exceeds remaining input %d", ErrMalformed, count, r.remaining())
	}
	if uint64(s.items)+uint64(count) > uint64(s.limits.MaxItems) {
		return fmt.Errorf("%w: descriptor item count exceeds %d", ErrLimit, s.limits.MaxItems)
	}
	s.items += int(count)
	return nil
}

func (s *state) value(r *reader, depth int) (Value, error) {
	typ, err := r.fourCC()
	if err != nil {
		return Value{}, err
	}
	v := Value{Type: typ}
	switch typ {
	case TypeObject, TypeGlobalObject:
		d, err := s.descriptor(r, depth+1)
		if err != nil {
			return Value{}, err
		}
		v.Object = &d
	case TypeList:
		count, err := r.u32()
		if err != nil {
			return Value{}, err
		}
		if err := s.checkCount(r, count, minListItemBytes); err != nil {
			return Value{}, err
		}
		v.List = make([]Value, 0, count)
		for range count {
			item, err := s.value(r, depth+1)
			if err != nil {
				return Value{}, err
			}
			v.List = append(v.List, item)
		}
	case TypeDouble:
		v.Float, err = r.f64()
	case TypeUnitFloat:
		v.Unit, err = r.fourCC()
		if err == nil {
			v.Float, err = r.f64()
		}
	case TypeText:
		v.String, err = r.unicodeString(s.limits.MaxStringBytes)
	case TypeEnum:
		v.Enum.Type, err = r.lengthAndKey(s.limits.MaxStringBytes)
		if err == nil {
			v.Enum.Value, err = r.lengthAndKey(s.limits.MaxStringBytes)
		}
	case TypeInteger:
		v.Integer, err = r.i32()
	case TypeBool:
		var b uint8
		b, err = r.u8()
		if err == nil && b > 1 {
			err = fmt.Errorf("%w: Boolean value %d", ErrMalformed, b)
		}
		v.Bool = b == 1
	case TypeClass, TypeGlobalClass, TypeClassAlt:
		// A class value is a unicode name FOLLOWED BY a class ID. Reading only
		// the class ID leaves the name's bytes in the stream and desynchronises
		// every value after it.
		v.Name, err = r.unicodeString(s.limits.MaxStringBytes)
		if err == nil {
			v.ClassID, err = r.lengthAndKey(s.limits.MaxStringBytes)
		}
	case TypeAlias, TypePath, TypeRawData:
		var n uint32
		n, err = r.u32()
		if err == nil && uint64(n) > uint64(s.limits.MaxDataBytes) {
			err = fmt.Errorf("%w: descriptor data is %d bytes", ErrLimit, n)
		}
		if err == nil {
			v.Data, err = r.take(int(n))
			if err == nil {
				v.Data = append([]byte(nil), v.Data...)
			}
		}
	default:
		return Value{}, fmt.Errorf("%w: descriptor value type %q", ErrUnsupported, typ)
	}
	return v, err
}
