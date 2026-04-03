package engine

import "fmt"

// PathBoolOp represents the type of boolean operation.
type PathBoolOp int

const (
	PathBoolCombine   PathBoolOp = iota // Merge subpaths (union-like)
	PathBoolSubtract                    // Add B reversed (subtract-like)
	PathBoolIntersect                   // Requires polygon clipping
	PathBoolExclude                     // XOR via even-odd
)

// PathBooleanPayload is the JSON payload for path boolean commands.
type PathBooleanPayload struct {
	PathIndexA int `json:"pathIndexA,omitempty"` // defaults to active path
	PathIndexB int `json:"pathIndexB,omitempty"` // defaults to next path
}

// pathBoolean performs a boolean operation on two paths.
// For Combine and Exclude, it merges subpaths from both paths.
// For Subtract, it reverses B's subpaths before merging.
// Intersect is not yet supported without a polygon clipping library.
func pathBoolean(a, b *Path, op PathBoolOp) (*Path, error) {
	if a == nil || b == nil {
		return nil, fmt.Errorf("both paths must be non-nil")
	}

	switch op {
	case PathBoolCombine, PathBoolExclude:
		// Merge all subpaths from both paths.
		result := &Path{}
		result.Subpaths = append(result.Subpaths, a.Subpaths...)
		result.Subpaths = append(result.Subpaths, b.Subpaths...)
		return result, nil

	case PathBoolSubtract:
		// Merge A's subpaths + B's subpaths with reversed point order.
		result := &Path{}
		result.Subpaths = append(result.Subpaths, a.Subpaths...)
		for _, sp := range b.Subpaths {
			reversed := reverseSubpath(sp)
			result.Subpaths = append(result.Subpaths, reversed)
		}
		return result, nil

	case PathBoolIntersect:
		return nil, fmt.Errorf("path intersect requires polygon clipping (not yet available)")

	default:
		return nil, fmt.Errorf("unknown path boolean operation: %d", op)
	}
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
