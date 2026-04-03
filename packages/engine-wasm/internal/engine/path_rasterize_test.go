package engine

import "testing"

// makeSquarePath returns a closed square path from (x0,y0) to (x1,y1).
func makeSquarePath(x0, y0, x1, y1 float64) Path {
	return Path{
		Subpaths: []Subpath{{
			Closed: true,
			Points: []PathPoint{
				{X: x0, Y: y0, InX: x0, InY: y0, OutX: x0, OutY: y0},
				{X: x1, Y: y0, InX: x1, InY: y0, OutX: x1, OutY: y0},
				{X: x1, Y: y1, InX: x1, InY: y1, OutX: x1, OutY: y1},
				{X: x0, Y: y1, InX: x0, InY: y1, OutX: x0, OutY: y1},
			},
		}},
	}
}

func TestRasterizePathToMask(t *testing.T) {
	// 100x100 square path from (10,10) to (90,90).
	sq := makeSquarePath(10, 10, 90, 90)
	mask, err := rasterizePathToMask(&sq, 100, 100)
	if err != nil {
		t.Fatalf("rasterizePathToMask: %v", err)
	}
	if len(mask) != 100*100 {
		t.Fatalf("mask len = %d, want %d", len(mask), 100*100)
	}

	// Center pixel should be 255 (inside).
	center := mask[50*100+50]
	if center != 255 {
		t.Errorf("center pixel (50,50) = %d, want 255", center)
	}

	// Corner (2,2) should be 0 (outside the path).
	corner := mask[2*100+2]
	if corner != 0 {
		t.Errorf("corner pixel (2,2) = %d, want 0", corner)
	}

	// Count non-zero pixels — the 80x80 square should have ~6400.
	count := 0
	for _, v := range mask {
		if v > 0 {
			count++
		}
	}
	if count < 6000 || count > 7000 {
		t.Errorf("expected ~6400 non-zero mask pixels, got %d", count)
	}
}

func TestRasterizePathToMaskEmpty(t *testing.T) {
	_, err := rasterizePathToMask(nil, 100, 100)
	if err == nil {
		t.Error("expected error for nil path")
	}

	empty := &Path{}
	_, err = rasterizePathToMask(empty, 100, 100)
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestMakeSelectionFromPath(t *testing.T) {
	sq := makeSquarePath(10, 10, 90, 90)
	doc := &Document{
		Width:  100,
		Height: 100,
		Paths:  []NamedPath{{Name: "Test", Path: sq}},
	}

	if err := doc.makeSelectionFromPath(0); err != nil {
		t.Fatalf("makeSelectionFromPath: %v", err)
	}

	if doc.Selection == nil {
		t.Fatal("selection is nil after makeSelectionFromPath")
	}
	if doc.Selection.Width != 100 || doc.Selection.Height != 100 {
		t.Errorf("selection size = %dx%d, want 100x100", doc.Selection.Width, doc.Selection.Height)
	}
	if len(doc.Selection.Mask) != 100*100 {
		t.Fatalf("mask len = %d, want %d", len(doc.Selection.Mask), 100*100)
	}

	// Center should be selected.
	if doc.Selection.Mask[50*100+50] == 0 {
		t.Error("center pixel should be selected")
	}
}

func TestMakeSelectionFromPathOutOfRange(t *testing.T) {
	doc := &Document{Width: 10, Height: 10}
	if err := doc.makeSelectionFromPath(0); err == nil {
		t.Error("expected error for out-of-range path index")
	}
}

// newPathTestInstance creates a test instance with a 100x100 document,
// one pixel layer, and one square path (10,10)-(90,90).
func newPathTestInstance(t *testing.T) *instance {
	t.Helper()
	w, h := 100, 100
	pixels := make([]byte, w*h*4)
	// Fill with transparent black.
	layer := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, pixels)
	sq := makeSquarePath(10, 10, 90, 90)
	doc := &Document{
		ID:            "doc-path-test",
		Width:         w,
		Height:        h,
		LayerRoot:     NewGroupLayer("Root"),
		ActiveLayerID: layer.ID(),
		Paths:         []NamedPath{{Name: "Square", Path: sq}},
		ActivePathIdx: 0,
	}
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	inst := &instance{
		manager:         newDocumentManager(),
		history:         newHistoryStack(16),
		viewport:        ViewportState{CanvasW: w, CanvasH: h, Zoom: 1, DevicePixelRatio: 1},
		foregroundColor: [4]uint8{255, 0, 0, 255},
	}
	inst.manager.Create(doc)
	return inst
}

func TestFillPath(t *testing.T) {
	inst := newPathTestInstance(t)
	doc := inst.manager.activeMut()
	red := [4]uint8{255, 0, 0, 255}
	if err := fillPathOnDoc(doc, 0, red); err != nil {
		t.Fatalf("fillPathOnDoc: %v", err)
	}

	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		t.Fatal("pixel layer not found")
	}

	// Center pixel (50,50) should be red.
	off := (50*100 + 50) * 4
	r, g, b, a := layer.Pixels[off], layer.Pixels[off+1], layer.Pixels[off+2], layer.Pixels[off+3]
	if r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("center pixel = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}

	// Corner pixel (2,2) should still be transparent.
	off2 := (2*100 + 2) * 4
	if layer.Pixels[off2+3] != 0 {
		t.Errorf("corner pixel alpha = %d, want 0", layer.Pixels[off2+3])
	}
}

func TestFillPathOutOfRange(t *testing.T) {
	inst := newPathTestInstance(t)
	doc := inst.manager.activeMut()
	if err := fillPathOnDoc(doc, 5, [4]uint8{255, 0, 0, 255}); err == nil {
		t.Error("expected error for out-of-range path index")
	}
}

func TestStrokePath(t *testing.T) {
	inst := newPathTestInstance(t)
	doc := inst.manager.activeMut()
	blue := [4]uint8{0, 0, 255, 255}
	if err := strokePathOnDoc(doc, 0, 3.0, blue); err != nil {
		t.Fatalf("strokePathOnDoc: %v", err)
	}

	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		t.Fatal("pixel layer not found")
	}

	// A pixel on the top edge of the square (e.g. 50,10) should be stroked.
	off := (10*100 + 50) * 4
	a := layer.Pixels[off+3]
	if a == 0 {
		t.Error("pixel on path edge (50,10) should have non-zero alpha after stroke")
	}

	// Center pixel (50,50) should remain transparent (stroke only, no fill).
	offCenter := (50*100 + 50) * 4
	if layer.Pixels[offCenter+3] != 0 {
		t.Errorf("center pixel alpha = %d, want 0 (stroke only)", layer.Pixels[offCenter+3])
	}
}

func TestStrokePathOutOfRange(t *testing.T) {
	inst := newPathTestInstance(t)
	doc := inst.manager.activeMut()
	if err := strokePathOnDoc(doc, 5, 1.0, [4]uint8{0, 0, 255, 255}); err == nil {
		t.Error("expected error for out-of-range path index")
	}
}
