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

// applyPressure scales brush Size and Flow by the pointer pressure value (0–1).
// At pressure=1.0 the brush is full size; at pressure=0.0 it is 50% size.
func applyPressure(p BrushParams, pressure float64) BrushParams {
	if pressure <= 0 {
		pressure = 0.5
	}
	p.Size = p.Size * (0.5 + 0.5*pressure)
	p.Flow = clampFloat(p.Flow*pressure, 0, 1)
	return p
}

// expandDirty grows the stroke's dirty bounding box to include the dab at (cx, cy).
// cx/cy are in document space.
func (s *activePaintStroke) expandDirty(layer *PixelLayer, cx, cy, size float64) {
	r := int(math.Ceil(size*0.5)) + 2 // +2 for AA fringe
	lx := int(cx) - layer.Bounds.X - r
	ly := int(cy) - layer.Bounds.Y - r
	rx := int(cx) - layer.Bounds.X + r
	ry := int(cy) - layer.Bounds.Y + r

	if lx < 0 {
		lx = 0
	}
	if ly < 0 {
		ly = 0
	}
	if rx > layer.Bounds.W {
		rx = layer.Bounds.W
	}
	if ry > layer.Bounds.H {
		ry = layer.Bounds.H
	}

	if !s.hasDirty {
		s.dirtyMin = [2]int{lx, ly}
		s.dirtyMax = [2]int{rx, ry}
		s.hasDirty = true
		return
	}
	if lx < s.dirtyMin[0] {
		s.dirtyMin[0] = lx
	}
	if ly < s.dirtyMin[1] {
		s.dirtyMin[1] = ly
	}
	if rx > s.dirtyMax[0] {
		s.dirtyMax[0] = rx
	}
	if ry > s.dirtyMax[1] {
		s.dirtyMax[1] = ry
	}
}

// findPixelLayer searches the document's layer tree for a PixelLayer with the given ID.
// Returns nil if not found or if the matching layer is not a PixelLayer.
func findPixelLayer(doc *Document, layerID string) *PixelLayer {
	if doc == nil || layerID == "" {
		return nil
	}
	var found *PixelLayer
	walkLayers(doc.LayerRoot, func(n LayerNode) bool {
		if n.ID() == layerID {
			if pl, ok := n.(*PixelLayer); ok {
				found = pl
				return false
			}
		}
		return true
	})
	return found
}

// walkLayers calls fn for each LayerNode in the tree (depth-first, pre-order).
// If fn returns false the walk stops early.
func walkLayers(root *GroupLayer, fn func(LayerNode) bool) {
	if root == nil {
		return
	}
	for _, child := range root.Children() {
		if !fn(child) {
			return
		}
		if g, ok := child.(*GroupLayer); ok {
			walkLayers(g, fn)
		}
	}
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
