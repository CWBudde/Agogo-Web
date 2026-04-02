package engine

import (
	"encoding/json"
	"testing"
)

func TestFilterInvert(t *testing.T) {
	// 4 pixels: red, green (half-alpha), black, white
	pixels := []byte{
		255, 0, 0, 255, // red, full alpha
		0, 255, 0, 128, // green, half alpha
		0, 0, 0, 255, // black
		255, 255, 255, 255, // white
	}

	err := filterInvert(pixels, 4, 1, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// red → cyan (0,255,255), alpha unchanged
	if pixels[0] != 0 || pixels[1] != 255 || pixels[2] != 255 || pixels[3] != 255 {
		t.Errorf("red→cyan: got %v", pixels[0:4])
	}
	// green → magenta (255,0,255), alpha unchanged at 128
	if pixels[4] != 255 || pixels[5] != 0 || pixels[6] != 255 || pixels[7] != 128 {
		t.Errorf("green→magenta: got %v", pixels[4:8])
	}
	// black → white
	if pixels[8] != 255 || pixels[9] != 255 || pixels[10] != 255 || pixels[11] != 255 {
		t.Errorf("black→white: got %v", pixels[8:12])
	}
	// white → black
	if pixels[12] != 0 || pixels[13] != 0 || pixels[14] != 0 || pixels[15] != 255 {
		t.Errorf("white→black: got %v", pixels[12:16])
	}
}

func TestFilterInvertWithSelectionMask(t *testing.T) {
	// 2 pixels: both red
	pixels := []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	}
	// First pixel fully selected, second not selected
	selMask := []byte{255, 0}

	err := filterInvert(pixels, 2, 1, selMask, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First pixel inverted: red → cyan
	if pixels[0] != 0 || pixels[1] != 255 || pixels[2] != 255 || pixels[3] != 255 {
		t.Errorf("selected pixel: expected cyan, got %v", pixels[0:4])
	}
	// Second pixel unchanged: still red
	if pixels[4] != 255 || pixels[5] != 0 || pixels[6] != 0 || pixels[7] != 255 {
		t.Errorf("unselected pixel: expected red, got %v", pixels[4:8])
	}
}

func TestFilterInvertPartialSelection(t *testing.T) {
	// 1 pixel: white (255,255,255)
	pixels := []byte{255, 255, 255, 255}
	// 50% selection
	selMask := []byte{128}

	err := filterInvert(pixels, 1, 1, selMask, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inverted target is (0,0,0). Blending white with black at 128/255 ≈ 50%.
	// blendByte(255, 0, 128) = (255*127 + 0*128 + 127) / 255 ≈ 127
	expected := blendByte(255, 0, 128)
	if pixels[0] != expected || pixels[1] != expected || pixels[2] != expected {
		t.Errorf("partial selection: expected ~%d, got %v", expected, pixels[0:3])
	}
	// Alpha unchanged
	if pixels[3] != 255 {
		t.Errorf("alpha should be unchanged, got %d", pixels[3])
	}
}

// ---------------------------------------------------------------------------
// Gaussian Blur
// ---------------------------------------------------------------------------

func TestFilterGaussianBlur(t *testing.T) {
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for i := 3; i < len(pixels); i += 4 {
		pixels[i] = 255
	}
	// Single white pixel at center.
	idx := (2*w + 2) * 4
	pixels[idx] = 255
	pixels[idx+1] = 255
	pixels[idx+2] = 255

	params, _ := json.Marshal(map[string]any{"radius": 1})
	if err := filterGaussianBlur(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	if pixels[idx] == 255 && pixels[idx+1] == 255 && pixels[idx+2] == 255 {
		t.Error("center pixel should have been spread by blur")
	}
	nIdx := (2*w + 3) * 4
	if pixels[nIdx] == 0 && pixels[nIdx+1] == 0 && pixels[nIdx+2] == 0 {
		t.Error("neighbour should have received blur spread")
	}
}

func TestFilterGaussianBlurZeroRadiusIsNoop(t *testing.T) {
	pixels := []byte{255, 0, 0, 255, 0, 255, 0, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"radius": 0})
	if err := filterGaussianBlur(pixels, 2, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with radius 0", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Brightness / Contrast
// ---------------------------------------------------------------------------

func TestFilterBrightnessContrast(t *testing.T) {
	tests := []struct {
		name       string
		brightness int
		contrast   int
		input      byte
		wantR      byte
		tolerance  int
	}{
		{"brightness +50", 50, 0, 100, 150, 1},
		{"brightness -50", -50, 0, 100, 50, 1},
		{"brightness clamps high", 200, 0, 200, 255, 0},
		{"brightness clamps low", -200, 0, 50, 0, 0},
		{"no change", 0, 0, 100, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixels := []byte{tt.input, tt.input, tt.input, 255}
			params, _ := json.Marshal(map[string]any{
				"brightness": tt.brightness,
				"contrast":   tt.contrast,
			})
			if err := filterBrightnessContrast(pixels, 1, 1, nil, params); err != nil {
				t.Fatal(err)
			}
			diff := int(pixels[0]) - int(tt.wantR)
			if diff < -tt.tolerance || diff > tt.tolerance {
				t.Errorf("R = %d, want ~%d (tolerance %d)", pixels[0], tt.wantR, tt.tolerance)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Unsharp Mask
// ---------------------------------------------------------------------------

func TestFilterUnsharpMask(t *testing.T) {
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if x < 3 {
				pixels[i], pixels[i+1], pixels[i+2] = 50, 50, 50
			} else {
				pixels[i], pixels[i+1], pixels[i+2] = 200, 200, 200
			}
			pixels[i+3] = 255
		}
	}
	orig := append([]byte(nil), pixels...)

	params, _ := json.Marshal(map[string]any{"amount": 100, "radius": 1, "threshold": 0})
	if err := filterUnsharpMask(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// Check pixel right at the edge boundary (column 2, which borders 3).
	darkIdx := (2*w + 2) * 4   // last dark column
	brightIdx := (2*w + 3) * 4 // first bright column
	if pixels[darkIdx] >= orig[darkIdx] {
		t.Errorf("dark edge pixel should be darker: was %d, now %d", orig[darkIdx], pixels[darkIdx])
	}
	if pixels[brightIdx] <= orig[brightIdx] {
		t.Errorf("bright edge pixel should be brighter: was %d, now %d", orig[brightIdx], pixels[brightIdx])
	}
}

// ---------------------------------------------------------------------------
// Add Noise
// ---------------------------------------------------------------------------

func TestFilterAddNoise(t *testing.T) {
	w, h := 100, 100
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 128
		pixels[i+1] = 128
		pixels[i+2] = 128
		pixels[i+3] = 255
	}
	orig := append([]byte(nil), pixels...)

	params, _ := json.Marshal(map[string]any{"amount": 25, "distribution": "gaussian", "monochromatic": false})
	if err := filterAddNoise(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	changed := 0
	for i := 0; i < len(pixels); i += 4 {
		if pixels[i] != orig[i] || pixels[i+1] != orig[i+1] || pixels[i+2] != orig[i+2] {
			changed++
		}
	}
	if changed < 100 {
		t.Errorf("expected many changed pixels, got %d", changed)
	}
}

func TestFilterAddNoiseMonochromatic(t *testing.T) {
	pixels := []byte{128, 128, 128, 255}
	params, _ := json.Marshal(map[string]any{"amount": 50, "distribution": "uniform", "monochromatic": true})
	if err := filterAddNoise(pixels, 1, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	dr := int(pixels[0]) - 128
	dg := int(pixels[1]) - 128
	db := int(pixels[2]) - 128
	if dr != dg || dg != db {
		t.Errorf("monochromatic noise should shift all channels equally: dr=%d dg=%d db=%d", dr, dg, db)
	}
}

// ---------------------------------------------------------------------------
// High Pass
// ---------------------------------------------------------------------------

func TestFilterHighPass(t *testing.T) {
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 100
		pixels[i+1] = 100
		pixels[i+2] = 100
		pixels[i+3] = 255
	}

	params, _ := json.Marshal(map[string]any{"radius": 2})
	if err := filterHighPass(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	idx := (2*w + 2) * 4
	diff := int(pixels[idx]) - 128
	if diff < -2 || diff > 2 {
		t.Errorf("uniform area should be ~128, got %d", pixels[idx])
	}
}

// ---------------------------------------------------------------------------
// Emboss
// ---------------------------------------------------------------------------

func TestFilterEmboss(t *testing.T) {
	w, h := 3, 3
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 50
		pixels[i+1] = 50
		pixels[i+2] = 50
		pixels[i+3] = 255
	}
	idx := (1*w + 1) * 4
	pixels[idx] = 200
	pixels[idx+1] = 200
	pixels[idx+2] = 200

	params, _ := json.Marshal(map[string]any{"angle": 135, "height": 1, "amount": 100})
	if err := filterEmboss(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	allSame := true
	first := pixels[0]
	for i := 4; i < len(pixels); i += 4 {
		if pixels[i] != first {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("emboss should produce varying pixel values at edges")
	}
}

// ---------------------------------------------------------------------------
// Solarize
// ---------------------------------------------------------------------------

func TestFilterSolarize(t *testing.T) {
	pixels := []byte{
		200, 200, 200, 255,
		50, 50, 50, 255,
	}
	if err := filterSolarize(pixels, 2, 1, nil, nil); err != nil {
		t.Fatal(err)
	}
	if pixels[0] != 55 {
		t.Errorf("R = %d, want 55 (255-200)", pixels[0])
	}
	if pixels[4] != 50 {
		t.Errorf("R = %d, want 50 (unchanged)", pixels[4])
	}
}

// ---------------------------------------------------------------------------
// Find Edges
// ---------------------------------------------------------------------------

func TestFilterFindEdges(t *testing.T) {
	w, h := 3, 3
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 128
		pixels[i+1] = 128
		pixels[i+2] = 128
		pixels[i+3] = 255
	}

	if err := filterFindEdges(pixels, w, h, nil, nil); err != nil {
		t.Fatal(err)
	}

	idx := (1*w + 1) * 4
	if pixels[idx] > 10 {
		t.Errorf("uniform area edge = %d, want ~0", pixels[idx])
	}
}

// ---------------------------------------------------------------------------
// Box Blur
// ---------------------------------------------------------------------------

func TestFilterBoxBlur(t *testing.T) {
	// Single white pixel on black background should spread.
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for i := 3; i < len(pixels); i += 4 {
		pixels[i] = 255
	}
	idx := (2*w + 2) * 4
	pixels[idx] = 255
	pixels[idx+1] = 255
	pixels[idx+2] = 255

	params, _ := json.Marshal(map[string]any{"radius": 1})
	if err := filterBoxBlur(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// Center pixel should no longer be pure white.
	if pixels[idx] == 255 && pixels[idx+1] == 255 && pixels[idx+2] == 255 {
		t.Error("center pixel should have been averaged down")
	}
	// Neighbour should have received some spread.
	nIdx := (2*w + 3) * 4
	if pixels[nIdx] == 0 && pixels[nIdx+1] == 0 && pixels[nIdx+2] == 0 {
		t.Error("neighbour should have received blur spread")
	}
}

func TestFilterBoxBlurZeroRadiusIsNoop(t *testing.T) {
	pixels := []byte{255, 0, 0, 255, 0, 255, 0, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"radius": 0})
	if err := filterBoxBlur(pixels, 2, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with radius 0", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Sharpen
// ---------------------------------------------------------------------------

func TestFilterSharpen(t *testing.T) {
	// Create a 5x5 image with a dark/bright boundary at column 3.
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if x < 3 {
				pixels[i], pixels[i+1], pixels[i+2] = 50, 50, 50
			} else {
				pixels[i], pixels[i+1], pixels[i+2] = 200, 200, 200
			}
			pixels[i+3] = 255
		}
	}
	orig := append([]byte(nil), pixels...)

	if err := filterSharpen(pixels, w, h, nil, nil); err != nil {
		t.Fatal(err)
	}

	// Dark side of edge should be darker or equal.
	darkIdx := (2*w + 2) * 4
	if pixels[darkIdx] > orig[darkIdx] {
		t.Errorf("dark edge pixel got brighter: was %d, now %d", orig[darkIdx], pixels[darkIdx])
	}
	// Bright side of edge should be brighter or equal.
	brightIdx := (2*w + 3) * 4
	if pixels[brightIdx] < orig[brightIdx] {
		t.Errorf("bright edge pixel got darker: was %d, now %d", orig[brightIdx], pixels[brightIdx])
	}
}

// ---------------------------------------------------------------------------
// Sharpen More
// ---------------------------------------------------------------------------

func TestFilterSharpenMore(t *testing.T) {
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	pixelsSharpen := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if x < 3 {
				pixels[i], pixels[i+1], pixels[i+2] = 50, 50, 50
			} else {
				pixels[i], pixels[i+1], pixels[i+2] = 200, 200, 200
			}
			pixels[i+3] = 255
		}
	}
	copy(pixelsSharpen, pixels)
	orig := append([]byte(nil), pixels...)

	_ = filterSharpen(pixelsSharpen, w, h, nil, nil)
	if err := filterSharpenMore(pixels, w, h, nil, nil); err != nil {
		t.Fatal(err)
	}

	// Sharpen More should produce a stronger effect than Sharpen.
	darkIdx := (2*w + 2) * 4
	brightIdx := (2*w + 3) * 4

	sharpenDark := int(orig[darkIdx]) - int(pixelsSharpen[darkIdx])
	moreSharpDark := int(orig[darkIdx]) - int(pixels[darkIdx])
	if moreSharpDark < sharpenDark {
		t.Errorf("sharpen-more should be stronger: sharpen dark diff=%d, more=%d", sharpenDark, moreSharpDark)
	}

	sharpenBright := int(pixelsSharpen[brightIdx]) - int(orig[brightIdx])
	moreSharpBright := int(pixels[brightIdx]) - int(orig[brightIdx])
	if moreSharpBright < sharpenBright {
		t.Errorf("sharpen-more should be stronger: sharpen bright diff=%d, more=%d", sharpenBright, moreSharpBright)
	}
}

// ---------------------------------------------------------------------------
// Median
// ---------------------------------------------------------------------------

func TestFilterMedian(t *testing.T) {
	// 5x5 uniform grey with one salt pixel.
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 100
		pixels[i+1] = 100
		pixels[i+2] = 100
		pixels[i+3] = 255
	}
	// Salt pixel at center.
	idx := (2*w + 2) * 4
	pixels[idx] = 255
	pixels[idx+1] = 255
	pixels[idx+2] = 255

	params, _ := json.Marshal(map[string]any{"radius": 1})
	if err := filterMedian(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// The outlier should be replaced by the median of its neighbours (100).
	if pixels[idx] != 100 {
		t.Errorf("median should remove salt pixel: got R=%d, want 100", pixels[idx])
	}
}

func TestFilterMedianZeroRadiusIsNoop(t *testing.T) {
	pixels := []byte{255, 0, 0, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"radius": 0})
	if err := filterMedian(pixels, 1, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with radius 0", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Despeckle
// ---------------------------------------------------------------------------

func TestFilterDespeckle(t *testing.T) {
	// Same setup as median r=1: salt pixel should be removed.
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 100
		pixels[i+1] = 100
		pixels[i+2] = 100
		pixels[i+3] = 255
	}
	idx := (2*w + 2) * 4
	pixels[idx] = 255
	pixels[idx+1] = 255
	pixels[idx+2] = 255

	if err := filterDespeckle(pixels, w, h, nil, nil); err != nil {
		t.Fatal(err)
	}

	if pixels[idx] != 100 {
		t.Errorf("despeckle should remove salt pixel: got R=%d, want 100", pixels[idx])
	}
}

// ---------------------------------------------------------------------------
// Minimum (erosion)
// ---------------------------------------------------------------------------

func TestFilterMinimum(t *testing.T) {
	// Bright pixel surrounded by dark → becomes dark.
	w, h := 3, 3
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 50
		pixels[i+1] = 50
		pixels[i+2] = 50
		pixels[i+3] = 255
	}
	idx := (1*w + 1) * 4
	pixels[idx] = 200
	pixels[idx+1] = 200
	pixels[idx+2] = 200

	params, _ := json.Marshal(map[string]any{"radius": 1})
	if err := filterMinimum(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	if pixels[idx] != 50 {
		t.Errorf("minimum should erode bright pixel to 50, got %d", pixels[idx])
	}
}

func TestFilterMinimumZeroRadiusIsNoop(t *testing.T) {
	pixels := []byte{255, 0, 0, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"radius": 0})
	if err := filterMinimum(pixels, 1, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with radius 0", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Maximum (dilation)
// ---------------------------------------------------------------------------

func TestFilterMaximum(t *testing.T) {
	// Dark pixel surrounded by bright → becomes bright.
	w, h := 3, 3
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 200
		pixels[i+1] = 200
		pixels[i+2] = 200
		pixels[i+3] = 255
	}
	idx := (1*w + 1) * 4
	pixels[idx] = 50
	pixels[idx+1] = 50
	pixels[idx+2] = 50

	params, _ := json.Marshal(map[string]any{"radius": 1})
	if err := filterMaximum(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	if pixels[idx] != 200 {
		t.Errorf("maximum should dilate dark pixel to 200, got %d", pixels[idx])
	}
}

func TestFilterMaximumZeroRadiusIsNoop(t *testing.T) {
	pixels := []byte{255, 0, 0, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"radius": 0})
	if err := filterMaximum(pixels, 1, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with radius 0", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Ripple (Distort)
// ---------------------------------------------------------------------------

func TestFilterRipple(t *testing.T) {
	// Create a 20x20 horizontal gradient image.
	w, h := 20, 20
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			v := byte(x * 255 / (w - 1))
			pixels[i] = v
			pixels[i+1] = v
			pixels[i+2] = v
			pixels[i+3] = 255
		}
	}
	orig := append([]byte(nil), pixels...)

	params, _ := json.Marshal(map[string]any{"amount": 3, "size": "small"})
	if err := filterRipple(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// At least some pixels should have been displaced from the original.
	changed := 0
	for i := 0; i < len(pixels); i += 4 {
		if pixels[i] != orig[i] || pixels[i+1] != orig[i+1] || pixels[i+2] != orig[i+2] {
			changed++
		}
	}
	if changed == 0 {
		t.Error("ripple should displace some pixels")
	}
}

func TestFilterRippleZeroAmountIsNoop(t *testing.T) {
	pixels := []byte{128, 64, 32, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"amount": 0, "size": "medium"})
	if err := filterRipple(pixels, 1, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with amount 0", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Twirl (Distort)
// ---------------------------------------------------------------------------

func TestFilterTwirl(t *testing.T) {
	// Create a 21x21 asymmetric image: left half dark, right half bright.
	w, h := 21, 21
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if x < w/2 {
				pixels[i], pixels[i+1], pixels[i+2] = 30, 30, 30
			} else {
				pixels[i], pixels[i+1], pixels[i+2] = 220, 220, 220
			}
			pixels[i+3] = 255
		}
	}

	// Save center pixel value before twirl.
	cx, cy := w/2, h/2
	centerIdx := (cy*w + cx) * 4
	centerOrig := pixels[centerIdx]

	params, _ := json.Marshal(map[string]any{"angle": 90})
	if err := filterTwirl(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// Center pixel should stay roughly the same (it's the pivot).
	centerDiff := int(pixels[centerIdx]) - int(centerOrig)
	if centerDiff < -10 || centerDiff > 10 {
		t.Errorf("center pixel shifted too much: was %d, now %d", centerOrig, pixels[centerIdx])
	}

	// Edge pixels outside maxDist should be unchanged — check corners.
	cornerIdx := 0 // top-left
	// Top-left is dark (30), it should remain 30 since it's outside the twirl radius.
	if pixels[cornerIdx] != 30 {
		t.Errorf("corner pixel should be unchanged: was 30, now %d", pixels[cornerIdx])
	}
}

func TestFilterTwirlZeroAngleIsNoop(t *testing.T) {
	pixels := []byte{128, 64, 32, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"angle": 0})
	if err := filterTwirl(pixels, 1, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with angle 0", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Offset (Distort)
// ---------------------------------------------------------------------------

func TestFilterOffset(t *testing.T) {
	// 4x1 strip: R, G, B, W
	w, h := 4, 1
	pixels := []byte{
		255, 0, 0, 255, // R
		0, 255, 0, 255, // G
		0, 0, 255, 255, // B
		255, 255, 255, 255, // W
	}

	// Shift right by 1 with wrap: each pixel's source is one to the left.
	params, _ := json.Marshal(map[string]any{"horizontal": 1, "vertical": 0, "wrap": "wrap"})
	if err := filterOffset(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// After shifting right by 1 with wrap:
	// pixel[0] sources from x=-1 → wraps to x=3 (W)
	// pixel[1] sources from x=0 (R)
	// pixel[2] sources from x=1 (G)
	// pixel[3] sources from x=2 (B)
	if pixels[0] != 255 || pixels[1] != 255 || pixels[2] != 255 {
		t.Errorf("pixel 0 should be W(255,255,255), got (%d,%d,%d)", pixels[0], pixels[1], pixels[2])
	}
	if pixels[4] != 255 || pixels[5] != 0 || pixels[6] != 0 {
		t.Errorf("pixel 1 should be R(255,0,0), got (%d,%d,%d)", pixels[4], pixels[5], pixels[6])
	}
	if pixels[8] != 0 || pixels[9] != 255 || pixels[10] != 0 {
		t.Errorf("pixel 2 should be G(0,255,0), got (%d,%d,%d)", pixels[8], pixels[9], pixels[10])
	}
	if pixels[12] != 0 || pixels[13] != 0 || pixels[14] != 255 {
		t.Errorf("pixel 3 should be B(0,0,255), got (%d,%d,%d)", pixels[12], pixels[13], pixels[14])
	}
}

func TestFilterOffsetRepeat(t *testing.T) {
	// 3x1 strip: 10, 20, 30
	w, h := 3, 1
	pixels := []byte{
		10, 10, 10, 255,
		20, 20, 20, 255,
		30, 30, 30, 255,
	}

	// Shift right by 2 with repeat (clamp): source for pixel 0 is x=-2 → clamped to 0.
	params, _ := json.Marshal(map[string]any{"horizontal": 2, "vertical": 0, "wrap": "repeat"})
	if err := filterOffset(pixels, w, h, nil, params); err != nil {
		t.Fatal(err)
	}

	// pixel[0] sources from x=-2 → clamped to 0 → value 10
	// pixel[1] sources from x=-1 → clamped to 0 → value 10
	// pixel[2] sources from x=0 → value 10
	if pixels[0] != 10 || pixels[4] != 10 || pixels[8] != 10 {
		t.Errorf("all pixels should be 10 with repeat clamp, got %d %d %d", pixels[0], pixels[4], pixels[8])
	}
}

func TestFilterOffsetZeroIsNoop(t *testing.T) {
	pixels := []byte{128, 64, 32, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"horizontal": 0, "vertical": 0, "wrap": "wrap"})
	if err := filterOffset(pixels, 1, 1, nil, params); err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with zero offset", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Polar Coordinates (Distort)
// ---------------------------------------------------------------------------

func TestFilterPolarCoordinates(t *testing.T) {
	// Round-trip test: rect-to-polar then polar-to-rect should approximately recover original.
	w, h := 32, 32
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			pixels[i] = byte(x * 255 / (w - 1))
			pixels[i+1] = byte(y * 255 / (h - 1))
			pixels[i+2] = 128
			pixels[i+3] = 255
		}
	}
	orig := append([]byte(nil), pixels...)

	// Forward: rectangular-to-polar
	params1, _ := json.Marshal(map[string]any{"mode": "rectangular-to-polar"})
	if err := filterPolarCoordinates(pixels, w, h, nil, params1); err != nil {
		t.Fatal(err)
	}

	// The image should have changed.
	changed := 0
	for i := 0; i < len(pixels); i += 4 {
		if pixels[i] != orig[i] || pixels[i+1] != orig[i+1] {
			changed++
		}
	}
	if changed == 0 {
		t.Error("polar coordinates should change the image")
	}

	// Reverse: polar-to-rectangular
	params2, _ := json.Marshal(map[string]any{"mode": "polar-to-rectangular"})
	if err := filterPolarCoordinates(pixels, w, h, nil, params2); err != nil {
		t.Fatal(err)
	}

	// Check center region pixels are approximately recovered (edges lose precision).
	tolerance := 30
	recovered := 0
	total := 0
	for y := h / 4; y < 3*h/4; y++ {
		for x := w / 4; x < 3*w/4; x++ {
			i := (y*w + x) * 4
			total++
			dr := int(pixels[i]) - int(orig[i])
			dg := int(pixels[i+1]) - int(orig[i+1])
			if dr >= -tolerance && dr <= tolerance && dg >= -tolerance && dg <= tolerance {
				recovered++
			}
		}
	}
	recoveryRate := float64(recovered) / float64(total)
	if recoveryRate < 0.3 {
		t.Errorf("round-trip recovery too low: %.1f%% of center pixels within tolerance", recoveryRate*100)
	}
}
