package engine

import (
	"encoding/json"
	"math"
	"math/rand/v2"

	agglib "github.com/cwbudde/agg_go"
)

type addNoiseParams struct {
	Amount        int    `json:"amount"`       // 0-400
	Distribution  string `json:"distribution"` // "uniform" or "gaussian"
	Monochromatic bool   `json:"monochromatic"`
}

func filterAddNoise(pixels []byte, _, _ int, selMask []byte, params json.RawMessage) error {
	var p addNoiseParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Amount <= 0 {
		return nil
	}

	rng := rand.New(rand.NewPCG(42, 0))
	amt := float64(p.Amount)

	noise := func() float64 {
		if p.Distribution == "gaussian" {
			return rng.NormFloat64() * amt * 0.5
		}
		return (rng.Float64()*2 - 1) * amt
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		if p.Monochromatic {
			n := noise()
			return clamp8(float64(pixels[i]) + n),
				clamp8(float64(pixels[i+1]) + n),
				clamp8(float64(pixels[i+2]) + n)
		}
		return clamp8(float64(pixels[i]) + noise()),
			clamp8(float64(pixels[i+1]) + noise()),
			clamp8(float64(pixels[i+2]) + noise())
	})
	return nil
}

type medianParams struct {
	Radius int `json:"radius"`
}

func filterMedian(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p medianParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}
	return applyMedian(pixels, w, h, selMask, p.Radius)
}

func applyMedian(pixels []byte, w, h int, selMask []byte, radius int) error {
	orig := append([]byte(nil), pixels...)
	diam := 2*radius + 1
	area := diam * diam

	bufR := make([]byte, area)
	bufG := make([]byte, area)
	bufB := make([]byte, area)

	med := make([]byte, len(pixels))
	copy(med, pixels)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := 0
			for ky := -radius; ky <= radius; ky++ {
				for kx := -radius; kx <= radius; kx++ {
					bufR[n] = clampedSample(orig, x+kx, y+ky, 0, w, h)
					bufG[n] = clampedSample(orig, x+kx, y+ky, 1, w, h)
					bufB[n] = clampedSample(orig, x+kx, y+ky, 2, w, h)
					n++
				}
			}
			mid := n / 2
			di := (y*w + x) * 4
			med[di] = selectMedian(bufR[:n], mid)
			med[di+1] = selectMedian(bufG[:n], mid)
			med[di+2] = selectMedian(bufB[:n], mid)
			med[di+3] = orig[di+3]
		}
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return med[i], med[i+1], med[i+2]
	})
	return nil
}

func selectMedian(buf []byte, mid int) byte {
	var counts [256]int
	for _, v := range buf {
		counts[v]++
	}
	sum := 0
	for i := range 256 {
		sum += counts[i]
		if sum > mid {
			return byte(i)
		}
	}
	return 255
}

func filterDespeckle(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	return applyMedian(pixels, w, h, selMask, 1)
}

type minMaxParams struct {
	Radius int `json:"radius"`
}

func filterMinimum(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p minMaxParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}

	orig := append([]byte(nil), pixels...)
	result := make([]byte, len(pixels))
	copy(result, pixels)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var minR, minG, minB byte = 255, 255, 255
			for ky := -p.Radius; ky <= p.Radius; ky++ {
				for kx := -p.Radius; kx <= p.Radius; kx++ {
					r := clampedSample(orig, x+kx, y+ky, 0, w, h)
					g := clampedSample(orig, x+kx, y+ky, 1, w, h)
					b := clampedSample(orig, x+kx, y+ky, 2, w, h)
					if r < minR {
						minR = r
					}
					if g < minG {
						minG = g
					}
					if b < minB {
						minB = b
					}
				}
			}
			di := (y*w + x) * 4
			result[di] = minR
			result[di+1] = minG
			result[di+2] = minB
			result[di+3] = orig[di+3]
		}
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return result[i], result[i+1], result[i+2]
	})
	return nil
}

func filterMaximum(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p minMaxParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}

	orig := append([]byte(nil), pixels...)
	result := make([]byte, len(pixels))
	copy(result, pixels)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var maxR, maxG, maxB byte
			for ky := -p.Radius; ky <= p.Radius; ky++ {
				for kx := -p.Radius; kx <= p.Radius; kx++ {
					r := clampedSample(orig, x+kx, y+ky, 0, w, h)
					g := clampedSample(orig, x+kx, y+ky, 1, w, h)
					b := clampedSample(orig, x+kx, y+ky, 2, w, h)
					if r > maxR {
						maxR = r
					}
					if g > maxG {
						maxG = g
					}
					if b > maxB {
						maxB = b
					}
				}
			}
			di := (y*w + x) * 4
			result[di] = maxR
			result[di+1] = maxG
			result[di+2] = maxB
			result[di+3] = orig[di+3]
		}
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return result[i], result[i+1], result[i+2]
	})
	return nil
}

type reduceNoiseParams struct {
	Strength         int `json:"strength"`           // 0-10
	PreserveDetails  int `json:"preserve_details"`   // 0-100 percent
	ReduceColorNoise int `json:"reduce_color_noise"` // 0-100 percent
	SharpenDetails   int `json:"sharpen_details"`    // 0-100 percent
}

func filterReduceNoise(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p reduceNoiseParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Strength <= 0 {
		return nil
	}

	radius := p.Strength
	if radius > 5 {
		radius = 5
	}
	edgeThresh := float64(25 + (100-p.PreserveDetails)*2)

	orig := append([]byte(nil), pixels...)
	denoised := make([]byte, len(pixels))
	copy(denoised, pixels)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ci := (y*w + x) * 4
			cr, cg, cb := float64(orig[ci]), float64(orig[ci+1]), float64(orig[ci+2])
			var sumR, sumG, sumB, sumW float64

			for ky := -radius; ky <= radius; ky++ {
				for kx := -radius; kx <= radius; kx++ {
					nr := float64(clampedSample(orig, x+kx, y+ky, 0, w, h))
					ng := float64(clampedSample(orig, x+kx, y+ky, 1, w, h))
					nb := float64(clampedSample(orig, x+kx, y+ky, 2, w, h))

					diff := (math.Abs(nr-cr) + math.Abs(ng-cg) + math.Abs(nb-cb)) / 3
					weight := math.Exp(-(diff * diff) / (2 * edgeThresh * edgeThresh))
					sumR += nr * weight
					sumG += ng * weight
					sumB += nb * weight
					sumW += weight
				}
			}

			if sumW > 0 {
				denoised[ci] = clamp8(sumR / sumW)
				denoised[ci+1] = clamp8(sumG / sumW)
				denoised[ci+2] = clamp8(sumB / sumW)
			}
			denoised[ci+3] = orig[ci+3]
		}
	}

	if p.ReduceColorNoise > 0 {
		chromaStrength := float64(p.ReduceColorNoise) / 100.0
		for i := 0; i < len(denoised); i += 4 {
			r, g, b := float64(denoised[i]), float64(denoised[i+1]), float64(denoised[i+2])
			lum := r*0.2126 + g*0.7152 + b*0.0722
			denoised[i] = clamp8(r + chromaStrength*(lum-r))
			denoised[i+1] = clamp8(g + chromaStrength*(lum-g))
			denoised[i+2] = clamp8(b + chromaStrength*(lum-b))
		}
	}

	if p.SharpenDetails > 0 {
		sharpAmt := float64(p.SharpenDetails) / 100.0 * 0.5
		blurred := append([]byte(nil), denoised...)
		sb := agglib.NewStackBlur[agglib.ColorSpaceSRGB]()
		sb.BlurRGBA8(blurred, w, h, w*4, 1)
		for i := 0; i < len(denoised); i += 4 {
			denoised[i] = clamp8(float64(denoised[i]) + sharpAmt*float64(int(denoised[i])-int(blurred[i])))
			denoised[i+1] = clamp8(float64(denoised[i+1]) + sharpAmt*float64(int(denoised[i+1])-int(blurred[i+1])))
			denoised[i+2] = clamp8(float64(denoised[i+2]) + sharpAmt*float64(int(denoised[i+2])-int(blurred[i+2])))
		}
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return denoised[i], denoised[i+1], denoised[i+2]
	})
	return nil
}
