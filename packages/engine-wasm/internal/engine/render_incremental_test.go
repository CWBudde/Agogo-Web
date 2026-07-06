package engine

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"testing"
)

// --- helpers ---------------------------------------------------------------

func newDirtyRectTestInstance(t testing.TB, doc *Document, vp ViewportState) (*instance, *Document) {
	t.Helper()
	inst := &instance{
		manager:         newDocumentManager(),
		viewport:        vp,
		history:         newHistoryStack(defaultHistoryMax),
		foregroundColor: [4]uint8{0, 0, 0, 255},
		backgroundColor: [4]uint8{255, 255, 255, 255},
	}
	inst.manager.Create(doc)
	// Create clones the document; work against the manager's live copy so
	// mutations hit the same document the render path uses.
	return inst, inst.manager.activeMut()
}

func fillPseudoRandom(buf []byte, seed uint32) {
	s := seed
	for i := range buf {
		s = s*1664525 + 1013904223
		buf[i] = byte(s >> 24)
	}
}

func addPixelLayerForTest(doc *Document, name string, bounds LayerBounds, seed uint32) *PixelLayer {
	pixels := make([]byte, bounds.W*bounds.H*4)
	fillPseudoRandom(pixels, seed)
	layer := NewPixelLayer(name, bounds, pixels)
	root := doc.ensureLayerRoot()
	root.SetChildren(append(root.Children(), layer))
	layer.SetParent(root)
	return layer
}

// mutateLayerRect overwrites a bounds-local sub-rectangle of a pixel layer and
// bumps the document's content version with the matching doc-space dirty rect,
// exactly like a paint batch does.
func mutateLayerRect(doc *Document, layer *PixelLayer, local DirtyRect, seed uint32) {
	s := seed
	for y := local.Y; y < local.Y+local.H; y++ {
		row := (y*layer.Bounds.W + local.X) * 4
		for x := 0; x < local.W*4; x++ {
			s = s*1664525 + 1013904223
			layer.Pixels[row+x] = byte(s >> 24)
		}
	}
	doc.bumpContentVersionRect(DirtyRect{
		X: layer.Bounds.X + local.X,
		Y: layer.Bounds.Y + local.Y,
		W: local.W,
		H: local.H,
	})
}

// referenceComposite renders a from-scratch full composite of the document's
// current state, bypassing every cache.
func referenceComposite(t testing.TB, doc *Document) []byte {
	t.Helper()
	want, err := doc.renderCompositeSurfaceChecked()
	if err != nil {
		t.Fatalf("reference composite: %v", err)
	}
	return want
}

func testMaskCheckerboard(docW, docH int) *LayerMask {
	data := make([]byte, docW*docH)
	for y := range docH {
		for x := range docW {
			switch {
			case (x/3+y/3)%2 == 0:
				data[y*docW+x] = 255
			case (x+y)%5 == 0:
				data[y*docW+x] = 128
			}
		}
	}
	return &LayerMask{Enabled: true, Width: docW, Height: docH, Data: data}
}

// --- deliverable 1: incremental document recomposite ------------------------

func TestIncrementalCompositeEquivalenceBasicStack(t *testing.T) {
	const docW, docH = 64, 48
	doc := testDocumentFixture("inc-basic", "IncBasic", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 1)
	upper := addPixelLayerForTest(doc, "upper", LayerBounds{X: 10, Y: 8, W: 30, H: 20}, 2)
	upper.SetBlendMode(BlendModeMultiply)
	upper.SetOpacity(0.7)
	masked := addPixelLayerForTest(doc, "masked", LayerBounds{X: -5, Y: 20, W: 40, H: 40}, 3)
	masked.SetMask(testMaskCheckerboard(docW, docH))
	masked.SetBlendMode(BlendModeScreen)

	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
	base := doc.Layers()[0].(*PixelLayer)

	if _, err := inst.compositeSurfaceChecked(doc); err != nil {
		t.Fatalf("prime composite: %v", err)
	}

	mutateLayerRect(doc, base, DirtyRect{X: 12, Y: 10, W: 16, H: 12}, 99)
	got, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("incremental composite: %v", err)
	}
	if inst.incrementalCompositeCount != 1 {
		t.Fatalf("incrementalCompositeCount = %d, want 1", inst.incrementalCompositeCount)
	}
	if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
		t.Fatal("incremental composite differs from full recomposite")
	}
}

func TestIncrementalCompositeEquivalenceStyledLayer(t *testing.T) {
	const docW, docH = 48, 40
	doc := testDocumentFixture("inc-styled", "IncStyled", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 4)
	styled := addPixelLayerForTest(doc, "styled", LayerBounds{X: 14, Y: 12, W: 16, H: 12}, 5)
	styled.SetStyleStack([]LayerStyle{{
		Kind:    string(LayerStyleKindDropShadow),
		Enabled: true,
		Params:  json.RawMessage(`{"distance": 4, "size": 3, "opacity": 0.8, "angle": 120}`),
	}})

	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
	base := doc.Layers()[0].(*PixelLayer)

	if _, err := inst.compositeSurfaceChecked(doc); err != nil {
		t.Fatalf("prime composite: %v", err)
	}

	// Paint on the UNstyled base layer: the dirty rect stays tight, and the
	// styled layer's shadow (rendered from its own unchanged content) must
	// still land byte-identically inside the rect.
	mutateLayerRect(doc, base, DirtyRect{X: 10, Y: 8, W: 20, H: 16}, 100)
	got, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("incremental composite: %v", err)
	}
	if inst.incrementalCompositeCount != 1 {
		t.Fatalf("incrementalCompositeCount = %d, want 1 (styled layers must not force a full recomposite)", inst.incrementalCompositeCount)
	}
	if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
		t.Fatal("incremental composite with styled layer differs from full recomposite")
	}
}

func TestIncrementalCompositeAdjustmentLayerBailsToFull(t *testing.T) {
	const docW, docH = 40, 32
	doc := testDocumentFixture("inc-adj", "IncAdj", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 6)
	adj := NewAdjustmentLayer("bc", "brightness-contrast", json.RawMessage(`{"brightness": 25, "contrast": 15}`))
	root := doc.ensureLayerRoot()
	root.SetChildren(append(root.Children(), adj))

	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
	base := doc.Layers()[0].(*PixelLayer)

	if _, err := inst.compositeSurfaceChecked(doc); err != nil {
		t.Fatalf("prime composite: %v", err)
	}

	mutateLayerRect(doc, base, DirtyRect{X: 5, Y: 4, W: 10, H: 8}, 101)
	got, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("composite after paint: %v", err)
	}
	if inst.incrementalCompositeCount != 0 {
		t.Fatalf("incrementalCompositeCount = %d, want 0 (visible adjustment layer must force a full recomposite)", inst.incrementalCompositeCount)
	}
	if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
		t.Fatal("composite with adjustment layer differs from full recomposite")
	}
}

func TestIncrementalCompositeEquivalenceGroupAndClipping(t *testing.T) {
	const docW, docH = 56, 44
	doc := testDocumentFixture("inc-group", "IncGroup", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 7)

	group := NewGroupLayer("group")
	group.SetOpacity(0.6)
	group.SetBlendMode(BlendModeMultiply)
	group.SetMask(testMaskCheckerboard(docW, docH))
	childA := NewPixelLayer("childA", LayerBounds{X: 4, Y: 4, W: 24, H: 20}, make([]byte, 24*20*4))
	fillPseudoRandom(childA.Pixels, 8)
	childB := NewPixelLayer("childB", LayerBounds{X: 16, Y: 10, W: 24, H: 20}, make([]byte, 24*20*4))
	fillPseudoRandom(childB.Pixels, 9)
	childB.SetBlendMode(BlendModeScreen)
	group.SetChildren([]LayerNode{childA, childB})
	childA.SetParent(group)
	childB.SetParent(group)

	clipBase := addPixelLayerForTest(doc, "clipBase", LayerBounds{X: 20, Y: 18, W: 20, H: 16}, 10)
	clipped := addPixelLayerForTest(doc, "clipped", LayerBounds{X: 14, Y: 12, W: 30, H: 26}, 11)
	clipped.SetClipToBelow(true)
	_ = clipBase

	root := doc.ensureLayerRoot()
	children := root.Children()
	// Insert the group between base and the clipping pair.
	reordered := []LayerNode{children[0], group, children[1], children[2]}
	root.SetChildren(reordered)
	group.SetParent(root)
	doc.normalizeClippingState()

	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
	base := doc.Layers()[0].(*PixelLayer)

	if _, err := inst.compositeSurfaceChecked(doc); err != nil {
		t.Fatalf("prime composite: %v", err)
	}

	// A rect overlapping the group, the clipping pair and a doc edge.
	mutateLayerRect(doc, base, DirtyRect{X: 0, Y: 6, W: 30, H: 24}, 102)
	got, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("incremental composite: %v", err)
	}
	if inst.incrementalCompositeCount != 1 {
		t.Fatalf("incrementalCompositeCount = %d, want 1", inst.incrementalCompositeCount)
	}
	if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
		t.Fatal("incremental composite with group/clipping differs from full recomposite")
	}
}

func TestIncrementalCompositeFullDocRectUsesFullPath(t *testing.T) {
	const docW, docH = 32, 24
	doc := testDocumentFixture("inc-full", "IncFull", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 12)

	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
	base := doc.Layers()[0].(*PixelLayer)

	primed, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("prime composite: %v", err)
	}

	mutateLayerRect(doc, base, DirtyRect{X: 0, Y: 0, W: docW, H: docH}, 103)
	got, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("composite: %v", err)
	}
	if inst.incrementalCompositeCount != 0 {
		t.Fatalf("incrementalCompositeCount = %d, want 0 for a full-document dirty rect", inst.incrementalCompositeCount)
	}
	// In-place buffer reuse: the full path should recycle the cached buffer.
	if len(primed) > 0 && len(got) > 0 && &primed[0] != &got[0] {
		t.Error("expected full recomposite to reuse the cached surface buffer in place")
	}
	if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
		t.Fatal("in-place full recomposite differs from fresh full recomposite")
	}
}

// TestIncrementalCompositeStaleDirtyRectFromSnapshotRestore simulates the
// undo/redo hazard: snapshot restores install a document CLONE that carries
// its dirty-rect state from snapshot time, which is unrelated to the delta
// between the engine's cached surface and the restored content. The
// dirtyCompositeBase provenance check must force a full recomposite instead
// of trusting the stale rect.
func TestIncrementalCompositeStaleDirtyRectFromSnapshotRestore(t *testing.T) {
	const docW, docH = 40, 30
	doc := testDocumentFixture("inc-stale", "IncStale", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 30)

	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
	base := doc.Layers()[0].(*PixelLayer)

	if _, err := inst.compositeSurfaceChecked(doc); err != nil {
		t.Fatalf("prime composite: %v", err)
	}

	// Simulate a restored snapshot: the content changed in region B (bottom
	// half) but the document carries a stale dirty rect A (top-left corner)
	// whose base version does NOT match the cached composite.
	for y := 20; y < 30; y++ {
		for x := 0; x < docW; x++ {
			i := (y*docW + x) * 4
			base.Pixels[i] = 200
			base.Pixels[i+1] = 10
			base.Pixels[i+2] = 10
			base.Pixels[i+3] = 255
		}
	}
	doc.ContentVersion += 1000000 // unique, differs from the cached version
	doc.dirtyComposite = DirtyRect{X: 0, Y: 0, W: 8, H: 8}
	doc.hasDirtyComposite = true
	doc.dirtyCompositeBase = doc.ContentVersion - 5 // stale provenance

	got, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("composite: %v", err)
	}
	if inst.incrementalCompositeCount != 0 {
		t.Fatalf("incrementalCompositeCount = %d, want 0 (stale dirty rect must not be trusted)", inst.incrementalCompositeCount)
	}
	if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
		t.Fatal("composite after simulated snapshot restore differs from full recomposite")
	}
}

func TestIncrementalCompositeEquivalenceRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	blendModes := []BlendMode{BlendModeNormal, BlendModeMultiply, BlendModeScreen}

	for iter := 0; iter < 30; iter++ {
		docW := 24 + rng.Intn(60)
		docH := 20 + rng.Intn(50)
		doc := testDocumentFixture("inc-rand", "IncRand", docW, docH)

		layerCount := 2 + rng.Intn(4)
		var pixelLayers []*PixelLayer
		for l := 0; l < layerCount; l++ {
			bounds := LayerBounds{
				X: rng.Intn(docW) - docW/4,
				Y: rng.Intn(docH) - docH/4,
				W: 8 + rng.Intn(docW),
				H: 8 + rng.Intn(docH),
			}
			layer := addPixelLayerForTest(doc, "L", bounds, uint32(iter*100+l)) //nolint:gosec // test seed
			layer.SetBlendMode(blendModes[rng.Intn(len(blendModes))])
			layer.SetOpacity(0.3 + rng.Float64()*0.7)
			if rng.Intn(3) == 0 {
				layer.SetMask(testMaskCheckerboard(docW, docH))
			}
			if l > 0 && rng.Intn(4) == 0 {
				layer.SetClipToBelow(true)
			}
			pixelLayers = append(pixelLayers, layer)
		}
		doc.normalizeClippingState()

		inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
		// Re-resolve the layers from the cloned document.
		pixelLayers = pixelLayers[:0]
		for _, node := range doc.Layers() {
			if pl, ok := node.(*PixelLayer); ok {
				pixelLayers = append(pixelLayers, pl)
			}
		}

		if _, err := inst.compositeSurfaceChecked(doc); err != nil {
			t.Fatalf("iter %d: prime composite: %v", iter, err)
		}

		target := pixelLayers[rng.Intn(len(pixelLayers))]
		local := DirtyRect{
			X: rng.Intn(target.Bounds.W),
			Y: rng.Intn(target.Bounds.H),
			W: 1 + rng.Intn(target.Bounds.W),
			H: 1 + rng.Intn(target.Bounds.H),
		}
		if local.X+local.W > target.Bounds.W {
			local.W = target.Bounds.W - local.X
		}
		if local.Y+local.H > target.Bounds.H {
			local.H = target.Bounds.H - local.Y
		}
		mutateLayerRect(doc, target, local, uint32(iter+7000)) //nolint:gosec // test seed

		got, err := inst.compositeSurfaceChecked(doc)
		if err != nil {
			t.Fatalf("iter %d: composite: %v", iter, err)
		}
		if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
			t.Fatalf("iter %d: incremental composite differs from full recomposite (docW=%d docH=%d layer bounds=%+v local=%+v)",
				iter, docW, docH, target.Bounds, local)
		}
	}
}
