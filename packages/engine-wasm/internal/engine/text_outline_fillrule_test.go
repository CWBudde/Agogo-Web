package engine

import "testing"

// interiorInkHoles counts pixels that are well inside the text layer's ink
// (alpha >= 250 across a 3x3 neighborhood of the bounds-local text raster)
// but FULLY TRANSPARENT (alpha == 0) at the corresponding document pixel of
// the doc-sized outline raster. The one-pixel erosion keeps anti-aliased edge
// fringes out of the comparison; genuine even-odd XOR holes are interior
// regions and survive it.
func interiorInkHoles(textRaster []byte, textBounds LayerBounds, outline []byte, docW, docH int) int {
	holes := 0
	w, h := textBounds.W, textBounds.H
	alphaAt := func(x, y int) uint8 {
		return textRaster[(y*w+x)*4+3]
	}
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			interior := true
			for dy := -1; dy <= 1 && interior; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if alphaAt(x+dx, y+dy) < 250 {
						interior = false
						break
					}
				}
			}
			if !interior {
				continue
			}
			docX := textBounds.X + x
			docY := textBounds.Y + y
			if docX < 0 || docY < 0 || docX >= docW || docY >= docH {
				continue
			}
			if outline[(docY*docW+docX)*4+3] == 0 {
				holes++
			}
		}
	}
	return holes
}

// TestConvertTextToPath_OverlappingGlyphs_NoEvenOddHoles is a regression test
// for Create Outlines rasterizing the converted vector layer with the even-odd
// fill rule while live text renders non-zero: contours of ADJACENT glyphs that
// overlap (negative tracking) XOR out to transparent holes under even-odd,
// while the original text raster fills them solid.
func TestConvertTextToPath_OverlappingGlyphs_NoEvenOddHoles(t *testing.T) {
	h := Init(`{"documentWidth":300,"documentHeight":150,"background":"transparent","resolution":72}`)
	if h <= 0 {
		t.Fatalf("Init returned invalid handle %d", h)
	}
	defer Free(h)

	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Overlap",
		Bounds:    LayerBounds{X: 60, Y: 90, W: 200, H: 60},
		Text:      "AV",
		FontSize:  48,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	layerID := addResult.UIMeta.ActiveLayerID

	// Negative tracking pulls the V into the A so their outlines overlap.
	tracking := -15.0
	if _, err := DispatchCommand(h, commandSetTextStyle, mustJSON(t, SetTextStylePayload{
		LayerID:  layerID,
		Tracking: &tracking,
	})); err != nil {
		t.Fatalf("SetTextStyle(tracking): %v", err)
	}

	inst, ok := instances[h]
	if !ok {
		t.Fatalf("no instance for handle %d", h)
	}
	doc := inst.manager.activeMut()
	tl, ok := doc.findLayer(layerID).(*TextLayer)
	if !ok {
		t.Fatalf("expected text layer, got %T", doc.findLayer(layerID))
	}
	textRaster := append([]byte(nil), tl.CachedRaster...)
	textBounds := tl.Bounds
	textColor := tl.Color
	if len(textRaster) != textBounds.W*textBounds.H*4 {
		t.Fatalf("unexpected text raster size %d for bounds %+v", len(textRaster), textBounds)
	}

	convResult, err := DispatchCommand(h, commandConvertTextToPath, mustJSON(t, ConvertTextToPathPayload{
		LayerID: layerID,
	}))
	if err != nil {
		t.Fatalf("ConvertTextToPath: %v", err)
	}

	doc = inst.manager.activeMut()
	vl, ok := doc.findLayer(convResult.UIMeta.ActiveLayerID).(*VectorLayer)
	if !ok {
		t.Fatalf("expected vector layer, got %T", doc.findLayer(convResult.UIMeta.ActiveLayerID))
	}

	// Setup sanity: the same outline path rasterized even-odd MUST show XOR
	// holes inside the text ink, i.e. the glyph outlines genuinely overlap.
	// If this fails the tracking value no longer produces overlap — the test
	// premise, not the code under test, is broken.
	evenOdd, err := rasterizeVectorShape(vl.Shape, doc.Width, doc.Height, textColor, [4]uint8{}, 0)
	if err != nil {
		t.Fatalf("even-odd reference rasterization: %v", err)
	}
	if holes := interiorInkHoles(textRaster, textBounds, evenOdd, doc.Width, doc.Height); holes == 0 {
		t.Fatal("test setup: even-odd rasterization shows no XOR holes; glyph outlines do not overlap — increase negative tracking")
	}

	// The converted layer's actual raster must fill the overlap solid: no
	// pixel opaque well inside the text ink may be fully transparent in it.
	if len(vl.CachedRaster) != doc.Width*doc.Height*4 {
		t.Fatalf("unexpected vector raster size %d for %dx%d document", len(vl.CachedRaster), doc.Width, doc.Height)
	}
	if holes := interiorInkHoles(textRaster, textBounds, vl.CachedRaster, doc.Width, doc.Height); holes > 0 {
		t.Fatalf("converted outline raster has %d fully-transparent holes inside solid text ink (even-odd fill applied to overlapping glyph contours)", holes)
	}
}
