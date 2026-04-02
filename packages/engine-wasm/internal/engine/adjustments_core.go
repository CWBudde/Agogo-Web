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

type channelMixerParams struct {
	Monochrome bool       `json:"monochrome,omitempty"`
	Red        [3]float64 `json:"red,omitempty"`
	Green      [3]float64 `json:"green,omitempty"`
	Blue       [3]float64 `json:"blue,omitempty"`
}

type selectiveColorTone struct {
	CyanRed      float64 `json:"cyanRed,omitempty"`
	MagentaGreen float64 `json:"magentaGreen,omitempty"`
	YellowBlue   float64 `json:"yellowBlue,omitempty"`
	Black        float64 `json:"black,omitempty"`
}

type selectiveColorParams struct {
	Mode     string             `json:"mode,omitempty"`
	Reds     selectiveColorTone `json:"reds,omitempty"`
	Yellows  selectiveColorTone `json:"yellows,omitempty"`
	Greens   selectiveColorTone `json:"greens,omitempty"`
	Cyans    selectiveColorTone `json:"cyans,omitempty"`
	Blues    selectiveColorTone `json:"blues,omitempty"`
	Magentas selectiveColorTone `json:"magentas,omitempty"`
	Whites   selectiveColorTone `json:"whites,omitempty"`
	Neutrals selectiveColorTone `json:"neutrals,omitempty"`
	Blacks   selectiveColorTone `json:"blacks,omitempty"`
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
	RegisterAdjustmentFactory("invert", invertAdjustmentFactory)
	RegisterAdjustmentFactory("channel-mixer", channelMixerAdjustmentFactory)
	RegisterAdjustmentFactory("channelmixer", channelMixerAdjustmentFactory)
	RegisterAdjustmentFactory("channel mixer", channelMixerAdjustmentFactory)
	RegisterAdjustmentFactory("threshold", thresholdAdjustmentFactory)
	RegisterAdjustmentFactory("posterize", posterizeAdjustmentFactory)
	RegisterAdjustmentFactory("selective-color", selectiveColorAdjustmentFactory)
	RegisterAdjustmentFactory("selectivecolor", selectiveColorAdjustmentFactory)
	RegisterAdjustmentFactory("selective color", selectiveColorAdjustmentFactory)
	RegisterAdjustmentFactory("photo-filter", photoFilterAdjustmentFactory)
	RegisterAdjustmentFactory("photofilter", photoFilterAdjustmentFactory)
	RegisterAdjustmentFactory("photo filter", photoFilterAdjustmentFactory)
	RegisterAdjustmentFactory("gradient-map", gradientMapAdjustmentFactory)
	RegisterAdjustmentFactory("gradientmap", gradientMapAdjustmentFactory)
	RegisterAdjustmentFactory("gradient map", gradientMapAdjustmentFactory)
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

func invertAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	_, err := decodeAdjustmentParams[struct{}](params)
	if err != nil {
		return nil, err
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		return 255 - r, 255 - g, 255 - b, a, nil
	}, nil
}

func channelMixerAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[channelMixerParams](params)
	if err != nil {
		return nil, err
	}
	matrix := [3][3]float64{
		cfg.Red,
		cfg.Green,
		cfg.Blue,
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		source := [3]float64{
			float64(r) / 255.0,
			float64(g) / 255.0,
			float64(b) / 255.0,
		}
		mixed := [3]float64{
			source[0]*matrix[0][0]/100.0 + source[1]*matrix[0][1]/100.0 + source[2]*matrix[0][2]/100.0,
			source[0]*matrix[1][0]/100.0 + source[1]*matrix[1][1]/100.0 + source[2]*matrix[1][2]/100.0,
			source[0]*matrix[2][0]/100.0 + source[1]*matrix[2][1]/100.0 + source[2]*matrix[2][2]/100.0,
		}
		if cfg.Monochrome {
			lum := clamp01(colorLuminance(mixed))
			rr, gg, bb := unitToRGBBytes(lum, lum, lum)
			return rr, gg, bb, a, nil
		}
		rr, gg, bb := unitToRGBBytes(clamp01(mixed[0]), clamp01(mixed[1]), clamp01(mixed[2]))
		return rr, gg, bb, a, nil
	}, nil
}

type thresholdParams struct {
	Threshold float64 `json:"threshold,omitempty"`
}

func thresholdAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[thresholdParams](params)
	if err != nil {
		return nil, err
	}
	if cfg.Threshold < 0 {
		cfg.Threshold = 0
	}
	if cfg.Threshold > 255 {
		cfg.Threshold = 255
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		lum := colorLuminance([3]float64{
			float64(r) / 255,
			float64(g) / 255,
			float64(b) / 255,
		}) * 255
		if lum >= cfg.Threshold {
			return 255, 255, 255, a, nil
		}
		return 0, 0, 0, a, nil
	}, nil
}

type posterizeParams struct {
	Levels float64 `json:"levels,omitempty"`
}

func posterizeAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[posterizeParams](params)
	if err != nil {
		return nil, err
	}
	levels := int(math.Round(cfg.Levels))
	if levels < 2 {
		levels = 2
	}
	if levels > 255 {
		levels = 255
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		return posterizeByte(r, levels), posterizeByte(g, levels), posterizeByte(b, levels), a, nil
	}, nil
}

type photoFilterParams struct {
	Color              [4]uint8 `json:"color,omitempty"`
	Density            float64  `json:"density,omitempty"`
	PreserveLuminosity bool     `json:"preserveLuminosity,omitempty"`
}

func photoFilterAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[photoFilterParams](params)
	if err != nil {
		return nil, err
	}
	density := clamp01(cfg.Density / 100.0)
	filter := [3]float64{
		float64(cfg.Color[0]) / 255.0,
		float64(cfg.Color[1]) / 255.0,
		float64(cfg.Color[2]) / 255.0,
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		source := [3]float64{
			float64(r) / 255.0,
			float64(g) / 255.0,
			float64(b) / 255.0,
		}
		filtered := [3]float64{
			source[0] * filter[0],
			source[1] * filter[1],
			source[2] * filter[2],
		}
		mixed := mixRGB(source, filtered, density)
		if cfg.PreserveLuminosity {
			mixed = setLuminosity(mixed, luminosity(source))
		}
		rr, gg, bb := unitToRGBBytes(mixed[0], mixed[1], mixed[2])
		return rr, gg, bb, a, nil
	}, nil
}

type gradientMapParams struct {
	Stops   []GradientStopPayload `json:"stops,omitempty"`
	Reverse bool                  `json:"reverse,omitempty"`
}

func gradientMapAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[gradientMapParams](params)
	if err != nil {
		return nil, err
	}

	lut := buildGradientLUT(cfg.Stops, [4]uint8{0, 0, 0, 255}, [4]uint8{255, 255, 255, 255})
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		rf, gf, bf := rgbBytesToUnit(r, g, b)
		lum := luminosity([3]float64{rf, gf, bf})
		if cfg.Reverse {
			lum = 1 - lum
		}
		mapped := gradientColorAt(lut, lum)
		mapped[3] = uint8((uint16(mapped[3]) * uint16(a)) / 255)
		return mapped[0], mapped[1], mapped[2], mapped[3], nil
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

func posterizeByte(value uint8, levels int) uint8 {
	if levels < 2 {
		return value
	}
	step := 255.0 / float64(levels-1)
	quantized := math.Round(float64(value)/step) * step
	return clampByte(quantized)
}

func selectiveColorRangeWeights(hue, saturation, luminance float64) [9]float64 {
	chromaWeight := clampRange(saturation/0.25, 0, 1)
	weights := [9]float64{
		hueSectorWeight(hue, 0) * chromaWeight,
		hueSectorWeight(hue, 60) * chromaWeight,
		hueSectorWeight(hue, 120) * chromaWeight,
		hueSectorWeight(hue, 180) * chromaWeight,
		hueSectorWeight(hue, 240) * chromaWeight,
		hueSectorWeight(hue, 300) * chromaWeight,
		clamp01((luminance-0.72)/0.28) * clamp01(1-saturation*2),
		clamp01(1-math.Abs(luminance-0.5)/0.45) * clamp01(1-saturation*1.4),
		clamp01((0.35 - luminance) / 0.35),
	}
	var total float64
	for _, weight := range weights {
		total += weight
	}
	if total <= 0 {
		weights[7] = 1
		return weights
	}
	for index := range weights {
		weights[index] = weights[index] / total
	}
	return weights
}

func selectiveColorApplyTone(source [3]float64, tone selectiveColorTone, mode string) [3]float64 {
	scale := func(value float64, channel float64) float64 {
		if mode != "absolute" {
			if value >= 0 {
				value *= 1 - channel
			} else {
				value *= channel
			}
		}
		return value
	}

	lum := colorLuminance(source)
	red := scale(tone.CyanRed/100.0, source[0])
	green := scale(tone.MagentaGreen/100.0, source[1])
	blue := scale(tone.YellowBlue/100.0, source[2])
	black := tone.Black / 100.0
	if mode != "absolute" {
		if black >= 0 {
			black *= 1 - lum
		} else {
			black *= lum
		}
	}
	return [3]float64{
		red - black,
		green - black,
		blue - black,
	}
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

func selectiveColorAdjustmentFactory(params json.RawMessage) (AdjustmentPixelFunc, error) {
	cfg, err := decodeAdjustmentParams[selectiveColorParams](params)
	if err != nil {
		return nil, err
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode != "absolute" {
		mode = "relative"
	}
	return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		source := [3]float64{
			float64(r) / 255.0,
			float64(g) / 255.0,
			float64(b) / 255.0,
		}
		h, s, l := rgbToHsl(source[0], source[1], source[2])
		hue := wrapDegrees(h * 360)
		weights := selectiveColorRangeWeights(hue, s, l)
		ranges := []struct {
			weight float64
			tone   selectiveColorTone
		}{
			{weights[0], cfg.Reds},
			{weights[1], cfg.Yellows},
			{weights[2], cfg.Greens},
			{weights[3], cfg.Cyans},
			{weights[4], cfg.Blues},
			{weights[5], cfg.Magentas},
			{weights[6], cfg.Whites},
			{weights[7], cfg.Neutrals},
			{weights[8], cfg.Blacks},
		}
		var total float64
		rr, gg, bb := source[0], source[1], source[2]
		for _, entry := range ranges {
			if entry.weight <= 0 {
				continue
			}
			total += entry.weight
			adjustment := selectiveColorApplyTone(source, entry.tone, mode)
			rr += adjustment[0] * entry.weight
			gg += adjustment[1] * entry.weight
			bb += adjustment[2] * entry.weight
		}
		if total > 0 {
			inv := 1.0 / total
			rr = clamp01(rr * inv)
			gg = clamp01(gg * inv)
			bb = clamp01(bb * inv)
		}
		ur, ug, ub := unitToRGBBytes(rr, gg, bb)
		return ur, ug, ub, a, nil
	}, nil
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
