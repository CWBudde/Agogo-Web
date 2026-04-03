package engine

import (
	"fmt"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

// kappa is the cubic Bezier approximation constant for a quarter circle:
// 4/3 * (sqrt(2) - 1) ≈ 0.5523.
const kappa = 0.5522847498

// corner returns a PathPoint at (px,py) with handles coincident with the anchor.
func corner(px, py float64) PathPoint {
	return PathPoint{X: px, Y: py, InX: px, InY: py, OutX: px, OutY: py}
}

// makeRectPath returns a closed rectangular path with corners (x,y)…(x+w,y+h).
func makeRectPath(x, y, w, h float64) *Path {
	return &Path{Subpaths: []Subpath{{
		Closed: true,
		Points: []PathPoint{
			corner(x, y),
			corner(x+w, y),
			corner(x+w, y+h),
			corner(x, y+h),
		},
	}}}
}

// makeRoundedRectPath returns a closed rounded-rectangle path.
// r is clamped to half of the shorter side.
func makeRoundedRectPath(x, y, w, h, r float64) *Path {
	if r <= 0 {
		return makeRectPath(x, y, w, h)
	}
	r = math.Min(r, math.Min(w, h)*0.5)
	k := kappa * r

	// Build 8 points (2 per corner), each with bezier handles.
	// Corners in order: top-left, top-right, bottom-right, bottom-left.
	pts := []PathPoint{
		// top edge left → right
		{X: x + r, Y: y, OutX: x + r + k, OutY: y, InX: x + w - r - k, InY: y},
		{X: x + w - r, Y: y, InX: x + w - r - k, InY: y, OutX: x + w - r + k, OutY: y},
		// right edge top → bottom
		{X: x + w, Y: y + r, InX: x + w, InY: y + r - k, OutX: x + w, OutY: y + r + k},
		{X: x + w, Y: y + h - r, InX: x + w, InY: y + h - r - k, OutX: x + w, OutY: y + h - r + k},
		// bottom edge right → left
		{X: x + w - r, Y: y + h, InX: x + w - r + k, InY: y + h, OutX: x + w - r - k, OutY: y + h},
		{X: x + r, Y: y + h, InX: x + r + k, InY: y + h, OutX: x + r - k, OutY: y + h},
		// left edge bottom → top
		{X: x, Y: y + h - r, InX: x, InY: y + h - r + k, OutX: x, OutY: y + h - r - k},
		{X: x, Y: y + r, InX: x, InY: y + r + k, OutX: x, OutY: y + r - k},
	}
	return &Path{Subpaths: []Subpath{{Closed: true, Points: pts}}}
}

// makeEllipsePath returns a closed ellipse path approximated by four cubic Bezier segments.
func makeEllipsePath(x, y, w, h float64) *Path {
	cx := x + w*0.5
	cy := y + h*0.5
	rx := w * 0.5
	ry := h * 0.5
	kx := kappa * rx
	ky := kappa * ry

	pts := []PathPoint{
		// top
		{X: cx, Y: cy - ry, InX: cx - kx, InY: cy - ry, OutX: cx + kx, OutY: cy - ry},
		// right
		{X: cx + rx, Y: cy, InX: cx + rx, InY: cy - ky, OutX: cx + rx, OutY: cy + ky},
		// bottom
		{X: cx, Y: cy + ry, InX: cx + kx, InY: cy + ry, OutX: cx - kx, OutY: cy + ry},
		// left
		{X: cx - rx, Y: cy, InX: cx - rx, InY: cy + ky, OutX: cx - rx, OutY: cy - ky},
	}
	return &Path{Subpaths: []Subpath{{Closed: true, Points: pts}}}
}

// makePolygonPath returns a closed regular polygon or star path inscribed in the ellipse
// defined by the bounding box (x,y,w,h).
//
// sides is the number of outer vertices (≥ 3).
// If starMode is true, inner vertices are interleaved at innerRadiusPct * outer radius.
func makePolygonPath(x, y, w, h float64, sides int, starMode bool, innerRadiusPct float64) (*Path, error) {
	if sides < 3 {
		return nil, fmt.Errorf("polygon requires at least 3 sides, got %d", sides)
	}
	cx := x + w*0.5
	cy := y + h*0.5
	rx := w * 0.5
	ry := h * 0.5

	totalPts := sides
	if starMode {
		totalPts = sides * 2
		if innerRadiusPct <= 0 {
			innerRadiusPct = 0.5
		}
	}

	pts := make([]PathPoint, totalPts)
	angleOffset := -math.Pi / 2 // start from top

	for i := range totalPts {
		var r float64
		if starMode {
			if i%2 == 0 {
				r = 1.0 // outer
			} else {
				r = innerRadiusPct
			}
		} else {
			r = 1.0
		}
		angle := angleOffset + float64(i)*2*math.Pi/float64(totalPts)
		px := cx + rx*r*math.Cos(angle)
		py := cy + ry*r*math.Sin(angle)
		pts[i] = corner(px, py)
	}
	return &Path{Subpaths: []Subpath{{Closed: true, Points: pts}}}, nil
}

// makeLinePath returns an open single-segment line path from (x1,y1) to (x2,y2).
func makeLinePath(x1, y1, x2, y2 float64) *Path {
	return &Path{Subpaths: []Subpath{{
		Closed: false,
		Points: []PathPoint{
			corner(x1, y1),
			corner(x2, y2),
		},
	}}}
}

// rasterizeVectorShape renders a Path with fill and/or stroke colors into an RGBA buffer
// sized docW×docH. The result is suitable for storing in VectorLayer.CachedRaster.
func rasterizeVectorShape(p *Path, docW, docH int, fillColor, strokeColor [4]uint8, strokeWidth float64) ([]byte, error) {
	if p == nil || len(p.Subpaths) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	stride := docW * 4
	buf := make([]byte, stride*docH)

	hasFill := fillColor[3] > 0
	hasStroke := strokeColor[3] > 0 && strokeWidth > 0

	if !hasFill && !hasStroke {
		return buf, nil
	}

	r := agglib.NewAgg2D()
	r.Attach(buf, docW, docH, stride)
	r.FillEvenOdd(true)
	r.ResetTransformations()
	r.ResetPath()
	applyPathToAgg2D(r, p)

	switch {
	case hasFill && hasStroke:
		r.FillColor(agglib.NewColor(fillColor[0], fillColor[1], fillColor[2], fillColor[3]))
		r.LineColor(agglib.NewColor(strokeColor[0], strokeColor[1], strokeColor[2], strokeColor[3]))
		r.LineWidth(strokeWidth)
		r.DrawPath(agglib.FillAndStroke)
	case hasFill:
		r.FillColor(agglib.NewColor(fillColor[0], fillColor[1], fillColor[2], fillColor[3]))
		r.NoLine()
		r.DrawPath(agglib.FillOnly)
	case hasStroke:
		r.NoFill()
		r.LineColor(agglib.NewColor(strokeColor[0], strokeColor[1], strokeColor[2], strokeColor[3]))
		r.LineWidth(strokeWidth)
		r.DrawPath(agglib.StrokeOnly)
	}

	return buf, nil
}
