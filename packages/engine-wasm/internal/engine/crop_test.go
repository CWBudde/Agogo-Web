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
	if crop.Resolution != 72 {
		t.Errorf("crop resolution = %.1f, want 72", crop.Resolution)
	}
	if crop.OverlayType != cropOverlayThirds {
		t.Errorf("crop overlay = %q, want %q", crop.OverlayType, cropOverlayThirds)
	}
	if crop.ContentAwareFill {
		t.Error("crop content-aware fill should default to false")
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
		ContentAwareFill: true,
		Resolution:       300,
		OverlayType:      cropOverlayGrid,
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
	if crop.Resolution != 300 {
		t.Errorf("crop resolution = %.1f, want 300", crop.Resolution)
	}
	if crop.OverlayType != cropOverlayGrid {
		t.Errorf("crop overlay = %q, want %q", crop.OverlayType, cropOverlayGrid)
	}
	if !crop.ContentAwareFill {
		t.Error("crop content-aware fill should be true after UpdateCrop")
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
		X: 10, Y: 5, W: 60, H: 40, Resolution: 144,
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
	if doc.Resolution != 144 {
		t.Errorf("doc resolution = %.1f, want 144 after commit", doc.Resolution)
	}
}

func TestCrop_CommitContentAwareFillCreatesExpansionLayer(t *testing.T) {
	h, layerID := makeDocWithLayer(t, 2, 2)
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginCrop, `{}`); err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandUpdateCrop, mustJSON(t, UpdateCropPayload{
		X: -1, Y: 0, W: 3, H: 2, ContentAwareFill: true,
	})); err != nil {
		t.Fatalf("UpdateCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandCommitCrop, `{}`); err != nil {
		t.Fatalf("CommitCrop: %v", err)
	}

	doc := instances[h].manager.Active()
	if doc.Width != 3 || doc.Height != 2 {
		t.Fatalf("doc size after content-aware crop = %dx%d, want 3x2", doc.Width, doc.Height)
	}
	if doc.ActiveLayerID != layerID {
		t.Fatalf("active layer after content-aware crop = %q, want %q", doc.ActiveLayerID, layerID)
	}
	if len(doc.ensureLayerRoot().Children()) != 2 {
		t.Fatalf("layer count after content-aware crop = %d, want 2", len(doc.ensureLayerRoot().Children()))
	}
	fillLayer, ok := doc.ensureLayerRoot().Children()[1].(*PixelLayer)
	if !ok {
		t.Fatal("expected content-aware crop fill to create a pixel layer")
	}
	if fillLayer.Name() != "Content-Aware Crop Fill" {
		t.Fatalf("fill layer name = %q, want Content-Aware Crop Fill", fillLayer.Name())
	}

	surface := doc.renderCompositeSurface()
	leftPixel := surface[:4]
	want := [4]byte{200, 100, 50, 255}
	if leftPixel[0] != want[0] || leftPixel[1] != want[1] || leftPixel[2] != want[2] || leftPixel[3] != want[3] {
		t.Fatalf("left expansion pixel = %v, want %v", leftPixel, want)
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
	if doc.Resolution != 72 {
		t.Errorf("doc resolution after undo = %.1f, want 72", doc.Resolution)
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

func TestApplyResizeCanvasAnchorsAndValidation(t *testing.T) {
	root := func() (*Document, *PixelLayer) {
		pixel := NewPixelLayer("Layer", LayerBounds{X: 5, Y: 7, W: 8, H: 6}, makeSolidPixels(8, 6, 200, 100, 50, 255))
		group := NewGroupLayer("Group")
		group.SetChildren([]LayerNode{pixel})
		layerRoot := NewGroupLayer("Root")
		layerRoot.SetChildren([]LayerNode{group})
		return &Document{Width: 100, Height: 80, LayerRoot: layerRoot}, pixel
	}

	doc, _ := root()
	if err := applyResizeCanvas(doc, 0, 80, "center"); err == nil {
		t.Fatal("expected invalid canvas dimensions error")
	}

	tests := []struct {
		name   string
		anchor string
		wantDX int
		wantDY int
	}{
		{name: "top-left", anchor: "top-left", wantDX: 0, wantDY: 0},
		{name: "top-center", anchor: "top-center", wantDX: 20, wantDY: 0},
		{name: "top-right", anchor: "top-right", wantDX: 40, wantDY: 0},
		{name: "middle-left", anchor: "middle-left", wantDX: 0, wantDY: 20},
		{name: "center", anchor: "center", wantDX: 20, wantDY: 20},
		{name: "middle-right", anchor: "middle-right", wantDX: 40, wantDY: 20},
		{name: "bottom-left", anchor: "bottom-left", wantDX: 0, wantDY: 40},
		{name: "bottom-center", anchor: "bottom-center", wantDX: 20, wantDY: 40},
		{name: "bottom-right", anchor: "bottom-right", wantDX: 40, wantDY: 40},
		{name: "fallback", anchor: "unexpected", wantDX: 0, wantDY: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, pixel := root()
			if err := applyResizeCanvas(doc, 140, 120, tc.anchor); err != nil {
				t.Fatalf("applyResizeCanvas: %v", err)
			}
			if doc.Width != 140 || doc.Height != 120 {
				t.Fatalf("document size = %dx%d, want 140x120", doc.Width, doc.Height)
			}
			if pixel.Bounds.X != 5+tc.wantDX || pixel.Bounds.Y != 7+tc.wantDY {
				t.Fatalf("pixel bounds = (%d,%d), want (%d,%d)", pixel.Bounds.X, pixel.Bounds.Y, 5+tc.wantDX, 7+tc.wantDY)
			}
		})
	}
}
