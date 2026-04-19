package psd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func BuildLayerEffectsPayload(styles []model.LayerStyle) []byte {
	filtered := make([]map[string]any, 0, len(styles))
	for _, style := range styles {
		token := effectDescriptorToken(style.Kind)
		if token == "" {
			continue
		}
		filtered = append(filtered, map[string]any{
			"token":   token,
			"kind":    style.Kind,
			"enabled": style.Enabled,
			"params":  string(style.Params),
		})
	}
	if len(filtered) == 0 {
		return nil
	}

	items := make([]DescriptorItem, 0, len(filtered)+1)
	items = append(items, DescriptorItem{Key: "masterFXSwitch", Type: "bool", Bool: true})
	for index, style := range filtered {
		items = append(items, DescriptorItem{
			Key:  style["token"].(string),
			Type: "TEXT",
			Text: marshalJSON(style),
		})
		items = append(items, DescriptorItem{
			Key:  fmt.Sprintf("fx%d", index),
			Type: "TEXT",
			Text: style["kind"].(string),
		})
	}

	var out bytes.Buffer
	writeUint32(&out, 0)
	writeUint32(&out, 16)
	WriteDescriptor(&out, "", "lfx2", items)
	return out.Bytes()
}

func BuildTextLayerPayload(layer TextLayerPayload) []byte {
	var out bytes.Buffer
	writeUint16(&out, 1)
	for _, value := range []float64{1, 0, 0, 1, 0, 0} {
		writeFloat64(&out, value)
	}
	writeUint16(&out, 50)
	writeUint32(&out, 16)
	textItems := []DescriptorItem{
		{Key: "Txt ", Type: "TEXT", Text: layer.Text},
		{Key: "font", Type: "TEXT", Text: layer.FontFamily},
		{Key: "fontStyle", Type: "TEXT", Text: layer.FontStyle},
		{Key: "antiAlias", Type: "TEXT", Text: layer.AntiAlias},
		{Key: "alignment", Type: "TEXT", Text: layer.Alignment},
		{Key: "textType", Type: "TEXT", Text: layer.TextType},
		{Key: "orientation", Type: "TEXT", Text: layer.Orientation},
		{Key: "fontSize", Type: "doub", Float64: layer.FontSize},
		{Key: "tracking", Type: "doub", Float64: layer.Tracking},
		{Key: "leading", Type: "doub", Float64: layer.Leading},
		{Key: "baselineShift", Type: "doub", Float64: layer.BaselineShift},
		{Key: "kerning", Type: "doub", Float64: layer.Kerning},
		{Key: "color", Type: "TEXT", Text: marshalJSON(layer.Color)},
		{Key: "styleJSON", Type: "TEXT", Text: marshalJSON(map[string]any{
			"bold":          layer.Bold,
			"italic":        layer.Italic,
			"language":      layer.Language,
			"superscript":   layer.Superscript,
			"subscript":     layer.Subscript,
			"underline":     layer.Underline,
			"strikethrough": layer.Strikethrough,
			"allCaps":       layer.AllCaps,
			"smallCaps":     layer.SmallCaps,
			"indentLeft":    layer.IndentLeft,
			"indentRight":   layer.IndentRight,
			"indentFirst":   layer.IndentFirst,
			"spaceBefore":   layer.SpaceBefore,
			"spaceAfter":    layer.SpaceAfter,
		})},
	}
	WriteDescriptor(&out, "", "TxLr", textItems)

	writeUint16(&out, 1)
	writeUint32(&out, 16)
	WriteDescriptor(&out, "", "warp", []DescriptorItem{
		{Key: "warpStyle", Type: "TEXT", Text: "warpNone"},
		{Key: "warpValue", Type: "doub", Float64: 0},
		{Key: "warpPerspective", Type: "doub", Float64: 0},
		{Key: "warpPerspectiveOther", Type: "doub", Float64: 0},
	})

	writeInt32(&out, int32(layer.Bounds.X))
	writeInt32(&out, int32(layer.Bounds.Y))
	writeInt32(&out, int32(layer.Bounds.X+layer.Bounds.W))
	writeInt32(&out, int32(layer.Bounds.Y+layer.Bounds.H))
	return out.Bytes()
}

func BuildAdjustmentLayerPayload(layer AdjustmentLayerPayload) []byte {
	payload := map[string]any{
		"kind": stringOrDefault(layer.Kind, ""),
		"params": func() any {
			if len(layer.Params) == 0 {
				return map[string]any{}
			}
			var parsed any
			if err := json.Unmarshal(layer.Params, &parsed); err == nil {
				return parsed
			}
			return string(layer.Params)
		}(),
	}
	var out bytes.Buffer
	writeUint16(&out, 1)
	out.WriteString(marshalJSON(payload))
	return out.Bytes()
}

func effectDescriptorToken(kind string) string {
	switch kind {
	case "drop-shadow":
		return "dropshadow"
	case "inner-shadow":
		return "innershadow"
	case "outer-glow":
		return "outerglow"
	case "inner-glow":
		return "innerglow"
	case "bevel-emboss":
		return "bevelemboss"
	case "stroke":
		return "strokestyle"
	case "color-overlay":
		return "coloroverlay"
	case "gradient-overlay":
		return "gradientoverlay"
	case "pattern-overlay":
		return "patternoverlay"
	case "satin":
		return "satin"
	default:
		return ""
	}
}

func marshalJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func stringOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
