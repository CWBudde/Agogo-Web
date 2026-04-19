package command

import docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"

const (
	commandSetMaskEditMode      int32 = 0x0115
	commandGetLayerThumbnails   int32 = 0x0116
	commandComputeHistogram     int32 = 0x011c
	commandIdentifyHueRange     int32 = 0x011e
	commandSetSelectionViewMode int32 = 0x0213
)

type uiSetMaskEditModePayload struct {
	LayerID string `json:"layerId"`
	Editing bool   `json:"editing"`
}

type uiSetSelectionViewModePayload struct {
	Mode string `json:"mode"`
}

type UIDeps struct {
	Decode                   func(string, any) error
	DefaultSelectionViewMode string
	GenerateThumbnails       func() (map[string]docpkg.ThumbnailEntry, error)
	ComputeHistogram         func(string) (any, error)
	IdentifyHueRange         func(string) (string, error)
}

type UIResult struct {
	Handled            bool
	MaskEditLayerID    *string
	SelectionViewMode  *string
	Thumbnails         map[string]docpkg.ThumbnailEntry
	Histogram          any
	IdentifiedHueRange string
	HasCustomRender    bool
}

func DispatchUI(commandID int32, payloadJSON string, deps UIDeps) (UIResult, error) {
	switch commandID {
	case commandSetMaskEditMode:
		var payload uiSetMaskEditModePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return UIResult{Handled: true}, err
		}
		layerID := ""
		if payload.Editing {
			layerID = payload.LayerID
		}
		return UIResult{Handled: true, MaskEditLayerID: &layerID}, nil

	case commandGetLayerThumbnails:
		thumbs, err := deps.GenerateThumbnails()
		if err != nil {
			return UIResult{Handled: true}, err
		}
		return UIResult{Handled: true, Thumbnails: thumbs, HasCustomRender: true}, nil

	case commandComputeHistogram:
		histogram, err := deps.ComputeHistogram(payloadJSON)
		if err != nil {
			return UIResult{Handled: true}, err
		}
		return UIResult{Handled: true, Histogram: histogram, HasCustomRender: true}, nil

	case commandIdentifyHueRange:
		rangeName, err := deps.IdentifyHueRange(payloadJSON)
		if err != nil {
			return UIResult{Handled: true}, err
		}
		return UIResult{Handled: true, IdentifiedHueRange: rangeName, HasCustomRender: true}, nil

	case commandSetSelectionViewMode:
		var payload uiSetSelectionViewModePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return UIResult{Handled: true}, err
		}
		mode := payload.Mode
		if mode == "" {
			mode = deps.DefaultSelectionViewMode
		}
		return UIResult{Handled: true, SelectionViewMode: &mode}, nil
	}

	return UIResult{}, nil
}
