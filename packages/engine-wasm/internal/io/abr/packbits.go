package abr

import "fmt"

func decodePackBitsRows(r *reader, width, height int) ([]byte, error) {
	lengthTableBytes := height * 2
	if height < 0 || lengthTableBytes/2 != height {
		return nil, fmt.Errorf("%w: PackBits row table overflow", ErrLimit)
	}
	table, err := r.take(lengthTableBytes)
	if err != nil {
		return nil, err
	}
	out := make([]byte, width*height)
	for row := 0; row < height; row++ {
		rowLen := int(table[row*2])<<8 | int(table[row*2+1])
		encoded, err := r.take(rowLen)
		if err != nil {
			return nil, fmt.Errorf("PackBits row %d: %w", row, err)
		}
		dst := out[row*width : (row+1)*width]
		if err := decodePackBitsRow(encoded, dst); err != nil {
			return nil, fmt.Errorf("PackBits row %d: %w", row, err)
		}
	}
	return out, nil
}

func decodePackBitsRow(src, dst []byte) error {
	si, di := 0, 0
	for si < len(src) {
		n := int(int8(src[si]))
		si++
		switch {
		case n >= 0:
			count := n + 1
			if count > len(src)-si || count > len(dst)-di {
				return fmt.Errorf("%w: literal run exceeds row", ErrMalformed)
			}
			copy(dst[di:], src[si:si+count])
			si += count
			di += count
		case n >= -127:
			count := 1 - n
			if si >= len(src) || count > len(dst)-di {
				return fmt.Errorf("%w: repeat run exceeds row", ErrMalformed)
			}
			for i := 0; i < count; i++ {
				dst[di+i] = src[si]
			}
			si++
			di += count
		case n == -128:
			// No-op.
		}
	}
	if di != len(dst) {
		return fmt.Errorf("%w: decoded row is %d bytes, want %d", ErrMalformed, di, len(dst))
	}
	return nil
}
