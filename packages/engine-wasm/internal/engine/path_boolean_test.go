package engine

import "testing"

// makeTriangle returns a simple closed triangular subpath.
func makeTriangle(x0, y0, x1, y1, x2, y2 float64) Subpath {
	return Subpath{
		Closed: true,
		Points: []PathPoint{
			{X: x0, Y: y0},
			{X: x1, Y: y1},
			{X: x2, Y: y2},
		},
	}
}

func TestPathBooleanCombine(t *testing.T) {
	a := &Path{Subpaths: []Subpath{makeTriangle(0, 0, 100, 0, 50, 100)}}
	b := &Path{Subpaths: []Subpath{makeTriangle(200, 200, 300, 200, 250, 300)}}

	result, err := pathBoolean(a, b, PathBoolCombine)
	if err != nil {
		t.Fatalf("combine: %v", err)
	}
	if len(result.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths, got %d", len(result.Subpaths))
	}
	// First subpath should be A's triangle.
	if result.Subpaths[0].Points[0].X != 0 {
		t.Errorf("expected first point X=0, got %f", result.Subpaths[0].Points[0].X)
	}
	// Second subpath should be B's triangle.
	if result.Subpaths[1].Points[0].X != 200 {
		t.Errorf("expected second subpath first point X=200, got %f", result.Subpaths[1].Points[0].X)
	}
}

func TestPathBooleanSubtract(t *testing.T) {
	a := &Path{Subpaths: []Subpath{makeTriangle(0, 0, 100, 0, 50, 100)}}
	b := &Path{Subpaths: []Subpath{makeTriangle(10, 10, 90, 10, 50, 80)}}

	result, err := pathBoolean(a, b, PathBoolSubtract)
	if err != nil {
		t.Fatalf("subtract: %v", err)
	}
	if len(result.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths, got %d", len(result.Subpaths))
	}
	// First subpath is A's (unchanged).
	if result.Subpaths[0].Points[0].X != 0 {
		t.Errorf("expected A's first point X=0, got %f", result.Subpaths[0].Points[0].X)
	}
	// Second subpath is B reversed: original order was (10,10), (90,10), (50,80)
	// Reversed: (50,80), (90,10), (10,10)
	rpts := result.Subpaths[1].Points
	if rpts[0].X != 50 || rpts[0].Y != 80 {
		t.Errorf("expected reversed first point (50,80), got (%f,%f)", rpts[0].X, rpts[0].Y)
	}
	if rpts[2].X != 10 || rpts[2].Y != 10 {
		t.Errorf("expected reversed last point (10,10), got (%f,%f)", rpts[2].X, rpts[2].Y)
	}
}

func TestPathBooleanExclude(t *testing.T) {
	a := &Path{Subpaths: []Subpath{makeTriangle(0, 0, 100, 0, 50, 100)}}
	b := &Path{Subpaths: []Subpath{makeTriangle(200, 200, 300, 200, 250, 300)}}

	result, err := pathBoolean(a, b, PathBoolExclude)
	if err != nil {
		t.Fatalf("exclude: %v", err)
	}
	// Same as combine: merge subpaths.
	if len(result.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths, got %d", len(result.Subpaths))
	}
}

func TestPathBooleanIntersect(t *testing.T) {
	a := &Path{Subpaths: []Subpath{makeTriangle(0, 0, 100, 0, 50, 100)}}
	b := &Path{Subpaths: []Subpath{makeTriangle(10, 10, 90, 10, 50, 80)}}

	_, err := pathBoolean(a, b, PathBoolIntersect)
	if err == nil {
		t.Fatal("expected error for intersect, got nil")
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

func TestReverseSubpath(t *testing.T) {
	sp := Subpath{
		Closed: true,
		Points: []PathPoint{
			{X: 0, Y: 0, OutX: 10, OutY: 5, InX: -10, InY: -5, HandleType: HandleSmooth},
			{X: 100, Y: 0, OutX: 20, OutY: 15, InX: -20, InY: -15, HandleType: HandleCorner},
			{X: 50, Y: 100, OutX: 30, OutY: 25, InX: -30, InY: -25, HandleType: HandleSmooth},
		},
	}

	rev := reverseSubpath(sp)

	if !rev.Closed {
		t.Fatal("expected reversed subpath to remain closed")
	}
	if len(rev.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(rev.Points))
	}

	// Point order should be reversed: original [0,1,2] -> reversed [2,1,0].
	// rev.Points[0] should correspond to sp.Points[2].
	if rev.Points[0].X != 50 || rev.Points[0].Y != 100 {
		t.Errorf("expected first reversed point (50,100), got (%f,%f)", rev.Points[0].X, rev.Points[0].Y)
	}
	if rev.Points[2].X != 0 || rev.Points[2].Y != 0 {
		t.Errorf("expected last reversed point (0,0), got (%f,%f)", rev.Points[2].X, rev.Points[2].Y)
	}

	// Handles should be swapped: In becomes Out and vice versa.
	// Original point 2: OutX=30, OutY=25, InX=-30, InY=-25
	// After reverse, it becomes rev.Points[0]: InX should be original OutX=30, OutX should be original InX=-30.
	if rev.Points[0].InX != 30 || rev.Points[0].InY != 25 {
		t.Errorf("expected swapped InX=30, InY=25, got InX=%f, InY=%f", rev.Points[0].InX, rev.Points[0].InY)
	}
	if rev.Points[0].OutX != -30 || rev.Points[0].OutY != -25 {
		t.Errorf("expected swapped OutX=-30, OutY=-25, got OutX=%f, OutY=%f", rev.Points[0].OutX, rev.Points[0].OutY)
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

	// Add a triangle to path A (index 0) via pen tool.
	// First, select path A as active.
	// Active path is currently 1 (Shape B), so we set it back to 0.
	// Use commandSetActiveTool to set pen, then add points to path A.

	// Instead, directly manipulate: create paths with subpaths by using
	// pen tool clicks. But for simplicity, let's just combine the two empty
	// paths and verify the command succeeds with the right path count.

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

	// Intersect should return error (not yet implemented).
	_, err = DispatchCommand(h, commandPathIntersect, mustJSON(t, PathBooleanPayload{}))
	if err == nil {
		t.Fatal("expected error for intersect")
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
