// Package engine — real boolean geometry for path operations.
//
// Clipping is delegated to github.com/bolom009/go-clipper2, a pure-Go port
// of Angus Johnson's Clipper2 library, licensed under the Boost Software
// License 1.0 (commercial-friendly — unlike the GPC dependency this project
// must eventually drop). It has no cgo, so it compiles for GOOS=js
// GOARCH=wasm, and it handles self-intersections, holes, and multi-contour
// inputs robustly via 64-bit integer coordinates.
//
// Note: the in-house port at github.com/cwbudde/go-clipper2 was evaluated
// first, but its only published commit mis-unions non-convex polygons
// (the fixes exist only in unpushed local commits); revisit once a fixed
// version is published.
//
// Bezier segments are flattened to polylines before clipping (Photopea does
// the same), so boolean results are polygonal paths with straight edges.
package engine

import (
	"fmt"
	"math"

	clipper "github.com/bolom009/go-clipper2"
)

// PathBoolOp represents the type of boolean operation.
type PathBoolOp int

const (
	PathBoolCombine   PathBoolOp = iota // Union
	PathBoolSubtract                    // Difference (A - B)
	PathBoolIntersect                   // Intersection
	PathBoolExclude                     // XOR (symmetric difference)
)

// PathBooleanPayload is the JSON payload for path boolean commands.
type PathBooleanPayload struct {
	PathIndexA int `json:"pathIndexA,omitempty"` // defaults to active path
	PathIndexB int `json:"pathIndexB,omitempty"` // defaults to next path
}

// pathBooleanScale converts document-space float coordinates to Clipper2's
// int64 coordinate space. 1000 gives 1/1000 px precision, far below the
// curve-flattening error, while leaving ample int64 headroom for any
// realistic document size.
const pathBooleanScale = 1000.0

// pathBooleanCurveSteps is the number of polyline steps per Bezier segment
// when flattening for clipping. Matches the engine's existing flattening
// convention in flattenSubpathToPolyline (path_agg.go), whose default is 16.
// The chord error grows with segment size: well below a pixel for typical
// on-canvas curves, approaching ~1px for very large ones (an adaptive
// flattener would remove that ceiling if it ever matters).
const pathBooleanCurveSteps = 16

// pathBoolean performs a real boolean geometry operation on two paths:
// Combine = union, Subtract = A minus B, Intersect = A ∩ B, Exclude = A xor B.
// Curved segments are flattened to polylines; the result is polygonal with
// closed subpaths. Output contours are non-overlapping (holes are separate,
// oppositely wound subpaths), so both even-odd and nonzero fills render the
// result identically — the engine rasterizes paths with even-odd fill.
func pathBoolean(a, b *Path, op PathBoolOp) (*Path, error) {
	if a == nil || b == nil {
		return nil, fmt.Errorf("both paths must be non-nil")
	}

	subjects := pathToClipperPaths(a)
	clips := pathToClipperPaths(b)

	// The engine renders paths with even-odd fill, so interpret the inputs
	// with the same rule to make the boolean match what the user sees.
	const fillRule = clipper.EvenOdd

	var solution clipper.Paths64
	switch op {
	case PathBoolCombine:
		solution = clipper.UnionWithClipPaths64(subjects, clips, fillRule)
	case PathBoolSubtract:
		solution = clipper.DifferenceWithClipPaths64(subjects, clips, fillRule)
	case PathBoolIntersect:
		solution = clipper.IntersectWithClipPaths64(subjects, clips, fillRule)
	case PathBoolExclude:
		solution = clipper.XorWithClipPaths64(subjects, clips, fillRule)
	default:
		return nil, fmt.Errorf("unknown path boolean operation: %d", op)
	}

	return clipperPathsToPath(solution), nil
}

// pathToClipperPaths flattens every subpath of p to a closed integer polygon.
// Open subpaths are implicitly closed, matching the engine's fill semantics
// (AGG closes open contours when filling). Degenerate subpaths (< 3 distinct
// points) contribute no area and are dropped by the clipper.
func pathToClipperPaths(p *Path) clipper.Paths64 {
	if p == nil {
		return nil
	}
	var out clipper.Paths64
	for i := range p.Subpaths {
		poly := flattenSubpathToPolyline(&p.Subpaths[i], pathBooleanCurveSteps)
		if len(poly) == 0 {
			continue
		}
		path := make(clipper.Path64, 0, len(poly))
		for _, pt := range poly {
			ipt := clipper.Point64{
				X: int64(math.Round(pt[0] * pathBooleanScale)),
				Y: int64(math.Round(pt[1] * pathBooleanScale)),
			}
			// Skip consecutive duplicates (flattening emits the first point
			// again at the end of closed subpaths).
			if n := len(path); n > 0 && path[n-1] == ipt {
				continue
			}
			path = append(path, ipt)
		}
		// Drop a trailing point that closes onto the start.
		if n := len(path); n > 1 && path[0] == path[n-1] {
			path = path[:n-1]
		}
		if len(path) >= 3 {
			out = append(out, path)
		}
	}
	return out
}

// clipperPathsToPath converts clipper output polygons back into the model's
// Path representation: closed subpaths of straight segments (handles
// coincide with anchors).
func clipperPathsToPath(paths clipper.Paths64) *Path {
	result := &Path{}
	for _, poly := range paths {
		if len(poly) < 3 {
			continue
		}
		sp := Subpath{Closed: true, Points: make([]PathPoint, len(poly))}
		for i, pt := range poly {
			x := float64(pt.X) / pathBooleanScale
			y := float64(pt.Y) / pathBooleanScale
			sp.Points[i] = PathPoint{
				X: x, Y: y,
				InX: x, InY: y,
				OutX: x, OutY: y,
				HandleType: HandleCorner,
			}
		}
		result.Subpaths = append(result.Subpaths, sp)
	}
	return result
}

// reverseSubpath returns a copy of the subpath with points in reverse order
// and In/Out handles swapped (since direction is reversed).
func reverseSubpath(sp Subpath) Subpath {
	n := len(sp.Points)
	reversed := Subpath{
		Closed: sp.Closed,
		Points: make([]PathPoint, n),
	}
	for i, pt := range sp.Points {
		rpt := PathPoint{
			X: pt.X, Y: pt.Y,
			// Swap In and Out handles (direction reverses).
			InX:        pt.OutX,
			InY:        pt.OutY,
			OutX:       pt.InX,
			OutY:       pt.InY,
			HandleType: pt.HandleType,
		}
		reversed.Points[n-1-i] = rpt
	}
	return reversed
}

// flattenPaths merges all document paths into a single path.
func flattenPaths(paths []NamedPath) *Path {
	result := &Path{}
	for _, np := range paths {
		result.Subpaths = append(result.Subpaths, np.Path.Subpaths...)
	}
	return result
}
