package engine

import (
	"encoding/json"
	"fmt"
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

type LayerNodeMeta struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	LayerType     LayerType       `json:"layerType"`
	ParentID      string          `json:"parentId,omitempty"`
	Visible       bool            `json:"visible"`
	LockMode      LayerLockMode   `json:"lockMode"`
	Opacity       float64         `json:"opacity"`
	FillOpacity   float64         `json:"fillOpacity"`
	BlendMode     BlendMode       `json:"blendMode"`
	ClippingBase  bool            `json:"clippingBase"`
	HasMask       bool            `json:"hasMask"`
	HasVectorMask bool            `json:"hasVectorMask"`
	Isolated      bool            `json:"isolated,omitempty"`
	Children      []LayerNodeMeta `json:"children,omitempty"`
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

func (doc *Document) SetLayerLock(layerID string, mode LayerLockMode) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetLockMode(mode)
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
	visible := make([]LayerNode, 0, len(children))
	hidden := make([]LayerNode, 0, len(children))
	for _, child := range children {
		if child.Visible() {
			visible = append(visible, child)
			continue
		}
		hidden = append(hidden, child)
	}
	if len(visible) == 0 {
		return fmt.Errorf("no visible layers to merge")
	}
	merged, err := doc.mergeVisibleNodes(visible)
	if err != nil {
		return err
	}
	hidden = append(hidden, merged)
	root.SetChildren(hidden)
	doc.ActiveLayerID = merged.ID()
	doc.touchModifiedAt()
	return nil
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
	for candidate := deletedIndex; candidate < len(children); candidate++ {
		if candidate == deletedIndex {
			continue
		}
		return children[candidate].ID()
	}
	if deletedIndex > 0 {
		return children[deletedIndex-1].ID()
	}
	if parent := deleted.Parent(); parent != nil && parent.Parent() != nil {
		return parent.ID()
	}
	return ""
}

func (doc *Document) touchModifiedAt() {
	doc.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
}

func (doc *Document) newLayerFromPayload(payload AddLayerPayload) (LayerNode, error) {
	switch payload.LayerType {
	case LayerTypePixel:
		return NewPixelLayer(payload.Name, payload.Bounds, payload.Pixels), nil
	case LayerTypeGroup:
		group := NewGroupLayer(payload.Name)
		group.Isolated = payload.Isolated
		return group, nil
	case LayerTypeAdjustment:
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
	buffer := make([]byte, doc.Width*doc.Height*4)
	if err := doc.compositeLayerOnto(buffer, bottom); err != nil {
		return nil, err
	}
	if err := doc.compositeLayerOnto(buffer, top); err != nil {
		return nil, err
	}
	return NewPixelLayer(name, LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, buffer), nil
}

func (doc *Document) mergeVisibleNodes(nodes []LayerNode) (*PixelLayer, error) {
	buffer := make([]byte, doc.Width*doc.Height*4)
	for _, node := range nodes {
		if err := doc.compositeLayerOnto(buffer, node); err != nil {
			return nil, err
		}
	}
	return NewPixelLayer("Merged Visible", LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, buffer), nil
}

func (doc *Document) renderLayerToSurface(layer LayerNode) ([]byte, error) {
	buffer := make([]byte, doc.Width*doc.Height*4)
	if err := doc.compositeLayerOnto(buffer, layer); err != nil {
		return nil, err
	}
	return buffer, nil
}

func (doc *Document) compositeLayerOnto(dest []byte, layer LayerNode) error {
	if layer == nil || !layer.Visible() {
		return nil
	}
	if err := ensureRasterizableLayer(layer); err != nil {
		return err
	}
	switch typed := layer.(type) {
	case *PixelLayer:
		return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.Pixels, typed.BlendMode(), effectiveLayerOpacity(typed), typed.Mask())
	case *TextLayer:
		return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.CachedRaster, typed.BlendMode(), effectiveLayerOpacity(typed), typed.Mask())
	case *VectorLayer:
		return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.CachedRaster, typed.BlendMode(), effectiveLayerOpacity(typed), typed.Mask())
	case *AdjustmentLayer:
		return fmt.Errorf("adjustment layer %q cannot be flattened before compositing is implemented", typed.Name())
	case *GroupLayer:
		if !typed.Isolated && typed.BlendMode() == BlendModeNormal && effectiveLayerOpacity(typed) >= 1 && typed.Mask() == nil {
			for _, child := range typed.Children() {
				if err := doc.compositeLayerOnto(dest, child); err != nil {
					return err
				}
			}
			return nil
		}
		temp := make([]byte, len(dest))
		for _, child := range typed.Children() {
			if err := doc.compositeLayerOnto(temp, child); err != nil {
				return err
			}
		}
		applyLayerMaskToSurface(temp, doc.Width, doc.Height, typed.Mask())
		compositeDocumentSurface(dest, temp, typed.BlendMode(), effectiveLayerOpacity(typed))
		return nil
	default:
		return fmt.Errorf("unsupported layer type %T", layer)
	}
}

func ensureRasterizableLayer(layer LayerNode) error {
	if layer.VectorMask() != nil {
		return fmt.Errorf("layer %q cannot be merged while vector masks are not implemented", layer.Name())
	}
	if layer.ClippingBase() {
		return fmt.Errorf("layer %q cannot be merged while clipping masks are not implemented", layer.Name())
	}
	if len(layer.StyleStack()) > 0 {
		return fmt.Errorf("layer %q cannot be merged while layer styles are not rasterized", layer.Name())
	}
	return nil
}

func effectiveLayerOpacity(layer LayerNode) float64 {
	return clampUnit(layer.Opacity() * layer.FillOpacity())
}

func compositeRasterIntoDocument(dest []byte, docW, docH int, bounds LayerBounds, src []byte, blendMode BlendMode, opacity float64, mask *LayerMask) error {
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
	for docY := 0; docY < docH; docY++ {
		for docX := 0; docX < docW; docX++ {
			maskAlpha := layerMaskAlphaAt(mask, docX, docY)
			if maskAlpha == 255 {
				continue
			}
			index := (docY*docW + docX) * 4
			surface[index+3] = scaleMaskedAlpha(surface[index+3], maskAlpha)
		}
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
	meta := LayerNodeMeta{
		ID:            layer.ID(),
		Name:          layer.Name(),
		LayerType:     layer.LayerType(),
		Visible:       layer.Visible(),
		LockMode:      layer.LockMode(),
		Opacity:       layer.Opacity(),
		FillOpacity:   layer.FillOpacity(),
		BlendMode:     layer.BlendMode(),
		ClippingBase:  layer.ClippingBase(),
		HasMask:       layer.Mask() != nil,
		HasVectorMask: layer.VectorMask() != nil,
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
