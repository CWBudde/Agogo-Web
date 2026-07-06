package engine

import (
	"fmt"

	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
	projectio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/project"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

type projectDocumentArchive = ProjectDocumentArchive

type projectLayerArchive = ProjectLayerArchive

// SaveProject serializes a document and layer tree into a portable JSON archive.
func SaveProject(doc *Document, history []HistoryEntry) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	archive := projectDocumentArchive{
		Width:           doc.Width,
		Height:          doc.Height,
		Resolution:      doc.Resolution,
		ColorMode:       doc.ColorMode,
		BitDepth:        doc.BitDepth,
		Background:      doc.Background,
		ID:              doc.ID,
		Name:            doc.Name,
		CreatedAt:       doc.CreatedAt,
		CreatedBy:       doc.CreatedBy,
		ModifiedAt:      doc.ModifiedAt,
		ActiveLayer:     doc.ActiveLayerID,
		Layers:          make([]projectLayerArchive, 0),
		Paths:           cloneNamedPaths(doc.Paths),
		ActivePathIdx:   doc.ActivePathIdx,
		SavedSelections: cloneSavedSelectionChannels(doc.SavedSelections),
		StylePresets:    cloneDocumentStylePresets(doc.StylePresets),
		Patterns:        model.ClonePatterns(doc.Patterns),
	}
	if root := doc.ensureLayerRoot(); root != nil {
		children := root.Children()
		archive.Layers = make([]projectLayerArchive, 0, len(children))
		for _, child := range children {
			archive.Layers = append(archive.Layers, buildProjectLayerArchive(child))
		}
	}
	return projectio.Save(archive, history)
}

// LoadProject deserializes a JSON archive and reconstructs a document tree.
func LoadProject(data []byte) (*Document, []HistoryEntry, error) {
	archive, history, err := projectio.Load(data)
	if err != nil {
		return nil, nil, err
	}
	doc, err := projectDocumentArchiveToDocument(archive)
	if err != nil {
		return nil, nil, err
	}
	return doc, history, nil
}

func buildProjectLayerArchive(layer LayerNode) projectLayerArchive {
	if layer == nil {
		return projectLayerArchive{}
	}
	archive := projectLayerArchive{
		ID:           layer.ID(),
		LayerType:    layer.LayerType(),
		Name:         layer.Name(),
		Visible:      layer.Visible(),
		LockMode:     layer.LockMode(),
		Opacity:      layer.Opacity(),
		FillOpacity:  layer.FillOpacity(),
		BlendMode:    layer.BlendMode(),
		ClipToBelow:  layer.ClipToBelow(),
		ClippingBase: layer.ClippingBase(),
		Mask:         cloneLayerMask(layer.Mask()),
		VectorMask:   clonePath(layer.VectorMask()),
		StyleStack:   cloneLayerStyles(layer.StyleStack()),
		BlendIf:      layer.BlendIf(),
	}
	if group, ok := layer.(*GroupLayer); ok {
		archive.Isolated = group.Isolated
		if group.Artboard != nil {
			bounds := group.Artboard.Bounds
			background := group.Artboard.Background
			archive.IsArtboard = true
			archive.ArtboardBounds = &bounds
			archive.ArtboardBG = &background
		}
		children := group.Children()
		archive.Children = make([]projectLayerArchive, 0, len(children))
		for _, child := range children {
			archive.Children = append(archive.Children, buildProjectLayerArchive(child))
		}
	}
	switch typed := layer.(type) {
	case *PixelLayer:
		bounds := typed.Bounds
		archive.Bounds = &bounds
		archive.Pixels = append([]byte(nil), typed.Pixels...)
	case *AdjustmentLayer:
		archive.AdjustmentKind = typed.AdjustmentKind
		archive.Params = cloneJSONRawMessage(typed.Params)
	case *TextLayer:
		bounds := typed.Bounds
		archive.Bounds = &bounds
		archive.Text = typed.Text
		archive.TextAnchorX = typed.AnchorX
		archive.TextAnchorY = typed.AnchorY
		archive.TextAnchorSet = typed.AnchorSet
		archive.FontFamily = typed.FontFamily
		archive.FontStyle = typed.FontStyle
		archive.FontSize = typed.FontSize
		archive.Bold = typed.Bold
		archive.Italic = typed.Italic
		archive.AntiAlias = typed.AntiAlias
		archive.Color = typed.Color
		archive.TextType = typed.TextType
		archive.TextAlignment = typed.Alignment
		archive.BaselineShift = typed.BaselineShift
		archive.TextLeading = typed.Leading
		archive.TextTracking = typed.Tracking
		archive.TextKerning = typed.Kerning
		archive.TextLanguage = typed.Language
		archive.TextOrientation = typed.Orientation
		archive.TextSuperscript = typed.Superscript
		archive.TextSubscript = typed.Subscript
		archive.TextUnderline = typed.Underline
		archive.TextStrikethrough = typed.Strikethrough
		archive.TextAllCaps = typed.AllCaps
		archive.TextSmallCaps = typed.SmallCaps
		archive.TextIndentLeft = typed.IndentLeft
		archive.TextIndentRight = typed.IndentRight
		archive.TextIndentFirst = typed.IndentFirst
		archive.TextSpaceBefore = typed.SpaceBefore
		archive.TextSpaceAfter = typed.SpaceAfter
		archive.CachedRaster = append([]byte(nil), typed.CachedRaster...)
	case *VectorLayer:
		bounds := typed.Bounds
		archive.Bounds = &bounds
		archive.Shape = clonePath(typed.Shape)
		archive.FillColor = typed.FillColor
		archive.StrokeColor = typed.StrokeColor
		archive.StrokeWidth = typed.StrokeWidth
		archive.FillRule = typed.FillRule
		archive.CachedRaster = append([]byte(nil), typed.CachedRaster...)
	}
	return archive
}

func projectDocumentArchiveToDocument(archive projectDocumentArchive) (*Document, error) {
	doc := newDocumentWithCore(DocumentCore{
		Width:         archive.Width,
		Height:        archive.Height,
		Resolution:    archive.Resolution,
		ColorMode:     archive.ColorMode,
		BitDepth:      archive.BitDepth,
		Background:    archive.Background,
		ID:            archive.ID,
		Name:          archive.Name,
		CreatedAt:     archive.CreatedAt,
		CreatedBy:     archive.CreatedBy,
		ModifiedAt:    archive.ModifiedAt,
		ActiveLayerID: archive.ActiveLayer,
	})
	doc.Paths = cloneNamedPaths(archive.Paths)
	doc.ActivePathIdx = archive.ActivePathIdx
	doc.SavedSelections = cloneSavedSelectionChannels(archive.SavedSelections)
	doc.StylePresets = cloneDocumentStylePresets(archive.StylePresets)
	doc.Patterns = model.ClonePatterns(archive.Patterns)
	children := make([]LayerNode, 0, len(archive.Layers))
	for _, childArchive := range archive.Layers {
		child, err := projectLayerArchiveToLayerNode(childArchive)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	doc.LayerRoot.SetChildren(children)
	doc.normalizeClippingState()
	return doc, nil
}

func projectLayerArchiveToLayerNode(archive projectLayerArchive) (LayerNode, error) {
	var layer LayerNode
	switch archive.LayerType {
	case LayerTypePixel:
		if archive.Bounds == nil {
			return nil, fmt.Errorf("pixel layer %q missing bounds", archive.Name)
		}
		layer = NewPixelLayer(archive.Name, *archive.Bounds, archive.Pixels)
	case LayerTypeGroup:
		group := NewGroupLayer(archive.Name)
		group.Isolated = archive.Isolated
		if archive.IsArtboard {
			background := docpkg.DefaultArtboardBackground()
			if archive.ArtboardBG != nil {
				background = *archive.ArtboardBG
			}
			if archive.ArtboardBounds == nil {
				return nil, fmt.Errorf("artboard group %q missing bounds", archive.Name)
			}
			group.Artboard = &ArtboardData{
				Bounds:     *archive.ArtboardBounds,
				Background: background,
			}
		}
		layer = group
	case LayerTypeAdjustment:
		layer = NewAdjustmentLayer(archive.Name, archive.AdjustmentKind, archive.Params)
	case LayerTypeText:
		if archive.Bounds == nil {
			return nil, fmt.Errorf("text layer %q missing bounds", archive.Name)
		}
		textLayer := NewTextLayer(archive.Name, *archive.Bounds, archive.Text, archive.CachedRaster)
		// Archives written before the anchor model carry no anchor fields;
		// AnchorSet stays false and the first rasterization derives the
		// anchor from the legacy bounds origin (see rasterizeTextLayer).
		textLayer.AnchorX = archive.TextAnchorX
		textLayer.AnchorY = archive.TextAnchorY
		textLayer.AnchorSet = archive.TextAnchorSet
		textLayer.FontFamily = archive.FontFamily
		if archive.FontStyle != "" {
			textLayer.FontStyle = archive.FontStyle
		}
		if archive.FontSize > 0 {
			textLayer.FontSize = archive.FontSize
		}
		textLayer.Bold = archive.Bold
		textLayer.Italic = archive.Italic
		if archive.AntiAlias != "" {
			textLayer.AntiAlias = archive.AntiAlias
		}
		if archive.Color != [4]uint8{} {
			textLayer.Color = archive.Color
		}
		if archive.TextType != "" {
			textLayer.TextType = archive.TextType
		}
		if archive.TextAlignment != "" {
			textLayer.Alignment = archive.TextAlignment
		}
		textLayer.BaselineShift = archive.BaselineShift
		if archive.TextLeading > 0 {
			textLayer.Leading = archive.TextLeading
		}
		textLayer.Tracking = archive.TextTracking
		textLayer.Kerning = archive.TextKerning
		textLayer.Language = archive.TextLanguage
		if archive.TextOrientation != "" {
			textLayer.Orientation = archive.TextOrientation
		}
		textLayer.Superscript = archive.TextSuperscript
		textLayer.Subscript = archive.TextSubscript
		textLayer.Underline = archive.TextUnderline
		textLayer.Strikethrough = archive.TextStrikethrough
		textLayer.AllCaps = archive.TextAllCaps
		textLayer.SmallCaps = archive.TextSmallCaps
		textLayer.IndentLeft = archive.TextIndentLeft
		textLayer.IndentRight = archive.TextIndentRight
		textLayer.IndentFirst = archive.TextIndentFirst
		textLayer.SpaceBefore = archive.TextSpaceBefore
		textLayer.SpaceAfter = archive.TextSpaceAfter
		layer = textLayer
	case LayerTypeVector:
		if archive.Bounds == nil {
			return nil, fmt.Errorf("vector layer %q missing bounds", archive.Name)
		}
		vectorLayer := NewVectorLayer(archive.Name, *archive.Bounds, archive.Shape, archive.CachedRaster)
		if archive.FillColor != [4]uint8{} {
			vectorLayer.FillColor = archive.FillColor
		}
		if archive.StrokeColor != [4]uint8{} {
			vectorLayer.StrokeColor = archive.StrokeColor
		}
		if archive.StrokeWidth > 0 {
			vectorLayer.StrokeWidth = archive.StrokeWidth
		}
		vectorLayer.FillRule = archive.FillRule
		layer = vectorLayer
	default:
		return nil, fmt.Errorf("unsupported layer type %q", archive.LayerType)
	}
	setLayerID(layer, archive.ID)
	layer.SetVisible(archive.Visible)
	layer.SetLockMode(archive.LockMode)
	layer.SetOpacity(archive.Opacity)
	layer.SetFillOpacity(archive.FillOpacity)
	layer.SetBlendMode(archive.BlendMode)
	layer.SetClipToBelow(archive.ClipToBelow)
	layer.SetClippingBase(archive.ClippingBase)
	layer.SetMask(cloneLayerMask(archive.Mask))
	layer.SetVectorMask(clonePath(archive.VectorMask))
	layer.SetStyleStack(cloneLayerStyles(archive.StyleStack))
	layer.SetBlendIf(archive.BlendIf)
	if group, ok := layer.(*GroupLayer); ok {
		children := make([]LayerNode, 0, len(archive.Children))
		for _, childArchive := range archive.Children {
			child, err := projectLayerArchiveToLayerNode(childArchive)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		group.SetChildren(children)
	}
	return layer, nil
}
