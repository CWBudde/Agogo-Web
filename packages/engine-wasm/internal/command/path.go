package command

const (
	commandSetActiveTool         int32 = 0x0600
	commandPenToolClick          int32 = 0x0601
	commandPenToolClose          int32 = 0x0602
	commandDirectSelectMove      int32 = 0x0603
	commandDirectSelectMarquee   int32 = 0x0604
	commandBreakHandle           int32 = 0x0605
	commandDeleteAnchor          int32 = 0x0606
	commandAddAnchorOnSegment    int32 = 0x0607
	commandPathCombine           int32 = 0x0610
	commandPathSubtract          int32 = 0x0611
	commandPathIntersect         int32 = 0x0612
	commandPathExclude           int32 = 0x0613
	commandFlattenPath           int32 = 0x0614
	commandRasterizePath         int32 = 0x0615
	commandRasterizeLayer        int32 = 0x0616
	commandCreatePath            int32 = 0x0620
	commandDeletePath            int32 = 0x0621
	commandRenamePath            int32 = 0x0622
	commandDuplicatePath         int32 = 0x0623
	commandMakeSelectionFromPath int32 = 0x0624
	commandStrokePath            int32 = 0x0625
	commandFillPath              int32 = 0x0626
	commandSetActivePath         int32 = 0x0627
)

type PathSetActiveToolPayload struct {
	Tool string `json:"tool"`
}

type PathPenToolClickPayload struct {
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	DragX *float64 `json:"dragX,omitempty"`
	DragY *float64 `json:"dragY,omitempty"`
	Shift bool     `json:"shift,omitempty"`
}

type PathDirectSelectMovePayload struct {
	SubpathIndex int     `json:"subpathIndex"`
	AnchorIndex  int     `json:"anchorIndex"`
	HandleKind   string  `json:"handleKind"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
}

type PathDirectSelectMarqueePayload struct {
	X1    float64 `json:"x1"`
	Y1    float64 `json:"y1"`
	X2    float64 `json:"x2"`
	Y2    float64 `json:"y2"`
	Shift bool    `json:"shift,omitempty"`
}

type PathBreakHandlePayload struct {
	SubpathIndex int `json:"subpathIndex"`
	AnchorIndex  int `json:"anchorIndex"`
}

type PathDeleteAnchorPayload struct {
	SubpathIndex  int   `json:"subpathIndex"`
	AnchorIndices []int `json:"anchorIndices"`
}

type PathAddAnchorOnSegmentPayload struct {
	SubpathIndex int     `json:"subpathIndex"`
	SegmentIndex int     `json:"segmentIndex"`
	T            float64 `json:"t"`
}

type PathBooleanPayload struct {
	PathIndexA int `json:"pathIndexA,omitempty"`
	PathIndexB int `json:"pathIndexB,omitempty"`
}

type pathCreatePayload struct {
	Name string `json:"name"`
}

type pathDeletePayload struct {
	PathIndex int `json:"pathIndex"`
}

type pathRenamePayload struct {
	PathIndex int    `json:"pathIndex"`
	Name      string `json:"name"`
}

type pathDuplicatePayload struct {
	PathIndex int `json:"pathIndex"`
}

type PathMakeSelectionPayload struct {
	PathIndex     *int    `json:"pathIndex,omitempty"`
	FeatherRadius float64 `json:"featherRadius,omitempty"`
	AntiAlias     bool    `json:"antiAlias,omitempty"`
}

type PathRasterizeLayerPayload struct {
	LayerID string `json:"layerId,omitempty"`
}

type PathFillPayload struct {
	PathIndex *int     `json:"pathIndex,omitempty"`
	Color     [4]uint8 `json:"color,omitempty"`
}

type PathStrokePayload struct {
	PathIndex *int     `json:"pathIndex,omitempty"`
	ToolWidth float64  `json:"toolWidth,omitempty"`
	Color     [4]uint8 `json:"color,omitempty"`
}

type pathSetActivePayload struct {
	PathIndex int `json:"pathIndex"`
}

type PathCRUDDeps struct {
	Decode        func(string, any) error
	CreatePath    func(name string) error
	DeletePath    func(pathIndex int) error
	RenamePath    func(pathIndex int, name string) error
	DuplicatePath func(pathIndex int) error
}

type PathDeps struct {
	Decode                func(string, any) error
	SetActiveTool         func(tool string) error
	PenToolClick          func(PathPenToolClickPayload) error
	PenToolClose          func() error
	DirectSelectMove      func(PathDirectSelectMovePayload) error
	DirectSelectMarquee   func(PathDirectSelectMarqueePayload) error
	BreakHandle           func(PathBreakHandlePayload) error
	DeleteAnchor          func(PathDeleteAnchorPayload) error
	AddAnchorOnSegment    func(PathAddAnchorOnSegmentPayload) error
	PathBoolean           func(op string, payload PathBooleanPayload) error
	FlattenPath           func() error
	RasterizePath         func(pathIndex *int) error
	RasterizeLayer        func(layerID string) error
	CreatePath            func(name string) error
	DeletePath            func(pathIndex int) error
	RenamePath            func(pathIndex int, name string) error
	DuplicatePath         func(pathIndex int) error
	MakeSelectionFromPath func(pathIndex *int) error
	FillPath              func(pathIndex *int, color [4]uint8) error
	StrokePath            func(pathIndex *int, toolWidth float64, color [4]uint8) error
	SetActivePath         func(pathIndex int) error
}

func DispatchPathCRUD(commandID int32, payloadJSON string, deps PathCRUDDeps) (bool, error) {
	switch commandID {
	case commandCreatePath:
		var payload pathCreatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.CreatePath(payload.Name)

	case commandDeletePath:
		var payload pathDeletePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeletePath(payload.PathIndex)

	case commandRenamePath:
		var payload pathRenamePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.RenamePath(payload.PathIndex, payload.Name)

	case commandDuplicatePath:
		var payload pathDuplicatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DuplicatePath(payload.PathIndex)
	}

	return false, nil
}

func DispatchPath(commandID int32, payloadJSON string, deps PathDeps) (bool, error) {
	switch commandID {
	case commandSetActiveTool:
		var payload PathSetActiveToolPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetActiveTool(payload.Tool)
	case commandPenToolClick:
		var payload PathPenToolClickPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PenToolClick(payload)
	case commandPenToolClose:
		return true, deps.PenToolClose()
	case commandDirectSelectMove:
		var payload PathDirectSelectMovePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DirectSelectMove(payload)
	case commandDirectSelectMarquee:
		var payload PathDirectSelectMarqueePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DirectSelectMarquee(payload)
	case commandBreakHandle:
		var payload PathBreakHandlePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.BreakHandle(payload)
	case commandDeleteAnchor:
		var payload PathDeleteAnchorPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeleteAnchor(payload)
	case commandAddAnchorOnSegment:
		var payload PathAddAnchorOnSegmentPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.AddAnchorOnSegment(payload)
	case commandPathCombine:
		var payload PathBooleanPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PathBoolean("combine", payload)
	case commandPathSubtract:
		var payload PathBooleanPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PathBoolean("subtract", payload)
	case commandPathIntersect:
		var payload PathBooleanPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PathBoolean("intersect", payload)
	case commandPathExclude:
		var payload PathBooleanPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PathBoolean("exclude", payload)
	case commandFlattenPath:
		return true, deps.FlattenPath()
	case commandRasterizePath:
		var payload PathMakeSelectionPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.RasterizePath(payload.PathIndex)
	case commandRasterizeLayer:
		var payload PathRasterizeLayerPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.RasterizeLayer(payload.LayerID)
	case commandCreatePath:
		var payload pathCreatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.CreatePath(payload.Name)
	case commandDeletePath:
		var payload pathDeletePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeletePath(payload.PathIndex)
	case commandRenamePath:
		var payload pathRenamePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.RenamePath(payload.PathIndex, payload.Name)
	case commandDuplicatePath:
		var payload pathDuplicatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DuplicatePath(payload.PathIndex)
	case commandMakeSelectionFromPath:
		var payload PathMakeSelectionPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.MakeSelectionFromPath(payload.PathIndex)
	case commandFillPath:
		var payload PathFillPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.FillPath(payload.PathIndex, payload.Color)
	case commandStrokePath:
		var payload PathStrokePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.StrokePath(payload.PathIndex, payload.ToolWidth, payload.Color)
	case commandSetActivePath:
		var payload pathSetActivePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetActivePath(payload.PathIndex)
	default:
		return false, nil
	}
}
