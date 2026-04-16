package engine

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"

	agglib "github.com/cwbudde/agg_go"
)

type DocumentStylePreset struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Styles          []LayerStyle `json:"styles"`
	ThumbnailBase64 string       `json:"thumbnailBase64,omitempty"`
}

type CreateDocumentStylePresetPayload struct {
	Name   string              `json:"name"`
	Styles []LayerStylePayload `json:"styles"`
}

type UpdateDocumentStylePresetPayload struct {
	PresetID string              `json:"presetId"`
	Name     *string             `json:"name,omitempty"`
	Styles   []LayerStylePayload `json:"styles,omitempty"`
}

type DeleteDocumentStylePresetPayload struct {
	PresetID string `json:"presetId"`
}

type ApplyDocumentStylePresetPayload struct {
	PresetID string `json:"presetId"`
	LayerID  string `json:"layerId"`
}

func (doc *Document) CreateDocumentStylePreset(name string, styles []LayerStyle) (DocumentStylePreset, error) {
	if doc == nil {
		return DocumentStylePreset{}, fmt.Errorf("document is required")
	}
	if name == "" {
		return DocumentStylePreset{}, fmt.Errorf("preset name is required")
	}

	cloned := clonePresetStyles(styles)
	preset := DocumentStylePreset{
		ID:              newLayerID(),
		Name:            name,
		Styles:          cloned,
		ThumbnailBase64: renderPresetThumbnail(cloned),
	}
	doc.StylePresets = append(doc.StylePresets, preset)
	doc.touchModifiedAt()
	return preset, nil
}

func (doc *Document) UpdateDocumentStylePreset(presetID string, name *string, styles []LayerStyle) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}

	for i := range doc.StylePresets {
		if doc.StylePresets[i].ID != presetID {
			continue
		}
		if name != nil {
			if *name == "" {
				return fmt.Errorf("preset name is required")
			}
			doc.StylePresets[i].Name = *name
		}
		if styles != nil {
			cloned := clonePresetStyles(styles)
			doc.StylePresets[i].Styles = cloned
			doc.StylePresets[i].ThumbnailBase64 = renderPresetThumbnail(cloned)
		}
		doc.touchModifiedAt()
		return nil
	}

	return fmt.Errorf("style preset %q not found", presetID)
}

func (doc *Document) DeleteDocumentStylePreset(presetID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}

	for i := range doc.StylePresets {
		if doc.StylePresets[i].ID != presetID {
			continue
		}
		doc.StylePresets = append(doc.StylePresets[:i], doc.StylePresets[i+1:]...)
		doc.touchModifiedAt()
		return nil
	}

	return fmt.Errorf("style preset %q not found", presetID)
}

func (doc *Document) ApplyDocumentStylePreset(presetID, layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	if layerID == "" {
		return fmt.Errorf("layer id is required")
	}

	var preset *DocumentStylePreset
	for i := range doc.StylePresets {
		if doc.StylePresets[i].ID == presetID {
			preset = &doc.StylePresets[i]
			break
		}
	}
	if preset == nil {
		return fmt.Errorf("style preset %q not found", presetID)
	}

	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetStyleStack(clonePresetStyles(preset.Styles))
	doc.touchModifiedAt()
	return nil
}

func (inst *instance) createDocumentStylePreset(payload CreateDocumentStylePresetPayload) error {
	return inst.executeDocCommand("Create style preset", func(doc *Document) error {
		_, err := doc.CreateDocumentStylePreset(payload.Name, layerStylePayloadsToStyles(payload.Styles))
		return err
	})
}

func (inst *instance) updateDocumentStylePreset(payload UpdateDocumentStylePresetPayload) error {
	return inst.executeDocCommand("Update style preset", func(doc *Document) error {
		return doc.UpdateDocumentStylePreset(payload.PresetID, payload.Name, layerStylePayloadsToStyles(payload.Styles))
	})
}

func (inst *instance) deleteDocumentStylePreset(payload DeleteDocumentStylePresetPayload) error {
	return inst.executeDocCommand("Delete style preset", func(doc *Document) error {
		return doc.DeleteDocumentStylePreset(payload.PresetID)
	})
}

func (inst *instance) applyDocumentStylePreset(payload ApplyDocumentStylePresetPayload) error {
	return inst.executeDocCommand("Apply style preset", func(doc *Document) error {
		return doc.ApplyDocumentStylePreset(payload.PresetID, payload.LayerID)
	})
}

const presetThumbnailSize = 64

func renderPresetThumbnail(styles []LayerStyle) string {
	const size = presetThumbnailSize
	stride := size * 4

	sourceSurface := make([]byte, stride*size)
	r := agglib.NewAgg2D()
	r.Attach(sourceSurface, size, size, stride)
	r.ClearAll(agglib.NewColor(0, 0, 0, 0))
	r.ResetTransformations()
	r.ResetPath()

	const padding = 4.0
	const radius = 6.0
	swatch := makeRoundedRectPath(padding, padding, float64(size)-2*padding, float64(size)-2*padding, radius)

	r.FillColor(agglib.NewColor(160, 160, 160, 255))
	r.LineColor(agglib.NewColor(110, 110, 110, 255))
	r.LineWidth(1)
	applyPathToAgg2D(r, swatch)
	r.DrawPath(agglib.FillAndStroke)

	baseSurface := append([]byte(nil), sourceSurface...)
	decoded := decodeLayerStyles(styles)
	finalSurface := applyLayerStylesToSurface(baseSurface, sourceSurface, size, size, decoded)

	img := &image.NRGBA{
		Pix:    finalSurface,
		Stride: stride,
		Rect:   image.Rect(0, 0, size, size),
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
