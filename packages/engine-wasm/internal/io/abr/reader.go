package abr

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

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

func (r *reader) u16() (uint16, error) {
	b, err := r.take(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
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

func (r *reader) classID(maxBytes int) (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return r.fourCC()
	}
	if uint64(n) > uint64(maxBytes) {
		return "", fmt.Errorf("%w: class ID is %d bytes", ErrLimit, n)
	}
	b, err := r.take(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
