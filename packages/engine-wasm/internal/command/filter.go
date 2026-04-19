package command

import "encoding/json"

const (
	commandApplyFilter         int32 = 0x0500
	commandReapplyFilter       int32 = 0x0501
	commandPreviewFilter       int32 = 0x0502
	commandCancelFilterPreview int32 = 0x0503
	commandCommitFilterPreview int32 = 0x0504
	commandFadeFilter          int32 = 0x0505
)

type FilterApplyPayload struct {
	LayerID  string          `json:"layerId"`
	FilterID string          `json:"filterId"`
	Params   json.RawMessage `json:"params"`
}

type FilterPreviewPayload struct {
	LayerID  string          `json:"layerId"`
	FilterID string          `json:"filterId"`
	Params   json.RawMessage `json:"params"`
	Scale    int             `json:"scale"`
}

type FilterFadePayload struct {
	Opacity   float64 `json:"opacity"`
	BlendMode string  `json:"blendMode"`
}

type FilterDeps struct {
	Decode              func(string, any) error
	ApplyFilter         func(FilterApplyPayload) error
	ReapplyFilter       func() error
	PreviewFilter       func(FilterPreviewPayload) error
	CancelFilterPreview func() error
	CommitFilterPreview func() error
	FadeFilter          func(FilterFadePayload) error
}

func DispatchFilter(commandID int32, payloadJSON string, deps FilterDeps) (bool, error) {
	switch commandID {
	case commandApplyFilter:
		var payload FilterApplyPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ApplyFilter(payload)
	case commandReapplyFilter:
		return true, deps.ReapplyFilter()
	case commandPreviewFilter:
		var payload FilterPreviewPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PreviewFilter(payload)
	case commandCancelFilterPreview:
		return true, deps.CancelFilterPreview()
	case commandCommitFilterPreview:
		return true, deps.CommitFilterPreview()
	case commandFadeFilter:
		var payload FilterFadePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.FadeFilter(payload)
	default:
		return false, nil
	}
}
