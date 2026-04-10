package engine

import (
	"encoding/json"
	"fmt"
)

type styleClipboard struct {
	styles   []LayerStyle
	hasValue bool
}

func (doc *Document) SetLayerStyleStack(layerID string, styles []LayerStyle) error {
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetStyleStack(styles)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerStyleEnabled(layerID string, kind LayerStyleKind, enabled bool) error {
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}

	styles := layer.StyleStack()
	for i := range styles {
		if styles[i].Kind == string(kind) {
			styles[i].Enabled = enabled
			layer.SetStyleStack(styles)
			doc.touchModifiedAt()
			return nil
		}
	}

	return fmt.Errorf("layer style %q not found on layer %q", kind, layerID)
}

func (doc *Document) SetLayerStyleParams(layerID string, kind LayerStyleKind, params json.RawMessage) error {
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}

	styles := layer.StyleStack()
	for i := range styles {
		if styles[i].Kind == string(kind) {
			styles[i].Params = cloneJSONRawMessage(params)
			layer.SetStyleStack(styles)
			doc.touchModifiedAt()
			return nil
		}
	}

	return fmt.Errorf("layer style %q not found on layer %q", kind, layerID)
}

func (doc *Document) ClearLayerStyle(layerID string) error {
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetStyleStack(nil)
	doc.touchModifiedAt()
	return nil
}

func (inst *instance) copyLayerStyle(doc *Document, layerID string) error {
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	inst.styleClipboard.styles = cloneLayerStyles(layer.StyleStack())
	inst.styleClipboard.hasValue = true
	return nil
}

func (inst *instance) pasteLayerStyle(doc *Document, layerID string) error {
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	if !inst.styleClipboard.hasValue {
		return fmt.Errorf("no layer styles copied")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetStyleStack(inst.styleClipboard.styles)
	doc.touchModifiedAt()
	return nil
}
