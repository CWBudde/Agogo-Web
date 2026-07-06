package engine

import (
	"encoding/json"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

// Alpha convention for all blur/averaging filters: colour is filtered
// alpha-weighted (premultiply → filter all four channels → unpremultiply).
// This matches the Gaussian path below and prevents transparent pixels from
// bleeding dark RGB halos into visible pixels.

type gaussianBlurParams struct {
	Radius int `json:"radius"`
}

func filterGaussianBlur(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p gaussianBlurParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}

	if selMask != nil {
		orig := append([]byte(nil), pixels...)
		premultiplyRGBA(pixels)
		sb := agglib.NewStackBlur[agglib.ColorSpaceSRGB]()
		sb.BlurRGBA8(pixels, w, h, w*4, p.Radius)
		unpremultiplyRGBA(pixels)
		for i := 0; i < len(pixels); i += 4 {
			idx := i / 4
			a := selMask[idx]
			if a == 0 {
				copy(pixels[i:i+4], orig[i:i+4])
			} else if a < 255 {
				// Blend original ↔ blurred in premultiplied space: the two
				// operands can carry different alpha, so a straight-space
				// per-channel lerp would darken feathered boundaries.
				pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = lerpRGBAPremul(
					orig[i], orig[i+1], orig[i+2], orig[i+3],
					pixels[i], pixels[i+1], pixels[i+2], pixels[i+3], a,
				)
			}
		}
		return nil
	}

	premultiplyRGBA(pixels)
	sb := agglib.NewStackBlur[agglib.ColorSpaceSRGB]()
	sb.BlurRGBA8(pixels, w, h, w*4, p.Radius)
	unpremultiplyRGBA(pixels)
	return nil
}

type boxBlurParams struct {
	Radius int `json:"radius"`
}

func filterBoxBlur(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p boxBlurParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	tmp := make([]byte, len(pixels))

	r := p.Radius
	diam := 2*r + 1

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sumR, sumG, sumB, sumA int
			for kx := -r; kx <= r; kx++ {
				sx := x + kx
				if sx < 0 {
					sx = 0
				} else if sx >= w {
					sx = w - 1
				}
				si := (y*w + sx) * 4
				sumR += int(work[si])
				sumG += int(work[si+1])
				sumB += int(work[si+2])
				sumA += int(work[si+3])
			}
			di := (y*w + x) * 4
			tmp[di] = byte(sumR / diam)
			tmp[di+1] = byte(sumG / diam)
			tmp[di+2] = byte(sumB / diam)
			tmp[di+3] = byte(sumA / diam)
		}
	}

	vert := make([]byte, len(pixels))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sumR, sumG, sumB, sumA int
			for ky := -r; ky <= r; ky++ {
				sy := y + ky
				if sy < 0 {
					sy = 0
				} else if sy >= h {
					sy = h - 1
				}
				si := (sy*w + x) * 4
				sumR += int(tmp[si])
				sumG += int(tmp[si+1])
				sumB += int(tmp[si+2])
				sumA += int(tmp[si+3])
			}
			di := (y*w + x) * 4
			vert[di] = byte(sumR / diam)
			vert[di+1] = byte(sumG / diam)
			vert[di+2] = byte(sumB / diam)
			vert[di+3] = byte(sumA / diam)
		}
	}

	unpremultiplyRGBA(vert)
	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		return vert[i], vert[i+1], vert[i+2], vert[i+3]
	})
	return nil
}

type motionBlurParams struct {
	Angle    int `json:"angle"`    // degrees, 0-360
	Distance int `json:"distance"` // pixels
}

func filterMotionBlur(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p motionBlurParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Distance <= 0 {
		return nil
	}

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	rad := float64(p.Angle) * math.Pi / 180.0
	dx := math.Cos(rad)
	dy := math.Sin(rad)
	dist := p.Distance

	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		px := (i / 4) % w
		py := (i / 4) / w
		var sumR, sumG, sumB, sumA float64
		count := 0
		for s := -dist; s <= dist; s++ {
			sx := float64(px) + float64(s)*dx
			sy := float64(py) + float64(s)*dy
			ix := int(math.Round(sx))
			iy := int(math.Round(sy))
			if ix < 0 {
				ix = 0
			} else if ix >= w {
				ix = w - 1
			}
			if iy < 0 {
				iy = 0
			} else if iy >= h {
				iy = h - 1
			}
			si := (iy*w + ix) * 4
			sumR += float64(work[si])
			sumG += float64(work[si+1])
			sumB += float64(work[si+2])
			sumA += float64(work[si+3])
			count++
		}
		inv := 1.0 / float64(count)
		return straightRGBAFromPremul(sumR*inv, sumG*inv, sumB*inv, sumA*inv)
	})
	return nil
}

type radialBlurParams struct {
	Type    string `json:"type"`    // "spin" or "zoom"
	Amount  int    `json:"amount"`  // 1-100
	Quality int    `json:"quality"` // 1 (draft) to 3 (best), controls sample count
}

func filterRadialBlur(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p radialBlurParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Amount <= 0 {
		return nil
	}
	if p.Quality <= 0 {
		p.Quality = 1
	}

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	cx := float64(w) / 2
	cy := float64(h) / 2

	samples := 8 << (p.Quality - 1)
	if samples > 32 {
		samples = 32
	}

	if p.Type == "zoom" {
		scale := float64(p.Amount) / 100.0 * 0.2
		applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
			px := (i / 4) % w
			py := (i / 4) / w
			var sumR, sumG, sumB, sumA float64
			for s := range samples {
				t := -scale/2 + scale*float64(s)/float64(samples-1)
				sx := cx + (float64(px)-cx)*(1+t)
				sy := cy + (float64(py)-cy)*(1+t)
				r, g, b, a := bilinearSampleRGBA(work, sx, sy, w, h)
				sumR += r
				sumG += g
				sumB += b
				sumA += a
			}
			inv := 1.0 / float64(samples)
			return straightRGBAFromPremul(sumR*inv, sumG*inv, sumB*inv, sumA*inv)
		})
	} else {
		maxAngle := float64(p.Amount) / 100.0 * math.Pi / 4
		applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
			px := (i / 4) % w
			py := (i / 4) / w
			dx := float64(px) - cx
			dy := float64(py) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			baseAngle := math.Atan2(dy, dx)
			var sumR, sumG, sumB, sumA float64
			for s := range samples {
				a := baseAngle - maxAngle/2 + maxAngle*float64(s)/float64(samples-1)
				sx := cx + dist*math.Cos(a)
				sy := cy + dist*math.Sin(a)
				r, g, b, sa := bilinearSampleRGBA(work, sx, sy, w, h)
				sumR += r
				sumG += g
				sumB += b
				sumA += sa
			}
			inv := 1.0 / float64(samples)
			return straightRGBAFromPremul(sumR*inv, sumG*inv, sumB*inv, sumA*inv)
		})
	}
	return nil
}

type surfaceBlurParams struct {
	Radius    int `json:"radius"`    // 1-100
	Threshold int `json:"threshold"` // 0-255
}

func filterSurfaceBlur(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p surfaceBlurParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 || p.Threshold <= 0 {
		return nil
	}

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	result := append([]byte(nil), work...)
	thresh := float64(p.Threshold)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ci := (y*w + x) * 4
			cr, cg, cb := float64(work[ci]), float64(work[ci+1]), float64(work[ci+2])
			var sumR, sumG, sumB, sumA, sumW float64

			for ky := -p.Radius; ky <= p.Radius; ky++ {
				for kx := -p.Radius; kx <= p.Radius; kx++ {
					nr := float64(clampedSample(work, x+kx, y+ky, 0, w, h))
					ng := float64(clampedSample(work, x+kx, y+ky, 1, w, h))
					nb := float64(clampedSample(work, x+kx, y+ky, 2, w, h))
					na := float64(clampedSample(work, x+kx, y+ky, 3, w, h))

					diff := (math.Abs(nr-cr) + math.Abs(ng-cg) + math.Abs(nb-cb)) / 3
					if diff > thresh {
						continue
					}
					weight := 1.0 - diff/thresh
					sumR += nr * weight
					sumG += ng * weight
					sumB += nb * weight
					sumA += na * weight
					sumW += weight
				}
			}

			if sumW > 0 {
				result[ci] = clamp8(sumR / sumW)
				result[ci+1] = clamp8(sumG / sumW)
				result[ci+2] = clamp8(sumB / sumW)
				result[ci+3] = clamp8(sumA / sumW)
			}
		}
	}

	unpremultiplyRGBA(result)
	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		return result[i], result[i+1], result[i+2], result[i+3]
	})
	return nil
}
