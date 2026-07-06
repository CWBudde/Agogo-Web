package engine

import (
	"encoding/json"
	"math"
)

// Distort filters displace all four channels (alpha included) so transparency
// moves with the pixels. Resampling happens on premultiplied data to avoid
// dark halos from transparent pixels' meaningless RGB (see filters_builtin_blur.go
// for the shared convention).

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

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	amt := float64(p.Amount)

	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w
		sx := float64(x) + amt*math.Sin(2*math.Pi*float64(y)/period)
		sy := float64(y) + amt*math.Sin(2*math.Pi*float64(x)/period)
		r, g, b, a := bilinearSampleRGBA(work, sx, sy, w, h)
		return straightRGBAFromPremul(r, g, b, a)
	})
	return nil
}

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
	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	cx := float64(w) / 2
	cy := float64(h) / 2
	maxDist := math.Min(cx, cy)
	maxAngle := float64(p.Angle) * math.Pi / 180.0

	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w
		dx := float64(x) - cx
		dy := float64(y) - cy
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist >= maxDist {
			return orig[i], orig[i+1], orig[i+2], orig[i+3]
		}
		angle := maxAngle * (1 - dist/maxDist)
		cosA := math.Cos(angle)
		sinA := math.Sin(angle)
		sx := cx + dx*cosA - dy*sinA
		sy := cy + dx*sinA + dy*cosA
		r, g, b, a := bilinearSampleRGBA(work, sx, sy, w, h)
		return straightRGBAFromPremul(r, g, b, a)
	})
	return nil
}

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

	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w
		sx := x - p.Horizontal
		sy := y - p.Vertical

		if p.Wrap == "wrap" {
			sx = ((sx % w) + w) % w
			sy = ((sy % h) + h) % h
		} else {
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
		return orig[si], orig[si+1], orig[si+2], orig[si+3]
	})
	return nil
}

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

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	cx := float64(w) / 2
	cy := float64(h) / 2
	maxRadius := math.Sqrt(cx*cx + cy*cy)

	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		x := (i / 4) % w
		y := (i / 4) / w

		if p.Mode == "polar-to-rectangular" {
			angle := float64(x) / float64(w) * 2 * math.Pi
			radius := float64(y) / float64(h) * maxRadius
			sx := cx + radius*math.Cos(angle)
			sy := cy + radius*math.Sin(angle)
			r, g, b, a := bilinearSampleRGBA(work, sx, sy, w, h)
			return straightRGBAFromPremul(r, g, b, a)
		}

		dx := float64(x) - cx
		dy := float64(y) - cy
		radius := math.Sqrt(dx*dx + dy*dy)
		angle := math.Atan2(dy, dx)
		if angle < 0 {
			angle += 2 * math.Pi
		}
		sx := angle / (2 * math.Pi) * float64(w)
		sy := radius / maxRadius * float64(h)
		r, g, b, a := bilinearSampleRGBA(work, sx, sy, w, h)
		return straightRGBAFromPremul(r, g, b, a)
	})
	return nil
}

type lensCorrectionParams struct {
	Distortion          float64 `json:"distortion"`             // -100 to +100
	ChromaticAberration float64 `json:"chromatic_aberration"`   // 0-100 (fringe offset in pixels)
	Vignette            float64 `json:"vignette"`               // -100 to +100
	PerspectiveV        float64 `json:"perspective_vertical"`   // -100 to +100
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

	work := append([]byte(nil), pixels...)
	premultiplyRGBA(work)
	result := make([]byte, len(pixels))

	cx := float64(w) / 2
	cy := float64(h) / 2
	maxR := math.Sqrt(cx*cx + cy*cy)
	distK := p.Distortion / 100.0 * 0.5
	caOffset := p.ChromaticAberration / 100.0 * 3.0
	vigAmount := p.Vignette / 100.0
	perspH := p.PerspectiveH / 100.0 * 0.3
	perspV := p.PerspectiveV / 100.0 * 0.3

	// unpremulChannel recovers a straight channel value from a premultiplied
	// sample and the alpha sampled at the same position.
	unpremulChannel := func(c, a float64) byte {
		if a <= 0 {
			return 0
		}
		if a > 255 {
			a = 255
		}
		return clamp8(c * 255.0 / a)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			di := (y*w + x) * 4

			nx := (float64(x) - cx) / cx
			ny := (float64(y) - cy) / cy

			if perspH != 0 || perspV != 0 {
				nx *= 1.0 + perspH*ny
				ny *= 1.0 + perspV*nx
			}

			r2 := nx*nx + ny*ny
			distFactor := 1.0 + distK*r2

			gx := cx + nx*distFactor*cx
			gy := cy + ny*distFactor*cy
			baseR, baseG, baseB, baseA := bilinearSampleRGBA(work, gx, gy, w, h)

			if caOffset != 0 {
				caFactor := caOffset * math.Sqrt(r2) / maxR
				rDistFactor := distFactor * (1.0 + caFactor*0.01)
				bDistFactor := distFactor * (1.0 - caFactor*0.01)

				rx := cx + nx*rDistFactor*cx
				ry := cy + ny*rDistFactor*cy
				rVal, _, _, rA := bilinearSampleRGBA(work, rx, ry, w, h)

				bx := cx + nx*bDistFactor*cx
				by := cy + ny*bDistFactor*cy
				_, _, bVal, bA := bilinearSampleRGBA(work, bx, by, w, h)

				result[di] = unpremulChannel(rVal, rA)
				result[di+1] = unpremulChannel(baseG, baseA)
				result[di+2] = unpremulChannel(bVal, bA)
			} else {
				sr, sg, sb, _ := straightRGBAFromPremul(baseR, baseG, baseB, baseA)
				result[di] = sr
				result[di+1] = sg
				result[di+2] = sb
			}

			if vigAmount != 0 {
				dist := math.Sqrt(r2)
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

			// Alpha is displaced along with the colour (sampled at the base
			// distorted position) instead of being copied from the original.
			result[di+3] = clamp8(baseA)
		}
	}

	applyFilteredRGBAWithMask(pixels, selMask, func(i int) (byte, byte, byte, byte) {
		return result[i], result[i+1], result[i+2], result[i+3]
	})
	return nil
}
