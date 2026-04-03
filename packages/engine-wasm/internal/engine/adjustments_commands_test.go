package engine

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Histogram tests
// ---------------------------------------------------------------------------

func TestComputeHistogramActiveLayer(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Add a 4x4 pixel layer filled with red (255,0,0).
	redPixels := make([]byte, 4*4*4)
	for i := 0; i < len(redPixels); i += 4 {
		redPixels[i] = 255
		redPixels[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "red-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    redPixels,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Compute histogram for active layer.
	result, err := DispatchCommand(h, commandComputeHistogram, mustJSON(t, ComputeHistogramPayload{}))
	if err != nil {
		t.Fatalf("compute histogram: %v", err)
	}

	if result.Histogram == nil {
		t.Fatal("expected histogram data, got nil")
	}

	// All 16 pixels are red (255,0,0) — Red[255] should be 16, Green[0] and Blue[0] should be 16.
	hist := result.Histogram
	if hist.Red[255] != 16 {
		t.Errorf("Red[255] = %d, want 16", hist.Red[255])
	}
	if hist.Green[0] != 16 {
		t.Errorf("Green[0] = %d, want 16", hist.Green[0])
	}
	if hist.Blue[0] != 16 {
		t.Errorf("Blue[0] = %d, want 16", hist.Blue[0])
	}
	// Red bin 0 should be 0 for the red channel.
	if hist.Red[0] != 0 {
		t.Errorf("Red[0] = %d, want 0", hist.Red[0])
	}
}

func TestComputeHistogramMerged(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Add a pixel layer.
	px := make([]byte, 2*2*4)
	for i := 0; i < len(px); i += 4 {
		px[i] = 128
		px[i+1] = 64
		px[i+2] = 32
		px[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    px,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	result, err := DispatchCommand(h, commandComputeHistogram, mustJSON(t, ComputeHistogramPayload{
		LayerID: "merged",
	}))
	if err != nil {
		t.Fatalf("compute histogram merged: %v", err)
	}

	if result.Histogram == nil {
		t.Fatal("expected histogram data")
	}
	// Merged composite should have data.
	var totalRed uint32
	for _, v := range result.Histogram.Red {
		totalRed += v
	}
	if totalRed == 0 {
		t.Error("merged histogram should have non-zero pixel count")
	}
}

func TestComputeHistogramLuminance(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// White pixels — luminance should be at 255.
	px := make([]byte, 2*2*4)
	for i := 0; i < len(px); i += 4 {
		px[i] = 255
		px[i+1] = 255
		px[i+2] = 255
		px[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "white",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    px,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	result, err := DispatchCommand(h, commandComputeHistogram, mustJSON(t, ComputeHistogramPayload{}))
	if err != nil {
		t.Fatalf("compute histogram: %v", err)
	}

	if result.Histogram.Luminance[255] != 4 {
		t.Errorf("Luminance[255] = %d, want 4", result.Histogram.Luminance[255])
	}
}

// ---------------------------------------------------------------------------
// Identify Hue Range tests
// ---------------------------------------------------------------------------

func TestClassifyHueRange(t *testing.T) {
	tests := []struct {
		name     string
		r, g, b  uint8
		expected string
	}{
		{"pure red", 255, 0, 0, "reds"},
		{"pure yellow", 255, 255, 0, "yellows"},
		{"pure green", 0, 255, 0, "greens"},
		{"pure cyan", 0, 255, 255, "cyans"},
		{"pure blue", 0, 0, 255, "blues"},
		{"pure magenta", 255, 0, 255, "magentas"},
		{"gray (low sat)", 128, 128, 128, "master"},
		{"white", 255, 255, 255, "master"},
		{"black", 0, 0, 0, "master"},
		{"orange", 255, 128, 0, "yellows"}, // hue ~30
		{"teal", 0, 128, 128, "cyans"},     // hue ~180
		{"violet", 128, 0, 255, "blues"},   // hue ~270
		{"warm red", 255, 50, 50, "reds"},  // hue ~0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHueRange(tt.r, tt.g, tt.b)
			if got != tt.expected {
				t.Errorf("classifyHueRange(%d,%d,%d) = %q, want %q", tt.r, tt.g, tt.b, got, tt.expected)
			}
		})
	}
}

func TestIdentifyHueRangeCommand(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Add a layer with a known blue pixel at (0,0).
	px := make([]byte, 2*2*4)
	for i := 0; i < len(px); i += 4 {
		px[i] = 0
		px[i+1] = 0
		px[i+2] = 255
		px[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "blue-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    px,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	result, err := DispatchCommand(h, commandIdentifyHueRange, mustJSON(t, IdentifyHueRangePayload{
		X: 0, Y: 0,
	}))
	if err != nil {
		t.Fatalf("identify hue range: %v", err)
	}

	if result.IdentifiedHueRange != "blues" {
		t.Errorf("expected 'blues', got %q", result.IdentifiedHueRange)
	}
}

// ---------------------------------------------------------------------------
// Set Point From Sample tests
// ---------------------------------------------------------------------------

func TestSetPointFromSampleBlackPoint(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Add a pixel layer with a known dark color.
	px := make([]byte, 4*4*4)
	for i := 0; i < len(px); i += 4 {
		px[i] = 30
		px[i+1] = 30
		px[i+2] = 30
		px[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "dark-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    px,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Add a curves adjustment layer.
	_, err = DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:      "adjustment",
		Name:           "curves",
		AdjustmentKind: "curves",
	}))
	if err != nil {
		t.Fatalf("add curves: %v", err)
	}

	// Set black point from sampling the dark pixel.
	_, err = DispatchCommand(h, commandSetPointFromSample, mustJSON(t, SetPointFromSamplePayload{
		X:    1,
		Y:    1,
		Mode: "black",
	}))
	if err != nil {
		t.Fatalf("set point: %v", err)
	}

	// Verify the curves params were updated.
	mu.Lock()
	inst := instances[h]
	mu.Unlock()
	doc := inst.manager.Active()
	adj := doc.findLayer(doc.ActiveLayerID).(*AdjustmentLayer)
	var params curvesParams
	if err := json.Unmarshal(adj.Params, &params); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// There should be a point mapping the sampled luminance → 0.
	found := false
	for _, p := range params.Points {
		if p.Y == 0 && p.X > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a curve point with Y=0, got points: %+v", params.Points)
	}
}

func TestSetPointFromSampleWhitePoint(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Bright pixel layer.
	px := make([]byte, 4*4*4)
	for i := 0; i < len(px); i += 4 {
		px[i] = 220
		px[i+1] = 220
		px[i+2] = 220
		px[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "bright-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    px,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Add curves adjustment.
	_, err = DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:      "adjustment",
		Name:           "curves",
		AdjustmentKind: "curves",
	}))
	if err != nil {
		t.Fatalf("add curves: %v", err)
	}

	_, err = DispatchCommand(h, commandSetPointFromSample, mustJSON(t, SetPointFromSamplePayload{
		X:    1,
		Y:    1,
		Mode: "white",
	}))
	if err != nil {
		t.Fatalf("set white point: %v", err)
	}

	mu.Lock()
	inst := instances[h]
	mu.Unlock()
	doc := inst.manager.Active()
	adj := doc.findLayer(doc.ActiveLayerID).(*AdjustmentLayer)
	var params curvesParams
	if err := json.Unmarshal(adj.Params, &params); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	found := false
	for _, p := range params.Points {
		if p.Y == 255 && p.X > 0 && p.X < 255 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a curve point with Y=255, got points: %+v", params.Points)
	}
}

func TestSetPointFromSampleRequiresCurvesLayer(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Add a pixel layer (not curves).
	px := make([]byte, 2*2*4)
	for i := 0; i < len(px); i += 4 {
		px[i] = 100
		px[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "pixel-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    px,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	_, err = DispatchCommand(h, commandSetPointFromSample, mustJSON(t, SetPointFromSamplePayload{
		X:    0,
		Y:    0,
		Mode: "black",
	}))
	if err == nil {
		t.Fatal("expected error for non-curves layer")
	}
}

// ---------------------------------------------------------------------------
// setOrAddCurvePoint helper tests
// ---------------------------------------------------------------------------

func TestSetOrAddCurvePointAdds(t *testing.T) {
	points := []curvePoint{{X: 0, Y: 0}, {X: 255, Y: 255}}
	result := setOrAddCurvePoint(points, 128, 64)
	if len(result) != 3 {
		t.Fatalf("expected 3 points, got %d", len(result))
	}
	// Should be sorted: 0, 128, 255.
	if result[1].X != 128 || result[1].Y != 64 {
		t.Errorf("middle point = {%.0f,%.0f}, want {128,64}", result[1].X, result[1].Y)
	}
}

func TestSetOrAddCurvePointUpdatesExisting(t *testing.T) {
	points := []curvePoint{{X: 0, Y: 0}, {X: 128, Y: 128}, {X: 255, Y: 255}}
	result := setOrAddCurvePoint(points, 130, 50) // within ±5 of 128
	if len(result) != 3 {
		t.Fatalf("expected 3 points (update, not add), got %d", len(result))
	}
	if result[1].Y != 50 {
		t.Errorf("updated point Y = %.0f, want 50", result[1].Y)
	}
}
