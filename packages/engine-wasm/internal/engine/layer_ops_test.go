package engine

import "testing"

func TestDocumentLayerOperationsAndUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	addPixel, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels: []byte{
			255, 0, 0, 255,
			255, 0, 0, 255,
			255, 0, 0, 255,
			255, 0, 0, 255,
		},
	}))
	if err != nil {
		t.Fatalf("add pixel layer: %v", err)
	}
	if len(addPixel.UIMeta.Layers) != 1 {
		t.Fatalf("layer count = %d, want 1", len(addPixel.UIMeta.Layers))
	}
	baseID := addPixel.UIMeta.ActiveLayerID

	addGroup, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeGroup,
		Name:      "Group",
		Isolated:  true,
	}))
	if err != nil {
		t.Fatalf("add group: %v", err)
	}
	groupID := addGroup.UIMeta.ActiveLayerID
	if len(addGroup.UIMeta.Layers) != 2 {
		t.Fatalf("layer count after group = %d, want 2", len(addGroup.UIMeta.Layers))
	}

	moveIndex := 0
	moved, err := DispatchCommand(h, commandMoveLayer, mustJSON(t, MoveLayerPayload{
		LayerID:       baseID,
		ParentLayerID: groupID,
		Index:         &moveIndex,
	}))
	if err != nil {
		t.Fatalf("move layer: %v", err)
	}
	groupMeta, ok := findLayerMetaByID(moved.UIMeta.Layers, groupID)
	if !ok {
		t.Fatalf("group %q not found in layer tree", groupID)
	}
	if len(groupMeta.Children) != 1 {
		t.Fatalf("group child count = %d, want 1", len(groupMeta.Children))
	}

	dup, err := DispatchCommand(h, commandDuplicateLayer, mustJSON(t, DuplicateLayerPayload{LayerID: baseID}))
	if err != nil {
		t.Fatalf("duplicate layer: %v", err)
	}
	if dup.UIMeta.ActiveLayerID == baseID {
		t.Fatal("duplicate layer reused the original id")
	}

	opacity := 0.5
	fillOpacity := 0.75
	updated, err := DispatchCommand(h, commandSetLayerOp, mustJSON(t, SetLayerOpacityPayload{
		LayerID:     dup.UIMeta.ActiveLayerID,
		Opacity:     &opacity,
		FillOpacity: &fillOpacity,
	}))
	if err != nil {
		t.Fatalf("set opacity: %v", err)
	}
	duplicatedLayer, ok := findLayerMetaByID(updated.UIMeta.Layers, dup.UIMeta.ActiveLayerID)
	if !ok {
		t.Fatalf("duplicated layer %q not found", dup.UIMeta.ActiveLayerID)
	}
	if duplicatedLayer.Opacity != 0.5 || duplicatedLayer.FillOpacity != 0.75 {
		t.Fatalf("duplicated layer opacity = %.2f/%.2f, want 0.5/0.75", duplicatedLayer.Opacity, duplicatedLayer.FillOpacity)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	undoneLayer, ok := findLayerMetaByID(undone.UIMeta.Layers, dup.UIMeta.ActiveLayerID)
	if !ok {
		t.Fatalf("duplicated layer %q missing after undo", dup.UIMeta.ActiveLayerID)
	}
	if undoneLayer.Opacity != 1 || undoneLayer.FillOpacity != 1 {
		t.Fatal("undo did not restore layer opacity defaults")
	}
	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo: %v", err)
	}
	redoneLayer, ok := findLayerMetaByID(redone.UIMeta.Layers, dup.UIMeta.ActiveLayerID)
	if !ok {
		t.Fatalf("duplicated layer %q missing after redo", dup.UIMeta.ActiveLayerID)
	}
	if redoneLayer.Opacity != 0.5 || redoneLayer.FillOpacity != 0.75 {
		t.Fatal("redo did not reapply layer opacity")
	}
}

func TestFlattenMergeDownAndMergeVisible(t *testing.T) {
	h := Init("")
	defer Free(h)

	text, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:    LayerTypeText,
		Name:         "Headline",
		Bounds:       LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Text:         "A",
		CachedRaster: []byte{255, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add text layer: %v", err)
	}
	textID := text.UIMeta.ActiveLayerID

	flattened, err := DispatchCommand(h, commandFlattenLayer, mustJSON(t, FlattenLayerPayload{LayerID: textID}))
	if err != nil {
		t.Fatalf("flatten text layer: %v", err)
	}
	if flattened.UIMeta.Layers[0].LayerType != LayerTypePixel {
		t.Fatalf("flattened layer type = %q, want %q", flattened.UIMeta.Layers[0].LayerType, LayerTypePixel)
	}

	first, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Bottom",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{0, 0, 255, 255},
	}))
	if err != nil {
		t.Fatalf("add bottom layer: %v", err)
	}
	bottomID := first.UIMeta.ActiveLayerID
	second, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Top",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{255, 255, 0, 128},
	}))
	if err != nil {
		t.Fatalf("add top layer: %v", err)
	}
	topID := second.UIMeta.ActiveLayerID

	mergedDown, err := DispatchCommand(h, commandMergeDown, mustJSON(t, MergeDownPayload{LayerID: topID}))
	if err != nil {
		t.Fatalf("merge down: %v", err)
	}
	if len(mergedDown.UIMeta.Layers) != 2 {
		t.Fatalf("layer count after merge down = %d, want 2", len(mergedDown.UIMeta.Layers))
	}
	if mergedDown.UIMeta.ActiveLayerID == topID || mergedDown.UIMeta.ActiveLayerID == bottomID {
		t.Fatal("merge down should create a new merged layer id")
	}

	if _, err := DispatchCommand(h, commandSetLayerBlend, mustJSON(t, SetLayerBlendModePayload{
		LayerID:   mergedDown.UIMeta.ActiveLayerID,
		BlendMode: BlendModeMultiply,
	})); err != nil {
		t.Fatalf("set merged layer blend mode: %v", err)
	}

	hidden, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Hidden",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{1, 2, 3, 255},
	}))
	if err != nil {
		t.Fatalf("add hidden layer: %v", err)
	}
	hiddenID := hidden.UIMeta.ActiveLayerID
	if _, err := DispatchCommand(h, commandSetLayerVis, mustJSON(t, SetLayerVisibilityPayload{LayerID: hiddenID, Visible: false})); err != nil {
		t.Fatalf("hide layer: %v", err)
	}

	mergedVisible, err := DispatchCommand(h, commandMergeVisible, "")
	if err != nil {
		t.Fatalf("merge visible: %v", err)
	}
	if len(mergedVisible.UIMeta.Layers) != 2 {
		t.Fatalf("layer count after merge visible = %d, want 2", len(mergedVisible.UIMeta.Layers))
	}
	hiddenMeta, ok := findLayerMetaByID(mergedVisible.UIMeta.Layers, hiddenID)
	if !ok {
		t.Fatalf("hidden layer %q missing after merge visible", hiddenID)
	}
	if hiddenMeta.Visible {
		t.Fatal("hidden layer should remain hidden after merge visible")
	}
}

func TestFlattenAndMergeSupportNonNormalBlendModes(t *testing.T) {
	h := Init("")
	defer Free(h)

	bottom, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Bottom",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{128, 128, 128, 255},
	}))
	if err != nil {
		t.Fatalf("add bottom: %v", err)
	}

	top, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Top",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{255, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add top: %v", err)
	}
	if _, err := DispatchCommand(h, commandSetLayerBlend, mustJSON(t, SetLayerBlendModePayload{
		LayerID:   top.UIMeta.ActiveLayerID,
		BlendMode: BlendModeScreen,
	})); err != nil {
		t.Fatalf("set top blend mode: %v", err)
	}

	merged, err := DispatchCommand(h, commandMergeDown, mustJSON(t, MergeDownPayload{LayerID: top.UIMeta.ActiveLayerID}))
	if err != nil {
		t.Fatalf("merge down with blend mode: %v", err)
	}
	if merged.UIMeta.ActiveLayerID == bottom.UIMeta.ActiveLayerID {
		t.Fatal("merge down should create a new layer for blended output")
	}

	text, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:    LayerTypeText,
		Name:         "Glow",
		Bounds:       LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Text:         "A",
		CachedRaster: []byte{0, 0, 255, 255},
	}))
	if err != nil {
		t.Fatalf("add text: %v", err)
	}
	if _, err := DispatchCommand(h, commandSetLayerBlend, mustJSON(t, SetLayerBlendModePayload{
		LayerID:   text.UIMeta.ActiveLayerID,
		BlendMode: BlendModeOverlay,
	})); err != nil {
		t.Fatalf("set text blend mode: %v", err)
	}
	flattened, err := DispatchCommand(h, commandFlattenLayer, mustJSON(t, FlattenLayerPayload{LayerID: text.UIMeta.ActiveLayerID}))
	if err != nil {
		t.Fatalf("flatten with blend mode: %v", err)
	}
	flattenedLayer, ok := findLayerMetaByID(flattened.UIMeta.Layers, flattened.UIMeta.ActiveLayerID)
	if !ok {
		t.Fatalf("flattened layer %q missing", flattened.UIMeta.ActiveLayerID)
	}
	if flattenedLayer.LayerType != LayerTypePixel {
		t.Fatalf("flattened layer type = %q, want %q", flattenedLayer.LayerType, LayerTypePixel)
	}
}

func TestRenderLayerToSurfaceAppliesRasterMask(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	layer := NewPixelLayer("Masked", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	layer.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{255, 0}})

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render masked layer: %v", err)
	}

	if surface[0] != 255 || surface[1] != 0 || surface[2] != 0 || surface[3] != 255 {
		t.Fatalf("first pixel = %v, want opaque red", surface[:4])
	}
	if surface[4] != 0 || surface[5] != 0 || surface[6] != 0 || surface[7] != 0 {
		t.Fatalf("second pixel = %v, want fully masked out", surface[4:8])
	}
}

func TestRenderLayerToSurfaceAppliesGroupRasterMask(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	child := NewPixelLayer("Child", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		0, 255, 0, 255,
		0, 255, 0, 255,
	})
	group := NewGroupLayer("Group")
	group.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{255, 0}})
	group.SetChildren([]LayerNode{child})

	surface, err := doc.renderLayerToSurface(group)
	if err != nil {
		t.Fatalf("render masked group: %v", err)
	}

	if surface[0] != 0 || surface[1] != 255 || surface[2] != 0 || surface[3] != 255 {
		t.Fatalf("first pixel = %v, want opaque green", surface[:4])
	}
	if surface[4] != 0 || surface[5] != 0 || surface[6] != 0 || surface[7] != 0 {
		t.Fatalf("second pixel = %v, want fully masked out", surface[4:8])
	}
}

func TestLayerMaskOperations(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	layer := NewPixelLayer("Masked", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	doc.LayerRoot.SetChildren([]LayerNode{layer})

	if err := doc.AddLayerMask(layer.ID(), AddLayerMaskHideAll); err != nil {
		t.Fatalf("add mask: %v", err)
	}
	mask := layer.Mask()
	if mask == nil || len(mask.Data) != 2 || mask.Data[0] != 0 || mask.Data[1] != 0 {
		t.Fatalf("mask = %#v, want 2 hidden pixels", mask)
	}

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render hidden mask: %v", err)
	}
	if surface[3] != 0 || surface[7] != 0 {
		t.Fatalf("hidden mask alpha = [%d %d], want [0 0]", surface[3], surface[7])
	}

	if err := doc.InvertLayerMask(layer.ID()); err != nil {
		t.Fatalf("invert mask: %v", err)
	}
	surface, err = doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render inverted mask: %v", err)
	}
	if surface[3] != 255 || surface[7] != 255 {
		t.Fatalf("inverted mask alpha = [%d %d], want [255 255]", surface[3], surface[7])
	}

	partial := layer.Mask()
	partial.Data = []byte{255, 0}
	layer.SetMask(partial)
	if err := doc.SetLayerMaskEnabled(layer.ID(), false); err != nil {
		t.Fatalf("disable mask: %v", err)
	}
	surface, err = doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render disabled mask: %v", err)
	}
	if surface[3] != 255 || surface[7] != 255 {
		t.Fatalf("disabled mask alpha = [%d %d], want [255 255]", surface[3], surface[7])
	}

	if err := doc.SetLayerMaskEnabled(layer.ID(), true); err != nil {
		t.Fatalf("enable mask: %v", err)
	}
	surface, err = doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render enabled mask: %v", err)
	}
	if surface[3] != 255 || surface[7] != 0 {
		t.Fatalf("enabled mask alpha = [%d %d], want [255 0]", surface[3], surface[7])
	}

	if err := doc.DeleteLayerMask(layer.ID()); err != nil {
		t.Fatalf("delete mask: %v", err)
	}
	if layer.Mask() != nil {
		t.Fatal("mask should be deleted")
	}
	if _, err := doc.renderLayerToSurface(layer); err != nil {
		t.Fatalf("render after delete: %v", err)
	}
}

func TestApplyLayerMaskBakesAlpha(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	layer := NewPixelLayer("Masked", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	layer.SetMask(&LayerMask{Enabled: false, Width: 2, Height: 1, Data: []byte{255, 0}})
	doc.LayerRoot.SetChildren([]LayerNode{layer})

	if err := doc.ApplyLayerMask(layer.ID()); err != nil {
		t.Fatalf("apply mask: %v", err)
	}
	if layer.Mask() != nil {
		t.Fatal("mask should be removed after apply")
	}
	if layer.Pixels[3] != 255 || layer.Pixels[7] != 0 {
		t.Fatalf("applied pixel alpha = [%d %d], want [255 0]", layer.Pixels[3], layer.Pixels[7])
	}
}

func TestAddLayerMaskFromSelectionRequiresSelectionEngine(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	layer := NewPixelLayer("Masked", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{255, 0, 0, 255})
	doc.LayerRoot.SetChildren([]LayerNode{layer})

	if err := doc.AddLayerMask(layer.ID(), AddLayerMaskFromSelection); err == nil {
		t.Fatal("expected from-selection mask creation to fail before selections exist")
	}
}

func TestClipToBelowCompositesAgainstBaseAlpha(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		0, 0, 255, 255,
		0, 0, 255, 0,
	})
	top := NewPixelLayer("Top", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	doc.LayerRoot.SetChildren([]LayerNode{base, top})

	if err := doc.SetLayerClipToBelow(top.ID(), true); err != nil {
		t.Fatalf("set clip to below: %v", err)
	}
	if !base.ClippingBase() || !top.ClipToBelow() {
		t.Fatalf("unexpected clipping flags: base=%v top=%v", base.ClippingBase(), top.ClipToBelow())
	}

	surface, err := doc.renderLayersToSurface(doc.LayerRoot.Children())
	if err != nil {
		t.Fatalf("render clipped stack: %v", err)
	}
	if surface[0] != 255 || surface[1] != 0 || surface[2] != 0 || surface[3] != 255 {
		t.Fatalf("first pixel = %v, want opaque red from clipped layer", surface[:4])
	}
	if surface[4] != 0 || surface[5] != 0 || surface[6] != 0 || surface[7] != 0 {
		t.Fatalf("second pixel = %v, want fully clipped transparent pixel", surface[4:8])
	}

	merged, err := doc.mergeNodesToPixelLayer(base, top, "Merged")
	if err != nil {
		t.Fatalf("merge clipped pair: %v", err)
	}
	if merged.Pixels[3] != 255 || merged.Pixels[7] != 0 {
		t.Fatalf("merged alpha = [%d %d], want [255 0]", merged.Pixels[3], merged.Pixels[7])
	}
}

func TestClipToBelowRequiresBaseLayer(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	layer := NewPixelLayer("Only", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{255, 0, 0, 255})
	doc.LayerRoot.SetChildren([]LayerNode{layer})

	if err := doc.SetLayerClipToBelow(layer.ID(), true); err == nil {
		t.Fatal("expected clip-to-below to fail without a base layer")
	}
}

func findLayerMetaByID(layers []LayerNodeMeta, targetID string) (LayerNodeMeta, bool) {
	for _, layer := range layers {
		if layer.ID == targetID {
			return layer, true
		}
		if child, ok := findLayerMetaByID(layer.Children, targetID); ok {
			return child, true
		}
	}
	return LayerNodeMeta{}, false
}
