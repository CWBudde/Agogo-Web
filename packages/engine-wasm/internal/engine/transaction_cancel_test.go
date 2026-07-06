package engine

import "testing"

// TestEndTransactionWithoutCommitRevertsDocumentMutations verifies that
// dispatching EndTransaction{commit:false} does not merely discard the grouped
// history entry but actually reverts the document to its pre-transaction
// state, while leaving the undo/redo stacks (and thus an existing redo entry)
// fully intact.
func TestEndTransactionWithoutCommitRevertsDocumentMutations(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{255, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	// Build a redo entry: set opacity to 0.5, then undo it.
	halfOpacity := 0.5
	if _, err := DispatchCommand(h, commandSetLayerOp, mustJSON(t, SetLayerOpacityPayload{
		LayerID: layerID,
		Opacity: &halfOpacity,
	})); err != nil {
		t.Fatalf("set opacity: %v", err)
	}
	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	preTxnLayer, ok := findLayerMetaByID(undone.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q missing after undo", layerID)
	}
	if preTxnLayer.Opacity != 1 {
		t.Fatalf("pre-transaction opacity = %.2f, want 1", preTxnLayer.Opacity)
	}
	historyLenBefore := len(undone.UIMeta.History)
	historyIndexBefore := undone.UIMeta.CurrentHistoryIndex

	// Interactive gesture: begin transaction, mutate, then abort (commit=false).
	if _, err := DispatchCommand(h, commandBeginTxn, mustJSON(t, BeginTransactionPayload{
		Description: "Drag opacity",
	})); err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	quarterOpacity := 0.25
	if _, err := DispatchCommand(h, commandSetLayerOp, mustJSON(t, SetLayerOpacityPayload{
		LayerID: layerID,
		Opacity: &quarterOpacity,
	})); err != nil {
		t.Fatalf("set opacity in transaction: %v", err)
	}
	cancelled, err := DispatchCommand(h, commandEndTxn, mustJSON(t, EndTransactionPayload{Commit: false}))
	if err != nil {
		t.Fatalf("end transaction without commit: %v", err)
	}

	cancelledLayer, ok := findLayerMetaByID(cancelled.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q missing after cancelled transaction", layerID)
	}
	if cancelledLayer.Opacity != 1 {
		t.Fatalf("opacity after cancelled transaction = %.2f, want 1 (mutation reverted)", cancelledLayer.Opacity)
	}
	if len(cancelled.UIMeta.History) != historyLenBefore {
		t.Fatalf("history length after cancel = %d, want %d (unchanged)", len(cancelled.UIMeta.History), historyLenBefore)
	}
	if cancelled.UIMeta.CurrentHistoryIndex != historyIndexBefore {
		t.Fatalf("history index after cancel = %d, want %d (unchanged)", cancelled.UIMeta.CurrentHistoryIndex, historyIndexBefore)
	}

	// The redo entry recorded before the transaction must still replay: cancel
	// restored the exact head state it was recorded against.
	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo after cancelled transaction: %v", err)
	}
	redoneLayer, ok := findLayerMetaByID(redone.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q missing after redo", layerID)
	}
	if redoneLayer.Opacity != 0.5 {
		t.Fatalf("opacity after redo = %.2f, want 0.5 (redo entry replayed)", redoneLayer.Opacity)
	}
}

// TestEndTransactionWithoutCommitIsNoOpWithoutActiveTransaction ensures the
// cancel path is safe to dispatch when no transaction is open (e.g. a stray
// pointercancel after the gesture already ended).
func TestEndTransactionWithoutCommitIsNoOpWithoutActiveTransaction(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{255, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	historyLenBefore := len(added.UIMeta.History)
	historyIndexBefore := added.UIMeta.CurrentHistoryIndex

	result, err := DispatchCommand(h, commandEndTxn, mustJSON(t, EndTransactionPayload{Commit: false}))
	if err != nil {
		t.Fatalf("end transaction without active transaction: %v", err)
	}
	if len(result.UIMeta.History) != historyLenBefore || result.UIMeta.CurrentHistoryIndex != historyIndexBefore {
		t.Fatalf("history changed by no-op cancel: len=%d/%d index=%d/%d",
			len(result.UIMeta.History), historyLenBefore, result.UIMeta.CurrentHistoryIndex, historyIndexBefore)
	}
	if _, ok := findLayerMetaByID(result.UIMeta.Layers, added.UIMeta.ActiveLayerID); !ok {
		t.Fatal("layer missing after no-op cancel")
	}
}
