package engine

import (
	"testing"
)

// setupOutlineTextLayer creates a 200×100 test document with a single point
// text layer and returns the handle and the layer's id.
func setupOutlineTextLayer(t *testing.T, content string, fontSize float64, x, y int) (int32, string) {
	t.Helper()
	h := initTextTestDoc(t)
	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Text",
		Bounds:    LayerBounds{X: x, Y: y, W: 180, H: 80},
		Text:      content,
		FontSize:  fontSize,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	return h, addResult.UIMeta.ActiveLayerID
}

// storedTextLayer fetches the live *TextLayer for id from the instance.
func storedTextLayer(t *testing.T, h int32, id string) *TextLayer {
	t.Helper()
	inst, ok := instances[h]
	if !ok {
		t.Fatalf("no instance for handle %d", h)
	}
	layer := inst.manager.activeMut().findLayer(id)
	tl, ok := layer.(*TextLayer)
	if !ok {
		t.Fatalf("layer %q is %T, want *TextLayer", id, layer)
	}
	return tl
}

// docInkMask converts a bounds-local RGBA raster into a document-space ink
// mask (alpha >= 128).
func docInkMask(raster []byte, bounds LayerBounds, docW, docH int) []bool {
	mask := make([]bool, docW*docH)
	for y := 0; y < bounds.H; y++ {
		dy := bounds.Y + y
		if dy < 0 || dy >= docH {
			continue
		}
		for x := 0; x < bounds.W; x++ {
			dx := bounds.X + x
			if dx < 0 || dx >= docW {
				continue
			}
			if raster[(y*bounds.W+x)*4+3] >= 128 {
				mask[dy*docW+dx] = true
			}
		}
	}
	return mask
}

// maskIoU returns intersection-over-union of two equally sized ink masks.
func maskIoU(a, b []bool) float64 {
	inter, union := 0, 0
	for i := range a {
		switch {
		case a[i] && b[i]:
			inter++
			union++
		case a[i] || b[i]:
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// maskBBox returns the inclusive bounding box of set pixels.
func maskBBox(mask []bool, w, h int) (x0, y0, x1, y1 int, ok bool) {
	x0, y0, x1, y1 = w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask[y*w+x] {
				if x < x0 {
					x0 = x
				}
				if x > x1 {
					x1 = x
				}
				if y < y0 {
					y0 = y
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	return x0, y0, x1, y1, x1 >= 0
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// convertToPath dispatches ConvertTextToPath and returns the resulting
// VectorLayer.
func convertToPath(t *testing.T, h int32, layerID string) *VectorLayer {
	t.Helper()
	result, err := DispatchCommand(h, commandConvertTextToPath, mustJSON(t, ConvertTextToPathPayload{
		LayerID: layerID,
	}))
	if err != nil {
		t.Fatalf("ConvertTextToPath: %v", err)
	}
	inst := instances[h]
	layer := inst.manager.Active().findLayer(result.UIMeta.ActiveLayerID)
	vl, ok := layer.(*VectorLayer)
	if !ok {
		t.Fatalf("active layer after convert is %T, want *VectorLayer", layer)
	}
	return vl
}

func TestBuildTextOutlinePath_ClosedSubpathsWithInitializedHandles(t *testing.T) {
	h, layerID := setupOutlineTextLayer(t, "OiA", 32, 50, 20)
	defer Free(h)
	tl := storedTextLayer(t, h, layerID)

	p := buildTextOutlinePath(tl)
	if p == nil || len(p.Subpaths) == 0 {
		t.Fatal("expected outline path with subpaths")
	}
	for si, sp := range p.Subpaths {
		if !sp.Closed {
			t.Errorf("subpath %d: Closed = false, want true (glyph fill contour)", si)
		}
		if len(sp.Points) < 3 {
			t.Errorf("subpath %d: %d points, want >= 3", si, len(sp.Points))
		}
		for pi, pt := range sp.Points {
			// The glyphs sit far from the document origin, so a handle at
			// (0,0) can only mean it was left uninitialized — path_agg.go's
			// hasNonTrivialHandles would treat it as a curve control point
			// at the origin.
			if pt.InX == 0 && pt.InY == 0 {
				t.Fatalf("subpath %d point %d: In handle uninitialized (0,0), anchor (%v,%v)", si, pi, pt.X, pt.Y)
			}
			if pt.OutX == 0 && pt.OutY == 0 {
				t.Fatalf("subpath %d point %d: Out handle uninitialized (0,0), anchor (%v,%v)", si, pi, pt.X, pt.Y)
			}
		}
	}
}

func TestBuildTextOutlinePath_ContourCounts(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"O", 2}, // outer ring + counter
		{"i", 2}, // stem + dot
		{"A", 2}, // outer + counter
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			h, layerID := setupOutlineTextLayer(t, tc.text, 32, 50, 20)
			defer Free(h)
			tl := storedTextLayer(t, h, layerID)

			p := buildTextOutlinePath(tl)
			if p == nil {
				t.Fatalf("buildTextOutlinePath(%q) = nil", tc.text)
			}
			if len(p.Subpaths) != tc.want {
				t.Errorf("%q: %d subpaths, want %d", tc.text, len(p.Subpaths), tc.want)
			}
		})
	}
}

func TestConvertTextToPath_FillOnlyVectorLayer(t *testing.T) {
	h, layerID := setupOutlineTextLayer(t, "A", 32, 50, 20)
	defer Free(h)
	tl := storedTextLayer(t, h, layerID)
	textColor := tl.Color

	vl := convertToPath(t, h, layerID)
	if vl.FillColor != textColor {
		t.Errorf("FillColor = %v, want text color %v", vl.FillColor, textColor)
	}
	if vl.StrokeWidth != 0 {
		t.Errorf("StrokeWidth = %v, want 0 (fill-only)", vl.StrokeWidth)
	}
	if vl.StrokeColor[3] != 0 {
		t.Errorf("StrokeColor = %v, want disabled (alpha 0)", vl.StrokeColor)
	}
	// The outline path is in document coordinates, so the layer keeps
	// document-origin bounds (bounds-local CachedRaster contract, see S.2).
	if vl.Bounds.X != 0 || vl.Bounds.Y != 0 || vl.Bounds.W != 200 || vl.Bounds.H != 100 {
		t.Errorf("Bounds = %+v, want document bounds {0 0 200 100}", vl.Bounds)
	}
	for si, sp := range vl.Shape.Subpaths {
		if !sp.Closed {
			t.Errorf("subpath %d: Closed = false, want true", si)
		}
	}
}

func TestConvertTextToPath_RasterMatchesTextRaster(t *testing.T) {
	const docW, docH = 200, 100
	h, layerID := setupOutlineTextLayer(t, "AGO", 32, 20, 20)
	defer Free(h)
	tl := storedTextLayer(t, h, layerID)
	if len(tl.CachedRaster) == 0 {
		t.Fatal("text layer has no cached raster")
	}
	textRaster := append([]byte(nil), tl.CachedRaster...)
	textMask := docInkMask(textRaster, tl.Bounds, docW, docH)

	vl := convertToPath(t, h, layerID)
	if len(vl.CachedRaster) != docW*docH*4 {
		t.Fatalf("vector raster length = %d, want %d", len(vl.CachedRaster), docW*docH*4)
	}
	outlineMask := docInkMask(vl.CachedRaster, vl.Bounds, docW, docH)

	if iou := maskIoU(textMask, outlineMask); iou <= 0.85 {
		t.Errorf("ink IoU between text raster and outline raster = %.3f, want > 0.85", iou)
	}
}

func TestConvertTextToPath_CenterAlignedMatchesRasterPosition(t *testing.T) {
	const docW, docH = 200, 100
	h, layerID := setupOutlineTextLayer(t, "Center", 24, 100, 20)
	defer Free(h)

	center := "center"
	if _, err := DispatchCommand(h, commandSetTextStyle, mustJSON(t, SetTextStylePayload{
		LayerID:   layerID,
		Alignment: &center,
	})); err != nil {
		t.Fatalf("SetTextStyle: %v", err)
	}

	tl := storedTextLayer(t, h, layerID)
	textRaster := append([]byte(nil), tl.CachedRaster...)
	textMask := docInkMask(textRaster, tl.Bounds, docW, docH)
	tx0, ty0, tx1, ty1, ok := maskBBox(textMask, docW, docH)
	if !ok {
		t.Fatal("text raster has no ink")
	}

	vl := convertToPath(t, h, layerID)
	outlineMask := docInkMask(vl.CachedRaster, vl.Bounds, docW, docH)
	ox0, oy0, ox1, oy1, ok := maskBBox(outlineMask, docW, docH)
	if !ok {
		t.Fatal("outline raster has no ink")
	}

	const tol = 3
	if abs(tx0-ox0) > tol || abs(ty0-oy0) > tol || abs(tx1-ox1) > tol || abs(ty1-oy1) > tol {
		t.Errorf("center-aligned ink bbox mismatch: text (%d,%d)-(%d,%d), outline (%d,%d)-(%d,%d)",
			tx0, ty0, tx1, ty1, ox0, oy0, ox1, oy1)
	}
}

func TestConvertTextToPath_UndoRestoresTextLayer(t *testing.T) {
	h, layerID := setupOutlineTextLayer(t, "Undo me", 24, 20, 20)
	defer Free(h)

	convertToPath(t, h, layerID)
	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	tl := storedTextLayer(t, h, layerID)
	if tl.Text != "Undo me" {
		t.Errorf("text after undo = %q, want %q", tl.Text, "Undo me")
	}
}
