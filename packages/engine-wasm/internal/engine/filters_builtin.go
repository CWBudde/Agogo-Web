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

	RegisterFilter(FilterDef{
		ID:        "box-blur",
		Name:      "Box Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterBoxBlur)

	RegisterFilter(FilterDef{
		ID:       "sharpen",
		Name:     "Sharpen",
		Category: FilterCategorySharpen,
	}, filterSharpen)

	RegisterFilter(FilterDef{
		ID:       "sharpen-more",
		Name:     "Sharpen More",
		Category: FilterCategorySharpen,
	}, filterSharpenMore)

	RegisterFilter(FilterDef{
		ID:        "median",
		Name:      "Median",
		Category:  FilterCategoryNoise,
		HasDialog: true,
	}, filterMedian)

	RegisterFilter(FilterDef{
		ID:       "despeckle",
		Name:     "Despeckle",
		Category: FilterCategoryNoise,
	}, filterDespeckle)

	RegisterFilter(FilterDef{
		ID:        "minimum",
		Name:      "Minimum",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterMinimum)

	RegisterFilter(FilterDef{
		ID:        "maximum",
		Name:      "Maximum",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterMaximum)

	RegisterFilter(FilterDef{
		ID:        "ripple",
		Name:      "Ripple",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterRipple)

	RegisterFilter(FilterDef{
		ID:        "twirl",
		Name:      "Twirl",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterTwirl)

	RegisterFilter(FilterDef{
		ID:        "offset",
		Name:      "Offset",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterOffset)

	RegisterFilter(FilterDef{
		ID:        "polar-coordinates",
		Name:      "Polar Coordinates",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterPolarCoordinates)

	RegisterFilter(FilterDef{
		ID:        "motion-blur",
		Name:      "Motion Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterMotionBlur)

	RegisterFilter(FilterDef{
		ID:        "radial-blur",
		Name:      "Radial Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterRadialBlur)

	RegisterFilter(FilterDef{
		ID:        "surface-blur",
		Name:      "Surface Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterSurfaceBlur)

	RegisterFilter(FilterDef{
		ID:        "smart-sharpen",
		Name:      "Smart Sharpen",
		Category:  FilterCategorySharpen,
		HasDialog: true,
	}, filterSmartSharpen)

	RegisterFilter(FilterDef{
		ID:        "reduce-noise",
		Name:      "Reduce Noise",
		Category:  FilterCategoryNoise,
		HasDialog: true,
	}, filterReduceNoise)

	RegisterFilter(FilterDef{
		ID:        "lens-correction",
		Name:      "Lens Correction",
		Category:  FilterCategoryDistort,
		HasDialog: true,
	}, filterLensCorrection)
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
			return clamp8(float64(orig[i]) - float64(orig[si])*scale + 128),
				clamp8(float64(orig[i+1]) - float64(orig[si+1])*scale + 128),
				clamp8(float64(orig[i+2]) - float64(orig[si+2])*scale + 128)
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

// ---------------------------------------------------------------------------
// Box Blur (separable two-pass)
// ---------------------------------------------------------------------------

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

	orig := append([]byte(nil), pixels...)
	tmp := make([]byte, len(pixels))

	r := p.Radius
	diam := 2*r + 1

	// Horizontal pass: orig → tmp
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sumR, sumG, sumB int
			for kx := -r; kx <= r; kx++ {
				sx := x + kx
				if sx < 0 {
					sx = 0
				} else if sx >= w {
					sx = w - 1
				}
				si := (y*w + sx) * 4
				sumR += int(orig[si])
				sumG += int(orig[si+1])
				sumB += int(orig[si+2])
			}
			di := (y*w + x) * 4
			tmp[di] = byte(sumR / diam)
			tmp[di+1] = byte(sumG / diam)
			tmp[di+2] = byte(sumB / diam)
			tmp[di+3] = orig[di+3]
		}
	}

	// Vertical pass: tmp → apply via mask
	vert := make([]byte, len(pixels))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sumR, sumG, sumB int
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
			}
			di := (y*w + x) * 4
			vert[di] = byte(sumR / diam)
			vert[di+1] = byte(sumG / diam)
			vert[di+2] = byte(sumB / diam)
			vert[di+3] = tmp[di+3]
		}
	}

	// Apply result with mask blending
	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return vert[i], vert[i+1], vert[i+2]
	})
	return nil
}

// ---------------------------------------------------------------------------
// Sharpen (fixed 3x3 kernel)
// ---------------------------------------------------------------------------

func applyKernel3x3(pixels []byte, w, h int, selMask []byte, kernel [9]int) {
	orig := append([]byte(nil), pixels...)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		px := (i / 4) % w
		py := (i / 4) / w
		var result [3]byte
		for c := range 3 {
			var sum int
			k := 0
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					sum += int(clampedSample(orig, px+kx, py+ky, c, w, h)) * kernel[k]
					k++
				}
			}
			result[c] = clamp8(float64(sum))
		}
		return result[0], result[1], result[2]
	})
}

func filterSharpen(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	kernel := [9]int{
		0, -1, 0,
		-1, 5, -1,
		0, -1, 0,
	}
	applyKernel3x3(pixels, w, h, selMask, kernel)
	return nil
}

func filterSharpenMore(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	kernel := [9]int{
		-1, -1, -1,
		-1, 9, -1,
		-1, -1, -1,
	}
	applyKernel3x3(pixels, w, h, selMask, kernel)
	return nil
}

// ---------------------------------------------------------------------------
// Median
// ---------------------------------------------------------------------------

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

	// Pre-allocate sort buffers.
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

// selectMedian returns the median value from buf using a counting sort (0-255 range).
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

// ---------------------------------------------------------------------------
// Despeckle (median radius=1)
// ---------------------------------------------------------------------------

func filterDespeckle(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	return applyMedian(pixels, w, h, selMask, 1)
}

// ---------------------------------------------------------------------------
// Minimum (morphological erosion)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Maximum (morphological dilation)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Bilinear sampling helper for coordinate-remapping filters
// ---------------------------------------------------------------------------

func bilinearSample(orig []byte, sx, sy float64, w, h int) (byte, byte, byte) {
	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))
	fx := sx - float64(x0)
	fy := sy - float64(y0)

	// Clamp coordinates
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

// ---------------------------------------------------------------------------
// Ripple (Distort)
// ---------------------------------------------------------------------------

type rippleParams struct {
	Amount int    `json:"amount"` // displacement in pixels
	Size   string `json:"size"`   // "small", "medium", "large"
}

func filterRipple(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p rippleParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Amount == 0 {
		return nil
	}

	var period float64
	switch p.Size {
	case "large":
		period = 60
	case "medium":
		period = 30
	default:
		period = 10
	}

	orig := append([]byte(nil), pixels...)
	amt := float64(p.Amount)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w
		sx := float64(x) + amt*math.Sin(2*math.Pi*float64(y)/period)
		sy := float64(y) + amt*math.Sin(2*math.Pi*float64(x)/period)
		return bilinearSample(orig, sx, sy, w, h)
	})
	return nil
}

// ---------------------------------------------------------------------------
// Twirl (Distort)
// ---------------------------------------------------------------------------

type twirlParams struct {
	Angle int `json:"angle"` // degrees at center
}

func filterTwirl(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p twirlParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Angle == 0 {
		return nil
	}

	orig := append([]byte(nil), pixels...)
	cx := float64(w) / 2
	cy := float64(h) / 2
	maxDist := math.Min(cx, cy)
	maxAngle := float64(p.Angle) * math.Pi / 180.0

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w
		dx := float64(x) - cx
		dy := float64(y) - cy
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist >= maxDist {
			return orig[i], orig[i+1], orig[i+2]
		}
		angle := maxAngle * (1 - dist/maxDist)
		cosA := math.Cos(angle)
		sinA := math.Sin(angle)
		sx := cx + dx*cosA - dy*sinA
		sy := cy + dx*sinA + dy*cosA
		return bilinearSample(orig, sx, sy, w, h)
	})
	return nil
}

// ---------------------------------------------------------------------------
// Offset (Distort)
// ---------------------------------------------------------------------------

type offsetParams struct {
	Horizontal int    `json:"horizontal"`
	Vertical   int    `json:"vertical"`
	Wrap       string `json:"wrap"` // "wrap" or "repeat"
}

func filterOffset(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p offsetParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Horizontal == 0 && p.Vertical == 0 {
		return nil
	}

	orig := append([]byte(nil), pixels...)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w
		sx := x - p.Horizontal
		sy := y - p.Vertical

		if p.Wrap == "wrap" {
			sx = ((sx % w) + w) % w
			sy = ((sy % h) + h) % h
		} else {
			// repeat: clamp to edge
			if sx < 0 {
				sx = 0
			} else if sx >= w {
				sx = w - 1
			}
			if sy < 0 {
				sy = 0
			} else if sy >= h {
				sy = h - 1
			}
		}

		si := (sy*w + sx) * 4
		return orig[si], orig[si+1], orig[si+2]
	})
	return nil
}

// ---------------------------------------------------------------------------
// Polar Coordinates (Distort)
// ---------------------------------------------------------------------------

type polarCoordinatesParams struct {
	Mode string `json:"mode"` // "rectangular-to-polar" or "polar-to-rectangular"
}

func filterPolarCoordinates(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p polarCoordinatesParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}

	orig := append([]byte(nil), pixels...)
	cx := float64(w) / 2
	cy := float64(h) / 2
	maxRadius := math.Sqrt(cx*cx + cy*cy)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w

		if p.Mode == "polar-to-rectangular" {
			// Input (x,y) treated as (angle, radius).
			// x maps to angle [0, 2*pi], y maps to radius [0, maxRadius].
			angle := float64(x) / float64(w) * 2 * math.Pi
			radius := float64(y) / float64(h) * maxRadius
			sx := cx + radius*math.Cos(angle)
			sy := cy + radius*math.Sin(angle)
			return bilinearSample(orig, sx, sy, w, h)
		}

		// rectangular-to-polar (default): convert (x,y) to polar, map to output.
		dx := float64(x) - cx
		dy := float64(y) - cy
		radius := math.Sqrt(dx*dx + dy*dy)
		angle := math.Atan2(dy, dx)
		if angle < 0 {
			angle += 2 * math.Pi
		}
		sx := angle / (2 * math.Pi) * float64(w)
		sy := radius / maxRadius * float64(h)
		return bilinearSample(orig, sx, sy, w, h)
	})
	return nil
}

// ---------------------------------------------------------------------------
// Motion Blur
// ---------------------------------------------------------------------------

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

	orig := append([]byte(nil), pixels...)
	rad := float64(p.Angle) * math.Pi / 180.0
	dx := math.Cos(rad)
	dy := math.Sin(rad)
	dist := p.Distance

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		px := (i / 4) % w
		py := (i / 4) / w
		var sumR, sumG, sumB float64
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
			sumR += float64(orig[si])
			sumG += float64(orig[si+1])
			sumB += float64(orig[si+2])
			count++
		}
		inv := 1.0 / float64(count)
		return clamp8(sumR * inv), clamp8(sumG * inv), clamp8(sumB * inv)
	})
	return nil
}

// ---------------------------------------------------------------------------
// Radial Blur (spin / zoom)
// ---------------------------------------------------------------------------

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

	orig := append([]byte(nil), pixels...)
	cx := float64(w) / 2
	cy := float64(h) / 2

	// Sample count scales with quality: 8, 16, 32.
	samples := 8 << (p.Quality - 1)
	if samples > 32 {
		samples = 32
	}

	if p.Type == "zoom" {
		// Zoom blur: sample along radial line from center through pixel.
		scale := float64(p.Amount) / 100.0 * 0.2 // max 20% zoom range
		applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
			px := (i / 4) % w
			py := (i / 4) / w
			var sumR, sumG, sumB float64
			for s := range samples {
				t := -scale/2 + scale*float64(s)/float64(samples-1)
				sx := cx + (float64(px)-cx)*(1+t)
				sy := cy + (float64(py)-cy)*(1+t)
				r, g, b := bilinearSample(orig, sx, sy, w, h)
				sumR += float64(r)
				sumG += float64(g)
				sumB += float64(b)
			}
			inv := 1.0 / float64(samples)
			return clamp8(sumR * inv), clamp8(sumG * inv), clamp8(sumB * inv)
		})
	} else {
		// Spin blur: sample along arc around center.
		maxAngle := float64(p.Amount) / 100.0 * math.Pi / 4 // max 45 degrees
		applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
			px := (i / 4) % w
			py := (i / 4) / w
			dx := float64(px) - cx
			dy := float64(py) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			baseAngle := math.Atan2(dy, dx)
			var sumR, sumG, sumB float64
			for s := range samples {
				a := baseAngle - maxAngle/2 + maxAngle*float64(s)/float64(samples-1)
				sx := cx + dist*math.Cos(a)
				sy := cy + dist*math.Sin(a)
				r, g, b := bilinearSample(orig, sx, sy, w, h)
				sumR += float64(r)
				sumG += float64(g)
				sumB += float64(b)
			}
			inv := 1.0 / float64(samples)
			return clamp8(sumR * inv), clamp8(sumG * inv), clamp8(sumB * inv)
		})
	}
	return nil
}

// ---------------------------------------------------------------------------
// Surface Blur (edge-preserving)
// ---------------------------------------------------------------------------

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

	orig := append([]byte(nil), pixels...)
	result := make([]byte, len(pixels))
	copy(result, pixels)
	thresh := float64(p.Threshold)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ci := (y*w + x) * 4
			cr, cg, cb := float64(orig[ci]), float64(orig[ci+1]), float64(orig[ci+2])
			var sumR, sumG, sumB, sumW float64

			for ky := -p.Radius; ky <= p.Radius; ky++ {
				for kx := -p.Radius; kx <= p.Radius; kx++ {
					nr := float64(clampedSample(orig, x+kx, y+ky, 0, w, h))
					ng := float64(clampedSample(orig, x+kx, y+ky, 1, w, h))
					nb := float64(clampedSample(orig, x+kx, y+ky, 2, w, h))

					// Weight based on color similarity — large difference = low weight.
					diff := (math.Abs(nr-cr) + math.Abs(ng-cg) + math.Abs(nb-cb)) / 3
					if diff > thresh {
						continue
					}
					weight := 1.0 - diff/thresh
					sumR += nr * weight
					sumG += ng * weight
					sumB += nb * weight
					sumW += weight
				}
			}

			if sumW > 0 {
				result[ci] = clamp8(sumR / sumW)
				result[ci+1] = clamp8(sumG / sumW)
				result[ci+2] = clamp8(sumB / sumW)
			}
			result[ci+3] = orig[ci+3]
		}
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return result[i], result[i+1], result[i+2]
	})
	return nil
}

// ---------------------------------------------------------------------------
// Smart Sharpen
// ---------------------------------------------------------------------------

type smartSharpenParams struct {
	Amount        int    `json:"amount"`         // 1-500 percent
	Radius        int    `json:"radius"`          // 1-64
	Remove        string `json:"remove"`          // "gaussian", "lens", "motion"
	Angle         int    `json:"angle"`           // for motion remove only
	ShadowFade    int    `json:"shadow_fade"`     // 0-100, reduce sharpening in shadows
	HighlightFade int    `json:"highlight_fade"`  // 0-100, reduce sharpening in highlights
}

func filterSmartSharpen(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p smartSharpenParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 || p.Amount <= 0 {
		return nil
	}

	// Create blurred version using the selected blur type.
	blurred := append([]byte(nil), pixels...)

	switch p.Remove {
	case "motion":
		// Use motion blur kernel for deconvolution-style sharpening.
		_ = filterMotionBlur(blurred, w, h, nil, marshalFilterParams(motionBlurParams{
			Angle:    p.Angle,
			Distance: p.Radius,
		}))
	case "lens":
		// Lens blur approximation: use box blur (more uniform than gaussian).
		_ = filterBoxBlur(blurred, w, h, nil, marshalFilterParams(boxBlurParams{
			Radius: p.Radius,
		}))
	default:
		// Gaussian (default).
		sb := agglib.NewStackBlur()
		sb.BlurRGBA8(blurred, w, h, p.Radius)
	}

	amt := float64(p.Amount) / 100.0
	shadowFade := float64(p.ShadowFade) / 100.0
	highlightFade := float64(p.HighlightFade) / 100.0

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		r, g, b := float64(pixels[i]), float64(pixels[i+1]), float64(pixels[i+2])
		br, bg, bb := float64(blurred[i]), float64(blurred[i+1]), float64(blurred[i+2])

		// Luminance for fade calculation (BT.709).
		lum := (r*0.2126 + g*0.7152 + b*0.0722) / 255.0

		// Reduce sharpening in shadows and highlights.
		fadeAmount := amt
		if shadowFade > 0 && lum < 0.5 {
			fade := shadowFade * (1 - lum*2) // stronger fade in deeper shadows
			fadeAmount *= (1 - fade)
		}
		if highlightFade > 0 && lum > 0.5 {
			fade := highlightFade * ((lum - 0.5) * 2) // stronger fade in brighter highlights
			fadeAmount *= (1 - fade)
		}

		nr := clamp8(r + fadeAmount*(r-br))
		ng := clamp8(g + fadeAmount*(g-bg))
		nb := clamp8(b + fadeAmount*(b-bb))
		return nr, ng, nb
	})
	return nil
}

// marshalFilterParams marshals v to JSON, panicking on failure (only used for internal filter params).
func marshalFilterParams(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---------------------------------------------------------------------------
// Reduce Noise
// ---------------------------------------------------------------------------

type reduceNoiseParams struct {
	Strength        int  `json:"strength"`         // 0-10
	PreserveDetails int  `json:"preserve_details"`  // 0-100 percent
	ReduceColorNoise int `json:"reduce_color_noise"` // 0-100 percent
	SharpenDetails  int  `json:"sharpen_details"`    // 0-100 percent
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

	// Step 1: Luminance denoising via edge-aware bilateral-style filter.
	// Use strength as radius, preserve_details controls edge threshold.
	radius := p.Strength
	if radius > 5 {
		radius = 5 // cap for performance
	}
	edgeThresh := float64(25 + (100-p.PreserveDetails)*2) // higher preserve = lower threshold = less blur

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

	// Step 2: Color noise reduction — blur chroma while keeping luminance.
	if p.ReduceColorNoise > 0 {
		chromaStrength := float64(p.ReduceColorNoise) / 100.0
		// Simple approach: blend each pixel's color toward the local luminance-weighted average.
		for i := 0; i < len(denoised); i += 4 {
			r, g, b := float64(denoised[i]), float64(denoised[i+1]), float64(denoised[i+2])
			lum := r*0.2126 + g*0.7152 + b*0.0722
			denoised[i] = clamp8(r + chromaStrength*(lum-r))
			denoised[i+1] = clamp8(g + chromaStrength*(lum-g))
			denoised[i+2] = clamp8(b + chromaStrength*(lum-b))
		}
	}

	// Step 3: Sharpen details — apply mild unsharp mask to denoised result.
	if p.SharpenDetails > 0 {
		sharpAmt := float64(p.SharpenDetails) / 100.0 * 0.5 // mild
		blurred := append([]byte(nil), denoised...)
		sb := agglib.NewStackBlur()
		sb.BlurRGBA8(blurred, w, h, 1)
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

// ---------------------------------------------------------------------------
// Lens Correction
// ---------------------------------------------------------------------------

type lensCorrectionParams struct {
	Distortion          float64 `json:"distortion"`            // -100 to +100
	ChromaticAberration float64 `json:"chromatic_aberration"`  // 0-100 (fringe offset in pixels)
	Vignette            float64 `json:"vignette"`              // -100 to +100
	PerspectiveV        float64 `json:"perspective_vertical"`  // -100 to +100
	PerspectiveH        float64 `json:"perspective_horizontal"` // -100 to +100
}

func filterLensCorrection(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p lensCorrectionParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Distortion == 0 && p.ChromaticAberration == 0 && p.Vignette == 0 &&
		p.PerspectiveV == 0 && p.PerspectiveH == 0 {
		return nil
	}

	orig := append([]byte(nil), pixels...)
	result := make([]byte, len(pixels))
	// Initialize to transparent black.
	for i := 3; i < len(result); i += 4 {
		result[i] = 255
	}

	cx := float64(w) / 2
	cy := float64(h) / 2
	maxR := math.Sqrt(cx*cx + cy*cy)
	distK := p.Distortion / 100.0 * 0.5 // barrel/pincushion coefficient
	caOffset := p.ChromaticAberration / 100.0 * 3.0 // max 3px fringe
	vigAmount := p.Vignette / 100.0
	perspH := p.PerspectiveH / 100.0 * 0.3
	perspV := p.PerspectiveV / 100.0 * 0.3

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			di := (y*w + x) * 4

			// Normalized coordinates [-1, 1].
			nx := (float64(x) - cx) / cx
			ny := (float64(y) - cy) / cy

			// Perspective correction.
			if perspH != 0 || perspV != 0 {
				nx *= 1.0 + perspH*ny
				ny *= 1.0 + perspV*nx
			}

			// Barrel/pincushion distortion.
			r2 := nx*nx + ny*ny
			distFactor := 1.0 + distK*r2

			// Green channel (reference).
			gx := cx + nx*distFactor*cx
			gy := cy + ny*distFactor*cy
			_, gVal, _ := bilinearSample(orig, gx, gy, w, h)

			if caOffset != 0 {
				// Red and blue channels with chromatic aberration offset.
				caFactor := caOffset * math.Sqrt(r2) / maxR
				rDistFactor := distFactor * (1.0 + caFactor*0.01)
				bDistFactor := distFactor * (1.0 - caFactor*0.01)

				rx := cx + nx*rDistFactor*cx
				ry := cy + ny*rDistFactor*cy
				rVal, _, _ := bilinearSample(orig, rx, ry, w, h)

				bx := cx + nx*bDistFactor*cx
				by := cy + ny*bDistFactor*cy
				_, _, bVal := bilinearSample(orig, bx, by, w, h)

				result[di] = rVal
				result[di+1] = gVal
				result[di+2] = bVal
			} else {
				rVal, _, bVal := bilinearSample(orig, gx, gy, w, h)
				result[di] = rVal
				result[di+1] = gVal
				result[di+2] = bVal
			}

			// Vignette.
			if vigAmount != 0 {
				dist := math.Sqrt(r2)
				// Cosine-based vignette falloff.
				vig := 1.0 - vigAmount*dist*dist
				if vig < 0 {
					vig = 0
				}
				if vig > 2 {
					vig = 2
				}
				result[di] = clamp8(float64(result[di]) * vig)
				result[di+1] = clamp8(float64(result[di+1]) * vig)
				result[di+2] = clamp8(float64(result[di+2]) * vig)
			}

			result[di+3] = orig[di+3]
		}
	}

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return result[i], result[i+1], result[i+2]
	})
	return nil
}
