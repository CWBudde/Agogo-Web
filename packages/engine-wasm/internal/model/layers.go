package model

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
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
	VectorMaskRaster() *VectorMaskRasterCache
	SetVectorMaskRaster(*VectorMaskRasterCache)
	ClipToBelow() bool
	SetClipToBelow(bool)
	ClippingBase() bool
	SetClippingBase(bool)
	StyleStack() []LayerStyle
	SetStyleStack([]LayerStyle)
	BlendIf() *BlendIfConfig
	SetBlendIf(*BlendIfConfig)
	Clone() LayerNode
}

type LayerMask struct {
	Enabled bool   `json:"enabled"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Data    []byte `json:"data,omitempty"`
}

// VectorMaskRasterCache memoizes the document-sized 8-bit coverage raster of a
// layer's vector mask (mirroring AdjustmentLayer.Cache). Validation is by
// CONTENT, not by invalidation hooks: transforms and crop mutate the mask path
// in place through the pointer returned by VectorMask(), so the cache is valid
// only while its dimensions match the document and PathEqual(Path, VectorMask())
// holds. The struct is immutable by convention — it is replaced wholesale on
// refresh and never mutated, so layer clones may safely share the pointer.
type VectorMaskRasterCache struct {
	W, H int
	Path *Path  `json:"-"` // deep clone taken at rasterization time (content key)
	Data []byte `json:"-"` // W*H coverage bytes, 255 = inside; treat as immutable
}

type HandleType int

const (
	HandleCorner HandleType = iota
	HandleSmooth
	HandleSymmetric
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

type ArtboardData struct {
	Bounds     LayerBounds `json:"bounds"`
	Background [4]uint8    `json:"background"`
}

type layerBase struct {
	id          string
	name        string
	visible     bool
	lockMode    LayerLockMode
	opacity     float64
	fillOpacity float64
	blendMode   BlendMode
	parent      LayerNode
	mask        *LayerMask
	vectorMask  *Path
	// vectorMaskRaster is a render-time memoization of the rasterized vector
	// mask. Not serialized; validated by content (see VectorMaskRasterCache).
	vectorMaskRaster *VectorMaskRasterCache
	clipToBelow      bool
	clippingBase     bool
	styleStack       []LayerStyle
	blendIf          *BlendIfConfig
}

type mutableLayerNode interface {
	LayerNode
	setID(string)
}

func newLayerBase(name string) layerBase {
	return layerBase{
		id:          NewLayerID(),
		name:        defaultLayerName(name),
		visible:     true,
		lockMode:    LayerLockNone,
		opacity:     1,
		fillOpacity: 1,
		blendMode:   BlendModeNormal,
		blendIf:     DefaultBlendIfConfig(),
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

func IsValidBlendMode(mode BlendMode) bool {
	return isValidBlendMode(mode)
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
	l.mask = CloneLayerMask(mask)
}

func (l *layerBase) VectorMask() *Path {
	return l.vectorMask
}

func (l *layerBase) SetVectorMask(mask *Path) {
	l.vectorMask = ClonePath(mask)
}

func (l *layerBase) VectorMaskRaster() *VectorMaskRasterCache {
	return l.vectorMaskRaster
}

func (l *layerBase) SetVectorMaskRaster(cache *VectorMaskRasterCache) {
	l.vectorMaskRaster = cache
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
	return CloneLayerStyles(l.styleStack)
}

func (l *layerBase) SetStyleStack(styles []LayerStyle) {
	l.styleStack = CloneLayerStyles(styles)
}

func (l *layerBase) BlendIf() *BlendIfConfig {
	return CloneBlendIfConfig(l.blendIf)
}

func (l *layerBase) SetBlendIf(config *BlendIfConfig) {
	l.blendIf = NormalizeBlendIfConfig(config)
}

func (l *layerBase) cloneBase() layerBase {
	return layerBase{
		id:          l.id,
		name:        l.name,
		visible:     l.visible,
		lockMode:    l.lockMode,
		opacity:     l.opacity,
		fillOpacity: l.fillOpacity,
		blendMode:   l.blendMode,
		mask:        CloneLayerMask(l.mask),
		vectorMask:  ClonePath(l.vectorMask),
		// Pointer copy on purpose: the cache struct is immutable by convention
		// (pointer-replaced on refresh, never mutated in place) and validated
		// by content, so sharing it across clones is safe and avoids
		// re-rasterizing after every snapshot.
		vectorMaskRaster: l.vectorMaskRaster,
		clipToBelow:      l.clipToBelow,
		clippingBase:     l.clippingBase,
		styleStack:       CloneLayerStyles(l.styleStack),
		blendIf:          CloneBlendIfConfig(l.blendIf),
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
	Cache          AdjustmentCache `json:"-"`
}

type AdjustmentCache struct {
	Kind           string
	ResolvedParams json.RawMessage
	DocW           int
	DocH           int
	Output         []byte
}

func NewAdjustmentLayer(name, adjustmentKind string, params json.RawMessage) *AdjustmentLayer {
	return &AdjustmentLayer{
		layerBase:      newLayerBase(name),
		AdjustmentKind: adjustmentKind,
		Params:         CloneJSONRawMessage(params),
	}
}

func (l *AdjustmentLayer) LayerType() LayerType {
	return LayerTypeAdjustment
}

func (l *AdjustmentLayer) Clone() LayerNode {
	return &AdjustmentLayer{
		layerBase:      l.cloneBase(),
		AdjustmentKind: l.AdjustmentKind,
		Params:         CloneJSONRawMessage(l.Params),
	}
}

type TextLayer struct {
	layerBase
	Bounds        LayerBounds `json:"bounds"`
	Text          string      `json:"text"`
	FontFamily    string      `json:"fontFamily"`
	FontStyle     string      `json:"fontStyle,omitempty"`
	FontSize      float64     `json:"fontSize"`
	Bold          bool        `json:"bold,omitempty"`
	Italic        bool        `json:"italic,omitempty"`
	AntiAlias     string      `json:"antiAlias,omitempty"`
	Color         [4]uint8    `json:"color"`
	TextType      string      `json:"textType,omitempty"`
	Alignment     string      `json:"alignment,omitempty"`
	BaselineShift float64     `json:"baselineShift,omitempty"`
	Leading       float64     `json:"leading,omitempty"`
	Tracking      float64     `json:"tracking,omitempty"`
	Kerning       float64     `json:"kerning,omitempty"`
	Language      string      `json:"language,omitempty"`
	Orientation   string      `json:"orientation,omitempty"`
	Superscript   bool        `json:"superscript,omitempty"`
	Subscript     bool        `json:"subscript,omitempty"`
	Underline     bool        `json:"underline,omitempty"`
	Strikethrough bool        `json:"strikethrough,omitempty"`
	AllCaps       bool        `json:"allCaps,omitempty"`
	SmallCaps     bool        `json:"smallCaps,omitempty"`
	IndentLeft    float64     `json:"indentLeft,omitempty"`
	IndentRight   float64     `json:"indentRight,omitempty"`
	IndentFirst   float64     `json:"indentFirst,omitempty"`
	SpaceBefore   float64     `json:"spaceBefore,omitempty"`
	SpaceAfter    float64     `json:"spaceAfter,omitempty"`
	// CachedRaster is the rasterized text content and is BOUNDS-LOCAL:
	// an RGBA buffer of exactly Bounds.W × Bounds.H × 4 bytes whose pixel
	// (0,0) corresponds to document pixel (Bounds.X, Bounds.Y). The layer
	// position is never baked into the raster — the compositor applies the
	// bounds offset, so translating the layer only updates Bounds.X/Y and
	// does not require re-rasterization. Any other length is a contract
	// violation and fails compositing with a raster-length error.
	CachedRaster []byte `json:"cachedRaster,omitempty"`
}

func NewTextLayer(name string, bounds LayerBounds, text string, cachedRaster []byte) *TextLayer {
	return &TextLayer{
		layerBase:    newLayerBase(name),
		Bounds:       bounds,
		Text:         text,
		FontFamily:   "system-ui",
		FontStyle:    "regular",
		FontSize:     16,
		AntiAlias:    "sharp",
		Color:        [4]uint8{0, 0, 0, 255},
		TextType:     "point",
		Alignment:    "left",
		Orientation:  "horizontal",
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
		FontStyle:     l.FontStyle,
		FontSize:      l.FontSize,
		Bold:          l.Bold,
		Italic:        l.Italic,
		AntiAlias:     l.AntiAlias,
		Color:         l.Color,
		TextType:      l.TextType,
		Alignment:     l.Alignment,
		BaselineShift: l.BaselineShift,
		Leading:       l.Leading,
		Tracking:      l.Tracking,
		Kerning:       l.Kerning,
		Language:      l.Language,
		Orientation:   l.Orientation,
		Superscript:   l.Superscript,
		Subscript:     l.Subscript,
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
	Bounds      LayerBounds `json:"bounds"`
	Shape       *Path       `json:"shape,omitempty"`
	FillColor   [4]uint8    `json:"fillColor"`
	StrokeColor [4]uint8    `json:"strokeColor"`
	StrokeWidth float64     `json:"strokeWidth"`
	// CachedRaster is the rasterized shape content and is BOUNDS-LOCAL:
	// an RGBA buffer of exactly Bounds.W × Bounds.H × 4 bytes whose pixel
	// (0,0) corresponds to document pixel (Bounds.X, Bounds.Y). Shape layers
	// are created with Bounds at the document origin (Shape coordinates are
	// document coordinates), and translation afterwards only moves Bounds.X/Y.
	// Any other length is a contract violation and fails compositing with a
	// raster-length error.
	CachedRaster []byte `json:"cachedRaster,omitempty"`
}

func NewVectorLayer(name string, bounds LayerBounds, shape *Path, cachedRaster []byte) *VectorLayer {
	return &VectorLayer{
		layerBase:    newLayerBase(name),
		Bounds:       bounds,
		Shape:        ClonePath(shape),
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
		Shape:        ClonePath(l.Shape),
		FillColor:    l.FillColor,
		StrokeColor:  l.StrokeColor,
		StrokeWidth:  l.StrokeWidth,
		CachedRaster: append([]byte(nil), l.CachedRaster...),
	}
}

type GroupLayer struct {
	layerBase
	children []LayerNode
	Isolated bool          `json:"isolated"`
	Artboard *ArtboardData `json:"artboard,omitempty"`
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
		Artboard:  CloneArtboard(l.Artboard),
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

func CloneLayerMask(mask *LayerMask) *LayerMask {
	if mask == nil {
		return nil
	}
	copyMask := *mask
	copyMask.Data = append([]byte(nil), mask.Data...)
	return &copyMask
}

func CloneArtboard(artboard *ArtboardData) *ArtboardData {
	if artboard == nil {
		return nil
	}
	copyArtboard := *artboard
	return &copyArtboard
}

func ClonePath(path *Path) *Path {
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

func CloneLayerStyles(styles []LayerStyle) []LayerStyle {
	if len(styles) == 0 {
		return nil
	}
	cloned := make([]LayerStyle, len(styles))
	for i := range styles {
		cloned[i] = CloneLayerStyle(styles[i])
	}
	return cloned
}

func CloneLayerStyle(style LayerStyle) LayerStyle {
	return LayerStyle{
		Kind:    style.Kind,
		Enabled: style.Enabled,
		Params:  CloneJSONRawMessage(style.Params),
	}
}

func CloneJSONRawMessage(message json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), message...)
}

func CloneLayerForDuplicate(layer LayerNode) LayerNode {
	if layer == nil {
		return nil
	}
	clone := layer.Clone()
	reassignLayerIDs(clone)
	clone.SetParent(nil)
	return clone
}

func SetLayerID(layer LayerNode, id string) {
	if mutable, ok := layer.(mutableLayerNode); ok {
		mutable.setID(id)
	}
}

func reassignLayerIDs(layer LayerNode) {
	if layer == nil {
		return
	}
	if mutable, ok := layer.(mutableLayerNode); ok {
		mutable.setID(NewLayerID())
	}
	for _, child := range layer.Children() {
		reassignLayerIDs(child)
	}
}

func CloneGroupLayer(group *GroupLayer) *GroupLayer {
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

func LayerTreeEqual(a, b LayerNode) bool {
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
	if !LayerMaskEqual(a.Mask(), b.Mask()) || !PathEqual(a.VectorMask(), b.VectorMask()) || !BlendIfEqual(a.BlendIf(), b.BlendIf()) {
		return false
	}
	if !LayerStylesEqual(a.StyleStack(), b.StyleStack()) {
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
		if left.FontStyle != right.FontStyle || left.FontSize != right.FontSize || left.Bold != right.Bold || left.Italic != right.Italic {
			return false
		}
		if left.AntiAlias != right.AntiAlias || left.Color != right.Color || !bytes.Equal(left.CachedRaster, right.CachedRaster) {
			return false
		}
		if left.TextType != right.TextType || left.Alignment != right.Alignment || left.BaselineShift != right.BaselineShift || left.Leading != right.Leading {
			return false
		}
		if left.Tracking != right.Tracking || left.Kerning != right.Kerning || left.Language != right.Language || left.Orientation != right.Orientation {
			return false
		}
		if left.Superscript != right.Superscript || left.Subscript != right.Subscript || left.Underline != right.Underline || left.Strikethrough != right.Strikethrough {
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
		if !ok || left.Bounds != right.Bounds || !PathEqual(left.Shape, right.Shape) {
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
		switch {
		case left.Artboard == nil && right.Artboard == nil:
		case left.Artboard == nil || right.Artboard == nil:
			return false
		case left.Artboard.Bounds != right.Artboard.Bounds || left.Artboard.Background != right.Artboard.Background:
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
		if !LayerTreeEqual(leftChildren[index], rightChildren[index]) {
			return false
		}
	}
	return true
}

func LayerMaskEqual(a, b *LayerMask) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Enabled == b.Enabled && a.Width == b.Width && a.Height == b.Height && bytes.Equal(a.Data, b.Data)
}

func PathEqual(a, b *Path) bool {
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

func LayerStylesEqual(a, b []LayerStyle) bool {
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

func BlendIfEqual(a, b *BlendIfConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// layerIDRandRead is the entropy source for NewLayerID; a package variable so
// tests can simulate entropy failure.
var layerIDRandRead = rand.Read

// layerIDFallbackCounter makes fallback IDs unique within the process when the
// entropy source fails.
var layerIDFallbackCounter uint64

func NewLayerID() string {
	raw := [16]byte{}
	if _, err := layerIDRandRead(raw[:]); err != nil {
		// Library code must not panic. Degrade to a process-unique ID derived
		// from the current time and a monotonic counter; callers keep working
		// with a valid (if less random) UUID-shaped ID.
		binary.BigEndian.PutUint64(raw[0:8], uint64(time.Now().UnixNano()))
		binary.BigEndian.PutUint64(raw[8:16], atomic.AddUint64(&layerIDFallbackCounter, 1))
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func WalkLayerTree(node LayerNode, fn func(LayerNode)) {
	if node == nil {
		return
	}
	fn(node)
	for _, child := range node.Children() {
		WalkLayerTree(child, fn)
	}
}

func ClampUnit(value float64) float64 {
	return clampUnit(value)
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
