// Package engine — brush dab rasterizer (Phase 4.1).
package engine

import (
	"math"

	agglib "github.com/MeKo-Christian/agg_go"
)

// BrushParams describes one brush dab's visual properties.
type BrushParams struct {
	Size      float64  `json:"size"`                // Diameter in document pixels
	Hardness  float64  `json:"hardness"`            // 0.0 (soft/feathered) – 1.0 (hard edge)
	Flow      float64  `json:"flow"`                // Per-dab alpha multiplier, 0–1
	Color     [4]uint8 `json:"color"`               // RGBA paint color
	BlendMode string   `json:"blendMode,omitempty"` // AGG blend mode string, e.g. "multiply", "screen"
}

// PaintDab renders a single brush dab centred at (cx, cy) in document space
// onto the given PixelLayer. The layer buffer is modified in place.
// cx/cy are in document coordinates; the layer's Bounds offset is subtracted.
// Sub-pixel placement is achieved via an AGG affine translate transform so the
// shape and gradient are specified at the origin and moved to the exact
// fractional-pixel position.
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

	// Apply blend mode (defaults to normal src-over when empty).
	if p.BlendMode != "" {
		renderer.BlendMode(agglib.StringToBlendMode(p.BlendMode))
	}

	// Sub-pixel placement: translate the rendering origin to the exact
	// fractional-pixel dab centre; shape and gradient are defined at (0, 0).
	renderer.Translate(lx, ly)

	renderer.ResetPath()
	renderer.AddEllipse(0, 0, radius, radius, agglib.CCW)

	if p.Hardness >= 1.0 {
		// Hard edge: solid fill; AGG provides sub-pixel AA at the ellipse boundary.
		alpha := uint8(float64(p.Color[3]) * flow)
		c := agglib.NewColor(p.Color[0], p.Color[1], p.Color[2], alpha)
		renderer.FillColor(c)
		renderer.DrawPath(agglib.FillOnly)
		return
	}

	// Soft edge: radial gradient from opaque centre to transparent edge.
	// Shape defined at origin; transform carries the centre to (lx, ly).
	centerAlpha := uint8(float64(p.Color[3]) * flow)
	c1 := agglib.NewColor(p.Color[0], p.Color[1], p.Color[2], centerAlpha)
	c2 := agglib.NewColor(p.Color[0], p.Color[1], p.Color[2], 0)
	renderer.FillRadialGradient(0, 0, radius, c1, c2, 1.0)
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
// Dab positions are interpolated along a Catmull-Rom spline through the raw
// input points, giving smooth curves even when pointer events arrive sparsely.
type brushStrokeState struct {
	prevPrev    [2]float64 // P0 control point for CR (point before prev)
	prev        [2]float64 // P1 — previous raw input, start of current CR segment
	hasPrev     bool
	hasPrevPrev bool
	travelled   float64 // carry-over distance since the last dab [0, interval)
}

// AddPoint takes a new pointer position and returns document-space positions
// where dabs should be placed. spacing is the dab interval as a fraction of
// brush diameter (e.g. 0.25 = every 25% of size). Always places a dab on
// the first call.
//
// Subsequent calls interpolate along a Catmull-Rom spline: the segment from
// prev→pt is smoothed using prevPrev as the before-tangent and an extrapolated
// after-tangent (2·pt − prev), so the stroke curves through every input point.
func (s *brushStrokeState) AddPoint(x, y, spacing, size float64) [][2]float64 {
	pt := [2]float64{x, y}

	// First point: always plant a dab at the exact start position.
	if !s.hasPrev {
		s.prev = pt
		s.hasPrev = true
		return [][2]float64{pt}
	}

	p1 := s.prev // segment start
	p2 := pt     // segment end

	// P0: the control point before p1.
	// Use the recorded prevPrev if available; otherwise reflect p2 around p1
	// so the tangent at the stroke start is directed away from p2.
	var p0 [2]float64
	if s.hasPrevPrev {
		p0 = s.prevPrev
	} else {
		p0 = [2]float64{2*p1[0] - p2[0], 2*p1[1] - p2[1]}
	}

	// P3: extrapolated "next" point used as the after-tangent control.
	// Extrapolating keeps the curve tangent at p2 pointed toward p3.
	p3 := [2]float64{2*p2[0] - p1[0], 2*p2[1] - p1[1]}

	positions := s.sampleCR(p0, p1, p2, p3, spacing, size)

	// Shift history.
	s.prevPrev = s.prev
	s.hasPrevPrev = true
	s.prev = pt

	return positions
}

// sampleCR places dabs along the Catmull-Rom segment from p1 to p2 (using p0
// and p3 as tangent controls) and returns their document-space positions.
// It respects the carry-over distance in s.travelled and updates it.
func (s *brushStrokeState) sampleCR(p0, p1, p2, p3 [2]float64, spacing, size float64) [][2]float64 {
	interval := spacing * size
	if interval < 1.0 {
		interval = 1.0
	}

	// Build an arc-length table by sampling the CR curve at nSamples steps.
	const nSamples = 32
	var arcLen [nSamples + 1]float64
	var crPts [nSamples + 1][2]float64
	crPts[0] = p1
	for i := 1; i <= nSamples; i++ {
		t := float64(i) / float64(nSamples)
		crPts[i] = catmullRomPoint(p0, p1, p2, p3, t)
		dx := crPts[i][0] - crPts[i-1][0]
		dy := crPts[i][1] - crPts[i-1][1]
		arcLen[i] = arcLen[i-1] + math.Sqrt(dx*dx+dy*dy)
	}
	totalLen := arcLen[nSamples]
	if totalLen == 0 {
		return nil
	}

	// prevTravelled is the carry-over from previous segments.
	prevTravelled := s.travelled
	s.travelled += totalLen

	// First dab in this segment is at arc-length offset = interval - prevTravelled.
	// This ensures even spacing with carry-over across segment boundaries.
	offset := interval - prevTravelled
	// Guard against floating-point drift pushing offset ≤ 0.
	for offset <= 0 {
		offset += interval
	}

	var positions [][2]float64
	for offset <= totalLen {
		pt := crArcLengthLookup(arcLen[:], crPts[:], offset)
		positions = append(positions, pt)
		offset += interval
	}

	// Correct s.travelled to reflect the distance since the last placed dab.
	if len(positions) > 0 {
		lastDabOffset := interval - prevTravelled + float64(len(positions)-1)*interval
		s.travelled = totalLen - lastDabOffset
	}

	return positions
}

// catmullRomPoint evaluates the standard uniform Catmull-Rom spline at parameter
// t ∈ [0, 1] for the segment p1→p2 with tangent controls p0 and p3.
func catmullRomPoint(p0, p1, p2, p3 [2]float64, t float64) [2]float64 {
	t2, t3 := t*t, t*t*t
	return [2]float64{
		0.5 * ((2 * p1[0]) + (-p0[0]+p2[0])*t + (2*p0[0]-5*p1[0]+4*p2[0]-p3[0])*t2 + (-p0[0]+3*p1[0]-3*p2[0]+p3[0])*t3),
		0.5 * ((2 * p1[1]) + (-p0[1]+p2[1])*t + (2*p0[1]-5*p1[1]+4*p2[1]-p3[1])*t2 + (-p0[1]+3*p1[1]-3*p2[1]+p3[1])*t3),
	}
}

// crArcLengthLookup returns the point on the sampled CR curve at the given
// arc-length s by binary-search into arcLen and linear interpolation.
func crArcLengthLookup(arcLen []float64, crPts [][2]float64, s float64) [2]float64 {
	n := len(arcLen) - 1
	lo, hi := 0, n
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if arcLen[mid] <= s {
			lo = mid
		} else {
			hi = mid
		}
	}
	segLen := arcLen[hi] - arcLen[lo]
	if segLen <= 0 {
		return crPts[lo]
	}
	frac := (s - arcLen[lo]) / segLen
	return [2]float64{
		crPts[lo][0] + (crPts[hi][0]-crPts[lo][0])*frac,
		crPts[lo][1] + (crPts[hi][1]-crPts[lo][1])*frac,
	}
}
