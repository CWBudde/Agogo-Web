package engine

import (
	"testing"
)

// TestPathFullWorkflow exercises the complete vector path workflow end-to-end:
// pen tool drawing, direct selection editing, boolean combine, make selection, and undo.
func TestPathFullWorkflow(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// 1. Activate pen tool.
	_, err := DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set tool: %v", err)
	}

	// 2. Draw a rectangle path with 4 corner anchors.
	for _, pt := range [][2]float64{{10, 10}, {90, 10}, {90, 90}, {10, 90}} {
		_, err := DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
		if err != nil {
			t.Fatalf("pen click at (%.0f,%.0f): %v", pt[0], pt[1], err)
		}
	}

	// 3. Close the path.
	result, err := DispatchCommand(h, commandPenToolClose, "{}")
	if err != nil {
		t.Fatalf("pen close: %v", err)
	}
	// Verify we have 1 path with a closed subpath of 4 points.
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path after drawing, got %d", len(result.UIMeta.Paths))
	}

	// Verify the underlying data: 1 closed subpath, 4 corner points.
	inst := instances[h]
	doc := inst.manager.Active()
	np := doc.Paths[0]
	if len(np.Path.Subpaths) != 1 {
		t.Fatalf("expected 1 subpath, got %d", len(np.Path.Subpaths))
	}
	sp := np.Path.Subpaths[0]
	if !sp.Closed {
		t.Fatal("expected subpath to be closed")
	}
	if len(sp.Points) != 4 {
		t.Fatalf("expected 4 points, got %d", len(sp.Points))
	}

	// 4. Switch to direct selection, move anchor index 2 from (90,90) to (95,95).
	_, err = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "direct-select"}))
	if err != nil {
		t.Fatalf("set direct-select tool: %v", err)
	}
	_, err = DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 2, HandleKind: "anchor", X: 95, Y: 95,
	}))
	if err != nil {
		t.Fatalf("move anchor: %v", err)
	}

	// Verify the anchor moved.
	doc = inst.manager.Active()
	movedPt := doc.Paths[0].Path.Subpaths[0].Points[2]
	if movedPt.X != 95 || movedPt.Y != 95 {
		t.Fatalf("expected anchor at (95,95), got (%.0f,%.0f)", movedPt.X, movedPt.Y)
	}

	// 5. Create a second path named "Inner" and draw a smaller rectangle.
	_, err = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set pen tool for second path: %v", err)
	}
	_, err = DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "Inner"}))
	if err != nil {
		t.Fatalf("create path: %v", err)
	}
	for _, pt := range [][2]float64{{30, 30}, {70, 30}, {70, 70}, {30, 70}} {
		_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
		if err != nil {
			t.Fatalf("pen click inner at (%.0f,%.0f): %v", pt[0], pt[1], err)
		}
	}
	result, err = DispatchCommand(h, commandPenToolClose, "{}")
	if err != nil {
		t.Fatalf("pen close inner: %v", err)
	}
	// Should now have 2 paths.
	if len(result.UIMeta.Paths) != 2 {
		t.Fatalf("expected 2 paths before combine, got %d", len(result.UIMeta.Paths))
	}

	// 6. Combine the two paths (boolean union).
	// Active path is 1 ("Inner"), combine merges active + next (wrapping to 0).
	result, err = DispatchCommand(h, commandPathCombine, mustJSON(t, PathBooleanPayload{}))
	if err != nil {
		t.Fatalf("path combine: %v", err)
	}
	// After combine: should have 1 path with 2 subpaths.
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path after combine, got %d", len(result.UIMeta.Paths))
	}
	doc = inst.manager.Active()
	if len(doc.Paths[0].Path.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths after combine, got %d", len(doc.Paths[0].Path.Subpaths))
	}

	// 7. Make selection from the combined path.
	result, err = DispatchCommand(h, commandMakeSelectionFromPath, mustJSON(t, MakeSelectionFromPathPayload{AntiAlias: true}))
	if err != nil {
		t.Fatalf("make selection: %v", err)
	}
	if !result.UIMeta.Selection.Active {
		t.Fatal("expected active selection after make-selection-from-path")
	}

	// 8. Undo should revert the selection.
	result, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo selection: %v", err)
	}
	if result.UIMeta.Selection.Active {
		t.Fatal("expected no selection after undo")
	}

	// 9. Undo again should revert the combine — back to 2 paths.
	result, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo combine: %v", err)
	}
	if len(result.UIMeta.Paths) != 2 {
		t.Fatalf("expected 2 paths after undoing combine, got %d", len(result.UIMeta.Paths))
	}
}

// TestPathCurvedFillWorkflow draws a curved path with smooth anchors and fills it.
func TestPathCurvedFillWorkflow(t *testing.T) {
	h := Init("")
	if h <= 0 {
		t.Fatalf("Init returned invalid handle %d", h)
	}
	defer Free(h)

	// Create a small 100x100 document with a white background.
	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "CurveFill", Width: 100, Height: 100, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8, Background: "white",
	}))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	// Add a pixel layer to paint on.
	pixels := make([]byte, 100*100*4)
	_, err = DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "Paint",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 100, H: 100},
		Pixels:    pixels,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Activate pen tool.
	_, err = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set pen tool: %v", err)
	}

	// First point: smooth anchor with drag (creates bezier handles).
	dragX, dragY := 30.0, 10.0
	_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{
		X: 10, Y: 50, DragX: &dragX, DragY: &dragY,
	}))
	if err != nil {
		t.Fatalf("pen click+drag: %v", err)
	}

	// Second point: corner.
	_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: 90, Y: 50}))
	if err != nil {
		t.Fatalf("pen click corner: %v", err)
	}

	// Close the path.
	_, err = DispatchCommand(h, commandPenToolClose, "{}")
	if err != nil {
		t.Fatalf("pen close: %v", err)
	}

	// Verify the first point is a smooth anchor.
	inst := instances[h]
	doc := inst.manager.Active()
	pt0 := doc.Paths[0].Path.Subpaths[0].Points[0]
	if pt0.HandleType != HandleSmooth {
		t.Fatalf("expected first point HandleSmooth, got %d", pt0.HandleType)
	}

	// Fill the path with red.
	red := [4]uint8{255, 0, 0, 255}
	_, err = DispatchCommand(h, commandFillPath, mustJSON(t, FillPathPayload{
		Color: red,
	}))
	if err != nil {
		t.Fatalf("fill path: %v", err)
	}

	// Verify that pixels were actually filled — check some pixel on the active layer.
	doc = inst.manager.Active()
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		t.Fatal("active pixel layer not found after fill")
	}

	// The path is a 2-point closed curve from (10,50) to (90,50).
	// Pixels near the center horizontal line should be filled.
	off := (50*100 + 50) * 4
	a := layer.Pixels[off+3]
	if a == 0 {
		t.Error("expected non-zero alpha at center (50,50) after fill")
	}
}
