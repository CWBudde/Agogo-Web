package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

type AddLayerPayload struct {
	LayerType      LayerType       `json:"layerType"`
	Name           string          `json:"name"`
	ParentLayerID  string          `json:"parentLayerId"`
	Index          *int            `json:"index,omitempty"`
	Bounds         LayerBounds     `json:"bounds"`
	Pixels         []byte          `json:"pixels,omitempty"`
	AdjustmentKind string          `json:"adjustmentKind,omitempty"`
	Params         json.RawMessage `json:"params,omitempty"`
	Text           string          `json:"text,omitempty"`
	FontFamily     string          `json:"fontFamily,omitempty"`
	FontSize       float64         `json:"fontSize,omitempty"`
	Color          [4]uint8        `json:"color,omitempty"`
	Path           *Path           `json:"path,omitempty"`
	FillColor      [4]uint8        `json:"fillColor,omitempty"`
	StrokeColor    [4]uint8        `json:"strokeColor,omitempty"`
	StrokeWidth    float64         `json:"strokeWidth,omitempty"`
	CachedRaster   []byte          `json:"cachedRaster,omitempty"`
	Isolated       bool            `json:"isolated,omitempty"`
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

type TranslateLayerPayload struct {
	LayerID string `json:"layerId"`
	DX      int    `json:"dx"`
	DY      int    `json:"dy"`
}

type PickLayerAtPointPayload struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// BeginFreeTransformPayload starts free transform on a layer (or the active
// layer when LayerID is empty).
// Mode, when non-empty, selects the initial transform sub-mode:
//   - "warp" – initialise a 4×4 control-point mesh from the layer corners.
//   - anything else (or empty) – normal affine free transform.
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
	// 4×4 grid of control points in doc space; row-major [row][col][x,y].
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

type LayerNodeMeta struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	LayerType      LayerType       `json:"layerType"`
	AdjustmentKind string          `json:"adjustmentKind,omitempty"`
	Params         json.RawMessage `json:"params,omitempty"`
	ParentID       string          `json:"parentId,omitempty"`
	Visible        bool            `json:"visible"`
	LockMode       LayerLockMode   `json:"lockMode"`
	Opacity        float64         `json:"opacity"`
	FillOpacity    float64         `json:"fillOpacity"`
	BlendMode      BlendMode       `json:"blendMode"`
	ClipToBelow    bool            `json:"clipToBelow"`
	ClippingBase   bool            `json:"clippingBase"`
	HasMask        bool            `json:"hasMask"`
	MaskEnabled    bool            `json:"maskEnabled"`
	HasVectorMask  bool            `json:"hasVectorMask"`
	Isolated       bool            `json:"isolated,omitempty"`
	Children       []LayerNodeMeta `json:"children,omitempty"`
	// VectorLayer-specific style fields. Only populated when LayerType == "vector".
	VecFillColor   *[4]uint8 `json:"fillColor,omitempty"`
	VecStrokeColor *[4]uint8 `json:"strokeColor,omitempty"`
	VecStrokeWidth *float64  `json:"strokeWidth,omitempty"`
}

func (doc *Document) ensureLayerRoot() *GroupLayer {
	if doc.LayerRoot != nil {
		return doc.LayerRoot
	}
	root := NewGroupLayer("Root")
	root.SetName("Root")
	root.SetParent(nil)
	doc.LayerRoot = root
	return root
}

func (doc *Document) Layers() []LayerNode {
	return doc.ensureLayerRoot().Children()
}

func (doc *Document) ActiveLayer() LayerNode {
	if doc == nil || doc.ActiveLayerID == "" {
		return nil
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID)
	if !ok {
		return nil
	}
	return layer
}

func (doc *Document) LayerMeta() []LayerNodeMeta {
	if doc == nil {
		return nil
	}
	children := doc.ensureLayerRoot().Children()
	meta := make([]LayerNodeMeta, 0, len(children))
	for _, child := range children {
		meta = append(meta, buildLayerNodeMeta(child))
	}
	return meta
}

func (doc *Document) AddLayer(layer LayerNode, parentLayerID string, index int) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if layer == nil {
		return fmt.Errorf("layer is required")
	}
	parent, err := doc.groupForID(parentLayerID)
	if err != nil {
		return err
	}
	insertChild(parent, layer, index)
	doc.normalizeClippingState()
	doc.ActiveLayerID = layer.ID()
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) DeleteLayer(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || parent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	children := parent.Children()
	nextActive := doc.nextActiveLayerID(children, index, layer)
	children = append(children[:index], children[index+1:]...)
	parent.SetChildren(children)
	doc.normalizeClippingState()
	if containsLayerID(layer, doc.ActiveLayerID) {
		doc.ActiveLayerID = nextActive
	}
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) DuplicateLayer(layerID, parentLayerID string, index int) (LayerNode, error) {
	source, parent, sourceIndex, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return nil, fmt.Errorf("layer %q not found", layerID)
	}
	clone := cloneLayerForDuplicate(source)
	targetParent := parent
	if parentLayerID != "" {
		var err error
		targetParent, err = doc.groupForID(parentLayerID)
		if err != nil {
			return nil, err
		}
	}
	if targetParent == parent && index < 0 {
		index = sourceIndex + 1
	}
	insertChild(targetParent, clone, index)
	doc.normalizeClippingState()
	doc.ActiveLayerID = clone.ID()
	doc.touchModifiedAt()
	return clone, nil
}

func (doc *Document) MoveLayer(layerID, parentLayerID string, index int) error {
	layer, currentParent, currentIndex, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || currentParent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	targetParent, err := doc.groupForID(parentLayerID)
	if err != nil {
		return err
	}
	if containsLayerID(layer, targetParent.ID()) {
		return fmt.Errorf("cannot move layer into its own descendant")
	}
	currentChildren := currentParent.Children()
	currentChildren = append(currentChildren[:currentIndex], currentChildren[currentIndex+1:]...)
	currentParent.SetChildren(currentChildren)
	if targetParent == currentParent && index > currentIndex {
		index--
	}
	insertChild(targetParent, layer, index)
	doc.normalizeClippingState()
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerVisibility(layerID string, visible bool) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetVisible(visible)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerOpacity(layerID string, opacity, fillOpacity *float64) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if opacity != nil {
		layer.SetOpacity(*opacity)
	}
	if fillOpacity != nil {
		layer.SetFillOpacity(*fillOpacity)
	}
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerBlendMode(layerID string, mode BlendMode) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetBlendMode(mode)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetAdjustmentLayerParams(layerID, adjustmentKind string, params json.RawMessage) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	typed, ok := layer.(*AdjustmentLayer)
	if !ok {
		return fmt.Errorf("layer %q is not an adjustment layer", layer.Name())
	}
	if adjustmentKind != "" {
		typed.AdjustmentKind = adjustmentKind
	}
	if typed.AdjustmentKind == "" {
		return fmt.Errorf("adjustment layer %q requires adjustmentKind", layer.Name())
	}
	typed.Params = cloneJSONRawMessage(params)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerLock(layerID string, mode LayerLockMode) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetLockMode(mode)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetActiveLayer(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if layerID == "" {
		doc.ActiveLayerID = ""
		return nil
	}
	if _, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID); !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	doc.ActiveLayerID = layerID
	return nil
}

func (doc *Document) SetLayerName(layerID, name string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetName(name)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) TranslateLayer(layerID string, dx, dy int) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if dx == 0 && dy == 0 {
		return nil
	}
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	if layerID == "" {
		return fmt.Errorf("no active layer")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if isLayerPositionLocked(layer) {
		return fmt.Errorf("layer %q position is locked", layer.Name())
	}
	if err := translateLayerNode(layer, dx, dy); err != nil {
		return err
	}
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) PickLayerAtPoint(x, y int) (LayerNode, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	if x < 0 || x >= doc.Width || y < 0 || y >= doc.Height {
		doc.ActiveLayerID = ""
		return nil, nil
	}
	layer := topmostLayerAtPoint(doc, doc.ensureLayerRoot().Children(), x, y)
	if layer == nil {
		doc.ActiveLayerID = ""
		return nil, nil
	}
	doc.ActiveLayerID = layer.ID()
	return layer, nil
}

func (doc *Document) AddLayerMask(layerID string, mode AddLayerMaskMode) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.Mask() != nil {
		return fmt.Errorf("layer %q already has a mask", layer.Name())
	}
	var fill byte
	switch mode {
	case "", AddLayerMaskRevealAll:
		fill = 255
	case AddLayerMaskHideAll:
		fill = 0
	case AddLayerMaskFromSelection:
		selection := normalizeSelection(cloneSelection(doc.Selection))
		if selection == nil {
			return fmt.Errorf("layer %q cannot create a mask without an active selection", layer.Name())
		}
		layer.SetMask(newLayerMaskFromSelection(selection))
		doc.touchModifiedAt()
		return nil
	default:
		return fmt.Errorf("unsupported layer mask mode %q", mode)
	}
	layer.SetMask(newFilledLayerMask(doc.Width, doc.Height, fill))
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) DeleteLayerMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.Mask() == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	layer.SetMask(nil)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) ApplyLayerMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	switch typed := layer.(type) {
	case *PixelLayer:
		if err := applyMaskToLayerRaster(typed.Bounds, typed.Pixels, mask); err != nil {
			return err
		}
	case *TextLayer:
		if err := applyMaskToLayerRaster(typed.Bounds, typed.CachedRaster, mask); err != nil {
			return err
		}
	case *VectorLayer:
		if err := applyMaskToLayerRaster(typed.Bounds, typed.CachedRaster, mask); err != nil {
			return err
		}
	case *GroupLayer:
		return fmt.Errorf("layer %q cannot apply a group mask without rasterizing the group", layer.Name())
	case *AdjustmentLayer:
		return fmt.Errorf("layer %q cannot apply a mask without raster content", layer.Name())
	default:
		return fmt.Errorf("unsupported layer type %T", layer)
	}
	layer.SetMask(nil)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) InvertLayerMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	inverted := cloneLayerMask(mask)
	for index := range inverted.Data {
		inverted.Data[index] = 255 - inverted.Data[index]
	}
	layer.SetMask(inverted)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerMaskEnabled(layerID string, enabled bool) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	updated := cloneLayerMask(mask)
	updated.Enabled = enabled
	layer.SetMask(updated)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) AddVectorMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.VectorMask() != nil {
		return fmt.Errorf("layer %q already has a vector mask", layer.Name())
	}
	// Placeholder: creates an empty path. Full path editing deferred to Phase 6.1.
	layer.SetVectorMask(&Path{Subpaths: []Subpath{{Closed: true}}})
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) DeleteVectorMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.VectorMask() == nil {
		return fmt.Errorf("layer %q has no vector mask", layer.Name())
	}
	layer.SetVectorMask(nil)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerClipToBelow(layerID string, clipToBelow bool) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || parent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if clipToBelow {
		children := parent.Children()
		if clippingBaseIndex(children, index) < 0 {
			return fmt.Errorf("layer %q cannot clip without a base layer below it", layer.Name())
		}
	}
	layer.SetClipToBelow(clipToBelow)
	doc.normalizeClippingState()
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) FlattenLayer(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || parent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	flattened, err := doc.rasterizeAsPixelLayer(layer, layer.Name())
	if err != nil {
		return err
	}
	replaceChild(parent, index, flattened)
	doc.normalizeClippingState()
	doc.ActiveLayerID = flattened.ID()
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) MergeDown(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || parent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if index == 0 {
		return fmt.Errorf("layer %q has no layer below it", layerID)
	}
	children := parent.Children()
	below := children[index-1]
	merged, err := doc.mergeNodesToPixelLayer(below, layer, fmt.Sprintf("%s + %s", below.Name(), layer.Name()))
	if err != nil {
		return err
	}
	children[index-1] = merged
	children = append(children[:index], children[index+1:]...)
	parent.SetChildren(children)
	doc.normalizeClippingState()
	doc.ActiveLayerID = merged.ID()
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) MergeVisible() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	root := doc.ensureLayerRoot()
	children := root.Children()

	hasVisible := false
	for _, child := range children {
		if child.Visible() {
			hasVisible = true
			break
		}
	}
	if !hasVisible {
		return fmt.Errorf("no visible layers to merge")
	}

	// Render the full document composite so that all nested visible content at every
	// level of the layer hierarchy is included, not just root-level layers.
	surface := doc.renderCompositeSurface()
	if surface == nil {
		return fmt.Errorf("failed to render composite surface")
	}
	merged := NewPixelLayer("Merged Visible", LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, surface)

	// Preserve root-level hidden layers; visible content (including groups and their
	// nested children) is represented by the merged composite.
	hidden := make([]LayerNode, 0, len(children))
	for _, child := range children {
		if !child.Visible() {
			hidden = append(hidden, child)
		}
	}
	hidden = append(hidden, merged)
	root.SetChildren(hidden)
	doc.normalizeClippingState()
	doc.ActiveLayerID = merged.ID()
	doc.touchModifiedAt()
	return nil
}

// FlattenImage merges all visible layers into a single "Background" pixel layer,
// discarding hidden layers. This is the standard Photoshop "Flatten Image" behaviour.
func (doc *Document) FlattenImage() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	root := doc.ensureLayerRoot()
	children := root.Children()

	hasVisible := false
	for _, child := range children {
		if child.Visible() {
			hasVisible = true
			break
		}
	}
	if !hasVisible {
		return fmt.Errorf("no visible layers to flatten")
	}

	surface := doc.renderCompositeSurface()
	if surface == nil {
		return fmt.Errorf("failed to render composite surface")
	}
	flattened := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, surface)
	root.SetChildren([]LayerNode{flattened})
	doc.normalizeClippingState()
	doc.ActiveLayerID = flattened.ID()
	doc.touchModifiedAt()
	return nil
}

// generateAllThumbnails builds thumbnailSize×thumbnailSize previews for every
// layer in the active document. The result is a map of layer ID → ThumbnailEntry.
func (doc *Document) generateAllThumbnails(thumbW, thumbH int) (map[string]ThumbnailEntry, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	all := flattenLayerTree(doc.ensureLayerRoot().Children())
	result := make(map[string]ThumbnailEntry, len(all))
	for _, layer := range all {
		layerRGBA, maskRGBA := doc.generateLayerThumbnail(layer, thumbW, thumbH)
		entry := ThumbnailEntry{
			LayerRGBA: base64.StdEncoding.EncodeToString(layerRGBA),
		}
		if len(maskRGBA) > 0 {
			entry.MaskRGBA = base64.StdEncoding.EncodeToString(maskRGBA)
		}
		result[layer.ID()] = entry
	}
	return result, nil
}

// generateLayerThumbnail returns RGBA pixel data for the layer thumbnail and (if
// a mask is present) the mask thumbnail. Both buffers are thumbW×thumbH×4 bytes.
func (doc *Document) generateLayerThumbnail(layer LayerNode, thumbW, thumbH int) (layerRGBA, maskRGBA []byte) {
	switch typed := layer.(type) {
	case *PixelLayer:
		if len(typed.Pixels) > 0 && typed.Bounds.W > 0 && typed.Bounds.H > 0 {
			layerRGBA = scaleRGBA(typed.Pixels, typed.Bounds.W, typed.Bounds.H, thumbW, thumbH)
		}
	case *TextLayer:
		if len(typed.CachedRaster) > 0 && typed.Bounds.W > 0 && typed.Bounds.H > 0 {
			layerRGBA = scaleRGBA(typed.CachedRaster, typed.Bounds.W, typed.Bounds.H, thumbW, thumbH)
		}
	case *VectorLayer:
		if len(typed.CachedRaster) > 0 && typed.Bounds.W > 0 && typed.Bounds.H > 0 {
			layerRGBA = scaleRGBA(typed.CachedRaster, typed.Bounds.W, typed.Bounds.H, thumbW, thumbH)
		}
	case *GroupLayer:
		if len(typed.Children()) > 0 {
			buf, err := doc.renderLayerToSurface(typed)
			if err == nil {
				layerRGBA = scaleRGBA(buf, doc.Width, doc.Height, thumbW, thumbH)
			}
		}
	}

	if mask := layer.Mask(); mask != nil && len(mask.Data) > 0 && mask.Width > 0 && mask.Height > 0 {
		maskRGBA = scaleGrayToRGBA(mask.Data, mask.Width, mask.Height, thumbW, thumbH)
	}
	return layerRGBA, maskRGBA
}

// flattenLayerTree returns all layers in depth-first order, recursing into groups.
func flattenLayerTree(layers []LayerNode) []LayerNode {
	result := make([]LayerNode, 0, len(layers))
	for _, layer := range layers {
		result = append(result, layer)
		if children := layer.Children(); len(children) > 0 {
			result = append(result, flattenLayerTree(children)...)
		}
	}
	return result
}

// scaleRGBA downsamples an RGBA pixel buffer using nearest-neighbour sampling.
// The source buffer must be srcW×srcH×4 bytes; the result is dstW×dstH×4 bytes.
func scaleRGBA(src []byte, srcW, srcH, dstW, dstH int) []byte {
	dst := make([]byte, dstW*dstH*4)
	for y := range dstH {
		for x := range dstW {
			sx := x * srcW / dstW
			sy := y * srcH / dstH
			si := (sy*srcW + sx) * 4
			di := (y*dstW + x) * 4
			if si+4 <= len(src) {
				copy(dst[di:di+4], src[si:si+4])
			}
		}
	}
	return dst
}

// scaleGrayToRGBA downsamples a grayscale mask buffer and converts it to RGBA.
// The source buffer must be srcW×srcH×1 bytes (one byte per pixel, 0=black, 255=white).
func scaleGrayToRGBA(src []byte, srcW, srcH, dstW, dstH int) []byte {
	dst := make([]byte, dstW*dstH*4)
	for y := range dstH {
		for x := range dstW {
			sx := x * srcW / dstW
			sy := y * srcH / dstH
			si := sy*srcW + sx
			di := (y*dstW + x) * 4
			var gray byte
			if si < len(src) {
				gray = src[si]
			}
			dst[di] = gray
			dst[di+1] = gray
			dst[di+2] = gray
			dst[di+3] = 255
		}
	}
	return dst
}

func (doc *Document) groupForID(groupID string) (*GroupLayer, error) {
	root := doc.ensureLayerRoot()
	if groupID == "" || groupID == root.ID() {
		return root, nil
	}
	layer, _, _, ok := findLayerByID(root, groupID)
	if !ok {
		return nil, fmt.Errorf("parent layer %q not found", groupID)
	}
	group, ok := layer.(*GroupLayer)
	if !ok {
		return nil, fmt.Errorf("layer %q is not a group", groupID)
	}
	return group, nil
}

func (doc *Document) nextActiveLayerID(children []LayerNode, deletedIndex int, deleted LayerNode) string {
	// Prefer the sibling immediately below the deleted layer.
	if deletedIndex+1 < len(children) {
		return children[deletedIndex+1].ID()
	}
	// Fall back to the sibling above.
	if deletedIndex > 0 {
		return children[deletedIndex-1].ID()
	}
	// No siblings; select the parent unless it is the root.
	if parent := deleted.Parent(); parent != nil && parent.Parent() != nil {
		return parent.ID()
	}
	return ""
}

func (doc *Document) touchModifiedAt() {
	doc.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	doc.ContentVersion = atomic.AddInt64(&nextDocVersion, 1)
}

func (doc *Document) newLayerFromPayload(payload AddLayerPayload) (LayerNode, error) {
	switch payload.LayerType {
	case LayerTypePixel:
		if payload.Bounds.W <= 0 || payload.Bounds.H <= 0 {
			return nil, fmt.Errorf("pixel layer requires valid bounds, got %dx%d", payload.Bounds.W, payload.Bounds.H)
		}
		pixels := payload.Pixels
		if len(pixels) == 0 {
			pixels = make([]byte, payload.Bounds.W*payload.Bounds.H*4)
		}
		return NewPixelLayer(payload.Name, payload.Bounds, pixels), nil
	case LayerTypeGroup:
		group := NewGroupLayer(payload.Name)
		group.Isolated = payload.Isolated
		return group, nil
	case LayerTypeAdjustment:
		if payload.AdjustmentKind == "" {
			return nil, fmt.Errorf("adjustment layer requires adjustmentKind")
		}
		return NewAdjustmentLayer(payload.Name, payload.AdjustmentKind, payload.Params), nil
	case LayerTypeText:
		layer := NewTextLayer(payload.Name, payload.Bounds, payload.Text, payload.CachedRaster)
		if payload.FontFamily != "" {
			layer.FontFamily = payload.FontFamily
		}
		if payload.FontSize > 0 {
			layer.FontSize = payload.FontSize
		}
		if payload.Color != [4]uint8{} {
			layer.Color = payload.Color
		}
		return layer, nil
	case LayerTypeVector:
		layer := NewVectorLayer(payload.Name, payload.Bounds, payload.Path, payload.CachedRaster)
		if payload.FillColor != [4]uint8{} {
			layer.FillColor = payload.FillColor
		}
		if payload.StrokeColor != [4]uint8{} {
			layer.StrokeColor = payload.StrokeColor
		}
		if payload.StrokeWidth > 0 {
			layer.StrokeWidth = payload.StrokeWidth
		}
		return layer, nil
	default:
		return nil, fmt.Errorf("unsupported layer type %q", payload.LayerType)
	}
}

func (doc *Document) rasterizeAsPixelLayer(layer LayerNode, name string) (*PixelLayer, error) {
	if _, ok := layer.(*AdjustmentLayer); ok {
		return nil, fmt.Errorf("layer %q cannot be flattened without raster content", layer.Name())
	}
	buffer, err := doc.renderLayerToSurface(layer)
	if err != nil {
		return nil, err
	}
	pixelLayer := NewPixelLayer(name, LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, buffer)
	pixelLayer.SetOpacity(layer.Opacity())
	pixelLayer.SetFillOpacity(layer.FillOpacity())
	pixelLayer.SetBlendMode(layer.BlendMode())
	pixelLayer.SetVisible(layer.Visible())
	pixelLayer.SetLockMode(layer.LockMode())
	return pixelLayer, nil
}

func (doc *Document) mergeNodesToPixelLayer(bottom, top LayerNode, name string) (*PixelLayer, error) {
	buffer, err := doc.renderLayersToSurface([]LayerNode{bottom, top})
	if err != nil {
		return nil, err
	}
	return NewPixelLayer(name, LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, buffer), nil
}

func (doc *Document) renderLayerToSurface(layer LayerNode) ([]byte, error) {
	buffer := make([]byte, doc.Width*doc.Height*4)
	clipAlpha, err := doc.clippingBaseSurfaceForLayer(layer)
	if err != nil {
		return nil, err
	}
	if err := doc.compositeLayerOntoWithClip(buffer, layer, clipAlpha); err != nil {
		return nil, err
	}
	return buffer, nil
}

func (doc *Document) renderLayersToSurface(layers []LayerNode) ([]byte, error) {
	buffer := make([]byte, doc.Width*doc.Height*4)
	if err := doc.compositeLayerStackOnto(buffer, layers, nil); err != nil {
		return nil, err
	}
	return buffer, nil
}

// compositeLayerOnto is retained for test coverage and delegates to the
// clip-aware compositor used by production code.
//
//nolint:unused
func (doc *Document) compositeLayerOnto(dest []byte, layer LayerNode) error {
	return doc.compositeLayerOntoWithClip(dest, layer, nil)
}

func (doc *Document) compositeLayerOntoWithClip(dest []byte, layer LayerNode, clipAlpha []byte) error {
	if layer == nil || !layer.Visible() {
		return nil
	}
	if err := ensureRasterizableLayer(layer); err != nil {
		return err
	}
	switch typed := layer.(type) {
	case *PixelLayer:
		return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.Pixels, typed.BlendMode(), effectiveContentOpacity(typed), typed.Mask(), clipAlpha)
	case *TextLayer:
		return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.CachedRaster, typed.BlendMode(), effectiveContentOpacity(typed), typed.Mask(), clipAlpha)
	case *VectorLayer:
		return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.CachedRaster, typed.BlendMode(), effectiveContentOpacity(typed), typed.Mask(), clipAlpha)
	case *AdjustmentLayer:
		return applyAdjustmentLayerToSurface(dest, doc.Width, doc.Height, typed, clipAlpha)
	case *GroupLayer:
		if !typed.Isolated && typed.BlendMode() == BlendModeNormal && effectiveLayerOpacity(typed) >= 1 && typed.Mask() == nil {
			return doc.compositeLayerStackOnto(dest, typed.Children(), clipAlpha)
		}
		temp := make([]byte, len(dest))
		if err := doc.compositeLayerStackOnto(temp, typed.Children(), nil); err != nil {
			return err
		}
		applyLayerMaskToSurface(temp, doc.Width, doc.Height, typed.Mask())
		applyClipSurfaceToSurface(temp, clipAlpha)
		compositeDocumentSurface(dest, temp, typed.BlendMode(), effectiveLayerOpacity(typed))
		return nil
	default:
		return fmt.Errorf("unsupported layer type %T", layer)
	}
}

func (doc *Document) compositeLayerStackOnto(dest []byte, layers []LayerNode, clipAlpha []byte) error {
	for index := 0; index < len(layers); index++ {
		layer := layers[index]
		if layer == nil {
			continue
		}
		if layer.ClipToBelow() {
			if err := doc.compositeLayerOntoWithClip(dest, layer, clipAlpha); err != nil {
				return err
			}
			continue
		}
		if err := doc.compositeLayerOntoWithClip(dest, layer, clipAlpha); err != nil {
			return err
		}
		if index+1 >= len(layers) || !layers[index+1].ClipToBelow() {
			continue
		}
		baseIndex := clippingBaseRenderableIndex(layers, index)
		if baseIndex < 0 {
			continue
		}
		baseSurface, err := doc.renderLayerToSurface(layers[baseIndex])
		if err != nil {
			return err
		}
		combinedClip := combineClipSurface(baseSurface, clipAlpha)
		for next := index + 1; next < len(layers) && layers[next].ClipToBelow(); next++ {
			if err := doc.compositeLayerOntoWithClip(dest, layers[next], combinedClip); err != nil {
				return err
			}
			index = next
		}
	}
	return nil
}

func ensureRasterizableLayer(layer LayerNode) error {
	// Vector masks are not yet rasterized during compositing (Phase 6.1).
	// They are stored as data but silently ignored in rendering for now.
	if len(layer.StyleStack()) > 0 {
		return fmt.Errorf("layer %q cannot be merged while layer styles are not rasterized", layer.Name())
	}
	return nil
}

// effectiveLayerOpacity returns the whole-layer composite opacity, which applies
// to the complete layer surface including any layer effects (Phase 5). Use this
// when compositing groups or checking pass-through conditions.
func effectiveLayerOpacity(layer LayerNode) float64 {
	return layer.Opacity()
}

// effectiveContentOpacity returns the opacity used when compositing pixel content.
// Fill opacity reduces only the layer's own pixels, not its effects (drop shadows,
// strokes, etc. added in Phase 5). Use this for PixelLayer, TextLayer, VectorLayer.
func effectiveContentOpacity(layer LayerNode) float64 {
	return clampUnit(layer.Opacity() * layer.FillOpacity())
}

func compositeRasterIntoDocument(dest []byte, docW, docH int, bounds LayerBounds, src []byte, blendMode BlendMode, opacity float64, mask *LayerMask, clipAlpha []byte) error {
	if bounds.W <= 0 || bounds.H <= 0 || len(src) == 0 || opacity <= 0 {
		return nil
	}
	expectedLen := bounds.W * bounds.H * 4
	if len(src) != expectedLen {
		return fmt.Errorf("raster length %d does not match bounds %dx%d", len(src), bounds.W, bounds.H)
	}
	for y := 0; y < bounds.H; y++ {
		docY := bounds.Y + y
		if docY < 0 || docY >= docH {
			continue
		}
		for x := 0; x < bounds.W; x++ {
			docX := bounds.X + x
			if docX < 0 || docX >= docW {
				continue
			}
			srcIndex := (y*bounds.W + x) * 4
			maskAlpha := layerMaskAlphaAt(mask, docX, docY)
			maskAlpha = scaleMaskedAlpha(maskAlpha, clipSurfaceAlphaAt(clipAlpha, docW, docX, docY))
			if maskAlpha == 0 {
				continue
			}
			destIndex := (docY*docW + docX) * 4
			srcPixel := src[srcIndex : srcIndex+4]
			if maskAlpha == 255 {
				compositePixelWithBlend(dest[destIndex:destIndex+4], srcPixel, blendMode, opacity, pixelNoiseSeed(docX, docY))
				continue
			}
			var masked [4]byte
			copy(masked[:], srcPixel)
			masked[3] = scaleMaskedAlpha(srcPixel[3], maskAlpha)
			if masked[3] == 0 {
				continue
			}
			compositePixelWithBlend(dest[destIndex:destIndex+4], masked[:], blendMode, opacity, pixelNoiseSeed(docX, docY))
		}
	}
	return nil
}

func applyLayerMaskToSurface(surface []byte, docW, docH int, mask *LayerMask) {
	if len(surface) == 0 || docW <= 0 || docH <= 0 || mask == nil || !mask.Enabled {
		return
	}
	for docY := range docH {
		for docX := range docW {
			maskAlpha := layerMaskAlphaAt(mask, docX, docY)
			if maskAlpha == 255 {
				continue
			}
			index := (docY*docW + docX) * 4
			surface[index+3] = scaleMaskedAlpha(surface[index+3], maskAlpha)
		}
	}
}

func applyClipSurfaceToSurface(surface, clipAlpha []byte) {
	if len(surface) == 0 || len(clipAlpha) != len(surface) {
		return
	}
	for offset := 0; offset < len(surface); offset += 4 {
		surface[offset+3] = scaleMaskedAlpha(surface[offset+3], clipAlpha[offset+3])
	}
}

func layerMaskAlphaAt(mask *LayerMask, docX, docY int) uint8 {
	if mask == nil || !mask.Enabled || mask.Width <= 0 || mask.Height <= 0 {
		return 255
	}
	expectedLen := mask.Width * mask.Height
	if len(mask.Data) < expectedLen {
		return 255
	}
	if docX < 0 || docX >= mask.Width || docY < 0 || docY >= mask.Height {
		return 0
	}
	return mask.Data[docY*mask.Width+docX]
}

func scaleMaskedAlpha(alpha, maskAlpha uint8) uint8 {
	return uint8((uint16(alpha)*uint16(maskAlpha) + 127) / 255)
}

func clipSurfaceAlphaAt(surface []byte, width, x, y int) uint8 {
	if len(surface) == 0 || width <= 0 || x < 0 || y < 0 {
		return 255
	}
	index := (y*width + x) * 4
	if index < 0 || index+3 >= len(surface) {
		return 0
	}
	return surface[index+3]
}

func combineClipSurface(baseSurface, clipAlpha []byte) []byte {
	if len(baseSurface) == 0 {
		return clipAlpha
	}
	if len(clipAlpha) == 0 {
		return baseSurface
	}
	combined := append([]byte(nil), baseSurface...)
	applyClipSurfaceToSurface(combined, clipAlpha)
	return combined
}

func newFilledLayerMask(width, height int, fill byte) *LayerMask {
	if width <= 0 || height <= 0 {
		return &LayerMask{Enabled: true, Width: width, Height: height}
	}
	data := make([]byte, width*height)
	if fill != 0 {
		for index := range data {
			data[index] = fill
		}
	}
	return &LayerMask{Enabled: true, Width: width, Height: height, Data: data}
}

func applyMaskToLayerRaster(bounds LayerBounds, raster []byte, mask *LayerMask) error {
	if bounds.W <= 0 || bounds.H <= 0 || len(raster) == 0 || mask == nil {
		return nil
	}
	expectedLen := bounds.W * bounds.H * 4
	if len(raster) != expectedLen {
		return fmt.Errorf("raster length %d does not match bounds %dx%d", len(raster), bounds.W, bounds.H)
	}
	for y := 0; y < bounds.H; y++ {
		docY := bounds.Y + y
		for x := 0; x < bounds.W; x++ {
			docX := bounds.X + x
			alpha := layerMaskDataAlphaAt(mask, docX, docY)
			if alpha == 255 {
				continue
			}
			pixelIndex := (y*bounds.W + x) * 4
			raster[pixelIndex+3] = scaleMaskedAlpha(raster[pixelIndex+3], alpha)
		}
	}
	return nil
}

func layerMaskDataAlphaAt(mask *LayerMask, docX, docY int) uint8 {
	if mask == nil || mask.Width <= 0 || mask.Height <= 0 {
		return 255
	}
	expectedLen := mask.Width * mask.Height
	if len(mask.Data) < expectedLen {
		return 255
	}
	if docX < 0 || docX >= mask.Width || docY < 0 || docY >= mask.Height {
		return 0
	}
	return mask.Data[docY*mask.Width+docX]
}

func (doc *Document) normalizeClippingState() {
	if doc == nil {
		return
	}
	normalizeGroupClipping(doc.ensureLayerRoot())
}

func normalizeGroupClipping(group *GroupLayer) {
	if group == nil {
		return
	}
	children := group.Children()
	for _, child := range children {
		child.SetClippingBase(false)
		if nested, ok := child.(*GroupLayer); ok {
			normalizeGroupClipping(nested)
		}
	}
	for index, child := range children {
		baseIndex := clippingBaseIndex(children, index)
		if !child.ClipToBelow() {
			continue
		}
		if baseIndex < 0 {
			child.SetClipToBelow(false)
			continue
		}
		children[baseIndex].SetClippingBase(true)
	}
	group.SetChildren(children)
}

func clippingBaseIndex(children []LayerNode, index int) int {
	for candidate := index - 1; candidate >= 0; candidate-- {
		if children[candidate] == nil {
			continue
		}
		if _, ok := children[candidate].(*AdjustmentLayer); ok {
			continue
		}
		if !children[candidate].ClipToBelow() {
			return candidate
		}
	}
	return -1
}

func clippingBaseRenderableIndex(children []LayerNode, index int) int {
	if index < 0 || index >= len(children) {
		return -1
	}
	if _, ok := children[index].(*AdjustmentLayer); !ok {
		return index
	}
	for candidate := index - 1; candidate >= 0; candidate-- {
		layer := children[candidate]
		if layer == nil || layer.ClipToBelow() {
			continue
		}
		if _, ok := layer.(*AdjustmentLayer); ok {
			continue
		}
		return candidate
	}
	return -1
}

func (doc *Document) clippingBaseSurfaceForLayer(layer LayerNode) ([]byte, error) {
	if doc == nil || layer == nil || !layer.ClipToBelow() {
		return nil, nil
	}
	parent := layer.Parent()
	group, ok := parent.(*GroupLayer)
	if !ok || group == nil {
		return nil, nil
	}
	children := group.Children()
	for index, candidate := range children {
		if candidate == nil || candidate.ID() != layer.ID() {
			continue
		}
		baseIndex := clippingBaseIndex(children, index)
		if baseIndex < 0 {
			return nil, nil
		}
		return doc.renderLayerToSurface(children[baseIndex])
	}
	return nil, nil
}

func compositeDocumentSurface(dest, src []byte, blendMode BlendMode, opacity float64) {
	if len(dest) != len(src) || opacity <= 0 {
		return
	}
	for offset := 0; offset < len(dest); offset += 4 {
		compositePixelWithBlend(dest[offset:offset+4], src[offset:offset+4], blendMode, opacity, uint32(offset/4))
	}
}

func pixelNoiseSeed(x, y int) uint32 {
	seed := uint32(x)*73856093 ^ uint32(y)*19349663 ^ 0x9e3779b9
	seed ^= seed >> 16
	return seed
}

func buildLayerNodeMeta(layer LayerNode) LayerNodeMeta {
	mask := layer.Mask()
	meta := LayerNodeMeta{
		ID:            layer.ID(),
		Name:          layer.Name(),
		LayerType:     layer.LayerType(),
		Visible:       layer.Visible(),
		LockMode:      layer.LockMode(),
		Opacity:       layer.Opacity(),
		FillOpacity:   layer.FillOpacity(),
		BlendMode:     layer.BlendMode(),
		ClipToBelow:   layer.ClipToBelow(),
		ClippingBase:  layer.ClippingBase(),
		HasMask:       mask != nil,
		MaskEnabled:   mask != nil && mask.Enabled,
		HasVectorMask: layer.VectorMask() != nil,
	}
	if adjustment, ok := layer.(*AdjustmentLayer); ok {
		meta.AdjustmentKind = adjustment.AdjustmentKind
		meta.Params = cloneJSONRawMessage(adjustment.Params)
	}
	if parent := layer.Parent(); parent != nil {
		meta.ParentID = parent.ID()
	}
	if group, ok := layer.(*GroupLayer); ok {
		meta.Isolated = group.Isolated
		children := group.Children()
		meta.Children = make([]LayerNodeMeta, 0, len(children))
		for _, child := range children {
			meta.Children = append(meta.Children, buildLayerNodeMeta(child))
		}
	}
	if vl, ok := layer.(*VectorLayer); ok {
		fc := vl.FillColor
		sc := vl.StrokeColor
		sw := vl.StrokeWidth
		meta.VecFillColor = &fc
		meta.VecStrokeColor = &sc
		meta.VecStrokeWidth = &sw
	}
	return meta
}

func insertChild(parent *GroupLayer, layer LayerNode, index int) {
	children := parent.Children()
	if index < 0 || index > len(children) {
		index = len(children)
	}
	updated := make([]LayerNode, 0, len(children)+1)
	updated = append(updated, children[:index]...)
	updated = append(updated, layer)
	updated = append(updated, children[index:]...)
	parent.SetChildren(updated)
}

func replaceChild(parent *GroupLayer, index int, layer LayerNode) {
	children := parent.Children()
	children[index] = layer
	parent.SetChildren(children)
}

// findLayer is a Document-scoped shortcut that returns the LayerNode with the
// given ID, or nil when not found.
func (doc *Document) findLayer(layerID string) LayerNode {
	if layerID == "" {
		return nil
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return nil
	}
	return layer
}

// kindDescription returns a human-readable history description for discrete
// transform kinds.
func kindDescription(kind string) string {
	switch kind {
	case "flipH":
		return "Flip Horizontal"
	case "flipV":
		return "Flip Vertical"
	case "rotate90cw":
		return "Rotate 90° CW"
	case "rotate90ccw":
		return "Rotate 90° CCW"
	case "rotate180":
		return "Rotate 180°"
	default:
		return "Transform"
	}
}

func findLayerByID(group *GroupLayer, layerID string) (LayerNode, *GroupLayer, int, bool) {
	children := group.Children()
	for index, child := range children {
		if child.ID() == layerID {
			return child, group, index, true
		}
		if nestedGroup, ok := child.(*GroupLayer); ok {
			if layer, parent, childIndex, found := findLayerByID(nestedGroup, layerID); found {
				return layer, parent, childIndex, true
			}
		}
	}
	return nil, nil, -1, false
}

func containsLayerID(layer LayerNode, targetID string) bool {
	if layer == nil || targetID == "" {
		return false
	}
	if layer.ID() == targetID {
		return true
	}
	for _, child := range layer.Children() {
		if containsLayerID(child, targetID) {
			return true
		}
	}
	return false
}

func isLayerPositionLocked(layer LayerNode) bool {
	if layer == nil {
		return false
	}
	switch layer.LockMode() {
	case LayerLockPosition, LayerLockAll:
		return true
	default:
		return false
	}
}

func translateLayerNode(layer LayerNode, dx, dy int) error {
	switch typed := layer.(type) {
	case *PixelLayer:
		typed.Bounds.X += dx
		typed.Bounds.Y += dy
		return nil
	case *TextLayer:
		typed.Bounds.X += dx
		typed.Bounds.Y += dy
		return nil
	case *VectorLayer:
		typed.Bounds.X += dx
		typed.Bounds.Y += dy
		return nil
	case *GroupLayer:
		for _, child := range typed.Children() {
			if err := translateLayerNode(child, dx, dy); err != nil {
				return err
			}
		}
		return nil
	case *AdjustmentLayer:
		return fmt.Errorf("adjustment layer %q cannot be moved", typed.Name())
	default:
		return fmt.Errorf("unsupported layer type %T", layer)
	}
}

func topmostLayerAtPoint(doc *Document, layers []LayerNode, x, y int) LayerNode {
	for index := len(layers) - 1; index >= 0; index-- {
		layer := layers[index]
		if layer == nil || !layer.Visible() {
			continue
		}
		if group, ok := layer.(*GroupLayer); ok {
			if hit := topmostLayerAtPoint(doc, group.Children(), x, y); hit != nil {
				return hit
			}
			continue
		}
		if layerHasVisibleAlphaAt(doc, layer, x, y) {
			return layer
		}
	}
	return nil
}

func layerHasVisibleAlphaAt(doc *Document, layer LayerNode, x, y int) bool {
	if doc == nil || layer == nil || x < 0 || x >= doc.Width || y < 0 || y >= doc.Height {
		return false
	}
	surface, err := doc.renderLayerToSurface(layer)
	if err != nil || len(surface) == 0 {
		return false
	}
	index := (y*doc.Width + x) * 4
	return index >= 0 && index+3 < len(surface) && surface[index+3] > 0
}
