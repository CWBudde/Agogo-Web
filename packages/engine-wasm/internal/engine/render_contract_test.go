package engine

import (
	"encoding/base64"
	"strings"
	"testing"
)

// newTextContractDoc builds a transparent 64×48 document containing a single
// text layer with the given bounds, rasterized through the production
// bounds-local raster path.
func newTextContractDoc(t *testing.T, bounds LayerBounds) *Document {
	t.Helper()
	doc := &Document{
		Width:      64,
		Height:     48,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "TextContract",
		LayerRoot:  NewGroupLayer("Root"),
	}
	tl := NewTextLayer("T", bounds, "H", nil)
	tl.FontSize = 16
	tl.Color = [4]uint8{255, 0, 0, 255}
	if err := rasterizeTextLayer(tl); err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
	// Point text computes tight bounds anchored at the legacy bounds origin;
	// the raster must satisfy the bounds-local contract against them.
	if got, want := len(tl.CachedRaster), tl.Bounds.W*tl.Bounds.H*4; got != want {
		t.Fatalf("raster length = %d, want bounds-local %d (%dx%d)", got, want, tl.Bounds.W, tl.Bounds.H)
	}
	doc.LayerRoot.SetChildren([]LayerNode{tl})
	return doc
}

// inkBBox returns the bounding box and pixel count of all non-transparent
// pixels in a w×h RGBA surface. ok is false when the surface has no ink.
func inkBBox(surface []byte, w, h int) (minX, minY, maxX, maxY, count int, ok bool) {
	minX, minY = w, h
	maxX, maxY = -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if surface[(y*w+x)*4+3] == 0 {
				continue
			}
			count++
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	return minX, minY, maxX, maxY, count, maxX >= 0
}

// TestTextLayerCompositesAtBoundsPositionWithoutDoubleOffset is the regression
// test for the CachedRaster geometry contract: rasters are bounds-local, so a
// text layer whose bounds sit at (20,12) must composite its glyphs exactly
// (20,12) document pixels away from the same layer at the origin — not (40,24)
// as the old doc-space raster + bounds offset double-application produced.
func TestTextLayerCompositesAtBoundsPositionWithoutDoubleOffset(t *testing.T) {
	const dx, dy = 20, 12
	origin := newTextContractDoc(t, LayerBounds{X: 0, Y: 0, W: 64, H: 48})
	moved := newTextContractDoc(t, LayerBounds{X: dx, Y: dy, W: 64, H: 48})

	surfOrigin, err := origin.renderCompositeSurfaceChecked()
	if err != nil {
		t.Fatalf("renderCompositeSurfaceChecked (origin): %v", err)
	}
	surfMoved, err := moved.renderCompositeSurfaceChecked()
	if err != nil {
		t.Fatalf("renderCompositeSurfaceChecked (moved): %v", err)
	}

	oMinX, oMinY, oMaxX, oMaxY, oCount, ok := inkBBox(surfOrigin, 64, 48)
	if !ok {
		t.Fatal("origin document composited no text ink")
	}
	mMinX, mMinY, mMaxX, mMaxY, mCount, ok := inkBBox(surfMoved, 64, 48)
	if !ok {
		t.Fatal("moved document composited no text ink")
	}

	if mMinX != oMinX+dx || mMinY != oMinY+dy || mMaxX != oMaxX+dx || mMaxY != oMaxY+dy {
		t.Fatalf("moved ink bbox = (%d,%d)-(%d,%d), want origin bbox (%d,%d)-(%d,%d) shifted by (%d,%d)",
			mMinX, mMinY, mMaxX, mMaxY, oMinX, oMinY, oMaxX, oMaxY, dx, dy)
	}
	if mCount != oCount {
		t.Fatalf("moved ink pixel count = %d, want %d (same glyphs, only translated)", mCount, oCount)
	}

	// Pixel-for-pixel: the moved composite must equal the origin composite
	// translated by (dx,dy).
	for y := oMinY; y <= oMaxY; y++ {
		for x := oMinX; x <= oMaxX; x++ {
			src := (y*64 + x) * 4
			dst := ((y+dy)*64 + (x + dx)) * 4
			for c := 0; c < 4; c++ {
				if surfOrigin[src+c] != surfMoved[dst+c] {
					t.Fatalf("pixel mismatch at origin (%d,%d) vs moved (%d,%d): channel %d = %d, want %d",
						x, y, x+dx, y+dy, c, surfMoved[dst+c], surfOrigin[src+c])
				}
			}
		}
	}
}

// TestRenderSurfacesCompositeErrorAndDoesNotCacheBlankFrame verifies that a
// CachedRaster whose length violates the bounds-local contract produces a
// propagated error on RenderResult instead of a silently cached blank frame,
// and that rendering recovers once the raster is fixed — without requiring a
// ContentVersion bump, proving the failed composite was not stored in the
// content-version cache.
func TestRenderSurfacesCompositeErrorAndDoesNotCacheBlankFrame(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Composite Error",
		Width:      16,
		Height:     16,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "transparent",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "T",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 16, H: 16},
		Text:      "H",
		FontSize:  12,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add text layer: %v", err)
	}
	if addResult.Error != "" {
		t.Fatalf("add-layer render reported error %q, want none", addResult.Error)
	}

	inst, ok := instances[h]
	if !ok {
		t.Fatalf("no instance for handle %d", h)
	}
	doc := inst.manager.activeMut()
	tl, ok := doc.findLayer(addResult.UIMeta.ActiveLayerID).(*TextLayer)
	if !ok {
		t.Fatal("active layer is not a text layer")
	}
	goodRaster := append([]byte(nil), tl.CachedRaster...)
	if want := tl.Bounds.W * tl.Bounds.H * 4; len(goodRaster) != want {
		t.Fatalf("expected bounds-local raster of %d bytes (%dx%d), got %d", want, tl.Bounds.W, tl.Bounds.H, len(goodRaster))
	}

	// Corrupt the raster (length no longer matches bounds) and invalidate the
	// composite cache the way any real mutation would.
	tl.CachedRaster = []byte{1, 2, 3, 4}
	doc.ContentVersion++

	broken, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("RenderFrame (broken): %v", err)
	}
	if broken.Error == "" {
		t.Fatal("RenderResult.Error is empty, want propagated composite error")
	}
	if !strings.Contains(broken.Error, "raster length") {
		t.Fatalf("RenderResult.Error = %q, want it to mention the raster length mismatch", broken.Error)
	}
	if len(inst.cachedDocSurface) != 0 {
		t.Fatalf("failed composite was cached (cachedDocSurface len=%d), want empty", len(inst.cachedDocSurface))
	}

	// Fix the raster WITHOUT bumping ContentVersion: recovery must not be
	// blocked by a cached blank frame keyed to the same version.
	tl.CachedRaster = goodRaster

	fixed, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("RenderFrame (fixed): %v", err)
	}
	if fixed.Error != "" {
		t.Fatalf("RenderResult.Error after fix = %q, want empty", fixed.Error)
	}
	if len(inst.cachedDocSurface) == 0 {
		t.Fatal("recovered composite surface was not rebuilt")
	}
	hasInk := false
	for i := 3; i < len(inst.cachedDocSurface); i += 4 {
		if inst.cachedDocSurface[i] != 0 {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Fatal("recovered composite surface is blank, want text ink")
	}
}

// TestImportProjectCorruptPSDSurfacesUnderlyingError verifies that a payload
// carrying the PSD signature but truncated content fails with the real PSD
// parse error instead of the generic "unsupported import payload".
func TestImportProjectCorruptPSDSurfacesUnderlyingError(t *testing.T) {
	h := Init("")
	defer Free(h)

	payload := base64.StdEncoding.EncodeToString([]byte("8BPS\x00\x01"))
	if _, err := ImportProject(h, payload); err == nil {
		t.Fatal("ImportProject succeeded on a truncated PSD, want error")
	} else {
		msg := err.Error()
		if strings.Contains(msg, "unsupported import payload") {
			t.Fatalf("ImportProject error = %q, want the underlying PSD failure, not the generic fallback", msg)
		}
		if !strings.Contains(msg, "load PSD") {
			t.Fatalf("ImportProject error = %q, want it wrapped with \"load PSD\" context", msg)
		}
	}
}
