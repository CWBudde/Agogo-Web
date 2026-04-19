package document

import (
	"encoding/json"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

type LayerNodeMeta struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	LayerType         model.LayerType      `json:"layerType"`
	AdjustmentKind    string               `json:"adjustmentKind,omitempty"`
	Params            json.RawMessage      `json:"params,omitempty"`
	ParentID          string               `json:"parentId,omitempty"`
	Visible           bool                 `json:"visible"`
	LockMode          model.LayerLockMode  `json:"lockMode"`
	Opacity           float64              `json:"opacity"`
	FillOpacity       float64              `json:"fillOpacity"`
	BlendMode         model.BlendMode      `json:"blendMode"`
	ClipToBelow       bool                 `json:"clipToBelow"`
	ClippingBase      bool                 `json:"clippingBase"`
	HasMask           bool                 `json:"hasMask"`
	MaskEnabled       bool                 `json:"maskEnabled"`
	HasVectorMask     bool                 `json:"hasVectorMask"`
	StyleStack        []model.LayerStyle   `json:"styleStack,omitempty"`
	BlendIf           *model.BlendIfConfig `json:"blendIf,omitempty"`
	Isolated          bool                 `json:"isolated,omitempty"`
	IsArtboard        bool                 `json:"isArtboard,omitempty"`
	ArtboardBounds    *model.LayerBounds   `json:"artboardBounds,omitempty"`
	ArtboardBG        *[4]uint8            `json:"artboardBackground,omitempty"`
	Children          []LayerNodeMeta      `json:"children,omitempty"`
	VecFillColor      *[4]uint8            `json:"fillColor,omitempty"`
	VecStrokeColor    *[4]uint8            `json:"strokeColor,omitempty"`
	VecStrokeWidth    *float64             `json:"strokeWidth,omitempty"`
	TextContent       *string              `json:"text,omitempty"`
	TextFontFamily    *string              `json:"fontFamily,omitempty"`
	TextFontStyle     *string              `json:"fontStyle,omitempty"`
	TextFontSize      *float64             `json:"fontSize,omitempty"`
	TextAntiAlias     *string              `json:"antiAlias,omitempty"`
	TextColor         *[4]uint8            `json:"textColor,omitempty"`
	TextAlignment     *string              `json:"textAlignment,omitempty"`
	TextType          *string              `json:"textType,omitempty"`
	TextBaselineShift *float64             `json:"baselineShift,omitempty"`
	TextBold          *bool                `json:"bold,omitempty"`
	TextItalic        *bool                `json:"italic,omitempty"`
	TextTracking      *float64             `json:"tracking,omitempty"`
	TextKerning       *float64             `json:"kerning,omitempty"`
	TextLanguage      *string              `json:"language,omitempty"`
	TextLeading       *float64             `json:"leading,omitempty"`
	TextOrientation   *string              `json:"orientation,omitempty"`
	TextSuperscript   *bool                `json:"superscript,omitempty"`
	TextSubscript     *bool                `json:"subscript,omitempty"`
	TextUnderline     *bool                `json:"underline,omitempty"`
	TextStrikethrough *bool                `json:"strikethrough,omitempty"`
	TextAllCaps       *bool                `json:"allCaps,omitempty"`
	TextSmallCaps     *bool                `json:"smallCaps,omitempty"`
	TextIndentLeft    *float64             `json:"indentLeft,omitempty"`
	TextIndentRight   *float64             `json:"indentRight,omitempty"`
	TextIndentFirst   *float64             `json:"indentFirst,omitempty"`
	TextSpaceBefore   *float64             `json:"spaceBefore,omitempty"`
	TextSpaceAfter    *float64             `json:"spaceAfter,omitempty"`
}

func BuildLayerMeta(layers []model.LayerNode) []LayerNodeMeta {
	if len(layers) == 0 {
		return nil
	}
	meta := make([]LayerNodeMeta, 0, len(layers))
	for _, child := range layers {
		meta = append(meta, BuildLayerNodeMeta(child))
	}
	return meta
}

func BuildLayerNodeMeta(layer model.LayerNode) LayerNodeMeta {
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
		StyleStack:    model.CloneLayerStyles(layer.StyleStack()),
	}
	if adjustment, ok := layer.(*model.AdjustmentLayer); ok {
		meta.AdjustmentKind = adjustment.AdjustmentKind
		meta.Params = model.CloneJSONRawMessage(adjustment.Params)
	}
	if parent := layer.Parent(); parent != nil {
		meta.ParentID = parent.ID()
	}
	if group, ok := layer.(*model.GroupLayer); ok {
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
			meta.Children = append(meta.Children, BuildLayerNodeMeta(child))
		}
	}
	if vl, ok := layer.(*model.VectorLayer); ok {
		fc := vl.FillColor
		sc := vl.StrokeColor
		sw := vl.StrokeWidth
		meta.VecFillColor = &fc
		meta.VecStrokeColor = &sc
		meta.VecStrokeWidth = &sw
		meta.BlendIf = layer.BlendIf()
	}
	if _, ok := layer.(*model.PixelLayer); ok {
		meta.BlendIf = layer.BlendIf()
	}
	if tl, ok := layer.(*model.TextLayer); ok {
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
