package engine

import (
	"bytes"
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
	ClipToBelow() bool
	SetClipToBelow(bool)
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

type HandleType int

const (
	HandleCorner    HandleType = iota // Handles move independently
	HandleSmooth                      // Handles stay collinear, independent lengths
	HandleSymmetric                   // Handles collinear + equal length
)

type PathPoint struct {
	X          float64    `json:"x"`
	Y          float64    `json:"y"`
	InX        float64    `json:"inX,omitempty"`
	InY        float64    `json:"inY,omitempty"`
	OutX       float64    `json:"outX,omitempty"`
	OutY       float64    `json:"outY,omitempty"`
	HandleType HandleType `json:"handleType,omitempty"`
}

type Subpath struct {
	Closed bool        `json:"closed"`
	Points []PathPoint `json:"points,omitempty"`
}

type Path struct {
	Subpaths []Subpath `json:"subpaths"`
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
	clipToBelow  bool
	clippingBase bool
	styleStack   []LayerStyle
}

type mutableLayerNode interface {
	LayerNode
	setID(string)
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

func (l *layerBase) setID(id string) {
	l.id = id
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

func isValidBlendMode(mode BlendMode) bool {
	switch mode {
	case BlendModeNormal, BlendModeDissolve, BlendModeMultiply, BlendModeColorBurn,
		BlendModeLinearBurn, BlendModeDarken, BlendModeDarkerColor, BlendModeScreen,
		BlendModeColorDodge, BlendModeLinearDodge, BlendModeLighten, BlendModeLighterColor,
		BlendModeOverlay, BlendModeSoftLight, BlendModeHardLight, BlendModeVividLight,
		BlendModeLinearLight, BlendModePinLight, BlendModeHardMix, BlendModeDifference,
		BlendModeExclusion, BlendModeSubtract, BlendModeDivide, BlendModeHue,
		BlendModeSaturation, BlendModeColor, BlendModeLuminosity:
		return true
	}
	return false
}

func (l *layerBase) SetBlendMode(mode BlendMode) {
	if !isValidBlendMode(mode) {
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

func (l *layerBase) ClipToBelow() bool {
	return l.clipToBelow
}

func (l *layerBase) SetClipToBelow(clipToBelow bool) {
	l.clipToBelow = clipToBelow
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
		clipToBelow:  l.clipToBelow,
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

type TextLayer struct {
	layerBase
	Bounds     LayerBounds `json:"bounds"`
	Text       string      `json:"text"`
	FontFamily string      `json:"fontFamily"`
	FontSize   float64     `json:"fontSize"`
	Color      [4]uint8    `json:"color"`
	// TextType is "point" (no wrap) or "area" (wraps within Bounds).
	TextType      string  `json:"textType,omitempty"`
	Alignment     string  `json:"alignment,omitempty"`
	Leading       float64 `json:"leading,omitempty"`
	Tracking      float64 `json:"tracking,omitempty"` // extra letter spacing in pixels
	Underline     bool    `json:"underline,omitempty"`
	Strikethrough bool    `json:"strikethrough,omitempty"`
	AllCaps       bool    `json:"allCaps,omitempty"`
	SmallCaps     bool    `json:"smallCaps,omitempty"`
	IndentLeft    float64 `json:"indentLeft,omitempty"`
	IndentRight   float64 `json:"indentRight,omitempty"`
	IndentFirst   float64 `json:"indentFirst,omitempty"`
	SpaceBefore   float64 `json:"spaceBefore,omitempty"`
	SpaceAfter    float64 `json:"spaceAfter,omitempty"`
	CachedRaster  []byte  `json:"cachedRaster,omitempty"`
}

func NewTextLayer(name string, bounds LayerBounds, text string, cachedRaster []byte) *TextLayer {
	return &TextLayer{
		layerBase:    newLayerBase(name),
		Bounds:       bounds,
		Text:         text,
		FontFamily:   "system-ui",
		FontSize:     16,
		Color:        [4]uint8{0, 0, 0, 255},
		TextType:     "point",
		Alignment:    "left",
		CachedRaster: append([]byte(nil), cachedRaster...),
	}
}

func (l *TextLayer) LayerType() LayerType {
	return LayerTypeText
}

func (l *TextLayer) Clone() LayerNode {
	return &TextLayer{
		layerBase:     l.cloneBase(),
		Bounds:        l.Bounds,
		Text:          l.Text,
		FontFamily:    l.FontFamily,
		FontSize:      l.FontSize,
		Color:         l.Color,
		TextType:      l.TextType,
		Alignment:     l.Alignment,
		Leading:       l.Leading,
		Tracking:      l.Tracking,
		Underline:     l.Underline,
		Strikethrough: l.Strikethrough,
		AllCaps:       l.AllCaps,
		SmallCaps:     l.SmallCaps,
		IndentLeft:    l.IndentLeft,
		IndentRight:   l.IndentRight,
		IndentFirst:   l.IndentFirst,
		SpaceBefore:   l.SpaceBefore,
		SpaceAfter:    l.SpaceAfter,
		CachedRaster:  append([]byte(nil), l.CachedRaster...),
	}
}

type VectorLayer struct {
	layerBase
	Bounds       LayerBounds `json:"bounds"`
	Shape        *Path       `json:"shape,omitempty"`
	FillColor    [4]uint8    `json:"fillColor"`
	StrokeColor  [4]uint8    `json:"strokeColor"`
	StrokeWidth  float64     `json:"strokeWidth"`
	CachedRaster []byte      `json:"cachedRaster,omitempty"`
}

func NewVectorLayer(name string, bounds LayerBounds, shape *Path, cachedRaster []byte) *VectorLayer {
	return &VectorLayer{
		layerBase:    newLayerBase(name),
		Bounds:       bounds,
		Shape:        clonePath(shape),
		FillColor:    [4]uint8{0, 0, 0, 255},
		StrokeColor:  [4]uint8{0, 0, 0, 0},
		StrokeWidth:  0,
		CachedRaster: append([]byte(nil), cachedRaster...),
	}
}

func (l *VectorLayer) LayerType() LayerType {
	return LayerTypeVector
}

func (l *VectorLayer) Clone() LayerNode {
	return &VectorLayer{
		layerBase:    l.cloneBase(),
		Bounds:       l.Bounds,
		Shape:        clonePath(l.Shape),
		FillColor:    l.FillColor,
		StrokeColor:  l.StrokeColor,
		StrokeWidth:  l.StrokeWidth,
		CachedRaster: append([]byte(nil), l.CachedRaster...),
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
	cp := &Path{Subpaths: make([]Subpath, len(path.Subpaths))}
	for i, sp := range path.Subpaths {
		cp.Subpaths[i] = Subpath{
			Closed: sp.Closed,
			Points: append([]PathPoint(nil), sp.Points...),
		}
	}
	return cp
}

func cloneLayerStyles(styles []LayerStyle) []LayerStyle {
	if len(styles) == 0 {
		return nil
	}
	cloned := make([]LayerStyle, len(styles))
	for i := range styles {
		cloned[i] = cloneLayerStyle(styles[i])
	}
	return cloned
}

func cloneLayerStyle(style LayerStyle) LayerStyle {
	return LayerStyle{
		Kind:    style.Kind,
		Enabled: style.Enabled,
		Params:  cloneJSONRawMessage(style.Params),
	}
}

func cloneJSONRawMessage(message json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), message...)
}

func cloneLayerForDuplicate(layer LayerNode) LayerNode {
	if layer == nil {
		return nil
	}
	clone := layer.Clone()
	reassignLayerIDs(clone)
	clone.SetParent(nil)
	return clone
}

func reassignLayerIDs(layer LayerNode) {
	if layer == nil {
		return
	}
	if mutable, ok := layer.(mutableLayerNode); ok {
		mutable.setID(newLayerID())
	}
	for _, child := range layer.Children() {
		reassignLayerIDs(child)
	}
}

func cloneGroupLayer(group *GroupLayer) *GroupLayer {
	if group == nil {
		return nil
	}
	clone, ok := group.Clone().(*GroupLayer)
	if !ok {
		return nil
	}
	clone.SetParent(nil)
	return clone
}

func layerTreeEqual(a, b LayerNode) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.ID() != b.ID() || a.LayerType() != b.LayerType() || a.Name() != b.Name() || a.Visible() != b.Visible() {
		return false
	}
	if a.LockMode() != b.LockMode() || a.Opacity() != b.Opacity() || a.FillOpacity() != b.FillOpacity() {
		return false
	}
	if a.BlendMode() != b.BlendMode() || a.ClipToBelow() != b.ClipToBelow() || a.ClippingBase() != b.ClippingBase() {
		return false
	}
	if !layerMaskEqual(a.Mask(), b.Mask()) || !pathEqual(a.VectorMask(), b.VectorMask()) {
		return false
	}
	if !layerStylesEqual(a.StyleStack(), b.StyleStack()) {
		return false
	}

	switch left := a.(type) {
	case *PixelLayer:
		right, ok := b.(*PixelLayer)
		if !ok || left.Bounds != right.Bounds || !bytes.Equal(left.Pixels, right.Pixels) {
			return false
		}
	case *AdjustmentLayer:
		right, ok := b.(*AdjustmentLayer)
		if !ok || left.AdjustmentKind != right.AdjustmentKind || !bytes.Equal(left.Params, right.Params) {
			return false
		}
	case *TextLayer:
		right, ok := b.(*TextLayer)
		if !ok || left.Bounds != right.Bounds || left.Text != right.Text || left.FontFamily != right.FontFamily {
			return false
		}
		if left.FontSize != right.FontSize || left.Color != right.Color || !bytes.Equal(left.CachedRaster, right.CachedRaster) {
			return false
		}
		if left.TextType != right.TextType || left.Alignment != right.Alignment || left.Leading != right.Leading {
			return false
		}
		if left.Tracking != right.Tracking || left.Underline != right.Underline || left.Strikethrough != right.Strikethrough {
			return false
		}
		if left.AllCaps != right.AllCaps || left.SmallCaps != right.SmallCaps {
			return false
		}
		if left.IndentLeft != right.IndentLeft || left.IndentRight != right.IndentRight || left.IndentFirst != right.IndentFirst {
			return false
		}
		if left.SpaceBefore != right.SpaceBefore || left.SpaceAfter != right.SpaceAfter {
			return false
		}
	case *VectorLayer:
		right, ok := b.(*VectorLayer)
		if !ok || left.Bounds != right.Bounds || !pathEqual(left.Shape, right.Shape) {
			return false
		}
		if left.FillColor != right.FillColor || left.StrokeColor != right.StrokeColor || left.StrokeWidth != right.StrokeWidth {
			return false
		}
		if !bytes.Equal(left.CachedRaster, right.CachedRaster) {
			return false
		}
	case *GroupLayer:
		right, ok := b.(*GroupLayer)
		if !ok || left.Isolated != right.Isolated {
			return false
		}
	default:
		return false
	}

	leftChildren := a.Children()
	rightChildren := b.Children()
	if len(leftChildren) != len(rightChildren) {
		return false
	}
	for index := range leftChildren {
		if !layerTreeEqual(leftChildren[index], rightChildren[index]) {
			return false
		}
	}
	return true
}

func layerMaskEqual(a, b *LayerMask) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Enabled == b.Enabled && a.Width == b.Width && a.Height == b.Height && bytes.Equal(a.Data, b.Data)
}

func pathEqual(a, b *Path) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if len(a.Subpaths) != len(b.Subpaths) {
		return false
	}
	for i := range a.Subpaths {
		sa, sb := a.Subpaths[i], b.Subpaths[i]
		if sa.Closed != sb.Closed || len(sa.Points) != len(sb.Points) {
			return false
		}
		for j := range sa.Points {
			if sa.Points[j] != sb.Points[j] {
				return false
			}
		}
	}
	return true
}

func layerStylesEqual(a, b []LayerStyle) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index].Kind != b[index].Kind || a[index].Enabled != b[index].Enabled {
			return false
		}
		if !bytes.Equal(a[index].Params, b[index].Params) {
			return false
		}
	}
	return true
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

func walkLayerTree(node LayerNode, fn func(LayerNode)) {
	if node == nil {
		return
	}
	fn(node)
	for _, child := range node.Children() {
		walkLayerTree(child, fn)
	}
}
