package engine

import (
	"testing"
)

// TestUndoPreservesOtherOpenDocuments is a regression test for the data-loss
// bug where undoing an edit in one document silently destroyed every other
// open document (restoreSnapshot used to rebuild the manager from scratch).
//
// It opens two documents, makes an undoable edit in the active one, undoes it,
// and asserts that BOTH documents still exist, that the untouched document's
// content survived, and that the edit was reverted.
func TestUndoPreservesOtherOpenDocuments(t *testing.T) {
	h := Init("")

	// Document A — the one that must survive an undo performed on B.
	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "DocA", Width: 120, Height: 90, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	})); err != nil {
		t.Fatalf("create document A: %v", err)
	}
	docAID := instances[h].manager.ActiveID()

	// Document B — becomes active on creation; the edit and undo happen here.
	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "DocB", Width: 200, Height: 150, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	})); err != nil {
		t.Fatalf("create document B: %v", err)
	}
	docBID := instances[h].manager.ActiveID()
	if docAID == "" || docBID == "" || docAID == docBID {
		t.Fatalf("unexpected document ids: A=%q B=%q", docAID, docBID)
	}

	// Snapshot A's content so we can prove it survived untouched.
	docABefore := instances[h].manager.Active() // active is B right now
	if docABefore.ID != docBID {
		t.Fatalf("expected B to be active, got %q", docABefore.ID)
	}

	layersBefore := len(instances[h].manager.Active().Layers())

	// Undoable edit on the active document (B): add a pixel layer.
	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Edit Layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 200, H: 150},
		Pixels:    makeSolidPixels(200, 150, 10, 20, 30, 255),
	})); err != nil {
		t.Fatalf("add layer to B: %v", err)
	}
	if got := len(instances[h].manager.Active().Layers()); got != layersBefore+1 {
		t.Fatalf("layer count after add = %d, want %d", got, layersBefore+1)
	}

	// Undo the edit.
	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatalf("undo add layer: %v", err)
	}

	mgr := instances[h].manager

	// 1. Both documents must still exist.
	ids := mgr.IDs()
	if len(ids) != 2 {
		t.Fatalf("open document count after undo = %d (%v), want 2", len(ids), ids)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen[docAID] || !seen[docBID] {
		t.Fatalf("document set after undo = %v, want to contain %q and %q", ids, docAID, docBID)
	}

	// 2. The active document should be the snapshot's document (B).
	if mgr.ActiveID() != docBID {
		t.Fatalf("active document after undo = %q, want %q", mgr.ActiveID(), docBID)
	}

	// 3. The edit was reverted on B.
	if got := len(mgr.Active().Layers()); got != layersBefore {
		t.Fatalf("layer count on B after undo = %d, want %d (edit not reverted)", got, layersBefore)
	}

	// 4. The untouched document A survived with its content intact.
	if err := mgr.SetActiveID(docAID); err != nil {
		t.Fatalf("document A missing after undo: %v", err)
	}
	docA := mgr.Active()
	if docA == nil {
		t.Fatal("document A is nil after undo")
	}
	if docA.Name != "DocA" || docA.Width != 120 || docA.Height != 90 {
		t.Fatalf("document A content after undo = {%q %dx%d}, want {DocA 120x90}",
			docA.Name, docA.Width, docA.Height)
	}
}
