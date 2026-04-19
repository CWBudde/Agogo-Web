package command

const (
	commandBeginFreeTransform  int32 = 0x0300
	commandUpdateFreeTransform int32 = 0x0301
	commandCommitFreeTransform int32 = 0x0302
	commandCancelFreeTransform int32 = 0x0303

	commandFlipLayerH       int32 = 0x0304
	commandFlipLayerV       int32 = 0x0305
	commandRotateLayer90CW  int32 = 0x0306
	commandRotateLayer90CCW int32 = 0x0307
	commandRotateLayer180   int32 = 0x0308
	commandTransformAgain   int32 = 0x0309

	commandBeginCrop  int32 = 0x0320
	commandUpdateCrop int32 = 0x0321
	commandCommitCrop int32 = 0x0322
	commandCancelCrop int32 = 0x0323

	commandResizeCanvas int32 = 0x0324
)

type TransformBeginFreePayload struct {
	LayerID string `json:"layerId,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

type TransformUpdateFreePayload struct {
	A             float64           `json:"a"`
	B             float64           `json:"b"`
	C             float64           `json:"c"`
	D             float64           `json:"d"`
	TX            float64           `json:"tx"`
	TY            float64           `json:"ty"`
	PivotX        float64           `json:"pivotX"`
	PivotY        float64           `json:"pivotY"`
	Interpolation string            `json:"interpolation"`
	Corners       *[4][2]float64    `json:"corners,omitempty"`
	WarpGrid      *[4][4][2]float64 `json:"warpGrid,omitempty"`
}

type TransformDiscretePayload struct {
	LayerID string `json:"layerId,omitempty"`
}

type TransformUpdateCropPayload struct {
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	W                float64 `json:"w"`
	H                float64 `json:"h"`
	Rotation         float64 `json:"rotation"`
	DeletePixels     bool    `json:"deletePixels"`
	ContentAwareFill bool    `json:"contentAwareFill"`
	Resolution       float64 `json:"resolution"`
	OverlayType      string  `json:"overlayType"`
}

type TransformResizeCanvasPayload struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Anchor string `json:"anchor"`
}

type TransformDeps struct {
	Decode              func(string, any) error
	BeginFreeTransform  func(TransformBeginFreePayload) error
	UpdateFreeTransform func(TransformUpdateFreePayload) error
	CommitFreeTransform func() error
	CancelFreeTransform func() error
	DiscreteTransform   func(kind string, layerID string) error
	TransformAgain      func() error
	BeginCrop           func() error
	UpdateCrop          func(TransformUpdateCropPayload) error
	CommitCrop          func() error
	CancelCrop          func() error
	ResizeCanvas        func(TransformResizeCanvasPayload) error
}

func DispatchTransform(commandID int32, payloadJSON string, deps TransformDeps) (bool, error) {
	switch commandID {
	case commandBeginFreeTransform:
		var payload TransformBeginFreePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.BeginFreeTransform(payload)
	case commandUpdateFreeTransform:
		var payload TransformUpdateFreePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.UpdateFreeTransform(payload)
	case commandCommitFreeTransform:
		return true, deps.CommitFreeTransform()
	case commandCancelFreeTransform:
		return true, deps.CancelFreeTransform()
	case commandFlipLayerH:
		var payload TransformDiscretePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DiscreteTransform("flipH", payload.LayerID)
	case commandFlipLayerV:
		var payload TransformDiscretePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DiscreteTransform("flipV", payload.LayerID)
	case commandRotateLayer90CW:
		var payload TransformDiscretePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DiscreteTransform("rotate90cw", payload.LayerID)
	case commandRotateLayer90CCW:
		var payload TransformDiscretePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DiscreteTransform("rotate90ccw", payload.LayerID)
	case commandRotateLayer180:
		var payload TransformDiscretePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DiscreteTransform("rotate180", payload.LayerID)
	case commandTransformAgain:
		return true, deps.TransformAgain()
	case commandBeginCrop:
		return true, deps.BeginCrop()
	case commandUpdateCrop:
		var payload TransformUpdateCropPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.UpdateCrop(payload)
	case commandCommitCrop:
		return true, deps.CommitCrop()
	case commandCancelCrop:
		return true, deps.CancelCrop()
	case commandResizeCanvas:
		var payload TransformResizeCanvasPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ResizeCanvas(payload)
	default:
		return false, nil
	}
}
