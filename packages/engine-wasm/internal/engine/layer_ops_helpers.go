package engine

import (
	"fmt"

	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

// pixelNoiseSeed returns a deterministic, well-mixed noise seed for a document
// pixel coordinate (dissolve blending). It packs (x, y) into a 64-bit word and
// applies the splitmix64 finalizer, which distributes seeds near-uniformly
// over the full uint32 range. The previous multiply-xor hash always set bit 31
// for small coordinates, so 50%-opacity dissolve left the area near the
// document origin untouched.
func pixelNoiseSeed(x, y int) uint32 {
	z := uint64(uint32(x))<<32 | uint64(uint32(y))
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	return uint32(z)
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

// layerEditKind classifies what a command wants to do to a layer so lock
// enforcement can decide whether the layer's lock mode forbids it.
type layerEditKind int

const (
	// editLayerPixels covers destructive pixel edits: painting, erasing,
	// fills, gradients, filters, and baking a mask into the raster.
	editLayerPixels layerEditKind = iota
	// editLayerPosition covers moving the layer.
	editLayerPosition
)

// ensureLayerEditable is the single lock check for layer edits: it returns a
// Photoshop-style error when the layer's lock mode forbids the given edit
// kind, and nil otherwise. The messages are part of the ABI surface — the
// frontend shows them verbatim — so keep them stable:
//
//	layer "X" is locked            (lock mode "all", any edit)
//	layer "X" pixels are locked    (lock mode "pixels", pixel edits)
//	layer "X" position is locked   (lock mode "position", moves)
//
// Callers must reject the command before mutating anything so a locked layer
// is a strict no-op (no history entry, no content-version bump).
func ensureLayerEditable(layer LayerNode, kind layerEditKind) error {
	if layer == nil {
		return nil
	}
	switch layer.LockMode() {
	case LayerLockAll:
		return fmt.Errorf("layer %q is locked", layer.Name())
	case LayerLockPixels:
		if kind == editLayerPixels {
			return fmt.Errorf("layer %q pixels are locked", layer.Name())
		}
	case LayerLockPosition:
		if kind == editLayerPosition {
			return fmt.Errorf("layer %q position is locked", layer.Name())
		}
	}
	return nil
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
