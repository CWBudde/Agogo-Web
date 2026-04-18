package engine

import (
	"encoding/json"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

type (
	LayerType         = model.LayerType
	LayerLockMode     = model.LayerLockMode
	BlendMode         = model.BlendMode
	LayerNode         = model.LayerNode
	LayerMask         = model.LayerMask
	HandleType        = model.HandleType
	PathPoint         = model.PathPoint
	Subpath           = model.Subpath
	Path              = model.Path
	LayerStyle        = model.LayerStyle
	LayerBounds       = model.LayerBounds
	ArtboardData      = model.ArtboardData
	PixelLayer        = model.PixelLayer
	AdjustmentLayer   = model.AdjustmentLayer
	AdjustmentCache   = model.AdjustmentCache
	TextLayer         = model.TextLayer
	VectorLayer       = model.VectorLayer
	GroupLayer        = model.GroupLayer
	BlendIfChannel    = model.BlendIfChannel
	BlendIfRange      = model.BlendIfRange
	BlendChannelsMask = model.BlendChannelsMask
	BlendIfConfig     = model.BlendIfConfig
)

const (
	LayerTypePixel      = model.LayerTypePixel
	LayerTypeGroup      = model.LayerTypeGroup
	LayerTypeAdjustment = model.LayerTypeAdjustment
	LayerTypeText       = model.LayerTypeText
	LayerTypeVector     = model.LayerTypeVector
)

const (
	LayerLockNone     = model.LayerLockNone
	LayerLockPixels   = model.LayerLockPixels
	LayerLockPosition = model.LayerLockPosition
	LayerLockAll      = model.LayerLockAll
)

const (
	BlendModeNormal       = model.BlendModeNormal
	BlendModeDissolve     = model.BlendModeDissolve
	BlendModeMultiply     = model.BlendModeMultiply
	BlendModeColorBurn    = model.BlendModeColorBurn
	BlendModeLinearBurn   = model.BlendModeLinearBurn
	BlendModeDarken       = model.BlendModeDarken
	BlendModeDarkerColor  = model.BlendModeDarkerColor
	BlendModeScreen       = model.BlendModeScreen
	BlendModeColorDodge   = model.BlendModeColorDodge
	BlendModeLinearDodge  = model.BlendModeLinearDodge
	BlendModeLighten      = model.BlendModeLighten
	BlendModeLighterColor = model.BlendModeLighterColor
	BlendModeOverlay      = model.BlendModeOverlay
	BlendModeSoftLight    = model.BlendModeSoftLight
	BlendModeHardLight    = model.BlendModeHardLight
	BlendModeVividLight   = model.BlendModeVividLight
	BlendModeLinearLight  = model.BlendModeLinearLight
	BlendModePinLight     = model.BlendModePinLight
	BlendModeHardMix      = model.BlendModeHardMix
	BlendModeDifference   = model.BlendModeDifference
	BlendModeExclusion    = model.BlendModeExclusion
	BlendModeSubtract     = model.BlendModeSubtract
	BlendModeDivide       = model.BlendModeDivide
	BlendModeHue          = model.BlendModeHue
	BlendModeSaturation   = model.BlendModeSaturation
	BlendModeColor        = model.BlendModeColor
	BlendModeLuminosity   = model.BlendModeLuminosity
)

const (
	HandleCorner    = model.HandleCorner
	HandleSmooth    = model.HandleSmooth
	HandleSymmetric = model.HandleSymmetric
)

func NewPixelLayer(name string, bounds LayerBounds, pixels []byte) *PixelLayer {
	return model.NewPixelLayer(name, bounds, pixels)
}

func NewAdjustmentLayer(name, adjustmentKind string, params json.RawMessage) *AdjustmentLayer {
	return model.NewAdjustmentLayer(name, adjustmentKind, params)
}

func NewTextLayer(name string, bounds LayerBounds, text string, cachedRaster []byte) *TextLayer {
	return model.NewTextLayer(name, bounds, text, cachedRaster)
}

func NewVectorLayer(name string, bounds LayerBounds, shape *Path, cachedRaster []byte) *VectorLayer {
	return model.NewVectorLayer(name, bounds, shape, cachedRaster)
}

func NewGroupLayer(name string) *GroupLayer {
	return model.NewGroupLayer(name)
}

func cloneLayerMask(mask *LayerMask) *LayerMask {
	return model.CloneLayerMask(mask)
}

func cloneArtboard(artboard *ArtboardData) *ArtboardData {
	return model.CloneArtboard(artboard)
}

func clonePath(path *Path) *Path {
	return model.ClonePath(path)
}

func cloneLayerStyles(styles []LayerStyle) []LayerStyle {
	return model.CloneLayerStyles(styles)
}

func cloneLayerStyle(style LayerStyle) LayerStyle {
	return model.CloneLayerStyle(style)
}

func cloneJSONRawMessage(message json.RawMessage) json.RawMessage {
	return model.CloneJSONRawMessage(message)
}

func cloneLayerForDuplicate(layer LayerNode) LayerNode {
	return model.CloneLayerForDuplicate(layer)
}

func cloneGroupLayer(group *GroupLayer) *GroupLayer {
	return model.CloneGroupLayer(group)
}

func layerTreeEqual(a, b LayerNode) bool {
	return model.LayerTreeEqual(a, b)
}

//nolint:unused // kept for package-local tests
func pathEqual(a, b *Path) bool {
	return model.PathEqual(a, b)
}

//nolint:unused // kept for package-local tests
func layerStylesEqual(a, b []LayerStyle) bool {
	return model.LayerStylesEqual(a, b)
}

func isValidBlendMode(mode BlendMode) bool {
	return model.IsValidBlendMode(mode)
}

func newLayerID() string {
	return model.NewLayerID()
}

func setLayerID(layer LayerNode, id string) {
	model.SetLayerID(layer, id)
}

func walkLayerTree(node LayerNode, fn func(LayerNode)) {
	model.WalkLayerTree(node, fn)
}

func clampUnit(value float64) float64 {
	return model.ClampUnit(value)
}
