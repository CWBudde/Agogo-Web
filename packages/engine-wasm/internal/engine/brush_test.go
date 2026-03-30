package engine

import (
	"math"
	"testing"
)

// --- Integration tests ---

func TestPaintStroke_LayerModified(t *testing.T) {
	const w, h = 200, 200
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("paint-test", "Paint", w, h)
	layer := NewPixelLayer("Paint Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	layerID := layer.ID()
	doc.ActiveLayerID = layerID
	inst.manager.Create(doc)

	// Helper: get pixels from the stored document layer.
	storedPixels := func() []byte {
		d := inst.manager.activeMut()
		if d == nil {
			t.Fatal("no active document")
		}
		l := findPixelLayer(d, layerID)
		if l == nil {
			t.Fatal("layer not found in stored document")
		}
		return l.Pixels
	}

	// All pixels must start fully transparent.
	for i, v := range storedPixels() {
		if i%4 == 3 && v != 0 {
			t.Fatalf("pixel[%d] alpha = %d before stroke, want 0", i/4, v)
		}
	}

	brush := BrushParams{Size: 30, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	cx, cy := float64(w/2), float64(h/2)

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: cx + 50, Y: cy, Pressure: 1.0})
	inst.handleEndPaintStroke()

	// At least one pixel must have non-zero alpha after the stroke.
	painted := false
	for i, v := range storedPixels() {
		if i%4 == 3 && v != 0 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatal("no pixels were painted after paint stroke")
	}

	// Undo must restore all pixels to transparent.
	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	inst.manager.activeMut().ContentVersion++

	for i, v := range storedPixels() {
		if i%4 == 3 && v != 0 {
			t.Fatalf("pixel[%d] alpha = %d after undo, want 0", i/4, v)
		}
	}
}

func TestPaintStroke_NilLayerIsNoop(t *testing.T) {
	// instance with no document — all three handlers must be no-ops
	inst := &instance{}
	inst.manager = newDocumentManager()
	inst.history = newHistoryStack(defaultHistoryMax)

	brush := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{0, 0, 0, 255}}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 50, Y: 50, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 60, Y: 50, Pressure: 1.0})
	inst.handleEndPaintStroke()
	// If we get here without panic: pass
}

func TestPaintDab_HardBrush_CenterPixelFilled(t *testing.T) {
	bounds := LayerBounds{X: 0, Y: 0, W: 20, H: 20}
	layer := NewPixelLayer("test", bounds, make([]byte, 20*20*4))

	params := BrushParams{
		Size:     10.0,
		Hardness: 1.0,
		Flow:     1.0,
		Color:    [4]uint8{255, 0, 0, 255},
	}

	PaintDab(layer, 10.0, 10.0, params)

	idx := (10*20 + 10) * 4
	if layer.Pixels[idx] < 200 {
		t.Errorf("center R = %d, want >= 200", layer.Pixels[idx])
	}
	if layer.Pixels[idx+3] < 200 {
		t.Errorf("center A = %d, want >= 200", layer.Pixels[idx+3])
	}
}

func TestPaintDab_SoftBrush_EdgePixelPartialAlpha(t *testing.T) {
	bounds := LayerBounds{X: 0, Y: 0, W: 40, H: 40}
	layer := NewPixelLayer("test", bounds, make([]byte, 40*40*4))

	params := BrushParams{
		Size:     20.0,
		Hardness: 0.0,
		Flow:     1.0,
		Color:    [4]uint8{0, 0, 255, 255},
	}

	PaintDab(layer, 20.0, 20.0, params)

	// Center should have blue
	centerIdx := (20*40 + 20) * 4
	if layer.Pixels[centerIdx+2] < 200 {
		t.Errorf("center B = %d, want > 200 for soft brush", layer.Pixels[centerIdx+2])
	}

	// Pixel outside the dab should be empty
	outerIdx := (32*40 + 20) * 4
	if layer.Pixels[outerIdx+2] != 0 {
		t.Errorf("pixel outside dab radius should be transparent, got B=%d", layer.Pixels[outerIdx+2])
	}
}

func TestPaintDab_FlowReducesOpacity(t *testing.T) {
	bounds := LayerBounds{X: 0, Y: 0, W: 20, H: 20}
	layer := NewPixelLayer("test", bounds, make([]byte, 20*20*4))

	params := BrushParams{
		Size:     10.0,
		Hardness: 1.0,
		Flow:     0.5,
		Color:    [4]uint8{255, 0, 0, 255},
	}

	PaintDab(layer, 10.0, 10.0, params)

	centerIdx := (10*20 + 10) * 4
	alpha := layer.Pixels[centerIdx+3]
	if alpha < 100 || alpha > 155 {
		t.Errorf("flow=0.5 center alpha = %d, want ~127", alpha)
	}
}

func TestBrushStrokeState_FirstPointAlwaysPlaced(t *testing.T) {
	var s brushStrokeState
	dabs := s.AddPoint(10, 10, 0.25, 20) // interval = 0.25*20 = 5px
	if len(dabs) != 1 {
		t.Fatalf("first point: want 1 dab, got %d", len(dabs))
	}
	if dabs[0] != [2]float64{10, 10} {
		t.Errorf("first dab = %v, want {10,10}", dabs[0])
	}
}

func TestBrushStrokeState_DabSpacing(t *testing.T) {
	var s brushStrokeState
	s.AddPoint(0, 0, 0.25, 20) // first dab at origin; interval = 5px

	// Move 12px right → 2 dabs (at ~5px and ~10px)
	dabs := s.AddPoint(12, 0, 0.25, 20)
	if len(dabs) != 2 {
		t.Fatalf("12px move at 5px interval: want 2 dabs, got %d", len(dabs))
	}
	if math.Abs(dabs[0][0]-5) > 0.5 {
		t.Errorf("dab[0].x = %.2f, want ~5", dabs[0][0])
	}
	if math.Abs(dabs[1][0]-10) > 0.5 {
		t.Errorf("dab[1].x = %.2f, want ~10", dabs[1][0])
	}
}

func TestBrushStrokeState_ShortMoveProducesNoDab(t *testing.T) {
	var s brushStrokeState
	s.AddPoint(0, 0, 0.25, 20) // interval = 5px
	dabs := s.AddPoint(2, 0, 0.25, 20)
	if len(dabs) != 0 {
		t.Fatalf("2px move: want 0 dabs, got %d", len(dabs))
	}
}
