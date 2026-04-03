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
