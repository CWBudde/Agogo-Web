package engine

import (
	"encoding/json"
	"fmt"
	"math"
)

func clamp8(v float64) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func abs8diff(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}

// applyFilteredWithMask blends per-pixel results using a selection mask.
// fn returns the new R, G, B for a pixel at flat index i (stride 4).
// The alpha channel is left untouched — use applyFilteredRGBAWithMask for
// filters that also process alpha.
func applyFilteredWithMask(pixels []byte, selMask []byte, fn func(i int) (byte, byte, byte)) {
	for i := 0; i < len(pixels); i += 4 {
		nr, ng, nb := fn(i)
		idx := i / 4
		if selMask != nil && idx < len(selMask) {
			a := selMask[idx]
			if a == 0 {
				continue
			}
			if a < 255 {
				pixels[i] = blendByte(pixels[i], nr, a)
				pixels[i+1] = blendByte(pixels[i+1], ng, a)
				pixels[i+2] = blendByte(pixels[i+2], nb, a)
				continue
			}
		}
		pixels[i] = nr
		pixels[i+1] = ng
		pixels[i+2] = nb
	}
}

// applyFilteredRGBAWithMask blends per-pixel results (all four channels,
// including alpha) using a selection mask. fn returns the new straight-alpha
// R, G, B, A for a pixel at flat index i (stride 4). Partial mask values are
// blended in premultiplied space (lerpRGBAPremul) because the two operands may
// have different alpha — lerping straight channels independently would darken
// colour at feathered selection boundaries.
func applyFilteredRGBAWithMask(pixels []byte, selMask []byte, fn func(i int) (byte, byte, byte, byte)) {
	for i := 0; i < len(pixels); i += 4 {
		nr, ng, nb, na := fn(i)
		idx := i / 4
		if selMask != nil && idx < len(selMask) {
			a := selMask[idx]
			if a == 0 {
				continue
			}
			if a < 255 {
				pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = lerpRGBAPremul(
					pixels[i], pixels[i+1], pixels[i+2], pixels[i+3],
					nr, ng, nb, na, a)
				continue
			}
		}
		pixels[i] = nr
		pixels[i+1] = ng
		pixels[i+2] = nb
		pixels[i+3] = na
	}
}

// lerpRGBAPremul interpolates between two straight-alpha RGBA pixels by
// t (0 = old, 255 = new) in premultiplied space and returns the straight
// result. When both alphas are equal (e.g. fully opaque) this reduces to a
// plain per-channel lerp; when they differ it keeps the colour of the more
// visible operand instead of darkening towards the less visible one.
func lerpRGBAPremul(or, og, ob, oa, nr, ng, nb, na, t byte) (byte, byte, byte, byte) {
	w0 := uint32(255 - t)
	w1 := uint32(t)
	oaU := uint32(oa)
	naU := uint32(na)

	// Blended alpha, scaled by 255.
	q := oaU*w0 + naU*w1
	if q == 0 {
		return 0, 0, 0, 0
	}
	outA := byte((q + 127) / 255)

	// Each channel is premultiplied by its own alpha, lerped at the same
	// scale as q, and unpremultiplied by dividing through q (scales cancel).
	lerpCh := func(o, n byte) byte {
		p := uint32(o)*oaU*w0 + uint32(n)*naU*w1
		v := (p + q/2) / q
		if v > 255 {
			v = 255
		}
		return byte(v)
	}
	return lerpCh(or, nr), lerpCh(og, ng), lerpCh(ob, nb), outA
}

// premultiplyRGBA converts a straight-alpha RGBA8 buffer to premultiplied
// alpha in place. Filters that average or resample colour across pixels must
// operate on premultiplied data so fully/partially transparent pixels cannot
// bleed their (meaningless) RGB into visible pixels.
func premultiplyRGBA(buf []byte) {
	for i := 0; i < len(buf)-3; i += 4 {
		a := uint32(buf[i+3])
		switch a {
		case 255:
			// opaque: premultiplied == straight
		case 0:
			buf[i], buf[i+1], buf[i+2] = 0, 0, 0
		default:
			buf[i] = byte((uint32(buf[i])*a + 127) / 255)
			buf[i+1] = byte((uint32(buf[i+1])*a + 127) / 255)
			buf[i+2] = byte((uint32(buf[i+2])*a + 127) / 255)
		}
	}
}

// unpremultiplyRGBA converts a premultiplied RGBA8 buffer back to straight
// alpha in place.
func unpremultiplyRGBA(buf []byte) {
	for i := 0; i < len(buf)-3; i += 4 {
		a := uint32(buf[i+3])
		if a == 255 || a == 0 {
			continue
		}
		r := (uint32(buf[i])*255 + a/2) / a
		g := (uint32(buf[i+1])*255 + a/2) / a
		b := (uint32(buf[i+2])*255 + a/2) / a
		buf[i] = byte(min(r, 255))
		buf[i+1] = byte(min(g, 255))
		buf[i+2] = byte(min(b, 255))
	}
}

// straightRGBAFromPremul converts premultiplied float channel values (on a
// 0..255 scale) to straight-alpha RGBA bytes.
func straightRGBAFromPremul(r, g, b, a float64) (byte, byte, byte, byte) {
	if a <= 0 {
		return 0, 0, 0, 0
	}
	if a > 255 {
		a = 255
	}
	inv := 255.0 / a
	return clamp8(r * inv), clamp8(g * inv), clamp8(b * inv), clamp8(a)
}

// clampedSample returns the pixel value at (x,y) channel c with edge clamping.
func clampedSample(buf []byte, x, y, c, w, h int) byte {
	if x < 0 {
		x = 0
	} else if x >= w {
		x = w - 1
	}
	if y < 0 {
		y = 0
	} else if y >= h {
		y = h - 1
	}
	return buf[(y*w+x)*4+c]
}

// bilinearSampleRGBA samples all four channels of buf at (sx, sy) with edge
// clamping and returns the interpolated values as floats on a 0..255 scale.
// The buffer's alpha convention (straight or premultiplied) is preserved;
// resampling filters should pass a premultiplied buffer and convert the
// result back via straightRGBAFromPremul.
func bilinearSampleRGBA(buf []byte, sx, sy float64, w, h int) (r, g, b, a float64) {
	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))
	fx := sx - float64(x0)
	fy := sy - float64(y0)

	x0c := max(0, min(x0, w-1))
	x1c := max(0, min(x0+1, w-1))
	y0c := max(0, min(y0, h-1))
	y1c := max(0, min(y0+1, h-1))

	i00 := (y0c*w + x0c) * 4
	i10 := (y0c*w + x1c) * 4
	i01 := (y1c*w + x0c) * 4
	i11 := (y1c*w + x1c) * 4

	w00 := (1 - fx) * (1 - fy)
	w10 := fx * (1 - fy)
	w01 := (1 - fx) * fy
	w11 := fx * fy

	var out [4]float64
	for c := range 4 {
		out[c] = float64(buf[i00+c])*w00 + float64(buf[i10+c])*w10 +
			float64(buf[i01+c])*w01 + float64(buf[i11+c])*w11
	}
	return out[0], out[1], out[2], out[3]
}

// marshalFilterParams marshals internal filter params to JSON. It returns an
// error instead of panicking so unmarshalable values (e.g. NaN floats) cannot
// crash the engine.
func marshalFilterParams(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal filter params: %w", err)
	}
	return b, nil
}
