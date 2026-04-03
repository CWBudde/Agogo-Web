package engine

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
