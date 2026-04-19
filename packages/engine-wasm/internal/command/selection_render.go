package command

const (
	commandMagneticLassoSuggestPath int32 = 0x020e
	commandSampleMergedColor        int32 = 0x0412
)

type SelectionPaintSuggestedPathPayload struct {
	X1           int    `json:"x1"`
	Y1           int    `json:"y1"`
	X2           int    `json:"x2"`
	Y2           int    `json:"y2"`
	LayerID      string `json:"layerId"`
	SampleMerged bool   `json:"sampleMerged"`
}

type SelectionPaintSampleColorPayload struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	SampleSize   int     `json:"sampleSize,omitempty"`
	SampleMerged bool    `json:"sampleMerged,omitempty"`
}

type SelectionPaintRenderResponse struct {
	SuggestedPath []SelectionPoint
	SampledColor  *[4]uint8
}

type SelectionPaintRenderDeps struct {
	Decode                   func(string, any) error
	MagneticLassoSuggestPath func(SelectionPaintSuggestedPathPayload) (*SelectionPaintRenderResponse, error)
	SampleMergedColor        func(SelectionPaintSampleColorPayload) (*SelectionPaintRenderResponse, error)
}

func DispatchSelectionPaintRender(commandID int32, payloadJSON string, deps SelectionPaintRenderDeps) (bool, *SelectionPaintRenderResponse, error) {
	switch commandID {
	case commandMagneticLassoSuggestPath:
		var payload SelectionPaintSuggestedPathPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, nil, err
		}
		response, err := deps.MagneticLassoSuggestPath(payload)
		return true, response, err
	case commandSampleMergedColor:
		var payload SelectionPaintSampleColorPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, nil, err
		}
		response, err := deps.SampleMergedColor(payload)
		return true, response, err
	default:
		return false, nil, nil
	}
}
