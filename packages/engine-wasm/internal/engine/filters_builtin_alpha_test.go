package engine

import (
	"encoding/json"
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// S.5 regression tests: non-Gaussian filters must process the alpha channel
// (premultiply → filter → unpremultiply) instead of averaging RGB only and
// copying the original alpha. See PLAN.md Phase S.5.
// ---------------------------------------------------------------------------

// makeAlphaTestImage builds a 16x16 image that is fully transparent black
// everywhere except an opaque grey (200,200,200,255) square at x,y ∈ [4,8).
func makeAlphaTestImage() (pixels []byte, w, h int) {
	w, h = 16, 16
	pixels = make([]byte, w*h*4)
	for y := 4; y < 8; y++ {
		for x := 4; x < 8; x++ {
			i := (y*w + x) * 4
			pixels[i] = 200
			pixels[i+1] = 200
			pixels[i+2] = 200
			pixels[i+3] = 255
		}
	}
	return pixels, w, h
}

// assertNoDarkHalo checks that every sufficiently visible pixel kept the
// visible colour (200 grey): with alpha-weighted (premultiplied) filtering,
// transparent black pixels must not darken RGB of visible pixels.
func assertNoDarkHalo(t *testing.T, pixels []byte, w, h int) {
	t.Helper()
	const minAlpha = 32
	const tol = 16
	for i := 0; i < len(pixels); i += 4 {
		a := pixels[i+3]
		if a < minAlpha {
			continue
		}
		for c := 0; c < 3; c++ {
			v := int(pixels[i+c])
			if v < 200-tol || v > 200+tol {
				x := (i / 4) % w
				y := (i / 4) / w
				t.Fatalf("dark halo at (%d,%d): channel %d = %d with alpha %d, want ~200 (alpha-weighted filtering)", x, y, c, v, a)
			}
		}
	}
}

// assertAlphaProcessed checks that the filter changed the alpha channel
// somewhere (i.e. alpha was filtered/displaced instead of copied verbatim).
func assertAlphaProcessed(t *testing.T, before, after []byte) {
	t.Helper()
	for i := 3; i < len(after); i += 4 {
		if after[i] != before[i] {
			return
		}
	}
	t.Fatal("alpha channel is unchanged everywhere — filter copied original alpha instead of processing it")
}

func TestBlurAndNoiseFiltersProcessAlpha(t *testing.T) {
	tests := []struct {
		name   string
		fn     FilterFunc
		params map[string]any
	}{
		{"gaussian-blur", filterGaussianBlur, map[string]any{"radius": 2}},
		{"box-blur", filterBoxBlur, map[string]any{"radius": 2}},
		{"motion-blur", filterMotionBlur, map[string]any{"angle": 0, "distance": 3}},
		{"radial-blur-spin", filterRadialBlur, map[string]any{"type": "spin", "amount": 80, "quality": 2}},
		{"radial-blur-zoom", filterRadialBlur, map[string]any{"type": "zoom", "amount": 80, "quality": 2}},
		{"surface-blur", filterSurfaceBlur, map[string]any{"radius": 2, "threshold": 255}},
		{"median", filterMedian, map[string]any{"radius": 1}},
		{"minimum", filterMinimum, map[string]any{"radius": 1}},
		{"maximum", filterMaximum, map[string]any{"radius": 1}},
		{"reduce-noise", filterReduceNoise, map[string]any{"strength": 3, "preserve_details": 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixels, w, h := makeAlphaTestImage()
			before := append([]byte(nil), pixels...)
			params, _ := json.Marshal(tt.params)
			if err := tt.fn(pixels, w, h, nil, params); err != nil {
				t.Fatal(err)
			}
			assertNoDarkHalo(t, pixels, w, h)
			assertAlphaProcessed(t, before, pixels)
		})
	}
}

func TestDistortFiltersDisplaceAlpha(t *testing.T) {
	tests := []struct {
		name   string
		fn     FilterFunc
		params map[string]any
	}{
		{"ripple", filterRipple, map[string]any{"amount": 3, "size": "small"}},
		{"twirl", filterTwirl, map[string]any{"angle": 180}},
		{"offset", filterOffset, map[string]any{"horizontal": 3, "vertical": 2, "wrap": "wrap"}},
		{"polar-coordinates", filterPolarCoordinates, map[string]any{"mode": "rectangular-to-polar"}},
		{"lens-correction", filterLensCorrection, map[string]any{"distortion": 60.0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixels, w, h := makeAlphaTestImage()
			before := append([]byte(nil), pixels...)
			params, _ := json.Marshal(tt.params)
			if err := tt.fn(pixels, w, h, nil, params); err != nil {
				t.Fatal(err)
			}
			assertNoDarkHalo(t, pixels, w, h)
			assertAlphaProcessed(t, before, pixels)
		})
	}
}

// Gaussian blur on a fully opaque image must not change alpha or introduce
// colour shifts (premultiply/unpremultiply round trip is the identity there).
func TestFilterGaussianBlurOpaqueImageKeepsAlphaOpaque(t *testing.T) {
	w, h := 8, 8
	pixels := makeGradientPixels(w, h)
	params, _ := json.Marshal(map[string]any{"radius": 2})
	if err := filterGaussianBlur(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] != 255 {
			t.Fatalf("alpha at pixel %d changed to %d, want 255", i/4, pixels[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Feathered-selection regression: the per-pixel mask blend must happen in
// premultiplied space. Lerping straight R,G,B,A independently darkens colour
// wherever the original and filtered alpha differ (e.g. transparent original
// vs semi-transparent filtered pixel under a feathered selection).
// ---------------------------------------------------------------------------

func TestApplyFilteredRGBAWithMaskFeatheredSelectionBlendsPremultiplied(t *testing.T) {
	// Original pixel: fully transparent black. Filtered result: (200,200,200,90).
	// Feathered selection at 128 must yield RGB ~200 at roughly half the
	// filtered alpha — not (100,100,100,45).
	pixels := []byte{0, 0, 0, 0}
	selMask := []byte{128}
	applyFilteredRGBAWithMask(pixels, selMask, func(int) (byte, byte, byte, byte) {
		return 200, 200, 200, 90
	})

	if a := pixels[3]; a < 43 || a > 47 {
		t.Errorf("alpha = %d, want ~45 (90 * 128/255)", a)
	}
	for c := 0; c < 3; c++ {
		if v := pixels[c]; v < 195 || v > 205 {
			t.Errorf("channel %d = %d, want ~200 — straight-space mask blend darkens colour", c, v)
		}
	}
}

func TestApplyFilteredRGBAWithMaskFeatheredOpaqueOperandsLerpNormally(t *testing.T) {
	// When both operands are opaque, the premultiplied blend must reduce to a
	// plain lerp (backwards compatible with the straight-space behaviour).
	pixels := []byte{0, 0, 0, 255}
	selMask := []byte{128}
	applyFilteredRGBAWithMask(pixels, selMask, func(int) (byte, byte, byte, byte) {
		return 200, 200, 200, 255
	})
	want := blendByte(0, 200, 128)
	for c := 0; c < 3; c++ {
		if v := pixels[c]; abs8diff(v, want) > 1 {
			t.Errorf("channel %d = %d, want ~%d (plain lerp for opaque operands)", c, v, want)
		}
	}
	if pixels[3] != 255 {
		t.Errorf("alpha = %d, want 255", pixels[3])
	}
}

func TestFilterGaussianBlurFeatheredSelectionNoDarkFringe(t *testing.T) {
	pixels, w, h := makeAlphaTestImage()
	selMask := make([]byte, w*h)
	for i := range selMask {
		selMask[i] = 128
	}
	params, _ := json.Marshal(map[string]any{"radius": 2})
	if err := filterGaussianBlur(pixels, w, h, selMask, params); err != nil {
		t.Fatal(err)
	}
	assertNoDarkHalo(t, pixels, w, h)
}

// ---------------------------------------------------------------------------
// S.5 regression tests: Add Noise seed handling. A hard-coded fixed seed makes
// every application replay the identical noise stream, so reapplying doubles
// the exact same pattern (2× amount, perfectly correlated) instead of adding
// independent noise.
// ---------------------------------------------------------------------------

func TestFilterAddNoiseReapplyIsNotCorrelated(t *testing.T) {
	w, h := 32, 32
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 128
		pixels[i+1] = 128
		pixels[i+2] = 128
		pixels[i+3] = 255
	}
	params, _ := json.Marshal(map[string]any{"amount": 20, "distribution": "uniform", "monochromatic": false})

	if err := filterAddNoise(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}
	afterFirst := append([]byte(nil), pixels...)
	if err := filterAddNoise(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// delta1 = noise added by the first application, delta2 by the second.
	// With a hard-coded seed both deltas are byte-identical (correlated
	// doubling). Independent applications must differ somewhere.
	for i := 0; i < len(pixels); i += 4 {
		for c := 0; c < 3; c++ {
			d1 := int(afterFirst[i+c]) - 128
			d2 := int(pixels[i+c]) - int(afterFirst[i+c])
			if d1 != d2 {
				return // patterns diverge — noise is not correlated
			}
		}
	}
	t.Fatal("second Add Noise application added the byte-identical noise pattern — reapply doubles instead of adding independent noise")
}

func TestFilterAddNoiseExplicitSeedIsDeterministic(t *testing.T) {
	mk := func() []byte {
		pixels := make([]byte, 16*16*4)
		for i := 0; i < len(pixels); i += 4 {
			pixels[i] = 128
			pixels[i+1] = 128
			pixels[i+2] = 128
			pixels[i+3] = 255
		}
		return pixels
	}
	params, _ := json.Marshal(map[string]any{"amount": 20, "distribution": "uniform", "seed": 1234})

	a := mk()
	b := mk()
	if err := filterAddNoise(a, 16, 16, nil, params); err != nil {
		t.Fatal(err)
	}
	if err := filterAddNoise(b, 16, 16, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("explicit seed must be deterministic: byte %d differs (%d vs %d)", i, a[i], b[i])
		}
	}

	// A different explicit seed must produce a different pattern.
	params2, _ := json.Marshal(map[string]any{"amount": 20, "distribution": "uniform", "seed": 5678})
	c := mk()
	if err := filterAddNoise(c, 16, 16, nil, params2); err != nil {
		t.Fatal(err)
	}
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different explicit seeds produced identical noise")
	}
}

// ---------------------------------------------------------------------------
// S.5 regression test: Fade with dissolve blend mode. The fade loop passed the
// flat byte index as the dissolve noise seed; normalized against 2^32 that is
// ~0 for every pixel, so *every* pixel dissolved to the filtered result
// regardless of opacity. Fade must use the same per-pixel noise convention as
// layer compositing (pixelNoiseSeed of document coordinates).
// ---------------------------------------------------------------------------

func TestFadeFilterDissolveMixesOriginalAndFiltered(t *testing.T) {
	registerTestInvertFilter(t, "fade-dissolve-invert")

	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// 16x16 opaque red layer at the origin.
	redPixels := make([]byte, 16*16*4)
	for i := 0; i < len(redPixels); i += 4 {
		redPixels[i] = 255
		redPixels[i+3] = 255
	}
	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "red-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 16, H: 16},
		Pixels:    redPixels,
	})); err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Apply invert: red → cyan.
	if _, err := DispatchCommand(h, commandApplyFilter, mustJSON(t, ApplyFilterPayload{
		FilterID: "fade-dissolve-invert",
	})); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Fade at 75% opacity with dissolve: some pixels must remain original.
	if _, err := DispatchCommand(h, commandFadeFilter, mustJSON(t, FadeFilterPayload{
		Opacity:   75,
		BlendMode: BlendModeDissolve,
	})); err != nil {
		t.Fatalf("fade: %v", err)
	}

	pl := getActivePixelLayer(t, h)
	filtered, original, other := 0, 0, 0
	for i := 0; i < len(pl.Pixels); i += 4 {
		r, g, b := pl.Pixels[i], pl.Pixels[i+1], pl.Pixels[i+2]
		switch {
		case r == 0 && g == 255 && b == 255:
			filtered++
		case r == 255 && g == 0 && b == 0:
			original++
		default:
			other++
		}
	}
	if other != 0 {
		t.Errorf("dissolve fade must be binary per pixel, found %d partially blended pixels", other)
	}
	if filtered == 0 {
		t.Error("dissolve fade at 75% left no filtered pixels")
	}
	if original == 0 {
		t.Errorf("dissolve fade at 75%% dissolved every pixel (filtered=%d) — noise seed is degenerate", filtered)
	}
}

// ---------------------------------------------------------------------------
// S.5 regression test: marshalFilterParams must return an error instead of
// panicking on unmarshalable values (e.g. NaN floats).
// ---------------------------------------------------------------------------

func TestMarshalFilterParamsInvalidValueReturnsError(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("marshalFilterParams panicked: %v", r)
		}
	}()
	got, err := marshalFilterParams(math.NaN())
	if err == nil {
		t.Fatalf("expected error for NaN params, got %q", string(got))
	}
}

func TestMarshalFilterParamsValidValue(t *testing.T) {
	got, err := marshalFilterParams(boxBlurParams{Radius: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var p boxBlurParams
	if err := json.Unmarshal(got, &p); err != nil {
		t.Fatalf("round trip failed: %v", err)
	}
	if p.Radius != 3 {
		t.Fatalf("radius = %d, want 3", p.Radius)
	}
}
