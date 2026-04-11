package engine

import (
	"encoding/json"
	"testing"
)

func TestProjectArchiveRoundTripPreservesDocument(t *testing.T) {
	doc := &Document{
		Width:         2,
		Height:        1,
		Resolution:    300,
		ColorMode:     "rgb",
		BitDepth:      8,
		Background:    parseBackground("white"),
		ID:            "doc-test",
		Name:          "Archive Test",
		CreatedAt:     "2026-03-27T10:00:00Z",
		CreatedBy:     "agogo-web",
		ModifiedAt:    "2026-03-27T10:05:00Z",
		ActiveLayerID: "layer-top",
		LayerRoot:     NewGroupLayer("Root"),
		StylePresets: []DocumentStylePreset{
			{
				ID:   "preset-outline",
				Name: "Outline",
				Styles: []LayerStyle{
					{
						Kind:    string(LayerStyleKindStroke),
						Enabled: true,
						Params:  jsonRawMessage(`{"size":2,"position":"outside","opacity":0.8}`),
					},
				},
			},
			{
				ID:     "preset-empty",
				Name:   "Empty",
				Styles: []LayerStyle{},
			},
		},
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		0, 0, 255, 255,
		0, 0, 255, 0,
	})
	base.SetMask(newFilledLayerMask(2, 1, 255))
	base.SetStyleStack([]LayerStyle{
		{
			Kind:    string(LayerStyleKindStroke),
			Enabled: true,
			Params:  jsonRawMessage(`{"size":2,"position":"outside","fillType":"color","color":[255,255,0,255],"opacity":0.8}`),
		},
		{
			Kind:    string(LayerStyleKindDropShadow),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"multiply","opacity":0.75,"angle":120,"distance":4,"spread":0.1,"size":6}`),
		},
	})
	top := NewPixelLayer("Top", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	top.SetClipToBelow(true)
	group := NewGroupLayer("Group")
	group.Isolated = true
	group.SetChildren([]LayerNode{base, top})
	doc.LayerRoot.SetChildren([]LayerNode{group})
	doc.normalizeClippingState()
	doc.ActiveLayerID = top.ID()

	archive, err := SaveProject(doc, []HistoryEntry{{ID: 1, Description: "Added layer", State: "done"}})
	if err != nil {
		t.Fatalf("save project: %v", err)
	}

	restored, history, err := LoadProject(archive)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if restored.Width != doc.Width || restored.Height != doc.Height || restored.Resolution != doc.Resolution {
		t.Fatalf("restored document metadata mismatch: got %+v want %+v", restored, doc)
	}
	if restored.ColorMode != doc.ColorMode || restored.BitDepth != doc.BitDepth || restored.Background != doc.Background {
		t.Fatalf("restored document settings mismatch: got %+v want %+v", restored, doc)
	}
	if restored.ID != doc.ID || restored.Name != doc.Name || restored.CreatedAt != doc.CreatedAt || restored.ModifiedAt != doc.ModifiedAt {
		t.Fatalf("restored document identity mismatch: got %+v want %+v", restored, doc)
	}
	if restored.ActiveLayerID != doc.ActiveLayerID {
		t.Fatalf("restored active layer mismatch: got %q want %q", restored.ActiveLayerID, doc.ActiveLayerID)
	}
	if !documentStylePresetsEqual(restored.StylePresets, doc.StylePresets) {
		t.Fatalf("restored style presets mismatch: got %+v want %+v", restored.StylePresets, doc.StylePresets)
	}
	originalChildren := doc.LayerRoot.Children()
	restoredChildren := restored.LayerRoot.Children()
	if len(originalChildren) != len(restoredChildren) {
		t.Fatalf("restored child count mismatch: got %d want %d", len(restoredChildren), len(originalChildren))
	}
	for index := range originalChildren {
		if !layerTreeEqual(originalChildren[index], restoredChildren[index]) {
			t.Fatalf("restored child %d did not match original", index)
		}
	}
	if len(history) != 1 || history[0].Description != "Added layer" {
		t.Fatalf("restored history = %+v", history)
	}
}

func TestProjectArchiveRoundTripPreservesDocumentStylePresets(t *testing.T) {
	input := []byte(`{
		"version": 1,
		"document": {
			"width": 4,
			"height": 4,
			"resolution": 72,
			"colorMode": "rgb",
			"bitDepth": 8,
			"background": {"kind": "transparent"},
			"id": "doc-presets",
			"name": "Preset Archive",
			"createdAt": "2026-04-11T08:00:00Z",
			"createdBy": "agogo-web",
			"modifiedAt": "2026-04-11T08:00:00Z",
			"stylePresets": [
				{
					"id": "preset-outline",
					"name": "Outline",
					"styles": [
						{"kind": "stroke", "enabled": true, "params": {"size": 3, "position": "outside"}}
					]
				},
				{
					"id": "preset-glow",
					"name": "Glow",
					"styles": [
						{"kind": "outer-glow", "enabled": true, "params": {"opacity": 0.75}}
					]
				},
				{
					"id": "preset-empty",
					"name": "Empty",
					"styles": []
				}
			],
			"layers": [
				{
					"id": "layer-base",
					"layerType": "pixel",
					"name": "Base",
					"visible": true,
					"lockMode": "none",
					"opacity": 1,
					"fillOpacity": 1,
					"blendMode": "normal",
					"bounds": {"x": 0, "y": 0, "w": 1, "h": 1},
					"pixels": [255, 0, 0, 255]
				}
			]
		}
	}`)

	doc, _, err := LoadProject(input)
	if err != nil {
		t.Fatalf("load project with style presets: %v", err)
	}

	saved, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatalf("save project with style presets: %v", err)
	}

	gotPresets := archiveStylePresetsFromJSON(t, saved)
	if len(gotPresets) != 3 {
		t.Fatalf("saved style preset count = %d, want 3", len(gotPresets))
	}

	wantOrder := []string{"preset-outline", "preset-glow", "preset-empty"}
	for i, wantID := range wantOrder {
		if gotPresets[i]["id"] != wantID {
			t.Fatalf("saved style preset order[%d] = %#v, want %q", i, gotPresets[i]["id"], wantID)
		}
	}

	if gotPresets[0]["id"] != "preset-outline" || gotPresets[0]["name"] != "Outline" {
		t.Fatalf("first saved style preset = %+v, want preset-outline / Outline", gotPresets[0])
	}
	firstStyles, ok := gotPresets[0]["styles"].([]any)
	if !ok || len(firstStyles) != 1 {
		t.Fatalf("first saved preset styles = %#v, want single style entry", gotPresets[0]["styles"])
	}
	firstStyle, ok := firstStyles[0].(map[string]any)
	if !ok {
		t.Fatalf("first saved preset style = %#v, want object", firstStyles[0])
	}
	if firstStyle["kind"] != "stroke" || firstStyle["enabled"] != true {
		t.Fatalf("first saved preset style = %#v, want enabled stroke", firstStyle)
	}
	firstParams, ok := firstStyle["params"].(map[string]any)
	if !ok || firstParams["size"] != float64(3) || firstParams["position"] != "outside" {
		t.Fatalf("first saved preset params = %#v, want size 3 / position outside", firstStyle["params"])
	}

	if gotPresets[1]["id"] != "preset-glow" || gotPresets[1]["name"] != "Glow" {
		t.Fatalf("second saved style preset = %+v, want preset-glow / Glow", gotPresets[1])
	}
	secondStyles, ok := gotPresets[1]["styles"].([]any)
	if !ok || len(secondStyles) != 1 {
		t.Fatalf("second saved preset styles = %#v, want single style entry", gotPresets[1]["styles"])
	}
	secondStyle, ok := secondStyles[0].(map[string]any)
	if !ok {
		t.Fatalf("second saved preset style = %#v, want object", secondStyles[0])
	}
	secondParams, ok := secondStyle["params"].(map[string]any)
	if !ok || secondStyle["kind"] != "outer-glow" || secondParams["opacity"] != 0.75 {
		t.Fatalf("second saved preset = %#v, want outer-glow opacity 0.75", secondStyle)
	}

	if gotPresets[2]["id"] != "preset-empty" || gotPresets[2]["name"] != "Empty" {
		t.Fatalf("third saved style preset = %+v, want preset-empty / Empty", gotPresets[2])
	}
	thirdStyles, ok := gotPresets[2]["styles"].([]any)
	if !ok || len(thirdStyles) != 0 {
		t.Fatalf("third saved preset styles = %#v, want explicit empty array", gotPresets[2]["styles"])
	}

	roundTripped, _, err := LoadProject(saved)
	if err != nil {
		t.Fatalf("load saved project with style presets: %v", err)
	}
	wantPresets := []DocumentStylePreset{
		{
			ID:   "preset-outline",
			Name: "Outline",
			Styles: []LayerStyle{
				{
					Kind:    string(LayerStyleKindStroke),
					Enabled: true,
					Params:  jsonRawMessage(`{"size":3,"position":"outside"}`),
				},
			},
		},
		{
			ID:   "preset-glow",
			Name: "Glow",
			Styles: []LayerStyle{
				{
					Kind:    string(LayerStyleKindOuterGlow),
					Enabled: true,
					Params:  jsonRawMessage(`{"opacity":0.75}`),
				},
			},
		},
		{
			ID:     "preset-empty",
			Name:   "Empty",
			Styles: []LayerStyle{},
		},
	}
	if !documentStylePresetsEqual(roundTripped.StylePresets, wantPresets) {
		t.Fatalf("round-tripped style presets = %+v, want %+v", roundTripped.StylePresets, wantPresets)
	}

	doc.StylePresets = append(doc.StylePresets, DocumentStylePreset{
		ID:     "preset-nil",
		Name:   "Nil",
		Styles: nil,
	})
	savedWithNil, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatalf("save project with nil preset styles: %v", err)
	}
	presetsWithNil := archiveStylePresetsFromJSON(t, savedWithNil)
	if len(presetsWithNil) != 4 {
		t.Fatalf("saved style preset count with nil entry = %d, want 4", len(presetsWithNil))
	}
	nilStyles, ok := presetsWithNil[3]["styles"].([]any)
	if !ok || len(nilStyles) != 0 {
		t.Fatalf("nil-style preset serialized styles = %#v, want explicit empty array", presetsWithNil[3]["styles"])
	}
}

func jsonRawMessage(value string) json.RawMessage {
	return json.RawMessage(value)
}

func archiveStylePresetsFromJSON(t *testing.T, data []byte) []map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode archive json: %v", err)
	}

	document, ok := decoded["document"].(map[string]any)
	if !ok {
		t.Fatalf("archive document = %#v, want object", decoded["document"])
	}

	rawPresets, ok := document["stylePresets"].([]any)
	if !ok {
		t.Fatalf("archive stylePresets = %#v, want array", document["stylePresets"])
	}

	presets := make([]map[string]any, 0, len(rawPresets))
	for _, rawPreset := range rawPresets {
		preset, ok := rawPreset.(map[string]any)
		if !ok {
			t.Fatalf("archive preset entry = %#v, want object", rawPreset)
		}
		presets = append(presets, preset)
	}
	return presets
}
