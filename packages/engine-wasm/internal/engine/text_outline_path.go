package engine

import (
	text "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/text"
)

// subpathPathSink adapts text.PathSink to the engine's Path model, collecting
// glyph contours as CLOSED fill subpaths in document space.
//
// CRITICAL handle convention: hasNonTrivialHandles (path_agg.go) treats
// zero-value handles as curve control points at (0,0), so EVERY PathPoint is
// created with In/Out initialized to its anchor coordinates (via corner) and
// only genuine Bezier control points overwrite them. Quadratic segments are
// elevated to exact cubics with text.QuadToCubic because the PathPoint model
// is cubic-only.
type subpathPathSink struct {
	subpaths []Subpath
	current  []PathPoint
}

func (s *subpathPathSink) MoveTo(x, y float64) {
	// AppendGlyphOutline closes every contour before a new MoveTo, so this
	// flush is defensive only.
	s.finalize()
	s.current = append(s.current, corner(x, y))
}

func (s *subpathPathSink) LineTo(x, y float64) {
	if len(s.current) == 0 {
		s.MoveTo(x, y)
		return
	}
	s.current = append(s.current, corner(x, y))
}

func (s *subpathPathSink) QuadTo(cx, cy, x, y float64) {
	if len(s.current) == 0 {
		s.MoveTo(x, y)
		return
	}
	prev := &s.current[len(s.current)-1]
	c1x, c1y, c2x, c2y := text.QuadToCubic(prev.X, prev.Y, cx, cy, x, y)
	prev.OutX, prev.OutY = c1x, c1y
	pt := corner(x, y)
	pt.InX, pt.InY = c2x, c2y
	s.current = append(s.current, pt)
}

func (s *subpathPathSink) CubeTo(c1x, c1y, c2x, c2y, x, y float64) {
	if len(s.current) == 0 {
		s.MoveTo(x, y)
		return
	}
	prev := &s.current[len(s.current)-1]
	prev.OutX, prev.OutY = c1x, c1y
	pt := corner(x, y)
	pt.InX, pt.InY = c2x, c2y
	s.current = append(s.current, pt)
}

func (s *subpathPathSink) ClosePath() { s.finalize() }

// finalize flushes the contour under construction as a closed subpath (glyph
// contours are always fill contours). Font contours typically end with a
// segment returning to the start point; that duplicate endpoint is folded
// into the first point — carrying its incoming handle along — so the closing
// segment emitted by applySubpathToAgg2D preserves the final curve.
// Degenerate contours (< 3 distinct points) enclose no area and are dropped.
func (s *subpathPathSink) finalize() {
	pts := s.current
	s.current = nil
	if n := len(pts); n >= 2 && pts[n-1].X == pts[0].X && pts[n-1].Y == pts[0].Y {
		pts[0].InX, pts[0].InY = pts[n-1].InX, pts[n-1].InY
		pts = pts[:n-1]
	}
	if len(pts) < 3 {
		return
	}
	s.subpaths = append(s.subpaths, Subpath{Closed: true, Points: pts})
}

// buildTextOutlinePath converts a TextLayer's glyph ink into closed fill
// contours in document space (Type > Create Outlines). It reuses the exact
// raster pipeline: layoutTextLayer for wrapping/alignment/caps and the same
// registry-resolved face for outlines, so the converted path lands where the
// rasterized text was drawn. Underline/strikethrough decoration bars are not
// converted — only glyph outlines become path geometry.
func buildTextOutlinePath(layer *TextLayer) *Path {
	if layer == nil || layer.Text == "" {
		return nil
	}

	// Layout runs in anchor-relative space; legacy layers that predate the
	// anchor model derive it from the bounds origin (mirrors
	// rasterizeTextLayer, but without mutating the layer).
	anchorX, anchorY := layer.AnchorX, layer.AnchorY
	if !layer.AnchorSet {
		anchorX = float64(layer.Bounds.X)
		anchorY = float64(layer.Bounds.Y)
	}

	layout := layoutTextLayer(layer)
	sink := &subpathPathSink{}
	for _, g := range layout.glyphs {
		// A glyph whose outline fails to load is skipped rather than
		// aborting the conversion (same policy as rasterizeTextLayer).
		_ = layout.face.AppendGlyphOutline(g.glyph, g.size, anchorX+g.x, anchorY+g.baselineY, sink)
	}

	if len(sink.subpaths) == 0 {
		return nil
	}
	return &Path{Subpaths: sink.subpaths}
}
