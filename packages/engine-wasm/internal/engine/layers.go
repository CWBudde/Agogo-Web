package engine

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
)

type LayerType string

const (
	LayerTypePixel      LayerType = "pixel"
	LayerTypeGroup      LayerType = "group"
	LayerTypeAdjustment LayerType = "adjustment"
	LayerTypeText       LayerType = "text"
	LayerTypeVector     LayerType = "vector"
)

type LayerLockMode string

const (
	LayerLockNone     LayerLockMode = "none"
	LayerLockPixels   LayerLockMode = "pixels"
	LayerLockPosition LayerLockMode = "position"
	LayerLockAll      LayerLockMode = "all"
)

type BlendMode string

const (
	BlendModeNormal       BlendMode = "normal"
	BlendModeDissolve     BlendMode = "dissolve"
	BlendModeMultiply     BlendMode = "multiply"
	BlendModeColorBurn    BlendMode = "color-burn"
	BlendModeLinearBurn   BlendMode = "linear-burn"
	BlendModeDarken       BlendMode = "darken"
	BlendModeDarkerColor  BlendMode = "darker-color"
	BlendModeScreen       BlendMode = "screen"
	BlendModeColorDodge   BlendMode = "color-dodge"
	BlendModeLinearDodge  BlendMode = "linear-dodge"
	BlendModeLighten      BlendMode = "lighten"
	BlendModeLighterColor BlendMode = "lighter-color"
	BlendModeOverlay      BlendMode = "overlay"
	BlendModeSoftLight    BlendMode = "soft-light"
	BlendModeHardLight    BlendMode = "hard-light"
	BlendModeVividLight   BlendMode = "vivid-light"
	BlendModeLinearLight  BlendMode = "linear-light"
	BlendModePinLight     BlendMode = "pin-light"
	BlendModeHardMix      BlendMode = "hard-mix"
	BlendModeDifference   BlendMode = "difference"
	BlendModeExclusion    BlendMode = "exclusion"
	BlendModeSubtract     BlendMode = "subtract"
	BlendModeDivide       BlendMode = "divide"
	BlendModeHue          BlendMode = "hue"
	BlendModeSaturation   BlendMode = "saturation"
	BlendModeColor        BlendMode = "color"
	BlendModeLuminosity   BlendMode = "luminosity"
)

type LayerNode interface {
	ID() string
	LayerType() LayerType
	Name() string
	SetName(string)
	Visible() bool
	SetVisible(bool)
	LockMode() LayerLockMode
	SetLockMode(LayerLockMode)
	Opacity() float64
	SetOpacity(float64)
	FillOpacity() float64
	SetFillOpacity(float64)
	BlendMode() BlendMode
	SetBlendMode(BlendMode)
	Parent() LayerNode
	SetParent(LayerNode)
	Children() []LayerNode
	SetChildren([]LayerNode)
	Mask() *LayerMask
	SetMask(*LayerMask)
	VectorMask() *Path
	SetVectorMask(*Path)
	ClippingBase() bool
	SetClippingBase(bool)
	StyleStack() []LayerStyle
	SetStyleStack([]LayerStyle)
	Clone() LayerNode
}

type LayerMask struct {
	Enabled bool   `json:"enabled"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Data    []byte `json:"data,omitempty"`
}

type Path struct {
	Closed bool        `json:"closed"`
	Points []PathPoint `json:"points,omitempty"`
}

type PathPoint struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	InX      float64 `json:"inX,omitempty"`
	InY      float64 `json:"inY,omitempty"`
	OutX     float64 `json:"outX,omitempty"`
	OutY     float64 `json:"outY,omitempty"`
	HasCurve bool    `json:"hasCurve,omitempty"`
}

type LayerStyle struct {
	Kind    string          `json:"kind"`
	Enabled bool            `json:"enabled"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type LayerBounds struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type layerBase struct {
	id           string
	name         string
	visible      bool
	lockMode     LayerLockMode
	opacity      float64
	fillOpacity  float64
	blendMode    BlendMode
	parent       LayerNode
	mask         *LayerMask
	vectorMask   *Path
	clippingBase bool
	styleStack   []LayerStyle
}

func newLayerBase(name string) layerBase {
	return layerBase{
		id:          newLayerID(),
		name:        defaultLayerName(name),
		visible:     true,
		lockMode:    LayerLockNone,
		opacity:     1,
		fillOpacity: 1,
		blendMode:   BlendModeNormal,
	}
}

func (l *layerBase) ID() string {
	return l.id
}

func (l *layerBase) Name() string {
	return l.name
}

func (l *layerBase) SetName(name string) {
	l.name = defaultLayerName(name)
}

func (l *layerBase) Visible() bool {
	return l.visible
}

func (l *layerBase) SetVisible(visible bool) {
	l.visible = visible
}

func (l *layerBase) LockMode() LayerLockMode {
	return l.lockMode
}

func (l *layerBase) SetLockMode(mode LayerLockMode) {
	switch mode {
	case LayerLockPixels, LayerLockPosition, LayerLockAll:
		l.lockMode = mode
	default:
		l.lockMode = LayerLockNone
	}
}

func (l *layerBase) Opacity() float64 {
	return l.opacity
}

func (l *layerBase) SetOpacity(opacity float64) {
	l.opacity = clampUnit(opacity)
}

func (l *layerBase) FillOpacity() float64 {
	return l.fillOpacity
}

func (l *layerBase) SetFillOpacity(opacity float64) {
	l.fillOpacity = clampUnit(opacity)
}

func (l *layerBase) BlendMode() BlendMode {
	return l.blendMode
}

func (l *layerBase) SetBlendMode(mode BlendMode) {
	if mode == "" {
		l.blendMode = BlendModeNormal
		return
	}
	l.blendMode = mode
}

func (l *layerBase) Parent() LayerNode {
	return l.parent
}

func (l *layerBase) SetParent(parent LayerNode) {
	l.parent = parent
}

func (l *layerBase) Children() []LayerNode {
	return nil
}

func (l *layerBase) SetChildren(_ []LayerNode) {}

func (l *layerBase) Mask() *LayerMask {
	return l.mask
}

func (l *layerBase) SetMask(mask *LayerMask) {
	l.mask = cloneLayerMask(mask)
}

func (l *layerBase) VectorMask() *Path {
	return l.vectorMask
}

func (l *layerBase) SetVectorMask(mask *Path) {
	l.vectorMask = clonePath(mask)
}

func (l *layerBase) ClippingBase() bool {
	return l.clippingBase
}

func (l *layerBase) SetClippingBase(clippingBase bool) {
	l.clippingBase = clippingBase
}

func (l *layerBase) StyleStack() []LayerStyle {
	return cloneLayerStyles(l.styleStack)
}

func (l *layerBase) SetStyleStack(styles []LayerStyle) {
	l.styleStack = cloneLayerStyles(styles)
}

func (l *layerBase) cloneBase() layerBase {
	return layerBase{
		id:           l.id,
		name:         l.name,
		visible:      l.visible,
		lockMode:     l.lockMode,
		opacity:      l.opacity,
		fillOpacity:  l.fillOpacity,
		blendMode:    l.blendMode,
		mask:         cloneLayerMask(l.mask),
		vectorMask:   clonePath(l.vectorMask),
		clippingBase: l.clippingBase,
		styleStack:   cloneLayerStyles(l.styleStack),
	}
}

type PixelLayer struct {
	layerBase
	Bounds LayerBounds `json:"bounds"`
	Pixels []byte      `json:"pixels,omitempty"`
}

func NewPixelLayer(name string, bounds LayerBounds, pixels []byte) *PixelLayer {
	copyPixels := append([]byte(nil), pixels...)
	return &PixelLayer{
		layerBase: newLayerBase(name),
		Bounds:    bounds,
		Pixels:    copyPixels,
	}
}

func (l *PixelLayer) LayerType() LayerType {
	return LayerTypePixel
}

func (l *PixelLayer) Clone() LayerNode {
	copyPixels := append([]byte(nil), l.Pixels...)
	return &PixelLayer{
		layerBase: l.cloneBase(),
		Bounds:    l.Bounds,
		Pixels:    copyPixels,
	}
}

type AdjustmentLayer struct {
	layerBase
	AdjustmentKind string          `json:"adjustmentKind"`
	Params         json.RawMessage `json:"params,omitempty"`
}

func NewAdjustmentLayer(name, adjustmentKind string, params json.RawMessage) *AdjustmentLayer {
	return &AdjustmentLayer{
		layerBase:      newLayerBase(name),
		AdjustmentKind: adjustmentKind,
		Params:         cloneJSONRawMessage(params),
	}
}

func (l *AdjustmentLayer) LayerType() LayerType {
	return LayerTypeAdjustment
}

func (l *AdjustmentLayer) Clone() LayerNode {
	return &AdjustmentLayer{
		layerBase:      l.cloneBase(),
		AdjustmentKind: l.AdjustmentKind,
		Params:         cloneJSONRawMessage(l.Params),
	}
}

type GroupLayer struct {
	layerBase
	children []LayerNode
	Isolated bool `json:"isolated"`
}

func NewGroupLayer(name string) *GroupLayer {
	return &GroupLayer{layerBase: newLayerBase(name)}
}

func (l *GroupLayer) LayerType() LayerType {
	return LayerTypeGroup
}

func (l *GroupLayer) Children() []LayerNode {
	return append([]LayerNode(nil), l.children...)
}

func (l *GroupLayer) SetChildren(children []LayerNode) {
	l.children = make([]LayerNode, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		child.SetParent(l)
		l.children = append(l.children, child)
	}
}

func (l *GroupLayer) Clone() LayerNode {
	clone := &GroupLayer{
		layerBase: l.cloneBase(),
		Isolated:  l.Isolated,
	}
	children := make([]LayerNode, 0, len(l.children))
	for _, child := range l.children {
		if child == nil {
			continue
		}
		children = append(children, child.Clone())
	}
	clone.SetChildren(children)
	return clone
}

func cloneLayerMask(mask *LayerMask) *LayerMask {
	if mask == nil {
		return nil
	}
	copyMask := *mask
	copyMask.Data = append([]byte(nil), mask.Data...)
	return &copyMask
}

func clonePath(path *Path) *Path {
	if path == nil {
		return nil
	}
	copyPath := *path
	copyPath.Points = append([]PathPoint(nil), path.Points...)
	return &copyPath
}

func cloneLayerStyles(styles []LayerStyle) []LayerStyle {
	if len(styles) == 0 {
		return nil
	}
	cloned := make([]LayerStyle, len(styles))
	copy(cloned, styles)
	for i := range cloned {
		cloned[i].Params = cloneJSONRawMessage(styles[i].Params)
	}
	return cloned
}

func cloneJSONRawMessage(message json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), message...)
}

func newLayerID() string {
	raw := [16]byte{}
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("generate layer id: %v", err))
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func defaultLayerName(name string) string {
	if name == "" {
		return "Layer"
	}
	return name
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
