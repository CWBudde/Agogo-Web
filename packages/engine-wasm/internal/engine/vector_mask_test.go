package engine

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

// --- helpers ---------------------------------------------------------------

func solidPixelsRGBA(w, h int, c [4]uint8) []byte {
	pixels := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pixels[i*4] = c[0]
		pixels[i*4+1] = c[1]
		pixels[i*4+2] = c[2]
		pixels[i*4+3] = c[3]
	}
	return pixels
}

func addSolidLayerForVMTest(doc *Document, name string, bounds LayerBounds, c [4]uint8) *PixelLayer {
	layer := NewPixelLayer(name, bounds, solidPixelsRGBA(bounds.W, bounds.H, c))
	root := doc.ensureLayerRoot()
	root.SetChildren(append(root.Children(), layer))
	layer.SetParent(root)
	return layer
}

func vmAlphaAt(surface []byte, docW, x, y int) uint8 {
	return surface[(y*docW+x)*4+3]
}

// placeholderVectorMask mirrors the reveal-all path created by AddVectorMask.
func placeholderVectorMask() *Path {
	return &Path{Subpaths: []Subpath{{Closed: true}}}
}

// --- compositing semantics ---------------------------------------------------

// A rectangular vector mask must clip the layer: outside transparent, inside
// unchanged, with intermediate anti-aliased coverage on half-pixel boundaries.
func TestVectorMaskClipsComposite(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-clip", "VMClip", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{255, 0, 0, 255})

	// Half-pixel rect edges force fractional AA coverage at the boundary.
	sq := makeSquarePath(4.5, 4.5, 27.5, 27.5)
	layer.SetVectorMask(&sq)

	got := referenceComposite(t, doc)

	if a := vmAlphaAt(got, docW, 16, 16); a != 255 {
		t.Fatalf("inside alpha = %d, want 255", a)
	}
	if r := got[(16*docW+16)*4]; r != 255 {
		t.Fatalf("inside red = %d, want 255", r)
	}
	if a := vmAlphaAt(got, docW, 1, 1); a != 0 {
		t.Fatalf("outside alpha = %d, want 0", a)
	}
	foundAA := false
	for x := 0; x < docW; x++ {
		if a := vmAlphaAt(got, docW, x, 16); a > 0 && a < 255 {
			foundAA = true
			break
		}
	}
	if !foundAA {
		t.Fatal("expected intermediate anti-aliased alpha along the mask boundary")
	}
}

// Raster and vector masks intersect: coverage is multiplied (Photoshop
// behavior), so a 50%-gray raster mask under a full vector mask halves alpha.
func TestVectorMaskMultipliesWithRasterMask(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-mul", "VMMul", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{0, 255, 0, 255})

	grayData := make([]byte, docW*docH)
	for i := range grayData {
		grayData[i] = 128
	}
	layer.SetMask(&LayerMask{Enabled: true, Width: docW, Height: docH, Data: grayData})
	sq := makeSquarePath(4, 4, 28, 28)
	layer.SetVectorMask(&sq)

	got := referenceComposite(t, doc)

	inside := vmAlphaAt(got, docW, 16, 16)
	if inside < 126 || inside > 130 {
		t.Fatalf("inside alpha = %d, want ~128 (raster 50%% x vector 100%%)", inside)
	}
	if a := vmAlphaAt(got, docW, 1, 1); a != 0 {
		t.Fatalf("outside alpha = %d, want 0", a)
	}
}

// The AddVectorMask placeholder (one closed subpath, no anchor points) means
// reveal-all and must keep the composite byte-identical to no mask at all.
func TestVectorMaskEmptyPlaceholderIsRevealAll(t *testing.T) {
	const docW, docH = 24, 24
	doc := testDocumentFixture("vm-empty", "VMEmpty", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{X: 3, Y: 5, W: 12, H: 10}, [4]uint8{10, 20, 200, 255})

	want := referenceComposite(t, doc)

	if err := doc.AddVectorMask(layer.ID(), false); err != nil {
		t.Fatalf("AddVectorMask: %v", err)
	}
	got := referenceComposite(t, doc)
	if !bytes.Equal(got, want) {
		t.Fatal("empty vector mask placeholder changed the composite; must be byte-identical to no mask")
	}
}

// Vector masks have no enabled flag: a DISABLED raster mask must not disable
// the vector mask.
func TestVectorMaskAppliesWhenRasterMaskDisabled(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-disabled", "VMDisabled", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{255, 0, 0, 255})

	// A hide-all raster mask that is disabled: it must contribute nothing.
	layer.SetMask(&LayerMask{Enabled: false, Width: docW, Height: docH, Data: make([]byte, docW*docH)})
	sq := makeSquarePath(4, 4, 28, 28)
	layer.SetVectorMask(&sq)

	got := referenceComposite(t, doc)
	if a := vmAlphaAt(got, docW, 16, 16); a != 255 {
		t.Fatalf("inside alpha = %d, want 255 (disabled raster mask must not hide content)", a)
	}
	if a := vmAlphaAt(got, docW, 1, 1); a != 0 {
		t.Fatalf("outside alpha = %d, want 0 (vector mask must still apply)", a)
	}
}

// A styled layer's effects must follow the vector-masked silhouette: a layer
// clipped by a vector mask renders byte-identically to a layer whose pixel
// bounds equal the masked region.
func TestVectorMaskClipsStyledLayerSilhouette(t *testing.T) {
	const docW, docH = 48, 40
	style := []LayerStyle{{
		Kind:    string(LayerStyleKindDropShadow),
		Enabled: true,
		Params:  json.RawMessage(`{"distance": 4, "size": 3, "opacity": 0.8, "angle": 120}`),
	}}

	masked := testDocumentFixture("vm-style-a", "VMStyleA", docW, docH)
	layer := addSolidLayerForVMTest(masked, "styled", LayerBounds{X: 10, Y: 10, W: 20, H: 20}, [4]uint8{200, 30, 30, 255})
	layer.SetStyleStack(style)
	sq := makeSquarePath(10, 10, 20, 30)
	layer.SetVectorMask(&sq)

	reference := testDocumentFixture("vm-style-b", "VMStyleB", docW, docH)
	refLayer := addSolidLayerForVMTest(reference, "styled", LayerBounds{X: 10, Y: 10, W: 10, H: 20}, [4]uint8{200, 30, 30, 255})
	refLayer.SetStyleStack(style)

	got := referenceComposite(t, masked)
	want := referenceComposite(t, reference)
	if !bytes.Equal(got, want) {
		t.Fatal("styled layer with vector mask differs from layer with equivalent pixel silhouette; effects must follow the mask")
	}

	// Sanity: the shadow itself must exist somewhere outside the silhouette.
	foundShadow := false
	for y := 0; y < docH && !foundShadow; y++ {
		for x := 0; x < docW; x++ {
			inSilhouette := x >= 10 && x < 20 && y >= 10 && y < 30
			if !inSilhouette && vmAlphaAt(got, docW, x, y) > 0 {
				foundShadow = true
				break
			}
		}
	}
	if !foundShadow {
		t.Fatal("expected drop shadow ink outside the masked silhouette")
	}
}

// A vector mask on a group clips all children; a placeholder vector mask keeps
// the pass-through fast path byte-identical (the child uses a non-normal blend
// mode so a wrong isolated-surface detour would change bytes).
func TestVectorMaskOnGroupClipsChildren(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-group", "VMGroup", docW, docH)
	addSolidLayerForVMTest(doc, "base", LayerBounds{W: docW, H: docH}, [4]uint8{0, 200, 0, 255})

	child := NewPixelLayer("child", LayerBounds{W: docW, H: docH}, solidPixelsRGBA(docW, docH, [4]uint8{200, 40, 40, 255}))
	child.SetBlendMode(BlendModeMultiply)
	group := NewGroupLayer("Group")
	group.SetChildren([]LayerNode{child})
	child.SetParent(group)
	root := doc.ensureLayerRoot()
	root.SetChildren(append(root.Children(), group))
	group.SetParent(root)

	baseline := referenceComposite(t, doc)

	// Placeholder mask on the group: must stay byte-identical (pass-through).
	group.SetVectorMask(placeholderVectorMask())
	if got := referenceComposite(t, doc); !bytes.Equal(got, baseline) {
		t.Fatal("placeholder vector mask on group changed the composite")
	}

	// Real vector mask: must behave exactly like the equivalent RASTER mask on
	// the group (the pre-existing masked-group semantics, isolated-surface
	// path included): inside the rect the group contributes, outside it the
	// composite equals the base alone.
	sq := makeSquarePath(8, 8, 24, 24)
	group.SetVectorMask(&sq)
	got := referenceComposite(t, doc)

	group.SetVectorMask(nil)
	rectRaster := make([]byte, docW*docH)
	for y := 8; y < 24; y++ {
		for x := 8; x < 24; x++ {
			rectRaster[y*docW+x] = 255
		}
	}
	group.SetMask(&LayerMask{Enabled: true, Width: docW, Height: docH, Data: rectRaster})
	want := referenceComposite(t, doc)

	if !bytes.Equal(got, want) {
		t.Fatal("vector mask on group differs from the equivalent raster mask on the group")
	}
	insideIdx := (16*docW + 16) * 4
	if got[insideIdx+3] == 0 {
		t.Fatal("inside mask pixel is transparent; group content must show through")
	}
	baseOnly := testDocumentFixture("vm-group-ref", "VMGroupRef", docW, docH)
	addSolidLayerForVMTest(baseOnly, "base", LayerBounds{W: docW, H: docH}, [4]uint8{0, 200, 0, 255})
	baseOnlySurface := referenceComposite(t, baseOnly)
	outsideIdx := (2*docW + 2) * 4
	if !bytes.Equal(got[outsideIdx:outsideIdx+4], baseOnlySurface[outsideIdx:outsideIdx+4]) {
		t.Fatalf("outside mask pixel = %v, want %v (base only)", got[outsideIdx:outsideIdx+4], baseOnlySurface[outsideIdx:outsideIdx+4])
	}
}

// --- cache behavior ----------------------------------------------------------

func TestVectorMaskRasterCacheReuseAndInvalidation(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-cache", "VMCache", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{255, 0, 0, 255})
	sq := makeSquarePath(4, 4, 20, 20)
	layer.SetVectorMask(&sq)

	c1 := vectorMaskCoverage(layer, docW, docH)
	if c1 == nil {
		t.Fatal("vectorMaskCoverage returned nil for a non-empty vector mask")
	}
	c2 := vectorMaskCoverage(layer, docW, docH)
	if &c1[0] != &c2[0] {
		t.Fatal("second call must reuse the cached coverage slice")
	}

	// Transforms mutate the mask path IN PLACE through the pointer; the cache
	// must detect the content change without any invalidation hook.
	vm := layer.VectorMask()
	for i := range vm.Subpaths[0].Points {
		p := &vm.Subpaths[0].Points[i]
		p.X += 8
		p.InX += 8
		p.OutX += 8
	}
	c3 := vectorMaskCoverage(layer, docW, docH)
	if &c3[0] == &c1[0] {
		t.Fatal("in-place path mutation must refresh the cached coverage")
	}
	if c3[10*docW+5] != 0 || c3[10*docW+16] != 255 {
		t.Fatalf("refreshed coverage does not reflect translated geometry: left=%d right=%d", c3[10*docW+5], c3[10*docW+16])
	}

	// Document dimension change refreshes the cache too.
	c4 := vectorMaskCoverage(layer, 16, 16)
	if len(c4) != 16*16 {
		t.Fatalf("coverage length = %d, want %d after dimension change", len(c4), 16*16)
	}
	if cache := layer.VectorMaskRaster(); cache == nil || cache.W != 16 || cache.H != 16 {
		t.Fatalf("cache dims not refreshed: %+v", cache)
	}

	// The cache key path is a deep clone: mutating the live path must not
	// alter the stored key (it must simply stop matching).
	if model.PathEqual(layer.VectorMaskRaster().Path, layer.VectorMask()) {
		vm.Subpaths[0].Points[0].Y += 1
		if model.PathEqual(layer.VectorMaskRaster().Path, layer.VectorMask()) {
			t.Fatal("cache key path aliases the live vector mask; must be a deep clone")
		}
	}
}

// Empty and nil vector masks yield nil coverage (reveal all, zero cost).
func TestVectorMaskCoverageEmptyIsNil(t *testing.T) {
	const docW, docH = 16, 16
	doc := testDocumentFixture("vm-nilcov", "VMNilCov", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{1, 2, 3, 255})

	if cov := vectorMaskCoverage(layer, docW, docH); cov != nil {
		t.Fatal("nil vector mask must yield nil coverage")
	}
	layer.SetVectorMask(placeholderVectorMask())
	if cov := vectorMaskCoverage(layer, docW, docH); cov != nil {
		t.Fatal("empty vector mask (no anchor points) must yield nil coverage")
	}
	if layer.VectorMaskRaster() != nil {
		t.Fatal("empty vector mask must not allocate a raster cache")
	}
}

// --- ABI / dispatch ----------------------------------------------------------

func TestSetVectorMaskPathDispatchAndUndo(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-dispatch", "VMDispatch", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{255, 0, 0, 255})
	layerID := layer.ID()
	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})

	baseline := referenceComposite(t, doc)

	// SetVectorMaskPath on a layer WITHOUT a vector mask must error.
	sq := makeSquarePath(4, 4, 28, 28)
	if _, err := inst.dispatchLayerCommand(commandSetVectorMaskPath, mustJSON(t, SetVectorMaskPathPayload{LayerID: layerID, Path: &sq})); err == nil {
		t.Fatal("SetVectorMaskPath without an existing vector mask must fail")
	}

	// Old bare AddVectorMask payload (no fromActivePath) must still decode.
	if _, err := inst.dispatchLayerCommand(commandAddVectorMask, `{"layerId":"`+layerID+`"}`); err != nil {
		t.Fatalf("AddVectorMask (legacy payload): %v", err)
	}
	doc = inst.manager.activeMut()
	if got := referenceComposite(t, doc); !bytes.Equal(got, baseline) {
		t.Fatal("AddVectorMask placeholder changed rendering; must be byte-identical")
	}

	// SetVectorMaskPath replaces the path and changes rendering.
	if _, err := inst.dispatchLayerCommand(commandSetVectorMaskPath, mustJSON(t, SetVectorMaskPathPayload{LayerID: layerID, Path: &sq})); err != nil {
		t.Fatalf("SetVectorMaskPath: %v", err)
	}
	doc = inst.manager.activeMut()
	updated, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		t.Fatal("layer lost after SetVectorMaskPath")
	}
	if !model.PathEqual(updated.VectorMask(), &sq) {
		t.Fatalf("vector mask path = %+v, want the dispatched rect", updated.VectorMask())
	}
	masked := referenceComposite(t, doc)
	if a := vmAlphaAt(masked, docW, 1, 1); a != 0 {
		t.Fatalf("outside alpha after SetVectorMaskPath = %d, want 0", a)
	}
	if a := vmAlphaAt(masked, docW, 16, 16); a != 255 {
		t.Fatalf("inside alpha after SetVectorMaskPath = %d, want 255", a)
	}

	// Undo restores the reveal-all placeholder and the baseline rendering.
	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("undo: %v", err)
	}
	doc = inst.manager.activeMut()
	if got := referenceComposite(t, doc); !bytes.Equal(got, baseline) {
		t.Fatal("undo of SetVectorMaskPath did not restore the baseline rendering")
	}
}

func TestAddVectorMaskFromActivePath(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-frompath", "VMFromPath", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{255, 0, 0, 255})
	layerID := layer.ID()
	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})

	sq := makeSquarePath(6, 6, 26, 26)
	doc.Paths = []NamedPath{{Name: "Work Path", Path: sq}}
	doc.ActivePathIdx = 0

	if _, err := inst.dispatchLayerCommand(commandAddVectorMask, mustJSON(t, AddVectorMaskPayload{LayerID: layerID, FromActivePath: true})); err != nil {
		t.Fatalf("AddVectorMask fromActivePath: %v", err)
	}
	doc = inst.manager.activeMut()
	updated, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		t.Fatal("layer lost after AddVectorMask")
	}
	if !model.PathEqual(updated.VectorMask(), &sq) {
		t.Fatalf("vector mask = %+v, want a copy of the active stored path", updated.VectorMask())
	}

	// The mask must be a deep clone, not an alias of the stored path.
	doc.Paths[0].Path.Subpaths[0].Points[0].X += 5
	if !model.PathEqual(updated.VectorMask(), &sq) {
		t.Fatal("vector mask aliases the stored path; must be a deep clone")
	}

	got := referenceComposite(t, doc)
	if a := vmAlphaAt(got, docW, 1, 1); a != 0 {
		t.Fatalf("outside alpha = %d, want 0 (seeded mask must clip)", a)
	}
}

// --- incremental recomposite ---------------------------------------------------

func TestIncrementalCompositeEquivalenceVectorMask(t *testing.T) {
	const docW, docH = 64, 48
	doc := testDocumentFixture("inc-vm", "IncVM", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 11)
	masked := addPixelLayerForTest(doc, "masked", LayerBounds{X: 4, Y: 4, W: 50, H: 36}, 12)
	sq := makeSquarePath(8.5, 6.5, 44.5, 34.5)
	masked.SetVectorMask(&sq)
	masked.SetBlendMode(BlendModeScreen)

	inst, doc := newDirtyRectTestInstance(t, doc, ViewportState{Zoom: 1, CanvasW: docW, CanvasH: docH})
	base := doc.Layers()[0].(*PixelLayer)

	if _, err := inst.compositeSurfaceChecked(doc); err != nil {
		t.Fatalf("prime composite: %v", err)
	}

	mutateLayerRect(doc, base, DirtyRect{X: 12, Y: 10, W: 16, H: 12}, 77)
	got, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("incremental composite: %v", err)
	}
	if inst.incrementalCompositeCount != 1 {
		t.Fatalf("incrementalCompositeCount = %d, want 1", inst.incrementalCompositeCount)
	}
	if want := referenceComposite(t, doc); !bytes.Equal(got, want) {
		t.Fatal("incremental composite over a vector-masked layer differs from full recomposite")
	}
}

// --- persistence -----------------------------------------------------------------

func TestProjectArchiveRoundTripRendersVectorMask(t *testing.T) {
	const docW, docH = 32, 32
	doc := testDocumentFixture("vm-archive", "VMArchive", docW, docH)
	layer := addSolidLayerForVMTest(doc, "solid", LayerBounds{W: docW, H: docH}, [4]uint8{255, 0, 0, 255})
	sq := makeSquarePath(4, 4, 28, 28)
	layer.SetVectorMask(&sq)

	want := referenceComposite(t, doc)
	if a := vmAlphaAt(want, docW, 1, 1); a != 0 {
		t.Fatalf("precondition: vector mask must clip before archiving (alpha=%d)", a)
	}

	archive, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatalf("save project: %v", err)
	}
	restored, _, err := LoadProject(archive)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	got := referenceComposite(t, restored)
	if !bytes.Equal(got, want) {
		t.Fatal("vector mask does not render identically after archive round-trip")
	}
}
