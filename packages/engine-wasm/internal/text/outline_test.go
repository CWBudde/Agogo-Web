package text

import (
	"math"
	"testing"
)

// bboxSink is a PathSink that records the bounding box of every emitted
// point, including control points. For glyphs like 'A' the control points
// stay close to the ink, so this is a good-enough ink-bounds approximation
// for orientation and size assertions.
type bboxSink struct {
	minX, minY, maxX, maxY float64
	moves, closes, points  int
}

func newBBoxSink() *bboxSink {
	return &bboxSink{
		minX: math.Inf(1), minY: math.Inf(1),
		maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
}

func (s *bboxSink) add(x, y float64) {
	s.points++
	s.minX = math.Min(s.minX, x)
	s.minY = math.Min(s.minY, y)
	s.maxX = math.Max(s.maxX, x)
	s.maxY = math.Max(s.maxY, y)
}

func (s *bboxSink) MoveTo(x, y float64) { s.moves++; s.add(x, y) }
func (s *bboxSink) LineTo(x, y float64) { s.add(x, y) }
func (s *bboxSink) QuadTo(cx, cy, x, y float64) {
	s.add(cx, cy)
	s.add(x, y)
}

func (s *bboxSink) CubeTo(c1x, c1y, c2x, c2y, x, y float64) {
	s.add(c1x, c1y)
	s.add(c2x, c2y)
	s.add(x, y)
}
func (s *bboxSink) ClosePath() { s.closes++ }

// TestOrientationPin pins the Y orientation of AppendGlyphOutline to
// Photoshop-style y-down document space: for an uppercase 'A' placed at a
// baseline origin, all glyph ink must lie ABOVE the baseline (smaller Y),
// with the top of the glyph roughly one cap height above the origin.
func TestOrientationPin(t *testing.T) {
	const (
		size    = 48.0
		originX = 100.0
		originY = 100.0
	)
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	if face == nil {
		t.Fatal("Resolve returned nil for DejaVu Sans regular")
	}

	glyph, err := face.GlyphIndex('A')
	if err != nil {
		t.Fatalf("GlyphIndex('A'): %v", err)
	}
	if glyph == 0 {
		t.Fatal("GlyphIndex('A') returned .notdef")
	}

	sink := newBBoxSink()
	if err := face.AppendGlyphOutline(glyph, size, originX, originY, sink); err != nil {
		t.Fatalf("AppendGlyphOutline: %v", err)
	}
	if sink.points == 0 {
		t.Fatal("no outline points emitted for 'A'")
	}
	if sink.moves == 0 || sink.closes == 0 {
		t.Fatalf("expected contours to be opened and closed, got moves=%d closes=%d", sink.moves, sink.closes)
	}
	if sink.closes != sink.moves {
		t.Fatalf("every MoveTo must be balanced by a ClosePath, got moves=%d closes=%d", sink.moves, sink.closes)
	}

	const eps = 0.5
	if sink.maxY >= originY+eps {
		t.Errorf("glyph ink extends below baseline: maxY=%.3f, want < %.3f (y-down space, ink above baseline)", sink.maxY, originY+eps)
	}

	capHeight := face.Metrics(size).CapHeight
	if capHeight <= 0 {
		t.Fatalf("CapHeight = %.3f, want > 0", capHeight)
	}
	wantMinY := originY - capHeight
	tol := 0.2 * capHeight
	if math.Abs(sink.minY-wantMinY) > tol {
		t.Errorf("glyph top minY=%.3f, want ~%.3f (origin - capHeight) within %.3f", sink.minY, wantMinY, tol)
	}
}

// TestAppendGlyphOutlineOriginOffset verifies the outline translates with the
// origin: same glyph at two origins yields bboxes differing exactly by the
// origin delta.
func TestAppendGlyphOutlineOriginOffset(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	glyph, err := face.GlyphIndex('g')
	if err != nil {
		t.Fatalf("GlyphIndex('g'): %v", err)
	}

	a := newBBoxSink()
	if err := face.AppendGlyphOutline(glyph, 32, 0, 0, a); err != nil {
		t.Fatalf("AppendGlyphOutline: %v", err)
	}
	b := newBBoxSink()
	if err := face.AppendGlyphOutline(glyph, 32, 10, -7, b); err != nil {
		t.Fatalf("AppendGlyphOutline: %v", err)
	}

	const tol = 1e-9
	if math.Abs((b.minX-a.minX)-10) > tol || math.Abs((b.minY-a.minY)-(-7)) > tol ||
		math.Abs((b.maxX-a.maxX)-10) > tol || math.Abs((b.maxY-a.maxY)-(-7)) > tol {
		t.Errorf("outline did not translate with origin: a=[%f %f %f %f] b=[%f %f %f %f]",
			a.minX, a.minY, a.maxX, a.maxY, b.minX, b.minY, b.maxX, b.maxY)
	}

	// A descender glyph must have ink below the baseline (maxY > originY).
	if a.maxY <= 0 {
		t.Errorf("'g' descender should extend below baseline: maxY=%.3f, want > 0", a.maxY)
	}
}

// quadPoint evaluates the quadratic Bezier (p0, q, p2) at t.
func quadPoint(p0x, p0y, qx, qy, p2x, p2y, t float64) (float64, float64) {
	u := 1 - t
	x := u*u*p0x + 2*u*t*qx + t*t*p2x
	y := u*u*p0y + 2*u*t*qy + t*t*p2y
	return x, y
}

// cubicPoint evaluates the cubic Bezier (p0, c1, c2, p3) at t.
func cubicPoint(p0x, p0y, c1x, c1y, c2x, c2y, p3x, p3y, t float64) (float64, float64) {
	u := 1 - t
	x := u*u*u*p0x + 3*u*u*t*c1x + 3*u*t*t*c2x + t*t*t*p3x
	y := u*u*u*p0y + 3*u*u*t*c1y + 3*u*t*t*c2y + t*t*t*p3y
	return x, y
}

// TestQuadToCubic verifies the degree elevation is exact: the elevated cubic
// traces the same curve as the source quadratic.
func TestQuadToCubic(t *testing.T) {
	tests := []struct {
		name                       string
		p0x, p0y, qx, qy, p2x, p2y float64
	}{
		{"unit", 0, 0, 0.5, 1, 1, 0},
		{"skewed", -3, 7, 12, -4, 5, 9},
		{"degenerate collinear", 0, 0, 1, 1, 2, 2},
		{"glyph scale", 103.25, 61.5, 110.75, 61.5, 114.5, 55.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c1x, c1y, c2x, c2y := QuadToCubic(tt.p0x, tt.p0y, tt.qx, tt.qy, tt.p2x, tt.p2y)
			for _, tv := range []float64{0, 0.25, 0.5, 0.75, 1} {
				qx, qy := quadPoint(tt.p0x, tt.p0y, tt.qx, tt.qy, tt.p2x, tt.p2y, tv)
				cx, cy := cubicPoint(tt.p0x, tt.p0y, c1x, c1y, c2x, c2y, tt.p2x, tt.p2y, tv)
				if math.Abs(qx-cx) > 1e-9 || math.Abs(qy-cy) > 1e-9 {
					t.Errorf("t=%.2f: quad=(%.12f,%.12f) cubic=(%.12f,%.12f)", tv, qx, qy, cx, cy)
				}
			}
		})
	}
}
