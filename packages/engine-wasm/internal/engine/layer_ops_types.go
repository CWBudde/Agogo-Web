package engine

import "encoding/json"

type AddLayerPayload struct {
	LayerType          LayerType       `json:"layerType"`
	Name               string          `json:"name"`
	ParentLayerID      string          `json:"parentLayerId"`
	Index              *int            `json:"index,omitempty"`
	Bounds             LayerBounds     `json:"bounds"`
	Pixels             []byte          `json:"pixels,omitempty"`
	AdjustmentKind     string          `json:"adjustmentKind,omitempty"`
	Params             json.RawMessage `json:"params,omitempty"`
	Text               string          `json:"text,omitempty"`
	FontFamily         string          `json:"fontFamily,omitempty"`
	FontSize           float64         `json:"fontSize,omitempty"`
	Color              [4]uint8        `json:"color,omitempty"`
	Path               *Path           `json:"path,omitempty"`
	FillColor          [4]uint8        `json:"fillColor,omitempty"`
	StrokeColor        [4]uint8        `json:"strokeColor,omitempty"`
	StrokeWidth        float64         `json:"strokeWidth,omitempty"`
	CachedRaster       []byte          `json:"cachedRaster,omitempty"`
	Isolated           bool            `json:"isolated,omitempty"`
	IsArtboard         bool            `json:"isArtboard,omitempty"`
	ArtboardBounds     *LayerBounds    `json:"artboardBounds,omitempty"`
	ArtboardBackground *[4]uint8       `json:"artboardBackground,omitempty"`
}

type OpenImageFilePayload struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Pixels []byte `json:"pixels"`
}

type DeleteLayerPayload struct {
	LayerID string `json:"layerId"`
}

type DuplicateLayerPayload struct {
	LayerID       string `json:"layerId"`
	ParentLayerID string `json:"parentLayerId"`
	Index         *int   `json:"index,omitempty"`
}

type MoveLayerPayload struct {
	LayerID       string `json:"layerId"`
	ParentLayerID string `json:"parentLayerId"`
	Index         *int   `json:"index,omitempty"`
}

type SetLayerVisibilityPayload struct {
	LayerID string `json:"layerId"`
	Visible bool   `json:"visible"`
}

type SetLayerOpacityPayload struct {
	LayerID     string   `json:"layerId"`
	Opacity     *float64 `json:"opacity,omitempty"`
	FillOpacity *float64 `json:"fillOpacity,omitempty"`
}

type SetLayerBlendModePayload struct {
	LayerID   string    `json:"layerId"`
	BlendMode BlendMode `json:"blendMode"`
}

type SetLayerLockPayload struct {
	LayerID  string        `json:"layerId"`
	LockMode LayerLockMode `json:"lockMode"`
}

type FlattenLayerPayload struct {
	LayerID string `json:"layerId"`
}

type MergeDownPayload struct {
	LayerID string `json:"layerId"`
}

type AddLayerMaskMode string

const (
	AddLayerMaskRevealAll     AddLayerMaskMode = "reveal-all"
	AddLayerMaskHideAll       AddLayerMaskMode = "hide-all"
	AddLayerMaskFromSelection AddLayerMaskMode = "from-selection"
)

type AddLayerMaskPayload struct {
	LayerID string           `json:"layerId"`
	Mode    AddLayerMaskMode `json:"mode"`
}

type DeleteLayerMaskPayload struct {
	LayerID string `json:"layerId"`
}

type ApplyLayerMaskPayload struct {
	LayerID string `json:"layerId"`
}

type InvertLayerMaskPayload struct {
	LayerID string `json:"layerId"`
}

type SetLayerMaskEnabledPayload struct {
	LayerID string `json:"layerId"`
	Enabled bool   `json:"enabled"`
}

type SetLayerClipToBelowPayload struct {
	LayerID     string `json:"layerId"`
	ClipToBelow bool   `json:"clipToBelow"`
}

type SetActiveLayerPayload struct {
	LayerID string `json:"layerId"`
}

type SetLayerNamePayload struct {
	LayerID string `json:"layerId"`
	Name    string `json:"name"`
}

type SetAdjustmentParamsPayload struct {
	LayerID        string          `json:"layerId"`
	AdjustmentKind string          `json:"adjustmentKind,omitempty"`
	Params         json.RawMessage `json:"params,omitempty"`
}

type LayerStyleKind string

const (
	LayerStyleKindDropShadow      LayerStyleKind = "drop-shadow"
	LayerStyleKindInnerShadow     LayerStyleKind = "inner-shadow"
	LayerStyleKindOuterGlow       LayerStyleKind = "outer-glow"
	LayerStyleKindInnerGlow       LayerStyleKind = "inner-glow"
	LayerStyleKindBevelEmboss     LayerStyleKind = "bevel-emboss"
	LayerStyleKindSatin           LayerStyleKind = "satin"
	LayerStyleKindColorOverlay    LayerStyleKind = "color-overlay"
	LayerStyleKindGradientOverlay LayerStyleKind = "gradient-overlay"
	LayerStyleKindPatternOverlay  LayerStyleKind = "pattern-overlay"
	LayerStyleKindStroke          LayerStyleKind = "stroke"
	LayerStyleKindBlendIf         LayerStyleKind = "blend-if"
)

type LayerStylePayload struct {
	Kind    LayerStyleKind  `json:"kind"`
	Enabled bool            `json:"enabled"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type SetLayerStyleStackPayload struct {
	LayerID string              `json:"layerId"`
	Styles  []LayerStylePayload `json:"styles"`
}

type SetLayerStyleEnabledPayload struct {
	LayerID string         `json:"layerId"`
	Kind    LayerStyleKind `json:"kind"`
	Enabled bool           `json:"enabled"`
}

type SetLayerStyleParamsPayload struct {
	LayerID string          `json:"layerId"`
	Kind    LayerStyleKind  `json:"kind"`
	Params  json.RawMessage `json:"params"`
}

type CopyLayerStylePayload struct {
	LayerID string `json:"layerId"`
}

type PasteLayerStylePayload struct {
	LayerID string `json:"layerId"`
}

type ClearLayerStylePayload struct {
	LayerID string `json:"layerId"`
}

type TranslateLayerPayload struct {
	LayerID string `json:"layerId"`
	DX      int    `json:"dx"`
	DY      int    `json:"dy"`
}

type SetArtboardPayload struct {
	LayerID    string      `json:"layerId"`
	Bounds     LayerBounds `json:"bounds"`
	Background *[4]uint8   `json:"background,omitempty"`
}

type PickLayerAtPointPayload struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// BeginFreeTransformPayload starts free transform on a layer (or the active
// layer when LayerID is empty).
// Mode, when non-empty, selects the initial transform sub-mode:
//   - "warp" - initialise a 4x4 control-point mesh from the layer corners.
//   - anything else (or empty) - normal affine free transform.
type BeginFreeTransformPayload struct {
	LayerID string `json:"layerId,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

// UpdateFreeTransformPayload sets the live transform matrix.
// The affine matrix maps layer-local pixel coordinates to document space:
//
//	docX = A*lx + C*ly + TX
//	docY = B*lx + D*ly + TY
type UpdateFreeTransformPayload struct {
	A             float64 `json:"a"`
	B             float64 `json:"b"`
	C             float64 `json:"c"`
	D             float64 `json:"d"`
	TX            float64 `json:"tx"`
	TY            float64 `json:"ty"`
	PivotX        float64 `json:"pivotX"`
	PivotY        float64 `json:"pivotY"`
	Interpolation string  `json:"interpolation"`
	// Corners, when non-nil, switches to homography/distort mode.
	// Order: TL, TR, BR, BL in doc space.
	Corners *[4][2]float64 `json:"corners,omitempty"`
	// WarpGrid, when non-nil, switches to mesh-warp mode.
	// 4x4 grid of control points in doc space; row-major [row][col][x,y].
	WarpGrid *[4][4][2]float64 `json:"warpGrid,omitempty"`
}

// DiscreteTransformPayload carries an optional layer ID for discrete (immediate)
// transforms such as flip and rotate.
type DiscreteTransformPayload struct {
	LayerID string `json:"layerId,omitempty"`
}

type AddVectorMaskPayload struct {
	LayerID string `json:"layerId"`
}

type DeleteVectorMaskPayload struct {
	LayerID string `json:"layerId"`
}

type SetMaskEditModePayload struct {
	LayerID string `json:"layerId"`
	Editing bool   `json:"editing"`
}
