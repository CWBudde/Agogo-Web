package abr

import "fmt"

type descriptorState struct {
	limits Limits
	items  int
}

func parseDescriptorSection(payload []byte, limits Limits) (Descriptor, error) {
	r := reader{b: payload}
	version, err := r.u32()
	if err != nil {
		return Descriptor{}, err
	}
	if version != 16 {
		return Descriptor{}, fmt.Errorf("%w: descriptor version %d", ErrUnsupported, version)
	}
	state := descriptorState{limits: limits}
	d, err := state.descriptor(&r, 0)
	if err != nil {
		return Descriptor{}, err
	}
	if r.remaining() != 0 {
		return Descriptor{}, fmt.Errorf("%w: %d trailing descriptor bytes", ErrMalformed, r.remaining())
	}
	return d, nil
}

func (s *descriptorState) descriptor(r *reader, depth int) (Descriptor, error) {
	if depth >= s.limits.MaxDescriptorDepth {
		return Descriptor{}, fmt.Errorf("%w: descriptor nesting exceeds %d", ErrLimit, s.limits.MaxDescriptorDepth)
	}
	name, err := r.unicodeString(s.limits.MaxStringBytes)
	if err != nil {
		return Descriptor{}, err
	}
	classID, err := r.classID(s.limits.MaxStringBytes)
	if err != nil {
		return Descriptor{}, err
	}
	count, err := r.u32()
	if err != nil {
		return Descriptor{}, err
	}
	if uint64(s.items)+uint64(count) > uint64(s.limits.MaxDescriptorItems) {
		return Descriptor{}, fmt.Errorf("%w: descriptor item count exceeds %d", ErrLimit, s.limits.MaxDescriptorItems)
	}
	s.items += int(count)
	d := Descriptor{Name: name, ClassID: classID, Items: make([]Item, 0, count)}
	for range count {
		key, err := r.classID(s.limits.MaxStringBytes)
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

func (s *descriptorState) value(r *reader, depth int) (Value, error) {
	typ, err := r.fourCC()
	if err != nil {
		return Value{}, err
	}
	v := Value{Type: typ}
	switch typ {
	case "Objc", "GlbO":
		d, err := s.descriptor(r, depth+1)
		if err != nil {
			return Value{}, err
		}
		v.Object = &d
	case "VlLs":
		count, err := r.u32()
		if err != nil {
			return Value{}, err
		}
		if uint64(s.items)+uint64(count) > uint64(s.limits.MaxDescriptorItems) {
			return Value{}, fmt.Errorf("%w: descriptor value count exceeds %d", ErrLimit, s.limits.MaxDescriptorItems)
		}
		s.items += int(count)
		v.List = make([]Value, 0, count)
		for range count {
			item, err := s.value(r, depth+1)
			if err != nil {
				return Value{}, err
			}
			v.List = append(v.List, item)
		}
	case "doub":
		v.Float, err = r.f64()
	case "UntF":
		v.Unit, err = r.fourCC()
		if err == nil {
			v.Float, err = r.f64()
		}
	case "TEXT":
		v.String, err = r.unicodeString(s.limits.MaxStringBytes)
	case "enum":
		v.Enum.Type, err = r.classID(s.limits.MaxStringBytes)
		if err == nil {
			v.Enum.Value, err = r.classID(s.limits.MaxStringBytes)
		}
	case "long":
		v.Integer, err = r.i32()
	case "bool":
		var b uint8
		b, err = r.u8()
		if err == nil && b > 1 {
			err = fmt.Errorf("%w: Boolean value %d", ErrMalformed, b)
		}
		v.Bool = b == 1
	case "type", "GlbC", "Clss":
		v.ClassID, err = r.classID(s.limits.MaxStringBytes)
	case "alis", "Pth ", "tdta":
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
