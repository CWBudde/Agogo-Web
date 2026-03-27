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
