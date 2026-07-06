package engine

// Phase S.6 Batch V3 regression + behavior tests for the layer-style
// parameters brought to life in this batch: gradient stops, pattern IDs,
// contour, noise, knockout, altitude, and technique.
//
// The pin*V3 hash constants below were captured from the PRE-V3 code
// (commit b60fd50, before any Batch V3 change) with a throwaway harness that
// rendered the exact scenarios reproduced by these tests. They pin the
// backward-compat contract: with default parameter values (empty stops,
// empty patternId, linear contour, noise 0, knockout false, altitude 30,
// technique smooth) the V3 code must be byte-identical to the old output.
//
// Deliberate exception: satin's DEFAULT contour is "gaussian", so wiring the
// contour up changes satin's default output for blurred (non-binary) masks.
// See TestBuildSatinSurface_DefaultContourGaussianRebaseline.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"testing"

	agglib "github.com/cwbudde/agg_go"
)

const (
	pinV3GradientOverlayDefault = "c68f692d7d76801498f38f824348b85468bf6c2e616dc9f76ee4656a731c2708"
	pinV3GradientOverlayAngled  = "53a55bff6a6b2c988a9200bdd96c76c2f2765aefd625f4b379564f26fb50aae3"
	pinV3StrokeGradientDefault  = "00780b230d467b453b9efa86831d3c49aa4f7c3dbb5b6f780f40cfba0fe8a850"
	pinV3PatternOverlayDefault  = "4321b344c745f02d7115561ce7f0e585910b30b0676540f65615b5a2841cbdb1"
	pinV3PatternOverlayScale2   = "32af5b7b1a7af16e9300dbce6f7d99178a6e774766045f04cb90abb4e0d65a4f"
	pinV3StrokePatternDefault   = "c76dda4b8d8b1ede79f4e1d968188bc2c7f1dc11833eb634f7e5357620920cff"
	pinV3DropShadowDefault      = "4018c7821a14a67e60af66d18682cb5207d1e27bf85bbe2f460ca743efe48b3c"
	pinV3InnerShadowDefault     = "e752b5a0ec08b176af3b746ff53273694bcef23b8b324f31a7d315d4b60bb61f"
	pinV3OuterGlowDefault       = "e01a8853a573e8a5f4f704df81198e25df51bac515bf3537556bd3cbdbeeeaae"
	pinV3InnerGlowDefault       = "acf073106b9b7feede8f97614ab4126b9a141d6d30c3025eafe27f069f3b8b36"
	pinV3BevelSmoothHighlight   = "4fb80a91702b92c5114c37f52b6bcd4c4da053f26ce482fb5a498621eb2c90aa"
	pinV3BevelSmoothShadow      = "cef23de84485b8a12139f46cbaef5a5f2836a88dacc739a1744583d4b8d2e1bc"
	pinV3BevelDefaultHighlight  = "c3ad23c1e7a08d67ff1254479558134b76ceb60f1161f3251eb88d9f76c7a92c"
	pinV3BevelDefaultShadow     = "eee45959712bf9974839b72d4acc8d1d9698104372c25af307d6357443def98e"
	// Satin with an explicit "linear" contour must still match the pre-V3
	// output (contour was previously ignored, i.e. effectively linear).
	pinV3SatinLinear = "4673117bc3e426959bdddcdc5efb1bb3b2a9a72ccaf3ad566fed9bca062e1673"
	// Pre-V3 output of satin's DEFAULT params (contour "gaussian" decoded but
	// dead). The V3 default output must DIFFER from this hash — deliberate
	// rebaseline, see TestBuildSatinSurface_DefaultContourGaussianRebaseline.
	pinV3SatinDefaultPreV3 = "4673117bc3e426959bdddcdc5efb1bb3b2a9a72ccaf3ad566fed9bca062e1673"
)

// v3StyleTestSource reproduces the capture-harness geometry: a 16x16
// document surface with an opaque white 8x8 square at (4,4).
func v3StyleTestSource() ([]byte, int, int) {
	const docW, docH = 16, 16
	surface := make([]byte, docW*docH*4)
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			i := (y*docW + x) * 4
			surface[i] = 255
			surface[i+1] = 255
			surface[i+2] = 255
			surface[i+3] = 255
		}
	}
	return surface, docW, docH
}

func hashSurface(surface []byte) string {
	sum := sha256.Sum256(surface)
	return hex.EncodeToString(sum[:])
}

func surfaceAlphaAt(surface []byte, width, x, y int) uint8 {
	return surface[(y*width+x)*4+3]
}

// --- A. Gradient overlay / gradient stroke with real stops ---------------

func TestBuildGradientOverlaySurface_EmptyStopsByteIdenticalLegacyPin(t *testing.T) {
	src, w, h := v3StyleTestSource()

	tests := []struct {
		name   string
		params json.RawMessage
		pin    string
	}{
		{"default params", json.RawMessage(`{}`), pinV3GradientOverlayDefault},
		{"angled reversed unaligned", json.RawMessage(`{"angle":37,"scale":1.5,"reverse":true,"align":false}`), pinV3GradientOverlayAngled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := decodeGradientOverlayParams(test.params)
			got := buildGradientOverlaySurface(src, w, h, params)
			if hash := hashSurface(got); hash != test.pin {
				t.Fatalf("empty-stops gradient overlay hash = %s, want pre-V3 pin %s", hash, test.pin)
			}
			// Live pin: the empty-stops path must be the exact legacy agg
			// expression, regardless of Align.
			mask := agglib.AlphaMaskFromRGBA(src, w, h)
			want := agglib.RenderMaskedLinearGradientRGBA(mask, gradientFill(params))
			if !bytes.Equal(got, want) {
				t.Fatal("empty-stops gradient overlay diverged from the legacy agg render path")
			}
		})
	}
}

func gradientTestStops() []GradientStopPayload {
	return []GradientStopPayload{
		{Position: 0, Color: [4]uint8{255, 0, 0, 255}},
		{Position: 0.5, Color: [4]uint8{0, 255, 0, 255}},
		{Position: 1, Color: [4]uint8{0, 0, 255, 255}},
	}
}

// horizontalStripSource builds a 16x16 surface with one opaque full-width
// row at y=8 so the align bbox spans x 0..15 and projections are exact.
func horizontalStripSource() ([]byte, int, int) {
	const docW, docH = 16, 16
	surface := make([]byte, docW*docH*4)
	for x := 0; x < docW; x++ {
		i := (8*docW + x) * 4
		surface[i], surface[i+1], surface[i+2], surface[i+3] = 255, 255, 255, 255
	}
	return surface, docW, docH
}

func TestBuildGradientOverlaySurface_ThreeStopExactLUTProjection(t *testing.T) {
	src, w, h := horizontalStripSource()
	params := GradientOverlayParams{Opacity: 1, Angle: 0, Scale: 1, Align: true, Stops: gradientTestStops()}
	surface := buildGradientOverlaySurface(src, w, h, params)

	lut := buildGradientLUT(params.Stops, gradientRampColor(0), gradientRampColor(1))
	// bbox x 0..15, y 8..8; center (7.5, 8); span 16; angle 0.
	// t(x) = (x + 0.5) / 16, LUT index = round(t*255).
	checks := []struct{ x, index int }{{0, 8}, {7, 120}, {15, 247}}
	for _, check := range checks {
		want := lut[check.index]
		got := rgbaAt(surface, w, check.x, 8)
		if got != ([4]uint8{want.R, want.G, want.B, want.A}) {
			t.Fatalf("pixel (%d,8) = %v, want exact LUT[%d] = %v", check.x, got, check.index, want)
		}
	}
}

func TestBuildGradientOverlaySurface_ReverseFlipsRamp(t *testing.T) {
	src, w, h := horizontalStripSource()
	forward := GradientOverlayParams{Opacity: 1, Angle: 0, Scale: 1, Align: true, Stops: gradientTestStops()}
	reversed := forward
	reversed.Reverse = true

	forwardSurface := buildGradientOverlaySurface(src, w, h, forward)
	reversedSurface := buildGradientOverlaySurface(src, w, h, reversed)
	if bytes.Equal(forwardSurface, reversedSurface) {
		t.Fatal("reverse=true must change the gradient output")
	}

	lut := buildGradientLUT(forward.Stops, gradientRampColor(0), gradientRampColor(1))
	// x=0 projects to t=0.03125; reversed samples t=0.96875 -> LUT[247].
	want := lut[247]
	if got := rgbaAt(reversedSurface, w, 0, 8); got != ([4]uint8{want.R, want.G, want.B, want.A}) {
		t.Fatalf("reversed pixel (0,8) = %v, want LUT[247] = %v", got, want)
	}
}

func TestBuildGradientOverlaySurface_AlignAnchorsToMaskBounds(t *testing.T) {
	// Off-center small square so aligned and document-spanning gradients
	// project differently.
	const docW, docH = 16, 16
	src := make([]byte, docW*docH*4)
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			i := (y*docW + x) * 4
			src[i], src[i+1], src[i+2], src[i+3] = 255, 255, 255, 255
		}
	}

	aligned := GradientOverlayParams{Opacity: 1, Angle: 0, Scale: 1, Align: true, Stops: gradientTestStops()}
	fullDoc := aligned
	fullDoc.Align = false

	alignedSurface := buildGradientOverlaySurface(src, docW, docH, aligned)
	fullDocSurface := buildGradientOverlaySurface(src, docW, docH, fullDoc)
	if bytes.Equal(alignedSurface, fullDocSurface) {
		t.Fatal("align=true and align=false must differ for an off-center shape")
	}

	// Aligned: bbox 2..5 both axes; span 4; t(x) = (x-1.5)/4.
	lut := buildGradientLUT(aligned.Stops, gradientRampColor(0), gradientRampColor(1))
	checks := []struct{ x, index int }{{2, 32}, {5, 223}}
	for _, check := range checks {
		want := lut[check.index]
		if got := rgbaAt(alignedSurface, docW, check.x, 3); got != ([4]uint8{want.R, want.G, want.B, want.A}) {
			t.Fatalf("aligned pixel (%d,3) = %v, want LUT[%d] = %v", check.x, got, check.index, want)
		}
	}
}

func TestBuildStrokeSurface_GradientStopsAndLegacyPin(t *testing.T) {
	src, w, h := v3StyleTestSource()

	legacyParams := decodeStrokeParams(json.RawMessage(`{"size":2,"position":"outside","fillType":"gradient"}`))
	legacy := buildStrokeSurface(src, w, h, legacyParams, styleRenderContext{})
	if hash := hashSurface(legacy); hash != pinV3StrokeGradientDefault {
		t.Fatalf("empty-stops gradient stroke hash = %s, want pre-V3 pin %s", hash, pinV3StrokeGradientDefault)
	}
	mask := agglib.AlphaMaskFromRGBA(src, w, h)
	strokeMask := strokeMaskFromAlpha(mask, legacyParams.Size, legacyParams.Position)
	want := agglib.RenderMaskedLinearGradientRGBA(strokeMask, gradientFill(GradientOverlayParams{Scale: 1}))
	if !bytes.Equal(legacy, want) {
		t.Fatal("empty-stops gradient stroke diverged from the legacy agg render path")
	}

	stopParams := legacyParams
	stopParams.Stops = gradientTestStops()
	stopParams.GradientAngle = 0
	withStops := buildStrokeSurface(src, w, h, stopParams, styleRenderContext{})
	if bytes.Equal(withStops, legacy) {
		t.Fatal("gradient stroke with explicit stops must differ from the legacy ramp")
	}
	// Interior of the shape stays empty: the gradient fills the stroke ring only.
	if got := surfaceAlphaAt(withStops, w, 8, 8); got != 0 {
		t.Fatalf("stroke interior alpha = %d, want 0", got)
	}

	rotated := stopParams
	rotated.GradientAngle = 90
	if bytes.Equal(buildStrokeSurface(src, w, h, rotated, styleRenderContext{}), withStops) {
		t.Fatal("stroke gradientAngle must rotate the gradient")
	}
}

// --- B. Pattern overlay / pattern stroke ----------------------------------

func TestBuildPatternOverlaySurface_ResolvedTileAndLegacyPin(t *testing.T) {
	src, w, h := v3StyleTestSource()
	ctx := documentStyleContext(nil)

	// Empty and unknown pattern IDs keep the legacy checker byte-identical.
	for name, raw := range map[string]json.RawMessage{
		"empty id":   json.RawMessage(`{}`),
		"unknown id": json.RawMessage(`{"patternId":"nope/missing"}`),
	} {
		params := decodePatternOverlayParams(raw)
		got := buildPatternOverlaySurface(src, w, h, params, ctx)
		if hash := hashSurface(got); hash != pinV3PatternOverlayDefault {
			t.Fatalf("%s pattern overlay hash = %s, want pre-V3 pin %s", name, hash, pinV3PatternOverlayDefault)
		}
		mask := agglib.AlphaMaskFromRGBA(src, w, h)
		want := agglib.RenderMaskedCheckerPatternRGBA(mask, checkerPatternFill(params.Scale))
		if !bytes.Equal(got, want) {
			t.Fatalf("%s pattern overlay diverged from the legacy checker path", name)
		}
	}
	scale2 := decodePatternOverlayParams(json.RawMessage(`{"scale":2}`))
	if hash := hashSurface(buildPatternOverlaySurface(src, w, h, scale2, ctx)); hash != pinV3PatternOverlayScale2 {
		t.Fatalf("scale-2 legacy pattern overlay hash = %s, want pre-V3 pin %s", hash, pinV3PatternOverlayScale2)
	}

	// Resolved builtin tile: exact doc-space tile phase at scale 1 and 2.
	stripes := resolvePattern(nil, "builtin/stripes")
	if stripes == nil {
		t.Fatal("builtin/stripes must resolve")
	}
	for _, scale := range []float64{1, 2} {
		params := PatternOverlayParams{Opacity: 1, Scale: scale, PatternID: "builtin/stripes"}
		surface := buildPatternOverlaySurface(src, w, h, params, ctx)
		for y := 4; y < 12; y++ {
			for x := 4; x < 12; x++ {
				want := samplePatternColor(stripes, x, y, scale)
				if got := rgbaAt(surface, w, x, y); got != want {
					t.Fatalf("scale %v pattern pixel (%d,%d) = %v, want doc-space tile sample %v", scale, x, y, got, want)
				}
			}
		}
		// Outside the mask nothing is painted.
		if got := surfaceAlphaAt(surface, w, 0, 0); got != 0 {
			t.Fatalf("pattern outside mask alpha = %d, want 0", got)
		}
	}

	one := buildPatternOverlaySurface(src, w, h, PatternOverlayParams{Opacity: 1, Scale: 1, PatternID: "builtin/stripes"}, ctx)
	two := buildPatternOverlaySurface(src, w, h, PatternOverlayParams{Opacity: 1, Scale: 2, PatternID: "builtin/stripes"}, ctx)
	if bytes.Equal(one, two) {
		t.Fatal("pattern scale must change the tile phase")
	}
}

func TestBuildStrokeSurface_PatternTileAndLegacyPin(t *testing.T) {
	src, w, h := v3StyleTestSource()
	ctx := documentStyleContext(nil)

	legacyParams := decodeStrokeParams(json.RawMessage(`{"size":2,"position":"outside","fillType":"pattern"}`))
	legacy := buildStrokeSurface(src, w, h, legacyParams, ctx)
	if hash := hashSurface(legacy); hash != pinV3StrokePatternDefault {
		t.Fatalf("empty-patternId pattern stroke hash = %s, want pre-V3 pin %s", hash, pinV3StrokePatternDefault)
	}

	dots := resolvePattern(nil, "builtin/dots")
	if dots == nil {
		t.Fatal("builtin/dots must resolve")
	}
	params := legacyParams
	params.PatternID = "builtin/dots"
	surface := buildStrokeSurface(src, w, h, params, ctx)
	if bytes.Equal(surface, legacy) {
		t.Fatal("pattern stroke with a resolved tile must differ from the legacy checker")
	}

	mask := agglib.AlphaMaskFromRGBA(src, w, h)
	strokeMask := strokeMaskFromAlpha(mask, params.Size, params.Position)
	found := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			coverage := strokeMask.At(x, y)
			got := rgbaAt(surface, w, x, y)
			if coverage == 0 {
				if got[3] != 0 {
					t.Fatalf("pattern stroke painted outside the stroke mask at (%d,%d): %v", x, y, got)
				}
				continue
			}
			found = true
			tile := samplePatternColor(dots, x, y, 1)
			want := [4]uint8{tile[0], tile[1], tile[2], scaleMaskedAlpha(tile[3], coverage)}
			if got != want {
				t.Fatalf("pattern stroke pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
	if !found {
		t.Fatal("stroke mask unexpectedly empty")
	}
}

// --- C.1 Contour ----------------------------------------------------------

func TestContourLUT_Shapes(t *testing.T) {
	linear := contourLUT("linear")
	for i := range linear {
		if linear[i] != uint8(i) {
			t.Fatalf("linear contour LUT[%d] = %d, want identity (regression pin)", i, linear[i])
		}
	}
	unknown := contourLUT("wobbly")
	if unknown != linear {
		t.Fatal("unknown contour names must fall back to the identity table")
	}

	gaussian := contourLUT("gaussian")
	if gaussian[0] != 0 || gaussian[255] != 255 {
		t.Fatalf("gaussian endpoints = %d,%d, want 0,255", gaussian[0], gaussian[255])
	}
	if gaussian[64] >= 64 || gaussian[191] <= 191 {
		t.Fatalf("gaussian LUT[64]=%d LUT[191]=%d, want S-curve below/above identity", gaussian[64], gaussian[191])
	}

	cone := contourLUT("cone")
	if cone[0] != 0 || cone[255] != 0 || cone[128] < 250 {
		t.Fatalf("cone LUT endpoints/peak = %d,%d,%d, want triangle peaking mid-band", cone[0], cone[128], cone[255])
	}

	rolling := contourLUT("rolling-slope")
	if rolling[0] != 0 || rolling[255] != 255 || rolling[64] <= 64 {
		t.Fatalf("rolling-slope LUT = %d,%d,%d, want quarter-circle above identity", rolling[0], rolling[64], rolling[255])
	}

	steps := contourLUT("rounded-steps")
	if steps[0] != 0 || steps[255] != 255 {
		t.Fatalf("rounded-steps endpoints = %d,%d, want 0,255", steps[0], steps[255])
	}
	for i := 1; i < 256; i++ {
		if steps[i] < steps[i-1] {
			t.Fatalf("rounded-steps LUT must be monotonic, dips at %d", i)
		}
	}
}

func TestBuildBevelEmbossSurfaces_ContourLinearPinAndConeReshapesBand(t *testing.T) {
	src, w, h := v3StyleTestSource()

	linearParams := decodeBevelEmbossParams(json.RawMessage(`{"style":"inner-bevel","technique":"smooth","depth":1,"direction":"up","size":3,"soften":1,"angle":0,"altitude":30,"contour":"linear"}`))
	highlight, shadow := buildBevelEmbossSurfaces(src, w, h, linearParams)
	if hash := hashSurface(highlight); hash != pinV3BevelSmoothHighlight {
		t.Fatalf("linear-contour bevel highlight hash = %s, want pre-V3 pin %s", hash, pinV3BevelSmoothHighlight)
	}
	if hash := hashSurface(shadow); hash != pinV3BevelSmoothShadow {
		t.Fatalf("linear-contour bevel shadow hash = %s, want pre-V3 pin %s", hash, pinV3BevelSmoothShadow)
	}

	// The blurred band contains intermediate alphas, so the cone contour
	// (peak mid-band, zero at full coverage) must reshape it.
	coneParams := linearParams
	coneParams.Soften = 2
	coneParams.Contour = "cone"
	linearSoft := linearParams
	linearSoft.Soften = 2
	coneHighlight, _ := buildBevelEmbossSurfaces(src, w, h, coneParams)
	linearHighlight, _ := buildBevelEmbossSurfaces(src, w, h, linearSoft)
	if bytes.Equal(coneHighlight, linearHighlight) {
		t.Fatal("cone contour must change the blurred bevel band")
	}

	// Cone maps full coverage to zero: the strongest linear-band pixel must
	// lose alpha under the cone contour.
	maxIndex, maxAlpha := -1, uint8(0)
	for i := 3; i < len(linearHighlight); i += 4 {
		if linearHighlight[i] > maxAlpha {
			maxAlpha = linearHighlight[i]
			maxIndex = i
		}
	}
	if maxIndex < 0 || maxAlpha == 0 {
		t.Fatal("expected a nonzero blurred bevel band")
	}
	if coneHighlight[maxIndex] >= linearHighlight[maxIndex] {
		t.Fatalf("cone contour alpha at band peak = %d, want < linear %d", coneHighlight[maxIndex], linearHighlight[maxIndex])
	}
}

func TestBuildSatinSurface_DefaultContourGaussianRebaseline(t *testing.T) {
	src, w, h := v3StyleTestSource()

	// Pin: an explicit linear contour reproduces the pre-V3 satin output
	// byte-identically (the contour param used to be ignored).
	linearParams := decodeSatinParams(json.RawMessage(`{"angle":19,"distance":2,"size":2,"contour":"linear"}`))
	linear := buildSatinSurface(src, w, h, linearParams)
	if hash := hashSurface(linear); hash != pinV3SatinLinear {
		t.Fatalf("linear-contour satin hash = %s, want pre-V3 pin %s", hash, pinV3SatinLinear)
	}

	// Deliberate rebaseline: satin's DEFAULT contour is "gaussian", so the
	// default output changes now that contours are honored. The new default
	// must differ from the pre-V3 hash and stay deterministic.
	defaultParams := decodeSatinParams(json.RawMessage(`{"angle":19,"distance":2,"size":2}`))
	if defaultParams.Contour != "gaussian" {
		t.Fatalf("satin default contour = %q, want gaussian", defaultParams.Contour)
	}
	gaussianSurface := buildSatinSurface(src, w, h, defaultParams)
	if hash := hashSurface(gaussianSurface); hash == pinV3SatinDefaultPreV3 {
		t.Fatal("satin default output still matches the pre-V3 hash; the gaussian contour rebaseline did not take effect")
	}
	if !bytes.Equal(gaussianSurface, buildSatinSurface(src, w, h, defaultParams)) {
		t.Fatal("satin default output must be deterministic")
	}
	if bytes.Equal(gaussianSurface, linear) {
		t.Fatal("gaussian and linear satin contours must differ for blurred masks")
	}
}

// --- C.2 Noise -------------------------------------------------------------

func TestShadowAndGlowNoise_ZeroPinAndDeterministicGrain(t *testing.T) {
	src, w, h := v3StyleTestSource()

	dropZero := buildDropShadowSurface(src, w, h, decodeDropShadowParams(json.RawMessage(`{"distance":3,"size":2,"angle":120}`)))
	if hash := hashSurface(dropZero); hash != pinV3DropShadowDefault {
		t.Fatalf("noise-0 drop shadow hash = %s, want pre-V3 pin %s", hash, pinV3DropShadowDefault)
	}
	if hash := hashSurface(buildInnerShadowSurface(src, w, h, decodeInnerShadowParams(json.RawMessage(`{"distance":2,"size":2,"angle":120}`)))); hash != pinV3InnerShadowDefault {
		t.Fatalf("noise-0 inner shadow hash = %s, want pre-V3 pin %s", hash, pinV3InnerShadowDefault)
	}
	if hash := hashSurface(buildOuterGlowSurface(src, w, h, decodeOuterGlowParams(json.RawMessage(`{"spread":0.5,"size":2}`)))); hash != pinV3OuterGlowDefault {
		t.Fatalf("noise-0 outer glow hash = %s, want pre-V3 pin %s", hash, pinV3OuterGlowDefault)
	}
	if hash := hashSurface(buildInnerGlowSurface(src, w, h, decodeInnerGlowParams(json.RawMessage(`{"spread":0.5,"size":2}`)))); hash != pinV3InnerGlowDefault {
		t.Fatalf("noise-0 inner glow hash = %s, want pre-V3 pin %s", hash, pinV3InnerGlowDefault)
	}

	noisyParams := decodeDropShadowParams(json.RawMessage(`{"distance":3,"size":2,"angle":120,"noise":0.5}`))
	noisyA := buildDropShadowSurface(src, w, h, noisyParams)
	noisyB := buildDropShadowSurface(src, w, h, noisyParams)
	if !bytes.Equal(noisyA, noisyB) {
		t.Fatal("noise must be deterministic: two renders diverged")
	}
	if bytes.Equal(noisyA, dropZero) {
		t.Fatal("noise 0.5 must change the drop shadow output")
	}

	for name, render := range map[string]func(noise string) []byte{
		"inner shadow": func(noise string) []byte {
			return buildInnerShadowSurface(src, w, h, decodeInnerShadowParams(json.RawMessage(`{"distance":2,"size":2,"angle":120,"noise":`+noise+`}`)))
		},
		"outer glow": func(noise string) []byte {
			return buildOuterGlowSurface(src, w, h, decodeOuterGlowParams(json.RawMessage(`{"spread":0.5,"size":2,"noise":`+noise+`}`)))
		},
		"inner glow": func(noise string) []byte {
			return buildInnerGlowSurface(src, w, h, decodeInnerGlowParams(json.RawMessage(`{"spread":0.5,"size":2,"noise":`+noise+`}`)))
		},
	} {
		if bytes.Equal(render("0"), render("0.5")) {
			t.Fatalf("%s: noise 0.5 must change the output", name)
		}
	}
}

func TestApplyNoiseToMask_FormulaAndDocSpaceSeed(t *testing.T) {
	mask := agglib.NewAlphaMask(8, 8)
	mask.Fill(255)

	if got := applyNoiseToMask(mask, 0); !bytes.Equal(got.Pix, mask.Pix) {
		t.Fatal("noise 0 must leave the mask untouched (regression pin)")
	}

	noisy := applyNoiseToMask(mask, 0.5)
	x, y := 3, 7
	u := float64(pixelNoiseSeed(x, y)) / (1 << 32)
	want := uint8(math.Round(255 * (1 - 0.5*u)))
	if got := noisy.Pix[y*noisy.Width+x]; got != want {
		t.Fatalf("noise alpha at (%d,%d) = %d, want %d (doc-space splitmix64 seed)", x, y, got, want)
	}
}

// --- C.3 Knockout -----------------------------------------------------------

func TestBuildDropShadowSurface_KnockoutSubtractsSourceAlpha(t *testing.T) {
	src, w, h := v3StyleTestSource()

	base := json.RawMessage(`{"distance":3,"angle":120,"size":0}`)
	off := buildDropShadowSurface(src, w, h, decodeDropShadowParams(base))
	explicitOff := buildDropShadowSurface(src, w, h, decodeDropShadowParams(json.RawMessage(`{"distance":3,"angle":120,"size":0,"knockout":false}`)))
	if !bytes.Equal(off, explicitOff) {
		t.Fatal("knockout=false must equal the field-absent default (regression pin)")
	}

	on := buildDropShadowSurface(src, w, h, decodeDropShadowParams(json.RawMessage(`{"distance":3,"angle":120,"size":0,"knockout":true}`)))
	if bytes.Equal(on, off) {
		t.Fatal("knockout=true must change the drop shadow output")
	}

	// Shadow offset for angle 120, distance 3 is (1, 3) — cos(120 deg)
	// computes to -0.4999..., so dx rounds to 1. The shifted square covers
	// x 5..12, y 7..14. Pixel (8,8) is under fully opaque content, so
	// knockout removes it; (12,14) is pure shadow and must be unaffected.
	if got := surfaceAlphaAt(on, w, 8, 8); got != 0 {
		t.Fatalf("knockout shadow alpha under opaque content = %d, want 0", got)
	}
	if surfaceAlphaAt(off, w, 8, 8) == 0 {
		t.Fatal("precondition failed: non-knockout shadow should cover (8,8)")
	}
	if got, want := surfaceAlphaAt(on, w, 12, 14), surfaceAlphaAt(off, w, 12, 14); got != want || got == 0 {
		t.Fatalf("pure-shadow pixel alpha with knockout = %d, want unchanged %d", got, want)
	}
}

// --- C.4 Altitude ------------------------------------------------------------

func TestBevelAltitude_ScalesDirectionalOffset(t *testing.T) {
	if factor := bevelAltitudeFactor(30); factor != 1 {
		t.Fatalf("bevelAltitudeFactor(30) = %v, want exactly 1 (regression pin)", factor)
	}
	if bevelAltitudeFactor(0) <= bevelAltitudeFactor(30) {
		t.Fatal("altitude 0 must produce a stronger offset than 30")
	}

	src, w, h := v3StyleTestSource()
	makeParams := func(altitude, size float64) BevelEmbossParams {
		params := defaultBevelEmbossParams()
		params.Angle = 0
		params.Size = size
		params.Altitude = altitude
		return params
	}

	// Altitude 90: overhead light, zero offset, no min-1px fallback — the
	// bevel flattens to nothing.
	highlight90, shadow90 := buildBevelEmbossSurfaces(src, w, h, makeParams(90, 3))
	for i := 3; i < len(highlight90); i += 4 {
		if highlight90[i] != 0 || shadow90[i] != 0 {
			t.Fatalf("altitude 90 bevel must be flat, found alpha at byte %d", i)
		}
	}

	countAlpha := func(surface []byte) int {
		count := 0
		for i := 3; i < len(surface); i += 4 {
			if surface[i] > 0 {
				count++
			}
		}
		return count
	}
	highlight30, _ := buildBevelEmbossSurfaces(src, w, h, makeParams(30, 4))
	highlight0, _ := buildBevelEmbossSurfaces(src, w, h, makeParams(0, 4))
	if countAlpha(highlight0) <= countAlpha(highlight30) {
		t.Fatalf("altitude 0 highlight coverage %d must exceed altitude 30 coverage %d", countAlpha(highlight0), countAlpha(highlight30))
	}
}

// --- C.5 Technique -----------------------------------------------------------

func TestBuildBevelEmbossSurfaces_TechniqueSmoothPinAndChiselRamp(t *testing.T) {
	src, w, h := v3StyleTestSource()

	// Smooth pin: default decode (technique smooth) is byte-identical to the
	// pre-V3 output.
	highlight, shadow := buildBevelEmbossSurfaces(src, w, h, decodeBevelEmbossParams(json.RawMessage(`{}`)))
	if hash := hashSurface(highlight); hash != pinV3BevelDefaultHighlight {
		t.Fatalf("default bevel highlight hash = %s, want pre-V3 pin %s", hash, pinV3BevelDefaultHighlight)
	}
	if hash := hashSurface(shadow); hash != pinV3BevelDefaultShadow {
		t.Fatalf("default bevel shadow hash = %s, want pre-V3 pin %s", hash, pinV3BevelDefaultShadow)
	}

	chiselHard := decodeBevelEmbossParams(json.RawMessage(`{"style":"inner-bevel","technique":"chisel-hard","size":3,"soften":0,"angle":0,"altitude":30,"contour":"linear"}`))
	hardHighlight, _ := buildBevelEmbossSurfaces(src, w, h, chiselHard)

	// Angle 0, size 3 puts the highlight in the right three columns of the
	// square (x 9..11). The distance ramp must fall off monotonically toward
	// the edge with the exact averaged-erosion plateau values.
	wantRamp := map[int]uint8{9: 255, 10: 170, 11: 85}
	for x, want := range wantRamp {
		if got := surfaceAlphaAt(hardHighlight, w, x, 7); got != want {
			t.Fatalf("chisel-hard ramp alpha at (%d,7) = %d, want %d", x, got, want)
		}
	}

	// Chisel-hard ignores Soften entirely.
	withSoften := chiselHard
	withSoften.Soften = 5
	softenedHighlight, _ := buildBevelEmbossSurfaces(src, w, h, withSoften)
	if !bytes.Equal(hardHighlight, softenedHighlight) {
		t.Fatal("chisel-hard must skip the Soften blur")
	}

	// Chisel-soft blurs the ramp: between hard and smooth.
	chiselSoft := chiselHard
	chiselSoft.Technique = "chisel-soft"
	softHighlight, _ := buildBevelEmbossSurfaces(src, w, h, chiselSoft)
	smooth := chiselHard
	smooth.Technique = "smooth"
	smoothHighlight, _ := buildBevelEmbossSurfaces(src, w, h, smooth)
	if bytes.Equal(softHighlight, hardHighlight) || bytes.Equal(softHighlight, smoothHighlight) {
		t.Fatal("chisel-soft must differ from both chisel-hard and smooth")
	}
	if bytes.Equal(hardHighlight, smoothHighlight) {
		t.Fatal("chisel-hard must differ from smooth")
	}
}

// --- Decode ------------------------------------------------------------------

func TestDecodeLayerStyles_V3ParamNormalization(t *testing.T) {
	gradient := decodeGradientOverlayParams(json.RawMessage(`{"stops":[{"position":-0.5,"color":[1,2,3,4]},{"position":1.5,"color":[5,6,7,8]}]}`))
	if len(gradient.Stops) != 2 || gradient.Stops[0].Position != 0 || gradient.Stops[1].Position != 1 {
		t.Fatalf("gradient overlay stops = %+v, want positions clamped to [0,1]", gradient.Stops)
	}
	if gradient.Stops[0].Color != ([4]uint8{1, 2, 3, 4}) {
		t.Fatalf("gradient overlay stop color = %v, want passthrough", gradient.Stops[0].Color)
	}

	stroke := decodeStrokeParams(json.RawMessage(`{"fillType":"gradient","gradientAngle":45,"patternId":"builtin/dots","stops":[{"position":2,"color":[9,9,9,9]}]}`))
	if stroke.GradientAngle != 45 || stroke.PatternID != "builtin/dots" {
		t.Fatalf("stroke decode = %+v, want gradientAngle/patternId passthrough", stroke)
	}
	if len(stroke.Stops) != 1 || stroke.Stops[0].Position != 1 {
		t.Fatalf("stroke stops = %+v, want position clamped to 1", stroke.Stops)
	}

	pattern := decodePatternOverlayParams(json.RawMessage(`{"patternId":"builtin/noise"}`))
	if pattern.PatternID != "builtin/noise" {
		t.Fatalf("pattern overlay patternId = %q, want passthrough", pattern.PatternID)
	}

	if got := decodeBevelEmbossParams(json.RawMessage(`{"altitude":-10}`)).Altitude; got != 0 {
		t.Fatalf("altitude -10 decoded to %v, want clamp to 0", got)
	}
	if got := decodeBevelEmbossParams(json.RawMessage(`{"altitude":120}`)).Altitude; got != 90 {
		t.Fatalf("altitude 120 decoded to %v, want clamp to 90", got)
	}

	bevel := decodeBevelEmbossParams(json.RawMessage(`{"technique":"rough","contour":"stairs"}`))
	if bevel.Technique != "smooth" || bevel.Contour != "linear" {
		t.Fatalf("bevel enums = %+v, want unknown technique/contour to fall back to defaults", bevel)
	}

	if got := decodeBevelEmbossParams(json.RawMessage(`{"size":10000}`)).Size; got != 250 {
		t.Fatalf("bevel size 10000 decoded to %v, want clamp to 250 (chisel cost is O(size) full-mask erosions)", got)
	}
	if got := decodeSatinParams(json.RawMessage(`{"contour":"zigzag"}`)).Contour; got != "gaussian" {
		t.Fatalf("satin unknown contour decoded to %q, want gaussian default", got)
	}
}
