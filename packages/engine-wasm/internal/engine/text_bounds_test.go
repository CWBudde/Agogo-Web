package engine

import (
	"bytes"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/text"
)

// newPointTextLayer builds a point-text layer anchored at (ax, ay) with the
// given text, ready for rasterizeTextLayer.
func newPointTextLayer(t *testing.T, content string, ax, ay, fontSize float64) *TextLayer {
	t.Helper()
	tl := NewTextLayer("T", LayerBounds{}, content, nil)
	tl.AnchorX = ax
	tl.AnchorY = ay
	tl.AnchorSet = true
	tl.FontSize = fontSize
	tl.Color = [4]uint8{0, 0, 0, 255}
	tl.TextType = "point"
	return tl
}

func mustRasterize(t *testing.T, tl *TextLayer) {
	t.Helper()
	if err := rasterizeTextLayer(tl); err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
}

// rasterInkBBox returns the ink bounding box of a layer's bounds-local raster.
func rasterInkBBox(t *testing.T, tl *TextLayer) (minX, minY, maxX, maxY int) {
	t.Helper()
	minX, minY, maxX, maxY, _, ok := inkBBox(tl.CachedRaster, tl.Bounds.W, tl.Bounds.H)
	if !ok {
		t.Fatal("raster has no ink")
	}
	return minX, minY, maxX, maxY
}

// assertNoEdgeInk fails when any pixel on the outermost rows/columns of the
// bounds-local raster is non-transparent — i.e. the tight bounds clipped ink.
func assertNoEdgeInk(t *testing.T, tl *TextLayer) {
	t.Helper()
	w, h := tl.Bounds.W, tl.Bounds.H
	buf := tl.CachedRaster
	for x := range w {
		if buf[x*4+3] != 0 || buf[((h-1)*w+x)*4+3] != 0 {
			t.Fatalf("ink touches top/bottom raster edge at x=%d — bounds clip the text", x)
		}
	}
	for y := range h {
		if buf[(y*w)*4+3] != 0 || buf[(y*w+w-1)*4+3] != 0 {
			t.Fatalf("ink touches left/right raster edge at y=%d — bounds clip the text", y)
		}
	}
}

// TestPointText_CenterAligned_TightBoundsAbsorbNegativeX is the S.2 clipping
// regression: center-aligned point text must extend LEFT of its anchor and
// none of its ink may be clipped by the layer bounds.
func TestPointText_CenterAligned_TightBoundsAbsorbNegativeX(t *testing.T) {
	tl := newPointTextLayer(t, "Hello", 30, 40, 24)
	tl.Alignment = "center"
	mustRasterize(t, tl)

	if got, want := len(tl.CachedRaster), tl.Bounds.W*tl.Bounds.H*4; got != want {
		t.Fatalf("raster length = %d, want bounds-local %d", got, want)
	}
	if tl.Bounds.X >= 30 {
		t.Errorf("bounds.X = %d, want < anchor 30 (centered text extends left of the anchor)", tl.Bounds.X)
	}
	minX, _, _, _ := rasterInkBBox(t, tl)
	if docInkLeft := tl.Bounds.X + minX; docInkLeft >= 30 {
		t.Errorf("leftmost ink at doc x=%d, want < anchor 30", docInkLeft)
	}
	assertNoEdgeInk(t, tl)
}

// TestPointText_RightAligned_EndsAtAnchor: right-aligned text ends at the
// anchor; all ink lies left of it and nothing is clipped.
func TestPointText_RightAligned_EndsAtAnchor(t *testing.T) {
	tl := newPointTextLayer(t, "Hello", 120, 40, 24)
	tl.Alignment = "right"
	mustRasterize(t, tl)

	_, _, maxX, _ := rasterInkBBox(t, tl)
	docInkRight := tl.Bounds.X + maxX
	if docInkRight > 120+1 {
		t.Errorf("rightmost ink at doc x=%d, want <= anchor 120 (+1 rounding)", docInkRight)
	}
	if tl.Bounds.X >= 120-10 {
		t.Errorf("bounds.X = %d, want well left of anchor 120", tl.Bounds.X)
	}
	assertNoEdgeInk(t, tl)
}

// TestTextRaster_BoundsLocalContract pins len(CachedRaster) == W*H*4 for both
// text types.
func TestTextRaster_BoundsLocalContract(t *testing.T) {
	point := newPointTextLayer(t, "Contract", 50, 30, 18)
	mustRasterize(t, point)
	if got, want := len(point.CachedRaster), point.Bounds.W*point.Bounds.H*4; got != want {
		t.Errorf("point raster length = %d, want %d", got, want)
	}

	area := NewTextLayer("A", LayerBounds{X: 10, Y: 10, W: 120, H: 80}, "wrap me around please", nil)
	area.TextType = "area"
	area.FontSize = 16
	area.Color = [4]uint8{0, 0, 0, 255}
	mustRasterize(t, area)
	if area.Bounds != (LayerBounds{X: 10, Y: 10, W: 120, H: 80}) {
		t.Errorf("area bounds mutated to %+v, want user frame kept", area.Bounds)
	}
	if got, want := len(area.CachedRaster), 120*80*4; got != want {
		t.Errorf("area raster length = %d, want %d", got, want)
	}
	if area.AnchorX != 10 || area.AnchorY != 10 {
		t.Errorf("area anchor = (%v,%v), want synced to frame origin (10,10)", area.AnchorX, area.AnchorY)
	}
}

// TestTranslateTextLayer_RerasterizeIsIdentical: moving a text layer shifts
// bounds AND anchor; re-rasterizing afterwards must reproduce the identical
// raster at the shifted position (no snap-back, no re-layout drift).
func TestTranslateTextLayer_RerasterizeIsIdentical(t *testing.T) {
	tl := newPointTextLayer(t, "Move me", 50, 40, 20)
	mustRasterize(t, tl)
	boundsBefore := tl.Bounds
	rasterBefore := append([]byte(nil), tl.CachedRaster...)

	if err := translateLayerNode(tl, 7, 9); err != nil {
		t.Fatalf("translateLayerNode: %v", err)
	}
	if tl.AnchorX != 57 || tl.AnchorY != 49 {
		t.Fatalf("anchor after translate = (%v,%v), want (57,49)", tl.AnchorX, tl.AnchorY)
	}
	mustRasterize(t, tl)

	wantBounds := boundsBefore
	wantBounds.X += 7
	wantBounds.Y += 9
	if tl.Bounds != wantBounds {
		t.Errorf("bounds after translate+rerasterize = %+v, want %+v", tl.Bounds, wantBounds)
	}
	if !bytes.Equal(tl.CachedRaster, rasterBefore) {
		t.Error("raster changed after translate + re-rasterize, want byte-identical")
	}
}

// TestBoldChangesRaster_UnknownFamilyFallsBack: Bold renders differently;
// an unknown family renders identically to the DejaVu Sans fallback.
func TestBoldChangesRaster_UnknownFamilyFallsBack(t *testing.T) {
	regular := newPointTextLayer(t, "Weight", 40, 30, 24)
	regular.FontFamily = "DejaVu Sans"
	mustRasterize(t, regular)

	bold := newPointTextLayer(t, "Weight", 40, 30, 24)
	bold.FontFamily = "DejaVu Sans"
	bold.Bold = true
	mustRasterize(t, bold)

	if bold.Bounds == regular.Bounds && bytes.Equal(bold.CachedRaster, regular.CachedRaster) {
		t.Error("bold raster identical to regular, want a different rendering")
	}

	unknown := newPointTextLayer(t, "Weight", 40, 30, 24)
	unknown.FontFamily = "unknown-font"
	mustRasterize(t, unknown)
	if unknown.Bounds != regular.Bounds || !bytes.Equal(unknown.CachedRaster, regular.CachedRaster) {
		t.Error("unknown font family did not fall back to DejaVu Sans rendering")
	}
}

// TestSmallCapsInkShorterThanAllCaps: small caps render lowercase letters as
// shrunken uppercase glyphs, so their ink is shorter than true all caps.
func TestSmallCapsInkShorterThanAllCaps(t *testing.T) {
	small := newPointTextLayer(t, "abc", 30, 30, 32)
	small.SmallCaps = true
	mustRasterize(t, small)
	_, sMinY, _, sMaxY := rasterInkBBox(t, small)

	caps := newPointTextLayer(t, "abc", 30, 30, 32)
	caps.AllCaps = true
	mustRasterize(t, caps)
	_, cMinY, _, cMaxY := rasterInkBBox(t, caps)

	if smallH, capsH := sMaxY-sMinY, cMaxY-cMinY; smallH >= capsH {
		t.Errorf("small-caps ink height = %d, want < all-caps ink height %d", smallH, capsH)
	}
}

// TestUnderline_BarAtPostTablePosition: underline ink appears below the
// baseline at the font's post-table position.
func TestUnderline_BarAtPostTablePosition(t *testing.T) {
	plain := newPointTextLayer(t, "Under", 20, 10, 24)
	mustRasterize(t, plain)
	_, _, _, plainMaxY := rasterInkBBox(t, plain)
	plainDocMaxY := plain.Bounds.Y + plainMaxY

	under := newPointTextLayer(t, "Under", 20, 10, 24)
	under.Underline = true
	mustRasterize(t, under)

	face := text.DefaultRegistry().Resolve("DejaVu Sans", false, false)
	m := face.Metrics(24)
	baselineDoc := under.AnchorY + m.Ascent
	barTopDoc := baselineDoc + m.UnderlinePosition

	// A row inside the bar must carry ink.
	rowLocal := int(barTopDoc+m.UnderlineThickness/2) - under.Bounds.Y
	if rowLocal < 0 || rowLocal >= under.Bounds.H {
		t.Fatalf("expected underline row %d inside raster height %d", rowLocal, under.Bounds.H)
	}
	inkOnRow := 0
	for x := range under.Bounds.W {
		if under.CachedRaster[(rowLocal*under.Bounds.W+x)*4+3] > 0 {
			inkOnRow++
		}
	}
	if inkOnRow < under.Bounds.W/3 {
		t.Errorf("underline row %d has %d ink pixels, want a bar spanning most of %d", rowLocal, inkOnRow, under.Bounds.W)
	}

	// The underline layer's ink must extend below the plain layer's ink.
	_, _, _, underMaxY := rasterInkBBox(t, under)
	if underDocMaxY := under.Bounds.Y + underMaxY; underDocMaxY <= plainDocMaxY {
		t.Errorf("underline ink bottom %d not below plain text bottom %d", underDocMaxY, plainDocMaxY)
	}
}

// TestKerningWidensLayout: manual kerning (1/1000 em per pair) widens the
// laid-out text and therefore the tight bounds.
func TestKerningWidensLayout(t *testing.T) {
	base := newPointTextLayer(t, "AVAVAV", 20, 20, 24)
	mustRasterize(t, base)

	kerned := newPointTextLayer(t, "AVAVAV", 20, 20, 24)
	kerned.Kerning = 200
	mustRasterize(t, kerned)

	if kerned.Bounds.W <= base.Bounds.W {
		t.Errorf("kerned bounds width = %d, want > %d", kerned.Bounds.W, base.Bounds.W)
	}
}

// TestBaselineShiftMovesPointTextBounds: a positive baseline shift raises the
// text; the tight bounds move up by the shift while the raster stays
// byte-identical (the bounds absorb the offset).
func TestBaselineShiftMovesPointTextBounds(t *testing.T) {
	base := newPointTextLayer(t, "Shift", 30, 40, 24)
	mustRasterize(t, base)

	shifted := newPointTextLayer(t, "Shift", 30, 40, 24)
	shifted.BaselineShift = 5
	mustRasterize(t, shifted)

	if shifted.Bounds.Y != base.Bounds.Y-5 {
		t.Errorf("shifted bounds.Y = %d, want %d (raised by 5)", shifted.Bounds.Y, base.Bounds.Y-5)
	}
	if shifted.Bounds.X != base.Bounds.X || shifted.Bounds.W != base.Bounds.W || shifted.Bounds.H != base.Bounds.H {
		t.Errorf("shifted bounds = %+v, want same X/W/H as %+v", shifted.Bounds, base.Bounds)
	}
	if !bytes.Equal(shifted.CachedRaster, base.CachedRaster) {
		t.Error("baseline-shifted raster differs, want identical (bounds absorb the shift)")
	}
}

// TestEmptyPointText_NonDegenerateBoundsAtAnchor: an empty text layer keeps a
// minimal hittable box at the anchor with a transparent bounds-local raster.
func TestEmptyPointText_NonDegenerateBoundsAtAnchor(t *testing.T) {
	tl := newPointTextLayer(t, "", 30, 20, 24)
	mustRasterize(t, tl)

	if tl.Bounds.W <= 0 || tl.Bounds.H <= 0 {
		t.Fatalf("empty text bounds = %+v, want non-degenerate", tl.Bounds)
	}
	if tl.Bounds.X > 30 || tl.Bounds.X+tl.Bounds.W < 30 || tl.Bounds.Y > 20 || tl.Bounds.Y+tl.Bounds.H < 20 {
		t.Errorf("bounds %+v do not contain the anchor (30,20)", tl.Bounds)
	}
	if got, want := len(tl.CachedRaster), tl.Bounds.W*tl.Bounds.H*4; got != want {
		t.Fatalf("raster length = %d, want %d", got, want)
	}
	for i := 3; i < len(tl.CachedRaster); i += 4 {
		if tl.CachedRaster[i] != 0 {
			t.Fatal("empty text raster has ink, want fully transparent")
		}
	}
}

// TestLegacyAnchorMigration_DerivedOnceFromBounds: a layer without AnchorSet
// (legacy archive shape) derives its anchor from the bounds origin exactly
// once; repeated rasterization must NOT re-derive it from the tighter bounds
// (the historical drift bug this flag exists for).
func TestLegacyAnchorMigration_DerivedOnceFromBounds(t *testing.T) {
	tl := NewTextLayer("Legacy", LayerBounds{X: 25, Y: 15, W: 100, H: 50}, "Old", nil)
	tl.FontSize = 20
	tl.Color = [4]uint8{0, 0, 0, 255}
	mustRasterize(t, tl)

	if tl.AnchorX != 25 || tl.AnchorY != 15 || !tl.AnchorSet {
		t.Fatalf("migrated anchor = (%v,%v,set=%v), want (25,15,true)", tl.AnchorX, tl.AnchorY, tl.AnchorSet)
	}
	bounds := tl.Bounds
	raster := append([]byte(nil), tl.CachedRaster...)

	// Rasterizing again must be a fixed point: same anchor, bounds, raster.
	mustRasterize(t, tl)
	if tl.AnchorX != 25 || tl.AnchorY != 15 {
		t.Errorf("anchor drifted to (%v,%v) on re-rasterize, want stable (25,15)", tl.AnchorX, tl.AnchorY)
	}
	if tl.Bounds != bounds {
		t.Errorf("bounds drifted to %+v on re-rasterize, want stable %+v", tl.Bounds, bounds)
	}
	if !bytes.Equal(tl.CachedRaster, raster) {
		t.Error("raster changed on re-rasterize, want byte-identical")
	}
}

// TestAnchorAtOrigin_NoMigrationDrift pins the corner case that motivated
// AnchorSet: a layer legitimately anchored at (0,0) has tight bounds at
// negative coordinates; re-rasterizing must not re-derive the anchor from
// them.
func TestAnchorAtOrigin_NoMigrationDrift(t *testing.T) {
	tl := newPointTextLayer(t, "Origin", 0, 0, 24)
	mustRasterize(t, tl)
	first := tl.Bounds
	if tl.Bounds.X >= 0 && tl.Bounds.Y >= 0 {
		t.Logf("note: tight bounds %+v unexpectedly non-negative", tl.Bounds)
	}
	mustRasterize(t, tl)
	if tl.AnchorX != 0 || tl.AnchorY != 0 {
		t.Fatalf("anchor drifted to (%v,%v), want (0,0)", tl.AnchorX, tl.AnchorY)
	}
	if tl.Bounds != first {
		t.Fatalf("bounds drifted from %+v to %+v across rasterizations", first, tl.Bounds)
	}
}

// TestUndoTextEdit_RestoresBoundsAndRaster drives the dispatch/history path:
// undoing a committed text edit must restore the previous tight bounds AND
// the previous raster bytes.
func TestUndoTextEdit_RestoresBoundsAndRaster(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	addResult, err := DispatchCommand(h, commandAddTextLayer, mustJSON(t, AddTextLayerPayload{
		X: 60, Y: 40, FontSize: 24, Color: [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddTextLayer: %v", err)
	}
	layerID := addResult.UIMeta.ActiveLayerID
	if _, err := DispatchCommand(h, commandTextEditInput, mustJSON(t, TextEditInputPayload{Text: "Hi"})); err != nil {
		t.Fatalf("TextEditInput: %v", err)
	}
	if _, err := DispatchCommand(h, commandCommitTextEdit, "{}"); err != nil {
		t.Fatalf("CommitTextEdit: %v", err)
	}

	inst := instances[h]
	tl := inst.manager.activeMut().findLayer(layerID).(*TextLayer)
	boundsHi := tl.Bounds
	rasterHi := append([]byte(nil), tl.CachedRaster...)

	// Second edit widens the layer.
	if _, err := DispatchCommand(h, commandEnterTextEditMode, mustJSON(t, EnterTextEditModePayload{LayerID: layerID})); err != nil {
		t.Fatalf("EnterTextEditMode: %v", err)
	}
	if _, err := DispatchCommand(h, commandTextEditInput, mustJSON(t, TextEditInputPayload{Text: "Hi there, wider"})); err != nil {
		t.Fatalf("TextEditInput: %v", err)
	}
	if _, err := DispatchCommand(h, commandCommitTextEdit, "{}"); err != nil {
		t.Fatalf("CommitTextEdit: %v", err)
	}
	tl = inst.manager.activeMut().findLayer(layerID).(*TextLayer)
	if tl.Bounds == boundsHi {
		t.Fatal("second edit did not change the tight bounds; test cannot discriminate")
	}

	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	tl = inst.manager.activeMut().findLayer(layerID).(*TextLayer)
	if tl.Text != "Hi" {
		t.Fatalf("text after undo = %q, want %q", tl.Text, "Hi")
	}
	if tl.Bounds != boundsHi {
		t.Errorf("bounds after undo = %+v, want %+v", tl.Bounds, boundsHi)
	}
	if !bytes.Equal(tl.CachedRaster, rasterHi) {
		t.Error("raster after undo differs from pre-edit raster")
	}
}

// TestPointText_MultiLine_StacksLines: explicit newlines in point text stack
// lines one lineHeight apart.
func TestPointText_MultiLine_StacksLines(t *testing.T) {
	one := newPointTextLayer(t, "Line", 30, 20, 20)
	mustRasterize(t, one)

	two := newPointTextLayer(t, "Line\nLine", 30, 20, 20)
	mustRasterize(t, two)

	if two.Bounds.H <= one.Bounds.H {
		t.Errorf("two-line bounds height = %d, want > single-line %d", two.Bounds.H, one.Bounds.H)
	}
}
