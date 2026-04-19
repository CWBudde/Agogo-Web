package engine

import (
	"fmt"

	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

func pixelNoiseSeed(x, y int) uint32 {
	seed := uint32(x)*73856093 ^ uint32(y)*19349663 ^ 0x9e3779b9
	seed ^= seed >> 16
	return seed
}

func insertChild(parent *GroupLayer, layer LayerNode, index int) {
	docpkg.InsertChild(parent, layer, index)
}

func replaceChild(parent *GroupLayer, index int, layer LayerNode) {
	docpkg.ReplaceChild(parent, index, layer)
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
	return docpkg.FindLayerByID(group, layerID)
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
		if typed.Artboard != nil {
			typed.Artboard.Bounds.X += dx
			typed.Artboard.Bounds.Y += dy
		}
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
