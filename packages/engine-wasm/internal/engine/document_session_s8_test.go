package engine

import "testing"

func TestDocumentSessionsKeepHistoryAndViewportIsolated(t *testing.T) {
	h := Init("")
	defer Free(h)

	first, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "First", Width: 100, Height: 80, Resolution: 72, ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	}))
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	firstID := first.UIMeta.ActiveDocumentID
	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel, Name: "First layer", Bounds: LayerBounds{W: 1, H: 1}, Pixels: []byte{255, 0, 0, 255},
	})); err != nil {
		t.Fatalf("add first layer: %v", err)
	}
	if _, err := DispatchCommand(h, commandPanSet, mustJSON(t, PanPayload{CenterX: 12, CenterY: 34})); err != nil {
		t.Fatalf("pan first: %v", err)
	}

	second, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Second", Width: 60, Height: 40, Resolution: 72, ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	}))
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	secondID := second.UIMeta.ActiveDocumentID
	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel, Name: "Second layer", Bounds: LayerBounds{W: 1, H: 1}, Pixels: []byte{0, 255, 0, 255},
	})); err != nil {
		t.Fatalf("add second layer: %v", err)
	}

	switched, err := DispatchCommand(h, commandSwitchDocument, mustJSON(t, map[string]string{"documentId": firstID}))
	if err != nil {
		t.Fatalf("switch first: %v", err)
	}
	if switched.Viewport.CenterX != 12 || switched.Viewport.CenterY != 34 {
		t.Fatalf("first viewport = %.1f,%.1f, want 12,34", switched.Viewport.CenterX, switched.Viewport.CenterY)
	}
	if len(switched.UIMeta.History) != 1 || !switched.UIMeta.CanUndo {
		t.Fatalf("first history not restored: %+v", switched.UIMeta.History)
	}
	if _, err := DispatchCommand(h, commandUndo, "{}"); err != nil {
		t.Fatalf("undo first: %v", err)
	}

	secondAgain, err := DispatchCommand(h, commandSwitchDocument, mustJSON(t, map[string]string{"documentId": secondID}))
	if err != nil {
		t.Fatalf("switch second: %v", err)
	}
	if len(secondAgain.UIMeta.Layers) != 1 || secondAgain.UIMeta.Layers[0].Name != "Second layer" {
		t.Fatalf("second document changed by first undo: %+v", secondAgain.UIMeta.Layers)
	}
	if len(secondAgain.UIMeta.Documents) != 2 {
		t.Fatalf("document summaries = %d, want 2", len(secondAgain.UIMeta.Documents))
	}
}

func TestDocumentSavedBaselineTracksUndoAndBranching(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)
	docID := instances[h].manager.ActiveID()

	if _, err := DispatchCommand(h, commandMarkDocumentSaved, mustJSON(t, map[string]string{"documentId": docID})); err != nil {
		t.Fatalf("mark saved: %v", err)
	}
	clean, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("render clean: %v", err)
	}
	if len(clean.UIMeta.Documents) != 1 || clean.UIMeta.Documents[0].Modified {
		t.Fatalf("saved document should be clean: %+v", clean.UIMeta.Documents)
	}
	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel, Name: "Edit", Bounds: LayerBounds{W: 1, H: 1}, Pixels: []byte{255, 255, 255, 255},
	})); err != nil {
		t.Fatalf("edit: %v", err)
	}
	dirty, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("render dirty: %v", err)
	}
	if !dirty.UIMeta.Documents[0].Modified {
		t.Fatal("edited document should be modified")
	}
	if _, err := DispatchCommand(h, commandUndo, "{}"); err != nil {
		t.Fatalf("undo: %v", err)
	}
	restored, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("render restored: %v", err)
	}
	if restored.UIMeta.Documents[0].Modified {
		t.Fatal("undo to saved content version should restore clean state")
	}
}

func TestCloseTargetDocumentActivatesLeftNeighbor(t *testing.T) {
	h := Init("")
	defer Free(h)
	ids := make([]string, 0, 3)
	for _, name := range []string{"One", "Two", "Three"} {
		result, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
			Name: name, Width: 10, Height: 10, Resolution: 72, ColorMode: "rgb", BitDepth: 8, Background: "transparent",
		}))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, result.UIMeta.ActiveDocumentID)
	}
	closed, err := DispatchCommand(h, commandCloseDocument, mustJSON(t, map[string]string{"documentId": ids[2]}))
	if err != nil {
		t.Fatalf("close third: %v", err)
	}
	if closed.UIMeta.ActiveDocumentID != ids[1] {
		t.Fatalf("active after close = %q, want left neighbor %q", closed.UIMeta.ActiveDocumentID, ids[1])
	}
}
