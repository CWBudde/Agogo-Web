package engine

import (
	"fmt"
	"math"
	"sort"
)

// pathToolState holds engine-side state for pen and direct selection tools.
type pathToolState struct {
	activeTool       string       // "pen", "direct-select", or ""
	activeSubpathIdx int          // subpath being extended (pen tool)
	selectedAnchors  map[int]bool // which anchors are selected (direct select)
	cursorDocX       float64      // last cursor position in doc space
	cursorDocY       float64
}

func newPathToolState() *pathToolState {
	return &pathToolState{
		selectedAnchors: make(map[int]bool),
	}
}

// anchorKey encodes a (subpathIndex, anchorIndex) pair into the key
// used by selectedAnchors. Matches the convention in path_overlay.go.
func anchorKey(subpathIdx, anchorIdx int) int {
	return subpathIdx*10000 + anchorIdx
}

// --- Payloads ---

// SetActiveToolPayload is the JSON payload for commandSetActiveTool.
type SetActiveToolPayload struct {
	Tool string `json:"tool"`
}

// PenToolClickPayload is the JSON payload for commandPenToolClick.
type PenToolClickPayload struct {
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	DragX *float64 `json:"dragX,omitempty"`
	DragY *float64 `json:"dragY,omitempty"`
	Shift bool     `json:"shift,omitempty"`
}

// PenToolClosePayload is the (empty) JSON payload for commandPenToolClose.
type PenToolClosePayload struct{}

// DirectSelectMovePayload is the JSON payload for commandDirectSelectMove.
type DirectSelectMovePayload struct {
	SubpathIndex int     `json:"subpathIndex"`
	AnchorIndex  int     `json:"anchorIndex"`
	HandleKind   string  `json:"handleKind"` // "anchor", "in", "out"
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
}

// DirectSelectMarqueePayload is the JSON payload for commandDirectSelectMarquee.
type DirectSelectMarqueePayload struct {
	X1    float64 `json:"x1"`
	Y1    float64 `json:"y1"`
	X2    float64 `json:"x2"`
	Y2    float64 `json:"y2"`
	Shift bool    `json:"shift,omitempty"`
}

// BreakHandlePayload is the JSON payload for commandBreakHandle.
type BreakHandlePayload struct {
	SubpathIndex int `json:"subpathIndex"`
	AnchorIndex  int `json:"anchorIndex"`
}

// DeleteAnchorPayload is the JSON payload for commandDeleteAnchor.
type DeleteAnchorPayload struct {
	SubpathIndex  int   `json:"subpathIndex"`
	AnchorIndices []int `json:"anchorIndices"`
}

// AddAnchorOnSegmentPayload is the JSON payload for commandAddAnchorOnSegment.
type AddAnchorOnSegmentPayload struct {
	SubpathIndex int     `json:"subpathIndex"`
	SegmentIndex int     `json:"segmentIndex"`
	T            float64 `json:"t"` // Parameter [0,1]
}

// --- Pen tool logic ---

// penToolClick adds a new anchor point to the active path. If no path exists,
// it auto-creates a "Work Path". If the last subpath is closed, a new subpath
// is started automatically.
func (inst *instance) penToolClick(payload PenToolClickPayload) error {
	return inst.executeDocCommand("Pen tool: add anchor", func(doc *Document) error {
		// Auto-create "Work Path" if no paths exist.
		if len(doc.Paths) == 0 {
			doc.CreatePath("Work Path")
			inst.pathTool.activeSubpathIdx = 0
		}

		np := &doc.Paths[doc.ActivePathIdx]
		p := &np.Path

		// If there are no subpaths yet, or the current subpath is closed, start a new one.
		if len(p.Subpaths) == 0 ||
			inst.pathTool.activeSubpathIdx >= len(p.Subpaths) ||
			p.Subpaths[inst.pathTool.activeSubpathIdx].Closed {
			p.Subpaths = append(p.Subpaths, Subpath{})
			inst.pathTool.activeSubpathIdx = len(p.Subpaths) - 1
		}

		sp := &p.Subpaths[inst.pathTool.activeSubpathIdx]

		pt := PathPoint{
			X: payload.X, Y: payload.Y,
			InX: payload.X, InY: payload.Y,
			OutX: payload.X, OutY: payload.Y,
			HandleType: HandleCorner,
		}

		if payload.DragX != nil && payload.DragY != nil {
			pt.HandleType = HandleSmooth
			pt.OutX = *payload.DragX
			pt.OutY = *payload.DragY
			// Mirror: In = 2*anchor - Out
			pt.InX = 2*payload.X - *payload.DragX
			pt.InY = 2*payload.Y - *payload.DragY
		}

		sp.Points = append(sp.Points, pt)
		return nil
	})
}

// penToolClose closes the active subpath and advances the subpath index so the
// next click starts a new subpath.
func (inst *instance) penToolClose() error {
	return inst.executeDocCommand("Pen tool: close path", func(doc *Document) error {
		if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
			return nil
		}
		p := &doc.Paths[doc.ActivePathIdx].Path
		idx := inst.pathTool.activeSubpathIdx
		if idx < 0 || idx >= len(p.Subpaths) {
			return nil
		}
		p.Subpaths[idx].Closed = true
		// Advance so next click starts a new subpath.
		inst.pathTool.activeSubpathIdx = idx + 1
		return nil
	})
}

// --- Direct selection tool logic ---

// mirrorHandle adjusts the opposite handle of a PathPoint after one handle
// has been moved, maintaining the smooth/symmetric constraint.
// movedHandle is "in" or "out".
func mirrorHandle(pt *PathPoint, movedHandle string) {
	if movedHandle == "out" {
		dx := pt.OutX - pt.X
		dy := pt.OutY - pt.Y
		if pt.HandleType == HandleSymmetric {
			pt.InX = pt.X - dx
			pt.InY = pt.Y - dy
		} else { // HandleSmooth
			inLen := math.Hypot(pt.InX-pt.X, pt.InY-pt.Y)
			outLen := math.Hypot(dx, dy)
			if outLen > 0 {
				scale := inLen / outLen
				pt.InX = pt.X - dx*scale
				pt.InY = pt.Y - dy*scale
			}
		}
	} else { // "in"
		dx := pt.InX - pt.X
		dy := pt.InY - pt.Y
		if pt.HandleType == HandleSymmetric {
			pt.OutX = pt.X - dx
			pt.OutY = pt.Y - dy
		} else { // HandleSmooth
			outLen := math.Hypot(pt.OutX-pt.X, pt.OutY-pt.Y)
			inLen := math.Hypot(dx, dy)
			if inLen > 0 {
				scale := outLen / inLen
				pt.OutX = pt.X - dx*scale
				pt.OutY = pt.Y - dy*scale
			}
		}
	}
}

// directSelectMove moves an anchor or control handle to a new position.
func (inst *instance) directSelectMove(payload DirectSelectMovePayload) error {
	return inst.executeDocCommand("Direct select: move", func(doc *Document) error {
		if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
			return fmt.Errorf("no active path")
		}
		p := &doc.Paths[doc.ActivePathIdx].Path
		if payload.SubpathIndex < 0 || payload.SubpathIndex >= len(p.Subpaths) {
			return fmt.Errorf("subpath index %d out of range", payload.SubpathIndex)
		}
		sp := &p.Subpaths[payload.SubpathIndex]
		if payload.AnchorIndex < 0 || payload.AnchorIndex >= len(sp.Points) {
			return fmt.Errorf("anchor index %d out of range", payload.AnchorIndex)
		}
		pt := &sp.Points[payload.AnchorIndex]

		switch payload.HandleKind {
		case "anchor":
			dx := payload.X - pt.X
			dy := payload.Y - pt.Y
			pt.X = payload.X
			pt.Y = payload.Y
			pt.InX += dx
			pt.InY += dy
			pt.OutX += dx
			pt.OutY += dy

		case "out":
			pt.OutX = payload.X
			pt.OutY = payload.Y
			if pt.HandleType == HandleSmooth || pt.HandleType == HandleSymmetric {
				mirrorHandle(pt, "out")
			}

		case "in":
			pt.InX = payload.X
			pt.InY = payload.Y
			if pt.HandleType == HandleSmooth || pt.HandleType == HandleSymmetric {
				mirrorHandle(pt, "in")
			}

		default:
			return fmt.Errorf("unknown handleKind %q", payload.HandleKind)
		}
		return nil
	})
}

// directSelectMarquee selects all anchors within the given rectangle.
func (inst *instance) directSelectMarquee(payload DirectSelectMarqueePayload) error {
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
		return fmt.Errorf("no active path")
	}
	p := &doc.Paths[doc.ActivePathIdx].Path

	// Normalise rectangle.
	minX := math.Min(payload.X1, payload.X2)
	maxX := math.Max(payload.X1, payload.X2)
	minY := math.Min(payload.Y1, payload.Y2)
	maxY := math.Max(payload.Y1, payload.Y2)

	if !payload.Shift {
		inst.pathTool.selectedAnchors = make(map[int]bool)
	}

	for spIdx, sp := range p.Subpaths {
		for aIdx, pt := range sp.Points {
			if pt.X >= minX && pt.X <= maxX && pt.Y >= minY && pt.Y <= maxY {
				inst.pathTool.selectedAnchors[anchorKey(spIdx, aIdx)] = true
			}
		}
	}
	return nil
}

// breakHandle converts a smooth/symmetric anchor to a corner anchor.
func (inst *instance) breakHandle(payload BreakHandlePayload) error {
	return inst.executeDocCommand("Break handle", func(doc *Document) error {
		if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
			return fmt.Errorf("no active path")
		}
		p := &doc.Paths[doc.ActivePathIdx].Path
		if payload.SubpathIndex < 0 || payload.SubpathIndex >= len(p.Subpaths) {
			return fmt.Errorf("subpath index %d out of range", payload.SubpathIndex)
		}
		sp := &p.Subpaths[payload.SubpathIndex]
		if payload.AnchorIndex < 0 || payload.AnchorIndex >= len(sp.Points) {
			return fmt.Errorf("anchor index %d out of range", payload.AnchorIndex)
		}
		sp.Points[payload.AnchorIndex].HandleType = HandleCorner
		return nil
	})
}

// deleteAnchor removes specified anchors from a subpath. If the subpath
// becomes empty it is removed. If all subpaths are removed, the path is removed.
func (inst *instance) deleteAnchor(payload DeleteAnchorPayload) error {
	return inst.executeDocCommand("Delete anchor", func(doc *Document) error {
		if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
			return fmt.Errorf("no active path")
		}
		p := &doc.Paths[doc.ActivePathIdx].Path
		if payload.SubpathIndex < 0 || payload.SubpathIndex >= len(p.Subpaths) {
			return fmt.Errorf("subpath index %d out of range", payload.SubpathIndex)
		}
		sp := &p.Subpaths[payload.SubpathIndex]

		// Build set of indices to remove, sort descending so removal is stable.
		toRemove := make(map[int]bool, len(payload.AnchorIndices))
		for _, idx := range payload.AnchorIndices {
			if idx < 0 || idx >= len(sp.Points) {
				return fmt.Errorf("anchor index %d out of range", idx)
			}
			toRemove[idx] = true
		}

		sorted := make([]int, 0, len(toRemove))
		for idx := range toRemove {
			sorted = append(sorted, idx)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(sorted)))

		for _, idx := range sorted {
			sp.Points = append(sp.Points[:idx], sp.Points[idx+1:]...)
		}

		// Clear selection for removed anchors.
		for _, idx := range payload.AnchorIndices {
			delete(inst.pathTool.selectedAnchors, anchorKey(payload.SubpathIndex, idx))
		}

		// Remove empty subpath.
		if len(sp.Points) == 0 {
			p.Subpaths = append(p.Subpaths[:payload.SubpathIndex], p.Subpaths[payload.SubpathIndex+1:]...)
		}

		// Remove empty path.
		if len(p.Subpaths) == 0 {
			doc.Paths = append(doc.Paths[:doc.ActivePathIdx], doc.Paths[doc.ActivePathIdx+1:]...)
			if doc.ActivePathIdx >= len(doc.Paths) {
				doc.ActivePathIdx = len(doc.Paths) - 1
			}
		}

		return nil
	})
}

// addAnchorOnSegment splits a cubic Bezier segment at parameter t using
// De Casteljau's algorithm, inserting a new anchor point.
func (inst *instance) addAnchorOnSegment(payload AddAnchorOnSegmentPayload) error {
	return inst.executeDocCommand("Add anchor on segment", func(doc *Document) error {
		if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
			return fmt.Errorf("no active path")
		}
		p := &doc.Paths[doc.ActivePathIdx].Path
		if payload.SubpathIndex < 0 || payload.SubpathIndex >= len(p.Subpaths) {
			return fmt.Errorf("subpath index %d out of range", payload.SubpathIndex)
		}
		sp := &p.Subpaths[payload.SubpathIndex]

		numSegments := len(sp.Points) - 1
		if sp.Closed && len(sp.Points) > 0 {
			numSegments = len(sp.Points)
		}
		if payload.SegmentIndex < 0 || payload.SegmentIndex >= numSegments {
			return fmt.Errorf("segment index %d out of range (have %d segments)", payload.SegmentIndex, numSegments)
		}
		if payload.T <= 0 || payload.T >= 1 {
			return fmt.Errorf("t must be in (0,1), got %f", payload.T)
		}

		i0 := payload.SegmentIndex
		i1 := (payload.SegmentIndex + 1) % len(sp.Points)

		p0 := &sp.Points[i0]
		p1 := &sp.Points[i1]

		// De Casteljau on cubic: P0, P0.Out, P1.In, P1
		ax, ay := p0.X, p0.Y
		bx, by := p0.OutX, p0.OutY
		cx, cy := p1.InX, p1.InY
		dx, dy := p1.X, p1.Y

		t := payload.T

		// First level
		abx := lerp(ax, bx, t)
		aby := lerp(ay, by, t)
		bcx := lerp(bx, cx, t)
		bcy := lerp(by, cy, t)
		cdx := lerp(cx, dx, t)
		cdy := lerp(cy, dy, t)

		// Second level
		abcx := lerp(abx, bcx, t)
		abcy := lerp(aby, bcy, t)
		bcdx := lerp(bcx, cdx, t)
		bcdy := lerp(bcy, cdy, t)

		// Third level — the split point
		mx := lerp(abcx, bcdx, t)
		my := lerp(abcy, bcdy, t)

		// Update existing points' handles.
		p0.OutX = abx
		p0.OutY = aby
		p1.InX = cdx
		p1.InY = cdy

		// Create the new mid-point.
		mid := PathPoint{
			X:          mx,
			Y:          my,
			InX:        abcx,
			InY:        abcy,
			OutX:       bcdx,
			OutY:       bcdy,
			HandleType: HandleSmooth,
		}

		// Insert mid after i0. For closed paths where i1 == 0, insert at end.
		insertIdx := i0 + 1
		sp.Points = append(sp.Points, PathPoint{})
		copy(sp.Points[insertIdx+1:], sp.Points[insertIdx:])
		sp.Points[insertIdx] = mid

		return nil
	})
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}
