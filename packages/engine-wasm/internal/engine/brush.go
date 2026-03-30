// Package engine — brush dab rasterizer (Phase 4.1).
package engine

import (
	"math"

	agglib "github.com/MeKo-Christian/agg_go"
)

// BrushParams describes one brush dab's visual properties.
type BrushParams struct {
	Size     float64  // Diameter in document pixels
	Hardness float64  // 0.0 (soft/feathered) – 1.0 (hard edge)
	Flow     float64  // Per-dab alpha multiplier, 0–1
	Color    [4]uint8 // RGBA paint color
}

// PaintDab renders a single brush dab centred at (cx, cy) in document space
// onto the given PixelLayer. The layer buffer is modified in place.
// cx/cy are in document coordinates; the layer's Bounds offset is subtracted.
func PaintDab(layer *PixelLayer, cx, cy float64, p BrushParams) {
	w := layer.Bounds.W
	h := layer.Bounds.H
	if w <= 0 || h <= 0 {
		return
	}

	// Convert to layer-local coordinates.
	lx := cx - float64(layer.Bounds.X)
	ly := cy - float64(layer.Bounds.Y)

	radius := p.Size * 0.5
	if radius < 0.5 {
		radius = 0.5
	}
	flow := clampFloat(p.Flow, 0, 1)

	renderer := agglib.NewAgg2D()
	renderer.Attach(layer.Pixels, w, h, w*4)
	renderer.NoLine()
	renderer.ResetPath()
	renderer.AddEllipse(lx, ly, radius, radius, agglib.CCW)

	if p.Hardness >= 1.0 {
		// Hard edge: solid fill; AGG provides sub-pixel AA at the ellipse boundary.
		alpha := uint8(float64(p.Color[3]) * flow)
		c := agglib.NewColor(p.Color[0], p.Color[1], p.Color[2], alpha)
		renderer.FillColor(c)
		renderer.DrawPath(agglib.FillOnly)
		return
	}

	// Soft edge: radial gradient from opaque centre to transparent edge.
	// FillRadialGradient(x, y, r, c1, c2, profile):
	//   r       = outer radius (gradient reaches 0 here)
	//   c1      = colour at centre
	//   c2      = colour at outer radius (transparent)
	//   profile = 1.0 (linear falloff)
	centerAlpha := uint8(float64(p.Color[3]) * flow)
	c1 := agglib.NewColor(p.Color[0], p.Color[1], p.Color[2], centerAlpha)
	c2 := agglib.NewColor(p.Color[0], p.Color[1], p.Color[2], 0)
	renderer.FillRadialGradient(lx, ly, radius, c1, c2, 1.0)
	renderer.DrawPath(agglib.FillOnly)
}

// brushStrokeState tracks an in-progress paint stroke for dab spacing.
type brushStrokeState struct {
	lastX     float64
	lastY     float64
	travelled float64 // distance accumulated since last dab
	hasDab    bool
}

// AddPoint takes a new pointer position and returns document-space positions
// where dabs should be placed. spacing is the dab interval as a fraction of
// brush diameter (e.g. 0.25 = every 25% of size). Always places a dab on
// the first call.
func (s *brushStrokeState) AddPoint(x, y, spacing, size float64) [][2]float64 {
	if !s.hasDab {
		s.lastX = x
		s.lastY = y
		s.hasDab = true
		return [][2]float64{{x, y}}
	}

	interval := spacing * size
	if interval < 1.0 {
		interval = 1.0
	}

	dx := x - s.lastX
	dy := y - s.lastY
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist == 0 {
		return nil
	}

	s.travelled += dist

	var positions [][2]float64
	for s.travelled >= interval {
		segOffset := dist - (s.travelled - interval)
		if segOffset < 0 {
			segOffset = 0
		}
		t := segOffset / dist
		positions = append(positions, [2]float64{
			s.lastX + dx*t,
			s.lastY + dy*t,
		})
		s.travelled -= interval
	}

	s.lastX = x
	s.lastY = y
	return positions
}
