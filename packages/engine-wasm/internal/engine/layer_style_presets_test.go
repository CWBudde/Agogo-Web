package engine

import (
	"encoding/json"
	"testing"
)

func TestDocumentStylePresetCommands_CreateUpdateDeleteApply(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Styled Layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels: []byte{
			255, 0, 0, 255,
		},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID
	if layerID == "" {
		t.Fatal("active layer id after add = empty, want created layer id")
	}

	_, err = DispatchCommand(h, commandSetLayerStyleStack, mustJSON(t, SetLayerStyleStackPayload{
		LayerID: layerID,
		Styles: []LayerStylePayload{
			{
				Kind:    LayerStyleKindColorOverlay,
				Enabled: true,
				Params:  json.RawMessage(`{"color":[0,255,0,255],"opacity":1}`),
			},
			{
				Kind:    LayerStyleKindStroke,
				Enabled: true,
				Params:  json.RawMessage(`{"size":4,"position":"inside"}`),
			},
		},
	}))
	if err != nil {
		t.Fatalf("seed layer style stack: %v", err)
	}

	inst := instances[h]
	historyBeforeCreate := inst.history.CurrentIndex()
	created, err := DispatchCommand(h, commandCreateDocumentStylePreset, mustJSON(t, CreateDocumentStylePresetPayload{
		Name: "Glow",
		Styles: []LayerStylePayload{
			{
				Kind:    LayerStyleKindOuterGlow,
				Enabled: true,
				Params:  json.RawMessage(`{"opacity":0.5}`),
			},
		},
	}))
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforeCreate+1 {
		t.Fatalf("create preset should add one history entry: got %d want %d", inst.history.CurrentIndex(), historyBeforeCreate+1)
	}
	if len(created.UIMeta.StylePresets) != 1 {
		t.Fatalf("style preset count after create = %d, want 1", len(created.UIMeta.StylePresets))
	}
	preset := created.UIMeta.StylePresets[0]
	if preset.ID == "" || preset.Name != "Glow" {
		t.Fatalf("created preset = %+v, want generated id and name Glow", preset)
	}
	if len(preset.Styles) != 1 || preset.Styles[0].Kind != string(LayerStyleKindOuterGlow) {
		t.Fatalf("created preset styles = %+v, want single outer-glow style", preset.Styles)
	}
	if !preset.Styles[0].Enabled {
		t.Fatalf("created preset enabled = %v, want true", preset.Styles[0].Enabled)
	}
	var createdParams map[string]any
	if err := json.Unmarshal(preset.Styles[0].Params, &createdParams); err != nil {
		t.Fatalf("decode created preset params: %v", err)
	}
	if createdParams["opacity"] != float64(0.5) {
		t.Fatalf("created preset params = %#v, want opacity 0.5", createdParams)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo create preset: %v", err)
	}
	if len(undone.UIMeta.StylePresets) != 0 {
		t.Fatalf("style presets after undo create = %+v, want empty slice", undone.UIMeta.StylePresets)
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo create preset: %v", err)
	}
	if len(redone.UIMeta.StylePresets) != 1 {
		t.Fatalf("style presets after redo create = %+v, want one preset", redone.UIMeta.StylePresets)
	}
	preset = redone.UIMeta.StylePresets[0]
	if preset.ID == "" || preset.Name != "Glow" {
		t.Fatalf("redone preset = %+v, want generated id and name Glow", preset)
	}

	historyBeforeApply := inst.history.CurrentIndex()
	applied, err := DispatchCommand(h, commandApplyDocumentStylePreset, mustJSON(t, ApplyDocumentStylePresetPayload{
		PresetID: preset.ID,
		LayerID:  layerID,
	}))
	if err != nil {
		t.Fatalf("apply preset: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforeApply+1 {
		t.Fatalf("apply preset should add one history entry: got %d want %d", inst.history.CurrentIndex(), historyBeforeApply+1)
	}
	if len(applied.UIMeta.Layers) != 1 {
		t.Fatalf("ui meta layers after apply = %d, want 1", len(applied.UIMeta.Layers))
	}
	if len(applied.UIMeta.Layers[0].StyleStack) != 1 || applied.UIMeta.Layers[0].StyleStack[0].Kind != string(LayerStyleKindOuterGlow) {
		t.Fatalf("applied layer style stack = %+v, want single outer-glow style", applied.UIMeta.Layers[0].StyleStack)
	}
	var appliedParams map[string]any
	if err := json.Unmarshal(applied.UIMeta.Layers[0].StyleStack[0].Params, &appliedParams); err != nil {
		t.Fatalf("decode applied preset params: %v", err)
	}
	if appliedParams["opacity"] != float64(0.5) {
		t.Fatalf("applied preset params = %#v, want opacity 0.5", appliedParams)
	}

	doc := inst.manager.Active()
	layer := doc.findLayer(layerID)
	if layer == nil {
		t.Fatalf("layer %q not found after apply", layerID)
	}
	wantApplied := []LayerStyle{{
		Kind:    string(LayerStyleKindOuterGlow),
		Enabled: true,
		Params:  json.RawMessage(`{"opacity":0.5}`),
	}}
	if got := layer.StyleStack(); !layerStylesEqual(got, wantApplied) {
		t.Fatalf("applied style stack = %#v, want %#v", got, wantApplied)
	}

	undone, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo apply preset: %v", err)
	}
	if !undone.UIMeta.CanUndo {
		t.Fatal("expected CanUndo to remain true after undoing preset apply")
	}
	doc = inst.manager.Active()
	layer = doc.findLayer(layerID)
	wantOriginal := []LayerStyle{
		{
			Kind:    string(LayerStyleKindColorOverlay),
			Enabled: true,
			Params:  json.RawMessage(`{"color":[0,255,0,255],"opacity":1}`),
		},
		{
			Kind:    string(LayerStyleKindStroke),
			Enabled: true,
			Params:  json.RawMessage(`{"size":4,"position":"inside"}`),
		},
	}
	if got := layer.StyleStack(); !layerStylesEqual(got, wantOriginal) {
		t.Fatalf("layer style stack after undo apply = %#v, want %#v", got, wantOriginal)
	}

	redone, err = DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo apply preset: %v", err)
	}
	if !redone.UIMeta.CanUndo {
		t.Fatal("expected CanUndo after redoing preset apply")
	}
	doc = inst.manager.Active()
	layer = doc.findLayer(layerID)
	if got := layer.StyleStack(); !layerStylesEqual(got, wantApplied) {
		t.Fatalf("layer style stack after redo apply = %#v, want %#v", got, wantApplied)
	}

	updatedName := "Outline"
	historyBeforeUpdate := inst.history.CurrentIndex()
	updated, err := DispatchCommand(h, commandUpdateDocumentStylePreset, mustJSON(t, UpdateDocumentStylePresetPayload{
		PresetID: preset.ID,
		Name:     &updatedName,
		Styles: []LayerStylePayload{
			{
				Kind:    LayerStyleKindStroke,
				Enabled: true,
				Params:  json.RawMessage(`{"size":2,"position":"outside"}`),
			},
		},
	}))
	if err != nil {
		t.Fatalf("update preset: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforeUpdate+1 {
		t.Fatalf("update preset should add one history entry: got %d want %d", inst.history.CurrentIndex(), historyBeforeUpdate+1)
	}
	if len(updated.UIMeta.StylePresets) != 1 {
		t.Fatalf("style preset count after update = %d, want 1", len(updated.UIMeta.StylePresets))
	}
	preset = updated.UIMeta.StylePresets[0]
	if preset.Name != updatedName {
		t.Fatalf("updated preset name = %q, want %q", preset.Name, updatedName)
	}
	if len(preset.Styles) != 1 || preset.Styles[0].Kind != string(LayerStyleKindStroke) {
		t.Fatalf("updated preset styles = %+v, want single stroke style", preset.Styles)
	}
	if !preset.Styles[0].Enabled {
		t.Fatalf("updated preset enabled = %v, want true", preset.Styles[0].Enabled)
	}
	var updatedParams map[string]any
	if err := json.Unmarshal(preset.Styles[0].Params, &updatedParams); err != nil {
		t.Fatalf("decode updated preset params: %v", err)
	}
	if updatedParams["size"] != float64(2) || updatedParams["position"] != "outside" {
		t.Fatalf("updated preset params = %#v, want size 2 / position outside", updatedParams)
	}

	undone, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo update preset: %v", err)
	}
	if len(undone.UIMeta.StylePresets) != 1 {
		t.Fatalf("style presets after undo update = %+v, want one preset", undone.UIMeta.StylePresets)
	}
	preset = undone.UIMeta.StylePresets[0]
	if preset.Name != "Glow" {
		t.Fatalf("preset name after undo update = %q, want Glow", preset.Name)
	}
	if len(preset.Styles) != 1 || preset.Styles[0].Kind != string(LayerStyleKindOuterGlow) {
		t.Fatalf("preset styles after undo update = %+v, want single outer-glow style", preset.Styles)
	}

	redone, err = DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo update preset: %v", err)
	}
	if len(redone.UIMeta.StylePresets) != 1 {
		t.Fatalf("style presets after redo update = %+v, want one preset", redone.UIMeta.StylePresets)
	}
	preset = redone.UIMeta.StylePresets[0]
	if preset.Name != updatedName {
		t.Fatalf("preset name after redo update = %q, want %q", preset.Name, updatedName)
	}
	if len(preset.Styles) != 1 || preset.Styles[0].Kind != string(LayerStyleKindStroke) {
		t.Fatalf("preset styles after redo update = %+v, want single stroke style", preset.Styles)
	}

	historyBeforeApply = inst.history.CurrentIndex()
	reapplied, err := DispatchCommand(h, commandApplyDocumentStylePreset, mustJSON(t, ApplyDocumentStylePresetPayload{
		PresetID: preset.ID,
		LayerID:  layerID,
	}))
	if err != nil {
		t.Fatalf("reapply updated preset: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforeApply+1 {
		t.Fatalf("reapply preset should add one history entry: got %d want %d", inst.history.CurrentIndex(), historyBeforeApply+1)
	}
	if len(reapplied.UIMeta.Layers) != 1 || len(reapplied.UIMeta.Layers[0].StyleStack) != 1 {
		t.Fatalf("reapplied layer style stack = %+v, want single style", reapplied.UIMeta.Layers)
	}
	reappliedStyle := reapplied.UIMeta.Layers[0].StyleStack[0]
	if reappliedStyle.Kind != string(LayerStyleKindStroke) || !reappliedStyle.Enabled {
		t.Fatalf("reapplied style = %+v, want enabled stroke", reappliedStyle)
	}
	var reappliedParams map[string]any
	if err := json.Unmarshal(reappliedStyle.Params, &reappliedParams); err != nil {
		t.Fatalf("decode reapplied preset params: %v", err)
	}
	if reappliedParams["size"] != float64(2) || reappliedParams["position"] != "outside" {
		t.Fatalf("reapplied preset params = %#v, want size 2 / position outside", reappliedParams)
	}

	historyBeforeDelete := inst.history.CurrentIndex()
	deleted, err := DispatchCommand(h, commandDeleteDocumentStylePreset, mustJSON(t, DeleteDocumentStylePresetPayload{
		PresetID: preset.ID,
	}))
	if err != nil {
		t.Fatalf("delete preset: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforeDelete+1 {
		t.Fatalf("delete preset should add one history entry: got %d want %d", inst.history.CurrentIndex(), historyBeforeDelete+1)
	}
	if len(deleted.UIMeta.StylePresets) != 0 {
		t.Fatalf("style presets after delete = %+v, want empty slice", deleted.UIMeta.StylePresets)
	}

	undone, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo delete preset: %v", err)
	}
	if len(undone.UIMeta.StylePresets) != 1 {
		t.Fatalf("style presets after undo delete = %+v, want one preset", undone.UIMeta.StylePresets)
	}

	redone, err = DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo delete preset: %v", err)
	}
	if len(redone.UIMeta.StylePresets) != 0 {
		t.Fatalf("style presets after redo delete = %+v, want empty slice", redone.UIMeta.StylePresets)
	}
}

func TestRenderUIMeta_ExposesLayerStyleStacksAndPresets(t *testing.T) {
	h := Init("")
	defer Free(h)

	doc, _, err := LoadProject([]byte(`{
		"version": 1,
		"document": {
			"width": 8,
			"height": 8,
			"resolution": 72,
			"colorMode": "rgb",
			"bitDepth": 8,
			"background": {"kind": "transparent"},
			"id": "doc-ui-presets",
			"name": "UI Presets",
			"createdAt": "2026-04-11T08:00:00Z",
			"createdBy": "agogo-web",
			"modifiedAt": "2026-04-11T08:00:00Z",
			"activeLayerId": "layer-styled",
			"stylePresets": [
				{
					"id": "preset-card",
					"name": "Card Glow",
					"styles": [
						{"kind": "outer-glow", "enabled": true, "params": {"opacity": 0.5}},
						{"kind": "stroke", "enabled": true, "params": {"size": 2}}
					]
				},
				{
					"id": "preset-empty",
					"name": "Empty Preset",
					"styles": []
				}
			],
			"layers": [
				{
					"id": "layer-styled",
					"layerType": "pixel",
					"name": "Styled Layer",
					"visible": true,
					"lockMode": "none",
					"opacity": 1,
					"fillOpacity": 1,
					"blendMode": "normal",
					"bounds": {"x": 0, "y": 0, "w": 1, "h": 1},
					"pixels": [255, 0, 0, 255],
					"styleStack": [
						{"kind": "color-overlay", "enabled": true, "params": {"color": [0, 255, 0, 255]}}
					]
				}
			]
		}
	}`))
	if err != nil {
		t.Fatalf("load project fixture: %v", err)
	}

	inst := instances[h]
	if err := inst.manager.ReplaceActive(doc); err != nil {
		t.Fatalf("ReplaceActive: %v", err)
	}
	activeDoc := inst.manager.activeMut()
	activeDoc.StylePresets = append(activeDoc.StylePresets, DocumentStylePreset{
		ID:     "preset-nil",
		Name:   "Nil Preset",
		Styles: nil,
	})

	uiMetaJSON, err := json.Marshal(inst.renderUIMeta())
	if err != nil {
		t.Fatalf("marshal ui meta: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(uiMetaJSON, &decoded); err != nil {
		t.Fatalf("decode ui meta: %v", err)
	}

	rawPresets, ok := decoded["stylePresets"].([]any)
	if !ok || len(rawPresets) != 3 {
		t.Fatalf("ui meta stylePresets = %#v, want three preset entries", decoded["stylePresets"])
	}
	firstPreset, ok := rawPresets[0].(map[string]any)
	if !ok {
		t.Fatalf("first ui preset = %#v, want object", rawPresets[0])
	}
	if firstPreset["id"] != "preset-card" || firstPreset["name"] != "Card Glow" {
		t.Fatalf("first ui preset = %#v, want preset-card / Card Glow", firstPreset)
	}
	firstPresetStyles, ok := firstPreset["styles"].([]any)
	if !ok || len(firstPresetStyles) != 2 {
		t.Fatalf("first ui preset styles = %#v, want two style entries", firstPreset["styles"])
	}
	firstPresetStyle, ok := firstPresetStyles[0].(map[string]any)
	if !ok {
		t.Fatalf("first preset style = %#v, want object", firstPresetStyles[0])
	}
	if firstPresetStyle["kind"] != "outer-glow" || firstPresetStyle["enabled"] != true {
		t.Fatalf("first preset style = %#v, want enabled outer-glow", firstPresetStyle)
	}
	firstPresetParams, ok := firstPresetStyle["params"].(map[string]any)
	if !ok || firstPresetParams["opacity"] != 0.5 {
		t.Fatalf("first preset params = %#v, want opacity 0.5", firstPresetStyle["params"])
	}
	secondPreset, ok := rawPresets[1].(map[string]any)
	if !ok {
		t.Fatalf("second ui preset = %#v, want object", rawPresets[1])
	}
	if secondPreset["id"] != "preset-empty" || secondPreset["name"] != "Empty Preset" {
		t.Fatalf("second ui preset = %#v, want preset-empty / Empty Preset", secondPreset)
	}
	secondPresetStyles, ok := secondPreset["styles"].([]any)
	if !ok || len(secondPresetStyles) != 0 {
		t.Fatalf("second ui preset styles = %#v, want explicit empty array", secondPreset["styles"])
	}
	thirdPreset, ok := rawPresets[2].(map[string]any)
	if !ok {
		t.Fatalf("third ui preset = %#v, want object", rawPresets[2])
	}
	if thirdPreset["id"] != "preset-nil" || thirdPreset["name"] != "Nil Preset" {
		t.Fatalf("third ui preset = %#v, want preset-nil / Nil Preset", thirdPreset)
	}
	thirdPresetStyles, ok := thirdPreset["styles"].([]any)
	if !ok || len(thirdPresetStyles) != 0 {
		t.Fatalf("third ui preset styles = %#v, want explicit empty array", thirdPreset["styles"])
	}

	layers, ok := decoded["layers"].([]any)
	if !ok || len(layers) != 1 {
		t.Fatalf("ui meta layers = %#v, want single layer entry", decoded["layers"])
	}

	layer, ok := layers[0].(map[string]any)
	if !ok {
		t.Fatalf("ui meta layer = %#v, want object", layers[0])
	}

	rawStyleStack, ok := layer["styleStack"].([]any)
	if !ok || len(rawStyleStack) != 1 {
		t.Fatalf("ui meta layer styleStack = %#v, want single style entry", layer["styleStack"])
	}
	styleEntry, ok := rawStyleStack[0].(map[string]any)
	if !ok {
		t.Fatalf("ui meta style entry = %#v, want object", rawStyleStack[0])
	}
	if styleEntry["kind"] != "color-overlay" || styleEntry["enabled"] != true {
		t.Fatalf("ui meta style entry = %#v, want enabled color-overlay", styleEntry)
	}
	styleParams, ok := styleEntry["params"].(map[string]any)
	if !ok {
		t.Fatalf("ui meta style params = %#v, want object", styleEntry["params"])
	}
	rawColor, ok := styleParams["color"].([]any)
	if !ok || len(rawColor) != 4 {
		t.Fatalf("ui meta style color = %#v, want rgba array", styleParams["color"])
	}
	if rawColor[0] != float64(0) || rawColor[1] != float64(255) || rawColor[2] != float64(0) || rawColor[3] != float64(255) {
		t.Fatalf("ui meta style color = %#v, want [0 255 0 255]", rawColor)
	}
}
