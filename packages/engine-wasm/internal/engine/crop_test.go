package engine

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers shared across crop tests.
// ---------------------------------------------------------------------------

// makeDocWithLayer creates an engine handle with a document (w×h) and a
// single full-size pixel layer, returning the handle and layer ID.
func makeDocWithLayer(t *testing.T, docW, docH int) (int32, string) {
	t.Helper()
	h := Init("")
	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "CropTest", Width: docW, Height: docH, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	}))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: docW, H: docH},
		Pixels:    makeSolidPixels(docW, docH, 200, 100, 50, 255),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	return h, result.UIMeta.ActiveLayerID
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestCrop_BeginActivatesState checks that BeginCrop sets crop.Active and
// initialises the box to the full document bounds.
func TestCrop_BeginActivatesState(t *testing.T) {
	h, _ := makeDocWithLayer(t, 100, 80)
	defer Free(h)

	result, err := DispatchCommand(h, commandBeginCrop, `{}`)
	if err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}
	crop := result.UIMeta.Crop
	if crop == nil || !crop.Active {
		t.Fatal("crop should be active after BeginCrop")
	}
	if crop.X != 0 || crop.Y != 0 {
		t.Errorf("crop origin = (%.1f, %.1f), want (0, 0)", crop.X, crop.Y)
	}
	if crop.W != 100 || crop.H != 80 {
		t.Errorf("crop size = %.1fx%.1f, want 100x80", crop.W, crop.H)
	}
}

// TestCrop_UpdateChangesBox verifies that UpdateCrop immediately reflects in
// the next render's UIMeta.
func TestCrop_UpdateChangesBox(t *testing.T) {
	h, _ := makeDocWithLayer(t, 100, 80)
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginCrop, `{}`); err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}

	result, err := DispatchCommand(h, commandUpdateCrop, mustJSON(t, UpdateCropPayload{
		X: 10, Y: 5, W: 60, H: 40,
	}))
	if err != nil {
		t.Fatalf("UpdateCrop: %v", err)
	}
	crop := result.UIMeta.Crop
	if crop == nil {
		t.Fatal("crop should still be active after UpdateCrop")
	}
	if crop.X != 10 || crop.Y != 5 || crop.W != 60 || crop.H != 40 {
		t.Errorf("crop box = (%.0f,%.0f,%.0f,%.0f), want (10,5,60,40)",
			crop.X, crop.Y, crop.W, crop.H)
	}
}

// TestCrop_CancelClearsState checks that CancelCrop deactivates the crop
// without modifying the document.
func TestCrop_CancelClearsState(t *testing.T) {
	h, _ := makeDocWithLayer(t, 100, 80)
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginCrop, `{}`); err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}

	result, err := DispatchCommand(h, commandCancelCrop, `{}`)
	if err != nil {
		t.Fatalf("CancelCrop: %v", err)
	}
	if result.UIMeta.Crop != nil && result.UIMeta.Crop.Active {
		t.Error("crop should not be active after CancelCrop")
	}

	// Document dimensions must be unchanged.
	doc := instances[h].manager.Active()
	if doc.Width != 100 || doc.Height != 80 {
		t.Errorf("doc size = %dx%d, want 100x80 after cancel", doc.Width, doc.Height)
	}
}

// TestCrop_CommitChangesDocumentSize verifies that CommitCrop resizes the
// document to match the crop box.
func TestCrop_CommitChangesDocumentSize(t *testing.T) {
	h, _ := makeDocWithLayer(t, 100, 80)
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginCrop, `{}`); err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandUpdateCrop, mustJSON(t, UpdateCropPayload{
		X: 10, Y: 5, W: 60, H: 40,
	})); err != nil {
		t.Fatalf("UpdateCrop: %v", err)
	}

	result, err := DispatchCommand(h, commandCommitCrop, `{}`)
	if err != nil {
		t.Fatalf("CommitCrop: %v", err)
	}
	if result.UIMeta.Crop != nil && result.UIMeta.Crop.Active {
		t.Error("crop should not be active after CommitCrop")
	}

	doc := instances[h].manager.Active()
	if doc.Width != 60 || doc.Height != 40 {
		t.Errorf("doc size = %dx%d, want 60x40 after commit", doc.Width, doc.Height)
	}
}

// TestCrop_CommitShiftsLayers verifies that CommitCrop shifts pixel layer
// bounds so the cropped-off area becomes the new origin.
func TestCrop_CommitShiftsLayers(t *testing.T) {
	h, _ := makeDocWithLayer(t, 100, 80)
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginCrop, `{}`); err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandUpdateCrop, mustJSON(t, UpdateCropPayload{
		X: 10, Y: 20, W: 50, H: 30,
	})); err != nil {
		t.Fatalf("UpdateCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandCommitCrop, `{}`); err != nil {
		t.Fatalf("CommitCrop: %v", err)
	}

	doc := instances[h].manager.Active()
	pl := doc.ActiveLayer().(*PixelLayer)
	// Layer started at (0,0); after cropping from (10,20) it should be at (-10,-20).
	if pl.Bounds.X != -10 || pl.Bounds.Y != -20 {
		t.Errorf("layer bounds after crop = (%d,%d), want (-10,-20)",
			pl.Bounds.X, pl.Bounds.Y)
	}
}

// TestCrop_CommitIsUndoable checks that undoing a committed crop restores the
// original document size and layer bounds.
func TestCrop_CommitIsUndoable(t *testing.T) {
	h, _ := makeDocWithLayer(t, 100, 80)
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginCrop, `{}`); err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandUpdateCrop, mustJSON(t, UpdateCropPayload{
		X: 10, Y: 5, W: 60, H: 40,
	})); err != nil {
		t.Fatalf("UpdateCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandCommitCrop, `{}`); err != nil {
		t.Fatalf("CommitCrop: %v", err)
	}

	// Undo.
	if _, err := DispatchCommand(h, commandUndo, `{}`); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	doc := instances[h].manager.Active()
	if doc.Width != 100 || doc.Height != 80 {
		t.Errorf("doc size after undo = %dx%d, want 100x80", doc.Width, doc.Height)
	}
	pl := doc.ActiveLayer().(*PixelLayer)
	if pl.Bounds.X != 0 || pl.Bounds.Y != 0 {
		t.Errorf("layer bounds after undo = (%d,%d), want (0,0)", pl.Bounds.X, pl.Bounds.Y)
	}
}

// TestCrop_CommitErrorsWithoutActiveCrop verifies that CommitCrop errors
// when the crop tool has not been started.
func TestCrop_CommitErrorsWithoutActiveCrop(t *testing.T) {
	h, _ := makeDocWithLayer(t, 50, 50)
	defer Free(h)

	_, err := DispatchCommand(h, commandCommitCrop, `{}`)
	if err == nil {
		t.Error("expected error from CommitCrop without active crop, got nil")
	}
}

func TestApplyRotatedCropToPixelLayerSolidPixels(t *testing.T) {
	pl := &PixelLayer{
		layerBase: newLayerBase("Layer"),
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    makeSolidPixels(2, 2, 200, 100, 50, 255),
	}

	pixels, bounds := applyRotatedCropToPixelLayer(pl, 1, 1, 2, 2, 0)
	if bounds != (LayerBounds{X: 0, Y: 0, W: 2, H: 2}) {
		t.Fatalf("crop bounds = %+v, want {X:0 Y:0 W:2 H:2}", bounds)
	}
	if len(pixels) != 16 {
		t.Fatalf("cropped pixel length = %d, want 16", len(pixels))
	}
	for index := 0; index < len(pixels); index += 4 {
		if pixels[index] != 200 || pixels[index+1] != 100 || pixels[index+2] != 50 || pixels[index+3] != 255 {
			t.Fatalf("pixel %d = %v, want solid source color", index/4, pixels[index:index+4])
		}
	}
}

func TestTrimPixelLayerToBoundsClearsOutsidePixels(t *testing.T) {
	pl := &PixelLayer{
		layerBase: newLayerBase("Layer"),
		Bounds:    LayerBounds{X: -1, Y: 0, W: 2, H: 1},
		Pixels: []byte{
			255, 0, 0, 255,
			0, 255, 0, 255,
		},
	}

	trimPixelLayerToBounds(pl, 1, 1)
	if got := pl.Pixels[:4]; got[0] != 0 || got[1] != 0 || got[2] != 0 || got[3] != 0 {
		t.Fatalf("outside pixel = %v, want fully cleared", got)
	}
	if got := pl.Pixels[4:8]; got[0] != 0 || got[1] != 255 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("inside pixel = %v, want unchanged", got)
	}
}
