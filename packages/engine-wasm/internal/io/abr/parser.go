package abr

import (
	"fmt"
	"strings"
)

const (
	samplePrefixV1 = 47
	samplePrefixV2 = 301
)

// Parse decodes an ABR library using DefaultLimits.
func Parse(data []byte) (*Library, error) {
	return ParseWithLimits(data, DefaultLimits())
}

// ParseWithLimits decodes an ABR library. On any error it returns a nil
// library, never a partially decoded result.
func ParseWithLimits(data []byte, limits Limits) (*Library, error) {
	limits = normalizedLimits(limits)
	if len(data) > limits.MaxFileBytes {
		return nil, fmt.Errorf("%w: file is %d bytes (maximum %d)", ErrLimit, len(data), limits.MaxFileBytes)
	}
	r := reader{b: data}
	version, err := r.u16()
	if err != nil {
		return nil, err
	}
	if version != 6 && version != 7 {
		return nil, fmt.Errorf("%w: major version %d", ErrUnsupported, version)
	}
	subversion, err := r.u16()
	if err != nil {
		return nil, err
	}
	if subversion != 1 && subversion != 2 {
		return nil, fmt.Errorf("%w: subversion %d", ErrUnsupported, subversion)
	}
	lib := &Library{Version: version, Subversion: subversion}
	for r.remaining() > 0 {
		if len(lib.Sections) >= limits.MaxRecords {
			return nil, fmt.Errorf("%w: section count exceeds %d", ErrLimit, limits.MaxRecords)
		}
		sig, err := r.fourCC()
		if err != nil {
			return nil, err
		}
		if sig != "8BIM" {
			return nil, fmt.Errorf("%w: invalid section signature %q", ErrMalformed, sig)
		}
		key, err := r.fourCC()
		if err != nil {
			return nil, err
		}
		length, err := r.u32()
		if err != nil {
			return nil, err
		}
		if uint64(length) > uint64(limits.MaxSectionBytes) {
			return nil, fmt.Errorf("%w: section %q is %d bytes", ErrLimit, key, length)
		}
		payload, err := r.take(int(length))
		if err != nil {
			return nil, fmt.Errorf("section %q: %w", key, err)
		}
		lib.Sections = append(lib.Sections, Section{Key: key, Length: length})
		switch key {
		case "samp":
			brushes, err := parseSampleSection(payload, subversion, limits, len(lib.Sampled))
			if err != nil {
				return nil, fmt.Errorf("section %q: %w", key, err)
			}
			lib.Sampled = append(lib.Sampled, brushes...)
		case "desc":
			d, err := parseDescriptorSection(payload, limits)
			if err != nil {
				return nil, fmt.Errorf("section %q: %w", key, err)
			}
			lib.Descriptors = append(lib.Descriptors, d)
		}
		padding := (4 - int(length)%4) % 4
		pad, err := r.take(padding)
		if err != nil {
			return nil, fmt.Errorf("section %q padding: %w", key, err)
		}
		for _, b := range pad {
			if b != 0 {
				return nil, fmt.Errorf("%w: non-zero section %q padding", ErrMalformed, key)
			}
		}
	}
	if len(lib.Sampled)+len(lib.Descriptors) == 0 {
		return nil, fmt.Errorf("%w: no supported brush records", ErrUnsupported)
	}
	return lib, nil
}

func parseSampleSection(payload []byte, subversion uint16, limits Limits, prior int) ([]SampledBrush, error) {
	r := reader{b: payload}
	brushes := make([]SampledBrush, 0)
	for r.remaining() > 0 {
		if prior+len(brushes) >= limits.MaxRecords {
			return nil, fmt.Errorf("%w: brush record count exceeds %d", ErrLimit, limits.MaxRecords)
		}
		n, err := r.u32()
		if err != nil {
			return nil, err
		}
		if uint64(n) > uint64(limits.MaxSectionBytes) {
			return nil, fmt.Errorf("%w: sampled record is %d bytes", ErrLimit, n)
		}
		recordBytes, err := r.take(int(n))
		if err != nil {
			return nil, err
		}
		brush, err := parseSampleRecord(recordBytes, subversion, limits)
		if err != nil {
			return nil, fmt.Errorf("sampled record %d: %w", prior+len(brushes), err)
		}
		brushes = append(brushes, brush)
		padding := (4 - int(n)%4) % 4
		pad, err := r.take(padding)
		if err != nil {
			return nil, err
		}
		for _, b := range pad {
			if b != 0 {
				return nil, fmt.Errorf("%w: non-zero sampled record padding", ErrMalformed)
			}
		}
	}
	return brushes, nil
}

func parseSampleRecord(data []byte, subversion uint16, limits Limits) (SampledBrush, error) {
	r := reader{b: data}
	prefixLen := samplePrefixV1
	if subversion == 2 {
		prefixLen = samplePrefixV2
	}
	prefix, err := r.take(prefixLen)
	if err != nil {
		return SampledBrush{}, err
	}
	key := sampleKey(prefix[:37])
	top, err := r.i32()
	if err != nil {
		return SampledBrush{}, err
	}
	left, err := r.i32()
	if err != nil {
		return SampledBrush{}, err
	}
	bottom, err := r.i32()
	if err != nil {
		return SampledBrush{}, err
	}
	right, err := r.i32()
	if err != nil {
		return SampledBrush{}, err
	}
	depth, err := r.u16()
	if err != nil {
		return SampledBrush{}, err
	}
	compression, err := r.u8()
	if err != nil {
		return SampledBrush{}, err
	}
	if depth != 8 {
		return SampledBrush{}, fmt.Errorf("%w: sampled depth %d", ErrUnsupported, depth)
	}
	width64, height64 := int64(right)-int64(left), int64(bottom)-int64(top)
	if width64 <= 0 || height64 <= 0 {
		return SampledBrush{}, fmt.Errorf("%w: invalid bounds (%d,%d)-(%d,%d)", ErrMalformed, left, top, right, bottom)
	}
	if width64 > int64(limits.MaxDimension) || height64 > int64(limits.MaxDimension) {
		return SampledBrush{}, fmt.Errorf("%w: dimensions %dx%d", ErrLimit, width64, height64)
	}
	maxInt := int64(^uint(0) >> 1)
	if width64 > maxInt || height64 > maxInt {
		return SampledBrush{}, fmt.Errorf("%w: dimensions exceed platform integer range", ErrLimit)
	}
	pixels64 := uint64(width64) * uint64(height64)
	if pixels64 > limits.MaxPixels || pixels64 > uint64(maxInt) {
		return SampledBrush{}, fmt.Errorf("%w: sampled tip has %d pixels", ErrLimit, pixels64)
	}
	width, height := int(width64), int(height64)
	var pixels []byte
	switch compression {
	case 0:
		pixels, err = r.take(int(pixels64))
		if err == nil {
			pixels = append([]byte(nil), pixels...)
		}
	case 1:
		pixels, err = decodePackBitsRows(&r, width, height)
	default:
		return SampledBrush{}, fmt.Errorf("%w: compression %d", ErrUnsupported, compression)
	}
	if err != nil {
		return SampledBrush{}, err
	}
	if r.remaining() != 0 {
		return SampledBrush{}, fmt.Errorf("%w: sampled record has %d trailing bytes", ErrMalformed, r.remaining())
	}
	return SampledBrush{
		Key: key, Top: top, Left: left, Width: width, Height: height,
		Depth: depth, Compression: compression, Pixels: pixels,
	}, nil
}

func sampleKey(b []byte) string {
	if len(b) == 37 && b[0] == 36 {
		return string(b[1:])
	}
	return strings.TrimRight(string(b), "\x00")
}

func normalizedLimits(got Limits) Limits {
	d := DefaultLimits()
	if got.MaxFileBytes > 0 {
		d.MaxFileBytes = got.MaxFileBytes
	}
	if got.MaxSectionBytes > 0 {
		d.MaxSectionBytes = got.MaxSectionBytes
	}
	if got.MaxRecords > 0 {
		d.MaxRecords = got.MaxRecords
	}
	if got.MaxDimension > 0 {
		d.MaxDimension = got.MaxDimension
	}
	if got.MaxPixels > 0 {
		d.MaxPixels = got.MaxPixels
	}
	if got.MaxDescriptorDepth > 0 {
		d.MaxDescriptorDepth = got.MaxDescriptorDepth
	}
	if got.MaxDescriptorItems > 0 {
		d.MaxDescriptorItems = got.MaxDescriptorItems
	}
	if got.MaxStringBytes > 0 {
		d.MaxStringBytes = got.MaxStringBytes
	}
	if got.MaxDataBytes > 0 {
		d.MaxDataBytes = got.MaxDataBytes
	}
	return d
}
