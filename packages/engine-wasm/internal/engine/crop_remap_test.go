package engine

import "testing"

// rasterHasContent reports whether an RGBA buffer contains any non-transparent
// pixel (alpha > 0).
func rasterHasContent(buf []byte) bool {
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 0 {
			return true
		}
	}
	return false
}

// setupRemapDoc builds a 100x80 document containing:
//   - a full-size pixel layer with a mask created from a rect selection
//   - a text layer at (40,30)
//   - a vector (rect) layer
//   - an active rectangular selection
//
// It returns the handle and the pixel/text/vector layer IDs.
func setupRemapDoc(t *testing.T) (h int32, pixelID, textID, vectorID string) {
	t.Helper()
	h = Init("")
	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Remap", Width: 100, Height: 80, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	pixelRes, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Pixel",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 100, H: 80},
		Pixels:    makeSolidPixels(100, 80, 200, 100, 50, 255),
	}))
	if err != nil {
		t.Fatalf("add pixel layer: %v", err)
	}
	pixelID = pixelRes.UIMeta.ActiveLayerID

	// Mask from a distinctive rect selection at (20,20,15,15).
	if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect, Mode: SelectionCombineReplace,
		Rect: LayerBounds{X: 20, Y: 20, W: 15, H: 15},
	})); err != nil {
		t.Fatalf("mask selection: %v", err)
	}
	if _, err := DispatchCommand(h, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{
		LayerID: pixelID, Mode: AddLayerMaskFromSelection,
	})); err != nil {
		t.Fatalf("add layer mask: %v", err)
	}

	textRes, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Text",
		Bounds:    LayerBounds{X: 40, Y: 30, W: 50, H: 24},
		Text:      "Hi",
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add text layer: %v", err)
	}
	textID = textRes.UIMeta.ActiveLayerID

	vecRes, err := DispatchCommand(h, commandDrawShape, mustJSON(t, DrawShapePayload{
		ShapeType: "rect", X: 20, Y: 15, W: 30, H: 20,
		FillColor: [4]uint8{255, 0, 0, 255}, Mode: "shape",
	}))
	if err != nil {
		t.Fatalf("draw shape: %v", err)
	}
	vectorID = vecRes.UIMeta.ActiveLayerID

	// Active selection for the selection-remap assertion.
	if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect, Mode: SelectionCombineReplace,
		Rect: LayerBounds{X: 15, Y: 12, W: 20, H: 20},
	})); err != nil {
		t.Fatalf("active selection: %v", err)
	}
	return h, pixelID, textID, vectorID
}

// TestCrop_RemapsAllLayerKinds verifies that committing a crop remaps every
// layer kind, its mask, and the document selection into the new coordinate
// space (crop from (10,5) → offset (-10,-5), new size 60x40).
func TestCrop_RemapsAllLayerKinds(t *testing.T) {
	h, pixelID, textID, vectorID := setupRemapDoc(t)
	defer Free(h)

	// Point-text bounds are computed (tight around the anchor), so pin the
	// crop remap as a delta against the pre-crop bounds.
	textBoundsBefore := instances[h].manager.Active().findLayer(textID).(*TextLayer).Bounds

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

	doc := instances[h].manager.Active()
	if doc.Width != 60 || doc.Height != 40 {
		t.Fatalf("doc size = %dx%d, want 60x40", doc.Width, doc.Height)
	}

	// Pixel layer: bounds shifted, mask cropped/offset to new document space.
	pl := doc.findLayer(pixelID).(*PixelLayer)
	if pl.Bounds.X != -10 || pl.Bounds.Y != -5 {
		t.Errorf("pixel bounds = (%d,%d), want (-10,-5)", pl.Bounds.X, pl.Bounds.Y)
	}
	mask := pl.Mask()
	if mask == nil {
		t.Fatal("pixel mask lost after crop")
	}
	if mask.Width != 60 || mask.Height != 40 {
		t.Errorf("mask size = %dx%d, want 60x40", mask.Width, mask.Height)
	}
	if len(mask.Data) != 60*40 {
		t.Fatalf("mask data length = %d, want %d", len(mask.Data), 60*40)
	}
	// Original mask active region (20,20,15,15) → shifted to (10,15,15,15).
	if got := mask.Data[15*60+10]; got != 255 {
		t.Errorf("mask at shifted active pixel (10,15) = %d, want 255", got)
	}
	if got := mask.Data[0]; got != 0 {
		t.Errorf("mask at (0,0) = %d, want 0 (outside active region)", got)
	}

	// Text layer: position shifted by (-10,-5); bounds-local raster still
	// valid & non-blank.
	tl := doc.findLayer(textID).(*TextLayer)
	if tl.Bounds.X != textBoundsBefore.X-10 || tl.Bounds.Y != textBoundsBefore.Y-5 {
		t.Errorf("text bounds = (%d,%d), want (%d,%d)", tl.Bounds.X, tl.Bounds.Y, textBoundsBefore.X-10, textBoundsBefore.Y-5)
	}
	if tl.AnchorX != 30 || tl.AnchorY != 25 {
		t.Errorf("text anchor = (%v,%v), want (30,25)", tl.AnchorX, tl.AnchorY)
	}
	if len(tl.CachedRaster) != tl.Bounds.W*tl.Bounds.H*4 {
		t.Errorf("text raster length = %d, want %d", len(tl.CachedRaster), tl.Bounds.W*tl.Bounds.H*4)
	}
	if !rasterHasContent(tl.CachedRaster) {
		t.Error("text raster is blank after crop")
	}

	// Vector layer: shape points shifted, bounds/raster resized to new document.
	vl := doc.findLayer(vectorID).(*VectorLayer)
	if vl.Bounds != (LayerBounds{X: 0, Y: 0, W: 60, H: 40}) {
		t.Errorf("vector bounds = %+v, want {0 0 60 40}", vl.Bounds)
	}
	if len(vl.CachedRaster) != 60*40*4 {
		t.Errorf("vector raster length = %d, want %d", len(vl.CachedRaster), 60*40*4)
	}
	if !rasterHasContent(vl.CachedRaster) {
		t.Error("vector raster is blank after crop")
	}
	// Rect shape's first anchor started at (20,15) → shifted to (10,10).
	first := vl.Shape.Subpaths[0].Points[0]
	if first.X != 10 || first.Y != 10 {
		t.Errorf("vector first point = (%.0f,%.0f), want (10,10)", first.X, first.Y)
	}

	// Selection: translated by (-10,-5) and clipped; region (15,12,20,20) → (5,7,20,20).
	if doc.Selection == nil {
		t.Fatal("selection lost after crop")
	}
	if doc.Selection.Width != 60 || doc.Selection.Height != 40 {
		t.Errorf("selection size = %dx%d, want 60x40", doc.Selection.Width, doc.Selection.Height)
	}
	if b, ok := doc.Selection.Bounds(); !ok || b.X != 5 || b.Y != 7 || b.W != 20 || b.H != 20 {
		t.Errorf("selection bounds = %+v (ok=%v), want {5 7 20 20}", b, ok)
	}
}

// TestCrop_RemapUndoRestores verifies the crop remap participates in the
// snapshot history so undo restores the original document space.
func TestCrop_RemapUndoRestores(t *testing.T) {
	h, pixelID, textID, vectorID := setupRemapDoc(t)
	defer Free(h)

	textBoundsBefore := instances[h].manager.Active().findLayer(textID).(*TextLayer).Bounds

	if _, err := DispatchCommand(h, commandBeginCrop, `{}`); err != nil {
		t.Fatalf("BeginCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandUpdateCrop, mustJSON(t, UpdateCropPayload{X: 10, Y: 5, W: 60, H: 40})); err != nil {
		t.Fatalf("UpdateCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandCommitCrop, `{}`); err != nil {
		t.Fatalf("CommitCrop: %v", err)
	}
	if _, err := DispatchCommand(h, commandUndo, `{}`); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	doc := instances[h].manager.Active()
	if doc.Width != 100 || doc.Height != 80 {
		t.Fatalf("doc size after undo = %dx%d, want 100x80", doc.Width, doc.Height)
	}
	tl := doc.findLayer(textID).(*TextLayer)
	if tl.Bounds != textBoundsBefore {
		t.Errorf("text bounds after undo = %+v, want %+v", tl.Bounds, textBoundsBefore)
	}
	if tl.AnchorX != 40 || tl.AnchorY != 30 {
		t.Errorf("text anchor after undo = (%v,%v), want (40,30)", tl.AnchorX, tl.AnchorY)
	}
	vl := doc.findLayer(vectorID).(*VectorLayer)
	if vl.Bounds != (LayerBounds{X: 0, Y: 0, W: 100, H: 80}) {
		t.Errorf("vector bounds after undo = %+v, want {0 0 100 80}", vl.Bounds)
	}
	pl := doc.findLayer(pixelID).(*PixelLayer)
	if m := pl.Mask(); m == nil || m.Width != 100 || m.Height != 80 {
		t.Errorf("mask after undo = %+v, want 100x80", m)
	}
}

// TestCanvasSize_RemapGrow verifies Canvas Size (grow) with a center anchor
// remaps all layer kinds, masks and the selection by the anchor offset.
func TestCanvasSize_RemapGrow(t *testing.T) {
	h, pixelID, textID, vectorID := setupRemapDoc(t)
	defer Free(h)

	textBoundsBefore := instances[h].manager.Active().findLayer(textID).(*TextLayer).Bounds

	// 100x80 → 140x120, center anchor: dx=20, dy=20.
	if _, err := DispatchCommand(h, commandResizeCanvas, mustJSON(t, ResizeCanvasPayload{
		Width: 140, Height: 120, Anchor: "center",
	})); err != nil {
		t.Fatalf("ResizeCanvas: %v", err)
	}

	doc := instances[h].manager.Active()
	if doc.Width != 140 || doc.Height != 120 {
		t.Fatalf("doc size = %dx%d, want 140x120", doc.Width, doc.Height)
	}

	pl := doc.findLayer(pixelID).(*PixelLayer)
	if pl.Bounds.X != 20 || pl.Bounds.Y != 20 {
		t.Errorf("pixel bounds = (%d,%d), want (20,20)", pl.Bounds.X, pl.Bounds.Y)
	}
	mask := pl.Mask()
	if mask == nil || mask.Width != 140 || mask.Height != 120 {
		t.Fatalf("mask size = %+v, want 140x120", mask)
	}
	// Original active region (20,20,15,15) → shifted to (40,40,15,15).
	if got := mask.Data[40*140+40]; got != 255 {
		t.Errorf("mask at shifted pixel (40,40) = %d, want 255", got)
	}

	tl := doc.findLayer(textID).(*TextLayer)
	if tl.Bounds.X != textBoundsBefore.X+20 || tl.Bounds.Y != textBoundsBefore.Y+20 {
		t.Errorf("text bounds = (%d,%d), want (%d,%d)", tl.Bounds.X, tl.Bounds.Y, textBoundsBefore.X+20, textBoundsBefore.Y+20)
	}
	if tl.AnchorX != 60 || tl.AnchorY != 50 {
		t.Errorf("text anchor = (%v,%v), want (60,50)", tl.AnchorX, tl.AnchorY)
	}
	if !rasterHasContent(tl.CachedRaster) {
		t.Error("text raster blank after grow")
	}

	vl := doc.findLayer(vectorID).(*VectorLayer)
	if vl.Bounds != (LayerBounds{X: 0, Y: 0, W: 140, H: 120}) {
		t.Errorf("vector bounds = %+v, want {0 0 140 120}", vl.Bounds)
	}
	if len(vl.CachedRaster) != 140*120*4 || !rasterHasContent(vl.CachedRaster) {
		t.Errorf("vector raster length = %d (content=%v), want %d non-blank", len(vl.CachedRaster), rasterHasContent(vl.CachedRaster), 140*120*4)
	}
	first := vl.Shape.Subpaths[0].Points[0]
	if first.X != 40 || first.Y != 35 {
		t.Errorf("vector first point = (%.0f,%.0f), want (40,35)", first.X, first.Y)
	}

	if doc.Selection == nil || doc.Selection.Width != 140 || doc.Selection.Height != 120 {
		t.Fatalf("selection = %+v, want 140x120", doc.Selection)
	}
	if b, ok := doc.Selection.Bounds(); !ok || b.X != 35 || b.Y != 32 {
		t.Errorf("selection bounds = %+v (ok=%v), want origin (35,32)", b, ok)
	}
}

// TestCanvasSize_RemapShrink verifies Canvas Size (shrink) with a top-left
// anchor (offset 0,0) still re-rasterizes the document-sized vector raster and
// crops masks to the smaller document even though positions do not move.
func TestCanvasSize_RemapShrink(t *testing.T) {
	h, pixelID, _, vectorID := setupRemapDoc(t)
	defer Free(h)

	// 100x80 → 60x40, top-left anchor: dx=0, dy=0.
	if _, err := DispatchCommand(h, commandResizeCanvas, mustJSON(t, ResizeCanvasPayload{
		Width: 60, Height: 40, Anchor: "top-left",
	})); err != nil {
		t.Fatalf("ResizeCanvas: %v", err)
	}

	doc := instances[h].manager.Active()
	if doc.Width != 60 || doc.Height != 40 {
		t.Fatalf("doc size = %dx%d, want 60x40", doc.Width, doc.Height)
	}

	// Vector raster must be resized to the new (smaller) document dimensions.
	vl := doc.findLayer(vectorID).(*VectorLayer)
	if vl.Bounds != (LayerBounds{X: 0, Y: 0, W: 60, H: 40}) {
		t.Errorf("vector bounds = %+v, want {0 0 60 40}", vl.Bounds)
	}
	if len(vl.CachedRaster) != 60*40*4 {
		t.Errorf("vector raster length = %d, want %d", len(vl.CachedRaster), 60*40*4)
	}
	if !rasterHasContent(vl.CachedRaster) {
		t.Error("vector raster blank after shrink")
	}

	// Mask cropped to the smaller document; active region (20,20,15,15) survives.
	pl := doc.findLayer(pixelID).(*PixelLayer)
	mask := pl.Mask()
	if mask == nil || mask.Width != 60 || mask.Height != 40 || len(mask.Data) != 60*40 {
		t.Fatalf("mask = %+v, want 60x40", mask)
	}
	if got := mask.Data[20*60+20]; got != 255 {
		t.Errorf("mask at (20,20) = %d, want 255", got)
	}
}

// TestCanvasSize_RemapDeselectsWhenOutside verifies that a selection pushed
// fully outside the new document by a shrink is deselected.
func TestCanvasSize_RemapDeselectsWhenOutside(t *testing.T) {
	h, _, _, _ := setupRemapDoc(t)
	defer Free(h)

	// Selection is at (15,12,20,20). Shrink to 10x10 top-left: fully outside.
	if _, err := DispatchCommand(h, commandResizeCanvas, mustJSON(t, ResizeCanvasPayload{
		Width: 10, Height: 10, Anchor: "top-left",
	})); err != nil {
		t.Fatalf("ResizeCanvas: %v", err)
	}
	doc := instances[h].manager.Active()
	if doc.Selection != nil {
		t.Errorf("selection should be deselected when fully outside, got %+v", doc.Selection)
	}
}
