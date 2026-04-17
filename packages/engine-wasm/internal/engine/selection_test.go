package engine

import (
	"bytes"
	"testing"
)

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

func TestSelectionSaveLoadChannelAndRefine(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:   "Saved Selection",
		Width:  6,
		Height: 6,
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 6, H: 6},
		Pixels:    filledPixels(6, 6, [4]byte{200, 100, 50, 255}),
	})); err != nil {
		t.Fatalf("add layer: %v", err)
	}

	if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Rect:  LayerBounds{X: 1, Y: 1, W: 3, H: 3},
	})); err != nil {
		t.Fatalf("new selection: %v", err)
	}

	saved, err := DispatchCommand(h, commandSaveSelectionToChannel, mustJSON(t, SaveSelectionToChannelPayload{Name: "Alpha 1"}))
	if err != nil {
		t.Fatalf("save selection: %v", err)
	}
	if len(saved.UIMeta.SavedSelectionChannels) != 1 || saved.UIMeta.SavedSelectionChannels[0].Name != "Alpha 1" {
		t.Fatalf("saved channels = %+v, want Alpha 1", saved.UIMeta.SavedSelectionChannels)
	}

	doc := instances[h].manager.Active()
	beforeRefine := append([]byte(nil), doc.Selection.Mask...)

	if _, err := DispatchCommand(h, commandRefineSelection, mustJSON(t, RefineSelectionPayload{
		SmartRadius: 1.5,
		Contrast:    50,
	})); err != nil {
		t.Fatalf("refine selection: %v", err)
	}
	afterRefine := instances[h].manager.Active().Selection
	if afterRefine == nil || bytes.Equal(beforeRefine, afterRefine.Mask) {
		t.Fatal("refine selection should change the mask")
	}

	if _, err := DispatchCommand(h, commandDeselect, ""); err != nil {
		t.Fatalf("deselect: %v", err)
	}
	loaded, err := DispatchCommand(h, commandLoadSelectionFromChannel, mustJSON(t, LoadSelectionFromChannelPayload{Name: "Alpha 1"}))
	if err != nil {
		t.Fatalf("load selection: %v", err)
	}
	if !loaded.UIMeta.Selection.Active || loaded.UIMeta.Selection.PixelCount != 9 {
		t.Fatalf("loaded selection meta = %+v, want active 3x3 selection", loaded.UIMeta.Selection)
	}
}

func TestSelectionOutputModes(t *testing.T) {
	t.Run("NewLayer", func(t *testing.T) {
		h := Init("")
		defer Free(h)

		if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
			Name:   "Output Layer",
			Width:  4,
			Height: 2,
		})); err != nil {
			t.Fatalf("create document: %v", err)
		}
		added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
			LayerType: LayerTypePixel,
			Name:      "Base",
			Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 2},
			Pixels: []byte{
				10, 0, 0, 255, 20, 0, 0, 255, 30, 0, 0, 255, 40, 0, 0, 255,
				50, 0, 0, 255, 60, 0, 0, 255, 70, 0, 0, 255, 80, 0, 0, 255,
			},
		}))
		if err != nil {
			t.Fatalf("add layer: %v", err)
		}
		layerID := added.UIMeta.ActiveLayerID
		if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
			Shape: SelectionShapeRect,
			Rect:  LayerBounds{X: 1, Y: 0, W: 2, H: 1},
		})); err != nil {
			t.Fatalf("new selection: %v", err)
		}
		if _, err := DispatchCommand(h, commandOutputSelection, mustJSON(t, OutputSelectionPayload{
			Mode:    OutputSelectionNewLayer,
			LayerID: layerID,
		})); err != nil {
			t.Fatalf("output selection new layer: %v", err)
		}
		doc := instances[h].manager.Active()
		if len(doc.ensureLayerRoot().Children()) != 2 {
			t.Fatalf("layer count = %d, want 2", len(doc.ensureLayerRoot().Children()))
		}
		layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID)
		if !ok {
			t.Fatal("new layer not found")
		}
		pixel, ok := layer.(*PixelLayer)
		if !ok {
			t.Fatalf("new layer type = %T, want *PixelLayer", layer)
		}
		if pixel.Bounds != (LayerBounds{X: 1, Y: 0, W: 2, H: 1}) {
			t.Fatalf("new layer bounds = %+v, want x=1 y=0 w=2 h=1", pixel.Bounds)
		}
		if pixel.Pixels[3] == 0 || pixel.Pixels[7] == 0 {
			t.Fatalf("new layer alpha = [%d %d], want opaque selected pixels", pixel.Pixels[3], pixel.Pixels[7])
		}
	})

	t.Run("NewLayerWithMask", func(t *testing.T) {
		h := Init("")
		defer Free(h)

		if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
			Name:   "Output Mask",
			Width:  3,
			Height: 1,
		})); err != nil {
			t.Fatalf("create document: %v", err)
		}
		added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
			LayerType: LayerTypePixel,
			Name:      "Base",
			Bounds:    LayerBounds{X: 0, Y: 0, W: 3, H: 1},
			Pixels: []byte{
				10, 0, 0, 255, 20, 0, 0, 255, 30, 0, 0, 255,
			},
		}))
		if err != nil {
			t.Fatalf("add layer: %v", err)
		}
		if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
			Shape: SelectionShapeRect,
			Rect:  LayerBounds{X: 1, Y: 0, W: 1, H: 1},
		})); err != nil {
			t.Fatalf("new selection: %v", err)
		}
		if _, err := DispatchCommand(h, commandOutputSelection, mustJSON(t, OutputSelectionPayload{
			Mode:    OutputSelectionNewLayerWithMask,
			LayerID: added.UIMeta.ActiveLayerID,
		})); err != nil {
			t.Fatalf("output selection new layer with mask: %v", err)
		}
		doc := instances[h].manager.Active()
		layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID)
		if !ok {
			t.Fatal("masked layer not found")
		}
		if layer.Mask() == nil {
			t.Fatal("expected new layer mask")
		}
	})

	t.Run("Document", func(t *testing.T) {
		h := Init("")
		defer Free(h)

		if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
			Name:   "Output Document",
			Width:  4,
			Height: 2,
		})); err != nil {
			t.Fatalf("create document: %v", err)
		}
		if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
			LayerType: LayerTypePixel,
			Name:      "Base",
			Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 2},
			Pixels:    filledPixels(4, 2, [4]byte{120, 90, 60, 255}),
		})); err != nil {
			t.Fatalf("add layer: %v", err)
		}
		if _, err := DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
			Shape: SelectionShapeRect,
			Rect:  LayerBounds{X: 1, Y: 0, W: 2, H: 2},
		})); err != nil {
			t.Fatalf("new selection: %v", err)
		}
		exported, err := DispatchCommand(h, commandOutputSelection, mustJSON(t, OutputSelectionPayload{
			Mode: OutputSelectionDocument,
		}))
		if err != nil {
			t.Fatalf("output selection document: %v", err)
		}
		if exported.UIMeta.DocumentWidth != 2 || exported.UIMeta.DocumentHeight != 2 {
			t.Fatalf("new document size = %dx%d, want 2x2", exported.UIMeta.DocumentWidth, exported.UIMeta.DocumentHeight)
		}
		doc := instances[h].manager.Active()
		if len(doc.ensureLayerRoot().Children()) != 1 {
			t.Fatalf("new document layer count = %d, want 1", len(doc.ensureLayerRoot().Children()))
		}
	})
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

func TestMagicWandGlobalAndContiguousModes(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:   "Magic Wand",
		Width:  5,
		Height: 1,
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Colors",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 5, H: 1},
		Pixels: []byte{
			255, 0, 0, 255,
			255, 0, 0, 255,
			0, 255, 0, 255,
			255, 0, 0, 255,
			255, 0, 0, 255,
		},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	global, err := DispatchCommand(h, commandMagicWand, mustJSON(t, MagicWandPayload{
		X:          0,
		Y:          0,
		Tolerance:  1,
		LayerID:    layerID,
		Contiguous: false,
	}))
	if err != nil {
		t.Fatalf("magic wand global: %v", err)
	}
	if global.UIMeta.Selection.PixelCount != 4 {
		t.Fatalf("magic wand global pixelCount = %d, want 4", global.UIMeta.Selection.PixelCount)
	}

	if _, err := DispatchCommand(h, commandDeselect, ""); err != nil {
		t.Fatalf("deselect: %v", err)
	}

	contiguous, err := DispatchCommand(h, commandMagicWand, mustJSON(t, MagicWandPayload{
		X:          0,
		Y:          0,
		Tolerance:  1,
		LayerID:    layerID,
		Contiguous: true,
	}))
	if err != nil {
		t.Fatalf("magic wand contiguous: %v", err)
	}
	if contiguous.UIMeta.Selection.PixelCount != 2 {
		t.Fatalf("magic wand contiguous pixelCount = %d, want 2", contiguous.UIMeta.Selection.PixelCount)
	}
}

func TestMagicWandAntiAliasSoftensEdge(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:   "Magic Wand AA",
		Width:  3,
		Height: 1,
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Hard Edge",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 3, H: 1},
		Pixels: []byte{
			255, 0, 0, 255,
			255, 0, 0, 255,
			0, 0, 255, 255,
		},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	hard, err := DispatchCommand(h, commandMagicWand, mustJSON(t, MagicWandPayload{
		X:          0,
		Y:          0,
		Tolerance:  1,
		LayerID:    layerID,
		Contiguous: true,
		AntiAlias:  false,
	}))
	if err != nil {
		t.Fatalf("magic wand hard edge: %v", err)
	}
	if hard.UIMeta.Selection.Bounds == nil {
		t.Fatal("hard-edge magic wand should produce bounds")
	}
	hardDoc := instances[h].manager.Active()
	hardMask := append([]byte(nil), hardDoc.Selection.Mask...)

	if _, err := DispatchCommand(h, commandDeselect, ""); err != nil {
		t.Fatalf("deselect: %v", err)
	}

	soft, err := DispatchCommand(h, commandMagicWand, mustJSON(t, MagicWandPayload{
		X:          0,
		Y:          0,
		Tolerance:  1,
		LayerID:    layerID,
		Contiguous: true,
		AntiAlias:  true,
	}))
	if err != nil {
		t.Fatalf("magic wand anti-aliased: %v", err)
	}
	softDoc := instances[h].manager.Active()
	softMask := softDoc.Selection.Mask
	// Hard-edge: both red pixels fully selected, blue pixel not selected.
	if hardMask[0] != 255 {
		t.Fatalf("hard-edge mask pixel[0] = %d, want 255", hardMask[0])
	}
	if hardMask[1] != 255 {
		t.Fatalf("hard-edge mask pixel[1] = %d, want 255", hardMask[1])
	}
	if hardMask[2] != 0 {
		t.Fatalf("hard-edge mask pixel[2] (blue) = %d, want 0", hardMask[2])
	}
	// Anti-alias: interior pixel[0] stays fully selected, boundary pixel[1]
	// is softened, exterior pixel[2] must NOT bleed outward.
	if softMask[0] != 255 {
		t.Fatalf("anti-aliased interior pixel[0] = %d, want 255", softMask[0])
	}
	if softMask[1] == 0 || softMask[1] == 255 {
		t.Fatalf("anti-aliased boundary pixel[1] = %d, want value between 1 and 254", softMask[1])
	}
	if softMask[2] != 0 {
		t.Fatalf("anti-aliased exterior pixel[2] = %d, want 0 (no outward bleed)", softMask[2])
	}
	_ = soft
}

func rgbaPixelAt(pixels []byte, width, x, y int) [4]byte {
	index := (y*width + x) * 4
	return [4]byte{pixels[index], pixels[index+1], pixels[index+2], pixels[index+3]}
}
