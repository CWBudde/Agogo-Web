package engine

import (
	"encoding/json"
	"reflect"
)

type DropShadowParams struct {
	BlendMode BlendMode `json:"blendMode"`
	Color     [4]uint8  `json:"color"`
	Opacity   float64   `json:"opacity"`
	Angle     float64   `json:"angle"`
	Distance  float64   `json:"distance"`
	Spread    float64   `json:"spread"`
	Size      float64   `json:"size"`
	Noise     float64   `json:"noise"`
	Knockout  bool      `json:"knockout"`
}

type InnerShadowParams struct {
	BlendMode BlendMode `json:"blendMode"`
	Color     [4]uint8  `json:"color"`
	Opacity   float64   `json:"opacity"`
	Angle     float64   `json:"angle"`
	Distance  float64   `json:"distance"`
	Choke     float64   `json:"choke"`
	Size      float64   `json:"size"`
	Noise     float64   `json:"noise"`
}

type GlowParams struct {
	BlendMode BlendMode `json:"blendMode"`
	Color     [4]uint8  `json:"color"`
	Opacity   float64   `json:"opacity"`
	Spread    float64   `json:"spread"`
	Size      float64   `json:"size"`
	Noise     float64   `json:"noise"`
}

type BevelEmbossParams struct {
	Style      string    `json:"style"`
	Technique  string    `json:"technique"`
	Depth      float64   `json:"depth"`
	Direction  string    `json:"direction"`
	Size       float64   `json:"size"`
	Soften     float64   `json:"soften"`
	Angle      float64   `json:"angle"`
	Altitude   float64   `json:"altitude"`
	Highlight  BlendMode `json:"highlightBlendMode"`
	HighlightC [4]uint8  `json:"highlightColor"`
	HighlightO float64   `json:"highlightOpacity"`
	Shadow     BlendMode `json:"shadowBlendMode"`
	ShadowC    [4]uint8  `json:"shadowColor"`
	ShadowO    float64   `json:"shadowOpacity"`
	Contour    string    `json:"contour"`
}

type SatinParams struct {
	BlendMode BlendMode `json:"blendMode"`
	Color     [4]uint8  `json:"color"`
	Opacity   float64   `json:"opacity"`
	Angle     float64   `json:"angle"`
	Distance  float64   `json:"distance"`
	Size      float64   `json:"size"`
	Invert    bool      `json:"invert"`
	Contour   string    `json:"contour"`
}

type ColorOverlayParams struct {
	BlendMode BlendMode `json:"blendMode"`
	Color     [4]uint8  `json:"color"`
	Opacity   float64   `json:"opacity"`
}

type GradientOverlayParams struct {
	BlendMode BlendMode `json:"blendMode"`
	Opacity   float64   `json:"opacity"`
	Angle     float64   `json:"angle"`
	Scale     float64   `json:"scale"`
	Reverse   bool      `json:"reverse"`
	Dither    bool      `json:"dither"`
	Align     bool      `json:"align"`
}

type PatternOverlayParams struct {
	BlendMode BlendMode `json:"blendMode"`
	Opacity   float64   `json:"opacity"`
	Scale     float64   `json:"scale"`
	Link      bool      `json:"link"`
}

type StrokeParams struct {
	Size      float64   `json:"size"`
	Position  string    `json:"position"`
	BlendMode BlendMode `json:"blendMode"`
	Opacity   float64   `json:"opacity"`
	Color     [4]uint8  `json:"color"`
	FillType  string    `json:"fillType"`
}

type DecodedLayerStyle struct {
	Kind            string
	Enabled         bool
	Raw             LayerStyle
	DropShadow      DropShadowParams
	InnerShadow     InnerShadowParams
	OuterGlow       GlowParams
	InnerGlow       GlowParams
	BevelEmboss     BevelEmbossParams
	Satin           SatinParams
	ColorOverlay    ColorOverlayParams
	GradientOverlay GradientOverlayParams
	PatternOverlay  PatternOverlayParams
	Stroke          StrokeParams
}

func decodeLayerStyles(styles []LayerStyle) []DecodedLayerStyle {
	decoded := make([]DecodedLayerStyle, 0, len(styles))
	for _, style := range styles {
		entry := DecodedLayerStyle{
			Kind:    style.Kind,
			Enabled: style.Enabled,
			Raw:     cloneLayerStyle(style),
		}
		switch LayerStyleKind(style.Kind) {
		case LayerStyleKindDropShadow:
			entry.DropShadow = decodeDropShadowParams(style.Params)
		case LayerStyleKindInnerShadow:
			entry.InnerShadow = decodeInnerShadowParams(style.Params)
		case LayerStyleKindOuterGlow:
			entry.OuterGlow = decodeOuterGlowParams(style.Params)
		case LayerStyleKindInnerGlow:
			entry.InnerGlow = decodeInnerGlowParams(style.Params)
		case LayerStyleKindBevelEmboss:
			entry.BevelEmboss = decodeBevelEmbossParams(style.Params)
		case LayerStyleKindSatin:
			entry.Satin = decodeSatinParams(style.Params)
		case LayerStyleKindColorOverlay:
			entry.ColorOverlay = decodeColorOverlayParams(style.Params)
		case LayerStyleKindGradientOverlay:
			entry.GradientOverlay = decodeGradientOverlayParams(style.Params)
		case LayerStyleKindPatternOverlay:
			entry.PatternOverlay = decodePatternOverlayParams(style.Params)
		case LayerStyleKindStroke:
			entry.Stroke = decodeStrokeParams(style.Params)
		default:
			entry.Enabled = false
		}
		decoded = append(decoded, entry)
	}
	return decoded
}

func decodeDropShadowParams(params json.RawMessage) DropShadowParams {
	decoded := defaultDropShadowParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeMultiply)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Distance = clampNonNegative(decoded.Distance)
	decoded.Spread = clampUnit(decoded.Spread)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Noise = clampUnit(decoded.Noise)
	return decoded
}

func decodeInnerShadowParams(params json.RawMessage) InnerShadowParams {
	decoded := defaultInnerShadowParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeMultiply)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Distance = clampNonNegative(decoded.Distance)
	decoded.Choke = clampUnit(decoded.Choke)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Noise = clampUnit(decoded.Noise)
	return decoded
}

func decodeOuterGlowParams(params json.RawMessage) GlowParams {
	decoded := defaultGlowParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeScreen)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Spread = clampUnit(decoded.Spread)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Noise = clampUnit(decoded.Noise)
	return decoded
}

func decodeInnerGlowParams(params json.RawMessage) GlowParams {
	decoded := defaultGlowParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeScreen)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Spread = clampUnit(decoded.Spread)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Noise = clampUnit(decoded.Noise)
	return decoded
}

func decodeBevelEmbossParams(params json.RawMessage) BevelEmbossParams {
	decoded := defaultBevelEmbossParams()
	decodeJSONInto(params, &decoded)
	decoded.Depth = clampNonNegative(decoded.Depth)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Soften = clampNonNegative(decoded.Soften)
	decoded.Highlight = normalizeBlendMode(decoded.Highlight, BlendModeScreen)
	decoded.HighlightO = clampUnit(decoded.HighlightO)
	decoded.Shadow = normalizeBlendMode(decoded.Shadow, BlendModeMultiply)
	decoded.ShadowO = clampUnit(decoded.ShadowO)
	return decoded
}

func decodeSatinParams(params json.RawMessage) SatinParams {
	decoded := defaultSatinParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeMultiply)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Distance = clampNonNegative(decoded.Distance)
	decoded.Size = clampNonNegative(decoded.Size)
	return decoded
}

func decodeColorOverlayParams(params json.RawMessage) ColorOverlayParams {
	decoded := defaultColorOverlayParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeNormal)
	decoded.Opacity = clampUnit(decoded.Opacity)
	return decoded
}

func decodeGradientOverlayParams(params json.RawMessage) GradientOverlayParams {
	decoded := defaultGradientOverlayParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeNormal)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Scale = clampNonNegative(decoded.Scale)
	return decoded
}

func decodePatternOverlayParams(params json.RawMessage) PatternOverlayParams {
	decoded := defaultPatternOverlayParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeNormal)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Scale = clampNonNegative(decoded.Scale)
	return decoded
}

func decodeStrokeParams(params json.RawMessage) StrokeParams {
	decoded := defaultStrokeParams()
	decodeJSONInto(params, &decoded)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeNormal)
	decoded.Opacity = clampUnit(decoded.Opacity)
	return decoded
}

func defaultDropShadowParams() DropShadowParams {
	return DropShadowParams{
		BlendMode: BlendModeMultiply,
		Color:     [4]uint8{0, 0, 0, 255},
		Opacity:   0.75,
		Angle:     120,
	}
}

func defaultInnerShadowParams() InnerShadowParams {
	return InnerShadowParams{
		BlendMode: BlendModeMultiply,
		Color:     [4]uint8{0, 0, 0, 255},
		Opacity:   0.75,
		Angle:     120,
	}
}

func defaultGlowParams() GlowParams {
	return GlowParams{
		BlendMode: BlendModeScreen,
		Color:     [4]uint8{255, 255, 255, 255},
		Opacity:   0.75,
	}
}

func defaultBevelEmbossParams() BevelEmbossParams {
	return BevelEmbossParams{
		Style:      "inner-bevel",
		Technique:  "smooth",
		Depth:      1,
		Direction:  "up",
		Angle:      120,
		Altitude:   30,
		Highlight:  BlendModeScreen,
		HighlightC: [4]uint8{255, 255, 255, 255},
		HighlightO: 0.75,
		Shadow:     BlendModeMultiply,
		ShadowC:    [4]uint8{0, 0, 0, 255},
		ShadowO:    0.75,
		Contour:    "linear",
	}
}

func defaultSatinParams() SatinParams {
	return SatinParams{
		BlendMode: BlendModeMultiply,
		Color:     [4]uint8{0, 0, 0, 255},
		Opacity:   0.5,
		Angle:     19,
		Contour:   "gaussian",
	}
}

func defaultColorOverlayParams() ColorOverlayParams {
	return ColorOverlayParams{
		BlendMode: BlendModeNormal,
		Color:     [4]uint8{0, 0, 0, 255},
		Opacity:   1,
	}
}

func defaultGradientOverlayParams() GradientOverlayParams {
	return GradientOverlayParams{
		BlendMode: BlendModeNormal,
		Opacity:   1,
		Angle:     90,
		Scale:     1,
		Align:     true,
	}
}

func defaultPatternOverlayParams() PatternOverlayParams {
	return PatternOverlayParams{
		BlendMode: BlendModeNormal,
		Opacity:   1,
		Scale:     1,
		Link:      true,
	}
}

func defaultStrokeParams() StrokeParams {
	return StrokeParams{
		Size:      1,
		Position:  "outside",
		BlendMode: BlendModeNormal,
		Opacity:   1,
		Color:     [4]uint8{0, 0, 0, 255},
		FillType:  "color",
	}
}

func decodeJSONInto(params json.RawMessage, target any) {
	if len(params) == 0 {
		return
	}
	value := reflect.ValueOf(target)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return
	}
	decoded := reflect.New(value.Elem().Type())
	decoded.Elem().Set(value.Elem())
	if err := json.Unmarshal(params, decoded.Interface()); err != nil {
		return
	}
	value.Elem().Set(decoded.Elem())
}

func normalizeBlendMode(mode BlendMode, fallback BlendMode) BlendMode {
	if !isValidBlendMode(mode) {
		return fallback
	}
	return mode
}

func clampNonNegative(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}
