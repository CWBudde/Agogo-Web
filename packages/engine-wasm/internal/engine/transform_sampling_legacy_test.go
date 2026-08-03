package engine

import "math"

// These samplers are retained only as migration oracles for the agg_go image
// filter tests. Production transforms and crop paths use DrawImageAffine.
func txPixelAt(pixels []byte, w, h, px, py int) [4]byte {
	px = clampInt(px, 0, w-1)
	py = clampInt(py, 0, h-1)
	i := (py*w + px) * 4
	return [4]byte{pixels[i], pixels[i+1], pixels[i+2], pixels[i+3]}
}

func sampleNearest(pixels []byte, w, h int, lx, ly float64) [4]byte {
	return txPixelAt(pixels, w, h, int(math.Round(lx-0.5)), int(math.Round(ly-0.5)))
}

func sampleBilinear(pixels []byte, w, h int, lx, ly float64) [4]byte {
	fx := lx - 0.5
	fy := ly - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	w00 := (1 - tx) * (1 - ty)
	w10 := tx * (1 - ty)
	w01 := (1 - tx) * ty
	w11 := tx * ty

	var x1, y1 int
	if x0 >= 0 && x0+1 < w && y0 >= 0 && y0+1 < h {
		x1, y1 = x0+1, y0+1
	} else {
		x0 = clampInt(x0, 0, w-1)
		y0 = clampInt(y0, 0, h-1)
		x1 = clampInt(x0+1, 0, w-1)
		y1 = clampInt(y0+1, 0, h-1)
	}
	i00 := (y0*w + x0) * 4
	i10 := (y0*w + x1) * 4
	i01 := (y1*w + x0) * 4
	i11 := (y1*w + x1) * 4
	return [4]byte{
		byte(float64(pixels[i00])*w00 + float64(pixels[i10])*w10 + float64(pixels[i01])*w01 + float64(pixels[i11])*w11),
		byte(float64(pixels[i00+1])*w00 + float64(pixels[i10+1])*w10 + float64(pixels[i01+1])*w01 + float64(pixels[i11+1])*w11),
		byte(float64(pixels[i00+2])*w00 + float64(pixels[i10+2])*w10 + float64(pixels[i01+2])*w01 + float64(pixels[i11+2])*w11),
		byte(float64(pixels[i00+3])*w00 + float64(pixels[i10+3])*w10 + float64(pixels[i01+3])*w01 + float64(pixels[i11+3])*w11),
	}
}

func catmullRomKernel(t, p0, p1, p2, p3 float64) float64 {
	return 0.5 * ((2 * p1) + (-p0+p2)*t + (2*p0-5*p1+4*p2-p3)*t*t + (-p0+3*p1-3*p2+p3)*t*t*t)
}

func sampleBicubic(pixels []byte, w, h int, lx, ly float64) [4]byte {
	fx, fy := lx-0.5, ly-0.5
	x, y := int(math.Floor(fx)), int(math.Floor(fy))
	tx, ty := fx-float64(x), fy-float64(y)
	var out [4]byte
	for channel := range 4 {
		var row [4]float64
		for offset := range 4 {
			row[offset] = catmullRomKernel(
				tx,
				float64(txPixelAt(pixels, w, h, x-1, y-1+offset)[channel]),
				float64(txPixelAt(pixels, w, h, x, y-1+offset)[channel]),
				float64(txPixelAt(pixels, w, h, x+1, y-1+offset)[channel]),
				float64(txPixelAt(pixels, w, h, x+2, y-1+offset)[channel]),
			)
		}
		out[channel] = byte(clampFloat(catmullRomKernel(ty, row[0], row[1], row[2], row[3]), 0, 255))
	}
	return out
}
