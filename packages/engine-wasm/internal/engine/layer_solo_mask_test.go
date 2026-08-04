package engine

import (
	"encoding/json"
	"testing"

	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

func TestSoloLayerVisibilityNestedGroupClippingRestoreAndRetarget(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	outside := NewPixelLayer("Outside", LayerBounds{}, nil)
	group := NewGroupLayer("Group")
	group.SetVisible(false)
	base := NewPixelLayer("Base", LayerBounds{}, nil)
	base.SetVisible(false)
	clipped := NewPixelLayer("Clipped", LayerBounds{}, nil)
	clipped.SetClipToBelow(true)
	sibling := NewPixelLayer("Sibling", LayerBounds{}, nil)
	group.SetChildren([]LayerNode{base, clipped, sibling})
	doc.LayerRoot.SetChildren([]LayerNode{outside, group})
	doc.normalizeClippingState()

	if err := doc.SoloLayerVisibility(clipped.ID()); err != nil {
		t.Fatal(err)
	}
	if outside.Visible() || sibling.Visible() || !group.Visible() || !base.Visible() || !clipped.Visible() {
		t.Fatalf("clipped solo visibility = outside:%v group:%v base:%v clipped:%v sibling:%v", outside.Visible(), group.Visible(), base.Visible(), clipped.Visible(), sibling.Visible())
	}

	// A different target is isolated from the original baseline, not the
	// already-isolated visibility state.
	if err := doc.SoloLayerVisibility(group.ID()); err != nil {
		t.Fatal(err)
	}
	if outside.Visible() || !group.Visible() || base.Visible() || !clipped.Visible() || !sibling.Visible() {
		t.Fatalf("group solo did not preserve baseline subtree visibility")
	}
	if err := doc.SoloLayerVisibility(group.ID()); err != nil {
		t.Fatal(err)
	}
	if !outside.Visible() || group.Visible() || base.Visible() || !clipped.Visible() || !sibling.Visible() {
		t.Fatalf("second group solo did not restore guarded baseline")
	}

	if err := doc.SoloLayerVisibility(outside.ID()); err != nil {
		t.Fatal(err)
	}
	newcomer := NewPixelLayer("New", LayerBounds{}, nil)
	if err := doc.AddLayer(newcomer, "", -1); err != nil {
		t.Fatal(err)
	}
	if err := doc.SoloLayerVisibility(outside.ID()); err != nil {
		t.Fatal(err)
	}
	if newcomer.Visible() {
		t.Fatal("structural edit must invalidate the stale solo baseline instead of restoring it")
	}
}

func TestSoloLayerVisibilityIsOneUndoableCommandAndOrdinaryToggleInvalidatesRestore(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	first, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{LayerType: LayerTypePixel, Name: "First", Bounds: LayerBounds{W: 1, H: 1}, Pixels: []byte{255, 0, 0, 255}}))
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.UIMeta.ActiveLayerID
	second, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{LayerType: LayerTypePixel, Name: "Second", Bounds: LayerBounds{W: 1, H: 1}, Pixels: []byte{0, 0, 255, 255}}))
	if err != nil {
		t.Fatal(err)
	}
	secondID := second.UIMeta.ActiveLayerID
	inst := instances[h]
	before := len(inst.history.Entries())
	if _, err := DispatchCommand(h, commandSoloLayerVisibility, mustJSON(t, map[string]string{"layerId": firstID})); err != nil {
		t.Fatal(err)
	}
	if got := len(inst.history.Entries()); got != before+1 {
		t.Fatalf("history entries = %d, want %d", got, before+1)
	}
	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatal(err)
	}
	if !inst.manager.Active().findLayer(secondID).Visible() {
		t.Fatal("undo did not restore second layer visibility")
	}
	if _, err := DispatchCommand(h, commandRedo, ""); err != nil {
		t.Fatal(err)
	}
	if inst.manager.Active().findLayer(secondID).Visible() {
		t.Fatal("redo did not restore solo visibility")
	}
	if _, err := DispatchCommand(h, commandSetLayerVis, mustJSON(t, SetLayerVisibilityPayload{LayerID: secondID, Visible: true})); err != nil {
		t.Fatal(err)
	}
	if _, err := DispatchCommand(h, commandSoloLayerVisibility, mustJSON(t, map[string]string{"layerId": firstID})); err != nil {
		t.Fatal(err)
	}
	if inst.manager.Active().findLayer(secondID).Visible() {
		t.Fatal("ordinary visibility edit must invalidate the old solo baseline")
	}
}

func TestLayerMaskDensityFeatherRenderCacheMetaCloneAndArchive(t *testing.T) {
	formulaMask := &LayerMask{Enabled: true, Width: 1, Height: 1, Data: []byte{0}}
	formulaDensity := 50
	formulaMask.SetProperties(&formulaDensity, nil)
	if got := effectiveRasterMask(formulaMask).Data[0]; got != 128 {
		t.Fatalf("50%% density over zero coverage = %d, want 128", got)
	}

	doc := &Document{Width: 3, Height: 1, ID: "mask-doc", Name: "Mask", Resolution: 72, ColorMode: "rgb", BitDepth: 8, LayerRoot: NewGroupLayer("Root")}
	layer := NewPixelLayer("Masked", LayerBounds{W: 3, H: 1}, []byte{255, 0, 0, 255, 255, 0, 0, 255, 255, 0, 0, 255})
	mask := &LayerMask{Enabled: true, Width: 3, Height: 1, Data: []byte{0, 255, 0}}
	density, feather := 50, 1
	mask.SetProperties(&density, &feather)
	layer.SetMask(mask)
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()

	effective := effectiveRasterMask(layer.Mask())
	if effective.Data[0] <= 127 || effective.Data[0] == 255 || effective.Data[1] <= effective.Data[0] {
		t.Fatalf("unexpected feather+density coverage %v", effective.Data)
	}
	if got := layer.Mask().Data; got[0] != 0 || got[1] != 255 || got[2] != 0 {
		t.Fatalf("derived coverage mutated authoritative mask data: %v", got)
	}
	firstCache := effective.Data
	if second := effectiveRasterMask(layer.Mask()).Data; &second[0] != &firstCache[0] {
		t.Fatal("effective mask coverage was not cached")
	}
	layer.Mask().Data[0] = 255
	if refreshed := effectiveRasterMask(layer.Mask()).Data; &refreshed[0] == &firstCache[0] {
		t.Fatal("mutating source mask data did not invalidate effective coverage")
	}

	meta := docpkg.BuildLayerNodeMeta(layer)
	if meta.MaskDensity == nil || *meta.MaskDensity != 50 || meta.MaskFeather != 1 {
		t.Fatalf("mask meta = density %v feather %d", meta.MaskDensity, meta.MaskFeather)
	}
	clone := layer.Clone()
	if !layerTreeEqual(layer, clone) || clone.Mask() == layer.Mask() || &clone.Mask().Data[0] == &layer.Mask().Data[0] {
		t.Fatal("layer mask properties/data were not deeply cloned and compared")
	}

	data, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	restored, _, err := LoadProject(data)
	if err != nil {
		t.Fatal(err)
	}
	restoredMask := restored.findLayer(layer.ID()).Mask()
	if restoredMask.DensityValue() != 50 || restoredMask.Feather != 1 {
		t.Fatalf("restored properties = density %d feather %d", restoredMask.DensityValue(), restoredMask.Feather)
	}

	var legacy LayerMask
	if err := json.Unmarshal([]byte(`{"enabled":true,"width":1,"height":1,"data":"/w=="}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.DensityValue() != 100 || legacy.Feather != 0 {
		t.Fatalf("legacy defaults = density %d feather %d", legacy.DensityValue(), legacy.Feather)
	}
}

func TestSetLayerMaskPropertiesClampAndHistory(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)
	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{LayerType: LayerTypePixel, Name: "Masked", Bounds: LayerBounds{W: 1, H: 1}, Pixels: []byte{255, 0, 0, 255}}))
	if err != nil {
		t.Fatal(err)
	}
	layerID := added.UIMeta.ActiveLayerID
	if _, err := DispatchCommand(h, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{LayerID: layerID, Mode: AddLayerMaskRevealAll})); err != nil {
		t.Fatal(err)
	}
	inst := instances[h]
	before := len(inst.history.Entries())
	density, feather := -20, 999
	if _, err := DispatchCommand(h, commandSetLayerMaskProperties, mustJSON(t, map[string]any{"layerId": layerID, "density": density, "feather": feather})); err != nil {
		t.Fatal(err)
	}
	mask := inst.manager.Active().findLayer(layerID).Mask()
	if mask.DensityValue() != 0 || mask.Feather != 254 {
		t.Fatalf("clamped properties = density %d feather %d", mask.DensityValue(), mask.Feather)
	}
	if len(inst.history.Entries()) != before+1 {
		t.Fatal("mask property command must add one history entry")
	}
	if _, err := DispatchCommand(h, commandUndo, ""); err != nil {
		t.Fatal(err)
	}
	mask = inst.manager.Active().findLayer(layerID).Mask()
	if mask.DensityValue() != 100 || mask.Feather != 0 {
		t.Fatalf("undo properties = density %d feather %d", mask.DensityValue(), mask.Feather)
	}
}
