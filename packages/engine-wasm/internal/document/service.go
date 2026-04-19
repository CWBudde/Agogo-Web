package document

import (
	"fmt"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func EnsureLayerRoot(root *model.GroupLayer) *model.GroupLayer {
	if root != nil {
		return root
	}
	root = model.NewGroupLayer("Root")
	root.SetName("Root")
	root.SetParent(nil)
	return root
}

func Layers(root *model.GroupLayer) []model.LayerNode {
	return EnsureLayerRoot(root).Children()
}

func ActiveLayer(root *model.GroupLayer, activeLayerID string) model.LayerNode {
	if activeLayerID == "" {
		return nil
	}
	layer, _, _, ok := FindLayerByID(EnsureLayerRoot(root), activeLayerID)
	if !ok {
		return nil
	}
	return layer
}

func FindLayerByID(group *model.GroupLayer, layerID string) (model.LayerNode, *model.GroupLayer, int, bool) {
	children := group.Children()
	for index, child := range children {
		if child.ID() == layerID {
			return child, group, index, true
		}
		if nestedGroup, ok := child.(*model.GroupLayer); ok {
			if layer, parent, childIndex, found := FindLayerByID(nestedGroup, layerID); found {
				return layer, parent, childIndex, true
			}
		}
	}
	return nil, nil, -1, false
}

func ContainsLayerID(layer model.LayerNode, targetID string) bool {
	if layer == nil || targetID == "" {
		return false
	}
	if layer.ID() == targetID {
		return true
	}
	for _, child := range layer.Children() {
		if ContainsLayerID(child, targetID) {
			return true
		}
	}
	return false
}

func GroupForID(root *model.GroupLayer, groupID string) (*model.GroupLayer, error) {
	root = EnsureLayerRoot(root)
	if groupID == "" || groupID == root.ID() {
		return root, nil
	}
	layer, _, _, ok := FindLayerByID(root, groupID)
	if !ok {
		return nil, fmt.Errorf("parent layer %q not found", groupID)
	}
	group, ok := layer.(*model.GroupLayer)
	if !ok {
		return nil, fmt.Errorf("layer %q is not a group", groupID)
	}
	return group, nil
}

func InsertChild(parent *model.GroupLayer, layer model.LayerNode, index int) {
	children := parent.Children()
	if index < 0 || index > len(children) {
		index = len(children)
	}
	updated := make([]model.LayerNode, 0, len(children)+1)
	updated = append(updated, children[:index]...)
	updated = append(updated, layer)
	updated = append(updated, children[index:]...)
	parent.SetChildren(updated)
}

func ReplaceChild(parent *model.GroupLayer, index int, layer model.LayerNode) {
	children := parent.Children()
	children[index] = layer
	parent.SetChildren(children)
}

func NextActiveLayerID(children []model.LayerNode, deletedIndex int, deleted model.LayerNode) string {
	if deletedIndex+1 < len(children) {
		return children[deletedIndex+1].ID()
	}
	if deletedIndex > 0 {
		return children[deletedIndex-1].ID()
	}
	if parent := deleted.Parent(); parent != nil && parent.Parent() != nil {
		return parent.ID()
	}
	return ""
}

func AddLayer(root *model.GroupLayer, layer model.LayerNode, parentLayerID string, index int) error {
	if layer == nil {
		return fmt.Errorf("layer is required")
	}
	parent, err := GroupForID(root, parentLayerID)
	if err != nil {
		return err
	}
	InsertChild(parent, layer, index)
	return nil
}

func DeleteLayer(root *model.GroupLayer, layerID, activeLayerID string) (deleted model.LayerNode, nextActiveLayerID string, activeChanged bool, err error) {
	layer, parent, index, ok := FindLayerByID(EnsureLayerRoot(root), layerID)
	if !ok || parent == nil {
		return nil, "", false, fmt.Errorf("layer %q not found", layerID)
	}
	children := parent.Children()
	nextActive := NextActiveLayerID(children, index, layer)
	children = append(children[:index], children[index+1:]...)
	parent.SetChildren(children)
	return layer, nextActive, ContainsLayerID(layer, activeLayerID), nil
}

func DuplicateLayer(root *model.GroupLayer, layerID, parentLayerID string, index int) (model.LayerNode, error) {
	source, parent, sourceIndex, ok := FindLayerByID(EnsureLayerRoot(root), layerID)
	if !ok {
		return nil, fmt.Errorf("layer %q not found", layerID)
	}
	clone := model.CloneLayerForDuplicate(source)
	targetParent := parent
	if parentLayerID != "" {
		var err error
		targetParent, err = GroupForID(root, parentLayerID)
		if err != nil {
			return nil, err
		}
	}
	if targetParent == parent && index < 0 {
		index = sourceIndex + 1
	}
	InsertChild(targetParent, clone, index)
	return clone, nil
}

func MoveLayer(root *model.GroupLayer, layerID, parentLayerID string, index int) error {
	layer, currentParent, currentIndex, ok := FindLayerByID(EnsureLayerRoot(root), layerID)
	if !ok || currentParent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	targetParent, err := GroupForID(root, parentLayerID)
	if err != nil {
		return err
	}
	if ContainsLayerID(layer, targetParent.ID()) {
		return fmt.Errorf("cannot move layer into its own descendant")
	}
	currentChildren := currentParent.Children()
	currentChildren = append(currentChildren[:currentIndex], currentChildren[currentIndex+1:]...)
	currentParent.SetChildren(currentChildren)
	if targetParent == currentParent && index > currentIndex {
		index--
	}
	InsertChild(targetParent, layer, index)
	return nil
}

func FullDocumentDirtyRect(docW, docH int) DirtyRect {
	if docW <= 0 || docH <= 0 {
		return DirtyRect{}
	}
	return DirtyRect{X: 0, Y: 0, W: docW, H: docH}
}

func MarkDirtyCompositeRect(current DirtyRect, hasCurrent bool, rect DirtyRect, docW, docH int) (DirtyRect, bool) {
	if docW <= 0 || docH <= 0 {
		return DirtyRect{}, false
	}
	normalized, err := normalizeDirtyRect(rect, docW, docH)
	if err != nil {
		return current, hasCurrent
	}
	if !hasCurrent {
		return normalized, true
	}
	return UnionDirtyRects(current, normalized), true
}

func CurrentDirtyCompositeRect(current DirtyRect, hasCurrent bool) (DirtyRect, bool) {
	if !hasCurrent {
		return DirtyRect{}, false
	}
	return current, true
}

func UnionDirtyRects(a, b DirtyRect) DirtyRect {
	x1 := minInt(a.X, b.X)
	y1 := minInt(a.Y, b.Y)
	x2 := maxInt(a.X+a.W, b.X+b.W)
	y2 := maxInt(a.Y+a.H, b.Y+b.H)
	return DirtyRect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

func LayerBoundsDirtyRect(bounds model.LayerBounds, docW, docH int) (DirtyRect, bool) {
	if docW <= 0 || docH <= 0 || bounds.W <= 0 || bounds.H <= 0 {
		return DirtyRect{}, false
	}
	rect := DirtyRect(bounds)
	normalized, err := normalizeDirtyRect(rect, docW, docH)
	if err != nil {
		return DirtyRect{}, false
	}
	return normalized, true
}

func DefaultArtboardBackground() [4]uint8 {
	return [4]uint8{255, 255, 255, 255}
}

func LayerCompositeDirtyRect(
	layer model.LayerNode,
	docW,
	docH int,
	hasEnabledStyle func([]model.LayerStyle) bool,
) (DirtyRect, bool) {
	if layer == nil {
		return DirtyRect{}, false
	}
	if _, ok := layer.(*model.AdjustmentLayer); ok {
		rect := FullDocumentDirtyRect(docW, docH)
		return rect, rect.W > 0 && rect.H > 0
	}
	if hasEnabledStyle != nil && hasEnabledStyle(layer.StyleStack()) {
		rect := FullDocumentDirtyRect(docW, docH)
		return rect, rect.W > 0 && rect.H > 0
	}

	switch typed := layer.(type) {
	case *model.PixelLayer:
		return LayerBoundsDirtyRect(typed.Bounds, docW, docH)
	case *model.TextLayer:
		return LayerBoundsDirtyRect(typed.Bounds, docW, docH)
	case *model.VectorLayer:
		return LayerBoundsDirtyRect(typed.Bounds, docW, docH)
	case *model.GroupLayer:
		var combined DirtyRect
		var hasCombined bool
		if typed.Artboard != nil {
			if rect, ok := LayerBoundsDirtyRect(typed.Artboard.Bounds, docW, docH); ok {
				combined = rect
				hasCombined = true
			}
		}
		for _, child := range typed.Children() {
			rect, ok := LayerCompositeDirtyRect(child, docW, docH, hasEnabledStyle)
			if !ok {
				continue
			}
			if !hasCombined {
				combined = rect
				hasCombined = true
				continue
			}
			combined = UnionDirtyRects(combined, rect)
		}
		return combined, hasCombined
	default:
		return DirtyRect{}, false
	}
}

func CombineLayerCompositeDirtyRects(
	layers []model.LayerNode,
	docW,
	docH int,
	hasEnabledStyle func([]model.LayerStyle) bool,
) (DirtyRect, bool) {
	var combined DirtyRect
	var hasCombined bool
	for _, layer := range layers {
		rect, ok := LayerCompositeDirtyRect(layer, docW, docH, hasEnabledStyle)
		if !ok {
			continue
		}
		if !hasCombined {
			combined = rect
			hasCombined = true
			continue
		}
		combined = UnionDirtyRects(combined, rect)
	}
	return combined, hasCombined
}

func DirtyRectForBounds(before, after model.LayerBounds, docW, docH int) (DirtyRect, bool) {
	beforeRect, hasBefore := LayerBoundsDirtyRect(before, docW, docH)
	afterRect, hasAfter := LayerBoundsDirtyRect(after, docW, docH)
	switch {
	case hasBefore && hasAfter:
		return UnionDirtyRects(beforeRect, afterRect), true
	case hasBefore:
		return beforeRect, true
	case hasAfter:
		return afterRect, true
	default:
		return DirtyRect{}, false
	}
}

func SetArtboard(root *model.GroupLayer, layerID string, bounds model.LayerBounds, background *[4]uint8) error {
	if bounds.W <= 0 || bounds.H <= 0 {
		return fmt.Errorf("artboard requires positive bounds, got %dx%d", bounds.W, bounds.H)
	}
	layer, _, _, ok := FindLayerByID(EnsureLayerRoot(root), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	group, ok := layer.(*model.GroupLayer)
	if !ok {
		return fmt.Errorf("layer %q is not a group", layerID)
	}
	artboard := model.CloneArtboard(group.Artboard)
	if artboard == nil {
		artboard = &model.ArtboardData{Background: DefaultArtboardBackground()}
	}
	artboard.Bounds = bounds
	if background != nil {
		artboard.Background = *background
	}
	group.Artboard = artboard
	return nil
}

func normalizeDirtyRect(rect DirtyRect, width, height int) (DirtyRect, error) {
	if width <= 0 || height <= 0 {
		return DirtyRect{}, fmt.Errorf("invalid surface dimensions %dx%d", width, height)
	}
	if rect.W <= 0 || rect.H <= 0 {
		return DirtyRect{}, fmt.Errorf("dirty rect must be positive, got %v", rect)
	}

	x1 := clampInt(rect.X, 0, width)
	y1 := clampInt(rect.Y, 0, height)
	x2 := clampInt(rect.X+rect.W, 0, width)
	y2 := clampInt(rect.Y+rect.H, 0, height)
	if x2 <= x1 || y2 <= y1 {
		return DirtyRect{}, fmt.Errorf("dirty rect outside surface bounds: %v", rect)
	}

	return DirtyRect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}, nil
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
