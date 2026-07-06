package engine

import (
	"math"
	"testing"
)

// makeTriangle returns a simple closed triangular subpath.
func makeTriangle(x0, y0, x1, y1, x2, y2 float64) Subpath {
	return Subpath{
		Closed: true,
		Points: []PathPoint{
			{X: x0, Y: y0, InX: x0, InY: y0, OutX: x0, OutY: y0},
			{X: x1, Y: y1, InX: x1, InY: y1, OutX: x1, OutY: y1},
			{X: x2, Y: y2, InX: x2, InY: y2, OutX: x2, OutY: y2},
		},
	}
}

// makeRectSubpath returns a closed axis-aligned rectangle subpath with
// trivial (anchor-coincident) handles, i.e. straight edges.
func makeRectSubpath(x0, y0, x1, y1 float64) Subpath {
	corners := [4][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	sp := Subpath{Closed: true}
	for _, c := range corners {
		sp.Points = append(sp.Points, PathPoint{
			X: c[0], Y: c[1],
			InX: c[0], InY: c[1],
			OutX: c[0], OutY: c[1],
		})
	}
	return sp
}

// makeCircleSubpath returns a closed subpath approximating a circle with
// four cubic Bezier segments (standard kappa construction).
func makeCircleSubpath(cx, cy, r float64) Subpath {
	const k = 0.5522847498307936
	kr := k * r
	return Subpath{
		Closed: true,
		Points: []PathPoint{
			// East
			{X: cx + r, Y: cy, InX: cx + r, InY: cy - kr, OutX: cx + r, OutY: cy + kr},
			// South
			{X: cx, Y: cy + r, InX: cx + kr, InY: cy + r, OutX: cx - kr, OutY: cy + r},
			// West
			{X: cx - r, Y: cy, InX: cx - r, InY: cy + kr, OutX: cx - r, OutY: cy - kr},
			// North
			{X: cx, Y: cy - r, InX: cx - kr, InY: cy - r, OutX: cx + kr, OutY: cy - r},
		},
	}
}

// rasterizeForTest renders a path to a mask via the engine's real
// rasterization pipeline (even-odd fill, anti-aliased).
func rasterizeForTest(t *testing.T, p *Path, w, h int) []byte {
	t.Helper()
	mask, err := rasterizePathToMask(p, w, h)
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	return mask
}

// maskArea counts pixels whose coverage is at least 50%.
func maskArea(mask []byte) float64 {
	area := 0
	for _, v := range mask {
		if v >= 128 {
			area++
		}
	}
	return float64(area)
}

func maskAt(mask []byte, w, x, y int) byte {
	return mask[y*w+x]
}

func assertAreaNear(t *testing.T, got, want, relTol float64) {
	t.Helper()
	if math.Abs(got-want) > want*relTol {
		t.Errorf("area = %.1f, want %.1f (±%.0f%%)", got, want, relTol*100)
	}
}

// Two overlapping 100x80 rectangles used by the boolean op tests:
// A = (10,10)-(110,90), B = (60,50)-(160,130), overlap = (60,50)-(110,90).
// Areas: A = 8000, B = 8000, overlap = 50*40 = 2000.
func overlappingRects() (*Path, *Path) {
	a := &Path{Subpaths: []Subpath{makeRectSubpath(10, 10, 110, 90)}}
	b := &Path{Subpaths: []Subpath{makeRectSubpath(60, 50, 160, 130)}}
	return a, b
}

func TestPathBooleanCombineOverlappingRects(t *testing.T) {
	a, b := overlappingRects()
	result, err := pathBoolean(a, b, PathBoolCombine)
	if err != nil {
		t.Fatalf("combine: %v", err)
	}

	mask := rasterizeForTest(t, result, 200, 160)
	assertAreaNear(t, maskArea(mask), 14000, 0.02)

	// Points in A-only, B-only, and the overlap must all be covered.
	if maskAt(mask, 200, 20, 20) < 128 {
		t.Error("union must cover A-only region")
	}
	if maskAt(mask, 200, 150, 120) < 128 {
		t.Error("union must cover B-only region")
	}
	if maskAt(mask, 200, 85, 70) < 128 {
		t.Error("union must cover the overlap region")
	}
}

func TestPathBooleanIntersectOverlappingRects(t *testing.T) {
	a, b := overlappingRects()
	result, err := pathBoolean(a, b, PathBoolIntersect)
	if err != nil {
		t.Fatalf("intersect: %v", err)
	}

	mask := rasterizeForTest(t, result, 200, 160)
	assertAreaNear(t, maskArea(mask), 2000, 0.02)

	if maskAt(mask, 200, 85, 70) < 128 {
		t.Error("intersection must cover the overlap region")
	}
	if maskAt(mask, 200, 20, 20) >= 128 {
		t.Error("intersection must not cover A-only region")
	}
	if maskAt(mask, 200, 150, 120) >= 128 {
		t.Error("intersection must not cover B-only region")
	}
}

func TestPathBooleanSubtractOverlappingRects(t *testing.T) {
	a, b := overlappingRects()
	result, err := pathBoolean(a, b, PathBoolSubtract)
	if err != nil {
		t.Fatalf("subtract: %v", err)
	}

	mask := rasterizeForTest(t, result, 200, 160)
	assertAreaNear(t, maskArea(mask), 6000, 0.02)

	if maskAt(mask, 200, 20, 20) < 128 {
		t.Error("subtract must keep A-only region")
	}
	if maskAt(mask, 200, 85, 70) >= 128 {
		t.Error("subtract must remove the overlap region")
	}
	if maskAt(mask, 200, 150, 120) >= 128 {
		t.Error("subtract must not cover B-only region")
	}
}

func TestPathBooleanExcludeOverlappingRects(t *testing.T) {
	a, b := overlappingRects()
	result, err := pathBoolean(a, b, PathBoolExclude)
	if err != nil {
		t.Fatalf("exclude: %v", err)
	}

	mask := rasterizeForTest(t, result, 200, 160)
	assertAreaNear(t, maskArea(mask), 12000, 0.02)

	if maskAt(mask, 200, 20, 20) < 128 {
		t.Error("exclude must keep A-only region")
	}
	if maskAt(mask, 200, 150, 120) < 128 {
		t.Error("exclude must keep B-only region")
	}
	if maskAt(mask, 200, 85, 70) >= 128 {
		t.Error("exclude must remove the overlap region")
	}
}

// Circle (from Bezier segments) intersected with a rectangle covering its
// right half: result area should approximate a half disc.
func TestPathBooleanIntersectCircleRect(t *testing.T) {
	circle := &Path{Subpaths: []Subpath{makeCircleSubpath(100, 100, 50)}}
	rect := &Path{Subpaths: []Subpath{makeRectSubpath(100, 40, 170, 160)}}

	result, err := pathBoolean(circle, rect, PathBoolIntersect)
	if err != nil {
		t.Fatalf("intersect: %v", err)
	}

	mask := rasterizeForTest(t, result, 200, 200)
	halfDisc := math.Pi * 50 * 50 / 2
	assertAreaNear(t, maskArea(mask), halfDisc, 0.03)

	if maskAt(mask, 200, 125, 100) < 128 {
		t.Error("right half of the circle must be covered")
	}
	if maskAt(mask, 200, 75, 100) >= 128 {
		t.Error("left half of the circle must not be covered")
	}
}

// Subtracting an inner square from an outer square must produce a donut:
// the hole must actually be empty when rasterized.
func TestPathBooleanSubtractCreatesHole(t *testing.T) {
	outer := &Path{Subpaths: []Subpath{makeRectSubpath(10, 10, 110, 110)}}
	inner := &Path{Subpaths: []Subpath{makeRectSubpath(40, 40, 80, 80)}}

	result, err := pathBoolean(outer, inner, PathBoolSubtract)
	if err != nil {
		t.Fatalf("subtract: %v", err)
	}
	if len(result.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths (contour + hole), got %d", len(result.Subpaths))
	}
	for i, sp := range result.Subpaths {
		if !sp.Closed {
			t.Errorf("subpath %d must be closed", i)
		}
	}

	mask := rasterizeForTest(t, result, 128, 128)
	assertAreaNear(t, maskArea(mask), 10000-1600, 0.02)

	if maskAt(mask, 128, 60, 60) >= 128 {
		t.Error("point inside the hole must be empty")
	}
	if maskAt(mask, 128, 20, 20) < 128 {
		t.Error("ring region must be covered")
	}
}

// Union of two disjoint shapes keeps both regions (two output contours).
func TestPathBooleanCombineDisjoint(t *testing.T) {
	a := &Path{Subpaths: []Subpath{makeRectSubpath(10, 10, 50, 50)}}
	b := &Path{Subpaths: []Subpath{makeRectSubpath(80, 80, 120, 120)}}

	result, err := pathBoolean(a, b, PathBoolCombine)
	if err != nil {
		t.Fatalf("combine: %v", err)
	}
	if len(result.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths, got %d", len(result.Subpaths))
	}

	mask := rasterizeForTest(t, result, 140, 140)
	assertAreaNear(t, maskArea(mask), 1600+1600, 0.02)
	if maskAt(mask, 140, 30, 30) < 128 {
		t.Error("first rect must be covered")
	}
	if maskAt(mask, 140, 100, 100) < 128 {
		t.Error("second rect must be covered")
	}
}

// Boolean ops on empty paths must not error (the UI dispatches them on
// freshly created, still-empty paths).
func TestPathBooleanEmptyInputs(t *testing.T) {
	for _, op := range []PathBoolOp{PathBoolCombine, PathBoolSubtract, PathBoolIntersect, PathBoolExclude} {
		result, err := pathBoolean(&Path{}, &Path{}, op)
		if err != nil {
			t.Fatalf("op %d on empty paths: %v", op, err)
		}
		if len(result.Subpaths) != 0 {
			t.Fatalf("op %d: expected empty result, got %d subpaths", op, len(result.Subpaths))
		}
	}
}

func TestPathBooleanNilInput(t *testing.T) {
	a := &Path{Subpaths: []Subpath{makeTriangle(0, 0, 100, 0, 50, 100)}}

	_, err := pathBoolean(a, nil, PathBoolCombine)
	if err == nil {
		t.Fatal("expected error for nil path B")
	}
	_, err = pathBoolean(nil, a, PathBoolCombine)
	if err == nil {
		t.Fatal("expected error for nil path A")
	}
}

func TestFlattenPaths(t *testing.T) {
	paths := []NamedPath{
		{Name: "Path 1", Path: Path{Subpaths: []Subpath{makeTriangle(0, 0, 100, 0, 50, 100)}}},
		{Name: "Path 2", Path: Path{Subpaths: []Subpath{
			makeTriangle(200, 200, 300, 200, 250, 300),
			makeTriangle(400, 400, 500, 400, 450, 500),
		}}},
		{Name: "Path 3", Path: Path{Subpaths: []Subpath{makeTriangle(600, 600, 700, 600, 650, 700)}}},
	}

	result := flattenPaths(paths)
	// Should have 1+2+1 = 4 subpaths total.
	if len(result.Subpaths) != 4 {
		t.Fatalf("expected 4 subpaths, got %d", len(result.Subpaths))
	}
}

func TestFlattenPathsEmpty(t *testing.T) {
	result := flattenPaths(nil)
	if len(result.Subpaths) != 0 {
		t.Fatalf("expected 0 subpaths for empty input, got %d", len(result.Subpaths))
	}
}

func TestPathBooleanViaDispatch(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// Create two paths.
	_, err := DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "Shape A"}))
	if err != nil {
		t.Fatalf("create path A: %v", err)
	}
	_, err = DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "Shape B"}))
	if err != nil {
		t.Fatalf("create path B: %v", err)
	}

	// Active path is 1 (Shape B). Combine should merge paths 1 and 0.
	// But the default is active + next, wrapping around: active=1, next=0.
	result, err := DispatchCommand(h, commandPathCombine, mustJSON(t, PathBooleanPayload{}))
	if err != nil {
		t.Fatalf("combine: %v", err)
	}
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path after combine, got %d", len(result.UIMeta.Paths))
	}
}

func TestPathBooleanViaDispatchTooFewPaths(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// Create only one path.
	_, err := DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "Lonely"}))
	if err != nil {
		t.Fatalf("create path: %v", err)
	}

	// Combine should fail: need at least 2 paths.
	_, err = DispatchCommand(h, commandPathCombine, mustJSON(t, PathBooleanPayload{}))
	if err == nil {
		t.Fatal("expected error combining with only 1 path")
	}
}

func TestPathBooleanIntersectViaDispatch(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	_, err := DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "A"}))
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	_, err = DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "B"}))
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	// Intersect is real geometry now: it must succeed and merge the two
	// (empty) paths into one empty result path.
	result, err := DispatchCommand(h, commandPathIntersect, mustJSON(t, PathBooleanPayload{}))
	if err != nil {
		t.Fatalf("intersect: %v", err)
	}
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path after intersect, got %d", len(result.UIMeta.Paths))
	}
}

func TestFlattenPathViaDispatch(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// Create three paths.
	_, err := DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "P1"}))
	if err != nil {
		t.Fatalf("create P1: %v", err)
	}
	_, err = DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "P2"}))
	if err != nil {
		t.Fatalf("create P2: %v", err)
	}
	_, err = DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "P3"}))
	if err != nil {
		t.Fatalf("create P3: %v", err)
	}

	result, err := DispatchCommand(h, commandFlattenPath, "{}")
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path after flatten, got %d", len(result.UIMeta.Paths))
	}
	// Should keep the first path's name.
	if result.UIMeta.Paths[0].Name != "P1" {
		t.Errorf("expected flattened path name %q, got %q", "P1", result.UIMeta.Paths[0].Name)
	}
}
