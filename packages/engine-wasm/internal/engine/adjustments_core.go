package engine

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

type levelsParams struct {
	Channel              string  `json:"channel,omitempty"`
	InputBlack           float64 `json:"inputBlack,omitempty"`
	InputWhite           float64 `json:"inputWhite,omitempty"`
	Gamma                float64 `json:"gamma,omitempty"`
	OutputBlack          float64 `json:"outputBlack,omitempty"`
	OutputWhite          float64 `json:"outputWhite,omitempty"`
	Auto                 bool    `json:"auto,omitempty"`
	ShadowClipPercent    float64 `json:"shadowClipPercent,omitempty"`
	HighlightClipPercent float64 `json:"highlightClipPercent,omitempty"`
}

type curvePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type curvesParams struct {
	Channel string       `json:"channel,omitempty"`
	Points  []curvePoint `json:"points,omitempty"`
}

type hueSatParams struct {
	HueShift   float64            `json:"hueShift,omitempty"`
	Saturation float64            `json:"saturation,omitempty"`
	Lightness  float64            `json:"lightness,omitempty"`
	Colorize   bool               `json:"colorize,omitempty"`
	Reds       *hueSatRangeParams `json:"reds,omitempty"`
	Yellows    *hueSatRangeParams `json:"yellows,omitempty"`
	Greens     *hueSatRangeParams `json:"greens,omitempty"`
	Cyans      *hueSatRangeParams `json:"cyans,omitempty"`
	Blues      *hueSatRangeParams `json:"blues,omitempty"`
	Magentas   *hueSatRangeParams `json:"magentas,omitempty"`
}

type hueSatRangeParams struct {
	HueShift   float64 `json:"hueShift,omitempty"`
	Saturation float64 `json:"saturation,omitempty"`
	Lightness  float64 `json:"lightness,omitempty"`
}

type colorBalanceTone struct {
	CyanRed      float64 `json:"cyanRed,omitempty"`
	MagentaGreen float64 `json:"magentaGreen,omitempty"`
	YellowBlue   float64 `json:"yellowBlue,omitempty"`
}

type colorBalanceParams struct {
	Shadows            colorBalanceTone `json:"shadows,omitempty"`
	Midtones           colorBalanceTone `json:"midtones,omitempty"`
	Highlights         colorBalanceTone `json:"highlights,omitempty"`
	PreserveLuminosity bool             `json:"preserveLuminosity,omitempty"`
}

type brightnessContrastParams struct {
	Brightness float64 `json:"brightness,omitempty"`
	Contrast   float64 `json:"contrast,omitempty"`
	Legacy     bool    `json:"legacy,omitempty"`
}

type exposureParams struct {
	Exposure float64 `json:"exposure,omitempty"`
	Offset   float64 `json:"offset,omitempty"`
	Gamma    float64 `json:"gamma,omitempty"`
}

type vibranceParams struct {
	Vibrance   float64 `json:"vibrance,omitempty"`
	Saturation float64 `json:"saturation,omitempty"`
}

type blackWhiteParams struct {
	Reds         float64  `json:"reds,omitempty"`
	Yellows      float64  `json:"yellows,omitempty"`
	Greens       float64  `json:"greens,omitempty"`
	Cyans        float64  `json:"cyans,omitempty"`
	Blues        float64  `json:"blues,omitempty"`
	Magentas     float64  `json:"magentas,omitempty"`
	Auto         bool     `json:"auto,omitempty"`
	Tint         bool     `json:"tint,omitempty"`
	TintColor    [3]uint8 `json:"tintColor,omitempty"`
	TintStrength float64  `json:"tintStrength,omitempty"`
}

func init() {
	registerCoreAdjustmentTransforms()
}

func registerCoreAdjustmentTransforms() {
	RegisterAdjustmentFactory("levels", levelsAdjustmentFactory)
	RegisterAdjustmentFactory("curves", curvesAdjustmentFactory)
	RegisterAdjustmentFactory("huesat", hueSatAdjustmentFactory)
	RegisterAdjustmentFactory("hue-sat", hueSatAdjustmentFactory)
	RegisterAdjustmentFactory("hue-saturation", hueSatAdjustmentFactory)
	RegisterAdjustmentFactory("hue/saturation", hueSatAdjustmentFactory)
	RegisterAdjustmentFactory("color-balance", colorBalanceAdjustmentFactory)
	RegisterAdjustmentFactory("colorbalance", colorBalanceAdjustmentFactory)
	RegisterAdjustmentFactory("brightness-contrast", brightnessContrastAdjustmentFactory)
	RegisterAdjustmentFactory("brightnesscontrast", brightnessContrastAdjustmentFactory)
	RegisterAdjustmentFactory("exposure", exposureAdjustmentFactory)
	RegisterAdjustmentFactory("vibrance", vibranceAdjustmentFactory)
	RegisterAdjustmentFactory("black-white", blackWhiteAdjustmentFactory)
	RegisterAdjustmentFactory("blackandwhite", blackWhiteAdjustmentFactory)
	RegisterAdjustmentFactory("black & white", blackWhiteAdjustmentFactory)
	RegisterAdjustmentFactory("black/white", blackWhiteAdjustmentFactory)
}

func levelsAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[levelsParams](params)
	if err != nil {
		return nil, err
	}
	if cfg.InputWhite <= 0 {
		cfg.InputWhite = 255
	}
	if cfg.Gamma <= 0 {
		cfg.Gamma = 1
	}
	if cfg.OutputWhite <= 0 && cfg.OutputBlack == 0 {
		cfg.OutputWhite = 255
	}
	channel := normalizeChannelSelector(cfg.Channel)
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		if channel == "" || channel == "rgb" || channel == "composite" {
			rr, gg, bb := applyLevelsToRGB(r, g, b, cfg)
			return rr, gg, bb, a, nil
		}
		switch channel {
		case "red":
			r = applyLevelsToValue(r, cfg)
		case "green":
			g = applyLevelsToValue(g, cfg)
		case "blue":
			b = applyLevelsToValue(b, cfg)
		}
		return r, g, b, a, nil
	}, nil
}

func curvesAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[curvesParams](params)
	if err != nil {
		return nil, err
	}
	points := normalizeCurvePoints(cfg.Points)
	channel := normalizeChannelSelector(cfg.Channel)
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		if len(points) == 0 {
			return r, g, b, a, nil
		}
		if channel == "" || channel == "rgb" || channel == "composite" {
			rr, gg, bb := applyCurvesToRGB(r, g, b, points)
			return rr, gg, bb, a, nil
		}
		switch channel {
		case "red":
			r = applyCurveValue(r, points)
		case "green":
			g = applyCurveValue(g, points)
		case "blue":
			b = applyCurveValue(b, points)
		}
		return r, g, b, a, nil
	}, nil
}

func hueSatAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[hueSatParams](params)
	if err != nil {
		return nil, err
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		rf, gf, bf := rgbBytesToUnit(r, g, b)
		h, s, l := rgbToHsl(rf, gf, bf)

		rangeShift, rangeSat, rangeLight := hueSatRangeAdjustments(h, s, cfg)
		h = wrapUnit(h + cfg.HueShift/360.0)
		s = clamp01(s + cfg.Saturation/100.0)
		l = clamp01(l + cfg.Lightness/100.0)
		if cfg.Colorize {
			h = wrapUnit(cfg.HueShift / 360.0)
			s = clamp01(0.75 + cfg.Saturation/100.0)
		} else {
			h = wrapUnit(h + rangeShift/360.0)
			s = clamp01(s + rangeSat/100.0)
			l = clamp01(l + rangeLight/100.0)
		}

		rr, gg, bb := hslToRGBBytes(h, s, l)
		return rr, gg, bb, a, nil
	}, nil
}

func colorBalanceAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[colorBalanceParams](params)
	if err != nil {
		return nil, err
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		rf, gf, bf := rgbBytesToUnit(r, g, b)
		lum := luminosity([3]float64{rf, gf, bf})
		shadowW, midW, highlightW := toneWeights(lum)

		rf += ((shadowW * cfg.Shadows.CyanRed) + (midW * cfg.Midtones.CyanRed) + (highlightW * cfg.Highlights.CyanRed)) / 100.0
		gf += ((shadowW * cfg.Shadows.MagentaGreen) + (midW * cfg.Midtones.MagentaGreen) + (highlightW * cfg.Highlights.MagentaGreen)) / 100.0
		bf += ((shadowW * cfg.Shadows.YellowBlue) + (midW * cfg.Midtones.YellowBlue) + (highlightW * cfg.Highlights.YellowBlue)) / 100.0

		color := [3]float64{clampUnit(rf), clampUnit(gf), clampUnit(bf)}
		if cfg.PreserveLuminosity {
			color = setLuminosity(color, lum)
		}

		rr, gg, bb := unitToRGBBytes(color[0], color[1], color[2])
		return rr, gg, bb, a, nil
	}, nil
}

func brightnessContrastAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[brightnessContrastParams](params)
	if err != nil {
		return nil, err
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		rf, gf, bf := rgbBytesToUnit(r, g, b)
		if cfg.Legacy {
			boost := cfg.Brightness / 150.0 * 0.5
			contrast := 1 + cfg.Contrast/100.0
			rf = clamp01((rf + boost) * contrast)
			gf = clamp01((gf + boost) * contrast)
			bf = clamp01((bf + boost) * contrast)
		} else {
			contrast := 1 + cfg.Contrast/100.0
			boost := cfg.Brightness / 150.0 * 0.5
			rf = clamp01((rf-0.5)*contrast + 0.5 + boost)
			gf = clamp01((gf-0.5)*contrast + 0.5 + boost)
			bf = clamp01((bf-0.5)*contrast + 0.5 + boost)
		}
		rr, gg, bb := unitToRGBBytes(rf, gf, bf)
		return rr, gg, bb, a, nil
	}, nil
}

func exposureAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[exposureParams](params)
	if err != nil {
		return nil, err
	}
	if cfg.Gamma <= 0 {
		cfg.Gamma = 1
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		rf, gf, bf := rgbBytesToUnit(r, g, b)
		factor := math.Pow(2, cfg.Exposure)
		rf = clamp01(math.Pow(clamp01((rf+cfg.Offset)*factor), cfg.Gamma))
		gf = clamp01(math.Pow(clamp01((gf+cfg.Offset)*factor), cfg.Gamma))
		bf = clamp01(math.Pow(clamp01((bf+cfg.Offset)*factor), cfg.Gamma))
		rr, gg, bb := unitToRGBBytes(rf, gf, bf)
		return rr, gg, bb, a, nil
	}, nil
}

func vibranceAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[vibranceParams](params)
	if err != nil {
		return nil, err
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		rf, gf, bf := rgbBytesToUnit(r, g, b)
		h, s, l := rgbToHsl(rf, gf, bf)
		s = clamp01(s + (1-s)*(cfg.Vibrance/100.0) + cfg.Saturation/100.0)
		rr, gg, bb := hslToRGBBytes(h, s, l)
		return rr, gg, bb, a, nil
	}, nil
}

func blackWhiteAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[blackWhiteParams](params)
	if err != nil {
		return nil, err
	}
	strength := cfg.TintStrength
	if strength <= 0 {
		strength = 0.35
	}
	tintColor := cfg.TintColor
	if tintColor == [3]uint8{} {
		tintColor = [3]uint8{112, 66, 20}
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		rf, gf, bf := rgbBytesToUnit(r, g, b)
		h, _, _ := rgbToHsl(rf, gf, bf)
		lum := colorLuminance([3]float64{rf, gf, bf})
		gray := lum + blackWhiteHueOffset(h*360, cfg)/100.0
		gray = clamp01(gray)

		if cfg.Tint {
			grayRGB := [3]float64{gray, gray, gray}
			tintRGB := [3]float64{
				float64(tintColor[0]) / 255,
				float64(tintColor[1]) / 255,
				float64(tintColor[2]) / 255,
			}
			mixed := mixRGB(grayRGB, tintRGB, strength)
			rr, gg, bb := unitToRGBBytes(mixed[0], mixed[1], mixed[2])
			return rr, gg, bb, a, nil
		}

		rr, gg, bb := unitToRGBBytes(gray, gray, gray)
		return rr, gg, bb, a, nil
	}, nil
}

func decodeAdjustmentParams[T any](params json.RawMessage) (T, error) {
	var cfg T
	if len(params) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(params, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func resolveLevelsParamsForSurface(surface []byte, docW, docH int, layer *AdjustmentLayer, clipAlpha []byte) (json.RawMessage, error) {
	cfg, err := decodeAdjustmentParams[levelsParams](layer.Params)
	if err != nil {
		return nil, err
	}
	if !cfg.Auto {
		return layer.Params, nil
	}

	black, white, ok := autoLevelsRange(surface, docW, docH, layer.Mask(), clipAlpha, normalizeChannelSelector(cfg.Channel), cfg.ShadowClipPercent, cfg.HighlightClipPercent)
	if !ok {
		return layer.Params, nil
	}
	cfg.InputBlack = float64(black)
	cfg.InputWhite = float64(white)
	if cfg.Gamma <= 0 {
		cfg.Gamma = 1
	}
	if cfg.OutputWhite <= 0 && cfg.OutputBlack == 0 {
		cfg.OutputWhite = 255
	}
	resolved, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func resolveBlackWhiteParamsForSurface(surface []byte, docW, docH int, layer *AdjustmentLayer, clipAlpha []byte) (json.RawMessage, error) {
	cfg, err := decodeAdjustmentParams[blackWhiteParams](layer.Params)
	if err != nil {
		return nil, err
	}
	if !cfg.Auto {
		return layer.Params, nil
	}

	auto := autoBlackWhiteOffsets(surface, docW, docH, layer.Mask(), clipAlpha)
	cfg.Reds += auto[0]
	cfg.Yellows += auto[1]
	cfg.Greens += auto[2]
	cfg.Cyans += auto[3]
	cfg.Blues += auto[4]
	cfg.Magentas += auto[5]

	resolved, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func normalizeChannelSelector(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "r":
		return "red"
	case "g":
		return "green"
	case "b":
		return "blue"
	case "", "rgb", "composite", "all":
		return "rgb"
	default:
		return strings.ToLower(strings.TrimSpace(channel))
	}
}

func autoLevelsRange(surface []byte, docW, docH int, mask *LayerMask, clipAlpha []byte, channel string, shadowClipPercent, highlightClipPercent float64) (uint8, uint8, bool) {
	if len(surface) == 0 || docW <= 0 || docH <= 0 {
		return 0, 0, false
	}

	shadowClipPercent = clampRange(shadowClipPercent, 0, 100)
	highlightClipPercent = clampRange(highlightClipPercent, 0, 100)
	if shadowClipPercent == 0 && highlightClipPercent == 0 {
		shadowClipPercent = 0.1
		highlightClipPercent = 0.1
	}

	var histogram [256]float64
	var totalWeight float64
	for y := 0; y < docH; y++ {
		for x := 0; x < docW; x++ {
			index := (y*docW + x) * 4
			if index < 0 || index+3 >= len(surface) {
				continue
			}

			coverage := scaleMaskedAlpha(clipSurfaceAlphaAt(clipAlpha, docW, x, y), layerMaskAlphaAt(mask, x, y))
			if coverage == 0 || surface[index+3] == 0 {
				continue
			}

			weight := float64(coverage) / 255.0 * float64(surface[index+3]) / 255.0
			if weight <= 0 {
				continue
			}

			value := histogramChannelValue(surface[index], surface[index+1], surface[index+2], channel)
			histogram[value] += weight
			totalWeight += weight
		}
	}
	if totalWeight <= 0 {
		return 0, 0, false
	}

	shadowTarget := totalWeight * shadowClipPercent / 100.0
	highlightTarget := totalWeight * highlightClipPercent / 100.0

	black := 0
	var cumulative float64
	for value, weight := range histogram {
		cumulative += weight
		if cumulative >= shadowTarget {
			black = value
			break
		}
	}

	white := 255
	cumulative = 0
	for value := len(histogram) - 1; value >= 0; value-- {
		cumulative += histogram[value]
		if cumulative >= highlightTarget {
			white = value
			break
		}
	}
	if white <= black {
		return 0, 0, false
	}
	return uint8(black), uint8(white), true
}

func applyLevelsToRGB(r, g, b uint8, cfg levelsParams) (uint8, uint8, uint8) {
	return applyLevelsToValue(r, cfg), applyLevelsToValue(g, cfg), applyLevelsToValue(b, cfg)
}

func applyLevelsToValue(value uint8, cfg levelsParams) uint8 {
	inputBlack := clampByte(cfg.InputBlack)
	inputWhite := clampByte(cfg.InputWhite)
	if inputWhite <= inputBlack {
		return value
	}
	outputBlack := clampByte(cfg.OutputBlack)
	outputWhite := clampByte(cfg.OutputWhite)
	gamma := cfg.Gamma
	if gamma <= 0 {
		gamma = 1
	}

	normalized := (float64(value) - float64(inputBlack)) / float64(inputWhite-inputBlack)
	normalized = clamp01(normalized)
	normalized = math.Pow(normalized, gamma)
	mapped := float64(outputBlack) + normalized*float64(int(outputWhite)-int(outputBlack))
	return clampByte(mapped)
}

func normalizeCurvePoints(points []curvePoint) []curvePoint {
	if len(points) == 0 {
		return nil
	}
	normalized := make([]curvePoint, 0, len(points)+2)
	for _, point := range points {
		normalized = append(normalized, curvePoint{
			X: clampRange(point.X, 0, 255),
			Y: clampRange(point.Y, 0, 255),
		})
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].X == normalized[j].X {
			return normalized[i].Y < normalized[j].Y
		}
		return normalized[i].X < normalized[j].X
	})
	if normalized[0].X > 0 {
		normalized = append([]curvePoint{{X: 0, Y: 0}}, normalized...)
	}
	if normalized[len(normalized)-1].X < 255 {
		normalized = append(normalized, curvePoint{X: 255, Y: 255})
	}
	return dedupeCurvePoints(normalized)
}

func dedupeCurvePoints(points []curvePoint) []curvePoint {
	if len(points) == 0 {
		return nil
	}
	result := make([]curvePoint, 0, len(points))
	for _, point := range points {
		if len(result) == 0 || result[len(result)-1].X != point.X {
			result = append(result, point)
			continue
		}
		result[len(result)-1] = point
	}
	return result
}

func applyCurvesToRGB(r, g, b uint8, points []curvePoint) (uint8, uint8, uint8) {
	return applyCurveValue(r, points), applyCurveValue(g, points), applyCurveValue(b, points)
}

func applyCurveValue(value uint8, points []curvePoint) uint8 {
	if len(points) == 0 {
		return value
	}
	x := float64(value)
	if x <= points[0].X {
		return clampByte(points[0].Y)
	}
	for index := 1; index < len(points); index++ {
		left := points[index-1]
		right := points[index]
		if x > right.X {
			continue
		}
		if right.X <= left.X {
			return clampByte(right.Y)
		}
		t := (x - left.X) / (right.X - left.X)
		return clampByte(left.Y + t*(right.Y-left.Y))
	}
	return clampByte(points[len(points)-1].Y)
}

func toneWeights(lum float64) (shadow, midtone, highlight float64) {
	lum = clamp01(lum)
	shadow = clamp01((0.5 - lum) * 2)
	highlight = clamp01((lum - 0.5) * 2)
	midtone = 1 - math.Abs(lum-0.5)*2
	return shadow, clamp01(midtone), highlight
}

func histogramChannelValue(r, g, b uint8, channel string) uint8 {
	switch channel {
	case "red":
		return r
	case "green":
		return g
	case "blue":
		return b
	default:
		lum := colorLuminance([3]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255})
		return clampByte(lum * 255)
	}
}

func hueSatRangeAdjustments(hue, saturation float64, cfg hueSatParams) (float64, float64, float64) {
	if saturation <= 0.01 {
		return 0, 0, 0
	}

	hue = wrapDegrees(hue * 360)
	ranges := [6]*hueSatRangeParams{cfg.Reds, cfg.Yellows, cfg.Greens, cfg.Cyans, cfg.Blues, cfg.Magentas}
	centers := [6]float64{0, 60, 120, 180, 240, 300}
	chromaWeight := clampRange(saturation/0.25, 0, 1)

	var hueShift float64
	var satShift float64
	var lightShift float64
	for index, center := range centers {
		if ranges[index] == nil {
			continue
		}
		weight := hueSectorWeight(hue, center) * chromaWeight
		if weight <= 0 {
			continue
		}
		hueShift += ranges[index].HueShift * weight
		satShift += ranges[index].Saturation * weight
		lightShift += ranges[index].Lightness * weight
	}
	return hueShift, satShift, lightShift
}

func blackWhiteHueOffset(hue float64, cfg blackWhiteParams) float64 {
	weights := [6]float64{
		hueSectorWeight(hue, 0),
		hueSectorWeight(hue, 60),
		hueSectorWeight(hue, 120),
		hueSectorWeight(hue, 180),
		hueSectorWeight(hue, 240),
		hueSectorWeight(hue, 300),
	}
	offset := weights[0]*cfg.Reds + weights[1]*cfg.Yellows + weights[2]*cfg.Greens + weights[3]*cfg.Cyans + weights[4]*cfg.Blues + weights[5]*cfg.Magentas
	return offset * 0.35
}

func hueSectorWeight(hue, center float64) float64 {
	dist := math.Abs(wrapDegrees(hue - center))
	if dist > 180 {
		dist = 360 - dist
	}
	return clamp01(1 - dist/60)
}

func autoBlackWhiteOffsets(surface []byte, docW, docH int, mask *LayerMask, clipAlpha []byte) [6]float64 {
	if len(surface) == 0 || docW <= 0 || docH <= 0 {
		return [6]float64{}
	}

	var hueWeight [6]float64
	var hueLum [6]float64
	var totalLum float64
	var totalWeight float64

	for y := 0; y < docH; y++ {
		for x := 0; x < docW; x++ {
			index := (y*docW + x) * 4
			if index < 0 || index+3 >= len(surface) {
				continue
			}

			coverage := scaleMaskedAlpha(clipSurfaceAlphaAt(clipAlpha, docW, x, y), layerMaskAlphaAt(mask, x, y))
			if coverage == 0 || surface[index+3] == 0 {
				continue
			}

			rf, gf, bf := rgbBytesToUnit(surface[index], surface[index+1], surface[index+2])
			h, s, _ := rgbToHsl(rf, gf, bf)
			h = wrapDegrees(h * 360)
			lum := colorLuminance([3]float64{rf, gf, bf})
			pixelWeight := float64(coverage) / 255.0 * float64(surface[index+3]) / 255.0
			if pixelWeight <= 0 {
				continue
			}

			totalLum += lum * pixelWeight
			totalWeight += pixelWeight

			chromaWeight := clampRange(s/0.2, 0, 1)
			if chromaWeight <= 0 {
				continue
			}

			for sector, center := range [...]float64{0, 60, 120, 180, 240, 300} {
				weight := hueSectorWeight(h, center) * chromaWeight * pixelWeight
				if weight <= 0 {
					continue
				}
				hueWeight[sector] += weight
				hueLum[sector] += lum * weight
			}
		}
	}

	if totalWeight <= 0 {
		return [6]float64{}
	}

	globalLum := totalLum / totalWeight
	var offsets [6]float64
	for sector := range offsets {
		if hueWeight[sector] <= 0 {
			continue
		}
		avgLum := hueLum[sector] / hueWeight[sector]
		offsets[sector] = clampRange((globalLum-avgLum)*160, -40, 40)
	}
	return offsets
}

func mixRGB(a, b [3]float64, amount float64) [3]float64 {
	amount = clamp01(amount)
	return [3]float64{
		a[0]*(1-amount) + b[0]*amount,
		a[1]*(1-amount) + b[1]*amount,
		a[2]*(1-amount) + b[2]*amount,
	}
}

func clampByte(value float64) uint8 {
	return uint8(clampRange(value, 0, 255) + 0.5)
}

func clampRange(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func rgbBytesToUnit(r, g, b uint8) (float64, float64, float64) {
	return float64(r) / 255, float64(g) / 255, float64(b) / 255
}

func unitToRGBBytes(r, g, b float64) (uint8, uint8, uint8) {
	return clampByte(r * 255), clampByte(g * 255), clampByte(b * 255)
}

func clamp01(value float64) float64 {
	return clampUnit(value)
}

func rgbToHsl(r, g, b float64) (float64, float64, float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l := (max + min) / 2
	if max == min {
		return 0, 0, l
	}

	d := max - min
	s := d / (1 - math.Abs(2*l-1))
	var h float64
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h /= 6
	return wrapUnit(h), clamp01(s), clamp01(l)
}

func hslToRGBBytes(h, s, l float64) (uint8, uint8, uint8) {
	r, g, b := hslToRGB(h, s, l)
	return unitToRGBBytes(r, g, b)
}

func hslToRGB(h, s, l float64) (float64, float64, float64) {
	h = wrapUnit(h)
	s = clamp01(s)
	l = clamp01(l)
	if s == 0 {
		return l, l, l
	}

	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	return hueToRGB(p, q, h+1.0/3.0), hueToRGB(p, q, h), hueToRGB(p, q, h-1.0/3.0)
}

func hueToRGB(p, q, t float64) float64 {
	t = wrapUnit(t)
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

func wrapUnit(value float64) float64 {
	value = math.Mod(value, 1)
	if value < 0 {
		value += 1
	}
	return value
}

func wrapDegrees(value float64) float64 {
	value = math.Mod(value, 360)
	if value < 0 {
		value += 360
	}
	return value
}
