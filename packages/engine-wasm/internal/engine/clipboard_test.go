package engine

import (
	"bytes"
	"strings"
	"testing"
)

func TestClipboardCopyPasteSelectionAndHistory(t *testing.T) {
	h := Init(mustJSON(t, EngineConfig{DocumentWidth: 4, DocumentHeight: 3, Background: "transparent"}))
	defer Free(h)

	pixels := []byte{
		255, 0, 0, 255, 0, 255, 0, 128,
		0, 0, 255, 255, 255, 255, 0, 64,
	}
	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Source",
		Bounds:    LayerBounds{X: 1, Y: 1, W: 2, H: 2},
		Pixels:    pixels,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	sourceID := added.UIMeta.ActiveLayerID
	if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Mode:  SelectionCombineReplace,
		Rect:  LayerBounds{X: 1, Y: 1, W: 1, H: 2},
	})); err != nil {
		t.Fatalf("select: %v", err)
	}
	if _, err := DispatchCommand(h, commandSetLayerVis, mustJSON(t, SetLayerVisibilityPayload{
		LayerID: sourceID,
		Visible: false,
	})); err != nil {
		t.Fatalf("hide source layer: %v", err)
	}

	beforeCopyHistory := len(instances[h].history.Entries())
	copied, err := DispatchCommand(h, commandCopy, "{}")
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if !copied.UIMeta.CanPaste {
		t.Fatal("copy did not advertise paste availability")
	}
	if got := len(instances[h].history.Entries()); got != beforeCopyHistory {
		t.Fatalf("copy history length = %d, want %d", got, beforeCopyHistory)
	}

	pasted, err := DispatchCommand(h, commandPaste, "{}")
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if pasted.UIMeta.ActiveLayerID == sourceID {
		t.Fatal("paste did not activate a new layer")
	}
	doc := instances[h].manager.activeMut()
	source, _, _, ok := findLayerByID(doc.ensureLayerRoot(), sourceID)
	if !ok || source.Visible() {
		t.Fatal("copy changed the hidden source layer's visibility")
	}
	layer := findPixelLayer(doc, pasted.UIMeta.ActiveLayerID)
	if layer == nil {
		t.Fatal("pasted layer is not a pixel layer")
	}
	if layer.Name() != "Source copy" {
		t.Errorf("pasted layer name = %q, want %q", layer.Name(), "Source copy")
	}
	if layer.Bounds != (LayerBounds{X: 1, Y: 1, W: 1, H: 2}) {
		t.Errorf("pasted bounds = %+v, want {1,1,1,2}", layer.Bounds)
	}
	wantPixels := []byte{255, 0, 0, 255, 0, 0, 255, 255}
	if !bytes.Equal(layer.Pixels, wantPixels) {
		t.Errorf("pasted pixels = %v, want %v", layer.Pixels, wantPixels)
	}
	if got := len(instances[h].history.Entries()); got != beforeCopyHistory+1 {
		t.Errorf("paste history length = %d, want %d", got, beforeCopyHistory+1)
	}
}

func TestClipboardCutUsesFeatherCoverageAndUndo(t *testing.T) {
	h := Init(mustJSON(t, EngineConfig{DocumentWidth: 1, DocumentHeight: 1, Background: "transparent"}))
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Source",
		Bounds:    LayerBounds{W: 1, H: 1},
		Pixels:    []byte{10, 20, 30, 200},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	sourceID := added.UIMeta.ActiveLayerID
	doc := instances[h].manager.activeMut()
	doc.Selection = &Selection{Width: 1, Height: 1, Mask: []byte{128}}

	cut, err := DispatchCommand(h, commandCut, "{}")
	if err != nil {
		t.Fatalf("cut: %v", err)
	}
	if !cut.UIMeta.CanPaste {
		t.Fatal("cut did not populate the clipboard")
	}
	afterCut := findPixelLayer(instances[h].manager.activeMut(), sourceID)
	wantRemainingAlpha := scaleMaskedAlpha(200, 127)
	if afterCut.Pixels[3] != wantRemainingAlpha {
		t.Errorf("alpha after cut = %d, want %d", afterCut.Pixels[3], wantRemainingAlpha)
	}
	if !bytes.Equal(afterCut.Pixels[:3], []byte{10, 20, 30}) {
		t.Errorf("cut changed hidden RGB: %v", afterCut.Pixels[:3])
	}

	if _, err := DispatchCommand(h, commandUndo, "{}"); err != nil {
		t.Fatalf("undo cut: %v", err)
	}
	afterUndo := findPixelLayer(instances[h].manager.activeMut(), sourceID)
	if afterUndo.Pixels[3] != 200 {
		t.Errorf("alpha after undo = %d, want 200", afterUndo.Pixels[3])
	}
	if !instances[h].pixelClipboard.valid() {
		t.Fatal("undo unexpectedly cleared the runtime clipboard")
	}
}

func TestClipboardCutRejectsLockedAndNonPixelLayers(t *testing.T) {
	t.Run("locked pixel layer", func(t *testing.T) {
		inst, _, layer := newLockTestInstance(t, LayerLockPixels)
		if err := inst.cutPixels(); err == nil || !strings.Contains(err.Error(), "locked") {
			t.Fatalf("cut error = %v, want lock error", err)
		}
		if inst.pixelClipboard.valid() {
			t.Fatal("rejected cut populated the clipboard")
		}
		if layer.Pixels[3] != 255 {
			t.Errorf("rejected cut changed alpha to %d", layer.Pixels[3])
		}
	})

	t.Run("vector layer", func(t *testing.T) {
		inst := &instance{manager: newDocumentManager(), history: newHistoryStack(defaultHistoryMax)}
		doc := inst.newDocument(CreateDocumentPayload{Name: "Doc", Width: 2, Height: 2, Background: "transparent"})
		layer := NewVectorLayer("Vector", LayerBounds{W: 2, H: 2}, nil, make([]byte, 16))
		if err := doc.AddLayer(layer, "", -1); err != nil {
			t.Fatalf("add vector layer: %v", err)
		}
		inst.manager.Create(doc)
		if err := inst.cutPixels(); err == nil || !strings.Contains(err.Error(), "pixel layer") {
			t.Fatalf("cut error = %v, want pixel-layer error", err)
		}
	})
}

func TestClipboardPasteCentersAcrossDocuments(t *testing.T) {
	inst := &instance{manager: newDocumentManager(), history: newHistoryStack(defaultHistoryMax)}
	source := inst.newDocument(CreateDocumentPayload{Name: "Source", Width: 8, Height: 8, Background: "transparent"})
	sourceLayer := NewPixelLayer("Tile", LayerBounds{X: 1, Y: 2, W: 2, H: 2}, make([]byte, 16))
	if err := source.AddLayer(sourceLayer, "", -1); err != nil {
		t.Fatalf("add source layer: %v", err)
	}
	inst.manager.Create(source)
	storedSource := inst.manager.activeMut()
	storedSource.Selection = newRectSelection(storedSource.Width, storedSource.Height, sourceLayer.Bounds)
	if err := inst.copyPixels(); err != nil {
		t.Fatalf("copy: %v", err)
	}

	target := inst.newDocument(CreateDocumentPayload{Name: "Target", Width: 10, Height: 6, Background: "transparent"})
	inst.manager.Create(target)
	if err := inst.pastePixels(); err != nil {
		t.Fatalf("paste: %v", err)
	}
	pasted := findPixelLayer(inst.manager.activeMut(), inst.manager.activeMut().ActiveLayerID)
	if pasted == nil {
		t.Fatal("pasted layer missing")
	}
	if pasted.Bounds != (LayerBounds{X: 4, Y: 2, W: 2, H: 2}) {
		t.Errorf("cross-document paste bounds = %+v, want centered {4,2,2,2}", pasted.Bounds)
	}
}
