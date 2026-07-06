package command

import (
	"encoding/json"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

const (
	commandAddLayer                  int32 = 0x0100
	commandDeleteLayer               int32 = 0x0101
	commandMoveLayer                 int32 = 0x0102
	commandSetLayerVis               int32 = 0x0103
	commandSetLayerOp                int32 = 0x0104
	commandSetLayerBlend             int32 = 0x0105
	commandDuplicateLayer            int32 = 0x0106
	commandSetLayerLock              int32 = 0x0107
	commandFlattenLayer              int32 = 0x0108
	commandMergeDown                 int32 = 0x0109
	commandMergeVisible              int32 = 0x010a
	commandAddLayerMask              int32 = 0x010b
	commandDeleteLayerMask           int32 = 0x010c
	commandApplyLayerMask            int32 = 0x010d
	commandInvertLayerMask           int32 = 0x010e
	commandSetMaskEnabled            int32 = 0x010f
	commandSetLayerClip              int32 = 0x0110
	commandSetActiveLayer            int32 = 0x0111
	commandSetLayerName              int32 = 0x0112
	commandAddVectorMask             int32 = 0x0113
	commandDeleteVectorMask          int32 = 0x0114
	commandSetAdjustmentParams       int32 = 0x011b
	commandSetPointFromSample        int32 = 0x011d
	commandSetLayerStyleStack        int32 = 0x011f
	commandSetLayerStyleEnabled      int32 = 0x0120
	commandSetLayerStyleParams       int32 = 0x0121
	commandCopyLayerStyle            int32 = 0x0122
	commandPasteLayerStyle           int32 = 0x0123
	commandClearLayerStyle           int32 = 0x0124
	commandCreateDocumentStylePreset int32 = 0x0125
	commandUpdateDocumentStylePreset int32 = 0x0126
	commandDeleteDocumentStylePreset int32 = 0x0127
	commandApplyDocumentStylePreset  int32 = 0x0128
	commandSetArtboard               int32 = 0x0129
	commandSetVectorMaskPath         int32 = 0x012a
)

type LayerAddPayload struct {
	LayerType          model.LayerType    `json:"layerType"`
	Name               string             `json:"name"`
	ParentLayerID      string             `json:"parentLayerId"`
	Index              *int               `json:"index,omitempty"`
	Bounds             model.LayerBounds  `json:"bounds"`
	Pixels             []byte             `json:"pixels,omitempty"`
	AdjustmentKind     string             `json:"adjustmentKind,omitempty"`
	Params             json.RawMessage    `json:"params,omitempty"`
	Text               string             `json:"text,omitempty"`
	FontFamily         string             `json:"fontFamily,omitempty"`
	FontSize           float64            `json:"fontSize,omitempty"`
	Color              [4]uint8           `json:"color,omitempty"`
	Path               *model.Path        `json:"path,omitempty"`
	FillColor          [4]uint8           `json:"fillColor,omitempty"`
	StrokeColor        [4]uint8           `json:"strokeColor,omitempty"`
	StrokeWidth        float64            `json:"strokeWidth,omitempty"`
	CachedRaster       []byte             `json:"cachedRaster,omitempty"`
	Isolated           bool               `json:"isolated,omitempty"`
	IsArtboard         bool               `json:"isArtboard,omitempty"`
	ArtboardBounds     *model.LayerBounds `json:"artboardBounds,omitempty"`
	ArtboardBackground *[4]uint8          `json:"artboardBackground,omitempty"`
}

type layerIDPayload struct {
	LayerID string `json:"layerId"`
}

type LayerDuplicatePayload struct {
	LayerID       string `json:"layerId"`
	ParentLayerID string `json:"parentLayerId"`
	Index         *int   `json:"index,omitempty"`
}

type LayerMovePayload struct {
	LayerID       string `json:"layerId"`
	ParentLayerID string `json:"parentLayerId"`
	Index         *int   `json:"index,omitempty"`
}

type LayerVisibilityPayload struct {
	LayerID string `json:"layerId"`
	Visible bool   `json:"visible"`
}

type LayerOpacityPayload struct {
	LayerID     string   `json:"layerId"`
	Opacity     *float64 `json:"opacity,omitempty"`
	FillOpacity *float64 `json:"fillOpacity,omitempty"`
}

type LayerBlendModePayload struct {
	LayerID   string          `json:"layerId"`
	BlendMode model.BlendMode `json:"blendMode"`
}

type LayerLockPayload struct {
	LayerID  string              `json:"layerId"`
	LockMode model.LayerLockMode `json:"lockMode"`
}

type LayerAddMaskMode string

type LayerAddMaskPayload struct {
	LayerID string           `json:"layerId"`
	Mode    LayerAddMaskMode `json:"mode"`
}

type LayerMaskEnabledPayload struct {
	LayerID string `json:"layerId"`
	Enabled bool   `json:"enabled"`
}

// LayerAddVectorMaskPayload extends the legacy bare {layerId} payload with an
// optional fromActivePath flag; old payloads decode unchanged (flag false).
type LayerAddVectorMaskPayload struct {
	LayerID        string `json:"layerId"`
	FromActivePath bool   `json:"fromActivePath,omitempty"`
}

type LayerVectorMaskPathPayload struct {
	LayerID string      `json:"layerId"`
	Path    *model.Path `json:"path"`
}

type LayerClipPayload struct {
	LayerID     string `json:"layerId"`
	ClipToBelow bool   `json:"clipToBelow"`
}

type LayerNamePayload struct {
	LayerID string `json:"layerId"`
	Name    string `json:"name"`
}

type LayerAdjustmentParamsPayload struct {
	LayerID        string          `json:"layerId"`
	AdjustmentKind string          `json:"adjustmentKind,omitempty"`
	Params         json.RawMessage `json:"params,omitempty"`
}

type LayerStyleKind string

type LayerStylePayload struct {
	Kind    LayerStyleKind  `json:"kind"`
	Enabled bool            `json:"enabled"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type LayerStyleStackPayload struct {
	LayerID string              `json:"layerId"`
	Styles  []LayerStylePayload `json:"styles"`
}

type LayerStyleEnabledPayload struct {
	LayerID string         `json:"layerId"`
	Kind    LayerStyleKind `json:"kind"`
	Enabled bool           `json:"enabled"`
}

type LayerStyleParamsPayload struct {
	LayerID string          `json:"layerId"`
	Kind    LayerStyleKind  `json:"kind"`
	Params  json.RawMessage `json:"params"`
}

type DocumentStylePresetCreatePayload struct {
	Name   string              `json:"name"`
	Styles []LayerStylePayload `json:"styles"`
}

type DocumentStylePresetUpdatePayload struct {
	PresetID string              `json:"presetId"`
	Name     *string             `json:"name,omitempty"`
	Styles   []LayerStylePayload `json:"styles,omitempty"`
}

type documentStylePresetIDPayload struct {
	PresetID string `json:"presetId"`
}

type DocumentStylePresetApplyPayload struct {
	PresetID string `json:"presetId"`
	LayerID  string `json:"layerId"`
}

type LayerArtboardPayload struct {
	LayerID    string            `json:"layerId"`
	Bounds     model.LayerBounds `json:"bounds"`
	Background *[4]uint8         `json:"background,omitempty"`
}

type LayerSetPointFromSamplePayload struct {
	LayerID string  `json:"layerId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Mode    string  `json:"mode"`
}

type LayerDeps struct {
	Decode                    func(string, any) error
	AddLayer                  func(LayerAddPayload) error
	DeleteLayer               func(layerID string) error
	MoveLayer                 func(LayerMovePayload) error
	SetLayerVisibility        func(LayerVisibilityPayload) error
	SetLayerOpacity           func(LayerOpacityPayload) error
	SetLayerBlendMode         func(LayerBlendModePayload) error
	DuplicateLayer            func(LayerDuplicatePayload) error
	SetLayerLock              func(LayerLockPayload) error
	FlattenLayer              func(layerID string) error
	MergeDown                 func(layerID string) error
	MergeVisible              func() error
	AddLayerMask              func(LayerAddMaskPayload) error
	DeleteLayerMask           func(layerID string) error
	ApplyLayerMask            func(layerID string) error
	InvertLayerMask           func(layerID string) error
	SetLayerMaskEnabled       func(LayerMaskEnabledPayload) error
	SetLayerClipToBelow       func(LayerClipPayload) error
	SetActiveLayer            func(layerID string) error
	SetLayerName              func(LayerNamePayload) error
	AddVectorMask             func(LayerAddVectorMaskPayload) error
	DeleteVectorMask          func(layerID string) error
	SetVectorMaskPath         func(LayerVectorMaskPathPayload) error
	SetAdjustmentParams       func(LayerAdjustmentParamsPayload) error
	SetLayerStyleStack        func(LayerStyleStackPayload) error
	SetLayerStyleEnabled      func(LayerStyleEnabledPayload) error
	SetLayerStyleParams       func(LayerStyleParamsPayload) error
	CopyLayerStyle            func(layerID string) error
	PasteLayerStyle           func(layerID string) error
	ClearLayerStyle           func(layerID string) error
	CreateDocumentStylePreset func(DocumentStylePresetCreatePayload) error
	UpdateDocumentStylePreset func(DocumentStylePresetUpdatePayload) error
	DeleteDocumentStylePreset func(presetID string) error
	ApplyDocumentStylePreset  func(DocumentStylePresetApplyPayload) error
	SetArtboard               func(LayerArtboardPayload) error
	SetPointFromSample        func(LayerSetPointFromSamplePayload) error
}

func DispatchLayer(commandID int32, payloadJSON string, deps LayerDeps) (bool, error) {
	switch commandID {
	case commandAddLayer:
		var payload LayerAddPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.AddLayer(payload)
	case commandDeleteLayer:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeleteLayer(payload.LayerID)
	case commandMoveLayer:
		var payload LayerMovePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.MoveLayer(payload)
	case commandSetLayerVis:
		var payload LayerVisibilityPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerVisibility(payload)
	case commandSetLayerOp:
		var payload LayerOpacityPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerOpacity(payload)
	case commandSetLayerBlend:
		var payload LayerBlendModePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerBlendMode(payload)
	case commandDuplicateLayer:
		var payload LayerDuplicatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DuplicateLayer(payload)
	case commandSetLayerLock:
		var payload LayerLockPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerLock(payload)
	case commandFlattenLayer:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.FlattenLayer(payload.LayerID)
	case commandMergeDown:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.MergeDown(payload.LayerID)
	case commandMergeVisible:
		return true, deps.MergeVisible()
	case commandAddLayerMask:
		var payload LayerAddMaskPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.AddLayerMask(payload)
	case commandDeleteLayerMask:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeleteLayerMask(payload.LayerID)
	case commandApplyLayerMask:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ApplyLayerMask(payload.LayerID)
	case commandInvertLayerMask:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.InvertLayerMask(payload.LayerID)
	case commandSetMaskEnabled:
		var payload LayerMaskEnabledPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerMaskEnabled(payload)
	case commandSetLayerClip:
		var payload LayerClipPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerClipToBelow(payload)
	case commandSetActiveLayer:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetActiveLayer(payload.LayerID)
	case commandSetLayerName:
		var payload LayerNamePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerName(payload)
	case commandAddVectorMask:
		var payload LayerAddVectorMaskPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.AddVectorMask(payload)
	case commandSetVectorMaskPath:
		var payload LayerVectorMaskPathPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetVectorMaskPath(payload)
	case commandDeleteVectorMask:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeleteVectorMask(payload.LayerID)
	case commandSetAdjustmentParams:
		var payload LayerAdjustmentParamsPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetAdjustmentParams(payload)
	case commandSetLayerStyleStack:
		var payload LayerStyleStackPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerStyleStack(payload)
	case commandSetLayerStyleEnabled:
		var payload LayerStyleEnabledPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerStyleEnabled(payload)
	case commandSetLayerStyleParams:
		var payload LayerStyleParamsPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetLayerStyleParams(payload)
	case commandCopyLayerStyle:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.CopyLayerStyle(payload.LayerID)
	case commandPasteLayerStyle:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.PasteLayerStyle(payload.LayerID)
	case commandClearLayerStyle:
		var payload layerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ClearLayerStyle(payload.LayerID)
	case commandCreateDocumentStylePreset:
		var payload DocumentStylePresetCreatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.CreateDocumentStylePreset(payload)
	case commandUpdateDocumentStylePreset:
		var payload DocumentStylePresetUpdatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.UpdateDocumentStylePreset(payload)
	case commandDeleteDocumentStylePreset:
		var payload documentStylePresetIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeleteDocumentStylePreset(payload.PresetID)
	case commandApplyDocumentStylePreset:
		var payload DocumentStylePresetApplyPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ApplyDocumentStylePreset(payload)
	case commandSetArtboard:
		var payload LayerArtboardPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetArtboard(payload)
	case commandSetPointFromSample:
		var payload LayerSetPointFromSamplePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetPointFromSample(payload)
	default:
		return false, nil
	}
}
