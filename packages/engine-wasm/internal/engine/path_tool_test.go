package engine

import (
	"testing"
)

func TestPenToolAddCornerAnchors(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// Activate pen tool.
	_, err := DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set active tool: %v", err)
	}

	// Click three corner points (triangle).
	for _, pt := range [][2]float64{{100, 100}, {200, 100}, {150, 200}} {
		_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
		if err != nil {
			t.Fatalf("pen click at (%.0f, %.0f): %v", pt[0], pt[1], err)
		}
	}

	// Close the path.
	result, err := DispatchCommand(h, commandPenToolClose, "{}")
	if err != nil {
		t.Fatalf("pen close: %v", err)
	}

	// Verify: 1 path, 1 closed subpath, 3 corner points.
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(result.UIMeta.Paths))
	}

	// Read the document directly for detailed checks.
	inst := instances[h]
	doc := inst.manager.Active()
	if doc.ActivePathIdx != 0 {
		t.Fatalf("expected active path index 0, got %d", doc.ActivePathIdx)
	}
	np := doc.Paths[0]
	if np.Name != "Work Path" {
		t.Fatalf("expected auto-created 'Work Path', got %q", np.Name)
	}
	if len(np.Path.Subpaths) != 1 {
		t.Fatalf("expected 1 subpath, got %d", len(np.Path.Subpaths))
	}
	sp := np.Path.Subpaths[0]
	if !sp.Closed {
		t.Fatal("expected subpath to be closed")
	}
	if len(sp.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(sp.Points))
	}
	for i, pt := range sp.Points {
		if pt.HandleType != HandleCorner {
			t.Errorf("point %d: expected HandleCorner, got %d", i, pt.HandleType)
		}
	}
}

func TestPenToolSmoothAnchor(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	_, err := DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set active tool: %v", err)
	}

	// Click+drag: anchor at (100,100), drag to (150,100).
	dragX, dragY := 150.0, 100.0
	_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{
		X: 100, Y: 100, DragX: &dragX, DragY: &dragY,
	}))
	if err != nil {
		t.Fatalf("pen click+drag: %v", err)
	}

	inst := instances[h]
	doc := inst.manager.Active()
	sp := doc.Paths[0].Path.Subpaths[0]
	pt := sp.Points[0]

	if pt.HandleType != HandleSmooth {
		t.Fatalf("expected HandleSmooth, got %d", pt.HandleType)
	}
	if pt.OutX != 150 || pt.OutY != 100 {
		t.Fatalf("expected Out=(150,100), got (%.0f,%.0f)", pt.OutX, pt.OutY)
	}
	// Mirror: In = 2*anchor - Out = (200-150, 200-100) = (50, 100)
	if pt.InX != 50 || pt.InY != 100 {
		t.Fatalf("expected In=(50,100), got (%.0f,%.0f)", pt.InX, pt.InY)
	}
}

func TestPenToolOverlay(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// Activate pen tool and add two points.
	_, err := DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set active tool: %v", err)
	}

	_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: 10, Y: 20}))
	if err != nil {
		t.Fatalf("pen click: %v", err)
	}

	result, err := DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: 30, Y: 40}))
	if err != nil {
		t.Fatalf("pen click: %v", err)
	}

	if result.UIMeta.PathOverlay == nil {
		t.Fatal("expected non-nil PathOverlay")
	}
	if len(result.UIMeta.PathOverlay.Anchors) != 2 {
		t.Fatalf("expected 2 anchors, got %d", len(result.UIMeta.PathOverlay.Anchors))
	}
	if len(result.UIMeta.PathOverlay.Segments) != 1 {
		t.Fatalf("expected 1 segment polyline, got %d", len(result.UIMeta.PathOverlay.Segments))
	}
	// First anchor should be marked as "first" since the subpath is open.
	if !result.UIMeta.PathOverlay.Anchors[0].First {
		t.Fatal("expected first anchor to have First=true")
	}
	// Rubber band should be present (open subpath with pen tool active).
	if result.UIMeta.PathOverlay.RubberBand == nil {
		t.Fatal("expected non-nil RubberBand for open subpath")
	}
}

func TestPenToolAutoCreateWorkPath(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// No paths exist initially.
	inst := instances[h]
	doc := inst.manager.Active()
	if len(doc.Paths) != 0 {
		t.Fatalf("expected 0 paths initially, got %d", len(doc.Paths))
	}

	_, err := DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set active tool: %v", err)
	}

	_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: 50, Y: 60}))
	if err != nil {
		t.Fatalf("pen click: %v", err)
	}

	// Refresh doc pointer (executeDocCommand may have replaced it).
	doc = inst.manager.Active()
	if len(doc.Paths) != 1 {
		t.Fatalf("expected 1 path after auto-create, got %d", len(doc.Paths))
	}
	if doc.Paths[0].Name != "Work Path" {
		t.Fatalf("expected 'Work Path', got %q", doc.Paths[0].Name)
	}
}

func TestPenToolNewSubpathAfterClose(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	_, err := DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set active tool: %v", err)
	}

	// Draw a triangle.
	for _, pt := range [][2]float64{{0, 0}, {10, 0}, {5, 10}} {
		_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
		if err != nil {
			t.Fatalf("pen click: %v", err)
		}
	}
	_, err = DispatchCommand(h, commandPenToolClose, "{}")
	if err != nil {
		t.Fatalf("pen close: %v", err)
	}

	// Click again — should start a second subpath.
	_, err = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: 50, Y: 50}))
	if err != nil {
		t.Fatalf("pen click for new subpath: %v", err)
	}

	inst := instances[h]
	doc := inst.manager.Active()
	p := &doc.Paths[0].Path
	if len(p.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths, got %d", len(p.Subpaths))
	}
	if !p.Subpaths[0].Closed {
		t.Fatal("expected first subpath to be closed")
	}
	if p.Subpaths[1].Closed {
		t.Fatal("expected second subpath to be open")
	}
	if len(p.Subpaths[1].Points) != 1 {
		t.Fatalf("expected 1 point in second subpath, got %d", len(p.Subpaths[1].Points))
	}
}
