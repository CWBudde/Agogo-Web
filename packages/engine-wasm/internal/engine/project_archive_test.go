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

func jsonRawMessage(value string) json.RawMessage {
	return json.RawMessage(value)
}
