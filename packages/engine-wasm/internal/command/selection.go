package command

const (
	commandPickLayerAtPoint         int32 = 0x011a
	commandNewSelection             int32 = 0x0200
	commandSelectAll                int32 = 0x0201
	commandDeselect                 int32 = 0x0202
	commandReselect                 int32 = 0x0203
	commandInvertSelection          int32 = 0x0204
	commandFeatherSelection         int32 = 0x0205
	commandExpandSelection          int32 = 0x0206
	commandContractSelection        int32 = 0x0207
	commandSmoothSelection          int32 = 0x0208
	commandBorderSelection          int32 = 0x0209
	commandTransformSelection       int32 = 0x020a
	commandSelectColorRange         int32 = 0x020b
	commandQuickSelect              int32 = 0x020c
	commandMagicWand                int32 = 0x020d
	commandSaveSelectionToChannel   int32 = 0x020f
	commandLoadSelectionFromChannel int32 = 0x0210
	commandRefineSelection          int32 = 0x0211
	commandOutputSelection          int32 = 0x0212
	commandCopy                     int32 = 0x0214
	commandCut                      int32 = 0x0215
	commandPaste                    int32 = 0x0216
)

type SelectionRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type SelectionPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type selectionPickLayerPayload struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type selectionCreatePayload struct {
	Shape     string           `json:"shape"`
	Mode      string           `json:"mode"`
	Rect      SelectionRect    `json:"rect"`
	Polygon   []SelectionPoint `json:"polygon,omitempty"`
	AntiAlias bool             `json:"antiAlias,omitempty"`
}

type selectionFeatherPayload struct {
	Radius float64 `json:"radius"`
}

type selectionExpandPayload struct {
	Pixels int `json:"pixels"`
}

type selectionContractPayload struct {
	Pixels int `json:"pixels"`
}

type selectionSmoothPayload struct {
	Radius int `json:"radius"`
}

type selectionBorderPayload struct {
	Width int `json:"width"`
}

type selectionTransformPayload struct {
	A  float64 `json:"a"`
	B  float64 `json:"b"`
	C  float64 `json:"c"`
	D  float64 `json:"d"`
	TX float64 `json:"tx"`
	TY float64 `json:"ty"`
}

type selectionColorRangePayload struct {
	LayerID      string   `json:"layerId"`
	TargetColor  [4]uint8 `json:"targetColor"`
	Fuzziness    float64  `json:"fuzziness"`
	SampleMerged bool     `json:"sampleMerged"`
	Mode         string   `json:"mode"`
}

type selectionQuickSelectPayload struct {
	X               int     `json:"x"`
	Y               int     `json:"y"`
	Tolerance       float64 `json:"tolerance"`
	EdgeSensitivity float64 `json:"edgeSensitivity"`
	LayerID         string  `json:"layerId"`
	SampleMerged    bool    `json:"sampleMerged"`
	Mode            string  `json:"mode"`
}

type selectionMagicWandPayload struct {
	X            int     `json:"x"`
	Y            int     `json:"y"`
	Tolerance    float64 `json:"tolerance"`
	LayerID      string  `json:"layerId"`
	SampleMerged bool    `json:"sampleMerged"`
	Contiguous   bool    `json:"contiguous"`
	AntiAlias    bool    `json:"antiAlias"`
	Mode         string  `json:"mode"`
}

type selectionSaveChannelPayload struct {
	Name string `json:"name"`
}

type selectionLoadChannelPayload struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type selectionRefinePayload struct {
	SmartRadius  float64 `json:"smartRadius,omitempty"`
	Contrast     float64 `json:"contrast,omitempty"`
	LayerID      string  `json:"layerId,omitempty"`
	SampleMerged bool    `json:"sampleMerged,omitempty"`
}

type selectionOutputPayload struct {
	Mode         string `json:"mode"`
	LayerID      string `json:"layerId,omitempty"`
	Name         string `json:"name,omitempty"`
	SampleMerged bool   `json:"sampleMerged,omitempty"`
}

type SelectionDeps struct {
	Decode                   func(string, any) error
	PickLayerAtPoint         func(x, y int) error
	CreateSelection          func(shape, mode string, rect SelectionRect, polygon []SelectionPoint, antiAlias bool) error
	SelectAll                func() error
	Deselect                 func() error
	Reselect                 func() error
	InvertSelection          func() error
	FeatherSelection         func(radius float64) error
	ExpandSelection          func(pixels int) error
	ContractSelection        func(pixels int) error
	SmoothSelection          func(radius int) error
	BorderSelection          func(width int) error
	TransformSelection       func(a, b, c, d, tx, ty float64) error
	SelectColorRange         func(layerID string, targetColor [4]uint8, fuzziness float64, sampleMerged bool, mode string) error
	QuickSelect              func(x, y int, tolerance, edgeSensitivity float64, layerID string, sampleMerged bool, mode string) error
	MagicWand                func(x, y int, tolerance float64, layerID string, sampleMerged, contiguous, antiAlias bool, mode string) error
	SaveSelectionToChannel   func(name string) error
	LoadSelectionFromChannel func(name, mode string) error
	RefineSelection          func(smartRadius, contrast float64, layerID string, sampleMerged bool) error
	OutputSelection          func(mode, layerID, name string, sampleMerged bool) error
	Copy                     func() error
	Cut                      func() error
	Paste                    func() error
}

func DispatchSelection(commandID int32, payloadJSON string, deps SelectionDeps) (bool, error) {
	switch commandID {
	case commandPickLayerAtPoint:
		var payload selectionPickLayerPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PickLayerAtPoint(payload.X, payload.Y)

	case commandNewSelection:
		var payload selectionCreatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.CreateSelection(payload.Shape, payload.Mode, payload.Rect, payload.Polygon, payload.AntiAlias)

	case commandSelectAll:
		return true, deps.SelectAll()

	case commandDeselect:
		return true, deps.Deselect()

	case commandReselect:
		return true, deps.Reselect()

	case commandInvertSelection:
		return true, deps.InvertSelection()

	case commandFeatherSelection:
		var payload selectionFeatherPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.FeatherSelection(payload.Radius)

	case commandExpandSelection:
		var payload selectionExpandPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ExpandSelection(payload.Pixels)

	case commandContractSelection:
		var payload selectionContractPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ContractSelection(payload.Pixels)

	case commandSmoothSelection:
		var payload selectionSmoothPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SmoothSelection(payload.Radius)

	case commandBorderSelection:
		var payload selectionBorderPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.BorderSelection(payload.Width)

	case commandTransformSelection:
		var payload selectionTransformPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.TransformSelection(payload.A, payload.B, payload.C, payload.D, payload.TX, payload.TY)

	case commandSelectColorRange:
		var payload selectionColorRangePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SelectColorRange(payload.LayerID, payload.TargetColor, payload.Fuzziness, payload.SampleMerged, payload.Mode)

	case commandQuickSelect:
		var payload selectionQuickSelectPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.QuickSelect(payload.X, payload.Y, payload.Tolerance, payload.EdgeSensitivity, payload.LayerID, payload.SampleMerged, payload.Mode)

	case commandMagicWand:
		var payload selectionMagicWandPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.MagicWand(payload.X, payload.Y, payload.Tolerance, payload.LayerID, payload.SampleMerged, payload.Contiguous, payload.AntiAlias, payload.Mode)

	case commandSaveSelectionToChannel:
		var payload selectionSaveChannelPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SaveSelectionToChannel(payload.Name)

	case commandLoadSelectionFromChannel:
		var payload selectionLoadChannelPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.LoadSelectionFromChannel(payload.Name, payload.Mode)

	case commandRefineSelection:
		var payload selectionRefinePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.RefineSelection(payload.SmartRadius, payload.Contrast, payload.LayerID, payload.SampleMerged)

	case commandOutputSelection:
		var payload selectionOutputPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.OutputSelection(payload.Mode, payload.LayerID, payload.Name, payload.SampleMerged)

	case commandCopy:
		return true, deps.Copy()

	case commandCut:
		return true, deps.Cut()

	case commandPaste:
		return true, deps.Paste()
	}

	return false, nil
}
