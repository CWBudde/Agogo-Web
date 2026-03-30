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

	PaintDab(layer, 10.0, 10.0, params, 0, 1)

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

	PaintDab(layer, 20.0, 20.0, params, 0, 1)

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

	PaintDab(layer, 10.0, 10.0, params, 0, 1)

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

func TestBrushStrokeState_CarryOverAcrossSegments(t *testing.T) {
	// Two successive 4px moves with a 5px interval.
	// After first move (4px): 0 dabs, travelled=4.
	// After second move (4px total=8px): 1 dab at x≈5, travelled=3.
	var s brushStrokeState
	s.AddPoint(0, 0, 0.25, 20) // first dab; interval=5
	d1 := s.AddPoint(4, 0, 0.25, 20)
	if len(d1) != 0 {
		t.Fatalf("first 4px segment: want 0 dabs, got %d", len(d1))
	}
	d2 := s.AddPoint(8, 0, 0.25, 20)
	if len(d2) != 1 {
		t.Fatalf("second 4px segment: want 1 dab, got %d", len(d2))
	}
	if math.Abs(d2[0][0]-5) > 0.5 {
		t.Errorf("carry-over dab x = %.2f, want ~5", d2[0][0])
	}
}

func TestCatmullRomPoint_EndpointInterpolation(t *testing.T) {
	// CR at t=0 must return p1 and at t=1 must return p2 exactly.
	p0 := [2]float64{0, 0}
	p1 := [2]float64{10, 0}
	p2 := [2]float64{20, 5}
	p3 := [2]float64{30, 0}

	got0 := catmullRomPoint(p0, p1, p2, p3, 0)
	if math.Abs(got0[0]-p1[0]) > 1e-9 || math.Abs(got0[1]-p1[1]) > 1e-9 {
		t.Errorf("t=0: got %v, want %v", got0, p1)
	}
	got1 := catmullRomPoint(p0, p1, p2, p3, 1)
	if math.Abs(got1[0]-p2[0]) > 1e-9 || math.Abs(got1[1]-p2[1]) > 1e-9 {
		t.Errorf("t=1: got %v, want %v", got1, p2)
	}
}

func TestCatmullRomPoint_CollinearIsLinear(t *testing.T) {
	// Collinear control points → CR degenerates to a straight line.
	p0 := [2]float64{0, 0}
	p1 := [2]float64{10, 0}
	p2 := [2]float64{20, 0}
	p3 := [2]float64{30, 0}

	for _, tc := range []struct{ t, wantX float64 }{
		{0.25, 12.5},
		{0.5, 15},
		{0.75, 17.5},
	} {
		got := catmullRomPoint(p0, p1, p2, p3, tc.t)
		if math.Abs(got[0]-tc.wantX) > 1e-9 || math.Abs(got[1]) > 1e-9 {
			t.Errorf("t=%.2f: got (%.4f, %.4f), want (%.4f, 0)", tc.t, got[0], got[1], tc.wantX)
		}
	}
}

func TestPaintDab_BlendModeDoesNotPanic(t *testing.T) {
	// Verify PaintDab runs without panic when a blend mode is set.
	bounds := LayerBounds{X: 0, Y: 0, W: 20, H: 20}
	layer := NewPixelLayer("test", bounds, make([]byte, 20*20*4))
	params := BrushParams{
		Size:      10.0,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     [4]uint8{255, 0, 0, 255},
		BlendMode: "multiply",
	}
	PaintDab(layer, 10.0, 10.0, params, 0, 1) // must not panic
}

func TestApplyTilt_NoTilt(t *testing.T) {
	az, sq := applyTilt(0, 0)
	if az != 0 || sq != 1 {
		t.Errorf("no tilt: got azimuth=%.4f squish=%.4f, want 0 1", az, sq)
	}
}

func TestApplyTilt_VerticalPen(t *testing.T) {
	// tiltMag → 0 means pen is perfectly upright → squish ≈ sin(90°) = 1
	az, sq := applyTilt(0, 1) // nearly upright
	if math.Abs(sq-math.Sin(89*math.Pi/180)) > 0.01 {
		t.Errorf("near-upright squish = %.4f, want ≈ sin(89°)=%.4f", sq, math.Sin(89*math.Pi/180))
	}
	_ = az
}

func TestApplyTilt_HorizontalPen_MinSquish(t *testing.T) {
	// tiltMag = 90 → altitude = 0° → squish = sin(0°) = 0 → clamp to minSquish
	az, sq := applyTilt(90, 0)
	if sq > 0.1 {
		t.Errorf("flat pen squish = %.4f, want minSquish (0.05)", sq)
	}
	if math.Abs(az) > 0.001 {
		t.Errorf("flat pen leaning along X: azimuth = %.4f, want ≈0", az)
	}
}

func TestApplyTilt_AzimuthDirection(t *testing.T) {
	// tiltY > 0 only → stylus leans toward +Y → azimuth = π/2
	az, _ := applyTilt(0, 45)
	if math.Abs(az-math.Pi/2) > 0.01 {
		t.Errorf("tiltY=45: azimuth = %.4f, want π/2=%.4f", az, math.Pi/2)
	}
}

func TestPaintDab_TiltedDabIsElliptical(t *testing.T) {
	// A tilted (squish<1) dab must paint fewer pixels than an untilted one because
	// the ellipse minor axis is squished. Sample column coverage at the dab centre row.
	const sz = 60
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}

	// Untilted round dab.
	round := NewPixelLayer("round", bounds, make([]byte, sz*sz*4))
	PaintDab(round, float64(sz/2), float64(sz/2), BrushParams{Size: 40, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}}, 0, 1)

	// Maximally squished dab (squish = minSquish ≈ 0.05, azimuth = 0 → major axis along X).
	squished := NewPixelLayer("squished", bounds, make([]byte, sz*sz*4))
	PaintDab(squished, float64(sz/2), float64(sz/2), BrushParams{Size: 40, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}}, 0, 0.05)

	// Count painted pixels in both layers.
	countPainted := func(l *PixelLayer) int {
		n := 0
		for i := 3; i < len(l.Pixels); i += 4 {
			if l.Pixels[i] > 0 {
				n++
			}
		}
		return n
	}

	roundN := countPainted(round)
	squishedN := countPainted(squished)
	if squishedN >= roundN {
		t.Errorf("tilted dab painted %d px, round dab painted %d px — expected fewer for squished", squishedN, roundN)
	}
}

func TestPaintDab_WetEdges_TransparentCentre(t *testing.T) {
	// Wet edges: centre pixel should have very low alpha; ring near edge higher.
	const sz = 60
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	params := BrushParams{
		Size:     50.0,
		Hardness: 0.0,
		Flow:     1.0,
		Color:    [4]uint8{0, 0, 255, 255},
		WetEdges: true,
	}
	cx, cy := float64(sz/2), float64(sz/2)
	PaintDab(layer, cx, cy, params, 0, 1)

	// Centre pixel must be mostly transparent.
	centreIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centreIdx+3] > 30 {
		t.Errorf("wet-edges centre alpha = %d, want ≤ 30 (transparent centre)", layer.Pixels[centreIdx+3])
	}

	// A pixel at ~75% radius (ring position) must be more opaque than centre.
	ringX := int(cx + 25*0.75) // radius=25, peak at 75%
	ringIdx := (sz/2*sz + ringX) * 4
	if layer.Pixels[ringIdx+3] <= layer.Pixels[centreIdx+3] {
		t.Errorf("wet-edges ring alpha (%d) should exceed centre alpha (%d)",
			layer.Pixels[ringIdx+3], layer.Pixels[centreIdx+3])
	}
}

func TestApplyScatter_ZeroIsIdentity(t *testing.T) {
	p := BrushParams{Size: 40, Scatter: 0}
	for range 20 {
		ox, oy := applyScatter(100, 200, p)
		if ox != 100 || oy != 200 {
			t.Fatalf("scatter=0: got (%.2f, %.2f), want (100, 200)", ox, oy)
		}
	}
}

func TestApplyScatter_OffsetWithinRadius(t *testing.T) {
	p := BrushParams{Size: 40, Scatter: 1.0} // maxR = 40
	for range 500 {
		ox, oy := applyScatter(0, 0, p)
		dist := math.Sqrt(ox*ox + oy*oy)
		if dist > p.Size*p.Scatter+1e-9 {
			t.Fatalf("scatter offset dist=%.4f exceeds maxR=%.4f", dist, p.Size*p.Scatter)
		}
	}
}

func TestApplyScatter_CoversDisc(t *testing.T) {
	// With scatter=1 and many samples, expect offsets in all four quadrants.
	p := BrushParams{Size: 40, Scatter: 1.0}
	q := [4]int{}
	for range 1000 {
		ox, oy := applyScatter(0, 0, p)
		switch {
		case ox >= 0 && oy >= 0:
			q[0]++
		case ox < 0 && oy >= 0:
			q[1]++
		case ox < 0 && oy < 0:
			q[2]++
		default:
			q[3]++
		}
	}
	for i, cnt := range q {
		if cnt < 50 {
			t.Errorf("quadrant %d has only %d samples out of 1000 — scatter not covering disc", i, cnt)
		}
	}
}
