package engine

import (
	agglib "github.com/cwbudde/agg_go"
)

// agg2DPathSink adapts an Agg2D path builder to the text.PathSink interface
// so glyph outlines stream straight into the engine's agg path (all pixel
// work stays inside agg_go, per the project's rendering constraint).
type agg2DPathSink struct{ r *agglib.Agg2D }

func (s agg2DPathSink) MoveTo(x, y float64) { s.r.MoveTo(x, y) }
func (s agg2DPathSink) LineTo(x, y float64) { s.r.LineTo(x, y) }
func (s agg2DPathSink) QuadTo(cx, cy, x, y float64) {
	s.r.QuadricCurveTo(cx, cy, x, y)
}

func (s agg2DPathSink) CubeTo(c1x, c1y, c2x, c2y, x, y float64) {
	s.r.CubicCurveTo(c1x, c1y, c2x, c2y, x, y)
}
func (s agg2DPathSink) ClosePath() { s.r.ClosePolygon() }

// rasterizeTextLayer lays out and rasterizes a TextLayer through the sfnt
// text engine, MUTATING the layer: it refreshes layer.CachedRaster (a
// BOUNDS-LOCAL RGBA buffer of Bounds.W × Bounds.H × 4 bytes whose pixel (0,0)
// corresponds to document pixel (Bounds.X, Bounds.Y)) and, for point text,
// recomputes layer.Bounds as the tight ink box around the anchor. Area text
// keeps its user frame as Bounds (wrapping width authority) with the anchor
// synced to the frame origin; a degenerate area frame clears the raster.
//
// The glyph offset inside the raster is (AnchorX - Bounds.X, AnchorY -
// Bounds.Y): layout runs in anchor-relative space, so translating a layer
// (bounds AND anchor together) re-rasterizes byte-identically.
func rasterizeTextLayer(layer *TextLayer) error {
	// Legacy migration: derive the anchor from the bounds origin exactly once
	// for layers that predate the anchor model (see model.TextLayer.AnchorSet).
	if !layer.AnchorSet {
		layer.AnchorX = float64(layer.Bounds.X)
		layer.AnchorY = float64(layer.Bounds.Y)
		layer.AnchorSet = true
	}
	if layer.TextType == "area" {
		// Area text: the frame is authoritative; the anchor follows its origin.
		layer.AnchorX = float64(layer.Bounds.X)
		layer.AnchorY = float64(layer.Bounds.Y)
	}

	layout := layoutTextLayer(layer)
	if layer.TextType != "area" {
		layer.Bounds = computePointTextBounds(layer, layout)
	}

	rasterW := layer.Bounds.W
	rasterH := layer.Bounds.H
	if rasterW <= 0 || rasterH <= 0 {
		layer.CachedRaster = nil
		return nil
	}
	stride := rasterW * 4
	buf := make([]byte, stride*rasterH)

	hasDecorations := (layer.Underline || layer.Strikethrough) && len(layout.spans) > 0
	if len(layout.glyphs) == 0 && !hasDecorations {
		layer.CachedRaster = buf
		return nil
	}

	r := agglib.NewAgg2D()
	r.Attach(buf, rasterW, rasterH, stride)
	r.ResetTransformations()
	applyTextAntiAlias(r, layer.AntiAlias)

	c := layer.Color
	r.FillColor(agglib.NewColor(c[0], c[1], c[2], c[3]))
	r.NoLine()
	// Non-zero winding: TrueType outlines encode counters via contour
	// direction, which even-odd would fill incorrectly for nested contours.
	r.FillEvenOdd(false)

	// Anchor-relative layout space → bounds-local raster space.
	offX := layer.AnchorX - float64(layer.Bounds.X)
	offY := layer.AnchorY - float64(layer.Bounds.Y)

	// Batch every glyph outline into a single path and fill once.
	sink := agg2DPathSink{r: r}
	r.ResetPath()
	for _, g := range layout.glyphs {
		// A glyph whose outline fails to load is skipped rather than
		// aborting the whole layer (.notdef and friends still render).
		_ = layout.face.AppendGlyphOutline(g.glyph, g.size, offX+g.x, offY+g.baselineY, sink)
	}
	r.DrawPath(agglib.FillOnly)

	if hasDecorations {
		thickness := layout.underlineThickness()
		r.ResetPath()
		for _, span := range layout.spans {
			if span.x1 <= span.x0 {
				continue
			}
			if layer.Underline {
				// Post-table position: top of the bar sits UnderlinePosition
				// below the baseline (positive-down convention).
				top := span.baselineY + layout.metrics.UnderlinePosition
				addRectPath(r, offX+span.x0, offY+top, offX+span.x1, offY+top+thickness)
			}
			if layer.Strikethrough {
				// Centered on half the x-height above the baseline.
				mid := span.baselineY - layout.metrics.XHeight/2
				top := mid - thickness/2
				addRectPath(r, offX+span.x0, offY+top, offX+span.x1, offY+top+thickness)
			}
		}
		r.DrawPath(agglib.FillOnly)
	}

	layer.CachedRaster = buf
	return nil
}

// applyTextAntiAlias maps the TextLayer.AntiAlias mode onto the Agg2D
// pipeline: "none" disables anti-aliasing entirely; "sharp"/"crisp" use a
// steeper gamma for higher-contrast edges; everything else ("smooth", empty,
// unknown) keeps the default anti-aliased pipeline.
func applyTextAntiAlias(r *agglib.Agg2D, mode string) {
	switch mode {
	case "none":
		r.SetAntiAliased(false)
	case "sharp", "crisp":
		r.SetAntiAliased(true)
		r.AntiAliasGamma(1.8)
	default:
		r.SetAntiAliased(true)
	}
}

// addRectPath appends an axis-aligned rectangle to the current Agg2D path.
func addRectPath(r *agglib.Agg2D, x0, y0, x1, y1 float64) {
	r.MoveTo(x0, y0)
	r.LineTo(x1, y0)
	r.LineTo(x1, y1)
	r.LineTo(x0, y1)
	r.ClosePolygon()
}
