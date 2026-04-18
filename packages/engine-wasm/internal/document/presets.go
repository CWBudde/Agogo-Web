package document

import "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"

type DocumentStylePreset struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Styles          []model.LayerStyle `json:"styles"`
	ThumbnailBase64 string             `json:"thumbnailBase64,omitempty"`
}

func CloneDocumentStylePresets(presets []DocumentStylePreset) []DocumentStylePreset {
	if presets == nil {
		return nil
	}
	cloned := make([]DocumentStylePreset, len(presets))
	for i := range presets {
		cloned[i] = DocumentStylePreset{
			ID:              presets[i].ID,
			Name:            presets[i].Name,
			Styles:          ClonePresetStyles(presets[i].Styles),
			ThumbnailBase64: presets[i].ThumbnailBase64,
		}
	}
	return cloned
}

func ClonePresetStyles(styles []model.LayerStyle) []model.LayerStyle {
	if styles == nil {
		return []model.LayerStyle{}
	}
	cloned := model.CloneLayerStyles(styles)
	if cloned == nil {
		return []model.LayerStyle{}
	}
	return cloned
}

func DocumentStylePresetsEqual(a, b []DocumentStylePreset) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Name != b[i].Name || !model.LayerStylesEqual(a[i].Styles, b[i].Styles) {
			return false
		}
	}
	return true
}
