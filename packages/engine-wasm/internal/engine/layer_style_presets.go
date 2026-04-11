package engine

import "fmt"

type DocumentStylePreset struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Styles []LayerStyle `json:"styles"`
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

	preset := DocumentStylePreset{
		ID:     newLayerID(),
		Name:   name,
		Styles: clonePresetStyles(styles),
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
			doc.StylePresets[i].Styles = clonePresetStyles(styles)
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
