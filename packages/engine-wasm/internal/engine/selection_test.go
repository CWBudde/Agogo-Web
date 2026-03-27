package engine

import "testing"

func TestSelectionCommandsCombineAndUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:   "Selection",
		Width:  8,
		Height: 8,
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	selected, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Rect:  LayerBounds{X: 1, Y: 1, W: 4, H: 4},
	}))
	if err != nil {
		t.Fatalf("new selection: %v", err)
	}
	if !selected.UIMeta.Selection.Active {
		t.Fatal("selection should be active")
	}
	if selected.UIMeta.Selection.PixelCount != 16 {
		t.Fatalf("pixelCount = %d, want 16", selected.UIMeta.Selection.PixelCount)
	}

	added, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Mode:  SelectionCombineAdd,
		Rect:  LayerBounds{X: 4, Y: 1, W: 2, H: 4},
	}))
	if err != nil {
		t.Fatalf("add selection: %v", err)
	}
	if added.UIMeta.Selection.PixelCount != 20 {
		t.Fatalf("pixelCount after add = %d, want 20", added.UIMeta.Selection.PixelCount)
	}

	subtracted, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Mode:  SelectionCombineSubtract,
		Rect:  LayerBounds{X: 2, Y: 2, W: 2, H: 2},
	}))
	if err != nil {
		t.Fatalf("subtract selection: %v", err)
	}
	if subtracted.UIMeta.Selection.PixelCount != 16 {
		t.Fatalf("pixelCount after subtract = %d, want 16", subtracted.UIMeta.Selection.PixelCount)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo selection: %v", err)
	}
	if undone.UIMeta.Selection.PixelCount != 20 {
		t.Fatalf("pixelCount after undo = %d, want 20", undone.UIMeta.Selection.PixelCount)
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo selection: %v", err)
	}
	if redone.UIMeta.Selection.PixelCount != 16 {
		t.Fatalf("pixelCount after redo = %d, want 16", redone.UIMeta.Selection.PixelCount)
	}
}

func TestSelectionModifyCommandsAndReselect(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:   "Selection Ops",
		Width:  12,
		Height: 12,
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Rect:  LayerBounds{X: 4, Y: 4, W: 2, H: 2},
	})); err != nil {
		t.Fatalf("new selection: %v", err)
	}

	expanded, err := DispatchCommand(h, commandExpandSelection, mustJSON(t, ExpandSelectionPayload{Pixels: 1}))
	if err != nil {
		t.Fatalf("expand selection: %v", err)
	}
	if expanded.UIMeta.Selection.PixelCount <= 4 {
		t.Fatalf("expanded selection pixelCount = %d, want > 4", expanded.UIMeta.Selection.PixelCount)
	}

	bordered, err := DispatchCommand(h, commandBorderSelection, mustJSON(t, BorderSelectionPayload{Width: 1}))
	if err != nil {
		t.Fatalf("border selection: %v", err)
	}
	if bordered.UIMeta.Selection.PixelCount == 0 || bordered.UIMeta.Selection.PixelCount >= expanded.UIMeta.Selection.PixelCount {
		t.Fatalf("border pixelCount = %d, want between 1 and %d", bordered.UIMeta.Selection.PixelCount, expanded.UIMeta.Selection.PixelCount-1)
	}

	feathered, err := DispatchCommand(h, commandFeatherSelection, mustJSON(t, FeatherSelectionPayload{Radius: 1}))
	if err != nil {
		t.Fatalf("feather selection: %v", err)
	}
	if !feathered.UIMeta.Selection.Active {
		t.Fatal("selection should remain active after feather")
	}

	deselected, err := DispatchCommand(h, commandDeselect, "")
	if err != nil {
		t.Fatalf("deselect: %v", err)
	}
	if deselected.UIMeta.Selection.Active {
		t.Fatal("selection should be inactive after deselect")
	}
	if !deselected.UIMeta.Selection.LastSelectionAvailable {
		t.Fatal("last selection should be available after deselect")
	}

	reselected, err := DispatchCommand(h, commandReselect, "")
	if err != nil {
		t.Fatalf("reselect: %v", err)
	}
	if !reselected.UIMeta.Selection.Active {
		t.Fatal("selection should be restored by reselect")
	}
	if reselected.UIMeta.Selection.PixelCount != feathered.UIMeta.Selection.PixelCount {
		t.Fatalf("reselected pixelCount = %d, want %d", reselected.UIMeta.Selection.PixelCount, feathered.UIMeta.Selection.PixelCount)
	}
}

func TestSelectionTransformColorRangeQuickSelectAndMask(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:   "Selection Sources",
		Width:  4,
		Height: 1,
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Colors",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 1},
		Pixels: []byte{
			255, 0, 0, 255,
			255, 0, 0, 255,
			0, 0, 255, 255,
			0, 255, 0, 255,
		},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	colorRange, err := DispatchCommand(h, commandSelectColorRange, mustJSON(t, SelectColorRangePayload{
		LayerID:     layerID,
		TargetColor: [4]uint8{255, 0, 0, 255},
		Fuzziness:   1,
	}))
	if err != nil {
		t.Fatalf("select color range: %v", err)
	}
	if colorRange.UIMeta.Selection.PixelCount != 2 {
		t.Fatalf("color range pixelCount = %d, want 2", colorRange.UIMeta.Selection.PixelCount)
	}

	quick, err := DispatchCommand(h, commandQuickSelect, mustJSON(t, QuickSelectPayload{
		X:         0,
		Y:         0,
		Tolerance: 1,
		LayerID:   layerID,
	}))
	if err != nil {
		t.Fatalf("quick select: %v", err)
	}
	if quick.UIMeta.Selection.PixelCount != 2 {
		t.Fatalf("quick selection pixelCount = %d, want 2", quick.UIMeta.Selection.PixelCount)
	}

	transformed, err := DispatchCommand(h, commandTransformSelection, mustJSON(t, TransformSelectionPayload{
		A:  1,
		D:  1,
		TX: 1,
	}))
	if err != nil {
		t.Fatalf("transform selection: %v", err)
	}
	if transformed.UIMeta.Selection.Bounds == nil || transformed.UIMeta.Selection.Bounds.X != 1 {
		t.Fatalf("transformed bounds = %+v, want x=1", transformed.UIMeta.Selection.Bounds)
	}

	if _, err := DispatchCommand(h, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{LayerID: layerID, Mode: AddLayerMaskFromSelection})); err != nil {
		t.Fatalf("add mask from selection: %v", err)
	}

	doc := instances[h].manager.Active()
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		t.Fatalf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil || len(mask.Data) != doc.Width*doc.Height {
		t.Fatalf("mask = %+v, want document-sized mask", mask)
	}
	if mask.Data[1] == 0 || mask.Data[0] != 0 {
		t.Fatalf("mask data = %v, want translated selection starting at x=1", mask.Data)
	}
}

func TestRenderSelectionOverlayMarches(t *testing.T) {
	doc := &Document{
		Width:      8,
		Height:     8,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("white"),
		Name:       "Overlay",
		Selection:  newRectSelection(8, 8, LayerBounds{X: 2, Y: 2, W: 4, H: 4}),
	}
	vp := &ViewportState{CenterX: 4, CenterY: 4, Zoom: 1, CanvasW: 8, CanvasH: 8, DevicePixelRatio: 1}

	base := RenderViewport(doc, vp, nil, nil)
	first := RenderSelectionOverlay(doc, vp, append([]byte(nil), base...), doc.Selection, 0)
	second := RenderSelectionOverlay(doc, vp, append([]byte(nil), base...), doc.Selection, 8)

	firstPixel := rgbaPixelAt(first, 8, 2, 2)
	secondPixel := rgbaPixelAt(second, 8, 2, 2)
	if firstPixel == secondPixel {
		t.Fatalf("overlay pixel did not animate: first=%v second=%v", firstPixel, secondPixel)
	}
}

func rgbaPixelAt(pixels []byte, width, x, y int) [4]byte {
	index := (y*width + x) * 4
	return [4]byte{pixels[index], pixels[index+1], pixels[index+2], pixels[index+3]}
}
