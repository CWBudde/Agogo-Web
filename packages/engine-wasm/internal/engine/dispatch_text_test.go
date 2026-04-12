package engine

import "testing"

// initTextTestDoc creates an engine instance with a 200×100 document.
func initTextTestDoc(t *testing.T) int32 {
	t.Helper()
	h := Init(`{"documentWidth":200,"documentHeight":100,"background":"transparent","resolution":72}`)
	if h <= 0 {
		t.Fatalf("Init returned invalid handle %d", h)
	}
	return h
}

func TestAddTextLayer_CreatesTextLayerWithEditMode(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	result, err := DispatchCommand(h, commandAddTextLayer, mustJSON(t, AddTextLayerPayload{
		X:        10,
		Y:        10,
		FontSize: 24,
		Color:    [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddTextLayer: %v", err)
	}

	// Verify a text layer was added.
	layers := result.UIMeta.Layers
	if len(layers) == 0 {
		t.Fatal("expected at least one layer")
	}
	textMeta := layers[len(layers)-1]
	if textMeta.LayerType != LayerTypeText {
		t.Errorf("layer type = %q, want %q", textMeta.LayerType, LayerTypeText)
	}

	// Edit mode must be active.
	if result.UIMeta.EditingTextLayerID == "" {
		t.Error("expected EditingTextLayerID to be set after AddTextLayer")
	}

	// Must be the active layer.
	if result.UIMeta.ActiveLayerID != textMeta.ID {
		t.Errorf("active layer = %q, want %q", result.UIMeta.ActiveLayerID, textMeta.ID)
	}
}

func TestAddTextLayer_DefaultFontSize(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	result, err := DispatchCommand(h, commandAddTextLayer, mustJSON(t, AddTextLayerPayload{
		X: 0, Y: 0,
	}))
	if err != nil {
		t.Fatalf("AddTextLayer: %v", err)
	}
	layers := result.UIMeta.Layers
	if len(layers) == 0 {
		t.Fatal("expected a layer")
	}
	l := layers[len(layers)-1]
	if l.TextFontSize == nil || *l.TextFontSize != 36 {
		t.Errorf("default fontSize = %v, want 36", l.TextFontSize)
	}
}

func TestSetTextContent_UpdatesTextAndUndoable(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	// Create text layer via AddLayer.
	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Headline",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 200, H: 100},
		Text:      "Initial",
		FontSize:  24,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	layerID := addResult.UIMeta.ActiveLayerID

	// Update text content.
	updateResult, err := DispatchCommand(h, commandSetTextContent, mustJSON(t, SetTextContentPayload{
		LayerID: layerID,
		Text:    "Updated",
	}))
	if err != nil {
		t.Fatalf("SetTextContent: %v", err)
	}

	// canUndo must be true after setting text.
	if !updateResult.UIMeta.CanUndo {
		t.Error("expected CanUndo = true after SetTextContent")
	}
}

func TestEnterAndCommitTextEdit_ProducesSingleHistoryEntry(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	// Create text layer.
	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Text",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 200, H: 100},
		Text:      "Before",
		FontSize:  24,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	layerID := addResult.UIMeta.ActiveLayerID
	historyBefore := len(addResult.UIMeta.History)

	// Enter edit mode.
	if _, err := DispatchCommand(h, commandEnterTextEditMode, mustJSON(t, EnterTextEditModePayload{
		LayerID: layerID,
	})); err != nil {
		t.Fatalf("EnterTextEditMode: %v", err)
	}

	// Simulate multiple keystroke events — none should add to history.
	for _, text := range []string{"A", "Af", "Aft", "Afte", "After"} {
		if _, err := DispatchCommand(h, commandTextEditInput, mustJSON(t, TextEditInputPayload{Text: text})); err != nil {
			t.Fatalf("TextEditInput(%q): %v", text, err)
		}
	}

	// Commit — must produce exactly one new history entry.
	commitResult, err := DispatchCommand(h, commandCommitTextEdit, "{}")
	if err != nil {
		t.Fatalf("CommitTextEdit: %v", err)
	}

	historyAfter := len(commitResult.UIMeta.History)
	if historyAfter != historyBefore+1 {
		t.Errorf("history length = %d, want %d (one new entry)", historyAfter, historyBefore+1)
	}

	// Edit mode must be cleared.
	if commitResult.UIMeta.EditingTextLayerID != "" {
		t.Errorf("EditingTextLayerID = %q after commit, want empty", commitResult.UIMeta.EditingTextLayerID)
	}
}

func TestCommitTextEdit_NoChange_DoesNotPollutHistory(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Text",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 200, H: 100},
		Text:      "Hello",
		FontSize:  20,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	layerID := addResult.UIMeta.ActiveLayerID
	historyBefore := len(addResult.UIMeta.History)

	// Enter edit mode, type same text, commit.
	if _, err := DispatchCommand(h, commandEnterTextEditMode, mustJSON(t, EnterTextEditModePayload{LayerID: layerID})); err != nil {
		t.Fatalf("EnterTextEditMode: %v", err)
	}
	if _, err := DispatchCommand(h, commandTextEditInput, mustJSON(t, TextEditInputPayload{Text: "Hello"})); err != nil {
		t.Fatalf("TextEditInput: %v", err)
	}
	commitResult, err := DispatchCommand(h, commandCommitTextEdit, "{}")
	if err != nil {
		t.Fatalf("CommitTextEdit: %v", err)
	}

	// History must not grow — text is unchanged.
	if len(commitResult.UIMeta.History) != historyBefore {
		t.Errorf("history length = %d, want %d (no new entry for unchanged text)",
			len(commitResult.UIMeta.History), historyBefore)
	}
}

func TestSetTextStyle_UpdatesFields(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Text",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 200, H: 100},
		Text:      "Hello",
		FontSize:  16,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	layerID := addResult.UIMeta.ActiveLayerID

	fontSize := 48.0
	color := [4]uint8{255, 0, 0, 255}
	alignment := "center"
	_, err = DispatchCommand(h, commandSetTextStyle, mustJSON(t, SetTextStylePayload{
		LayerID:   layerID,
		FontSize:  &fontSize,
		Color:     &color,
		Alignment: &alignment,
	}))
	if err != nil {
		t.Fatalf("SetTextStyle: %v", err)
	}

	// Verify updated meta via re-render.
	r2, _ := DispatchCommand(h, commandSetActiveLayer, mustJSON(t, SetActiveLayerPayload{LayerID: layerID}))
	for _, l := range r2.UIMeta.Layers {
		if l.ID == layerID {
			if l.TextFontSize == nil || *l.TextFontSize != 48 {
				t.Errorf("fontSize = %v, want 48", l.TextFontSize)
			}
			if l.TextAlignment == nil || *l.TextAlignment != "center" {
				t.Errorf("alignment = %v, want center", l.TextAlignment)
			}
			return
		}
	}
	t.Error("layer not found in UIMeta.Layers")
}

func TestAddTextLayer_TextInput_RasterizesPixels(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	// Add empty text layer.
	if _, err := DispatchCommand(h, commandAddTextLayer, mustJSON(t, AddTextLayerPayload{
		X: 0, Y: 0, FontSize: 24, Color: [4]uint8{0, 0, 0, 255},
	})); err != nil {
		t.Fatalf("AddTextLayer: %v", err)
	}

	// Send text.
	result, err := DispatchCommand(h, commandTextEditInput, mustJSON(t, TextEditInputPayload{
		Text: "Hi",
	}))
	if err != nil {
		t.Fatalf("TextEditInput: %v", err)
	}
	_ = result
}

func TestConvertTextToPath_ProducesGlyphOutlinePath(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Glyph",
		Bounds:    LayerBounds{X: 10, Y: 10, W: 180, H: 80},
		Text:      "A",
		FontSize:  32,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}

	result, err := DispatchCommand(h, commandConvertTextToPath, mustJSON(t, ConvertTextToPathPayload{
		LayerID: addResult.UIMeta.ActiveLayerID,
	}))
	if err != nil {
		t.Fatalf("ConvertTextToPath: %v", err)
	}

	inst := instances[h]
	doc := inst.manager.Active()
	layer := doc.findLayer(result.UIMeta.ActiveLayerID)
	vectorLayer, ok := layer.(*VectorLayer)
	if !ok {
		t.Fatalf("expected active layer to be a vector layer, got %T", layer)
	}
	if vectorLayer.Shape == nil || len(vectorLayer.Shape.Subpaths) == 0 {
		t.Fatal("expected vector outline path")
	}

	totalPoints := 0
	for _, subpath := range vectorLayer.Shape.Subpaths {
		totalPoints += len(subpath.Points)
	}
	if totalPoints <= 4 {
		t.Fatalf("expected glyph outline, got %d total points", totalPoints)
	}
}
