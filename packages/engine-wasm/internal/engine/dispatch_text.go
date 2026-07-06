package engine

import (
	"fmt"
	"sync"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
)

// textEditOriginalMu guards textEditOriginalText.
var textEditOriginalMu sync.Mutex

// textEditOriginalText remembers, for each instance with an in-flight text
// edit, what the layer's text was at the moment editing began (keyed by the
// owning instance). textEditInput intentionally mutates the live document
// directly so keystrokes render immediately, bypassing history — which means
// by the time commitTextEdit runs, the stored document's text already equals
// the final working text. commitTextEdit needs the pre-edit snapshot (not the
// already-mutated live value) to correctly detect a genuine no-op edit.
var textEditOriginalText = map[*instance]string{}

// AddTextLayerPayload is the JSON payload for commandAddTextLayer.
type AddTextLayerPayload struct {
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	FontSize float64  `json:"fontSize,omitempty"`
	Color    [4]uint8 `json:"color,omitempty"`
	TextType string   `json:"textType,omitempty"`
}

// SetTextContentPayload is the JSON payload for commandSetTextContent.
type SetTextContentPayload struct {
	LayerID string `json:"layerId"`
	Text    string `json:"text"`
}

// SetTextStylePayload is the JSON payload for commandSetTextStyle.
type SetTextStylePayload struct {
	LayerID       string    `json:"layerId"`
	FontFamily    *string   `json:"fontFamily,omitempty"`
	FontStyle     *string   `json:"fontStyle,omitempty"`
	FontSize      *float64  `json:"fontSize,omitempty"`
	Bold          *bool     `json:"bold,omitempty"`
	Italic        *bool     `json:"italic,omitempty"`
	Color         *[4]uint8 `json:"color,omitempty"`
	Alignment     *string   `json:"alignment,omitempty"`
	Leading       *float64  `json:"leading,omitempty"`
	TextType      *string   `json:"textType,omitempty"`
	Tracking      *float64  `json:"tracking,omitempty"`
	AntiAlias     *string   `json:"antiAlias,omitempty"`
	Kerning       *float64  `json:"kerning,omitempty"`
	Language      *string   `json:"language,omitempty"`
	BaselineShift *float64  `json:"baselineShift,omitempty"`
	Superscript   *bool     `json:"superscript,omitempty"`
	Subscript     *bool     `json:"subscript,omitempty"`
	Orientation   *string   `json:"orientation,omitempty"`
	Underline     *bool     `json:"underline,omitempty"`
	Strikethrough *bool     `json:"strikethrough,omitempty"`
	AllCaps       *bool     `json:"allCaps,omitempty"`
	SmallCaps     *bool     `json:"smallCaps,omitempty"`
	IndentLeft    *float64  `json:"indentLeft,omitempty"`
	IndentRight   *float64  `json:"indentRight,omitempty"`
	IndentFirst   *float64  `json:"indentFirst,omitempty"`
	SpaceBefore   *float64  `json:"spaceBefore,omitempty"`
	SpaceAfter    *float64  `json:"spaceAfter,omitempty"`
}

// EnterTextEditModePayload is the JSON payload for commandEnterTextEditMode.
type EnterTextEditModePayload struct {
	LayerID string `json:"layerId"`
}

// TextEditInputPayload is the JSON payload for commandTextEditInput.
// The frontend sends the complete current text string on every keystroke.
type TextEditInputPayload struct {
	Text string `json:"text"`
}

// ConvertTextToPathPayload is the JSON payload for commandConvertTextToPath.
type ConvertTextToPathPayload struct {
	LayerID string `json:"layerId"`
}

func (inst *instance) dispatchTextCommand(commandID int32, payloadJSON string) (bool, error) {
	if inst.manager.Active() == nil {
		return true, fmt.Errorf("no active document")
	}
	return cmdpkg.DispatchText(commandID, payloadJSON, cmdpkg.TextDeps{
		Decode: decodePayloadAny,
		AddTextLayer: func(payload cmdpkg.TextAddLayerPayload) error {
			return inst.addTextLayer(AddTextLayerPayload{
				X: payload.X, Y: payload.Y, FontSize: payload.FontSize, Color: payload.Color, TextType: payload.TextType,
			})
		},
		SetTextContent: func(payload cmdpkg.TextSetContentPayload) error {
			return inst.setTextContent(SetTextContentPayload{LayerID: payload.LayerID, Text: payload.Text})
		},
		SetTextStyle: func(payload cmdpkg.TextSetStylePayload) error {
			return inst.setTextStyle(SetTextStylePayload(payload))
		},
		EnterTextEditMode: func(layerID string) error {
			return inst.enterTextEditMode(EnterTextEditModePayload{LayerID: layerID})
		},
		TextEditInput: func(text string) error {
			return inst.textEditInput(TextEditInputPayload{Text: text})
		},
		CommitTextEdit: inst.commitTextEdit,
		ConvertTextToPath: func(layerID string) error {
			return inst.convertTextToPath(ConvertTextToPathPayload{LayerID: layerID})
		},
	})
}

// addTextLayer creates a new TextLayer at (x,y) and immediately enters edit mode.
func (inst *instance) addTextLayer(p AddTextLayerPayload) error {
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}

	fontSize := p.FontSize
	if fontSize <= 0 {
		fontSize = 36
	}
	color := p.Color
	if color == [4]uint8{} {
		color = [4]uint8{0, 0, 0, 255}
	}
	textType := p.TextType
	if textType == "" {
		textType = "point"
	}

	var newLayerID string
	if err := inst.executeDocCommand("Add text layer", func(doc *Document) error {
		bounds := LayerBounds{
			X: int(p.X),
			Y: int(p.Y),
			W: doc.Width,
			H: doc.Height,
		}
		layer := NewTextLayer("Text", bounds, "", nil)
		layer.FontSize = fontSize
		layer.Color = color
		layer.TextType = textType

		// Insert above active layer.
		parentID := ""
		index := -1
		if _, parent, idx, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID); ok && parent != nil {
			parentID = parent.ID()
			if parentID == doc.ensureLayerRoot().ID() {
				parentID = ""
			}
			index = idx + 1
		}
		if err := doc.AddLayer(layer, parentID, index); err != nil {
			return err
		}
		doc.ActiveLayerID = layer.ID()
		newLayerID = layer.ID()
		return nil
	}); err != nil {
		return err
	}

	// Enter edit mode immediately (UI-only state, no history entry).
	inst.textEdit.layerID = newLayerID
	inst.textEdit.workingText = ""
	textEditOriginalMu.Lock()
	textEditOriginalText[inst] = ""
	textEditOriginalMu.Unlock()
	return nil
}

// setTextContent replaces a text layer's string and re-rasterizes.
func (inst *instance) setTextContent(p SetTextContentPayload) error {
	return inst.executeDocCommand("Edit text", func(doc *Document) error {
		layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), p.LayerID)
		if !ok {
			return fmt.Errorf("layer %q not found", p.LayerID)
		}
		tl, ok := layer.(*TextLayer)
		if !ok {
			return fmt.Errorf("layer %q is not a text layer", p.LayerID)
		}
		tl.Text = p.Text
		raster, err := rasterizeTextLayer(tl, doc.Width, doc.Height)
		if err != nil {
			return err
		}
		tl.CachedRaster = raster
		return nil
	})
}

// setTextStyle updates style properties on a text layer and re-rasterizes.
func (inst *instance) setTextStyle(p SetTextStylePayload) error {
	return inst.executeDocCommand("Set text style", func(doc *Document) error {
		layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), p.LayerID)
		if !ok {
			return fmt.Errorf("layer %q not found", p.LayerID)
		}
		tl, ok := layer.(*TextLayer)
		if !ok {
			return fmt.Errorf("layer %q is not a text layer", p.LayerID)
		}
		if p.FontFamily != nil {
			tl.FontFamily = *p.FontFamily
		}
		if p.FontStyle != nil {
			tl.FontStyle = *p.FontStyle
		}
		if p.FontSize != nil {
			tl.FontSize = *p.FontSize
		}
		if p.Bold != nil {
			tl.Bold = *p.Bold
		}
		if p.Italic != nil {
			tl.Italic = *p.Italic
		}
		if p.Color != nil {
			tl.Color = *p.Color
		}
		if p.Alignment != nil {
			tl.Alignment = *p.Alignment
		}
		if p.Leading != nil {
			tl.Leading = *p.Leading
		}
		if p.TextType != nil {
			tl.TextType = *p.TextType
		}
		if p.Tracking != nil {
			tl.Tracking = *p.Tracking
		}
		if p.AntiAlias != nil {
			tl.AntiAlias = *p.AntiAlias
		}
		if p.Kerning != nil {
			tl.Kerning = *p.Kerning
		}
		if p.Language != nil {
			tl.Language = *p.Language
		}
		if p.BaselineShift != nil {
			tl.BaselineShift = *p.BaselineShift
		}
		if p.Superscript != nil {
			tl.Superscript = *p.Superscript
		}
		if p.Subscript != nil {
			tl.Subscript = *p.Subscript
		}
		if p.Orientation != nil {
			tl.Orientation = *p.Orientation
		}
		if p.Underline != nil {
			tl.Underline = *p.Underline
		}
		if p.Strikethrough != nil {
			tl.Strikethrough = *p.Strikethrough
		}
		if p.AllCaps != nil {
			tl.AllCaps = *p.AllCaps
		}
		if p.SmallCaps != nil {
			tl.SmallCaps = *p.SmallCaps
		}
		if p.IndentLeft != nil {
			tl.IndentLeft = *p.IndentLeft
		}
		if p.IndentRight != nil {
			tl.IndentRight = *p.IndentRight
		}
		if p.IndentFirst != nil {
			tl.IndentFirst = *p.IndentFirst
		}
		if p.SpaceBefore != nil {
			tl.SpaceBefore = *p.SpaceBefore
		}
		if p.SpaceAfter != nil {
			tl.SpaceAfter = *p.SpaceAfter
		}
		raster, err := rasterizeTextLayer(tl, doc.Width, doc.Height)
		if err != nil {
			return err
		}
		tl.CachedRaster = raster
		return nil
	})
}

// enterTextEditMode sets up in-flight text edit state without creating a history entry.
func (inst *instance) enterTextEditMode(p EnterTextEditModePayload) error {
	doc := inst.manager.activeMut()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), p.LayerID)
	if !ok {
		return fmt.Errorf("layer %q not found", p.LayerID)
	}
	tl, ok := layer.(*TextLayer)
	if !ok {
		return fmt.Errorf("layer %q is not a text layer", p.LayerID)
	}
	inst.textEdit.layerID = p.LayerID
	inst.textEdit.workingText = tl.Text
	textEditOriginalMu.Lock()
	textEditOriginalText[inst] = tl.Text
	textEditOriginalMu.Unlock()
	doc.ActiveLayerID = p.LayerID
	return nil
}

// textEditInput updates the working text and re-rasterizes without creating a
// history entry. Called on every keystroke while in text edit mode.
func (inst *instance) textEditInput(p TextEditInputPayload) error {
	if inst.textEdit.layerID == "" {
		return nil
	}
	inst.textEdit.workingText = p.Text

	doc := inst.manager.activeMut()
	if doc == nil {
		return nil
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), inst.textEdit.layerID)
	if !ok {
		return nil
	}
	tl, ok := layer.(*TextLayer)
	if !ok {
		return nil
	}
	// Direct mutation — intentionally bypasses executeDocCommand so that
	// mid-edit keystrokes are not individual undo entries.
	tl.Text = p.Text
	raster, err := rasterizeTextLayer(tl, doc.Width, doc.Height)
	if err != nil {
		return err
	}
	tl.CachedRaster = raster
	doc.ContentVersion++
	return nil
}

// commitTextEdit finalizes the in-flight edit as a single undoable history entry.
// If the text did not change, no history entry is created.
func (inst *instance) commitTextEdit() error {
	if inst.textEdit.layerID == "" {
		return nil
	}
	layerID := inst.textEdit.layerID
	newText := inst.textEdit.workingText
	inst.textEdit = textEditState{}

	textEditOriginalMu.Lock()
	originalText := textEditOriginalText[inst]
	delete(textEditOriginalText, inst)
	textEditOriginalMu.Unlock()

	doc := inst.manager.activeMut()
	if doc == nil {
		return nil
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return nil
	}
	tl, ok := layer.(*TextLayer)
	if !ok {
		return nil
	}
	// Skip history entry when text is unchanged from what it was when
	// editing began. The live document's text already reflects the
	// in-progress edit — textEditInput mutates the stored document
	// directly — so the comparison must be against the pre-edit snapshot
	// captured when editing began, not the live value.
	if originalText == newText {
		return nil
	}
	// executeDocCommand captures its "before" history snapshot from the live
	// document at the moment it runs. Because textEditInput already mutated
	// the live document to newText on the last keystroke, revert it back to
	// the pre-edit text/raster here so history records the correct
	// original -> newText transition (and undo restores the original text).
	tl.Text = originalText
	if raster, err := rasterizeTextLayer(tl, doc.Width, doc.Height); err == nil {
		tl.CachedRaster = raster
	}
	return inst.executeDocCommand("Edit text", func(doc *Document) error {
		l, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
		if !ok {
			return nil
		}
		textLayer, ok := l.(*TextLayer)
		if !ok {
			return nil
		}
		textLayer.Text = newText
		raster, err := rasterizeTextLayer(textLayer, doc.Width, doc.Height)
		if err != nil {
			return err
		}
		textLayer.CachedRaster = raster
		return nil
	})
}

// convertTextToPath converts a TextLayer into a VectorLayer by tracing GSV
// glyph centerlines. GSV is a stroke font, so the resulting vector layer is
// stroke-only: open centerline subpaths stroked at the same width Agg2D uses
// when rasterizing GSV text.
func (inst *instance) convertTextToPath(p ConvertTextToPathPayload) error {
	return inst.executeDocCommand("Create Outlines", func(doc *Document) error {
		layer, parent, idx, ok := findLayerByID(doc.ensureLayerRoot(), p.LayerID)
		if !ok {
			return fmt.Errorf("layer %q not found", p.LayerID)
		}
		tl, ok := layer.(*TextLayer)
		if !ok {
			return fmt.Errorf("layer %q is not a text layer", p.LayerID)
		}

		outlinePath := buildTextOutlinePath(tl)
		if outlinePath == nil || len(outlinePath.Subpaths) == 0 {
			return fmt.Errorf("text layer %q has no content to convert to outlines", tl.Name())
		}

		fontSize := tl.FontSize
		if fontSize <= 0 {
			fontSize = 16
		}
		strokeWidth := gsvOutlineStrokeWidth(fontSize)
		raster, err := rasterizeVectorShape(outlinePath, doc.Width, doc.Height, [4]uint8{}, tl.Color, strokeWidth)
		if err != nil {
			return err
		}
		vectorLayer := NewVectorLayer(tl.Name()+" Outlines", tl.Bounds, outlinePath, raster)
		vectorLayer.FillColor = [4]uint8{}
		vectorLayer.StrokeColor = tl.Color
		vectorLayer.StrokeWidth = strokeWidth

		// Replace the text layer with the vector layer at the same position.
		if parent == nil {
			return fmt.Errorf("layer %q has no parent", p.LayerID)
		}
		children := parent.Children()
		updated := make([]LayerNode, 0, len(children))
		for i, child := range children {
			if i == idx {
				updated = append(updated, vectorLayer)
			} else {
				updated = append(updated, child)
			}
		}
		parent.SetChildren(updated)
		doc.ActiveLayerID = vectorLayer.ID()
		doc.normalizeClippingState()
		return nil
	})
}
