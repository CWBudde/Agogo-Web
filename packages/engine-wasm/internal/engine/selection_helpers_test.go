package engine

import "testing"

func TestSelectionHelpersCloneNormalizeAndMeta(t *testing.T) {
	selection := &Selection{Width: 4, Height: 3, Mask: make([]byte, 12)}
	selection.Mask[1] = 255
	selection.Mask[6] = 128
	selection.Mask[10] = 255

	cloned := cloneSelection(selection)
	if !selectionEqual(selection, cloned) {
		t.Fatal("cloneSelection should produce an equal selection")
	}
	cloned.Mask[1] = 0
	if selection.Mask[1] != 255 {
		t.Fatal("cloneSelection should deep-copy the mask")
	}
	if selectionEqual(selection, cloned) {
		t.Fatal("modified clone should no longer compare equal")
	}

	bounds, ok := selection.bounds()
	if !ok {
		t.Fatal("selection bounds should be available")
	}
	if bounds != (DirtyRect{X: 1, Y: 0, W: 2, H: 3}) {
		t.Fatalf("bounds = %+v, want {X:1 Y:0 W:2 H:3}", bounds)
	}
	if got := selection.pixelCount(); got != 3 {
		t.Fatalf("pixelCount = %d, want 3", got)
	}

	if normalized := normalizeSelection(&Selection{Width: 2, Height: 2, Mask: []byte{0, 0, 0, 0}}); normalized != nil {
		t.Fatal("all-zero selection should normalize to nil")
	}
	if normalized := normalizeSelection(&Selection{Width: 2, Height: 2, Mask: []byte{255}}); normalized != nil {
		t.Fatal("short mask should normalize to nil")
	}
	normalized := normalizeSelection(&Selection{Width: 2, Height: 2, Mask: []byte{0, 255, 0, 0, 9}})
	if normalized == nil || len(normalized.Mask) != 4 {
		t.Fatalf("normalized selection = %+v, want 4-byte mask", normalized)
	}

	mask := newLayerMaskFromSelection(selection)
	if mask == nil || !mask.Enabled || mask.Width != 4 || mask.Height != 3 {
		t.Fatalf("layer mask = %+v, want enabled 4x3 mask", mask)
	}
	mask.Data[1] = 0
	if selection.Mask[1] != 255 {
		t.Fatal("newLayerMaskFromSelection should deep-copy the selection data")
	}

	doc := &Document{
		Selection:     selection,
		LastSelection: &Selection{Width: 1, Height: 1, Mask: []byte{255}},
	}
	meta := doc.selectionMeta()
	if !meta.Active || !meta.LastSelectionAvailable {
		t.Fatalf("selection meta = %+v, want active selection with last selection available", meta)
	}
	if meta.PixelCount != 3 {
		t.Fatalf("meta pixelCount = %d, want 3", meta.PixelCount)
	}
	if meta.Bounds == nil || *meta.Bounds != bounds {
		t.Fatalf("meta bounds = %+v, want %+v", meta.Bounds, bounds)
	}
}

func TestSelectionCommandsCoverMissingBranches(t *testing.T) {
	doc := &Document{Width: 5, Height: 5}

	if err := doc.InvertSelection(); err != nil {
		t.Fatalf("InvertSelection without active selection: %v", err)
	}
	if doc.Selection == nil || doc.Selection.pixelCount() != 25 {
		t.Fatalf("SelectAll via invert fallback produced %v pixels, want 25", doc.Selection.pixelCount())
	}

	if err := doc.InvertSelection(); err != nil {
		t.Fatalf("InvertSelection with active selection: %v", err)
	}
	if doc.Selection != nil {
		t.Fatal("inverting a full selection should clear it")
	}

	doc.Selection = newRectSelection(5, 5, LayerBounds{X: 1, Y: 1, W: 3, H: 3})
	if err := doc.ContractSelection(1); err != nil {
		t.Fatalf("ContractSelection: %v", err)
	}
	if got := doc.Selection.pixelCount(); got != 1 {
		t.Fatalf("contracted pixelCount = %d, want 1", got)
	}

	doc.Selection = newRectSelection(5, 5, LayerBounds{X: 1, Y: 1, W: 3, H: 3})
	if err := doc.SmoothSelection(1); err != nil {
		t.Fatalf("SmoothSelection: %v", err)
	}
	if doc.Selection == nil {
		t.Fatal("smoothed selection should remain active")
	}
	if doc.Selection.Mask[0] == 0 {
		t.Fatal("smoothed selection should soften the outer corner")
	}
	if doc.Selection.Mask[2*5+2] != 255 {
		t.Fatalf("smoothed center = %d, want 255", doc.Selection.Mask[2*5+2])
	}

	doc.Selection = newRectSelection(5, 5, LayerBounds{X: 1, Y: 1, W: 3, H: 3})
	if err := doc.BorderSelection(3); err != nil {
		t.Fatalf("BorderSelection: %v", err)
	}
	if doc.Selection == nil || doc.Selection.pixelCount() == 0 {
		t.Fatal("border selection should remain non-empty")
	}
	if doc.Selection.Mask[2*5+2] != 0 {
		t.Fatalf("border center = %d, want 0", doc.Selection.Mask[2*5+2])
	}
}

func TestSelectionShapeHelpersAndCoverageSampling(t *testing.T) {
	ellipse := newEllipseSelection(7, 7, LayerBounds{X: 1, Y: 1, W: 5, H: 5}, true)
	if ellipse == nil || ellipse.pixelCount() == 0 {
		t.Fatal("ellipse selection should contain pixels")
	}
	if ellipse.Mask[3*7+3] != 255 {
		t.Fatalf("ellipse center = %d, want 255", ellipse.Mask[3*7+3])
	}
	if ellipse.Mask[0] != 0 {
		t.Fatalf("ellipse outside pixel = %d, want 0", ellipse.Mask[0])
	}
	if ellipse.Mask[1*7+1] == 0 || ellipse.Mask[1*7+1] == 255 {
		t.Fatalf("ellipse boundary pixel = %d, want partial coverage", ellipse.Mask[1*7+1])
	}

	triangle := []SelectionPoint{{X: 1, Y: 1}, {X: 4, Y: 1}, {X: 1, Y: 4}}
	polygon := newPolygonSelection(6, 6, triangle, false)
	if polygon.Mask[2*6+2] != 255 {
		t.Fatalf("polygon interior = %d, want 255", polygon.Mask[2*6+2])
	}
	if polygon.Mask[4*6+4] != 0 {
		t.Fatalf("polygon exterior = %d, want 0", polygon.Mask[4*6+4])
	}
	if !pointInPolygon(triangle, 2, 2) {
		t.Fatal("point should be inside triangle")
	}
	if pointInPolygon(triangle, 4.5, 4.5) {
		t.Fatal("point should be outside triangle")
	}

	hard := sampledCoverage(false, func(sampleX, sampleY float64) bool {
		return false
	}, 0, 0)
	if hard != 0 {
		t.Fatalf("hard sampled coverage = %d, want 0", hard)
	}
	soft := sampledCoverage(true, func(sampleX, sampleY float64) bool {
		return sampleX < 0.5
	}, 0, 0)
	if soft == 0 || soft == 255 {
		t.Fatalf("soft sampled coverage = %d, want partial coverage", soft)
	}

	if _, ok := sampleSurfaceColor(make([]byte, 4*4), 2, 2, 3, 0); ok {
		t.Fatal("sampleSurfaceColor should reject out-of-bounds coordinates")
	}
	if _, ok := sampleSurfaceColor([]byte{1, 2, 3}, 1, 1, 0, 0); ok {
		t.Fatal("sampleSurfaceColor should reject undersized surfaces")
	}
}
