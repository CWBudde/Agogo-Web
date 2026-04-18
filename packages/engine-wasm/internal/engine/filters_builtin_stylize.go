package engine

import (
	"encoding/json"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

func filterInvert(pixels []byte, _, _ int, selMask []byte, _ json.RawMessage) error {
	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return 255 - pixels[i], 255 - pixels[i+1], 255 - pixels[i+2]
	})
	return nil
}

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
	sb := agglib.NewStackBlur[agglib.ColorSpaceSRGB]()
	sb.BlurRGBA8(blurred, w, h, w*4, p.Radius)

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		return clamp8(float64(pixels[i]) - float64(blurred[i]) + 128),
			clamp8(float64(pixels[i+1]) - float64(blurred[i+1]) + 128),
			clamp8(float64(pixels[i+2]) - float64(blurred[i+2]) + 128)
	})
	return nil
}

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
