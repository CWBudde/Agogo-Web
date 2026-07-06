package engine

// Phase S.5 regression tests: flatten opacity convention, Merge Visible
// z-order, dissolve noise-seed coordinates, and pixelNoiseSeed distribution.

import (
	"bytes"
	"fmt"
	"testing"
)

// S.5.1: FlattenLayer must not double-apply opacity/fill-opacity. The layer
// surface is rendered WITH opacity baked in (same convention as MergeDown and
// MergeVisible), so the resulting pixel layer must composite at 100% — a 50%
// red layer over white must stay pink, not drop to 25%.
func TestFlattenLayerDoesNotDoubleApplyOpacity(t *testing.T) {
	cases := []struct {
		name  string
		setup func(layer *PixelLayer)
	}{
		{name: "opacity", setup: func(layer *PixelLayer) { layer.SetOpacity(0.5) }},
		{name: "fillOpacity", setup: func(layer *PixelLayer) { layer.SetFillOpacity(0.5) }},
		{name: "both", setup: func(layer *PixelLayer) {
			layer.SetOpacity(0.5)
			layer.SetFillOpacity(0.5)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := &Document{Width: 2, Height: 2, LayerRoot: NewGroupLayer("Root")}
			white := NewPixelLayer("White", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, bytes.Repeat([]byte{255, 255, 255, 255}, 4))
			red := NewPixelLayer("Red", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, bytes.Repeat([]byte{255, 0, 0, 255}, 4))
			tc.setup(red)
			doc.LayerRoot.SetChildren([]LayerNode{white, red})

			before, err := doc.renderCompositeSurfaceChecked()
			if err != nil {
				t.Fatalf("render before flatten: %v", err)
			}
			if err := doc.FlattenLayer(red.ID()); err != nil {
				t.Fatalf("flatten layer: %v", err)
			}
			after, err := doc.renderCompositeSurfaceChecked()
			if err != nil {
				t.Fatalf("render after flatten: %v", err)
			}
			if len(before) != len(after) {
				t.Fatalf("surface length changed: %d != %d", len(before), len(after))
			}
			for i := range before {
				diff := int(before[i]) - int(after[i])
				if diff < -1 || diff > 1 {
					t.Fatalf("composite changed at byte %d: before=%d after=%d (opacity applied twice?)", i, before[i], after[i])
				}
			}

			flattened, ok := doc.findLayer(doc.ActiveLayerID).(*PixelLayer)
			if !ok {
				t.Fatalf("flattened layer type = %T, want *PixelLayer", doc.findLayer(doc.ActiveLayerID))
			}
			if flattened.Opacity() != 1 || flattened.FillOpacity() != 1 {
				t.Fatalf("flattened layer opacity=%v fillOpacity=%v, want 1/1 (baked-in convention shared with MergeDown/MergeVisible)", flattened.Opacity(), flattened.FillOpacity())
			}
		})
	}
}

// S.5.2: Merge Visible must keep hidden layers at their original stack
// positions; the merged layer replaces the topmost visible layer's slot
// (Photoshop behaviour), instead of pushing hidden layers below the result.
func TestMergeVisiblePreservesHiddenLayerZOrder(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	newLayer := func(name string, visible bool) *PixelLayer {
		layer := NewPixelLayer(name, LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{10, 20, 30, 255})
		layer.SetVisible(visible)
		return layer
	}
	// Bottom → top: H0(hidden), V1, H2(hidden), V3, H4(hidden).
	h0 := newLayer("H0", false)
	v1 := newLayer("V1", true)
	h2 := newLayer("H2", false)
	v3 := newLayer("V3", true)
	h4 := newLayer("H4", false)
	doc.LayerRoot.SetChildren([]LayerNode{h0, v1, h2, v3, h4})

	if err := doc.MergeVisible(); err != nil {
		t.Fatalf("merge visible: %v", err)
	}

	children := doc.LayerRoot.Children()
	got := make([]string, 0, len(children))
	for _, child := range children {
		got = append(got, child.Name())
	}
	want := []string{"H0", "H2", "Merged Visible", "H4"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("layer order after merge visible = %v, want %v (merged replaces topmost visible slot, hidden layers keep positions)", got, want)
	}
	if children[2].ID() != doc.ActiveLayerID {
		t.Fatalf("active layer = %q, want merged layer %q", doc.ActiveLayerID, children[2].ID())
	}
	for _, index := range []int{0, 1, 3} {
		if children[index].Visible() {
			t.Fatalf("hidden layer %q became visible after merge visible", children[index].Name())
		}
	}
}

// S.5.4: The document-surface dissolve path must derive its per-pixel noise
// seed from document coordinates via pixelNoiseSeed(docX, docY) — the same
// convention as compositeRasterIntoDocument — not from the flat byte index.
func TestCompositeDocumentSurfaceDissolveSeedUsesDocCoordinates(t *testing.T) {
	const docW, docH = 8, 8
	makeSurfaces := func() (dest, src []byte) {
		dest = bytes.Repeat([]byte{0, 0, 0, 255}, docW*docH)
		src = bytes.Repeat([]byte{255, 255, 255, 255}, docW*docH)
		return dest, src
	}

	dest, src := makeSurfaces()
	compositeDocumentSurfaceClipped(dest, src, docW, BlendModeDissolve, 0.5, nil, nil)

	dissolved, kept := 0, 0
	for y := 0; y < docH; y++ {
		for x := 0; x < docW; x++ {
			offset := (y*docW + x) * 4
			want := byte(0)
			if dissolveEnabled(0.5, pixelNoiseSeed(x, y)) {
				want = 255
				dissolved++
			} else {
				kept++
			}
			if dest[offset] != want {
				t.Fatalf("pixel (%d,%d) = %d, want %d — dissolve seed must be pixelNoiseSeed(docX, docY)", x, y, dest[offset], want)
			}
		}
	}
	if dissolved == 0 || kept == 0 {
		t.Fatalf("degenerate dissolve at 50%%: dissolved=%d kept=%d, expected a mix", dissolved, kept)
	}

	// Clipped pass must be identical to the full pass inside the clip rect.
	clipDest, clipSrc := makeSurfaces()
	clip := &DirtyRect{X: 2, Y: 3, W: 4, H: 3}
	compositeDocumentSurfaceClipped(clipDest, clipSrc, docW, BlendModeDissolve, 0.5, nil, clip)
	for y := clip.Y; y < clip.Y+clip.H; y++ {
		for x := clip.X; x < clip.X+clip.W; x++ {
			offset := (y*docW + x) * 4
			if clipDest[offset] != dest[offset] {
				t.Fatalf("clipped dissolve differs from full pass at (%d,%d): %d != %d", x, y, clipDest[offset], dest[offset])
			}
		}
	}
}

// S.5.5: pixelNoiseSeed must be roughly uniform. The previous hash always set
// bit 31 for small coordinates (x < ~59, y < ~222), so at 50% dissolve opacity
// nothing dissolved near the document origin.
func TestPixelNoiseSeedDistribution(t *testing.T) {
	// Determinism per (x, y).
	first := pixelNoiseSeed(5, 7)
	if again := pixelNoiseSeed(5, 7); again != first {
		t.Fatalf("pixelNoiseSeed must be deterministic per coordinate: %d != %d", first, again)
	}

	// Global uniformity: over a 256×256 grid the fraction of seeds in the
	// lower half of the uint32 range must be close to 0.5.
	below := 0
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			if pixelNoiseSeed(x, y) < 1<<31 {
				below++
			}
		}
	}
	fraction := float64(below) / (256 * 256)
	if fraction < 0.45 || fraction > 0.55 {
		t.Fatalf("fraction of seeds below 0.5 over 256x256 grid = %.4f, want within [0.45, 0.55]", fraction)
	}

	// Near-origin regression: the old hash produced zero seeds below 2^31 in
	// this region, so 50%-opacity dissolve left the area fully untouched.
	nearOriginBelow := 0
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if dissolveEnabled(0.5, pixelNoiseSeed(x, y)) {
				nearOriginBelow++
			}
		}
	}
	nearFraction := float64(nearOriginBelow) / (32 * 32)
	if nearFraction < 0.35 || nearFraction > 0.65 {
		t.Fatalf("near-origin dissolve fraction at 50%% opacity = %.4f, want within [0.35, 0.65] (biased hash regression)", nearFraction)
	}
}
