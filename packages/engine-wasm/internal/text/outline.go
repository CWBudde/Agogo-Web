package text

import (
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// PathSink receives glyph outline geometry in y-down document space. It is
// the package's only output channel: implementations adapt outlines to the
// engine's path model or straight into an agg_go path without this package
// importing either.
type PathSink interface {
	MoveTo(x, y float64)
	LineTo(x, y float64)
	QuadTo(cx, cy, x, y float64)
	CubeTo(c1x, c1y, c2x, c2y, x, y float64)
	ClosePath()
}

// AppendGlyphOutline emits the outline of a glyph rendered at size pixels
// per em, positioned with its baseline origin at (originX, originY), into
// sink.
//
// Coordinates are y-down document space: sfnt.Font.LoadGlyph already emits
// segments with the Y axis increasing down at ppem scale (ascenders have
// negative y), so segment coordinates are added to the origin unchanged and
// the ink of an 'A' lands above the baseline (y < originY). This
// orientation is pinned by TestOrientationPin.
//
// Every contour is explicitly closed: a ClosePath is emitted before each
// MoveTo except the first, and after the final segment. TrueType quadratic
// segments are forwarded as QuadTo; CFF cubic segments as CubeTo. Sinks
// that only support cubics can elevate quadratics with QuadToCubic.
func (f *Face) AppendGlyphOutline(glyph sfnt.GlyphIndex, size, originX, originY float64, sink PathSink) error {
	if size <= 0 {
		return nil
	}

	f.mu.Lock()
	segments, err := f.segmentsLocked(glyph, fixedFromFloat(size))
	f.mu.Unlock()
	if err != nil {
		return err
	}

	started := false
	for _, seg := range segments {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			if started {
				sink.ClosePath()
			}
			x, y := outlinePoint(seg.Args[0], originX, originY)
			sink.MoveTo(x, y)
			started = true
		case sfnt.SegmentOpLineTo:
			x, y := outlinePoint(seg.Args[0], originX, originY)
			sink.LineTo(x, y)
		case sfnt.SegmentOpQuadTo:
			cx, cy := outlinePoint(seg.Args[0], originX, originY)
			x, y := outlinePoint(seg.Args[1], originX, originY)
			sink.QuadTo(cx, cy, x, y)
		case sfnt.SegmentOpCubeTo:
			c1x, c1y := outlinePoint(seg.Args[0], originX, originY)
			c2x, c2y := outlinePoint(seg.Args[1], originX, originY)
			x, y := outlinePoint(seg.Args[2], originX, originY)
			sink.CubeTo(c1x, c1y, c2x, c2y, x, y)
		}
	}
	if started {
		sink.ClosePath()
	}
	return nil
}

// outlinePoint translates one y-down 26.6 segment coordinate into document
// space.
func outlinePoint(p fixed.Point26_6, originX, originY float64) (float64, float64) {
	return originX + floatFromFixed(p.X), originY + floatFromFixed(p.Y)
}

// QuadToCubic elevates the quadratic Bezier (p0, q, p2) to an exact cubic
// with the same endpoints: c1 = p0 + 2/3*(q-p0), c2 = p2 + 2/3*(q-p2). The
// elevated curve traces the identical path. Exported because the outline
// conversion in the engine's PathPoint model is cubic-only.
func QuadToCubic(p0x, p0y, qx, qy, p2x, p2y float64) (c1x, c1y, c2x, c2y float64) {
	c1x = p0x + 2.0/3.0*(qx-p0x)
	c1y = p0y + 2.0/3.0*(qy-p0y)
	c2x = p2x + 2.0/3.0*(qx-p2x)
	c2y = p2y + 2.0/3.0*(qy-p2y)
	return c1x, c1y, c2x, c2y
}
