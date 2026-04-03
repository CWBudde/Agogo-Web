package engine

import "testing"

// newShapeTestInstance creates a test instance with a 100x100 document
// containing one VectorLayer (filled red rectangle).
func newShapeTestInstance(t *testing.T) (*instance, string) {
	t.Helper()
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{CanvasW: 100, CanvasH: 100, Zoom: 1, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
		pathTool: newPathToolState(),
	}
	doc := &Document{
		ID:        "doc-shape-edit-test",
		Width:     100,
		Height:    100,
		LayerRoot: NewGroupLayer("Root"),
	}
	path := makeRectPath(10, 10, 80, 80)
	fill := [4]uint8{255, 0, 0, 255}
	raster, _ := rasterizeVectorShape(path, 100, 100, fill, [4]uint8{}, 0)
	vl := NewVectorLayer("Rect", LayerBounds{W: 100, H: 100}, path, raster)
	vl.FillColor = fill
	doc.LayerRoot.SetChildren([]LayerNode{vl})
	doc.ActiveLayerID = vl.ID()
	inst.manager.Create(doc)
	return inst, vl.ID()
}

func TestEnterVectorEditMode(t *testing.T) {
	inst, layerID := newShapeTestInstance(t)
	err := inst.enterVectorEditMode(EnterVectorEditModePayload{LayerID: layerID})
	if err != nil {
		t.Fatalf("enterVectorEditMode: %v", err)
	}
	if inst.editingVectorLayerID != layerID {
		t.Errorf("editingVectorLayerID = %q, want %q", inst.editingVectorLayerID, layerID)
	}
	if inst.pathTool.activeTool != "direct-select" {
		t.Errorf("activeTool = %q, want direct-select", inst.pathTool.activeTool)
	}
	doc := inst.manager.Active()
	if len(doc.Paths) == 0 {
		t.Fatal("expected path to be loaded into doc.Paths")
	}
	if len(doc.Paths[0].Path.Subpaths) == 0 {
		t.Error("loaded path has no subpaths")
	}
}

func TestEnterVectorEditMode_WrongType(t *testing.T) {
	inst := &instance{
		manager:  newDocumentManager(),
		history:  newHistoryStack(defaultHistoryMax),
		pathTool: newPathToolState(),
	}
	doc := &Document{
		ID:        "doc-wrong-type",
		Width:     100,
		Height:    100,
		LayerRoot: NewGroupLayer("Root"),
	}
	px := NewPixelLayer("Bg", LayerBounds{W: 100, H: 100}, make([]byte, 100*100*4))
	doc.LayerRoot.SetChildren([]LayerNode{px})
	doc.ActiveLayerID = px.ID()
	inst.manager.Create(doc)

	err := inst.enterVectorEditMode(EnterVectorEditModePayload{LayerID: px.ID()})
	if err == nil {
		t.Error("expected error for non-VectorLayer")
	}
}

func TestCommitVectorEdit(t *testing.T) {
	inst, layerID := newShapeTestInstance(t)
	if err := inst.enterVectorEditMode(EnterVectorEditModePayload{LayerID: layerID}); err != nil {
		t.Fatalf("enterVectorEditMode: %v", err)
	}
	// Modify the active path in-place.
	doc := inst.manager.activeMut()
	doc.Paths[0].Path = *makeRectPath(5, 5, 90, 90) // enlarged rect

	if err := inst.commitVectorEdit(); err != nil {
		t.Fatalf("commitVectorEdit: %v", err)
	}
	if inst.editingVectorLayerID != "" {
		t.Error("editingVectorLayerID should be cleared after commit")
	}

	doc2 := inst.manager.Active()
	layer, _, _, ok := findLayerByID(doc2.ensureLayerRoot(), layerID)
	if !ok {
		t.Fatal("layer not found")
	}
	vl := layer.(*VectorLayer)
	if len(vl.Shape.Subpaths) == 0 {
		t.Fatal("VectorLayer.Shape is empty after commit")
	}
	expectedLen := 100 * 100 * 4
	if len(vl.CachedRaster) != expectedLen {
		t.Errorf("CachedRaster len = %d, want %d", len(vl.CachedRaster), expectedLen)
	}
}

func TestCommitVectorEdit_Noop(t *testing.T) {
	inst, _ := newShapeTestInstance(t)
	if err := inst.commitVectorEdit(); err != nil {
		t.Errorf("commitVectorEdit with no edit mode: %v", err)
	}
}

func TestSetVectorLayerStyle(t *testing.T) {
	inst, layerID := newShapeTestInstance(t)
	blue := [4]uint8{0, 0, 255, 255}
	err := inst.setVectorLayerStyle(SetVectorLayerStylePayload{
		LayerID:     layerID,
		FillColor:   blue,
		StrokeColor: [4]uint8{255, 255, 0, 255},
		StrokeWidth: 3,
	})
	if err != nil {
		t.Fatalf("setVectorLayerStyle: %v", err)
	}
	doc := inst.manager.Active()
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		t.Fatal("layer not found")
	}
	vl := layer.(*VectorLayer)
	if vl.FillColor != blue {
		t.Errorf("FillColor = %v, want %v", vl.FillColor, blue)
	}
	if vl.StrokeWidth != 3 {
		t.Errorf("StrokeWidth = %v, want 3", vl.StrokeWidth)
	}
	expectedLen := 100 * 100 * 4
	if len(vl.CachedRaster) != expectedLen {
		t.Errorf("CachedRaster len = %d, want %d", len(vl.CachedRaster), expectedLen)
	}
	// Centre pixel should be blue (fill).
	idx := (50*100 + 50) * 4
	if vl.CachedRaster[idx+2] != 255 || vl.CachedRaster[idx] != 0 {
		t.Errorf("centre pixel not blue: %v", vl.CachedRaster[idx:idx+4])
	}
}

func TestSetVectorLayerStyle_WrongType(t *testing.T) {
	inst := &instance{
		manager:  newDocumentManager(),
		history:  newHistoryStack(defaultHistoryMax),
		pathTool: newPathToolState(),
	}
	doc := &Document{
		ID:        "doc-wrong-type-style",
		Width:     100,
		Height:    100,
		LayerRoot: NewGroupLayer("Root"),
	}
	px := NewPixelLayer("Bg", LayerBounds{W: 100, H: 100}, make([]byte, 100*100*4))
	doc.LayerRoot.SetChildren([]LayerNode{px})
	inst.manager.Create(doc)

	err := inst.setVectorLayerStyle(SetVectorLayerStylePayload{LayerID: px.ID()})
	if err == nil {
		t.Error("expected error for non-VectorLayer")
	}
}
