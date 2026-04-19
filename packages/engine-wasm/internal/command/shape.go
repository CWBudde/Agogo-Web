package command

import "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"

const (
	commandDrawShape           int32 = 0x0630
	commandEnterVectorEditMode int32 = 0x0631
	commandCommitVectorEdit    int32 = 0x0632
	commandSetVectorLayerStyle int32 = 0x0633
)

type ShapeDrawPayload struct {
	ShapeType      string            `json:"shapeType"`
	X              float64           `json:"x"`
	Y              float64           `json:"y"`
	W              float64           `json:"w"`
	H              float64           `json:"h"`
	CornerRadius   float64           `json:"cornerRadius,omitempty"`
	Sides          int               `json:"sides,omitempty"`
	StarMode       bool              `json:"starMode,omitempty"`
	InnerRadiusPct float64           `json:"innerRadiusPct,omitempty"`
	FillColor      [4]uint8          `json:"fillColor,omitempty"`
	StrokeColor    [4]uint8          `json:"strokeColor,omitempty"`
	StrokeWidth    float64           `json:"strokeWidth,omitempty"`
	Mode           string            `json:"mode,omitempty"`
	Closed         bool              `json:"closed,omitempty"`
	Points         []model.PathPoint `json:"points,omitempty"`
	Subpaths       []model.Subpath   `json:"subpaths,omitempty"`
}

type shapeEnterVectorEditModePayload struct {
	LayerID string `json:"layerId"`
}

type ShapeSetVectorLayerStylePayload struct {
	LayerID     string   `json:"layerId"`
	FillColor   [4]uint8 `json:"fillColor"`
	StrokeColor [4]uint8 `json:"strokeColor"`
	StrokeWidth float64  `json:"strokeWidth"`
}

type ShapeDeps struct {
	Decode              func(string, any) error
	DrawShape           func(ShapeDrawPayload) error
	EnterVectorEditMode func(layerID string) error
	CommitVectorEdit    func() error
	SetVectorLayerStyle func(ShapeSetVectorLayerStylePayload) error
}

func DispatchShape(commandID int32, payloadJSON string, deps ShapeDeps) (bool, error) {
	switch commandID {
	case commandDrawShape:
		var payload ShapeDrawPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DrawShape(payload)
	case commandEnterVectorEditMode:
		var payload shapeEnterVectorEditModePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.EnterVectorEditMode(payload.LayerID)
	case commandCommitVectorEdit:
		return true, deps.CommitVectorEdit()
	case commandSetVectorLayerStyle:
		var payload ShapeSetVectorLayerStylePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetVectorLayerStyle(payload)
	default:
		return false, nil
	}
}
