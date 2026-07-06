package engine

import (
	"bytes"
	"fmt"
	"testing"
)

// This file is the regression net for the pointer-snapshot history design
// (Phase S.3): captureSnapshot stores the active document POINTER instead of a
// deep clone, relying on the immutability-by-replacement invariant documented
// on captureSnapshot. The tests below interleave snapshot-based commands with
// in-place pixel mutations (brush strokes) and walk the history in every
// direction, asserting byte-exact states throughout.

// storedLayerPixels returns the pixel buffer of layerID in the STORED active
// document (not a clone), so assertions observe exactly what history sees.
func storedLayerPixels(t *testing.T, h int32, layerID string) []byte {
	t.Helper()
	mu.Lock()
	inst := instances[h]
	mu.Unlock()
	doc := inst.manager.activeMut()
	if doc == nil {
		t.Fatal("no active document")
	}
	layer := findPixelLayer(doc, layerID)
	if layer == nil {
		t.Fatalf("layer %q not found in stored document", layerID)
	}
	return layer.Pixels
}

func storedActiveDoc(t *testing.T, h int32) *Document {
	t.Helper()
	mu.Lock()
	inst := instances[h]
	mu.Unlock()
	doc := inst.manager.activeMut()
	if doc == nil {
		t.Fatal("no active document")
	}
	return doc
}

// alphaAt reads the alpha byte at document coordinates (x, y) of a full-size
// layer positioned at the document origin.
func alphaAt(pixels []byte, docW, x, y int) byte {
	return pixels[(y*docW+x)*4+3]
}

func redAt(pixels []byte, docW, x, y int) byte {
	return pixels[(y*docW+x)*4]
}

func greenAt(pixels []byte, docW, x, y int) byte {
	return pixels[(y*docW+x)*4+1]
}

// TestHistoryStressPointerSnapshotsWithStrokes interleaves snapshot commands
// (add layer, rename, set opacity) with in-place brush-stroke pixel deltas,
// then walks the history: undo all the way asserting the exact expected state
// at every step, redo all the way, jump to the middle, branch with a fresh
// edit (redo truncation), and undo again.
func TestHistoryStressPointerSnapshotsWithStrokes(t *testing.T) {
	const docW, docH = 64, 64
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Stress", Width: docW, Height: docH, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	// S1: add a transparent full-size pixel layer.
	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "L1",
		Bounds:    LayerBounds{X: 0, Y: 0, W: docW, H: docH},
		Pixels:    make([]byte, docW*docH*4),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	mu.Lock()
	inst := instances[h]
	mu.Unlock()

	layerState := func() (name string, opacity float64) {
		doc := storedActiveDoc(t, h)
		layer := doc.findLayer(layerID)
		if layer == nil {
			t.Fatalf("layer %q missing", layerID)
		}
		return layer.Name(), layer.Opacity()
	}
	layerCount := func() int {
		return len(storedActiveDoc(t, h).Layers())
	}

	// S2: rename the layer.
	if _, err := DispatchCommand(h, commandSetLayerName, mustJSON(t, SetLayerNamePayload{
		LayerID: layerID, Name: "Painted",
	})); err != nil {
		t.Fatalf("rename layer: %v", err)
	}

	// S3: red brush stroke through the layer centre (pixel delta, in-place).
	redBrush := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 32, Y: 32, Pressure: 1.0, Brush: redBrush})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end red stroke: %v", err)
	}
	if a := alphaAt(storedLayerPixels(t, h, layerID), docW, 32, 32); a == 0 {
		t.Fatal("red stroke did not paint centre pixel")
	}
	if r := redAt(storedLayerPixels(t, h, layerID), docW, 32, 32); r == 0 {
		t.Fatal("red stroke painted but red channel is zero")
	}

	// S4: set opacity to 0.5.
	halfOpacity := 0.5
	if _, err := DispatchCommand(h, int32(0x0104), mustJSON(t, SetLayerOpacityPayload{
		LayerID: layerID, Opacity: &halfOpacity,
	})); err != nil {
		t.Fatalf("set opacity: %v", err)
	}

	// S5: green brush stroke near the top-left corner.
	greenBrush := BrushParams{Size: 6, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{0, 255, 0, 255}}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 10, Y: 10, Pressure: 1.0, Brush: greenBrush})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end green stroke: %v", err)
	}
	if g := greenAt(storedLayerPixels(t, h, layerID), docW, 10, 10); g == 0 {
		t.Fatal("green stroke did not paint (10,10)")
	}

	undo := func() {
		t.Helper()
		if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
			t.Fatalf("undo: %v", err)
		}
	}
	redo := func() {
		t.Helper()
		if _, err := DispatchCommand(h, commandRedo, ""); err != nil {
			t.Fatalf("redo: %v", err)
		}
	}

	// ---- Undo all the way down, asserting each intermediate state. ----

	// S5 -> S4: green stroke gone, red stroke and opacity remain.
	undo()
	if g := greenAt(storedLayerPixels(t, h, layerID), docW, 10, 10); g != 0 {
		t.Fatalf("after undo green stroke: green at (10,10) = %d, want 0", g)
	}
	if a := alphaAt(storedLayerPixels(t, h, layerID), docW, 10, 10); a != 0 {
		t.Fatalf("after undo green stroke: alpha at (10,10) = %d, want 0", a)
	}
	if a := alphaAt(storedLayerPixels(t, h, layerID), docW, 32, 32); a == 0 {
		t.Fatal("after undo green stroke: red stroke must survive")
	}
	if name, op := layerState(); name != "Painted" || op != 0.5 {
		t.Fatalf("after undo green stroke: layer = (%q, %v), want (Painted, 0.5)", name, op)
	}

	// S4 -> S3: opacity back to 1.
	undo()
	if name, op := layerState(); name != "Painted" || op != 1 {
		t.Fatalf("after undo opacity: layer = (%q, %v), want (Painted, 1)", name, op)
	}
	if a := alphaAt(storedLayerPixels(t, h, layerID), docW, 32, 32); a == 0 {
		t.Fatal("after undo opacity: red stroke must survive")
	}

	// S3 -> S2: red stroke gone, layer fully transparent again.
	undo()
	for i, v := range storedLayerPixels(t, h, layerID) {
		if v != 0 {
			t.Fatalf("after undo red stroke: pixels[%d] = %d, want fully transparent layer", i, v)
		}
	}
	if name, _ := layerState(); name != "Painted" {
		t.Fatalf("after undo red stroke: name = %q, want Painted", name)
	}

	// S2 -> S1: original name restored.
	undo()
	if name, _ := layerState(); name != "L1" {
		t.Fatalf("after undo rename: name = %q, want L1", name)
	}

	// S1 -> S0: layer removed.
	undo()
	if got := layerCount(); got != 0 {
		t.Fatalf("after undo add layer: layer count = %d, want 0", got)
	}

	// ---- Redo all the way back up to S5. ----
	for range 5 {
		redo()
	}
	if got := layerCount(); got != 1 {
		t.Fatalf("after redo x5: layer count = %d, want 1", got)
	}
	if name, op := layerState(); name != "Painted" || op != 0.5 {
		t.Fatalf("after redo x5: layer = (%q, %v), want (Painted, 0.5)", name, op)
	}
	if a := alphaAt(storedLayerPixels(t, h, layerID), docW, 32, 32); a == 0 {
		t.Fatal("after redo x5: red stroke missing")
	}
	if g := greenAt(storedLayerPixels(t, h, layerID), docW, 10, 10); g == 0 {
		t.Fatal("after redo x5: green stroke missing")
	}

	// ---- Jump to the middle (S3: create doc + add + rename + red stroke,
	// history index 4 because the document creation is entry 1). ----
	if _, err := DispatchCommand(h, commandJumpHistory, `{"historyIndex":4}`); err != nil {
		t.Fatalf("jump to index 4: %v", err)
	}
	if name, op := layerState(); name != "Painted" || op != 1 {
		t.Fatalf("after jump to 4: layer = (%q, %v), want (Painted, 1)", name, op)
	}
	if a := alphaAt(storedLayerPixels(t, h, layerID), docW, 32, 32); a == 0 {
		t.Fatal("after jump to 4: red stroke missing")
	}
	if g := greenAt(storedLayerPixels(t, h, layerID), docW, 10, 10); g != 0 {
		t.Fatalf("after jump to 4: green at (10,10) = %d, want 0", g)
	}

	// ---- Fresh edit from the middle: redo stack must be truncated. ----
	if _, err := DispatchCommand(h, commandSetLayerName, mustJSON(t, SetLayerNamePayload{
		LayerID: layerID, Name: "Branch",
	})); err != nil {
		t.Fatalf("branch rename: %v", err)
	}
	if inst.history.CanRedo() {
		t.Fatal("redo stack must be truncated after a fresh edit")
	}
	if name, _ := layerState(); name != "Branch" {
		t.Fatalf("after branch rename: name = %q, want Branch", name)
	}

	// ---- Undo down the new branch. ----
	undo() // Branch -> S3
	if name, _ := layerState(); name != "Painted" {
		t.Fatalf("after undo branch rename: name = %q, want Painted", name)
	}
	undo() // S3 -> S2 (red stroke gone)
	for i, v := range storedLayerPixels(t, h, layerID) {
		if v != 0 {
			t.Fatalf("branch undo of red stroke: pixels[%d] = %d, want 0", i, v)
		}
	}
	undo() // S2 -> S1
	if name, _ := layerState(); name != "L1" {
		t.Fatalf("branch undo of rename: name = %q, want L1", name)
	}
	undo() // S1 -> S0
	if got := layerCount(); got != 0 {
		t.Fatalf("branch undo of add layer: layer count = %d, want 0", got)
	}
}

// TestFreeTransformCommitUndoRedo verifies that committing a free transform
// after live preview updates records the PRE-transform state as the history
// "before" snapshot: undo must restore the original pixels and bounds (not the
// last preview frame), and redo must reproduce the transformed result.
func TestFreeTransformCommitUndoRedo(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "FT", Width: 20, Height: 20, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	})); err != nil {
		t.Fatalf("create doc: %v", err)
	}

	original := makeSolidPixels(4, 4, 200, 100, 50, 255)
	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Layer 1",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    append([]byte(nil), original...),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	if _, err := DispatchCommand(h, commandBeginFreeTransform, mustJSON(t, BeginFreeTransformPayload{
		LayerID: layerID,
	})); err != nil {
		t.Fatalf("begin free transform: %v", err)
	}

	// Live preview: integer translate to (8, 0). This replaces the stored
	// document with preview pixels — exactly the state a naive commit would
	// wrongly record as its before-snapshot.
	if _, err := DispatchCommand(h, commandUpdateFreeTransform, mustJSON(t, UpdateFreeTransformPayload{
		A: 1, B: 0, C: 0, D: 1, TX: 8, TY: 0, PivotX: 10, PivotY: 2,
	})); err != nil {
		t.Fatalf("update free transform: %v", err)
	}

	if _, err := DispatchCommand(h, commandCommitFreeTransform, `{}`); err != nil {
		t.Fatalf("commit free transform: %v", err)
	}

	getLayer := func() *PixelLayer {
		doc := storedActiveDoc(t, h)
		layer := doc.findLayer(layerID)
		pl, ok := layer.(*PixelLayer)
		if !ok || pl == nil {
			t.Fatalf("layer %q not found or not a pixel layer", layerID)
		}
		return pl
	}

	committed := getLayer()
	if committed.Bounds.X != 8 || committed.Bounds.Y != 0 {
		t.Fatalf("after commit: bounds origin = (%d,%d), want (8,0)", committed.Bounds.X, committed.Bounds.Y)
	}
	committedPixels := append([]byte(nil), committed.Pixels...)

	// Undo: the layer must return byte-exactly to its pre-transform state.
	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatalf("undo: %v", err)
	}
	undone := getLayer()
	if undone.Bounds != (LayerBounds{X: 0, Y: 0, W: 4, H: 4}) {
		t.Fatalf("after undo: bounds = %+v, want origin (0,0) 4x4", undone.Bounds)
	}
	if !bytes.Equal(undone.Pixels, original) {
		t.Fatal("after undo: pixels differ from the pre-transform original (undo restored the preview instead of the source)")
	}

	// Redo: the transformed result must come back.
	if _, err := DispatchCommand(h, commandRedo, ""); err != nil {
		t.Fatalf("redo: %v", err)
	}
	redone := getLayer()
	if redone.Bounds.X != 8 || redone.Bounds.Y != 0 {
		t.Fatalf("after redo: bounds origin = (%d,%d), want (8,0)", redone.Bounds.X, redone.Bounds.Y)
	}
	if !bytes.Equal(redone.Pixels, committedPixels) {
		t.Fatal("after redo: pixels differ from the committed transform result")
	}
}

// TestUndoDuringFilterPreviewDoesNotCorruptHistory guards the pointer-snapshot
// invariant against in-flight filter previews: a preview mutates the stored
// document in place, and that document is referenced directly by the latest
// history snapshot. A history restore arriving mid-preview must revert the
// preview bytes first, so that a subsequent redo reinstates the CLEAN
// post-command state, not the preview-tainted one.
func TestUndoDuringFilterPreviewDoesNotCorruptHistory(t *testing.T) {
	registerTestInvertFilter(t, "midpreview-invert")

	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })
	addRedPixelLayer(t, h)

	pl := getActivePixelLayer(t, h)
	layerID := pl.ID()
	cleanPixels := append([]byte(nil), pl.Pixels...)

	// Undoable command whose after-snapshot references the stored document.
	if _, err := DispatchCommand(h, commandSetLayerName, mustJSON(t, SetLayerNamePayload{
		LayerID: layerID, Name: "Renamed",
	})); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Start a preview: mutates the stored document's pixels in place.
	if _, err := DispatchCommand(h, commandPreviewFilter, mustJSON(t, PreviewFilterPayload{
		FilterID: "midpreview-invert",
		Scale:    1,
	})); err != nil {
		t.Fatalf("preview filter: %v", err)
	}

	// Undo the rename while the preview is still open.
	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatalf("undo: %v", err)
	}

	// Redo must bring back the renamed state with CLEAN pixels — if the
	// pointer snapshot saw the preview mutation, the red layer comes back
	// inverted (cyan).
	if _, err := DispatchCommand(h, commandRedo, ""); err != nil {
		t.Fatalf("redo: %v", err)
	}
	after := storedLayerPixels(t, h, layerID)
	if !bytes.Equal(after, cleanPixels) {
		t.Fatalf("after undo/redo across an open filter preview: pixels[0..3] = %v, want clean red %v (preview leaked into history)",
			after[:4], cleanPixels[:4])
	}
}

// TestUndoMidPaintStrokeRevertsStroke guards the invariant against an undo
// arriving between BeginPaintStroke and EndPaintStroke (e.g. Ctrl+Z during a
// drag): the half-finished stroke mutated the stored document in place without
// having pushed its pixel delta yet, so the restore must first roll the stroke
// back byte-exactly and discard it.
func TestUndoMidPaintStrokeRevertsStroke(t *testing.T) {
	const docW, docH = 64, 64
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "MidStroke", Width: docW, Height: docH, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	})); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "L1",
		Bounds:    LayerBounds{X: 0, Y: 0, W: docW, H: docH},
		Pixels:    make([]byte, docW*docH*4),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	if _, err := DispatchCommand(h, commandSetLayerName, mustJSON(t, SetLayerNamePayload{
		LayerID: layerID, Name: "Renamed",
	})); err != nil {
		t.Fatalf("rename: %v", err)
	}

	mu.Lock()
	inst := instances[h]
	mu.Unlock()

	// Begin a stroke but do NOT end it.
	brush := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 20, Y: 20, Pressure: 1.0, Brush: brush})
	if a := alphaAt(storedLayerPixels(t, h, layerID), docW, 20, 20); a == 0 {
		t.Fatal("stroke did not paint before the interrupting undo")
	}

	// Undo the rename mid-stroke.
	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if inst.paintStroke != nil {
		t.Fatal("in-flight paint stroke must be discarded by a history restore")
	}
	for i, v := range storedLayerPixels(t, h, layerID) {
		if v != 0 {
			t.Fatalf("after mid-stroke undo: pixels[%d] = %d, want fully transparent (stroke reverted)", i, v)
		}
	}

	// Redo the rename: the pointer-captured after-snapshot must be untainted.
	if _, err := DispatchCommand(h, commandRedo, ""); err != nil {
		t.Fatalf("redo: %v", err)
	}
	for i, v := range storedLayerPixels(t, h, layerID) {
		if v != 0 {
			t.Fatalf("after redo: pixels[%d] = %d, want fully transparent (half-finished stroke leaked into history)", i, v)
		}
	}
	doc := storedActiveDoc(t, h)
	if name := doc.findLayer(layerID).Name(); name != "Renamed" {
		t.Fatalf("after redo: name = %q, want Renamed", name)
	}
}

// BenchmarkExecuteDocCommandRename measures the per-command cost of a trivial
// metadata edit (layer rename) on a document carrying several megabytes of
// pixel data. Before the pointer-snapshot change this performed four full
// document deep-clones per command; now it performs exactly one.
func BenchmarkExecuteDocCommandRename(b *testing.B) {
	const docW, docH = 1024, 1024 // 4 MB per layer
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("bench-doc", "Bench", docW, docH)
	var layerID string
	for i := range 3 {
		layer := NewPixelLayer(fmt.Sprintf("Layer %d", i), LayerBounds{X: 0, Y: 0, W: docW, H: docH}, make([]byte, docW*docH*4))
		doc.LayerRoot.SetChildren(append(doc.LayerRoot.Children(), layer))
		layerID = layer.ID()
	}
	doc.ActiveLayerID = layerID
	inst.manager.Create(doc)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		name := fmt.Sprintf("Renamed %d", i)
		if err := inst.executeDocCommand("Rename layer", func(d *Document) error {
			return d.SetLayerName(layerID, name)
		}); err != nil {
			b.Fatal(err)
		}
	}
}
