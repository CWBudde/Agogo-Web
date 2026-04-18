package engine

import (
	"math"
	"testing"
)

func TestMakeRectPath(t *testing.T) {
	p := makeRectPath(10, 20, 100, 50)
	if len(p.Subpaths) != 1 {
		t.Fatalf("expected 1 subpath, got %d", len(p.Subpaths))
	}
	sp := p.Subpaths[0]
	if !sp.Closed {
		t.Error("rect path should be closed")
	}
	if len(sp.Points) != 4 {
		t.Fatalf("expected 4 points, got %d", len(sp.Points))
	}
	// Top-left corner
	if sp.Points[0].X != 10 || sp.Points[0].Y != 20 {
		t.Errorf("top-left: got (%v,%v) want (10,20)", sp.Points[0].X, sp.Points[0].Y)
	}
	// Bottom-right corner
	if sp.Points[2].X != 110 || sp.Points[2].Y != 70 {
		t.Errorf("bottom-right: got (%v,%v) want (110,70)", sp.Points[2].X, sp.Points[2].Y)
	}
}

func TestMakeRoundedRectPath(t *testing.T) {
	p := makeRoundedRectPath(0, 0, 100, 60, 10)
	if len(p.Subpaths) != 1 {
		t.Fatalf("expected 1 subpath, got %d", len(p.Subpaths))
	}
	sp := p.Subpaths[0]
	if !sp.Closed {
		t.Error("rounded-rect path should be closed")
	}
	if len(sp.Points) != 8 {
		t.Fatalf("expected 8 points, got %d", len(sp.Points))
	}
}

func TestMakeRoundedRectPath_ZeroRadius(t *testing.T) {
	p := makeRoundedRectPath(0, 0, 100, 60, 0)
	if len(p.Subpaths[0].Points) != 4 {
		t.Error("zero corner radius should produce a plain rect (4 points)")
	}
}

func TestMakeRoundedRectPath_ClampRadius(t *testing.T) {
	// Corner radius larger than half-height should be clamped.
	p := makeRoundedRectPath(0, 0, 100, 60, 1000)
	sp := p.Subpaths[0]
	if len(sp.Points) != 8 {
		t.Fatalf("expected 8 points, got %d", len(sp.Points))
	}
	// With clamped r = 30, top-left anchor should be at (30, 0).
	if sp.Points[0].X != 30 || sp.Points[0].Y != 0 {
		t.Errorf("clamped top anchor: got (%v,%v) want (30,0)", sp.Points[0].X, sp.Points[0].Y)
	}
}

func TestMakeEllipsePath(t *testing.T) {
	p := makeEllipsePath(0, 0, 100, 80)
	if len(p.Subpaths) != 1 {
		t.Fatalf("expected 1 subpath, got %d", len(p.Subpaths))
	}
	sp := p.Subpaths[0]
	if !sp.Closed {
		t.Error("ellipse path should be closed")
	}
	if len(sp.Points) != 4 {
		t.Fatalf("expected 4 points (top/right/bottom/left), got %d", len(sp.Points))
	}
	// Bounding box (0,0,100,80) → cx=50, cy=40, ry=40 → top point at (50, 0).
	top := sp.Points[0]
	if top.X != 50 || top.Y != 0 {
		t.Errorf("top point: got (%v,%v) want (50,0)", top.X, top.Y)
	}
}

func TestMakePolygonPath_Hexagon(t *testing.T) {
	p, err := makePolygonPath(0, 0, 100, 100, 6, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	sp := p.Subpaths[0]
	if !sp.Closed {
		t.Error("polygon path should be closed")
	}
	if len(sp.Points) != 6 {
		t.Fatalf("hexagon: expected 6 points, got %d", len(sp.Points))
	}
}

func TestMakePolygonPath_Star(t *testing.T) {
	p, err := makePolygonPath(0, 0, 100, 100, 5, true, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	sp := p.Subpaths[0]
	if len(sp.Points) != 10 {
		t.Fatalf("5-point star: expected 10 points, got %d", len(sp.Points))
	}
}

func TestMakePolygonPath_TooFewSides(t *testing.T) {
	_, err := makePolygonPath(0, 0, 100, 100, 2, false, 0)
	if err == nil {
		t.Error("expected error for fewer than 3 sides")
	}
}

func TestMakeLinePath(t *testing.T) {
	p := makeLinePath(10, 20, 100, 200)
	if len(p.Subpaths) != 1 {
		t.Fatalf("expected 1 subpath, got %d", len(p.Subpaths))
	}
	sp := p.Subpaths[0]
	if sp.Closed {
		t.Error("line path should not be closed")
	}
	if len(sp.Points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(sp.Points))
	}
	if sp.Points[0].X != 10 || sp.Points[0].Y != 20 {
		t.Errorf("start: got (%v,%v) want (10,20)", sp.Points[0].X, sp.Points[0].Y)
	}
	if sp.Points[1].X != 100 || sp.Points[1].Y != 200 {
		t.Errorf("end: got (%v,%v) want (100,200)", sp.Points[1].X, sp.Points[1].Y)
	}
}

func TestRasterizeVectorShape_Rect(t *testing.T) {
	p := makeRectPath(10, 10, 80, 60)
	fill := [4]uint8{255, 0, 0, 255}
	buf, err := rasterizeVectorShape(p, 100, 100, fill, [4]uint8{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != 100*100*4 {
		t.Fatalf("buffer size mismatch: got %d", len(buf))
	}
	// Centre pixel (50,50) should be filled red.
	idx := (50*100 + 50) * 4
	if buf[idx] != 255 || buf[idx+1] != 0 || buf[idx+2] != 0 || buf[idx+3] != 255 {
		t.Errorf("centre pixel: got %v want [255 0 0 255]", buf[idx:idx+4])
	}
}

func TestRasterizeVectorShape_EmptyPath(t *testing.T) {
	_, err := rasterizeVectorShape(nil, 100, 100, [4]uint8{255, 0, 0, 255}, [4]uint8{}, 0)
	if err == nil {
		t.Error("expected error for nil path")
	}
}

func TestBuildShapePath_AllTypes(t *testing.T) {
	cases := []DrawShapePayload{
		{ShapeType: "rect", X: 0, Y: 0, W: 100, H: 100},
		{ShapeType: "rounded-rect", X: 0, Y: 0, W: 100, H: 100, CornerRadius: 10},
		{ShapeType: "ellipse", X: 0, Y: 0, W: 100, H: 100},
		{ShapeType: "polygon", X: 0, Y: 0, W: 100, H: 100, Sides: 5},
		{ShapeType: "line", X: 0, Y: 0, W: 100, H: 100},
	}
	for _, c := range cases {
		p, err := buildShapePath(c)
		if err != nil {
			t.Errorf("shapeType=%q: %v", c.ShapeType, err)
			continue
		}
		if p == nil || len(p.Subpaths) == 0 {
			t.Errorf("shapeType=%q: got nil or empty path", c.ShapeType)
		}
	}
}

func TestBuildShapePath_Unknown(t *testing.T) {
	_, err := buildShapePath(DrawShapePayload{ShapeType: "triangle"})
	if err == nil {
		t.Error("expected error for unknown shape type")
	}
}

func TestBuildShapePath_CustomShapePreservesHandles(t *testing.T) {
	payload := DrawShapePayload{
		ShapeType: "custom-shape",
		Closed:    true,
		Points: []PathPoint{
			{X: 10, Y: 10, OutX: 25, OutY: 5, HandleType: HandleSmooth},
			{X: 40, Y: 30, InX: 35, InY: 15, HandleType: HandleSmooth},
		},
	}

	path, err := buildShapePath(payload)
	if err != nil {
		t.Fatalf("buildShapePath(custom-shape): %v", err)
	}
	if len(path.Subpaths) != 1 || len(path.Subpaths[0].Points) != 2 {
		t.Fatalf("unexpected custom-shape path shape: %+v", path.Subpaths)
	}

	got := path.Subpaths[0].Points
	if !path.Subpaths[0].Closed {
		t.Fatal("expected custom-shape subpath to be closed")
	}
	if got[0].OutX != 25 || got[0].OutY != 5 {
		t.Fatalf("first point out-handle = (%v,%v), want (25,5)", got[0].OutX, got[0].OutY)
	}
	if got[1].InX != 35 || got[1].InY != 15 {
		t.Fatalf("second point in-handle = (%v,%v), want (35,15)", got[1].InX, got[1].InY)
	}
}

func TestDrawShape_CustomShapeShapeAndPathModes(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	points := []PathPoint{
		{X: 120, Y: 100},
		{X: 170, Y: 100},
		{X: 170, Y: 150},
		{X: 120, Y: 150},
	}

	if _, err := DispatchCommand(h, commandDrawShape, mustJSON(t, DrawShapePayload{
		ShapeType:   "custom-shape",
		Closed:      true,
		Points:      points,
		FillColor:   [4]uint8{255, 0, 0, 255},
		StrokeColor: [4]uint8{},
		StrokeWidth: 0,
		Mode:        "shape",
	})); err != nil {
		t.Fatalf("draw custom-shape layer: %v", err)
	}

	doc := instances[h].manager.Active()
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID)
	if !ok {
		t.Fatal("expected active layer after drawing custom-shape")
	}
	vectorLayer, ok := layer.(*VectorLayer)
	if !ok {
		t.Fatalf("active layer type = %T, want *VectorLayer", layer)
	}
	if vectorLayer.Shape == nil || len(vectorLayer.Shape.Subpaths) != 1 {
		t.Fatal("expected vector layer shape path")
	}
	if len(vectorLayer.Shape.Subpaths[0].Points) != len(points) {
		t.Fatalf("custom-shape point count = %d, want %d", len(vectorLayer.Shape.Subpaths[0].Points), len(points))
	}

	if _, err := DispatchCommand(h, commandDrawShape, mustJSON(t, DrawShapePayload{
		ShapeType: "custom-shape",
		Closed:    true,
		Points: []PathPoint{
			{X: 220, Y: 200},
			{X: 260, Y: 200},
			{X: 260, Y: 240},
			{X: 220, Y: 240},
		},
		Mode: "path",
	})); err != nil {
		t.Fatalf("draw custom-shape path: %v", err)
	}

	doc = instances[h].manager.Active()
	if len(doc.Paths) == 0 {
		t.Fatal("expected custom-shape path in document paths")
	}
	lastPath := doc.Paths[len(doc.Paths)-1].Path
	if len(lastPath.Subpaths) != 1 || len(lastPath.Subpaths[0].Points) != 4 {
		t.Fatalf("unexpected custom-shape path payload: %+v", lastPath.Subpaths)
	}
	if !lastPath.Subpaths[0].Closed {
		t.Fatal("expected path-mode custom shape to be closed")
	}
}

// TestPolygonVerticesOnEllipse checks that all outer vertices of a hexagon lie on the ellipse.
func TestPolygonVerticesOnEllipse(t *testing.T) {
	x, y, w, h := 0.0, 0.0, 100.0, 80.0
	cx, cy := x+w/2, y+h/2
	rx, ry := w/2, h/2
	p, err := makePolygonPath(x, y, w, h, 6, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, pt := range p.Subpaths[0].Points {
		dx := (pt.X - cx) / rx
		dy := (pt.Y - cy) / ry
		dist := math.Sqrt(dx*dx + dy*dy)
		if math.Abs(dist-1) > 1e-9 {
			t.Errorf("point %d not on ellipse: dist=%v", i, dist)
		}
	}
}
