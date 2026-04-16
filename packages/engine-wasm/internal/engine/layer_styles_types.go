package engine

import (
	"encoding/json"
	"reflect"
)

// BlendIfChannel models a Photoshop "Blend If" channel slider.
// Values are [lowHard, lowSoft, highSoft, highHard] in the 0..255 domain.
// A hard cutoff has lowHard == lowSoft and highSoft == highHard.
// Alt-dragging a handle in the UI splits it, producing a smooth fade
// between the hard and soft values.
type BlendIfChannel [4]float64

type BlendIfRange struct {
	Gray  BlendIfChannel `json:"gray"`
	Red   BlendIfChannel `json:"red"`
	Green BlendIfChannel `json:"green"`
	Blue  BlendIfChannel `json:"blue"`
}

type BlendChannelsMask struct {
	R bool `json:"r"`
	G bool `json:"g"`
	B bool `json:"b"`
}

type BlendIfConfig struct {
	ThisLayer       BlendIfRange      `json:"thisLayer"`
	UnderlyingLayer BlendIfRange      `json:"underlyingLayer"`
	Channels        BlendChannelsMask `json:"channels"`
}

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
	decoded.Color = normalizeRGBAField(params, "color", decoded.Color, defaultDropShadowParams().Color)
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
	decoded.Color = normalizeRGBAField(params, "color", decoded.Color, defaultInnerShadowParams().Color)
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
	decoded.Color = normalizeRGBAField(params, "color", decoded.Color, defaultGlowParams().Color)
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
	decoded.Color = normalizeRGBAField(params, "color", decoded.Color, defaultGlowParams().Color)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Spread = clampUnit(decoded.Spread)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Noise = clampUnit(decoded.Noise)
	return decoded
}

func decodeBevelEmbossParams(params json.RawMessage) BevelEmbossParams {
	decoded := defaultBevelEmbossParams()
	decodeJSONInto(params, &decoded)
	decoded.Style = normalizeStringEnum(decoded.Style, defaultBevelEmbossParams().Style, "inner-bevel", "outer-bevel", "emboss", "pillow-emboss", "stroke-emboss")
	decoded.Technique = normalizeStringEnum(decoded.Technique, defaultBevelEmbossParams().Technique, "smooth", "chisel-hard", "chisel-soft")
	decoded.Depth = clampNonNegative(decoded.Depth)
	decoded.Direction = normalizeStringEnum(decoded.Direction, defaultBevelEmbossParams().Direction, "up", "down")
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Soften = clampNonNegative(decoded.Soften)
	decoded.Highlight = normalizeBlendMode(decoded.Highlight, BlendModeScreen)
	decoded.HighlightC = normalizeRGBAField(params, "highlightColor", decoded.HighlightC, defaultBevelEmbossParams().HighlightC)
	decoded.HighlightO = clampUnit(decoded.HighlightO)
	decoded.Shadow = normalizeBlendMode(decoded.Shadow, BlendModeMultiply)
	decoded.ShadowC = normalizeRGBAField(params, "shadowColor", decoded.ShadowC, defaultBevelEmbossParams().ShadowC)
	decoded.ShadowO = clampUnit(decoded.ShadowO)
	decoded.Contour = normalizeStringEnum(decoded.Contour, defaultBevelEmbossParams().Contour, "linear", "gaussian", "cone", "rolling-slope", "rounded-steps")
	return decoded
}

func decodeSatinParams(params json.RawMessage) SatinParams {
	decoded := defaultSatinParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeMultiply)
	decoded.Color = normalizeRGBAField(params, "color", decoded.Color, defaultSatinParams().Color)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Distance = clampNonNegative(decoded.Distance)
	decoded.Size = clampNonNegative(decoded.Size)
	decoded.Contour = normalizeStringEnum(decoded.Contour, defaultSatinParams().Contour, "gaussian", "linear", "cone", "rolling-slope")
	return decoded
}

func decodeColorOverlayParams(params json.RawMessage) ColorOverlayParams {
	decoded := defaultColorOverlayParams()
	decodeJSONInto(params, &decoded)
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeNormal)
	decoded.Color = normalizeRGBAField(params, "color", decoded.Color, defaultColorOverlayParams().Color)
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
	decoded.Position = normalizeStringEnum(decoded.Position, defaultStrokeParams().Position, "outside", "inside", "center")
	decoded.BlendMode = normalizeBlendMode(decoded.BlendMode, BlendModeNormal)
	decoded.Opacity = clampUnit(decoded.Opacity)
	decoded.Color = normalizeRGBAField(params, "color", decoded.Color, defaultStrokeParams().Color)
	decoded.FillType = normalizeStringEnum(decoded.FillType, defaultStrokeParams().FillType, "color", "gradient", "pattern")
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

func defaultBlendIfChannel() BlendIfChannel {
	return BlendIfChannel{0, 0, 255, 255}
}

func defaultBlendIfRange() BlendIfRange {
	return BlendIfRange{
		Gray:  defaultBlendIfChannel(),
		Red:   defaultBlendIfChannel(),
		Green: defaultBlendIfChannel(),
		Blue:  defaultBlendIfChannel(),
	}
}

func defaultBlendChannelsMask() BlendChannelsMask {
	return BlendChannelsMask{R: true, G: true, B: true}
}

func defaultBlendIfConfig() *BlendIfConfig {
	return &BlendIfConfig{
		ThisLayer:       defaultBlendIfRange(),
		UnderlyingLayer: defaultBlendIfRange(),
		Channels:        defaultBlendChannelsMask(),
	}
}

func cloneBlendIfConfig(config *BlendIfConfig) *BlendIfConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	return &cloned
}

func normalizeBlendIfConfig(config *BlendIfConfig) *BlendIfConfig {
	if config == nil {
		return defaultBlendIfConfig()
	}
	normalized := *config
	normalized.ThisLayer = normalizeBlendIfRange(normalized.ThisLayer)
	normalized.UnderlyingLayer = normalizeBlendIfRange(normalized.UnderlyingLayer)
	return &normalized
}

func normalizeBlendIfRange(r BlendIfRange) BlendIfRange {
	return BlendIfRange{
		Gray:  normalizeBlendIfChannel(r.Gray),
		Red:   normalizeBlendIfChannel(r.Red),
		Green: normalizeBlendIfChannel(r.Green),
		Blue:  normalizeBlendIfChannel(r.Blue),
	}
}

func normalizeBlendIfChannel(channel BlendIfChannel) BlendIfChannel {
	// Clamp each value to 0..255 then enforce ordering
	// lowHard <= lowSoft <= highSoft <= highHard.
	values := [4]float64{
		clampBlendIfValue(channel[0]),
		clampBlendIfValue(channel[1]),
		clampBlendIfValue(channel[2]),
		clampBlendIfValue(channel[3]),
	}
	if values[1] < values[0] {
		values[1] = values[0]
	}
	if values[2] < values[1] {
		values[2] = values[1]
	}
	if values[3] < values[2] {
		values[3] = values[2]
	}
	return BlendIfChannel(values)
}

func clampBlendIfValue(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}

// UnmarshalJSON accepts both the current BlendIfConfig shape and the legacy
// shape from project archives ({"gray":[min,max], ...}) so older documents
// continue to load.
func (c *BlendIfConfig) UnmarshalJSON(data []byte) error {
	type newShape struct {
		ThisLayer       BlendIfRange      `json:"thisLayer"`
		UnderlyingLayer BlendIfRange      `json:"underlyingLayer"`
		Channels        BlendChannelsMask `json:"channels"`
	}
	type legacyShape struct {
		Gray  [2]float64 `json:"gray"`
		Red   [2]float64 `json:"red"`
		Green [2]float64 `json:"green"`
		Blue  [2]float64 `json:"blue"`
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	if _, ok := probe["thisLayer"]; ok {
		var parsed newShape
		if err := json.Unmarshal(data, &parsed); err != nil {
			return err
		}
		c.ThisLayer = parsed.ThisLayer
		c.UnderlyingLayer = parsed.UnderlyingLayer
		c.Channels = parsed.Channels
		if _, present := probe["channels"]; !present {
			c.Channels = defaultBlendChannelsMask()
		}
		return nil
	}

	var legacy legacyShape
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	c.ThisLayer = BlendIfRange{
		Gray:  BlendIfChannel{legacy.Gray[0], legacy.Gray[0], legacy.Gray[1], legacy.Gray[1]},
		Red:   BlendIfChannel{legacy.Red[0], legacy.Red[0], legacy.Red[1], legacy.Red[1]},
		Green: BlendIfChannel{legacy.Green[0], legacy.Green[0], legacy.Green[1], legacy.Green[1]},
		Blue:  BlendIfChannel{legacy.Blue[0], legacy.Blue[0], legacy.Blue[1], legacy.Blue[1]},
	}
	c.UnderlyingLayer = defaultBlendIfRange()
	c.Channels = defaultBlendChannelsMask()
	return nil
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

func normalizeStringEnum(value, fallback string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

func normalizeRGBAField(params json.RawMessage, key string, current, fallback [4]uint8) [4]uint8 {
	if len(params) == 0 {
		return current
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(params, &raw); err != nil {
		return fallback
	}
	value, ok := raw[key]
	if !ok {
		return current
	}
	var rgba []uint8
	if err := json.Unmarshal(value, &rgba); err != nil || len(rgba) != 4 {
		return fallback
	}
	return [4]uint8{rgba[0], rgba[1], rgba[2], rgba[3]}
}

func clampNonNegative(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}
