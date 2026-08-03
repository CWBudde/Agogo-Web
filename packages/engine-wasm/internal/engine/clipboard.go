package engine

import (
	"fmt"

	agglib "github.com/cwbudde/agg_go"
)

// pixelClipboard is runtime-only application state. Pixels are straight RGBA
// in a tight selection/canvas rectangle; Bounds remain in document space so a
// same-document paste lands at the copied position.
type pixelClipboard struct {
	Pixels           []byte
	Bounds           LayerBounds
	SourceDocumentID string
	SourceLayerName  string
}

func (clipboard pixelClipboard) valid() bool {
	return clipboard.Bounds.W > 0 && clipboard.Bounds.H > 0 && len(clipboard.Pixels) == clipboard.Bounds.W*clipboard.Bounds.H*4
}

func (inst *instance) copyPixels() error {
	doc := inst.manager.Active()
	clipboard, err := buildPixelClipboard(doc)
	if err != nil {
		return err
	}
	inst.pixelClipboard = clipboard
	return nil
}

func (inst *instance) cutPixels() error {
	var nextClipboard pixelClipboard
	err := inst.executeDocCommand("Cut", func(doc *Document) error {
		layer := findPixelLayer(doc, doc.ActiveLayerID)
		if layer == nil {
			return fmt.Errorf("active layer must be a pixel layer to cut")
		}
		if err := ensureLayerEditable(layer, editLayerPixels); err != nil {
			return err
		}
		clipboard, err := buildPixelClipboard(doc)
		if err != nil {
			return err
		}
		changed, err := clearPixelLayerSelection(layer, doc.Selection)
		if err != nil {
			return err
		}
		if changed {
			doc.touchModifiedAtLayer(layer)
		}
		nextClipboard = clipboard
		return nil
	})
	if err != nil {
		return err
	}
	inst.pixelClipboard = nextClipboard
	return nil
}

func (inst *instance) pastePixels() error {
	if !inst.pixelClipboard.valid() {
		return fmt.Errorf("clipboard is empty")
	}
	clipboard := inst.pixelClipboard
	return inst.executeDocCommand("Paste", func(doc *Document) error {
		bounds := clipboard.Bounds
		if clipboard.SourceDocumentID != doc.ID {
			bounds.X = (doc.Width - bounds.W) / 2
			bounds.Y = (doc.Height - bounds.H) / 2
		}
		name := "Pasted Layer"
		if clipboard.SourceLayerName != "" {
			name = clipboard.SourceLayerName + " copy"
		}
		layer := NewPixelLayer(name, bounds, append([]byte(nil), clipboard.Pixels...))
		parent := doc.ensureLayerRoot()
		index := len(parent.Children())
		if doc.ActiveLayerID != "" {
			if _, activeParent, activeIndex, ok := findLayerByID(parent, doc.ActiveLayerID); ok && activeParent != nil {
				parent = activeParent
				index = activeIndex + 1
			}
		}
		insertChild(parent, layer, index)
		doc.ActiveLayerID = layer.ID()
		doc.normalizeClippingState()
		doc.touchModifiedAtLayer(layer)
		return nil
	})
}

func buildPixelClipboard(doc *Document) (pixelClipboard, error) {
	if doc == nil {
		return pixelClipboard{}, fmt.Errorf("no active document")
	}
	layer := doc.ActiveLayer()
	if layer == nil {
		return pixelClipboard{}, fmt.Errorf("no active layer")
	}
	if _, ok := layer.(*AdjustmentLayer); ok {
		return pixelClipboard{}, fmt.Errorf("adjustment layers cannot be copied without a backdrop")
	}
	wasVisible := layer.Visible()
	layer.SetVisible(true)
	surface, err := doc.renderLayerToSurface(layer)
	layer.SetVisible(wasVisible)
	if err != nil {
		return pixelClipboard{}, err
	}

	bounds := LayerBounds{W: doc.Width, H: doc.Height}
	pixels := surface
	if selection := normalizeSelection(cloneSelection(doc.Selection)); selection != nil {
		var ok bool
		pixels, bounds, ok = extractSelectionFromSurface(surface, doc.Width, doc.Height, selection)
		if !ok {
			return pixelClipboard{}, fmt.Errorf("selection contains no copyable area")
		}
	}

	return pixelClipboard{
		Pixels:           pixels,
		Bounds:           bounds,
		SourceDocumentID: doc.ID,
		SourceLayerName:  layer.Name(),
	}, nil
}

func clearPixelLayerSelection(layer *PixelLayer, selection *Selection) (bool, error) {
	if layer == nil {
		return false, nil
	}
	// Build an RGBA coverage image from the selection and let AGG's
	// Porter-Duff destination-out compositor perform the destructive erase.
	// RGB equals alpha so the source is valid premultiplied RGBA; DstOut only
	// consumes source alpha.
	maskPixels := make([]byte, layer.Bounds.W*layer.Bounds.H*4)
	changed := false
	for y := range layer.Bounds.H {
		for x := range layer.Bounds.W {
			coverage := selectionCoverageAt(selection, layer.Bounds.X+x, layer.Bounds.Y+y)
			if coverage == 0 {
				continue
			}
			index := (y*layer.Bounds.W + x) * 4
			maskPixels[index] = coverage
			maskPixels[index+1] = coverage
			maskPixels[index+2] = coverage
			maskPixels[index+3] = coverage
			if layer.Pixels[index+3] != 0 {
				changed = true
			}
		}
	}
	if !changed {
		return false, nil
	}

	originalPixels := append([]byte(nil), layer.Pixels...)
	renderer := agglib.NewAgg2D()
	destination := agglib.NewImage(layer.Pixels, layer.Bounds.W, layer.Bounds.H, layer.Bounds.W*4)
	if err := destination.Premultiply(); err != nil {
		return false, fmt.Errorf("premultiply cut destination: %w", err)
	}
	renderer.AttachImage(destination)
	renderer.BlendMode(agglib.BlendDstOut)
	mask := agglib.NewImage(maskPixels, layer.Bounds.W, layer.Bounds.H, layer.Bounds.W*4)
	blendErr := renderer.BlendImageSimpleDefaultAlpha(mask, 0, 0)
	demultiplyErr := destination.Demultiply()
	if blendErr != nil {
		return false, fmt.Errorf("erase cut selection: %w", blendErr)
	}
	if demultiplyErr != nil {
		return false, fmt.Errorf("demultiply cut destination: %w", demultiplyErr)
	}
	// DstOut only changes alpha. Preserve the straight-alpha layer's hidden RGB
	// exactly; an 8-bit premultiply/demultiply round trip is otherwise lossy.
	for index := 0; index < len(layer.Pixels); index += 4 {
		copy(layer.Pixels[index:index+3], originalPixels[index:index+3])
	}
	return true, nil
}
