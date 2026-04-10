package engine

import (
	"encoding/json"
	"testing"
)

func mustRawJSON(t *testing.T, raw string) json.RawMessage {
	t.Helper()
	return json.RawMessage(raw)
}

func TestLayerStyleCommands_SetEnableClearCopyPasteUndo(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	first, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "First",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{255, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add first layer: %v", err)
	}
	firstID := first.UIMeta.ActiveLayerID

	second, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Second",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{0, 0, 255, 255},
	}))
	if err != nil {
		t.Fatalf("add second layer: %v", err)
	}
	secondID := second.UIMeta.ActiveLayerID

	if _, err := DispatchCommand(h, commandSetLayerStyleStack, mustJSON(t, SetLayerStyleStackPayload{
		LayerID: firstID,
		Styles: []LayerStylePayload{{
			Kind:    LayerStyleKindColorOverlay,
			Enabled: true,
			Params:  mustRawJSON(t, `{"color":[0,255,0,255],"opacity":1}`),
		}},
	})); err != nil {
		t.Fatalf("set layer style stack: %v", err)
	}

	inst := instances[h]
	doc := inst.manager.Active()
	firstLayer := doc.findLayer(firstID)
	if firstLayer == nil {
		t.Fatalf("first layer %q not found", firstID)
	}
	wantInitial := []LayerStyle{{
		Kind:    string(LayerStyleKindColorOverlay),
		Enabled: true,
		Params:  mustRawJSON(t, `{"color":[0,255,0,255],"opacity":1}`),
	}}
	if got := firstLayer.StyleStack(); !layerStylesEqual(got, wantInitial) {
		t.Fatalf("initial first-layer styles = %#v, want %#v", got, wantInitial)
	}

	if _, err := DispatchCommand(h, commandSetLayerStyleEnabled, mustJSON(t, SetLayerStyleEnabledPayload{
		LayerID: firstID,
		Kind:    LayerStyleKindColorOverlay,
		Enabled: false,
	})); err != nil {
		t.Fatalf("disable layer style: %v", err)
	}

	if _, err := DispatchCommand(h, commandSetLayerStyleParams, mustJSON(t, SetLayerStyleParamsPayload{
		LayerID: firstID,
		Kind:    LayerStyleKindColorOverlay,
		Params:  mustRawJSON(t, `{"color":[255,255,0,255],"opacity":0.5}`),
	})); err != nil {
		t.Fatalf("set layer style params: %v", err)
	}

	doc = inst.manager.Active()
	firstLayer = doc.findLayer(firstID)
	wantUpdated := []LayerStyle{{
		Kind:    string(LayerStyleKindColorOverlay),
		Enabled: false,
		Params:  mustRawJSON(t, `{"color":[255,255,0,255],"opacity":0.5}`),
	}}
	if got := firstLayer.StyleStack(); !layerStylesEqual(got, wantUpdated) {
		t.Fatalf("updated first-layer styles = %#v, want %#v", got, wantUpdated)
	}

	if _, err := DispatchCommand(h, commandSetLayerStyleStack, mustJSON(t, SetLayerStyleStackPayload{
		LayerID: secondID,
		Styles: []LayerStylePayload{{
			Kind:    LayerStyleKindColorOverlay,
			Enabled: true,
			Params:  mustRawJSON(t, `{"color":[0,0,255,255],"opacity":0.75}`),
		}},
	})); err != nil {
		t.Fatalf("set second layer style stack: %v", err)
	}

	doc = inst.manager.Active()
	secondLayer := doc.findLayer(secondID)
	if secondLayer == nil {
		t.Fatalf("second layer %q not found", secondID)
	}
	wantSecondInitial := []LayerStyle{{
		Kind:    string(LayerStyleKindColorOverlay),
		Enabled: true,
		Params:  mustRawJSON(t, `{"color":[0,0,255,255],"opacity":0.75}`),
	}}
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantSecondInitial) {
		t.Fatalf("initial second-layer styles = %#v, want %#v", got, wantSecondInitial)
	}

	historyBeforePaste := inst.history.CurrentIndex()
	if _, err := DispatchCommand(h, commandPasteLayerStyle, mustJSON(t, PasteLayerStylePayload{LayerID: secondID})); err == nil {
		t.Fatal("expected paste before copy to fail")
	}

	doc = inst.manager.Active()
	secondLayer = doc.findLayer(secondID)
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantSecondInitial) {
		t.Fatalf("second-layer styles after paste-before-copy = %#v, want %#v", got, wantSecondInitial)
	}
	if inst.history.CurrentIndex() != historyBeforePaste {
		t.Fatalf("paste-before-copy should not change history index: got %d want %d", inst.history.CurrentIndex(), historyBeforePaste)
	}

	historyBeforeCopy := inst.history.CurrentIndex()
	if _, err := DispatchCommand(h, commandCopyLayerStyle, mustJSON(t, CopyLayerStylePayload{LayerID: firstID})); err != nil {
		t.Fatalf("copy layer style: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforeCopy {
		t.Fatalf("copy should not change history index: got %d want %d", inst.history.CurrentIndex(), historyBeforeCopy)
	}

	historyBeforePaste = inst.history.CurrentIndex()
	if _, err := DispatchCommand(h, commandPasteLayerStyle, mustJSON(t, PasteLayerStylePayload{LayerID: secondID})); err != nil {
		t.Fatalf("paste layer style: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforePaste+1 {
		t.Fatalf("paste should add one history entry: got %d want %d", inst.history.CurrentIndex(), historyBeforePaste+1)
	}

	doc = inst.manager.Active()
	secondLayer = doc.findLayer(secondID)
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantUpdated) {
		t.Fatalf("second-layer styles after paste = %#v, want %#v", got, wantUpdated)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo paste layer style: %v", err)
	}
	if !undone.UIMeta.CanUndo {
		t.Fatal("expected CanUndo to remain true after undoing paste layer style")
	}

	doc = inst.manager.Active()
	secondLayer = doc.findLayer(secondID)
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantSecondInitial) {
		t.Fatalf("second-layer styles after undoing paste = %#v, want %#v", got, wantSecondInitial)
	}

	if _, err := DispatchCommand(h, commandClearLayerStyle, mustJSON(t, ClearLayerStylePayload{LayerID: firstID})); err != nil {
		t.Fatalf("clear layer style: %v", err)
	}

	doc = inst.manager.Active()
	firstLayer = doc.findLayer(firstID)
	if got := firstLayer.StyleStack(); len(got) != 0 {
		t.Fatalf("first-layer styles after clear = %#v, want empty", got)
	}

	historyBeforePaste = inst.history.CurrentIndex()
	if _, err := DispatchCommand(h, commandCopyLayerStyle, mustJSON(t, CopyLayerStylePayload{LayerID: firstID})); err != nil {
		t.Fatalf("copy empty layer style: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforePaste {
		t.Fatalf("copying empty style stack should not change history index: got %d want %d", inst.history.CurrentIndex(), historyBeforePaste)
	}

	historyBeforePaste = inst.history.CurrentIndex()
	if _, err := DispatchCommand(h, commandPasteLayerStyle, mustJSON(t, PasteLayerStylePayload{LayerID: secondID})); err != nil {
		t.Fatalf("paste empty layer style: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforePaste+1 {
		t.Fatalf("paste of empty style stack should add one history entry: got %d want %d", inst.history.CurrentIndex(), historyBeforePaste+1)
	}

	doc = inst.manager.Active()
	secondLayer = doc.findLayer(secondID)
	if got := secondLayer.StyleStack(); len(got) != 0 {
		t.Fatalf("second-layer styles after pasting empty stack = %#v, want empty", got)
	}

	undone, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo empty paste layer style: %v", err)
	}
	if !undone.UIMeta.CanUndo {
		t.Fatal("expected CanUndo to remain true after undoing empty paste layer style")
	}

	doc = inst.manager.Active()
	secondLayer = doc.findLayer(secondID)
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantSecondInitial) {
		t.Fatalf("second-layer styles after undoing empty paste = %#v, want %#v", got, wantSecondInitial)
	}

	undone, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo clear layer style: %v", err)
	}
	if !undone.UIMeta.CanUndo {
		t.Fatal("expected CanUndo to remain true after undoing clear layer style")
	}

	doc = inst.manager.Active()
	firstLayer = doc.findLayer(firstID)
	if got := firstLayer.StyleStack(); !layerStylesEqual(got, wantUpdated) {
		t.Fatalf("first-layer styles after undo = %#v, want %#v", got, wantUpdated)
	}
	secondLayer = doc.findLayer(secondID)
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantSecondInitial) {
		t.Fatalf("second-layer styles after undo = %#v, want %#v", got, wantSecondInitial)
	}
}
