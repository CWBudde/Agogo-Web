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

func TestMixerBrushStroke_SamplesMergedCanvasColor(t *testing.T) {
	const w, h = 20, 20
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("mixer-test", "Mixer", w, h)

	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	for i := 0; i < len(background.Pixels); i += 4 {
		background.Pixels[i] = 255
		background.Pixels[i+3] = 255
	}
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 255, 255},
		MixerBrush:   true,
		MixerMix:     0.5,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 10, Y: 10, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after mixer stroke")
	}
	idx := (10*20 + 10) * 4
	if layer.Pixels[idx] < 80 || layer.Pixels[idx+2] < 80 {
		t.Fatalf("mixed center pixel = RGBA(%d,%d,%d,%d), want blended red+blue", layer.Pixels[idx], layer.Pixels[idx+1], layer.Pixels[idx+2], layer.Pixels[idx+3])
	}
	if layer.Pixels[idx+1] > 32 {
		t.Fatalf("mixed center green channel = %d, want near 0", layer.Pixels[idx+1])
	}
}

func TestCloneStampStroke_ClonesMergedSourcePixels(t *testing.T) {
	const w, h = 24, 24
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-test", "Clone", w, h)

	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	for i := 0; i < len(background.Pixels); i += 4 {
		background.Pixels[i] = 255
		background.Pixels[i+3] = 255
	}
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		CloneStamp:   true,
		CloneSourceX: 6,
		CloneSourceY: 6,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 16, Y: 16, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after clone stroke")
	}
	idx := (16*24 + 16) * 4
	if layer.Pixels[idx] < 200 || layer.Pixels[idx+1] != 0 || layer.Pixels[idx+2] != 0 {
		t.Fatalf("cloned center pixel = RGBA(%d,%d,%d,%d), want red copied from source", layer.Pixels[idx], layer.Pixels[idx+1], layer.Pixels[idx+2], layer.Pixels[idx+3])
	}
	if layer.Pixels[idx+3] < 200 {
		t.Fatalf("cloned alpha = %d, want opaque", layer.Pixels[idx+3])
	}
}

func TestHistoryBrushStroke_RestoresPreviousHistoryState(t *testing.T) {
	const w, h = 24, 24
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("history-test", "History", w, h)
	layer := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)

	red := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	blue := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{0, 0, 255, 255}}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: red})
	inst.handleEndPaintStroke()
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: blue})
	inst.handleEndPaintStroke()

	historyBrush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		HistoryBrush: true,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: historyBrush})
	inst.handleEndPaintStroke()

	painted := findPixelLayer(inst.manager.activeMut(), layer.ID())
	if painted == nil {
		t.Fatal("layer not found after history brush stroke")
	}
	idx := (12*24 + 12) * 4
	if painted.Pixels[idx] < 200 || painted.Pixels[idx+1] != 0 || painted.Pixels[idx+2] != 0 {
		t.Fatalf("history-brushed pixel = RGBA(%d,%d,%d,%d), want previous red state", painted.Pixels[idx], painted.Pixels[idx+1], painted.Pixels[idx+2], painted.Pixels[idx+3])
	}
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

// ── Stabilizer ────────────────────────────────────────────────────────────────

func TestStabilizer_ZeroLag_Passthrough(t *testing.T) {
	s := newStabilizer(0)
	for _, pt := range [][2]float64{{10, 20}, {30, 40}, {50, 60}} {
		ox, oy := s.Push(pt[0], pt[1])
		if ox != pt[0] || oy != pt[1] {
			t.Fatalf("lag=0: got (%.1f,%.1f), want (%.1f,%.1f)", ox, oy, pt[0], pt[1])
		}
	}
}

func TestStabilizer_FirstPushReturnsInput(t *testing.T) {
	s := newStabilizer(5)
	ox, oy := s.Push(100, 200)
	if ox != 100 || oy != 200 {
		t.Fatalf("first push: got (%.1f,%.1f), want (100,200)", ox, oy)
	}
}

func TestStabilizer_AveragesBuffer(t *testing.T) {
	s := newStabilizer(3)
	s.Push(0, 0)
	s.Push(6, 6)
	ox, oy := s.Push(3, 3) // buf = [0,6,3] avg = 3
	if math.Abs(ox-3) > 0.01 || math.Abs(oy-3) > 0.01 {
		t.Fatalf("3-point average: got (%.4f,%.4f), want (3,3)", ox, oy)
	}
}

func TestStabilizer_SmoothsPath(t *testing.T) {
	// Feed alternating y=60/40 jitter around 50. After the buffer fills the
	// mean output should be much closer to 50 than the ±10 raw values.
	s := newStabilizer(8)
	raw := []float64{60, 40, 60, 40, 60, 40, 60, 40, 60, 40}
	var sumDev float64
	n := 0
	for i, v := range raw {
		_, oy := s.Push(0, v)
		if i >= 8 { // only measure once buffer is full
			sumDev += math.Abs(oy - 50)
			n++
		}
	}
	if n == 0 {
		t.Fatal("not enough samples to evaluate smoothing")
	}
	meanDev := sumDev / float64(n)
	if meanDev >= 5 {
		t.Errorf("mean deviation after buffer full = %.2f, want < 5 (raw jitter = 10)", meanDev)
	}
}

// ── Scatter ───────────────────────────────────────────────────────────────────

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

// ── Pencil / Auto-erase ───────────────────────────────────────────────────────

// newPencilTestInstance creates a minimal instance with a single white-filled
// pixel layer ready for pencil/auto-erase tests.
func newPencilTestInstance(t *testing.T, w, h int, fillColor [4]uint8) (*instance, *PixelLayer, string) {
	t.Helper()
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("pencil-test", "Pencil", w, h)
	pixels := make([]byte, w*h*4)
	// Pre-fill every pixel with fillColor.
	for i := 0; i < w*h; i++ {
		pixels[i*4] = fillColor[0]
		pixels[i*4+1] = fillColor[1]
		pixels[i*4+2] = fillColor[2]
		pixels[i*4+3] = fillColor[3]
	}
	layer := NewPixelLayer("Pencil Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, pixels)
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)
	return inst, layer, layer.ID()
}

func TestAutoErase_SwitchedToBackgroundWhenFGMatches(t *testing.T) {
	// Layer pre-filled with red (the foreground color).
	// Auto-erase should switch to the background color (blue) for the stroke.
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, red)
	inst.backgroundColor = blue

	cx, cy := float64(w/2), float64(h/2)
	brush := BrushParams{
		Size:      10,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     red, // foreground
		AutoErase: true,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	idx := (int(cy)*w + int(cx)) * 4
	if layer.Pixels[idx] != 0 || layer.Pixels[idx+2] < 200 {
		t.Errorf("auto-erase: center R=%d B=%d, want R≈0 B≈255 (painted blue)", layer.Pixels[idx], layer.Pixels[idx+2])
	}
}

func TestAutoErase_NoPaintSwitchWhenFGDiffers(t *testing.T) {
	// Layer pre-filled with green — does NOT match the red foreground.
	// Auto-erase should NOT switch; stroke paints red.
	const w, h = 50, 50
	green := [4]uint8{0, 255, 0, 255}
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, green)
	inst.backgroundColor = blue

	cx, cy := float64(w/2), float64(h/2)
	brush := BrushParams{
		Size:      10,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     red, // foreground
		AutoErase: true,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	idx := (int(cy)*w + int(cx)) * 4
	if layer.Pixels[idx] < 200 {
		t.Errorf("no auto-erase: center R=%d, want ≥200 (painted red)", layer.Pixels[idx])
	}
}

func TestAutoErase_FalseNeverSwitches(t *testing.T) {
	// AutoErase=false: even if start pixel matches FG, stroke uses FG (red).
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, red)
	inst.backgroundColor = blue

	cx, cy := float64(w/2), float64(h/2)
	brush := BrushParams{
		Size:      10,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     red,
		AutoErase: false,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	idx := (int(cy)*w + int(cx)) * 4
	// Should still be (approximately) red at center.
	if layer.Pixels[idx] < 200 {
		t.Errorf("autoErase=false: center R=%d, want ≥200 (no color switch)", layer.Pixels[idx])
	}
}

func TestPencilTool_HardnessOneIsFullyOpaque(t *testing.T) {
	// Pencil mode uses hardness=1.0. Center pixel must be fully opaque.
	const sz = 40
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	params := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	PaintDab(layer, float64(sz/2), float64(sz/2), params, 0, 1)

	idx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[idx+3] < 250 {
		t.Errorf("pencil center alpha = %d, want ~255 (hardness=1.0)", layer.Pixels[idx+3])
	}
}

// ── Eraser ────────────────────────────────────────────────────────────────────

func TestNormalErase_RemovesAlpha(t *testing.T) {
	// Paint a solid red dab, then erase the center — center alpha must drop.
	const sz = 40
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	paint := BrushParams{Size: 30, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	PaintDab(layer, float64(sz/2), float64(sz/2), paint, 0, 1)

	centerIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centerIdx+3] < 250 {
		t.Fatalf("pre-erase: center alpha = %d, want ~255", layer.Pixels[centerIdx+3])
	}

	erase := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, Erase: true}
	PaintDab(layer, float64(sz/2), float64(sz/2), erase, 0, 1)

	if layer.Pixels[centerIdx+3] > 10 {
		t.Errorf("after erase: center alpha = %d, want ~0", layer.Pixels[centerIdx+3])
	}
}

func TestNormalErase_PartialFlow(t *testing.T) {
	// flow=0.5 should reduce alpha to roughly half, not fully erase.
	const sz = 40
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	paint := BrushParams{Size: 30, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	PaintDab(layer, float64(sz/2), float64(sz/2), paint, 0, 1)

	erase := BrushParams{Size: 20, Hardness: 1.0, Flow: 0.5, Erase: true}
	PaintDab(layer, float64(sz/2), float64(sz/2), erase, 0, 1)

	centerIdx := (sz/2*sz + sz/2) * 4
	alpha := layer.Pixels[centerIdx+3]
	// dst-out with src_alpha=~128: result = 255 * (1 - 128/255) ≈ 127
	if alpha < 100 || alpha > 160 {
		t.Errorf("partial erase: center alpha = %d, want ~127", alpha)
	}
}

func TestEraseBackground_MatchingColorErased(t *testing.T) {
	// Layer filled with red. Background erase with red as base color should erase.
	const sz = 40
	red := [4]uint8{255, 0, 0, 255}
	pixels := make([]byte, sz*sz*4)
	for i := range sz * sz {
		pixels[i*4] = red[0]
		pixels[i*4+1] = red[1]
		pixels[i*4+2] = red[2]
		pixels[i*4+3] = red[3]
	}
	layer := NewPixelLayer("test", LayerBounds{X: 0, Y: 0, W: sz, H: sz}, pixels)

	params := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, EraseBackground: true, EraseTolerance: 30}
	EraseBackgroundDab(layer, float64(sz/2), float64(sz/2), params, red)

	centerIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centerIdx+3] > 10 {
		t.Errorf("background erase: center alpha = %d, want ~0 (red matches base)", layer.Pixels[centerIdx+3])
	}
}

func TestEraseBackground_NonMatchingColorKept(t *testing.T) {
	// Layer filled with blue. Background erase with red base color should NOT erase blue.
	const sz = 40
	blue := [4]uint8{0, 0, 255, 255}
	red := [4]uint8{255, 0, 0, 255}
	pixels := make([]byte, sz*sz*4)
	for i := range sz * sz {
		pixels[i*4] = blue[0]
		pixels[i*4+1] = blue[1]
		pixels[i*4+2] = blue[2]
		pixels[i*4+3] = blue[3]
	}
	layer := NewPixelLayer("test", LayerBounds{X: 0, Y: 0, W: sz, H: sz}, pixels)

	params := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, EraseBackground: true, EraseTolerance: 30}
	EraseBackgroundDab(layer, float64(sz/2), float64(sz/2), params, red)

	centerIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centerIdx+3] < 250 {
		t.Errorf("background erase: blue pixel alpha = %d, want ~255 (outside tolerance)", layer.Pixels[centerIdx+3])
	}
}

func TestMagicErase_ClearsMatchingPixels(t *testing.T) {
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, red)

	payload := MagicErasePayload{
		X: float64(w / 2), Y: float64(h / 2),
		Tolerance:  30,
		Contiguous: true,
	}
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	if err := inst.handleMagicErase(payload, doc, layer); err != nil {
		t.Fatalf("handleMagicErase: %v", err)
	}

	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	for i := 3; i < len(layer.Pixels); i += 4 {
		if layer.Pixels[i] != 0 {
			t.Fatalf("pixel %d alpha = %d after magic erase, want 0", i/4, layer.Pixels[i])
		}
	}
}

func TestMagicErase_IsUndoable(t *testing.T) {
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}
	inst, _, layerID := newPencilTestInstance(t, w, h, red)

	payload := MagicErasePayload{X: float64(w / 2), Y: float64(h / 2), Tolerance: 30, Contiguous: true}
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	if err := inst.handleMagicErase(payload, doc, layer); err != nil {
		t.Fatalf("handleMagicErase: %v", err)
	}

	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	inst.manager.activeMut().ContentVersion++

	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	for i := 3; i < len(layer.Pixels); i += 4 {
		if layer.Pixels[i] != red[3] {
			t.Fatalf("pixel %d alpha = %d after undo, want %d", i/4, layer.Pixels[i], red[3])
		}
	}
}
