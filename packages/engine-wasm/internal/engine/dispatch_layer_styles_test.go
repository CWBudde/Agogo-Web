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

	historyBeforeCopy := inst.history.CurrentIndex()
	if _, err := DispatchCommand(h, commandCopyLayerStyle, mustJSON(t, CopyLayerStylePayload{LayerID: firstID})); err != nil {
		t.Fatalf("copy layer style: %v", err)
	}
	if inst.history.CurrentIndex() != historyBeforeCopy {
		t.Fatalf("copy should not change history index: got %d want %d", inst.history.CurrentIndex(), historyBeforeCopy)
	}

	if _, err := DispatchCommand(h, commandPasteLayerStyle, mustJSON(t, PasteLayerStylePayload{LayerID: secondID})); err != nil {
		t.Fatalf("paste layer style: %v", err)
	}

	doc = inst.manager.Active()
	secondLayer := doc.findLayer(secondID)
	if secondLayer == nil {
		t.Fatalf("second layer %q not found", secondID)
	}
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantUpdated) {
		t.Fatalf("second-layer styles after paste = %#v, want %#v", got, wantUpdated)
	}

	if _, err := DispatchCommand(h, commandClearLayerStyle, mustJSON(t, ClearLayerStylePayload{LayerID: firstID})); err != nil {
		t.Fatalf("clear layer style: %v", err)
	}

	doc = inst.manager.Active()
	firstLayer = doc.findLayer(firstID)
	if got := firstLayer.StyleStack(); len(got) != 0 {
		t.Fatalf("first-layer styles after clear = %#v, want empty", got)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
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
	if got := secondLayer.StyleStack(); !layerStylesEqual(got, wantUpdated) {
		t.Fatalf("second-layer styles after undo = %#v, want %#v", got, wantUpdated)
	}
}
