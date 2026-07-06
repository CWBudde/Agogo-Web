package engine

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"sync/atomic"

	agglib "github.com/cwbudde/agg_go"
)

type addNoiseParams struct {
	Amount        int    `json:"amount"`       // 0-400
	Distribution  string `json:"distribution"` // "uniform" or "gaussian"
	Monochromatic bool   `json:"monochromatic"`
	Seed          uint64 `json:"seed"` // optional: fixed seed for reproducible noise; 0 = fresh noise per application
}

// addNoiseInvocation makes each seedless Add Noise application draw from a
// distinct RNG stream. A hard-coded seed made every application replay the
// identical noise pattern, so reapplying the filter doubled the exact same
// offsets (perfectly correlated 2× noise) instead of adding independent noise.
var addNoiseInvocation atomic.Uint64

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

	seed := p.Seed
	if seed == 0 {
		seed = 0x9E3779B97F4A7C15 ^ addNoiseInvocation.Add(1)
	}
	rng := rand.New(rand.NewPCG(seed, 0xA0761D6478BD642F))
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
	// Median runs on premultiplied data (all four channels). Per-channel order
	// statistics preserve premulC <= alpha, so unpremultiplying cannot overshoot.
	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	diam := 2*radius + 1
	area := diam * diam

	bufR := make([]byte, area)
	bufG := make([]byte, area)
	bufB := make([]byte, area)
	bufA := make([]byte, area)

	med := make([]byte, len(pixels))
	copy(med, work)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := 0
			for ky := -radius; ky <= radius; ky++ {
				for kx := -radius; kx <= radius; kx++ {
					bufR[n] = clampedSample(work, x+kx, y+ky, 0, w, h)
					bufG[n] = clampedSample(work, x+kx, y+ky, 1, w, h)
					bufB[n] = clampedSample(work, x+kx, y+ky, 2, w, h)
					bufA[n] = clampedSample(work, x+kx, y+ky, 3, w, h)
					n++
				}
			}
			mid := n / 2
			di := (y*w + x) * 4
			med[di] = selectMedian(bufR[:n], mid)
			med[di+1] = selectMedian(bufG[:n], mid)
			med[di+2] = selectMedian(bufB[:n], mid)
			med[di+3] = selectMedian(bufA[:n], mid)
		}
	}

	unpremultiplyRGBA(med)
	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		return med[i], med[i+1], med[i+2], med[i+3]
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

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	result := make([]byte, len(pixels))
	copy(result, work)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var minR, minG, minB, minA byte = 255, 255, 255, 255
			for ky := -p.Radius; ky <= p.Radius; ky++ {
				for kx := -p.Radius; kx <= p.Radius; kx++ {
					r := clampedSample(work, x+kx, y+ky, 0, w, h)
					g := clampedSample(work, x+kx, y+ky, 1, w, h)
					b := clampedSample(work, x+kx, y+ky, 2, w, h)
					a := clampedSample(work, x+kx, y+ky, 3, w, h)
					if r < minR {
						minR = r
					}
					if g < minG {
						minG = g
					}
					if b < minB {
						minB = b
					}
					if a < minA {
						minA = a
					}
				}
			}
			di := (y*w + x) * 4
			result[di] = minR
			result[di+1] = minG
			result[di+2] = minB
			result[di+3] = minA
		}
	}

	unpremultiplyRGBA(result)
	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		return result[i], result[i+1], result[i+2], result[i+3]
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

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	result := make([]byte, len(pixels))
	copy(result, work)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var maxR, maxG, maxB, maxA byte
			for ky := -p.Radius; ky <= p.Radius; ky++ {
				for kx := -p.Radius; kx <= p.Radius; kx++ {
					r := clampedSample(work, x+kx, y+ky, 0, w, h)
					g := clampedSample(work, x+kx, y+ky, 1, w, h)
					b := clampedSample(work, x+kx, y+ky, 2, w, h)
					a := clampedSample(work, x+kx, y+ky, 3, w, h)
					if r > maxR {
						maxR = r
					}
					if g > maxG {
						maxG = g
					}
					if b > maxB {
						maxB = b
					}
					if a > maxA {
						maxA = a
					}
				}
			}
			di := (y*w + x) * 4
			result[di] = maxR
			result[di+1] = maxG
			result[di+2] = maxB
			result[di+3] = maxA
		}
	}

	unpremultiplyRGBA(result)
	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		return result[i], result[i+1], result[i+2], result[i+3]
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

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	denoised := make([]byte, len(pixels))
	copy(denoised, work)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ci := (y*w + x) * 4
			cr, cg, cb := float64(work[ci]), float64(work[ci+1]), float64(work[ci+2])
			var sumR, sumG, sumB, sumA, sumW float64

			for ky := -radius; ky <= radius; ky++ {
				for kx := -radius; kx <= radius; kx++ {
					nr := float64(clampedSample(work, x+kx, y+ky, 0, w, h))
					ng := float64(clampedSample(work, x+kx, y+ky, 1, w, h))
					nb := float64(clampedSample(work, x+kx, y+ky, 2, w, h))
					na := float64(clampedSample(work, x+kx, y+ky, 3, w, h))

					diff := (math.Abs(nr-cr) + math.Abs(ng-cg) + math.Abs(nb-cb)) / 3
					weight := math.Exp(-(diff * diff) / (2 * edgeThresh * edgeThresh))
					sumR += nr * weight
					sumG += ng * weight
					sumB += nb * weight
					sumA += na * weight
					sumW += weight
				}
			}

			if sumW > 0 {
				denoised[ci] = clamp8(sumR / sumW)
				denoised[ci+1] = clamp8(sumG / sumW)
				denoised[ci+2] = clamp8(sumB / sumW)
				denoised[ci+3] = clamp8(sumA / sumW)
			}
		}
	}

	unpremultiplyRGBA(denoised)

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

	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		return denoised[i], denoised[i+1], denoised[i+2], denoised[i+3]
	})
	return nil
}
