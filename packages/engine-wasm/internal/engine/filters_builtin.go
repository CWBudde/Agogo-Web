package engine

import (
	"encoding/json"
	"math"
	"math/rand/v2"

	agglib "github.com/cwbudde/agg_go"
)

func init() {
	RegisterFilter(FilterDef{
		ID:       "invert",
		Name:     "Invert",
		Category: FilterCategoryOther,
	}, filterInvert)

	RegisterFilter(FilterDef{
		ID:        "gaussian-blur",
		Name:      "Gaussian Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterGaussianBlur)

	RegisterFilter(FilterDef{
		ID:        "brightness-contrast",
		Name:      "Brightness/Contrast",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterBrightnessContrast)

	RegisterFilter(FilterDef{
		ID:        "unsharp-mask",
		Name:      "Unsharp Mask",
		Category:  FilterCategorySharpen,
		HasDialog: true,
	}, filterUnsharpMask)

	RegisterFilter(FilterDef{
		ID:        "add-noise",
		Name:      "Add Noise",
		Category:  FilterCategoryNoise,
		HasDialog: true,
	}, filterAddNoise)

	RegisterFilter(FilterDef{
		ID:        "high-pass",
		Name:      "High Pass",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterHighPass)

	RegisterFilter(FilterDef{
		ID:        "emboss",
		Name:      "Emboss",
		Category:  FilterCategoryStylize,
		HasDialog: true,
	}, filterEmboss)

	RegisterFilter(FilterDef{
		ID:       "solarize",
		Name:     "Solarize",
		Category: FilterCategoryStylize,
	}, filterSolarize)

	RegisterFilter(FilterDef{
		ID:       "find-edges",
		Name:     "Find Edges",
		Category: FilterCategoryStylize,
	}, filterFindEdges)
}

// ---------------------------------------------------------------------------
// Helpers shared across filters
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Invert
// ---------------------------------------------------------------------------

func filterInvert(pixels []byte, _, _ int, selMask []byte, _ json.RawMessage) error {
	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return 255 - pixels[i], 255 - pixels[i+1], 255 - pixels[i+2]
	})
	return nil
}

// ---------------------------------------------------------------------------
// Gaussian Blur (agg_go StackBlur)
// ---------------------------------------------------------------------------

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
		sb := agglib.NewStackBlur()
		sb.BlurRGBA8(pixels, w, h, p.Radius)
		// Blend blurred result with original using selection mask.
		for i := 0; i < len(pixels); i += 4 {
			idx := i / 4
			a := selMask[idx]
			if a == 0 {
				copy(pixels[i:i+4], orig[i:i+4])
			} else if a < 255 {
				pixels[i] = blendByte(orig[i], pixels[i], a)
				pixels[i+1] = blendByte(orig[i+1], pixels[i+1], a)
				pixels[i+2] = blendByte(orig[i+2], pixels[i+2], a)
				pixels[i+3] = blendByte(orig[i+3], pixels[i+3], a)
			}
		}
		return nil
	}

	sb := agglib.NewStackBlur()
	sb.BlurRGBA8(pixels, w, h, p.Radius)
	return nil
}

// ---------------------------------------------------------------------------
// Brightness / Contrast
// ---------------------------------------------------------------------------

type filterBCParams struct {
	Brightness int `json:"brightness"` // -150 to +150
	Contrast   int `json:"contrast"`   // -100 to +100
}

func filterBrightnessContrast(pixels []byte, _, _ int, selMask []byte, params json.RawMessage) error {
	var p filterBCParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Brightness == 0 && p.Contrast == 0 {
		return nil
	}

	// Build LUT — Photoshop-style contrast formula.
	var lut [256]byte
	contrastFactor := float64(259*(p.Contrast+255)) / float64(255*(259-p.Contrast))
	for i := range 256 {
		v := float64(i) + float64(p.Brightness)
		v = contrastFactor*(v-128) + 128
		lut[i] = clamp8(v)
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return lut[pixels[i]], lut[pixels[i+1]], lut[pixels[i+2]]
	})
	return nil
}

// ---------------------------------------------------------------------------
// Unsharp Mask (agg_go StackBlur)
// ---------------------------------------------------------------------------

type unsharpMaskParams struct {
	Amount    int `json:"amount"`    // percentage, 1-500
	Radius    int `json:"radius"`    // blur radius, 1-250
	Threshold int `json:"threshold"` // 0-255
}

func filterUnsharpMask(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p unsharpMaskParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 || p.Amount <= 0 {
		return nil
	}

	blurred := append([]byte(nil), pixels...)
	sb := agglib.NewStackBlur()
	sb.BlurRGBA8(blurred, w, h, p.Radius)

	amt := float64(p.Amount) / 100.0
	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		dr := abs8diff(pixels[i], blurred[i])
		dg := abs8diff(pixels[i+1], blurred[i+1])
		db := abs8diff(pixels[i+2], blurred[i+2])
		if int(dr)+int(dg)+int(db) < p.Threshold*3 {
			return pixels[i], pixels[i+1], pixels[i+2]
		}
		nr := clamp8(float64(pixels[i]) + amt*float64(int(pixels[i])-int(blurred[i])))
		ng := clamp8(float64(pixels[i+1]) + amt*float64(int(pixels[i+1])-int(blurred[i+1])))
		nb := clamp8(float64(pixels[i+2]) + amt*float64(int(pixels[i+2])-int(blurred[i+2])))
		return nr, ng, nb
	})
	return nil
}

// ---------------------------------------------------------------------------
// Add Noise
// ---------------------------------------------------------------------------

type addNoiseParams struct {
	Amount        int    `json:"amount"`        // 0-400
	Distribution  string `json:"distribution"`  // "uniform" or "gaussian"
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

// ---------------------------------------------------------------------------
// High Pass (agg_go StackBlur)
// ---------------------------------------------------------------------------

type highPassParams struct {
	Radius int `json:"radius"`
}

func filterHighPass(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p highPassParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}

	blurred := append([]byte(nil), pixels...)
	sb := agglib.NewStackBlur()
	sb.BlurRGBA8(blurred, w, h, p.Radius)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return clamp8(float64(pixels[i]) - float64(blurred[i]) + 128),
			clamp8(float64(pixels[i+1]) - float64(blurred[i+1]) + 128),
			clamp8(float64(pixels[i+2]) - float64(blurred[i+2]) + 128)
	})
	return nil
}

// ---------------------------------------------------------------------------
// Emboss
// ---------------------------------------------------------------------------

type embossParams struct {
	Angle  int `json:"angle"`  // degrees, 0-360
	Height int `json:"height"` // 1-10
	Amount int `json:"amount"` // 1-500 percent
}

func filterEmboss(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p embossParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Height <= 0 {
		p.Height = 1
	}
	if p.Amount <= 0 {
		p.Amount = 100
	}

	rad := float64(p.Angle) * math.Pi / 180.0
	dx := math.Cos(rad)
	dy := math.Sin(rad)
	scale := float64(p.Amount) / 100.0 * float64(p.Height)

	orig := append([]byte(nil), pixels...)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		px := (i / 4) % w
		py := (i / 4) / w
		sx := px + int(math.Round(dx))
		sy := py + int(math.Round(dy))
		si := samplePixelIdx(sx, sy, w, h)
		if si >= 0 {
			return clamp8(float64(orig[i])-float64(orig[si])*scale + 128),
				clamp8(float64(orig[i+1])-float64(orig[si+1])*scale + 128),
				clamp8(float64(orig[i+2])-float64(orig[si+2])*scale + 128)
		}
		return 128, 128, 128
	})
	return nil
}

func samplePixelIdx(x, y, w, h int) int {
	if x < 0 || x >= w || y < 0 || y >= h {
		return -1
	}
	return (y*w + x) * 4
}

// ---------------------------------------------------------------------------
// Solarize
// ---------------------------------------------------------------------------

func filterSolarize(pixels []byte, _, _ int, selMask []byte, _ json.RawMessage) error {
	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		r, g, b := pixels[i], pixels[i+1], pixels[i+2]
		if r >= 128 {
			r = 255 - r
		}
		if g >= 128 {
			g = 255 - g
		}
		if b >= 128 {
			b = 255 - b
		}
		return r, g, b
	})
	return nil
}

// ---------------------------------------------------------------------------
// Find Edges (Sobel operator)
// ---------------------------------------------------------------------------

func filterFindEdges(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	orig := append([]byte(nil), pixels...)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		px := (i / 4) % w
		py := (i / 4) / w
		var result [3]byte
		for c := range 3 {
			gx := -sobelSample(orig, px-1, py-1, c, w, h) - 2*sobelSample(orig, px-1, py, c, w, h) - sobelSample(orig, px-1, py+1, c, w, h) +
				sobelSample(orig, px+1, py-1, c, w, h) + 2*sobelSample(orig, px+1, py, c, w, h) + sobelSample(orig, px+1, py+1, c, w, h)
			gy := -sobelSample(orig, px-1, py-1, c, w, h) - 2*sobelSample(orig, px, py-1, c, w, h) - sobelSample(orig, px+1, py-1, c, w, h) +
				sobelSample(orig, px-1, py+1, c, w, h) + 2*sobelSample(orig, px, py+1, c, w, h) + sobelSample(orig, px+1, py+1, c, w, h)
			result[c] = clamp8(math.Sqrt(float64(gx*gx + gy*gy)))
		}
		return result[0], result[1], result[2]
	})
	return nil
}

func sobelSample(pixels []byte, x, y, c, w, h int) int {
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
	return int(pixels[(y*w+x)*4+c])
}
