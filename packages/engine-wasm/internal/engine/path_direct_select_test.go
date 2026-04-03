package engine

import (
	"math"
	"testing"
)

// helper: create a path with one open subpath containing the given points.
func setupPathWithPoints(t *testing.T, points []PathPoint) (int32, *instance) {
	t.Helper()
	h := initWithDefaultDoc(t)

	inst := instances[h]
	doc := inst.manager.Active()
	doc.CreatePath("Test Path")
	np := &doc.Paths[doc.ActivePathIdx]
	np.Path.Subpaths = []Subpath{{Points: points}}
	// Persist the changes back into the document manager.
	if err := inst.manager.ReplaceActive(doc); err != nil {
		t.Fatalf("ReplaceActive: %v", err)
	}

	return h, inst
}

func TestDirectSelectMoveAnchor(t *testing.T) {
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 100, Y: 100, InX: 90, InY: 100, OutX: 110, OutY: 100, HandleType: HandleSmooth},
		{X: 200, Y: 200, InX: 200, InY: 200, OutX: 200, OutY: 200, HandleType: HandleCorner},
	})
	defer Free(h)

	// Move first anchor from (100,100) to (120,130).
	_, err := DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 0, HandleKind: "anchor", X: 120, Y: 130,
	}))
	if err != nil {
		t.Fatalf("directSelectMove: %v", err)
	}

	doc := inst.manager.Active()
	pt := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0].Points[0]

	if pt.X != 120 || pt.Y != 130 {
		t.Fatalf("expected anchor (120,130), got (%.1f,%.1f)", pt.X, pt.Y)
	}
	// Handles should have moved by the same delta (dx=20, dy=30).
	if pt.InX != 110 || pt.InY != 130 {
		t.Fatalf("expected In (110,130), got (%.1f,%.1f)", pt.InX, pt.InY)
	}
	if pt.OutX != 130 || pt.OutY != 130 {
		t.Fatalf("expected Out (130,130), got (%.1f,%.1f)", pt.OutX, pt.OutY)
	}
}

func TestDirectSelectMoveOutHandleSmooth(t *testing.T) {
	// Smooth anchor at (100,100) with In at (80,100) and Out at (120,100).
	// Both handles have length 20.
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 100, Y: 100, InX: 80, InY: 100, OutX: 120, OutY: 100, HandleType: HandleSmooth},
		{X: 200, Y: 200, InX: 200, InY: 200, OutX: 200, OutY: 200, HandleType: HandleCorner},
	})
	defer Free(h)

	// Move Out handle to (100, 120) — straight down from anchor.
	_, err := DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 0, HandleKind: "out", X: 100, Y: 120,
	}))
	if err != nil {
		t.Fatalf("directSelectMove: %v", err)
	}

	doc := inst.manager.Active()
	pt := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0].Points[0]

	if pt.OutX != 100 || pt.OutY != 120 {
		t.Fatalf("expected Out (100,120), got (%.1f,%.1f)", pt.OutX, pt.OutY)
	}
	// Smooth: In handle should mirror direction, keep original length (20).
	// Out direction is (0, 20), so In should be (0, -20) scaled to length 20 = (0, -20).
	// In = anchor - direction * (inLen/outLen) = (100, 100) - (0, 20) * (20/20) = (100, 80).
	if math.Abs(pt.InX-100) > 0.001 || math.Abs(pt.InY-80) > 0.001 {
		t.Fatalf("expected In (100,80), got (%.4f,%.4f)", pt.InX, pt.InY)
	}
}

func TestDirectSelectMoveOutHandleSymmetric(t *testing.T) {
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 100, Y: 100, InX: 80, InY: 100, OutX: 120, OutY: 100, HandleType: HandleSymmetric},
		{X: 200, Y: 200, InX: 200, InY: 200, OutX: 200, OutY: 200, HandleType: HandleCorner},
	})
	defer Free(h)

	// Move Out handle to (130, 110).
	_, err := DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 0, HandleKind: "out", X: 130, Y: 110,
	}))
	if err != nil {
		t.Fatalf("directSelectMove: %v", err)
	}

	doc := inst.manager.Active()
	pt := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0].Points[0]

	// Symmetric: In = 2*anchor - Out = (200-130, 200-110) = (70, 90).
	if pt.InX != 70 || pt.InY != 90 {
		t.Fatalf("expected In (70,90), got (%.1f,%.1f)", pt.InX, pt.InY)
	}
}

func TestDirectSelectMoveOutHandleCorner(t *testing.T) {
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 100, Y: 100, InX: 80, InY: 100, OutX: 120, OutY: 100, HandleType: HandleCorner},
		{X: 200, Y: 200, InX: 200, InY: 200, OutX: 200, OutY: 200, HandleType: HandleCorner},
	})
	defer Free(h)

	// Move Out handle — In should remain unchanged for corner type.
	_, err := DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 0, HandleKind: "out", X: 150, Y: 120,
	}))
	if err != nil {
		t.Fatalf("directSelectMove: %v", err)
	}

	doc := inst.manager.Active()
	pt := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0].Points[0]

	if pt.OutX != 150 || pt.OutY != 120 {
		t.Fatalf("expected Out (150,120), got (%.1f,%.1f)", pt.OutX, pt.OutY)
	}
	// In handle must be unchanged.
	if pt.InX != 80 || pt.InY != 100 {
		t.Fatalf("expected In (80,100) unchanged, got (%.1f,%.1f)", pt.InX, pt.InY)
	}
}

func TestBreakHandle(t *testing.T) {
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 100, Y: 100, InX: 80, InY: 100, OutX: 120, OutY: 100, HandleType: HandleSmooth},
	})
	defer Free(h)

	_, err := DispatchCommand(h, commandBreakHandle, mustJSON(t, BreakHandlePayload{
		SubpathIndex: 0, AnchorIndex: 0,
	}))
	if err != nil {
		t.Fatalf("breakHandle: %v", err)
	}

	doc := inst.manager.Active()
	pt := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0].Points[0]

	if pt.HandleType != HandleCorner {
		t.Fatalf("expected HandleCorner after break, got %d", pt.HandleType)
	}
	// Handle positions should be preserved.
	if pt.InX != 80 || pt.OutX != 120 {
		t.Fatalf("handles should be preserved: In=(%.0f), Out=(%.0f)", pt.InX, pt.OutX)
	}
}

func TestMarqueeSelect(t *testing.T) {
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 10, Y: 10, InX: 10, InY: 10, OutX: 10, OutY: 10, HandleType: HandleCorner},
		{X: 50, Y: 50, InX: 50, InY: 50, OutX: 50, OutY: 50, HandleType: HandleCorner},
		{X: 200, Y: 200, InX: 200, InY: 200, OutX: 200, OutY: 200, HandleType: HandleCorner},
	})
	defer Free(h)

	// Marquee selecting the area (0,0)-(100,100) should select first two points.
	_, err := DispatchCommand(h, commandDirectSelectMarquee, mustJSON(t, DirectSelectMarqueePayload{
		X1: 0, Y1: 0, X2: 100, Y2: 100,
	}))
	if err != nil {
		t.Fatalf("marqueeSelect: %v", err)
	}

	if !inst.pathTool.selectedAnchors[anchorKey(0, 0)] {
		t.Fatal("expected anchor 0 to be selected")
	}
	if !inst.pathTool.selectedAnchors[anchorKey(0, 1)] {
		t.Fatal("expected anchor 1 to be selected")
	}
	if inst.pathTool.selectedAnchors[anchorKey(0, 2)] {
		t.Fatal("expected anchor 2 to NOT be selected")
	}

	// Shift-marquee to add anchor 2.
	_, err = DispatchCommand(h, commandDirectSelectMarquee, mustJSON(t, DirectSelectMarqueePayload{
		X1: 150, Y1: 150, X2: 250, Y2: 250, Shift: true,
	}))
	if err != nil {
		t.Fatalf("marqueeSelect shift: %v", err)
	}

	// All three should now be selected.
	for i := 0; i < 3; i++ {
		if !inst.pathTool.selectedAnchors[anchorKey(0, i)] {
			t.Fatalf("expected anchor %d to be selected after shift-marquee", i)
		}
	}

	// Non-shift marquee should replace selection.
	_, err = DispatchCommand(h, commandDirectSelectMarquee, mustJSON(t, DirectSelectMarqueePayload{
		X1: 190, Y1: 190, X2: 210, Y2: 210,
	}))
	if err != nil {
		t.Fatalf("marqueeSelect replace: %v", err)
	}

	if inst.pathTool.selectedAnchors[anchorKey(0, 0)] {
		t.Fatal("expected anchor 0 to be deselected after non-shift marquee")
	}
	if !inst.pathTool.selectedAnchors[anchorKey(0, 2)] {
		t.Fatal("expected anchor 2 to be selected")
	}
}

func TestDeleteAnchor(t *testing.T) {
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 10, Y: 10, InX: 10, InY: 10, OutX: 10, OutY: 10, HandleType: HandleCorner},
		{X: 50, Y: 50, InX: 50, InY: 50, OutX: 50, OutY: 50, HandleType: HandleCorner},
		{X: 100, Y: 100, InX: 100, InY: 100, OutX: 100, OutY: 100, HandleType: HandleCorner},
	})
	defer Free(h)

	// Delete the middle anchor.
	_, err := DispatchCommand(h, commandDeleteAnchor, mustJSON(t, DeleteAnchorPayload{
		SubpathIndex: 0, AnchorIndices: []int{1},
	}))
	if err != nil {
		t.Fatalf("deleteAnchor: %v", err)
	}

	doc := inst.manager.Active()
	sp := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0]
	if len(sp.Points) != 2 {
		t.Fatalf("expected 2 points after delete, got %d", len(sp.Points))
	}
	if sp.Points[0].X != 10 || sp.Points[1].X != 100 {
		t.Fatalf("wrong remaining points: X=%.0f, X=%.0f", sp.Points[0].X, sp.Points[1].X)
	}
}

func TestDeleteAnchorRemovesSubpath(t *testing.T) {
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 10, Y: 10, InX: 10, InY: 10, OutX: 10, OutY: 10, HandleType: HandleCorner},
	})
	defer Free(h)

	// Delete the only anchor — should remove the subpath and the path.
	_, err := DispatchCommand(h, commandDeleteAnchor, mustJSON(t, DeleteAnchorPayload{
		SubpathIndex: 0, AnchorIndices: []int{0},
	}))
	if err != nil {
		t.Fatalf("deleteAnchor: %v", err)
	}

	doc := inst.manager.Active()
	if len(doc.Paths) != 0 {
		t.Fatalf("expected 0 paths after removing last anchor, got %d", len(doc.Paths))
	}
}

func TestAddAnchorOnSegment(t *testing.T) {
	// Straight line segment: (0,0) -> (100,0), no curves (handles at anchor positions).
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 0, Y: 0, InX: 0, InY: 0, OutX: 0, OutY: 0, HandleType: HandleCorner},
		{X: 100, Y: 0, InX: 100, InY: 0, OutX: 100, OutY: 0, HandleType: HandleCorner},
	})
	defer Free(h)

	// Split at t=0.5 — midpoint should be (50, 0).
	_, err := DispatchCommand(h, commandAddAnchorOnSegment, mustJSON(t, AddAnchorOnSegmentPayload{
		SubpathIndex: 0, SegmentIndex: 0, T: 0.5,
	}))
	if err != nil {
		t.Fatalf("addAnchorOnSegment: %v", err)
	}

	doc := inst.manager.Active()
	sp := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0]
	if len(sp.Points) != 3 {
		t.Fatalf("expected 3 points after split, got %d", len(sp.Points))
	}

	mid := sp.Points[1]
	if math.Abs(mid.X-50) > 0.001 || math.Abs(mid.Y) > 0.001 {
		t.Fatalf("expected mid at (50,0), got (%.4f,%.4f)", mid.X, mid.Y)
	}
	if mid.HandleType != HandleSmooth {
		t.Fatalf("expected HandleSmooth on new point, got %d", mid.HandleType)
	}
}

func TestAddAnchorOnCurvedSegment(t *testing.T) {
	// Cubic Bezier: P0=(0,0), Out=(0,100), In=(100,100), P1=(100,0).
	// This is a classic arch shape.
	h, inst := setupPathWithPoints(t, []PathPoint{
		{X: 0, Y: 0, InX: 0, InY: 0, OutX: 0, OutY: 100, HandleType: HandleCorner},
		{X: 100, Y: 0, InX: 100, InY: 100, OutX: 100, OutY: 0, HandleType: HandleCorner},
	})
	defer Free(h)

	_, err := DispatchCommand(h, commandAddAnchorOnSegment, mustJSON(t, AddAnchorOnSegmentPayload{
		SubpathIndex: 0, SegmentIndex: 0, T: 0.5,
	}))
	if err != nil {
		t.Fatalf("addAnchorOnSegment: %v", err)
	}

	doc := inst.manager.Active()
	sp := doc.Paths[doc.ActivePathIdx].Path.Subpaths[0]
	if len(sp.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(sp.Points))
	}

	mid := sp.Points[1]
	// At t=0.5 on this symmetric arch, X should be 50, Y should be 75.
	if math.Abs(mid.X-50) > 0.001 || math.Abs(mid.Y-75) > 0.001 {
		t.Fatalf("expected mid at (50,75), got (%.4f,%.4f)", mid.X, mid.Y)
	}
}
