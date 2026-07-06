package engine

import (
	"bytes"
	"testing"
)

// newPaintBatchTestInstance builds a fresh instance with a single opaque-capable
// transparent pixel layer, mirroring the setup used by the other brush tests.
func newPaintBatchTestInstance(t *testing.T, w, h int) (*instance, string) {
	t.Helper()
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("paint-batch", "PaintBatch", w, h)
	layer := NewPixelLayer("Paint Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)
	return inst, layer.ID()
}

func batchStoredLayerPixels(t *testing.T, inst *instance, layerID string) []byte {
	t.Helper()
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

// TestContinuePaintStroke_BatchEqualsSinglePoints verifies that dispatching one
// multi-point ContinuePaintStroke command with N points produces byte-identical
// layer pixels to dispatching N single-point ContinuePaintStroke commands, given
// identical begin/end conditions and stroke parameters.
func TestContinuePaintStroke_BatchEqualsSinglePoints(t *testing.T) {
	const w, h = 200, 200
	brush := BrushParams{Size: 24, Hardness: 0.8, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}

	points := []StrokePoint{
		{X: 60, Y: 100, Pressure: 0.4},
		{X: 75, Y: 108, Pressure: 0.7},
		{X: 92, Y: 95, Pressure: 1.0},
		{X: 110, Y: 120, Pressure: 0.5},
		{X: 130, Y: 100, Pressure: 0.9},
	}

	// Run A: N single-point continue commands.
	instA, layerA := newPaintBatchTestInstance(t, w, h)
	instA.handleBeginPaintStroke(BeginPaintStrokePayload{X: 50, Y: 100, Pressure: 0.5, Brush: brush})
	for _, p := range points {
		instA.handleContinuePaintStroke(ContinuePaintStrokePayload(p))
	}
	_ = instA.handleEndPaintStroke()
	pixelsA := append([]byte(nil), batchStoredLayerPixels(t, instA, layerA)...)

	// Run B: one multi-point continue command carrying the same points.
	instB, layerB := newPaintBatchTestInstance(t, w, h)
	instB.handleBeginPaintStroke(BeginPaintStrokePayload{X: 50, Y: 100, Pressure: 0.5, Brush: brush})
	instB.handleContinuePaintStrokePoints(points)
	_ = instB.handleEndPaintStroke()
	pixelsB := append([]byte(nil), batchStoredLayerPixels(t, instB, layerB)...)

	if len(pixelsA) != len(pixelsB) {
		t.Fatalf("pixel buffer length mismatch: single=%d batch=%d", len(pixelsA), len(pixelsB))
	}
	if !bytes.Equal(pixelsA, pixelsB) {
		diffs := 0
		firstIdx := -1
		for i := range pixelsA {
			if pixelsA[i] != pixelsB[i] {
				if firstIdx < 0 {
					firstIdx = i
				}
				diffs++
			}
		}
		t.Fatalf("multi-point batch pixels differ from single-point pixels: %d byte diffs, first at index %d (px %d)", diffs, firstIdx, firstIdx/4)
	}

	// Sanity: the stroke actually painted something.
	painted := false
	for i := 3; i < len(pixelsA); i += 4 {
		if pixelsA[i] != 0 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatal("no pixels were painted; test would trivially pass")
	}
}
