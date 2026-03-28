package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
)

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

func TestSetActiveLayerAndImplicitEmptyPixels(t *testing.T) {
	h := Init("")
	defer Free(h)

	base, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
	}))
	if err != nil {
		t.Fatalf("add pixel layer without pixels: %v", err)
	}
	baseID := base.UIMeta.ActiveLayerID

	group, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeGroup,
		Name:      "Group",
	}))
	if err != nil {
		t.Fatalf("add group: %v", err)
	}

	selected, err := DispatchCommand(h, commandSetActiveLayer, mustJSON(t, SetActiveLayerPayload{LayerID: baseID}))
	if err != nil {
		t.Fatalf("set active layer: %v", err)
	}
	if selected.UIMeta.ActiveLayerID != baseID {
		t.Fatalf("active layer id = %q, want %q", selected.UIMeta.ActiveLayerID, baseID)
	}

	frame, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("render frame: %v", err)
	}
	if frame.BufferLen == 0 {
		t.Fatal("render frame returned an empty buffer")
	}
	if frame.UIMeta.ActiveLayerID != baseID {
		t.Fatalf("render active layer id = %q, want %q", frame.UIMeta.ActiveLayerID, baseID)
	}
	if len(frame.UIMeta.Layers) != 2 || frame.UIMeta.ActiveLayerName == group.UIMeta.ActiveLayerName {
		t.Fatal("layer selection did not persist correctly across render")
	}
}

func TestRenameLayerSupportsUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Sketch",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	renamed, err := DispatchCommand(h, commandSetLayerName, mustJSON(t, SetLayerNamePayload{
		LayerID: layerID,
		Name:    "Headline",
	}))
	if err != nil {
		t.Fatalf("rename layer: %v", err)
	}
	meta, ok := findLayerMetaByID(renamed.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("renamed layer %q missing", layerID)
	}
	if meta.Name != "Headline" || renamed.UIMeta.ActiveLayerName != "Headline" {
		t.Fatalf("renamed layer name = %q / %q, want Headline", meta.Name, renamed.UIMeta.ActiveLayerName)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo rename: %v", err)
	}
	meta, ok = findLayerMetaByID(undone.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q missing after undo", layerID)
	}
	if meta.Name != "Sketch" {
		t.Fatalf("layer name after undo = %q, want Sketch", meta.Name)
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo rename: %v", err)
	}
	meta, ok = findLayerMetaByID(redone.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q missing after redo", layerID)
	}
	if meta.Name != "Headline" {
		t.Fatalf("layer name after redo = %q, want Headline", meta.Name)
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

func TestDeleteLayerCommandSelectsNextSiblingAndSupportsUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	first, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "First",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{1, 2, 3, 255},
	}))
	if err != nil {
		t.Fatalf("add first layer: %v", err)
	}
	firstID := first.UIMeta.ActiveLayerID

	middle, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Middle",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{4, 5, 6, 255},
	}))
	if err != nil {
		t.Fatalf("add middle layer: %v", err)
	}
	middleID := middle.UIMeta.ActiveLayerID

	last, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Last",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{7, 8, 9, 255},
	}))
	if err != nil {
		t.Fatalf("add last layer: %v", err)
	}
	lastID := last.UIMeta.ActiveLayerID

	selected, err := DispatchCommand(h, commandSetActiveLayer, mustJSON(t, SetActiveLayerPayload{LayerID: middleID}))
	if err != nil {
		t.Fatalf("set active layer: %v", err)
	}
	if selected.UIMeta.ActiveLayerID != middleID {
		t.Fatalf("active layer id = %q, want %q", selected.UIMeta.ActiveLayerID, middleID)
	}

	deleted, err := DispatchCommand(h, commandDeleteLayer, mustJSON(t, DeleteLayerPayload{LayerID: middleID}))
	if err != nil {
		t.Fatalf("delete middle layer: %v", err)
	}
	if deleted.UIMeta.ActiveLayerID != lastID {
		t.Fatalf("active layer after delete = %q, want %q", deleted.UIMeta.ActiveLayerID, lastID)
	}
	if _, ok := findLayerMetaByID(deleted.UIMeta.Layers, middleID); ok {
		t.Fatalf("deleted layer %q still present after delete", middleID)
	}
	if _, ok := findLayerMetaByID(deleted.UIMeta.Layers, firstID); !ok {
		t.Fatalf("first layer %q missing after delete", firstID)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo delete layer: %v", err)
	}
	if undone.UIMeta.ActiveLayerID != middleID {
		t.Fatalf("active layer after undo = %q, want %q", undone.UIMeta.ActiveLayerID, middleID)
	}
	if _, ok := findLayerMetaByID(undone.UIMeta.Layers, middleID); !ok {
		t.Fatalf("layer %q missing after undo", middleID)
	}
}

func TestDeleteLayerSelectsPreviousSiblingWhenDeletingLastSibling(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	first := NewPixelLayer("First", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{1, 2, 3, 255})
	last := NewPixelLayer("Last", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{4, 5, 6, 255})
	doc.LayerRoot.SetChildren([]LayerNode{first, last})
	doc.ActiveLayerID = last.ID()

	if got := len(doc.Layers()); got != 2 {
		t.Fatalf("len(Layers()) = %d, want 2", got)
	}
	if err := doc.DeleteLayer(last.ID()); err != nil {
		t.Fatalf("DeleteLayer(last): %v", err)
	}
	if doc.ActiveLayerID != first.ID() {
		t.Fatalf("active layer after deleting last sibling = %q, want %q", doc.ActiveLayerID, first.ID())
	}
	if got := len(doc.Layers()); got != 1 {
		t.Fatalf("len(Layers()) after delete = %d, want 1", got)
	}
}

func TestDeleteLayerSelectsParentWhenDeletingOnlyChild(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	group := NewGroupLayer("Group")
	child := NewPixelLayer("Child", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{1, 2, 3, 255})
	group.SetChildren([]LayerNode{child})
	doc.LayerRoot.SetChildren([]LayerNode{group})
	doc.ActiveLayerID = child.ID()

	if err := doc.DeleteLayer(child.ID()); err != nil {
		t.Fatalf("DeleteLayer(child): %v", err)
	}
	if doc.ActiveLayerID != group.ID() {
		t.Fatalf("active layer after deleting only child = %q, want %q", doc.ActiveLayerID, group.ID())
	}
	if got := len(group.Children()); got != 0 {
		t.Fatalf("group child count after delete = %d, want 0", got)
	}
}

func TestDeleteLayerClearsActiveWhenDeletingLastRootLayer(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	only := NewPixelLayer("Only", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{1, 2, 3, 255})
	doc.LayerRoot.SetChildren([]LayerNode{only})
	doc.ActiveLayerID = only.ID()

	if err := doc.DeleteLayer(only.ID()); err != nil {
		t.Fatalf("DeleteLayer(only): %v", err)
	}
	if doc.ActiveLayerID != "" {
		t.Fatalf("active layer after deleting last root layer = %q, want empty", doc.ActiveLayerID)
	}
	if got := len(doc.Layers()); got != 0 {
		t.Fatalf("len(Layers()) after deleting last root layer = %d, want 0", got)
	}
}

func TestDeleteLayerReturnsErrorForUnknownLayer(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	if err := doc.DeleteLayer("missing"); err == nil {
		t.Fatal("expected DeleteLayer to fail for an unknown layer")
	}
}

func TestSetLayerLockCommandUpdatesMetadataAndSupportsUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Locked",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{10, 20, 30, 255},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	locked, err := DispatchCommand(h, commandSetLayerLock, mustJSON(t, SetLayerLockPayload{
		LayerID:  layerID,
		LockMode: LayerLockAll,
	}))
	if err != nil {
		t.Fatalf("set layer lock: %v", err)
	}
	meta, ok := findLayerMetaByID(locked.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q not found after locking", layerID)
	}
	if meta.LockMode != LayerLockAll {
		t.Fatalf("lock mode after set = %q, want %q", meta.LockMode, LayerLockAll)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo lock change: %v", err)
	}
	meta, ok = findLayerMetaByID(undone.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q missing after undo", layerID)
	}
	if meta.LockMode != LayerLockNone {
		t.Fatalf("lock mode after undo = %q, want %q", meta.LockMode, LayerLockNone)
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo lock change: %v", err)
	}
	meta, ok = findLayerMetaByID(redone.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q missing after redo", layerID)
	}
	if meta.LockMode != LayerLockAll {
		t.Fatalf("lock mode after redo = %q, want %q", meta.LockMode, LayerLockAll)
	}

	doc := instances[h].manager.Active()
	active := doc.ActiveLayer()
	if active == nil || active.LockMode() != LayerLockAll {
		t.Fatalf("active layer lock mode = %v, want %q", active, LayerLockAll)
	}

	if err := doc.SetLayerLock("missing", LayerLockPixels); err == nil {
		t.Fatal("expected SetLayerLock to fail for an unknown layer")
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

func TestApplyLayerMaskSupportsTextAndVectorLayers(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	text := NewTextLayer("Text", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, "AB", []byte{
		10, 20, 30, 255,
		40, 50, 60, 255,
	})
	text.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{255, 0}})

	vector := NewVectorLayer("Vector", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, &Path{Closed: true, Points: []PathPoint{{X: 0, Y: 0}, {X: 2, Y: 0}}}, []byte{
		70, 80, 90, 255,
		100, 110, 120, 255,
	})
	vector.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{0, 255}})
	vector.SetParent(doc.LayerRoot)
	text.SetParent(doc.LayerRoot)
	doc.LayerRoot.SetChildren([]LayerNode{text, vector})

	if err := doc.ApplyLayerMask(text.ID()); err != nil {
		t.Fatalf("ApplyLayerMask(text): %v", err)
	}
	if text.Mask() != nil {
		t.Fatal("text mask should be removed after apply")
	}
	if text.CachedRaster[3] != 255 || text.CachedRaster[7] != 0 {
		t.Fatalf("text cached raster alpha = [%d %d], want [255 0]", text.CachedRaster[3], text.CachedRaster[7])
	}

	if err := doc.ApplyLayerMask(vector.ID()); err != nil {
		t.Fatalf("ApplyLayerMask(vector): %v", err)
	}
	if vector.Mask() != nil {
		t.Fatal("vector mask should be removed after apply")
	}
	if vector.CachedRaster[3] != 0 || vector.CachedRaster[7] != 255 {
		t.Fatalf("vector cached raster alpha = [%d %d], want [0 255]", vector.CachedRaster[3], vector.CachedRaster[7])
	}
}

func TestApplyLayerMaskRejectsUnsupportedTargetsAndInvalidRaster(t *testing.T) {
	t.Run("group layer", func(t *testing.T) {
		doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
		group := NewGroupLayer("Group")
		group.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{255, 255}})
		group.SetChildren([]LayerNode{NewPixelLayer("Child", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{1, 2, 3, 255, 4, 5, 6, 255})})
		doc.LayerRoot.SetChildren([]LayerNode{group})

		if err := doc.ApplyLayerMask(group.ID()); err == nil {
			t.Fatal("expected ApplyLayerMask to reject group layers")
		}
	})

	t.Run("adjustment layer", func(t *testing.T) {
		doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
		adjustment := NewAdjustmentLayer("Curves", "curves", json.RawMessage(`{"points":[[0,0],[255,255]]}`))
		adjustment.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{255, 255}})
		doc.LayerRoot.SetChildren([]LayerNode{adjustment})

		if err := doc.ApplyLayerMask(adjustment.ID()); err == nil {
			t.Fatal("expected ApplyLayerMask to reject adjustment layers")
		}
	})

	t.Run("invalid raster length", func(t *testing.T) {
		doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
		broken := NewPixelLayer("Broken", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{1, 2, 3, 255})
		broken.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{255, 255}})
		doc.LayerRoot.SetChildren([]LayerNode{broken})

		if err := doc.ApplyLayerMask(broken.ID()); err == nil {
			t.Fatal("expected ApplyLayerMask to fail for malformed raster data")
		}
	})

	t.Run("nil document", func(t *testing.T) {
		var doc *Document
		if err := doc.ApplyLayerMask("any"); err == nil {
			t.Fatal("expected ApplyLayerMask to fail for a nil document")
		}
	})

	t.Run("unknown layer", func(t *testing.T) {
		doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
		if err := doc.ApplyLayerMask("missing"); err == nil {
			t.Fatal("expected ApplyLayerMask to fail for an unknown layer")
		}
	})

	t.Run("layer without mask", func(t *testing.T) {
		doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
		plain := NewPixelLayer("Plain", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{1, 2, 3, 255, 4, 5, 6, 255})
		doc.LayerRoot.SetChildren([]LayerNode{plain})

		if err := doc.ApplyLayerMask(plain.ID()); err == nil {
			t.Fatal("expected ApplyLayerMask to fail when the layer has no mask")
		}
	})
}

func TestApplyMaskToLayerRasterAndMaskAlphaHelpers(t *testing.T) {
	t.Run("apply mask no-op inputs", func(t *testing.T) {
		raster := []byte{1, 2, 3, 255}
		if err := applyMaskToLayerRaster(LayerBounds{}, raster, &LayerMask{Enabled: true, Width: 1, Height: 1, Data: []byte{0}}); err != nil {
			t.Fatalf("applyMaskToLayerRaster with empty bounds: %v", err)
		}
		if err := applyMaskToLayerRaster(LayerBounds{X: 0, Y: 0, W: 1, H: 1}, nil, &LayerMask{Enabled: true, Width: 1, Height: 1, Data: []byte{0}}); err != nil {
			t.Fatalf("applyMaskToLayerRaster with empty raster: %v", err)
		}
		if err := applyMaskToLayerRaster(LayerBounds{X: 0, Y: 0, W: 1, H: 1}, raster, nil); err != nil {
			t.Fatalf("applyMaskToLayerRaster with nil mask: %v", err)
		}
	})

	t.Run("layerMaskDataAlphaAt branches", func(t *testing.T) {
		if got := layerMaskDataAlphaAt(nil, 0, 0); got != 255 {
			t.Fatalf("layerMaskDataAlphaAt(nil) = %d, want 255", got)
		}
		if got := layerMaskDataAlphaAt(&LayerMask{Enabled: true, Width: 0, Height: 1}, 0, 0); got != 255 {
			t.Fatalf("layerMaskDataAlphaAt(zero width) = %d, want 255", got)
		}
		if got := layerMaskDataAlphaAt(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{123}}, 0, 0); got != 255 {
			t.Fatalf("layerMaskDataAlphaAt(short data) = %d, want 255", got)
		}

		mask := &LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{77, 155}}
		if got := layerMaskDataAlphaAt(mask, -1, 0); got != 0 {
			t.Fatalf("layerMaskDataAlphaAt(negative x) = %d, want 0", got)
		}
		if got := layerMaskDataAlphaAt(mask, 2, 0); got != 0 {
			t.Fatalf("layerMaskDataAlphaAt(out of bounds x) = %d, want 0", got)
		}
		if got := layerMaskDataAlphaAt(mask, 1, 1); got != 0 {
			t.Fatalf("layerMaskDataAlphaAt(out of bounds y) = %d, want 0", got)
		}
		if got := layerMaskDataAlphaAt(mask, 1, 0); got != 155 {
			t.Fatalf("layerMaskDataAlphaAt(valid) = %d, want 155", got)
		}
	})
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

func TestClippingBaseSurfaceForLayerAndClipHelpers(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	group := NewGroupLayer("Group")
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		0, 0, 255, 255,
		0, 0, 255, 0,
	})
	clipped := NewPixelLayer("Clipped", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	group.SetChildren([]LayerNode{base, clipped})
	doc.LayerRoot.SetChildren([]LayerNode{group})

	if err := doc.SetLayerClipToBelow(clipped.ID(), true); err != nil {
		t.Fatalf("SetLayerClipToBelow: %v", err)
	}

	expectedBaseSurface, err := doc.renderLayerToSurface(base)
	if err != nil {
		t.Fatalf("renderLayerToSurface(base): %v", err)
	}
	clipSurface, err := doc.clippingBaseSurfaceForLayer(clipped)
	if err != nil {
		t.Fatalf("clippingBaseSurfaceForLayer(clipped): %v", err)
	}
	if !bytes.Equal(clipSurface, expectedBaseSurface) {
		t.Fatalf("clipping base surface = %v, want %v", clipSurface, expectedBaseSurface)
	}

	var nilDoc *Document
	if got, err := nilDoc.clippingBaseSurfaceForLayer(clipped); err != nil || got != nil {
		t.Fatalf("nil doc clipping base surface = %v, %v, want nil, nil", got, err)
	}
	if got, err := doc.clippingBaseSurfaceForLayer(base); err != nil || got != nil {
		t.Fatalf("unclipped layer clipping base surface = %v, %v, want nil, nil", got, err)
	}

	externalClip := []byte{
		0, 0, 0, 128,
		0, 0, 0, 255,
	}
	combined := combineClipSurface(expectedBaseSurface, externalClip)
	if bytes.Equal(combined, expectedBaseSurface) {
		t.Fatal("combineClipSurface should create a clipped copy when both surfaces are present")
	}
	if combined[3] != 128 || combined[7] != 0 {
		t.Fatalf("combined clip alpha = [%d %d], want [128 0]", combined[3], combined[7])
	}
	if expectedBaseSurface[3] != 255 || expectedBaseSurface[7] != 0 {
		t.Fatalf("base surface should remain unchanged, got alpha [%d %d]", expectedBaseSurface[3], expectedBaseSurface[7])
	}

	surface := []byte{
		9, 9, 9, 255,
		9, 9, 9, 255,
	}
	applyClipSurfaceToSurface(surface, combined)
	if surface[3] != 128 || surface[7] != 0 {
		t.Fatalf("applyClipSurfaceToSurface alpha = [%d %d], want [128 0]", surface[3], surface[7])
	}

	unchanged := []byte{1, 2, 3, 255}
	applyClipSurfaceToSurface(unchanged, []byte{1, 2, 3})
	if unchanged[3] != 255 {
		t.Fatalf("mismatched clip length should leave surface unchanged, got alpha %d", unchanged[3])
	}

	if got := combineClipSurface(nil, externalClip); !bytes.Equal(got, externalClip) {
		t.Fatalf("combineClipSurface(nil, clip) = %v, want %v", got, externalClip)
	}
	if got := combineClipSurface(expectedBaseSurface, nil); !bytes.Equal(got, expectedBaseSurface) {
		t.Fatalf("combineClipSurface(base, nil) = %v, want %v", got, expectedBaseSurface)
	}
}

func TestFlattenImageCommandDiscardsHiddenLayersAndSupportsUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Flatten Image",
		Width:      2,
		Height:     2,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "transparent",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels: []byte{
			10, 20, 30, 255,
			10, 20, 30, 255,
			10, 20, 30, 255,
			10, 20, 30, 255,
		},
	})); err != nil {
		t.Fatalf("add base layer: %v", err)
	}

	hidden, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Hidden",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    filledPixels(2, 2, [4]byte{200, 10, 10, 255}),
	}))
	if err != nil {
		t.Fatalf("add hidden layer: %v", err)
	}
	hiddenID := hidden.UIMeta.ActiveLayerID
	if _, err := DispatchCommand(h, commandSetLayerVis, mustJSON(t, SetLayerVisibilityPayload{LayerID: hiddenID, Visible: false})); err != nil {
		t.Fatalf("hide layer: %v", err)
	}

	group, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeGroup,
		Name:      "Group",
	}))
	if err != nil {
		t.Fatalf("add group: %v", err)
	}
	groupID := group.UIMeta.ActiveLayerID

	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:     LayerTypePixel,
		Name:          "Group Red",
		ParentLayerID: groupID,
		Bounds:        LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:        []byte{255, 0, 0, 255},
	})); err != nil {
		t.Fatalf("add group red layer: %v", err)
	}
	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:     LayerTypePixel,
		Name:          "Group Blue",
		ParentLayerID: groupID,
		Bounds:        LayerBounds{X: 1, Y: 0, W: 1, H: 1},
		Pixels:        []byte{0, 0, 255, 255},
	})); err != nil {
		t.Fatalf("add group blue layer: %v", err)
	}

	expectedSurface := append([]byte(nil), instances[h].manager.Active().renderCompositeSurface()...)

	flattened, err := DispatchCommand(h, commandFlattenImage, "")
	if err != nil {
		t.Fatalf("flatten image: %v", err)
	}
	if len(flattened.UIMeta.Layers) != 1 {
		t.Fatalf("flattened root layer count = %d, want 1", len(flattened.UIMeta.Layers))
	}
	if flattened.UIMeta.Layers[0].Name != "Background" || flattened.UIMeta.Layers[0].LayerType != LayerTypePixel {
		t.Fatalf("flattened layer meta = %+v, want a Background pixel layer", flattened.UIMeta.Layers[0])
	}

	activeDoc := instances[h].manager.Active()
	children := activeDoc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("active document child count = %d, want 1", len(children))
	}
	flattenedLayer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("flattened layer type = %T, want *PixelLayer", children[0])
	}
	if !bytes.Equal(flattenedLayer.Pixels, expectedSurface) {
		t.Fatalf("flattened pixels = %v, want %v", flattenedLayer.Pixels, expectedSurface)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo flatten image: %v", err)
	}
	if _, ok := findLayerMetaByID(undone.UIMeta.Layers, hiddenID); !ok {
		t.Fatalf("hidden layer %q missing after undo", hiddenID)
	}
	hiddenMeta, _ := findLayerMetaByID(undone.UIMeta.Layers, hiddenID)
	if hiddenMeta.Visible {
		t.Fatal("hidden layer should remain hidden after undoing flatten image")
	}
}

func TestFlattenImageFailsWithoutVisibleLayers(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	hidden := NewPixelLayer("Hidden", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{1, 2, 3, 255})
	hidden.SetVisible(false)
	doc.LayerRoot.SetChildren([]LayerNode{hidden})

	if err := doc.FlattenImage(); err == nil {
		t.Fatal("expected flatten image to fail without visible layers")
	}
}

func TestGenerateAllThumbnailsIncludesMixedLayerTypesAndMasks(t *testing.T) {
	doc := &Document{Width: 2, Height: 2, LayerRoot: NewGroupLayer("Root")}
	pixel := NewPixelLayer("Pixel", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	pixel.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 2, Data: []byte{255, 0, 0, 255}})

	text := NewTextLayer("Text", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, "A", []byte{0, 255, 0, 255})
	vector := NewVectorLayer("Vector", LayerBounds{X: 1, Y: 0, W: 1, H: 1}, &Path{Closed: true, Points: []PathPoint{{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 1}}}, []byte{0, 0, 255, 255})
	group := NewGroupLayer("Group")
	group.SetChildren([]LayerNode{text, vector})
	doc.LayerRoot.SetChildren([]LayerNode{pixel, group})

	thumbs, err := doc.generateAllThumbnails(2, 2)
	if err != nil {
		t.Fatalf("generateAllThumbnails: %v", err)
	}
	if len(thumbs) != 4 {
		t.Fatalf("thumbnail count = %d, want 4", len(thumbs))
	}

	pixelThumb := mustDecodeThumbnail(t, thumbs[pixel.ID()].LayerRGBA)
	if pixelThumb[0] != 255 || pixelThumb[1] != 0 || pixelThumb[2] != 0 || pixelThumb[3] != 255 {
		t.Fatalf("pixel thumbnail first pixel = %v, want opaque red", pixelThumb[:4])
	}
	maskThumb := mustDecodeThumbnail(t, thumbs[pixel.ID()].MaskRGBA)
	if maskThumb[0] != 255 || maskThumb[1] != 255 || maskThumb[2] != 255 || maskThumb[3] != 255 {
		t.Fatalf("mask thumbnail first pixel = %v, want opaque white", maskThumb[:4])
	}
	if maskThumb[4] != 0 || maskThumb[5] != 0 || maskThumb[6] != 0 || maskThumb[7] != 255 {
		t.Fatalf("mask thumbnail second pixel = %v, want opaque black", maskThumb[4:8])
	}

	textThumb := mustDecodeThumbnail(t, thumbs[text.ID()].LayerRGBA)
	if textThumb[0] != 0 || textThumb[1] != 255 || textThumb[2] != 0 || textThumb[3] != 255 {
		t.Fatalf("text thumbnail first pixel = %v, want opaque green", textThumb[:4])
	}

	vectorThumb := mustDecodeThumbnail(t, thumbs[vector.ID()].LayerRGBA)
	if vectorThumb[0] != 0 || vectorThumb[1] != 0 || vectorThumb[2] != 255 || vectorThumb[3] != 255 {
		t.Fatalf("vector thumbnail first pixel = %v, want opaque blue", vectorThumb[:4])
	}

	groupThumb := mustDecodeThumbnail(t, thumbs[group.ID()].LayerRGBA)
	if groupThumb[0] != 0 || groupThumb[1] != 255 || groupThumb[2] != 0 || groupThumb[3] != 255 {
		t.Fatalf("group thumbnail first pixel = %v, want opaque green", groupThumb[:4])
	}
	if groupThumb[4] != 0 || groupThumb[5] != 0 || groupThumb[6] != 255 || groupThumb[7] != 255 {
		t.Fatalf("group thumbnail second pixel = %v, want opaque blue", groupThumb[4:8])
	}
}

func TestGetLayerThumbnailsCommandReturnsEntriesWithoutHistoryMutation(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Thumbs",
		Width:      2,
		Height:     2,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "transparent",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}
	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Masked",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    filledPixels(2, 2, [4]byte{50, 60, 70, 255}),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID
	if _, err := DispatchCommand(h, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{LayerID: layerID, Mode: AddLayerMaskRevealAll})); err != nil {
		t.Fatalf("add layer mask: %v", err)
	}

	historyBefore := instances[h].history.CurrentIndex()
	result, err := DispatchCommand(h, commandGetLayerThumbnails, "")
	if err != nil {
		t.Fatalf("get layer thumbnails: %v", err)
	}
	if instances[h].history.CurrentIndex() != historyBefore {
		t.Fatal("get layer thumbnails should not add a history entry")
	}
	entry, ok := result.Thumbnails[layerID]
	if !ok {
		t.Fatalf("missing thumbnail for layer %q", layerID)
	}
	if got := len(mustDecodeThumbnail(t, entry.LayerRGBA)); got != thumbnailSize*thumbnailSize*4 {
		t.Fatalf("layer thumbnail length = %d, want %d", got, thumbnailSize*thumbnailSize*4)
	}
	if got := len(mustDecodeThumbnail(t, entry.MaskRGBA)); got != thumbnailSize*thumbnailSize*4 {
		t.Fatalf("mask thumbnail length = %d, want %d", got, thumbnailSize*thumbnailSize*4)
	}
}

func TestFlattenLayerTreeAndScaleHelpers(t *testing.T) {
	first := NewPixelLayer("First", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{1, 2, 3, 4})
	childA := NewPixelLayer("Child A", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{5, 6, 7, 8})
	childB := NewPixelLayer("Child B", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{9, 10, 11, 12})
	group := NewGroupLayer("Group")
	group.SetChildren([]LayerNode{childA, childB})

	flattened := flattenLayerTree([]LayerNode{first, group})
	if got, want := len(flattened), 4; got != want {
		t.Fatalf("flattened layer count = %d, want %d", got, want)
	}
	if flattened[0].ID() != first.ID() || flattened[1].ID() != group.ID() || flattened[2].ID() != childA.ID() || flattened[3].ID() != childB.ID() {
		t.Fatalf("unexpected flatten order: [%s %s %s %s]", flattened[0].ID(), flattened[1].ID(), flattened[2].ID(), flattened[3].ID())
	}

	scaledRGBA := scaleRGBA([]byte{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}, 2, 2, 1, 1)
	if !bytes.Equal(scaledRGBA, []byte{1, 2, 3, 4}) {
		t.Fatalf("scaled RGBA = %v, want [1 2 3 4]", scaledRGBA)
	}

	scaledGray := scaleGrayToRGBA([]byte{255, 0, 0, 128}, 2, 2, 1, 1)
	if !bytes.Equal(scaledGray, []byte{255, 255, 255, 255}) {
		t.Fatalf("scaled gray RGBA = %v, want [255 255 255 255]", scaledGray)
	}
}

func TestTranslateLayerRecursesIntoGroupsAndHonorsPositionLock(t *testing.T) {
	doc := &Document{
		Width:      16,
		Height:     16,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Translate Test",
	}
	root := doc.ensureLayerRoot()
	group := NewGroupLayer("Group")
	pixel := NewPixelLayer("Pixel", LayerBounds{X: 2, Y: 3, W: 1, H: 1}, []byte{255, 0, 0, 255})
	text := NewTextLayer("Text", LayerBounds{X: 6, Y: 7, W: 1, H: 1}, "A", []byte{0, 0, 0, 255})
	group.SetChildren([]LayerNode{pixel, text})
	if err := doc.AddLayer(group, root.ID(), -1); err != nil {
		t.Fatalf("AddLayer(group): %v", err)
	}

	if err := doc.TranslateLayer(group.ID(), 4, -2); err != nil {
		t.Fatalf("TranslateLayer(group): %v", err)
	}
	if pixel.Bounds.X != 6 || pixel.Bounds.Y != 1 {
		t.Fatalf("pixel bounds = %+v, want {X:6 Y:1 W:1 H:1}", pixel.Bounds)
	}
	if text.Bounds.X != 10 || text.Bounds.Y != 5 {
		t.Fatalf("text bounds = %+v, want {X:10 Y:5 W:1 H:1}", text.Bounds)
	}

	pixel.SetLockMode(LayerLockPosition)
	if err := doc.TranslateLayer(pixel.ID(), 1, 0); err == nil {
		t.Fatal("TranslateLayer on a position-locked layer should fail")
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

func mustDecodeThumbnail(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("DecodeString(%q): %v", value, err)
	}
	return decoded
}
