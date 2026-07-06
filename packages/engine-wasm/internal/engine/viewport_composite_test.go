package engine

import "testing"

// buildRGBASurface builds a doc-sized straight-alpha RGBA surface from a slice of
// per-pixel [4]byte values in row-major order.
func buildRGBASurface(w, h int, pixels [][4]byte) []byte {
	surf := make([]byte, w*h*4)
	for i, p := range pixels {
		copy(surf[i*4:i*4+4], p[:])
	}
	return surf
}

// nearestReference composites the surface onto a fresh copy of the background
// using the same integer 1:1 mapping and per-pixel semantics the identity fast
// path is expected to reproduce (opaque copy / transparent skip / blend).
func nearestReference(bg []byte, canvasW, canvasH int, doc *Document, surf []byte, baseX, baseY int) []byte {
	out := make([]byte, len(bg))
	copy(out, bg)
	for cy := 0; cy < canvasH; cy++ {
		sy := cy + baseY
		if sy < 0 || sy >= doc.Height {
			continue
		}
		for cx := 0; cx < canvasW; cx++ {
			sx := cx + baseX
			if sx < 0 || sx >= doc.Width {
				continue
			}
			dst := (cy*canvasW + cx) * 4
			src := (sy*doc.Width + sx) * 4
			a := surf[src+3]
			if a == 0 {
				continue
			}
			if a == 255 {
				copy(out[dst:dst+4], surf[src:src+4])
			} else {
				compositePixelWithBlend(out[dst:dst+4], surf[src:src+4], BlendModeNormal, 1, pixelNoiseSeed(cx, cy))
			}
		}
	}
	return out
}

func filledCanvas(w, h int, c [4]byte) []byte {
	buf := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		copy(buf[i*4:i*4+4], c[:])
	}
	return buf
}

// Test a: identity fast path must equal the nearest-neighbour reference exactly.
func TestCompositeIdentityFastPathMatchesNearest(t *testing.T) {
	w, h := 3, 3
	doc := &Document{Width: w, Height: h}
	// Mixed alpha: opaque, transparent, and semi-transparent pixels.
	surf := buildRGBASurface(w, h, [][4]byte{
		{255, 0, 0, 255},
		{0, 255, 0, 0},
		{0, 0, 255, 128},
		{10, 20, 30, 255},
		{40, 50, 60, 200},
		{0, 0, 0, 0},
		{200, 100, 50, 255},
		{5, 5, 5, 64},
		{255, 255, 255, 255},
	})
	bg := [4]byte{30, 40, 50, 255}

	vp := &ViewportState{
		CenterX:  float64(w) / 2, // 1.5, with halfCanvasW 1.5 → offset 0 (aligned)
		CenterY:  float64(h) / 2,
		Zoom:     1.0,
		Rotation: 0,
	}

	canvas := filledCanvas(w, h, bg)
	compositeDocumentToViewport(canvas, w, h, doc, vp, surf)

	want := nearestReference(filledCanvas(w, h, bg), w, h, doc, surf, 0, 0)

	for i := range canvas {
		if canvas[i] != want[i] {
			t.Fatalf("identity fast path mismatch at byte %d (pixel %d, chan %d): got %d want %d",
				i, i/4, i%4, canvas[i], want[i])
		}
	}
}

// Test b: alpha-weighted bilinear must not let a transparent black neighbour
// darken an opaque red edge. Sampling exactly halfway between an opaque red
// pixel and a fully transparent (0,0,0,0) pixel must yield full red at ~50%
// alpha, i.e. the composite over white keeps the red channel saturated.
func TestBilinearAlphaWeightingNoDarkFringe(t *testing.T) {
	// Doc 2x1: red opaque, transparent black.
	w, h := 2, 1
	doc := &Document{Width: w, Height: h}
	surf := buildRGBASurface(w, h, [][4]byte{
		{255, 0, 0, 255},
		{0, 0, 0, 0},
	})

	// Canvas 1x1, zoom 2. Chosen so the single canvas pixel samples docX=0.5,
	// docY=0 — exactly between the two doc pixels.
	//   docX = (0 + 0.5 - 0.5)/2 + CenterX - 0.5 = CenterX - 0.5
	// CenterX = 1 → docX = 0.5. CenterY = 0.5 → docY = 0.
	canvasW, canvasH := 1, 1
	vp := &ViewportState{CenterX: 1, CenterY: 0.5, Zoom: 2, Rotation: 0}

	white := [4]byte{255, 255, 255, 255}
	canvas := filledCanvas(canvasW, canvasH, white)
	compositeDocumentToViewport(canvas, canvasW, canvasH, doc, vp, surf)

	// Expected: interpolated src = (255,0,0,~128) → composited over white gives
	// red channel 255 (no darkening), alpha 255, green ~127.
	if canvas[0] < 254 {
		t.Fatalf("red channel darkened by transparent neighbour: got %d, want >= 254 (full red)", canvas[0])
	}
	if canvas[3] != 255 {
		t.Fatalf("alpha over opaque white must be 255, got %d", canvas[3])
	}
	// Green should be roughly the 50%% blend of white(255) and red-src green(0).
	if canvas[1] < 118 || canvas[1] > 138 {
		t.Fatalf("green channel = %d, want ~127 (±10)", canvas[1])
	}

	// Compare against the OLD (buggy) straight-RGB interpolation, which would
	// have produced src red ~128 and thus a dimmer composited red (~191).
	oldRed := oldBuggyCompositedRed()
	if int(canvas[0]) <= oldRed {
		t.Fatalf("alpha-weighted red (%d) must exceed old straight-RGB red (%d)", canvas[0], oldRed)
	}
}

// oldBuggyCompositedRed reproduces the pre-fix straight-RGB interpolation for
// the Test b scenario: src red = round(255*0.5)=128 with alpha 128, composited
// over opaque white.
func oldBuggyCompositedRed() int {
	src := []byte{128, 0, 0, 128}
	dst := []byte{255, 255, 255, 255}
	compositePixelWithBlend(dst, src, BlendModeNormal, 1, 0)
	return int(dst[0])
}

// Test c: at zoom==1 with a subpixel pan the fast path must NOT be taken; the
// result must differ from the integer-aligned identity output and match bilinear.
func TestSubpixelPanUsesBilinearNotFastPath(t *testing.T) {
	w, h := 4, 1
	doc := &Document{Width: w, Height: h}
	// Sharp black→white edge so any interpolation is visible.
	surf := buildRGBASurface(w, h, [][4]byte{
		{0, 0, 0, 255},
		{255, 255, 255, 255},
		{0, 0, 0, 255},
		{255, 255, 255, 255},
	})

	bg := [4]byte{0, 0, 0, 255}

	// Integer-aligned reference (fast path): CenterX = w/2 = 2, halfCanvasW = 2.
	aligned := filledCanvas(w, h, bg)
	compositeDocumentToViewport(aligned, w, h, doc,
		&ViewportState{CenterX: float64(w) / 2, CenterY: 0.5, Zoom: 1, Rotation: 0}, surf)

	// Subpixel pan by 0.3 doc-pixels: offset frac 0.3 → not integer aligned.
	panned := filledCanvas(w, h, bg)
	compositeDocumentToViewport(panned, w, h, doc,
		&ViewportState{CenterX: float64(w)/2 + 0.3, CenterY: 0.5, Zoom: 1, Rotation: 0}, surf)

	if bytesEqual(aligned, panned) {
		t.Fatalf("subpixel-panned output equals integer-aligned output; fast path was wrongly taken")
	}

	// The panned result must contain at least one intermediate (blended) grey
	// value that pure nearest/identity (only 0 or 255) would never produce.
	foundIntermediate := false
	for i := 0; i < len(panned); i += 4 {
		v := panned[i]
		if v > 5 && v < 250 {
			foundIntermediate = true
			break
		}
	}
	if !foundIntermediate {
		t.Fatalf("subpixel pan produced no interpolated values; expected bilinear blending")
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BenchmarkCompositeViewportZoom1 compares the identity fast path against the
// bilinear resample at zoom==1 over a full canvas.
func BenchmarkCompositeViewportZoom1(b *testing.B) {
	const w, h = 512, 512
	doc := &Document{Width: w, Height: h}
	surf := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		surf[i*4] = byte(i)
		surf[i*4+1] = byte(i >> 3)
		surf[i*4+2] = byte(i >> 6)
		surf[i*4+3] = 255
	}
	canvas := make([]byte, w*h*4)

	b.Run("identity-aligned", func(b *testing.B) {
		vp := &ViewportState{CenterX: float64(w) / 2, CenterY: float64(h) / 2, Zoom: 1, Rotation: 0}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			compositeDocumentToViewport(canvas, w, h, doc, vp, surf)
		}
	})

	b.Run("bilinear-subpixel", func(b *testing.B) {
		vp := &ViewportState{CenterX: float64(w)/2 + 0.5, CenterY: float64(h) / 2, Zoom: 1, Rotation: 0}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			compositeDocumentToViewport(canvas, w, h, doc, vp, surf)
		}
	})
}
