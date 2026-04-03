package engine

import (
	"math"
	"testing"

	agglib "github.com/cwbudde/agg_go"
)

// helper: create an Agg2D renderer attached to a fresh RGBA buffer.
func makeTestRenderer(w, h int) (*agglib.Agg2D, []byte) {
	buf := make([]byte, w*h*4)
	r := agglib.NewAgg2D()
	r.Attach(buf, w, h, w*4)
	return r, buf
}

// helper: count non-zero-alpha pixels.
func countFilledPixels(buf []byte) int {
	n := 0
	for i := 3; i < len(buf); i += 4 {
		if buf[i] > 0 {
			n++
		}
	}
	return n
}

// pixelAlpha returns the alpha at pixel (x, y).
func pixelAlpha(buf []byte, w, x, y int) uint8 {
	return buf[(y*w+x)*4+3]
}

// --- hasNonTrivialHandles ---

func TestHasNonTrivialHandles(t *testing.T) {
	// Trivial: both handles coincide with anchors.
	a := PathPoint{X: 10, Y: 20, InX: 10, InY: 20, OutX: 10, OutY: 20}
	b := PathPoint{X: 30, Y: 40, InX: 30, InY: 40, OutX: 30, OutY: 40}
	if hasNonTrivialHandles(a, b) {
		t.Error("expected trivial handles → false")
	}

	// Non-trivial outgoing handle on prev.
	a2 := PathPoint{X: 10, Y: 20, OutX: 15, OutY: 20}
	b2 := PathPoint{X: 30, Y: 40, InX: 30, InY: 40}
	if !hasNonTrivialHandles(a2, b2) {
		t.Error("expected non-trivial (prev out)")
	}

	// Non-trivial incoming handle on curr.
	a3 := PathPoint{X: 10, Y: 20, OutX: 10, OutY: 20}
	b3 := PathPoint{X: 30, Y: 40, InX: 25, InY: 40}
	if !hasNonTrivialHandles(a3, b3) {
		t.Error("expected non-trivial (curr in)")
	}

	// Both non-trivial.
	a4 := PathPoint{X: 0, Y: 0, OutX: 5, OutY: 0}
	b4 := PathPoint{X: 10, Y: 10, InX: 5, InY: 10}
	if !hasNonTrivialHandles(a4, b4) {
		t.Error("expected non-trivial (both)")
	}
}

// --- evaluateBezier ---

func TestEvaluateBezier(t *testing.T) {
	// Straight-ish curve: P0=(0,0), P1=(10,0), P2=(20,0), P3=(30,0)
	// t=0 → (0,0), t=1 → (30,0), t=0.5 → (15,0)
	x0, y0 := evaluateBezier(0, 0, 10, 0, 20, 0, 30, 0, 0)
	if x0 != 0 || y0 != 0 {
		t.Errorf("t=0: got (%v,%v), want (0,0)", x0, y0)
	}

	x1, y1 := evaluateBezier(0, 0, 10, 0, 20, 0, 30, 0, 1)
	if x1 != 30 || y1 != 0 {
		t.Errorf("t=1: got (%v,%v), want (30,0)", x1, y1)
	}

	xm, ym := evaluateBezier(0, 0, 10, 0, 20, 0, 30, 0, 0.5)
	if math.Abs(xm-15) > 1e-9 || math.Abs(ym) > 1e-9 {
		t.Errorf("t=0.5: got (%v,%v), want (15,0)", xm, ym)
	}

	// Curved: P0=(0,0), P1=(0,10), P2=(10,10), P3=(10,0) — an S-like curve.
	xh, yh := evaluateBezier(0, 0, 0, 10, 10, 10, 10, 0, 0.5)
	if math.Abs(xh-5) > 1e-9 || math.Abs(yh-7.5) > 1e-9 {
		t.Errorf("curved t=0.5: got (%v,%v), want (5,7.5)", xh, yh)
	}
}

// --- applyPathToAgg2D: filled square ---

func TestApplyPathToAgg2D(t *testing.T) {
	r, buf := makeTestRenderer(100, 100)

	// Build a square: (10,10) → (90,10) → (90,90) → (10,90), closed.
	// All handles coincide with anchors (straight edges).
	sq := Path{
		Subpaths: []Subpath{{
			Closed: true,
			Points: []PathPoint{
				{X: 10, Y: 10, InX: 10, InY: 10, OutX: 10, OutY: 10},
				{X: 90, Y: 10, InX: 90, InY: 10, OutX: 90, OutY: 10},
				{X: 90, Y: 90, InX: 90, InY: 90, OutX: 90, OutY: 90},
				{X: 10, Y: 90, InX: 10, InY: 90, OutX: 10, OutY: 90},
			},
		}},
	}

	r.ResetPath()
	r.FillColor(agglib.NewColor(255, 0, 0, 255))
	r.NoLine()
	applyPathToAgg2D(r, &sq)
	r.DrawPath(agglib.FillOnly)

	// Pixel inside the square should be filled.
	if pixelAlpha(buf, 100, 50, 50) == 0 {
		t.Error("center pixel (50,50) should be filled")
	}
	// Pixel outside the square should be empty.
	if pixelAlpha(buf, 100, 2, 2) != 0 {
		t.Error("corner pixel (2,2) should be empty")
	}

	filled := countFilledPixels(buf)
	// The square is roughly 80×80 = 6400 pixels; allow some AA margin.
	if filled < 6000 || filled > 7000 {
		t.Errorf("expected ~6400 filled pixels, got %d", filled)
	}
}

// --- applyPathToAgg2D with curves ---

func TestApplyPathToAgg2DWithCurves(t *testing.T) {
	r, buf := makeTestRenderer(100, 100)

	// A curved closed shape: 3 points with control handles forming a blob.
	curved := Path{
		Subpaths: []Subpath{{
			Closed: true,
			Points: []PathPoint{
				{X: 50, Y: 10, InX: 30, InY: 10, OutX: 70, OutY: 10},
				{X: 90, Y: 50, InX: 90, InY: 30, OutX: 90, OutY: 70},
				{X: 50, Y: 90, InX: 70, InY: 90, OutX: 30, OutY: 90},
				{X: 10, Y: 50, InX: 10, InY: 70, OutX: 10, OutY: 30},
			},
		}},
	}

	r.ResetPath()
	r.FillColor(agglib.NewColor(0, 255, 0, 255))
	r.NoLine()
	applyPathToAgg2D(r, &curved)
	r.DrawPath(agglib.FillOnly)

	// Should render without panic and fill a non-trivial area.
	filled := countFilledPixels(buf)
	if filled < 1000 {
		t.Errorf("curved path should fill a significant area, got %d pixels", filled)
	}

	// Center should be filled.
	if pixelAlpha(buf, 100, 50, 50) == 0 {
		t.Error("center pixel (50,50) should be filled for curved shape")
	}
}

// --- Compound path (donut) with even-odd fill ---

func TestApplyPathToAgg2DCompound(t *testing.T) {
	r, buf := makeTestRenderer(100, 100)

	// Outer square (CW): 5,5 → 95,5 → 95,95 → 5,95
	// Inner square (CCW): 30,30 → 30,70 → 70,70 → 70,30
	donut := Path{
		Subpaths: []Subpath{
			{
				Closed: true,
				Points: []PathPoint{
					{X: 5, Y: 5, InX: 5, InY: 5, OutX: 5, OutY: 5},
					{X: 95, Y: 5, InX: 95, InY: 5, OutX: 95, OutY: 5},
					{X: 95, Y: 95, InX: 95, InY: 95, OutX: 95, OutY: 95},
					{X: 5, Y: 95, InX: 5, InY: 95, OutX: 5, OutY: 95},
				},
			},
			{
				Closed: true,
				Points: []PathPoint{
					// Wound in opposite direction for even-odd hole.
					{X: 30, Y: 30, InX: 30, InY: 30, OutX: 30, OutY: 30},
					{X: 30, Y: 70, InX: 30, InY: 70, OutX: 30, OutY: 70},
					{X: 70, Y: 70, InX: 70, InY: 70, OutX: 70, OutY: 70},
					{X: 70, Y: 30, InX: 70, InY: 30, OutX: 70, OutY: 30},
				},
			},
		},
	}

	r.FillEvenOdd(true)
	r.ResetPath()
	r.FillColor(agglib.NewColor(0, 0, 255, 255))
	r.NoLine()
	applyPathToAgg2D(r, &donut)
	r.DrawPath(agglib.FillOnly)

	// Outer region should be filled.
	if pixelAlpha(buf, 100, 10, 10) == 0 {
		t.Error("outer pixel (10,10) should be filled")
	}
	// Inner (hole) region should be empty.
	if pixelAlpha(buf, 100, 50, 50) != 0 {
		t.Errorf("inner pixel (50,50) should be empty (hole), alpha=%d", pixelAlpha(buf, 100, 50, 50))
	}
}

// --- flattenSubpathToPolyline ---

func TestFlattenSubpathToPolyline(t *testing.T) {
	// Straight-line triangle (closed): 3 points, no curves.
	tri := Subpath{
		Closed: true,
		Points: []PathPoint{
			{X: 0, Y: 0, InX: 0, InY: 0, OutX: 0, OutY: 0},
			{X: 10, Y: 0, InX: 10, InY: 0, OutX: 10, OutY: 0},
			{X: 5, Y: 10, InX: 5, InY: 10, OutX: 5, OutY: 10},
		},
	}

	pts := flattenSubpathToPolyline(&tri, 16)
	// First point + 2 segments + closing segment = 4 points.
	if len(pts) != 4 {
		t.Errorf("triangle: expected 4 points, got %d", len(pts))
	}

	// Verify first and last points.
	if pts[0] != [2]float64{0, 0} {
		t.Errorf("first point: got %v, want [0,0]", pts[0])
	}
	if pts[len(pts)-1] != [2]float64{0, 0} {
		t.Errorf("last point: got %v, want [0,0]", pts[len(pts)-1])
	}
}

func TestFlattenSubpathToPolylineCurved(t *testing.T) {
	// Two-point open curved segment.
	curved := Subpath{
		Closed: false,
		Points: []PathPoint{
			{X: 0, Y: 0, InX: 0, InY: 0, OutX: 5, OutY: 10},
			{X: 20, Y: 0, InX: 15, InY: 10, OutX: 20, OutY: 0},
		},
	}

	pts := flattenSubpathToPolyline(&curved, 8)
	// First point + 8 curve samples = 9 points.
	if len(pts) != 9 {
		t.Errorf("curved: expected 9 points, got %d", len(pts))
	}

	// First point is the start anchor.
	if pts[0] != [2]float64{0, 0} {
		t.Errorf("first point: got %v, want [0,0]", pts[0])
	}
	// Last point is the end anchor.
	if pts[len(pts)-1] != [2]float64{20, 0} {
		t.Errorf("last point: got %v, want [20,0]", pts[len(pts)-1])
	}
}

func TestFlattenSubpathToPolylineEmpty(t *testing.T) {
	empty := Subpath{Points: nil}
	pts := flattenSubpathToPolyline(&empty, 16)
	if pts != nil {
		t.Errorf("empty subpath should return nil, got %v", pts)
	}
}

// --- Edge cases ---

func TestApplyPathToAgg2DNil(t *testing.T) {
	r, _ := makeTestRenderer(10, 10)
	r.ResetPath()
	// Should not panic.
	applyPathToAgg2D(r, nil)
}

func TestApplyPathToAgg2DEmptySubpath(t *testing.T) {
	r, _ := makeTestRenderer(10, 10)
	r.ResetPath()
	p := &Path{Subpaths: []Subpath{{Points: nil}}}
	// Should not panic.
	applyPathToAgg2D(r, p)
}

func TestApplyPathToAgg2DSinglePoint(t *testing.T) {
	r, _ := makeTestRenderer(10, 10)
	r.ResetPath()
	p := &Path{Subpaths: []Subpath{{
		Closed: true,
		Points: []PathPoint{{X: 5, Y: 5, InX: 5, InY: 5, OutX: 5, OutY: 5}},
	}}}
	// Single point should not panic (no segments to emit).
	applyPathToAgg2D(r, p)
}
