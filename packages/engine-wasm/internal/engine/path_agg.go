// Package engine — Path ↔ AGG bridge: converts the engine's Path/Subpath model
// into AGG path commands for rendering via Agg2D.
package engine

import agglib "github.com/cwbudde/agg_go"

// applyPathToAgg2D emits MoveTo/LineTo/CubicCurveTo commands for a Path onto
// an Agg2D context. The caller is responsible for calling ResetPath() before
// and DrawPath() after.
func applyPathToAgg2D(agg *agglib.Agg2D, p *Path) {
	if p == nil {
		return
	}
	for i := range p.Subpaths {
		applySubpathToAgg2D(agg, &p.Subpaths[i])
	}
}

// applySubpathToAgg2D emits commands for a single subpath.
func applySubpathToAgg2D(agg *agglib.Agg2D, sp *Subpath) {
	if len(sp.Points) == 0 {
		return
	}

	agg.MoveTo(sp.Points[0].X, sp.Points[0].Y)

	for i := 1; i < len(sp.Points); i++ {
		emitSegment(agg, sp.Points[i-1], sp.Points[i])
	}

	if sp.Closed && len(sp.Points) > 1 {
		// Emit the closing segment from last → first (may be curved).
		last := sp.Points[len(sp.Points)-1]
		first := sp.Points[0]
		if hasNonTrivialHandles(last, first) {
			agg.CubicCurveTo(last.OutX, last.OutY, first.InX, first.InY, first.X, first.Y)
		}
		agg.ClosePolygon()
	}
}

// emitSegment draws a line or cubic bezier from prev to curr.
func emitSegment(agg *agglib.Agg2D, prev, curr PathPoint) {
	if hasNonTrivialHandles(prev, curr) {
		agg.CubicCurveTo(prev.OutX, prev.OutY, curr.InX, curr.InY, curr.X, curr.Y)
	} else {
		agg.LineTo(curr.X, curr.Y)
	}
}

// hasNonTrivialHandles returns true if the segment from prev to curr has
// Bezier control points that differ from the anchor positions (i.e., the
// segment is curved).
func hasNonTrivialHandles(prev, curr PathPoint) bool {
	return (prev.OutX != prev.X || prev.OutY != prev.Y) ||
		(curr.InX != curr.X || curr.InY != curr.Y)
}

// evaluateBezier returns a point on the cubic Bezier curve at parameter t ∈ [0,1].
// Used for generating polyline approximations for overlays.
func evaluateBezier(p0x, p0y, p1x, p1y, p2x, p2y, p3x, p3y, t float64) (float64, float64) {
	u := 1 - t
	uu := u * u
	uuu := uu * u
	tt := t * t
	ttt := tt * t

	x := uuu*p0x + 3*uu*t*p1x + 3*u*tt*p2x + ttt*p3x
	y := uuu*p0y + 3*uu*t*p1y + 3*u*tt*p2y + ttt*p3y
	return x, y
}

// flattenSubpathToPolyline converts a subpath to a polyline (series of x,y
// points) by evaluating Bezier curves at the given number of steps per curved
// segment. Straight segments produce a single output point.
// Used for overlay rendering, not for pixel rasterization (AGG does that).
func flattenSubpathToPolyline(sp *Subpath, stepsPerCurve int) [][2]float64 {
	if len(sp.Points) == 0 {
		return nil
	}
	if stepsPerCurve < 2 {
		stepsPerCurve = 16
	}

	pts := [][2]float64{{sp.Points[0].X, sp.Points[0].Y}}

	for i := 1; i < len(sp.Points); i++ {
		prev := sp.Points[i-1]
		curr := sp.Points[i]
		if hasNonTrivialHandles(prev, curr) {
			for s := 1; s <= stepsPerCurve; s++ {
				t := float64(s) / float64(stepsPerCurve)
				x, y := evaluateBezier(
					prev.X, prev.Y, prev.OutX, prev.OutY,
					curr.InX, curr.InY, curr.X, curr.Y, t,
				)
				pts = append(pts, [2]float64{x, y})
			}
		} else {
			pts = append(pts, [2]float64{curr.X, curr.Y})
		}
	}

	// Closing segment
	if sp.Closed && len(sp.Points) > 1 {
		last := sp.Points[len(sp.Points)-1]
		first := sp.Points[0]
		if hasNonTrivialHandles(last, first) {
			for s := 1; s <= stepsPerCurve; s++ {
				t := float64(s) / float64(stepsPerCurve)
				x, y := evaluateBezier(
					last.X, last.Y, last.OutX, last.OutY,
					first.InX, first.InY, first.X, first.Y, t,
				)
				pts = append(pts, [2]float64{x, y})
			}
		} else {
			pts = append(pts, [2]float64{first.X, first.Y})
		}
	}

	return pts
}
