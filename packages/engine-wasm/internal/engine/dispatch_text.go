package engine

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/text"
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

// LoadFontDataPayload is the JSON payload for commandLoadFontData. Data
// carries the base64-encoded TTF/OTF font bytes.
type LoadFontDataPayload struct {
	Name   string `json:"name"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
	Data   string `json:"data"`
}

func (inst *instance) dispatchTextCommand(commandID int32, payloadJSON string) (bool, error) {
	// LoadFontData registers app-level (not document) state and must work
	// before any document is open — the frontend loads fonts at startup.
	// Every other text command targets the active document.
	if commandID != commandLoadFontData && inst.manager.Active() == nil {
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
		LoadFontData: func(payload cmdpkg.TextLoadFontDataPayload) error {
			return inst.loadFontData(LoadFontDataPayload(payload))
		},
		CancelTextEdit: func() error {
			// Escape: discard the in-flight edit and restore the pre-edit
			// text. No history entry; a no-op when nothing is being edited.
			inst.revertLiveTextEdit()
			return nil
		},
	})
}

// loadFontData decodes and registers TTF/OTF font bytes in the app-level
// font registry. It is deliberately NOT undoable: fonts are application
// state, not document state, so no history entry is created (the central
// DispatchCommand path already bumps uiMetaVersion for the UIMeta refresh).
func (inst *instance) loadFontData(p LoadFontDataPayload) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("font name required")
	}
	data, err := base64.StdEncoding.DecodeString(p.Data)
	if err != nil {
		return fmt.Errorf("decode font data: %w", err)
	}
	return text.DefaultRegistry().Register(p.Name, p.Bold, p.Italic, data)
}

// FontFamilyMeta describes one registered font family for UIMeta consumption.
type FontFamilyMeta struct {
	Family string   `json:"family"`
	Styles []string `json:"styles"`
}

// buildAvailableFontsMeta enumerates the app-level font registry for UIMeta.
func buildAvailableFontsMeta() []FontFamilyMeta {
	infos := text.DefaultRegistry().List()
	meta := make([]FontFamilyMeta, 0, len(infos))
	for _, info := range infos {
		meta = append(meta, FontFamilyMeta{Family: info.Family, Styles: info.Styles})
	}
	return meta
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
		// The click point is the pen origin (anchor). Area text keeps a
		// document-sized default frame (the wrapping width authority); point
		// text gets its tight bounds computed by rasterization below.
		bounds := LayerBounds{
			X: int(p.X),
			Y: int(p.Y),
			W: doc.Width,
			H: doc.Height,
		}
		layer := NewTextLayer("Text", bounds, "", nil)
		layer.AnchorX = p.X
		layer.AnchorY = p.Y
		layer.AnchorSet = true
		layer.FontSize = fontSize
		layer.Color = color
		layer.TextType = textType
		if err := rasterizeTextLayer(layer); err != nil {
			return err
		}

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
		beforeBounds := tl.Bounds
		tl.Text = p.Text
		if err := rasterizeTextLayer(tl); err != nil {
			return err
		}
		// Invalidate the union of the old and new bounds: tight point-text
		// bounds can shrink, and the vacated region must repaint too.
		doc.touchModifiedAtBounds(beforeBounds, tl.Bounds)
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
		beforeBounds := tl.Bounds
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
		if err := rasterizeTextLayer(tl); err != nil {
			return err
		}
		// Invalidate the union of the old and new bounds: style changes (font
		// size, tracking, alignment, …) can shrink the tight point-text box,
		// and the vacated region must repaint too.
		doc.touchModifiedAtBounds(beforeBounds, tl.Bounds)
		return nil
	})
}

// enterTextEditMode sets up in-flight text edit state without creating a history entry.
//
// The ActiveLayerID change deliberately uses the clone-and-replace pattern
// (like SetActiveLayer) instead of mutating the stored document in place:
// history snapshots reference stored documents directly (see captureSnapshot),
// and ActiveLayerID is part of the compared/captured document state, so an
// in-place write here would retroactively alter the latest history snapshot.
func (inst *instance) enterTextEditMode(p EnterTextEditModePayload) error {
	doc := inst.manager.Active()
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
	doc.ActiveLayerID = p.LayerID
	if err := inst.manager.ReplaceActive(doc); err != nil {
		return err
	}
	inst.textEdit.layerID = p.LayerID
	inst.textEdit.workingText = tl.Text
	textEditOriginalMu.Lock()
	textEditOriginalText[inst] = tl.Text
	textEditOriginalMu.Unlock()
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
	if err := rasterizeTextLayer(tl); err != nil {
		return err
	}
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
	// the pre-edit text/raster (and bounds — rasterization recomputes the
	// tight point-text box) here so history records the correct
	// original -> newText transition (and undo restores the original text).
	tl.Text = originalText
	_ = rasterizeTextLayer(tl)
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
		return rasterizeTextLayer(textLayer)
	})
}

// revertLiveTextEdit rolls back an in-flight text edit to the pre-edit text
// recorded in the side table and discards the edit state without committing a
// history entry. Called when a history restore interrupts live text editing
// (see revertInFlightPreviewMutations): textEditInput mutates the stored
// document in place on every keystroke, and pointer-based history snapshots
// may reference that document, so the uncommitted keystrokes must be undone
// byte-exactly before a snapshot is installed.
func (inst *instance) revertLiveTextEdit() {
	if inst.textEdit.layerID == "" {
		return
	}
	layerID := inst.textEdit.layerID
	inst.textEdit = textEditState{}

	textEditOriginalMu.Lock()
	originalText := textEditOriginalText[inst]
	delete(textEditOriginalText, inst)
	textEditOriginalMu.Unlock()

	doc := inst.manager.activeMut()
	if doc == nil {
		return
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return
	}
	tl, ok := layer.(*TextLayer)
	if !ok || tl.Text == originalText {
		return
	}
	tl.Text = originalText
	_ = rasterizeTextLayer(tl)
	doc.ContentVersion++
}

// convertTextToPath converts a TextLayer into a fill-only VectorLayer whose
// shape is the layer's glyph outlines as closed contours (Type > Create
// Outlines). The vector layer fills with the text color and carries no
// stroke, matching the rasterized text's appearance.
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

		// Rasterize with the NON-ZERO winding rule, matching the live text
		// render (text_render.go): TrueType contours encode counters via
		// winding direction, and contours of ADJACENT glyphs can overlap
		// (negative tracking/kerning) — even-odd would XOR the overlap out
		// to holes that the original raster fills solid. The rule is
		// persisted on the layer (FillRule below) so later
		// re-rasterizations (style change, vector edit, crop) keep it.
		raster, err := rasterizeVectorShapeFillRule(outlinePath, doc.Width, doc.Height, tl.Color, [4]uint8{}, 0, FillRuleNonZero)
		if err != nil {
			return err
		}
		// The outline path and its raster are both in document coordinates, so
		// the vector layer's bounds must sit at the document origin to satisfy
		// the bounds-local CachedRaster contract (raster pixel (0,0) maps to
		// document pixel (Bounds.X, Bounds.Y)). Using tl.Bounds here would
		// apply the text layer's position twice.
		outlineBounds := LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}
		vectorLayer := NewVectorLayer(tl.Name()+" Outlines", outlineBounds, outlinePath, raster)
		vectorLayer.FillColor = tl.Color
		vectorLayer.StrokeColor = [4]uint8{}
		vectorLayer.StrokeWidth = 0
		vectorLayer.FillRule = FillRuleNonZero

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
		// The new vector layer has document-sized bounds, so a layer-scoped
		// touch also covers the replaced text layer's (smaller) bounds.
		doc.touchModifiedAtLayer(vectorLayer)
		return nil
	})
}
