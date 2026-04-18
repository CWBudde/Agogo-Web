package engine

import "fmt"

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
		StyleStack:    cloneLayerStyles(layer.StyleStack()),
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
		if group.Artboard != nil {
			bounds := group.Artboard.Bounds
			background := group.Artboard.Background
			meta.IsArtboard = true
			meta.ArtboardBounds = &bounds
			meta.ArtboardBG = &background
		}
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
		meta.BlendIf = layer.BlendIf()
	}
	if _, ok := layer.(*PixelLayer); ok {
		meta.BlendIf = layer.BlendIf()
	}
	if tl, ok := layer.(*TextLayer); ok {
		text := tl.Text
		family := tl.FontFamily
		fontStyle := tl.FontStyle
		size := tl.FontSize
		antiAlias := tl.AntiAlias
		color := tl.Color
		alignment := tl.Alignment
		textType := tl.TextType
		baselineShift := tl.BaselineShift
		bold := tl.Bold
		italic := tl.Italic
		tracking := tl.Tracking
		kerning := tl.Kerning
		language := tl.Language
		leading := tl.Leading
		orientation := tl.Orientation
		superscript := tl.Superscript
		subscript := tl.Subscript
		underline := tl.Underline
		strikethrough := tl.Strikethrough
		allCaps := tl.AllCaps
		smallCaps := tl.SmallCaps
		indentLeft := tl.IndentLeft
		indentRight := tl.IndentRight
		indentFirst := tl.IndentFirst
		spaceBefore := tl.SpaceBefore
		spaceAfter := tl.SpaceAfter
		meta.TextContent = &text
		meta.TextFontFamily = &family
		meta.TextFontStyle = &fontStyle
		meta.TextFontSize = &size
		meta.TextAntiAlias = &antiAlias
		meta.TextColor = &color
		meta.TextAlignment = &alignment
		meta.TextType = &textType
		meta.TextBaselineShift = &baselineShift
		meta.TextBold = &bold
		meta.TextItalic = &italic
		meta.TextTracking = &tracking
		meta.TextKerning = &kerning
		meta.TextLanguage = &language
		meta.TextLeading = &leading
		meta.TextOrientation = &orientation
		meta.TextSuperscript = &superscript
		meta.TextSubscript = &subscript
		meta.TextUnderline = &underline
		meta.TextStrikethrough = &strikethrough
		meta.TextAllCaps = &allCaps
		meta.TextSmallCaps = &smallCaps
		meta.TextIndentLeft = &indentLeft
		meta.TextIndentRight = &indentRight
		meta.TextIndentFirst = &indentFirst
		meta.TextSpaceBefore = &spaceBefore
		meta.TextSpaceAfter = &spaceAfter
		meta.BlendIf = layer.BlendIf()
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
