package engine

import (
	"errors"
	"testing"
)

// TestRenameToSameNameIsNoOp verifies that a command which changes nothing (here,
// renaming a layer to its current name) is suppressed and adds no history entry,
// while a genuine rename does add one.
func TestRenameToSameNameIsNoOp(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Sketch",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	inst := instances[h]
	if inst == nil {
		t.Fatal("instance not found for handle")
	}
	depthBefore := inst.history.CurrentIndex()

	// Rename to the SAME name: must be suppressed.
	if _, err := DispatchCommand(h, commandSetLayerName, mustJSON(t, SetLayerNamePayload{
		LayerID: layerID,
		Name:    "Sketch",
	})); err != nil {
		t.Fatalf("rename to same name: %v", err)
	}
	if got := inst.history.CurrentIndex(); got != depthBefore {
		t.Fatalf("history depth = %d after no-op rename, want %d (no entry added)", got, depthBefore)
	}

	// Rename to a DIFFERENT name: must add exactly one entry.
	if _, err := DispatchCommand(h, commandSetLayerName, mustJSON(t, SetLayerNamePayload{
		LayerID: layerID,
		Name:    "Headline",
	})); err != nil {
		t.Fatalf("real rename: %v", err)
	}
	if got := inst.history.CurrentIndex(); got != depthBefore+1 {
		t.Fatalf("history depth = %d after real rename, want %d", got, depthBefore+1)
	}
}

// TestHandleEndPaintStrokeDeltaFailureClearsRedo verifies the non-undoable-stroke
// semantics: when the undo delta cannot be built the stroke pixels are already
// committed (the document has diverged from history), so handleEndPaintStroke must
// return an error and clear the redo stack (a stale redo entry would now corrupt
// the document).
func TestHandleEndPaintStrokeDeltaFailureClearsRedo(t *testing.T) {
	const w, hgt = 64, 64
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: hgt, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("paint-fail", "PaintFail", w, hgt)
	layer := NewPixelLayer("Layer", LayerBounds{X: 0, Y: 0, W: w, H: hgt}, make([]byte, w*hgt*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)

	brush := BrushParams{Size: 20, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}}

	// A real stroke to populate the undo stack, then undo it to populate redo.
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 32, Y: 32, Pressure: 1, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 40, Y: 32, Pressure: 1})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("first stroke end: %v", err)
	}
	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("undo first stroke: %v", err)
	}
	if !inst.history.CanRedo() {
		t.Fatal("precondition: redo stack should be non-empty after undo")
	}

	// Force the delta build to fail on the next stroke.
	orig := buildStrokeDelta
	buildStrokeDelta = func(_ []byte, _, _ int, _ []byte, _, _ int, _ DirtyRect) (PixelDelta, error) {
		return PixelDelta{}, errors.New("forced delta failure")
	}
	defer func() { buildStrokeDelta = orig }()

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 32, Y: 32, Pressure: 1, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 40, Y: 32, Pressure: 1})
	err := inst.handleEndPaintStroke()
	if err == nil {
		t.Fatal("handleEndPaintStroke should return an error when the delta build fails")
	}
	if inst.history.CanRedo() {
		t.Fatal("redo stack must be cleared after a non-undoable stroke")
	}
}
