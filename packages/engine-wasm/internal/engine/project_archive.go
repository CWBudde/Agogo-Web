package engine

import (
	"encoding/json"
	"fmt"
)

const projectArchiveVersion = 1

type projectArchive struct {
	Version  int                    `json:"version"`
	Document projectDocumentArchive `json:"document"`
	History  []HistoryEntry         `json:"history,omitempty"`
}

type projectDocumentArchive struct {
	Width         int                   `json:"width"`
	Height        int                   `json:"height"`
	Resolution    float64               `json:"resolution"`
	ColorMode     string                `json:"colorMode"`
	BitDepth      int                   `json:"bitDepth"`
	Background    Background            `json:"background"`
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	CreatedAt     string                `json:"createdAt"`
	CreatedBy     string                `json:"createdBy"`
	ModifiedAt    string                `json:"modifiedAt"`
	ActiveLayer   string                `json:"activeLayerId,omitempty"`
	Layers        []projectLayerArchive `json:"layers"`
	Paths         []NamedPath           `json:"paths,omitempty"`
	ActivePathIdx int                   `json:"activePathIdx,omitempty"`
	StylePresets  []DocumentStylePreset `json:"stylePresets,omitempty"`
}

type projectLayerArchive struct {
	ID                string                `json:"id"`
	LayerType         LayerType             `json:"layerType"`
	Name              string                `json:"name"`
	Visible           bool                  `json:"visible"`
	LockMode          LayerLockMode         `json:"lockMode"`
	Opacity           float64               `json:"opacity"`
	FillOpacity       float64               `json:"fillOpacity"`
	BlendMode         BlendMode             `json:"blendMode"`
	ClipToBelow       bool                  `json:"clipToBelow"`
	ClippingBase      bool                  `json:"clippingBase"`
	Mask              *LayerMask            `json:"mask,omitempty"`
	VectorMask        *Path                 `json:"vectorMask,omitempty"`
	StyleStack        []LayerStyle          `json:"styleStack,omitempty"`
	BlendIf           *BlendIfConfig        `json:"blendIf,omitempty"`
	Isolated          bool                  `json:"isolated,omitempty"`
	IsArtboard        bool                  `json:"isArtboard,omitempty"`
	ArtboardBounds    *LayerBounds          `json:"artboardBounds,omitempty"`
	ArtboardBG        *[4]uint8             `json:"artboardBackground,omitempty"`
	Bounds            *LayerBounds          `json:"bounds,omitempty"`
	Pixels            []byte                `json:"pixels,omitempty"`
	AdjustmentKind    string                `json:"adjustmentKind,omitempty"`
	Params            json.RawMessage       `json:"params,omitempty"`
	Text              string                `json:"text,omitempty"`
	FontFamily        string                `json:"fontFamily,omitempty"`
	FontStyle         string                `json:"fontStyle,omitempty"`
	FontSize          float64               `json:"fontSize,omitempty"`
	Bold              bool                  `json:"bold,omitempty"`
	Italic            bool                  `json:"italic,omitempty"`
	AntiAlias         string                `json:"antiAlias,omitempty"`
	Color             [4]uint8              `json:"color,omitempty"`
	TextType          string                `json:"textType,omitempty"`
	TextAlignment     string                `json:"textAlignment,omitempty"`
	BaselineShift     float64               `json:"baselineShift,omitempty"`
	TextLeading       float64               `json:"textLeading,omitempty"`
	TextTracking      float64               `json:"textTracking,omitempty"`
	TextKerning       float64               `json:"textKerning,omitempty"`
	TextLanguage      string                `json:"textLanguage,omitempty"`
	TextOrientation   string                `json:"textOrientation,omitempty"`
	TextSuperscript   bool                  `json:"textSuperscript,omitempty"`
	TextSubscript     bool                  `json:"textSubscript,omitempty"`
	TextUnderline     bool                  `json:"textUnderline,omitempty"`
	TextStrikethrough bool                  `json:"textStrikethrough,omitempty"`
	TextAllCaps       bool                  `json:"textAllCaps,omitempty"`
	TextSmallCaps     bool                  `json:"textSmallCaps,omitempty"`
	TextIndentLeft    float64               `json:"textIndentLeft,omitempty"`
	TextIndentRight   float64               `json:"textIndentRight,omitempty"`
	TextIndentFirst   float64               `json:"textIndentFirst,omitempty"`
	TextSpaceBefore   float64               `json:"textSpaceBefore,omitempty"`
	TextSpaceAfter    float64               `json:"textSpaceAfter,omitempty"`
	Shape             *Path                 `json:"shape,omitempty"`
	FillColor         [4]uint8              `json:"fillColor,omitempty"`
	StrokeColor       [4]uint8              `json:"strokeColor,omitempty"`
	StrokeWidth       float64               `json:"strokeWidth,omitempty"`
	CachedRaster      []byte                `json:"cachedRaster,omitempty"`
	Children          []projectLayerArchive `json:"children,omitempty"`
}

// SaveProject serializes a document and layer tree into a portable JSON archive.
func SaveProject(doc *Document, history []HistoryEntry) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	archive := projectArchive{
		Version: projectArchiveVersion,
		Document: projectDocumentArchive{
			Width:         doc.Width,
			Height:        doc.Height,
			Resolution:    doc.Resolution,
			ColorMode:     doc.ColorMode,
			BitDepth:      doc.BitDepth,
			Background:    doc.Background,
			ID:            doc.ID,
			Name:          doc.Name,
			CreatedAt:     doc.CreatedAt,
			CreatedBy:     doc.CreatedBy,
			ModifiedAt:    doc.ModifiedAt,
			ActiveLayer:   doc.ActiveLayerID,
			Layers:        make([]projectLayerArchive, 0),
			Paths:         cloneNamedPaths(doc.Paths),
			ActivePathIdx: doc.ActivePathIdx,
			StylePresets:  cloneDocumentStylePresets(doc.StylePresets),
		},
		History: append([]HistoryEntry(nil), history...),
	}
	if root := doc.ensureLayerRoot(); root != nil {
		children := root.Children()
		archive.Document.Layers = make([]projectLayerArchive, 0, len(children))
		for _, child := range children {
			archive.Document.Layers = append(archive.Document.Layers, buildProjectLayerArchive(child))
		}
	}
	return json.Marshal(archive)
}

// LoadProject deserializes a JSON archive and reconstructs a document tree.
func LoadProject(data []byte) (*Document, []HistoryEntry, error) {
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("project archive is empty")
	}
	var archive projectArchive
	if err := json.Unmarshal(data, &archive); err != nil {
		return nil, nil, fmt.Errorf("decode project archive: %w", err)
	}
	if archive.Version != 0 && archive.Version != projectArchiveVersion {
		return nil, nil, fmt.Errorf("unsupported project archive version %d", archive.Version)
	}
	doc, err := archive.Document.toDocument()
	if err != nil {
		return nil, nil, err
	}
	return doc, append([]HistoryEntry(nil), archive.History...), nil
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
		archive.CachedRaster = append([]byte(nil), typed.CachedRaster...)
	}
	return archive
}

func (archive projectDocumentArchive) toDocument() (*Document, error) {
	doc := &Document{
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
		LayerRoot:     NewGroupLayer("Root"),
		Paths:         cloneNamedPaths(archive.Paths),
		ActivePathIdx: archive.ActivePathIdx,
		StylePresets:  cloneDocumentStylePresets(archive.StylePresets),
	}
	children := make([]LayerNode, 0, len(archive.Layers))
	for _, childArchive := range archive.Layers {
		child, err := childArchive.toLayerNode()
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	doc.LayerRoot.SetChildren(children)
	doc.normalizeClippingState()
	return doc, nil
}

func (archive projectLayerArchive) toLayerNode() (LayerNode, error) {
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
			background := defaultArtboardBackground()
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
		layer = vectorLayer
	default:
		return nil, fmt.Errorf("unsupported layer type %q", archive.LayerType)
	}
	if mutable, ok := layer.(mutableLayerNode); ok {
		mutable.setID(archive.ID)
	}
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
			child, err := childArchive.toLayerNode()
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		group.SetChildren(children)
	}
	return layer, nil
}
