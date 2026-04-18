package engine

import (
	"encoding/json"
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

func bilinearSample(orig []byte, sx, sy float64, w, h int) (byte, byte, byte) {
	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))
	fx := sx - float64(x0)
	fy := sy - float64(y0)

	x0c := max(0, min(x0, w-1))
	x1c := max(0, min(x0+1, w-1))
	y0c := max(0, min(y0, h-1))
	y1c := max(0, min(y0+1, h-1))

	var r, g, b float64
	for c := range 3 {
		v00 := float64(orig[(y0c*w+x0c)*4+c])
		v10 := float64(orig[(y0c*w+x1c)*4+c])
		v01 := float64(orig[(y1c*w+x0c)*4+c])
		v11 := float64(orig[(y1c*w+x1c)*4+c])
		v := v00*(1-fx)*(1-fy) + v10*fx*(1-fy) + v01*(1-fx)*fy + v11*fx*fy
		switch c {
		case 0:
			r = v
		case 1:
			g = v
		case 2:
			b = v
		}
	}
	return clamp8(r), clamp8(g), clamp8(b)
}

// marshalFilterParams marshals v to JSON, panicking on failure (only used for internal filter params).
func marshalFilterParams(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
