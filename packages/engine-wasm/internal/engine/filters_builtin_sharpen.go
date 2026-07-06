package engine

import (
	"encoding/json"

	agglib "github.com/cwbudde/agg_go"
)

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
	sb := agglib.NewStackBlur[agglib.ColorSpaceSRGB]()
	sb.BlurRGBA8(blurred, w, h, w*4, p.Radius)

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

type smartSharpenParams struct {
	Amount        int    `json:"amount"`         // 1-500 percent
	Radius        int    `json:"radius"`         // 1-64
	Remove        string `json:"remove"`         // "gaussian", "lens", "motion"
	Angle         int    `json:"angle"`          // for motion remove only
	ShadowFade    int    `json:"shadow_fade"`    // 0-100, reduce sharpening in shadows
	HighlightFade int    `json:"highlight_fade"` // 0-100, reduce sharpening in highlights
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

	blurred := append([]byte(nil), pixels...)

	switch p.Remove {
	case "motion":
		mp, err := marshalFilterParams(motionBlurParams{
			Angle:    p.Angle,
			Distance: p.Radius,
		})
		if err != nil {
			return err
		}
		if err := filterMotionBlur(blurred, w, h, nil, mp); err != nil {
			return err
		}
	case "lens":
		bp, err := marshalFilterParams(boxBlurParams{
			Radius: p.Radius,
		})
		if err != nil {
			return err
		}
		if err := filterBoxBlur(blurred, w, h, nil, bp); err != nil {
			return err
		}
	default:
		sb := agglib.NewStackBlur[agglib.ColorSpaceSRGB]()
		sb.BlurRGBA8(blurred, w, h, w*4, p.Radius)
	}

	amt := float64(p.Amount) / 100.0
	shadowFade := float64(p.ShadowFade) / 100.0
	highlightFade := float64(p.HighlightFade) / 100.0

	applyFilteredWithMask(pixels, selMask, func(i int) (byte, byte, byte) {
		r, g, b := float64(pixels[i]), float64(pixels[i+1]), float64(pixels[i+2])
		br, bg, bb := float64(blurred[i]), float64(blurred[i+1]), float64(blurred[i+2])

		lum := (r*0.2126 + g*0.7152 + b*0.0722) / 255.0

		fadeAmount := amt
		if shadowFade > 0 && lum < 0.5 {
			fade := shadowFade * (1 - lum*2)
			fadeAmount *= (1 - fade)
		}
		if highlightFade > 0 && lum > 0.5 {
			fade := highlightFade * ((lum - 0.5) * 2)
			fadeAmount *= (1 - fade)
		}

		nr := clamp8(r + fadeAmount*(r-br))
		ng := clamp8(g + fadeAmount*(g-bg))
		nb := clamp8(b + fadeAmount*(b-bb))
		return nr, ng, nb
	})
	return nil
}
