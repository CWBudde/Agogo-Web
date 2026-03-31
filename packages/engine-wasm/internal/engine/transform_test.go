package engine

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeSolidPixels creates a w×h RGBA buffer filled with (r,g,b,a).
func makeSolidPixels(w, h int, r, g, b, a byte) []byte {
	buf := make([]byte, w*h*4)
	for i := 0; i < len(buf); i += 4 {
		buf[i] = r
		buf[i+1] = g
		buf[i+2] = b
		buf[i+3] = a
	}
	return buf
}

// pixelRGBA returns the RGBA at (x, y) within a w-wide buffer.
func pixelRGBA(buf []byte, w, x, y int) (r, g, b, a byte) {
	i := (y*w + x) * 4
	return buf[i], buf[i+1], buf[i+2], buf[i+3]
}

// ---------------------------------------------------------------------------
// identityState builds a FreeTransformState with the identity transform for a
// layer with the given pixel dimensions placed at origin.
func identityState(w, h int, pixels []byte) *FreeTransformState {
	return &FreeTransformState{
		Active:         true,
		LayerID:        "test",
		OriginalPixels: pixels,
		OriginalBounds: LayerBounds{X: 0, Y: 0, W: w, H: h},
		A:              1, B: 0, C: 0, D: 1,
		TX: 0, TY: 0,
		PivotX:        float64(w) / 2,
		PivotY:        float64(h) / 2,
		Interpolation: InterpolNearest,
	}
}

// ---------------------------------------------------------------------------
// applyPixelTransform tests
// ---------------------------------------------------------------------------

func TestApplyPixelTransform_Identity(t *testing.T) {
	const w, h = 4, 4
	pixels := makeSolidPixels(w, h, 255, 0, 0, 255) // solid red
	// Mark corner pixel distinctively.
	pixels[0], pixels[1], pixels[2], pixels[3] = 0, 255, 0, 255 // TL = green

	s := identityState(w, h, pixels)
	outPixels, outBounds := applyPixelTransform(s, InterpolNearest)

	if outBounds.W != w || outBounds.H != h {
		t.Fatalf("identity transform changed size: got %dx%d, want %dx%d",
			outBounds.W, outBounds.H, w, h)
	}
	r, g, b, a := pixelRGBA(outPixels, outBounds.W, 0, 0)
	if r != 0 || g != 255 || b != 0 || a != 255 {
		t.Errorf("TL pixel = (%d,%d,%d,%d), want (0,255,0,255)", r, g, b, a)
	}
}

func TestApplyPixelTransform_Scale2x(t *testing.T) {
	const w, h = 4, 4
	pixels := makeSolidPixels(w, h, 200, 100, 50, 255)
	s := identityState(w, h, pixels)
	// Scale by 2×: doc = layer * 2 — set matrix to [[2,0],[0,2]] at origin.
	s.A = 2
	s.D = 2

	outPixels, outBounds := applyPixelTransform(s, InterpolNearest)

	wantW, wantH := 8, 8
	if outBounds.W != wantW || outBounds.H != wantH {
		t.Fatalf("2× scale output size: got %dx%d, want %dx%d",
			outBounds.W, outBounds.H, wantW, wantH)
	}
	// Centre pixel should match source colour.
	r, g, b, a := pixelRGBA(outPixels, outBounds.W, 4, 4)
	if r != 200 || g != 100 || b != 50 || a != 255 {
		t.Errorf("centre pixel = (%d,%d,%d,%d), want (200,100,50,255)", r, g, b, a)
	}
}

func TestApplyPixelTransform_Degenerate(t *testing.T) {
	// Singular matrix — should return blank pixels at original size.
	pixels := makeSolidPixels(4, 4, 255, 0, 0, 255)
	s := identityState(4, 4, pixels)
	s.A = 0
	s.D = 0 // det = 0

	outPixels, outBounds := applyPixelTransform(s, InterpolBilinear)
	if outBounds.W != 4 || outBounds.H != 4 {
		t.Errorf("degenerate: unexpected bounds %+v", outBounds)
	}
	for i := 0; i < len(outPixels); i++ {
		if outPixels[i] != 0 {
			t.Errorf("degenerate: pixel[%d] = %d, want 0", i, outPixels[i])
		}
	}
}

// ---------------------------------------------------------------------------
// sampleBilinear tests
// ---------------------------------------------------------------------------

func TestSampleBilinear_CentrePixel(t *testing.T) {
	// 2×2 all-white image — sampling at any coordinate should give white.
	pixels := makeSolidPixels(2, 2, 255, 255, 255, 255)
	got := sampleBilinear(pixels, 2, 2, 1.0, 1.0)
	for i, v := range got {
		if v != 255 {
			t.Errorf("channel %d = %d, want 255", i, v)
		}
	}
}

func TestSampleBilinear_Interpolation(t *testing.T) {
	// 1×2 image: top pixel red (255,0,0,255), bottom pixel blue (0,0,255,255).
	pixels := []byte{
		255, 0, 0, 255, // top
		0, 0, 255, 255, // bottom
	}
	// At ly=1.0 (midpoint between top and bottom) we expect equal mix.
	got := sampleBilinear(pixels, 1, 2, 0.5, 1.0)
	// Midpoint should be ≈127/128.
	const tol = 5
	if math.Abs(float64(got[0])-127) > tol || math.Abs(float64(got[2])-127) > tol {
		t.Errorf("midpoint bilinear = (%d,%d,%d,%d), want ≈(127,0,127,255)", got[0], got[1], got[2], got[3])
	}
}

// ---------------------------------------------------------------------------
// sampleNearest tests
// ---------------------------------------------------------------------------

func TestSampleNearest_ExactPixel(t *testing.T) {
	// 3×3 image: only the centre pixel is non-zero.
	pixels := make([]byte, 3*3*4)
	pixels[(1*3+1)*4+1] = 200 // centre green
	pixels[(1*3+1)*4+3] = 255

	got := sampleNearest(pixels, 3, 3, 1.5, 1.5)
	if got[1] != 200 || got[3] != 255 {
		t.Errorf("nearest centre: got (%d,%d,%d,%d), want (0,200,0,255)", got[0], got[1], got[2], got[3])
	}
}

// ---------------------------------------------------------------------------
// flipPixelsH tests
// ---------------------------------------------------------------------------

func TestFlipPixelsH_2x1(t *testing.T) {
	// 2×1: left=red, right=blue.
	pixels := []byte{
		255, 0, 0, 255, // left
		0, 0, 255, 255, // right
	}
	out := flipPixelsH(pixels, 2, 1)
	// After flip: left=blue, right=red.
	if out[0] != 0 || out[2] != 255 {
		t.Errorf("flipH 2×1: left pixel = (%d,%d,%d,%d), want (0,0,255,255)", out[0], out[1], out[2], out[3])
	}
	if out[4] != 255 || out[6] != 0 {
		t.Errorf("flipH 2×1: right pixel = (%d,%d,%d,%d), want (255,0,0,255)", out[4], out[5], out[6], out[7])
	}
}

func TestFlipPixelsH_Idempotent(t *testing.T) {
	pixels := makeSolidPixels(3, 3, 12, 34, 56, 255)
	pixels[0], pixels[1], pixels[2], pixels[3] = 1, 2, 3, 4 // mark TL
	once := flipPixelsH(pixels, 3, 3)
	twice := flipPixelsH(once, 3, 3)
	for i, v := range pixels {
		if twice[i] != v {
			t.Fatalf("flipH twice != original at byte %d", i)
		}
	}
}

// ---------------------------------------------------------------------------
// flipPixelsV tests
// ---------------------------------------------------------------------------

func TestFlipPixelsV_2x2(t *testing.T) {
	// 2×2: top-left=red, others=zero.
	pixels := make([]byte, 2*2*4)
	pixels[0] = 255
	pixels[3] = 255

	out := flipPixelsV(pixels, 2, 2)
	// After vertical flip: bottom-left (row 1, col 0) should be red.
	r, g, b, a := pixelRGBA(out, 2, 0, 1)
	if r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("flipV: bottom-left = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}
	// Top-left should now be zero.
	r2, _, _, _ := pixelRGBA(out, 2, 0, 0)
	if r2 != 0 {
		t.Errorf("flipV: top-left red channel = %d, want 0", r2)
	}
}

// ---------------------------------------------------------------------------
// rotatePixels90CW tests
// ---------------------------------------------------------------------------

func TestRotatePixels90CW_2x3(t *testing.T) {
	// 2-wide × 3-tall → after CW becomes 3-wide × 2-tall.
	pixels := make([]byte, 2*3*4)
	// Mark top-left (0,0) as red.
	pixels[0] = 200
	pixels[3] = 255

	out, newW, newH := rotatePixels90CW(pixels, 2, 3)
	if newW != 3 || newH != 2 {
		t.Fatalf("rotatePixels90CW dims: got %dx%d, want 3x2", newW, newH)
	}
	// After 90° CW, TL (0,0) maps to top-right (W-1, 0) = (2, 0).
	r, _, _, a := pixelRGBA(out, newW, 2, 0)
	if r != 200 || a != 255 {
		t.Errorf("rotatePixels90CW: top-right = (%d,...,%d), want (200,...,255)", r, a)
	}
}

func TestRotatePixels90CW_4xIdentity(t *testing.T) {
	// Four 90° CW rotations on a square should be the identity.
	pixels := make([]byte, 4*4*4)
	for i := 0; i < 4*4; i++ {
		pixels[i*4] = byte(i * 16 % 256)
		pixels[i*4+3] = 255
	}
	w, h := 4, 4
	orig := make([]byte, len(pixels))
	copy(orig, pixels)

	cur := pixels
	for range 4 {
		cur, w, h = rotatePixels90CW(cur, w, h)
	}
	if w != 4 || h != 4 {
		t.Fatalf("after 4× CW: dims = %dx%d, want 4×4", w, h)
	}
	for i, v := range orig {
		if cur[i] != v {
			t.Fatalf("after 4× CW: pixel[%d] = %d, want %d", i, cur[i], v)
		}
	}
}

// ---------------------------------------------------------------------------
// rotatePixels90CCW tests
// ---------------------------------------------------------------------------

func TestRotatePixels90CCW_DimsSwap(t *testing.T) {
	pixels := makeSolidPixels(5, 3, 100, 100, 100, 255)
	_, newW, newH := rotatePixels90CCW(pixels, 5, 3)
	if newW != 3 || newH != 5 {
		t.Errorf("rotatePixels90CCW dims: got %dx%d, want 3x5", newW, newH)
	}
}

func TestRotatePixels90CW_CCW_Inverse(t *testing.T) {
	// CW then CCW should give back the original.
	pixels := make([]byte, 3*4*4)
	for i := range len(pixels) {
		pixels[i] = byte(i)
	}
	orig := make([]byte, len(pixels))
	copy(orig, pixels)

	cw, cwW, cwH := rotatePixels90CW(pixels, 3, 4)
	back, _, _ := rotatePixels90CCW(cw, cwW, cwH)
	for i, v := range orig {
		if back[i] != v {
			t.Fatalf("CW+CCW: pixel[%d] = %d, want %d", i, back[i], v)
		}
	}
}

// ---------------------------------------------------------------------------
// rotatePixels180 tests
// ---------------------------------------------------------------------------

func TestRotatePixels180_Idempotent(t *testing.T) {
	pixels := make([]byte, 4*3*4)
	for i := range len(pixels) {
		pixels[i] = byte(i % 256)
	}
	once := rotatePixels180(pixels, 4, 3)
	twice := rotatePixels180(once, 4, 3)
	for i, v := range pixels {
		if twice[i] != v {
			t.Fatalf("rotate180 twice != original at byte %d", i)
		}
	}
}

func TestRotatePixels180_CornerSwap(t *testing.T) {
	// 3×3: TL=red, BR=zero, rest=zero.
	pixels := make([]byte, 3*3*4)
	pixels[0] = 255
	pixels[3] = 255

	out := rotatePixels180(pixels, 3, 3)
	// After 180°, TL is mapped to BR (2, 2).
	r, _, _, a := pixelRGBA(out, 3, 2, 2)
	if r != 255 || a != 255 {
		t.Errorf("rotate180: BR = (%d,...,%d), want (255,...,255)", r, a)
	}
	// TL should now be zero.
	r2, _, _, _ := pixelRGBA(out, 3, 0, 0)
	if r2 != 0 {
		t.Errorf("rotate180: TL red = %d, want 0", r2)
	}
}

// ---------------------------------------------------------------------------
// applyDiscreteTransformToLayer tests
// ---------------------------------------------------------------------------

func TestApplyDiscreteTransformToLayer_FlipH(t *testing.T) {
	pixels := make([]byte, 4*2*4)
	// Mark leftmost column.
	pixels[0], pixels[3] = 255, 255
	pixels[4*4+0], pixels[4*4+3] = 255, 255

	layer := &PixelLayer{
		Pixels: pixels,
		Bounds: LayerBounds{X: 10, Y: 20, W: 4, H: 2},
	}
	applyDiscreteTransformToLayer(layer, "flipH")

	if layer.Bounds.X != 10 || layer.Bounds.Y != 20 {
		t.Errorf("flipH changed bounds: %+v", layer.Bounds)
	}
	// Right column (x=3) should now be red.
	r, _, _, a := pixelRGBA(layer.Pixels, 4, 3, 0)
	if r != 255 || a != 255 {
		t.Errorf("flipH: right column = (%d,...,%d), want (255,...,255)", r, a)
	}
}

func TestApplyDiscreteTransformToLayer_Rotate90CW_PreservesCentre(t *testing.T) {
	// 4×2 layer centred at (10, 20) — centre is (12, 21).
	pixels := makeSolidPixels(4, 2, 50, 100, 150, 255)
	layer := &PixelLayer{
		Pixels: pixels,
		Bounds: LayerBounds{X: 10, Y: 20, W: 4, H: 2},
	}

	cx := layer.Bounds.X + layer.Bounds.W/2 // 12
	cy := layer.Bounds.Y + layer.Bounds.H/2 // 21

	applyDiscreteTransformToLayer(layer, "rotate90cw")

	// After rotate90cw dims become 2×4.
	if layer.Bounds.W != 2 || layer.Bounds.H != 4 {
		t.Fatalf("rotate90cw: dims = %dx%d, want 2×4", layer.Bounds.W, layer.Bounds.H)
	}
	// Centre should be preserved.
	newCX := layer.Bounds.X + layer.Bounds.W/2
	newCY := layer.Bounds.Y + layer.Bounds.H/2
	if newCX != cx || newCY != cy {
		t.Errorf("rotate90cw: centre moved from (%d,%d) to (%d,%d)", cx, cy, newCX, newCY)
	}
}

// ---------------------------------------------------------------------------
// FreeTransformState.meta tests
// ---------------------------------------------------------------------------

func TestFreeTransformMeta_Nil(t *testing.T) {
	var s *FreeTransformState
	if s.meta() != nil {
		t.Error("nil state.meta() should return nil")
	}
}

func TestFreeTransformMeta_Identity(t *testing.T) {
	pixels := makeSolidPixels(10, 10, 128, 128, 128, 255)
	s := identityState(10, 10, pixels)
	m := s.meta()
	if m == nil {
		t.Fatal("meta() returned nil for active state")
	}
	if !m.Active {
		t.Error("meta.Active should be true")
	}
	if math.Abs(m.Rotation) > 0.01 {
		t.Errorf("identity rotation = %f, want 0", m.Rotation)
	}
	if math.Abs(m.ScaleX-100) > 0.1 || math.Abs(m.ScaleY-100) > 0.1 {
		t.Errorf("identity scale = (%f, %f), want (100, 100)", m.ScaleX, m.ScaleY)
	}
}

func TestFreeTransformMeta_Rotation90(t *testing.T) {
	pixels := makeSolidPixels(10, 10, 0, 0, 0, 255)
	s := identityState(10, 10, pixels)
	// 90° CW rotation matrix.
	s.A = 0
	s.B = 1
	s.C = -1
	s.D = 0

	m := s.meta()
	if math.Abs(math.Abs(m.Rotation)-90) > 0.1 {
		t.Errorf("rotation = %f, want ±90", m.Rotation)
	}
}

// ---------------------------------------------------------------------------
// inverseTransformPoint tests
// ---------------------------------------------------------------------------

func TestInverseTransformPoint_Identity(t *testing.T) {
	s := &FreeTransformState{A: 1, B: 0, C: 0, D: 1, TX: 5, TY: 3}
	lx, ly, ok := s.inverseTransformPoint(15, 13)
	if !ok {
		t.Fatal("inverse of identity returned ok=false")
	}
	if math.Abs(lx-10) > 1e-9 || math.Abs(ly-10) > 1e-9 {
		t.Errorf("inverse: got (%f, %f), want (10, 10)", lx, ly)
	}
}

func TestInverseTransformPoint_Singular(t *testing.T) {
	s := &FreeTransformState{A: 0, B: 0, C: 0, D: 0}
	_, _, ok := s.inverseTransformPoint(1, 1)
	if ok {
		t.Error("singular matrix should return ok=false")
	}
}

// ---------------------------------------------------------------------------
// End-to-end free transform via engine commands
// ---------------------------------------------------------------------------

func TestFreeTransform_BeginCommit(t *testing.T) {
	h := Init("")
	defer Free(h)

	// Create a document with one pixel layer.
	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Test", Width: 20, Height: 20, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	}))
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Layer 1",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    makeSolidPixels(4, 4, 200, 100, 50, 255),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	// Begin free transform.
	result, err = DispatchCommand(h, commandBeginFreeTransform, mustJSON(t, BeginFreeTransformPayload{
		LayerID: layerID,
	}))
	if err != nil {
		t.Fatalf("begin free transform: %v", err)
	}
	if result.UIMeta.FreeTransform == nil || !result.UIMeta.FreeTransform.Active {
		t.Fatal("freeTransform should be active after BeginFreeTransform")
	}

	// Commit immediately (identity transform).
	result, err = DispatchCommand(h, commandCommitFreeTransform, `{}`)
	if err != nil {
		t.Fatalf("commit free transform: %v", err)
	}
	if result.UIMeta.FreeTransform != nil && result.UIMeta.FreeTransform.Active {
		t.Error("freeTransform should not be active after commit")
	}
}

func TestFreeTransform_Cancel(t *testing.T) {
	h := Init("")
	defer Free(h)

	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Test", Width: 20, Height: 20, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	}))
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Layer 1",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    makeSolidPixels(4, 4, 200, 100, 50, 255),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	_, err = DispatchCommand(h, commandBeginFreeTransform, mustJSON(t, BeginFreeTransformPayload{
		LayerID: layerID,
	}))
	if err != nil {
		t.Fatalf("begin free transform: %v", err)
	}

	result, err = DispatchCommand(h, commandCancelFreeTransform, `{}`)
	if err != nil {
		t.Fatalf("cancel free transform: %v", err)
	}
	if result.UIMeta.FreeTransform != nil && result.UIMeta.FreeTransform.Active {
		t.Error("freeTransform should not be active after cancel")
	}
}

func TestDiscreteTransform_FlipH(t *testing.T) {
	h := Init("")
	defer Free(h)

	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Test", Width: 10, Height: 10, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	}))
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}
	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "L",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    makeSolidPixels(4, 4, 100, 200, 150, 255),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	_, err = DispatchCommand(h, commandFlipLayerH, mustJSON(t, DiscreteTransformPayload{
		LayerID: layerID,
	}))
	if err != nil {
		t.Fatalf("flipH: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Floating-selection helpers unit tests
// ---------------------------------------------------------------------------

func TestExtractSelectionContent_Basic(t *testing.T) {
	// 4×4 layer, solid red.
	pixels := makeSolidPixels(4, 4, 255, 0, 0, 255)
	pl := &PixelLayer{
		layerBase: newLayerBase("L"),
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    pixels,
	}
	// Select the top-left 2×2 region.
	sel := newSelection(4, 4)
	for y := range 2 {
		for x := range 2 {
			sel.Mask[y*4+x] = 255
		}
	}

	floatPixels, floatBounds, ok := extractSelectionContent(pl, sel)
	if !ok {
		t.Fatal("expected content to be extracted")
	}
	if floatBounds.W != 2 || floatBounds.H != 2 {
		t.Fatalf("floatBounds = %v, want 2×2", floatBounds)
	}
	// All extracted pixels should be fully red.
	for i := 0; i < len(floatPixels); i += 4 {
		if floatPixels[i] != 255 || floatPixels[i+1] != 0 || floatPixels[i+2] != 0 || floatPixels[i+3] != 255 {
			t.Fatalf("pixel[%d] = %v, want red", i/4, floatPixels[i:i+4])
		}
	}
}

func TestExtractSelectionContent_NoOverlap(t *testing.T) {
	// 4×4 layer at origin; selection entirely outside layer bounds.
	pixels := makeSolidPixels(2, 2, 255, 0, 0, 255)
	pl := &PixelLayer{
		layerBase: newLayerBase("L"),
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    pixels,
	}
	sel := newSelection(4, 4)
	// Select only bottom-right corner, outside the 2×2 layer.
	sel.Mask[3*4+3] = 255

	_, _, ok := extractSelectionContent(pl, sel)
	if ok {
		t.Fatal("expected no content when selection doesn't overlap layer")
	}
}

func TestClearSelectionContent(t *testing.T) {
	pixels := makeSolidPixels(4, 4, 200, 100, 50, 255)
	pl := &PixelLayer{
		layerBase: newLayerBase("L"),
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    append([]byte(nil), pixels...),
	}
	// Select the entire layer.
	sel := newSelection(4, 4)
	for i := range sel.Mask {
		sel.Mask[i] = 255
	}

	clearSelectionContent(pl, sel)

	for i := 3; i < len(pl.Pixels); i += 4 {
		if pl.Pixels[i] != 0 {
			t.Fatalf("pixel alpha[%d] = %d, want 0", i/4, pl.Pixels[i])
		}
	}
}

func TestMergePixelLayerOnto_Basic(t *testing.T) {
	// dst: 4×4 solid green.
	dst := &PixelLayer{
		layerBase: newLayerBase("dst"),
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    makeSolidPixels(4, 4, 0, 255, 0, 255),
	}
	// src: 2×2 solid red, overlapping top-left of dst.
	srcPixels := makeSolidPixels(2, 2, 255, 0, 0, 255)
	srcBounds := LayerBounds{X: 0, Y: 0, W: 2, H: 2}

	mergePixelLayerOnto(dst, srcPixels, srcBounds)

	if dst.Bounds.W != 4 || dst.Bounds.H != 4 {
		t.Fatalf("dst bounds = %v, want 4×4", dst.Bounds)
	}
	// Top-left 2×2 should now be red (src-over fully opaque src).
	for y := range 2 {
		for x := range 2 {
			r, g, b, a := pixelRGBA(dst.Pixels, 4, x, y)
			if r != 255 || g != 0 || b != 0 || a != 255 {
				t.Errorf("pixel(%d,%d) = (%d,%d,%d,%d), want red", x, y, r, g, b, a)
			}
		}
	}
	// Bottom-right pixels should remain green.
	r, g, b, a := pixelRGBA(dst.Pixels, 4, 3, 3)
	if r != 0 || g != 255 || b != 0 || a != 255 {
		t.Errorf("pixel(3,3) = (%d,%d,%d,%d), want green", r, g, b, a)
	}
}

// ---------------------------------------------------------------------------
// Integration: floating-selection free transform
// ---------------------------------------------------------------------------

func TestFreeTransform_FloatingSelection_Commit(t *testing.T) {
	h := Init("")
	defer Free(h)

	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Test", Width: 8, Height: 8, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	}))
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}
	// Add a 4×4 solid red pixel layer at origin.
	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    makeSolidPixels(4, 4, 255, 0, 0, 255),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	// Select the left half (2×4).
	_, err = DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Mode:  SelectionCombineReplace,
		Rect:  LayerBounds{X: 0, Y: 0, W: 2, H: 4},
	}))
	if err != nil {
		t.Fatalf("create selection: %v", err)
	}

	// Begin free transform — should enter floating-selection mode.
	result, err = DispatchCommand(h, commandBeginFreeTransform, mustJSON(t, BeginFreeTransformPayload{
		LayerID: layerID,
	}))
	if err != nil {
		t.Fatalf("begin free transform: %v", err)
	}
	if result.UIMeta.FreeTransform == nil || !result.UIMeta.FreeTransform.Active {
		t.Fatal("freeTransform should be active")
	}
	// The active layer should now be the floating layer (different from the original).
	floatingID := result.UIMeta.ActiveLayerID
	if floatingID == layerID {
		t.Fatal("active layer should be the floating layer, not the original")
	}

	// Commit (identity transform — merges back in place).
	result, err = DispatchCommand(h, commandCommitFreeTransform, `{}`)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if result.UIMeta.FreeTransform != nil && result.UIMeta.FreeTransform.Active {
		t.Error("freeTransform should be inactive after commit")
	}
	// Selection should be cleared after commit.
	if result.UIMeta.Selection.Active {
		t.Error("selection should be cleared after commit")
	}
	// Active layer should be the original source layer.
	if result.UIMeta.ActiveLayerID != layerID {
		t.Errorf("active layer = %q, want %q", result.UIMeta.ActiveLayerID, layerID)
	}
	// Floating layer should be gone (only 1 non-background layer in doc).
	layers := result.UIMeta.Layers
	pixelLayers := 0
	for _, l := range layers {
		if l.LayerType == LayerTypePixel {
			pixelLayers++
		}
	}
	if pixelLayers != 1 {
		t.Errorf("pixel layer count = %d, want 1 after merge", pixelLayers)
	}
}

func TestFreeTransform_FloatingSelection_Cancel(t *testing.T) {
	h := Init("")
	defer Free(h)

	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Test", Width: 8, Height: 8, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	}))
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}
	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    makeSolidPixels(4, 4, 0, 200, 100, 255),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	_, err = DispatchCommand(h, commandNewSelection, mustJSON(t, CreateSelectionPayload{
		Shape: SelectionShapeRect,
		Mode:  SelectionCombineReplace,
		Rect:  LayerBounds{X: 0, Y: 0, W: 2, H: 2},
	}))
	if err != nil {
		t.Fatalf("create selection: %v", err)
	}

	_, err = DispatchCommand(h, commandBeginFreeTransform, mustJSON(t, BeginFreeTransformPayload{
		LayerID: layerID,
	}))
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Cancel — source layer should be restored, floating layer removed.
	result, err = DispatchCommand(h, commandCancelFreeTransform, `{}`)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if result.UIMeta.FreeTransform != nil && result.UIMeta.FreeTransform.Active {
		t.Error("freeTransform should be inactive after cancel")
	}
	if result.UIMeta.ActiveLayerID != layerID {
		t.Errorf("active layer = %q, want original %q", result.UIMeta.ActiveLayerID, layerID)
	}
	// Only the original pixel layer should remain.
	pixelLayers := 0
	for _, l := range result.UIMeta.Layers {
		if l.LayerType == LayerTypePixel {
			pixelLayers++
		}
	}
	if pixelLayers != 1 {
		t.Errorf("pixel layer count = %d, want 1 after cancel", pixelLayers)
	}
}
