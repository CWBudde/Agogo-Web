package command

const (
	commandCreateDocument int32 = 0x0001
	commandCloseDocument  int32 = 0x0002
	commandZoomSet        int32 = 0x0010
	commandPanSet         int32 = 0x0011
	commandRotateViewSet  int32 = 0x0012
	commandResize         int32 = 0x0013
	commandFitToView      int32 = 0x0014
	commandPointerEvent   int32 = 0x0015
	commandJumpHistory    int32 = 0x0016
	commandSetShowGuides  int32 = 0x0017
	commandFlattenImage   int32 = 0x0117
	commandOpenImageFile  int32 = 0x0118
	commandTranslateLayer int32 = 0x0119
	commandBeginTxn       int32 = 0xffe0
	commandEndTxn         int32 = 0xffe1
	commandClearHistory   int32 = 0xffe2
	commandUndo           int32 = 0xfff0
	commandRedo           int32 = 0xfff1
)

type CoreCreateDocumentPayload struct {
	Name       string  `json:"name"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Resolution float64 `json:"resolution"`
	ColorMode  string  `json:"colorMode"`
	BitDepth   int     `json:"bitDepth"`
	Background string  `json:"background"`
}

type CoreZoomPayload struct {
	Zoom      float64 `json:"zoom"`
	HasAnchor bool    `json:"hasAnchor"`
	AnchorX   float64 `json:"anchorX"`
	AnchorY   float64 `json:"anchorY"`
}

type CorePanPayload struct {
	CenterX float64 `json:"centerX"`
	CenterY float64 `json:"centerY"`
}

type CoreRotatePayload struct {
	Rotation float64 `json:"rotation"`
}

type CoreResizePayload struct {
	CanvasW          int     `json:"canvasW"`
	CanvasH          int     `json:"canvasH"`
	DevicePixelRatio float64 `json:"devicePixelRatio"`
}

type CorePointerEventPayload struct {
	Phase     string  `json:"phase"`
	PointerID int     `json:"pointerId"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Button    int     `json:"button"`
	Buttons   int     `json:"buttons"`
	PanMode   bool    `json:"panMode"`
	Pressure  float64 `json:"pressure"`
}

type coreBeginTransactionPayload struct {
	Description string `json:"description"`
}

type coreEndTransactionPayload struct {
	Commit bool `json:"commit"`
}

type coreJumpHistoryPayload struct {
	HistoryIndex int `json:"historyIndex"`
}

type coreSetShowGuidesPayload struct {
	Show bool `json:"show"`
}

type CoreOpenImageFilePayload struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Pixels []byte `json:"pixels"`
}

type CoreTranslateLayerPayload struct {
	LayerID string `json:"layerId"`
	DX      int    `json:"dx"`
	DY      int    `json:"dy"`
}

type CoreDeps struct {
	Decode         func(string, any) error
	CreateDocument func(CoreCreateDocumentPayload) error
	CloseDocument  func() error
	ZoomSet        func(CoreZoomPayload) error
	PanSet         func(CorePanPayload) error
	RotateViewSet  func(CoreRotatePayload) error
	Resize         func(CoreResizePayload) error
	PointerEvent   func(CorePointerEventPayload) error
	BeginTxn       func(description string) error
	EndTxn         func(commit bool) error
	JumpHistory    func(historyIndex int) error
	SetShowGuides  func(show bool) error
	ClearHistory   func() error
	FitToView      func() error
	Undo           func() error
	Redo           func() error
	FlattenImage   func() error
	OpenImageFile  func(CoreOpenImageFilePayload) error
	TranslateLayer func(CoreTranslateLayerPayload) error
}

func DispatchCore(commandID int32, payloadJSON string, deps CoreDeps) (bool, error) {
	switch commandID {
	case commandCreateDocument:
		var payload CoreCreateDocumentPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.CreateDocument(payload)

	case commandCloseDocument:
		return true, deps.CloseDocument()

	case commandZoomSet:
		var payload CoreZoomPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ZoomSet(payload)

	case commandPanSet:
		var payload CorePanPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PanSet(payload)

	case commandRotateViewSet:
		var payload CoreRotatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.RotateViewSet(payload)

	case commandResize:
		var payload CoreResizePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.Resize(payload)

	case commandPointerEvent:
		var payload CorePointerEventPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PointerEvent(payload)

	case commandBeginTxn:
		var payload coreBeginTransactionPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.BeginTxn(payload.Description)

	case commandEndTxn:
		var payload coreEndTransactionPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		commit := payload.Commit
		if payloadJSON == "" {
			commit = true
		}
		return true, deps.EndTxn(commit)

	case commandJumpHistory:
		var payload coreJumpHistoryPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.JumpHistory(payload.HistoryIndex)

	case commandSetShowGuides:
		var payload coreSetShowGuidesPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetShowGuides(payload.Show)

	case commandClearHistory:
		return true, deps.ClearHistory()

	case commandFitToView:
		return true, deps.FitToView()

	case commandUndo:
		return true, deps.Undo()

	case commandRedo:
		return true, deps.Redo()

	case commandFlattenImage:
		return true, deps.FlattenImage()

	case commandOpenImageFile:
		var payload CoreOpenImageFilePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.OpenImageFile(payload)

	case commandTranslateLayer:
		var payload CoreTranslateLayerPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.TranslateLayer(payload)
	}

	return false, nil
}
