package document

import "bytes"

// BrushTipResource is a sampled grayscale brush tip owned by a document.
// Alpha contains one byte per pixel in row-major order.
type BrushTipResource struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Alpha  []byte `json:"alpha,omitempty"`
}

// ImportedBrushPreset is the engine-provided metadata associated with an
// imported sampled tip. Documents retain this metadata alongside the tip so a
// portable project does not depend on the original ABR file.
type ImportedBrushPreset struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	TipResourceID string   `json:"tipResourceId,omitempty"`
	ThumbnailRGBA string   `json:"thumbnailRGBA,omitempty"`
	TipShape      string   `json:"tipShape"`
	Size          float64  `json:"size"`
	Hardness      float64  `json:"hardness"`
	Spacing       float64  `json:"spacing"`
	Angle         float64  `json:"angle"`
	Roundness     float64  `json:"roundness"`
	SizeJitter    float64  `json:"sizeJitter"`
	OpacityJitter float64  `json:"opacityJitter"`
	FlowJitter    float64  `json:"flowJitter"`
	ControlSource string   `json:"controlSource"`
	FadeDabs      int      `json:"fadeDabs"`
	Warnings      []string `json:"warnings,omitempty"`
}

// CloneBrushTipResource deep-copies a sampled brush tip.
func CloneBrushTipResource(resource *BrushTipResource) *BrushTipResource {
	if resource == nil {
		return nil
	}
	cloned := *resource
	cloned.Alpha = append([]byte(nil), resource.Alpha...)
	return &cloned
}

// CloneBrushTipResources deep-copies a document's sampled-tip registry.
func CloneBrushTipResources(resources map[string]*BrushTipResource) map[string]*BrushTipResource {
	if resources == nil {
		return nil
	}
	cloned := make(map[string]*BrushTipResource, len(resources))
	for id, resource := range resources {
		cloned[id] = CloneBrushTipResource(resource)
	}
	return cloned
}

// BrushTipResourcesEqual reports whether two sampled-tip registries have the
// same metadata and alpha bytes.
func BrushTipResourcesEqual(a, b map[string]*BrushTipResource) bool {
	if len(a) != len(b) {
		return false
	}
	for id, left := range a {
		right, ok := b[id]
		if !ok || (left == nil) != (right == nil) {
			return false
		}
		if left != nil && (left.ID != right.ID || left.Width != right.Width || left.Height != right.Height || !bytes.Equal(left.Alpha, right.Alpha)) {
			return false
		}
	}
	return true
}

// CloneImportedBrushPreset deep-copies imported preset metadata.
func CloneImportedBrushPreset(preset ImportedBrushPreset) ImportedBrushPreset {
	cloned := preset
	cloned.Warnings = append([]string(nil), preset.Warnings...)
	return cloned
}

// CloneImportedBrushPresets deep-copies a document's imported preset registry.
func CloneImportedBrushPresets(presets map[string]ImportedBrushPreset) map[string]ImportedBrushPreset {
	if presets == nil {
		return nil
	}
	cloned := make(map[string]ImportedBrushPreset, len(presets))
	for id, preset := range presets {
		cloned[id] = CloneImportedBrushPreset(preset)
	}
	return cloned
}

// ImportedBrushPresetsEqual reports whether two imported preset registries are
// identical, including warnings and preview metadata.
func ImportedBrushPresetsEqual(a, b map[string]ImportedBrushPreset) bool {
	if len(a) != len(b) {
		return false
	}
	for id, left := range a {
		right, ok := b[id]
		if !ok || left.ID != right.ID || left.Name != right.Name || left.TipResourceID != right.TipResourceID || left.ThumbnailRGBA != right.ThumbnailRGBA ||
			left.TipShape != right.TipShape || left.Size != right.Size || left.Hardness != right.Hardness || left.Spacing != right.Spacing || left.Angle != right.Angle || left.Roundness != right.Roundness ||
			left.SizeJitter != right.SizeJitter || left.OpacityJitter != right.OpacityJitter || left.FlowJitter != right.FlowJitter || left.ControlSource != right.ControlSource || left.FadeDabs != right.FadeDabs {
			return false
		}
		if len(left.Warnings) != len(right.Warnings) {
			return false
		}
		for index := range left.Warnings {
			if left.Warnings[index] != right.Warnings[index] {
				return false
			}
		}
	}
	return true
}
