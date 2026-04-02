package engine

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestCompositeSurfaceCacheReuseOnViewportChange(t *testing.T) {
	h := Init("")
	defer Free(h)

	// Create a document with a coloured layer so the surface is non-trivial.
	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "CacheTest", Width: 8, Height: 8,
	}))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	_, err = DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{CanvasW: 8, CanvasH: 8, DevicePixelRatio: 1}))
	if err != nil {
		t.Fatalf("resize: %v", err)
	}

	// Prime the cache.
	inst := instances[h]
	doc := inst.manager.Active()
	surface1 := inst.compositeSurface(doc)

	// Viewport-only change: pan without touching layers.
	inst.viewport.CenterX = 10

	// Cache should still be valid because ContentVersion hasn't changed.
	doc2 := inst.manager.Active()
	surface2 := inst.compositeSurface(doc2)
	if &surface1[0] != &surface2[0] {
		t.Error("expected cache to be reused after viewport-only change, but got a new allocation")
	}
}

func TestCompositeSurfaceCacheInvalidateOnLayerChange(t *testing.T) {
	h := Init("")
	defer Free(h)

	_, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "CacheInvalidate", Width: 8, Height: 8,
	}))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	_, err = DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{CanvasW: 8, CanvasH: 8, DevicePixelRatio: 1}))
	if err != nil {
		t.Fatalf("resize: %v", err)
	}

	// Prime the cache.
	inst := instances[h]
	doc := inst.manager.Active()
	surface1 := inst.compositeSurface(doc)
	firstPtr := &surface1[0]

	// Mutate the document (add a layer), which changes ModifiedAt.
	_, err = DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Red",
		Bounds:    LayerBounds{W: 8, H: 8},
		Pixels:    filledPixels(8, 8, [4]byte{255, 0, 0, 255}),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Cache must be invalidated; the new surface has different content.
	doc2 := inst.manager.Active()
	surface2 := inst.compositeSurface(doc2)
	if &surface2[0] == firstPtr {
		t.Error("expected cache to be invalidated after layer change, but old surface was reused")
	}
}

func TestDispatchCommandRejectsInvalidPayloadAndUnsupportedCommand(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandZoomSet, "{"); err == nil {
		t.Fatal("expected invalid JSON payload to fail")
	} else if !strings.Contains(err.Error(), "decode payload") {
		t.Fatalf("invalid payload error = %q, want decode payload context", err)
	}

	if _, err := DispatchCommand(h, 0x7fff, ""); err == nil {
		t.Fatal("expected unsupported command id to fail")
	} else if !strings.Contains(err.Error(), "unsupported command id") {
		t.Fatalf("unsupported command error = %q, want unsupported command id context", err)
	}
}

func TestGetBufferPtrAndFreePointerBehavior(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	if got := GetBufferPtr(h); got != 0 {
		t.Fatalf("GetBufferPtr before render = %d, want 0", got)
	}
	if got := GetBufferPtr(9999); got != 0 {
		t.Fatalf("GetBufferPtr for invalid handle = %d, want 0", got)
	}
	FreePointer(12345)

	rendered, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("RenderFrame: %v", err)
	}
	ptr := GetBufferPtr(h)
	if ptr == 0 {
		t.Fatal("GetBufferPtr after render = 0, want non-zero")
	}
	if ptr != rendered.BufferPtr {
		t.Fatalf("GetBufferPtr after render = %d, want %d", ptr, rendered.BufferPtr)
	}
	FreePointer(ptr)
	if got := GetBufferPtr(h); got != ptr {
		t.Fatalf("FreePointer should be a no-op, GetBufferPtr after FreePointer = %d, want %d", got, ptr)
	}
}

func TestDispatchCommandTransactionDefaultsToCommitWhenPayloadEmpty(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginTxn, mustJSON(t, BeginTransactionPayload{})); err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 2.5})); err != nil {
		t.Fatalf("zoom in transaction: %v", err)
	}

	committed, err := DispatchCommand(h, commandEndTxn, "")
	if err != nil {
		t.Fatalf("end transaction with empty payload: %v", err)
	}
	if len(committed.UIMeta.History) != 1 {
		t.Fatalf("history length after committed transaction = %d, want 1", len(committed.UIMeta.History))
	}
	if committed.UIMeta.History[0].Description != "Transaction" {
		t.Fatalf("transaction description = %q, want Transaction", committed.UIMeta.History[0].Description)
	}
	if committed.UIMeta.CurrentHistoryIndex != 1 || !committed.UIMeta.CanUndo {
		t.Fatalf("unexpected history state after commit: index=%d canUndo=%v", committed.UIMeta.CurrentHistoryIndex, committed.UIMeta.CanUndo)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo committed transaction: %v", err)
	}
	if undone.Viewport.Zoom != 1 {
		t.Fatalf("zoom after undo = %.2f, want 1", undone.Viewport.Zoom)
	}
}

func TestDispatchCommandFitToViewCentersAndScalesDocument(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	if _, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{CanvasW: 500, CanvasH: 250})); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if _, err := DispatchCommand(h, commandPanSet, mustJSON(t, PanPayload{CenterX: 17, CenterY: 29})); err != nil {
		t.Fatalf("pan before fit: %v", err)
	}
	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 12})); err != nil {
		t.Fatalf("zoom before fit: %v", err)
	}

	fitted, err := DispatchCommand(h, commandFitToView, "")
	if err != nil {
		t.Fatalf("fit to view: %v", err)
	}

	doc := instances[h].manager.Active()
	if doc == nil {
		t.Fatal("expected active document after fit to view")
	}
	if fitted.Viewport.CenterX != float64(doc.Width)/2 || fitted.Viewport.CenterY != float64(doc.Height)/2 {
		t.Fatalf("viewport center after fit = %.2f, %.2f, want %.2f, %.2f", fitted.Viewport.CenterX, fitted.Viewport.CenterY, float64(doc.Width)/2, float64(doc.Height)/2)
	}
	expectedZoom := clampZoom(math.Min(float64(fitted.Viewport.CanvasW)*0.84/float64(maxInt(doc.Width, 1)), float64(fitted.Viewport.CanvasH)*0.84/float64(maxInt(doc.Height, 1))))
	if fitted.Viewport.Zoom != expectedZoom {
		t.Fatalf("zoom after fit = %.6f, want %.6f", fitted.Viewport.Zoom, expectedZoom)
	}
	if len(fitted.UIMeta.History) == 0 || fitted.UIMeta.History[len(fitted.UIMeta.History)-1].Description != "Fit document on screen" {
		t.Fatalf("unexpected history after fit to view: %+v", fitted.UIMeta.History)
	}
}

func TestDispatchCommandOpenImageFileAndSetActiveLayerWithoutHistoryEntry(t *testing.T) {
	h := Init("")
	defer Free(h)

	opened, err := DispatchCommand(h, commandOpenImageFile, mustJSON(t, OpenImageFilePayload{
		Name:   "Imported",
		Width:  4,
		Height: 2,
		Pixels: filledPixels(4, 2, [4]byte{120, 45, 210, 255}),
	}))
	if err != nil {
		t.Fatalf("open image file: %v", err)
	}
	if opened.UIMeta.ActiveDocumentName != "Imported" {
		t.Fatalf("active document name = %q, want Imported", opened.UIMeta.ActiveDocumentName)
	}
	if opened.UIMeta.DocumentWidth != 4 || opened.UIMeta.DocumentHeight != 2 {
		t.Fatalf("opened document size = %dx%d, want 4x2", opened.UIMeta.DocumentWidth, opened.UIMeta.DocumentHeight)
	}

	doc := instances[h].manager.Active()
	if doc == nil {
		t.Fatal("expected active document after open image")
	}
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("opened image layer count = %d, want 1", len(children))
	}
	backgroundID := children[0].ID()

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Top",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 2},
		Pixels:    filledPixels(4, 2, [4]byte{255, 0, 0, 255}),
	}))
	if err != nil {
		t.Fatalf("add second layer: %v", err)
	}
	if added.UIMeta.ActiveLayerID == backgroundID {
		t.Fatal("expected newly added layer to become active")
	}
	historyIndex := added.UIMeta.CurrentHistoryIndex
	historyLen := len(added.UIMeta.History)

	switched, err := DispatchCommand(h, commandSetActiveLayer, mustJSON(t, SetActiveLayerPayload{LayerID: backgroundID}))
	if err != nil {
		t.Fatalf("set active layer: %v", err)
	}
	if switched.UIMeta.ActiveLayerID != backgroundID {
		t.Fatalf("active layer after switch = %q, want %q", switched.UIMeta.ActiveLayerID, backgroundID)
	}
	if switched.UIMeta.CurrentHistoryIndex != historyIndex || len(switched.UIMeta.History) != historyLen {
		t.Fatalf("set active layer should not change history, got index=%d len=%d want %d/%d", switched.UIMeta.CurrentHistoryIndex, len(switched.UIMeta.History), historyIndex, historyLen)
	}
}

func TestCropDocument(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	result, err := DispatchCommand(h, commandBeginCrop, "")
	if err != nil {
		t.Fatalf("begin crop: %v", err)
	}
	if result.UIMeta.Crop == nil || !result.UIMeta.Crop.Active {
		t.Fatal("expected active crop meta in UI")
	}

	updatePayload := UpdateCropPayload{X: 10, Y: 20, W: 100, H: 50}
	result, err = DispatchCommand(h, commandUpdateCrop, mustJSON(t, updatePayload))
	if err != nil {
		t.Fatalf("update crop: %v", err)
	}
	if result.UIMeta.Crop.X != 10 || result.UIMeta.Crop.W != 100 {
		t.Fatalf("crop meta after update = %+v, want X=10 W=100", result.UIMeta.Crop)
	}

	result, err = DispatchCommand(h, commandCommitCrop, "")
	if err != nil {
		t.Fatalf("commit crop: %v", err)
	}
	if result.UIMeta.Crop != nil {
		t.Fatal("expected crop meta to be cleared after commit")
	}
	if result.UIMeta.DocumentWidth != 100 || result.UIMeta.DocumentHeight != 50 {
		t.Fatalf("document size after crop = %dx%d, want 100x50", result.UIMeta.DocumentWidth, result.UIMeta.DocumentHeight)
	}
	if len(result.UIMeta.History) == 0 || result.UIMeta.History[len(result.UIMeta.History)-1].Description != "Crop Document" {
		t.Fatalf("unexpected history after crop: %+v", result.UIMeta.History)
	}

	result, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo crop: %v", err)
	}
	if result.UIMeta.DocumentWidth == 100 {
		t.Fatal("expected document width to be restored after undo")
	}
}

func TestCanvasSize(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		Name:      "Layer 1",
		LayerType: LayerTypePixel,
		Bounds:    LayerBounds{X: 0, Y: 0, W: instances[h].manager.Active().Width, H: instances[h].manager.Active().Height},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	doc := instances[h].manager.Active()
	oldW, oldH := doc.Width, doc.Height

	payload := ResizeCanvasPayload{Width: oldW + 20, Height: oldH + 40, Anchor: "center"}
	_ = layerID
	result, err := DispatchCommand(h, commandResizeCanvas, mustJSON(t, payload))
	if err != nil {
		t.Fatalf("resize canvas: %v", err)
	}

	if result.UIMeta.DocumentWidth != oldW+20 || result.UIMeta.DocumentHeight != oldH+40 {
		t.Fatalf("document size after resize = %dx%d, want %dx%d", result.UIMeta.DocumentWidth, result.UIMeta.DocumentHeight, oldW+20, oldH+40)
	}

	doc = instances[h].manager.Active()
	pl := doc.ActiveLayer().(*PixelLayer)
	if pl.Bounds.X != 10 || pl.Bounds.Y != 20 {
		t.Fatalf("layer bounds after center resize = %d,%d, want 10,20", pl.Bounds.X, pl.Bounds.Y)
	}

	result, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo resize: %v", err)
	}
	if result.UIMeta.DocumentWidth != oldW {
		t.Fatal("expected document width to be restored after undo")
	}
}

func TestDocumentManagerSwitchReplaceAndCloseActive(t *testing.T) {
	manager := newDocumentManager()
	docA := testDocumentFixture("doc-a", "A", 100, 80)
	if err := manager.ReplaceActive(docA); err != nil {
		t.Fatalf("ReplaceActive without active document: %v", err)
	}
	if manager.ActiveID() != docA.ID {
		t.Fatalf("active document id = %q, want %q", manager.ActiveID(), docA.ID)
	}

	activeCopy := manager.Active()
	activeCopy.Name = "mutated"
	if manager.Active().Name != docA.Name {
		t.Fatal("Active should return a clone, not the stored document pointer")
	}

	docB := testDocumentFixture("doc-b", "B", 320, 200)
	manager.Create(docB)
	if manager.ActiveID() != docB.ID {
		t.Fatalf("active document id after Create = %q, want %q", manager.ActiveID(), docB.ID)
	}

	if err := manager.Switch("missing"); err == nil {
		t.Fatal("expected Switch to fail for an unknown document")
	}
	if err := manager.Switch(docA.ID); err != nil {
		t.Fatalf("Switch(docA): %v", err)
	}
	if manager.ActiveID() != docA.ID {
		t.Fatalf("active document id after Switch = %q, want %q", manager.ActiveID(), docA.ID)
	}

	replacement := testDocumentFixture(docA.ID, "A updated", 640, 480)
	if err := manager.ReplaceActive(replacement); err != nil {
		t.Fatalf("ReplaceActive(docA replacement): %v", err)
	}
	active := manager.Active()
	if active.Name != replacement.Name || active.Width != replacement.Width || active.Height != replacement.Height {
		t.Fatalf("active document after replace = %+v, want %+v", active, replacement)
	}

	if err := manager.CloseActive(); err != nil {
		t.Fatalf("CloseActive on first document: %v", err)
	}
	if manager.ActiveID() != docB.ID {
		t.Fatalf("active document id after closing A = %q, want %q", manager.ActiveID(), docB.ID)
	}

	if err := manager.CloseActive(); err != nil {
		t.Fatalf("CloseActive on last document: %v", err)
	}
	if manager.ActiveID() != "" {
		t.Fatalf("active document id after closing all documents = %q, want empty", manager.ActiveID())
	}
	if manager.Active() != nil {
		t.Fatal("expected Active to return nil after closing all documents")
	}
	if err := manager.CloseActive(); err != nil {
		t.Fatalf("CloseActive without active document should be a no-op: %v", err)
	}
}

func TestCloseDocumentActivatesPreviousDocumentAndUndoRestoresClosedDocument(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Second",
		Width:      800,
		Height:     600,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "white",
	})); err != nil {
		t.Fatalf("create second document: %v", err)
	}
	third, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Third",
		Width:      320,
		Height:     240,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "transparent",
	}))
	if err != nil {
		t.Fatalf("create third document: %v", err)
	}
	if third.UIMeta.ActiveDocumentName != "Third" {
		t.Fatalf("active document name before close = %q, want Third", third.UIMeta.ActiveDocumentName)
	}

	closed, err := DispatchCommand(h, commandCloseDocument, "")
	if err != nil {
		t.Fatalf("close document: %v", err)
	}
	if closed.UIMeta.ActiveDocumentName != "Second" {
		t.Fatalf("active document name after close = %q, want Second", closed.UIMeta.ActiveDocumentName)
	}
	if closed.UIMeta.DocumentWidth != 800 || closed.UIMeta.DocumentHeight != 600 {
		t.Fatalf("active document size after close = %dx%d, want 800x600", closed.UIMeta.DocumentWidth, closed.UIMeta.DocumentHeight)
	}
	if closed.Viewport.CenterX != 400 || closed.Viewport.CenterY != 300 {
		t.Fatalf("viewport center after close = %.2f, %.2f, want 400, 300", closed.Viewport.CenterX, closed.Viewport.CenterY)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo close document: %v", err)
	}
	if undone.UIMeta.ActiveDocumentName != "Third" {
		t.Fatalf("active document name after undo = %q, want Third", undone.UIMeta.ActiveDocumentName)
	}
	if undone.UIMeta.DocumentWidth != 320 || undone.UIMeta.DocumentHeight != 240 {
		t.Fatalf("active document size after undo = %dx%d, want 320x240", undone.UIMeta.DocumentWidth, undone.UIMeta.DocumentHeight)
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo close document: %v", err)
	}
	if redone.UIMeta.ActiveDocumentName != "Second" {
		t.Fatalf("active document name after redo = %q, want Second", redone.UIMeta.ActiveDocumentName)
	}
}

func TestCloseLastDocumentReturnsNoActiveDocumentState(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	closed, err := DispatchCommand(h, commandCloseDocument, "")
	if err != nil {
		t.Fatalf("close last document: %v", err)
	}
	if closed.BufferLen != 0 || closed.BufferPtr != 0 {
		t.Fatalf("buffer after closing last document = len %d ptr %d, want 0/0", closed.BufferLen, closed.BufferPtr)
	}
	if closed.UIMeta.ActiveDocumentID != "" || closed.UIMeta.ActiveDocumentName != "" {
		t.Fatalf("active document after closing last = %q/%q, want empty", closed.UIMeta.ActiveDocumentID, closed.UIMeta.ActiveDocumentName)
	}
	if closed.UIMeta.StatusText != "No active document" {
		t.Fatalf("status text after closing last document = %q, want No active document", closed.UIMeta.StatusText)
	}
	if !closed.UIMeta.CanUndo {
		t.Fatal("closing the last document should still be undoable")
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo close last document: %v", err)
	}
	if undone.UIMeta.ActiveDocumentName == "" {
		t.Fatal("undo close last document should restore an active document")
	}
	if undone.BufferLen == 0 {
		t.Fatal("undo close last document should restore the render buffer")
	}
}

func TestVectorMaskAddDeleteUndoable(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{W: 4, H: 4},
		Pixels:    make([]byte, 4*4*4),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	after, err := DispatchCommand(h, commandAddVectorMask, mustJSON(t, AddVectorMaskPayload{LayerID: layerID}))
	if err != nil {
		t.Fatalf("add vector mask: %v", err)
	}
	layers := after.UIMeta.Layers
	if len(layers) == 0 || !layers[0].HasVectorMask {
		t.Fatal("expected layer to have a vector mask after AddVectorMask")
	}

	after, err = DispatchCommand(h, commandDeleteVectorMask, mustJSON(t, DeleteVectorMaskPayload{LayerID: layerID}))
	if err != nil {
		t.Fatalf("delete vector mask: %v", err)
	}
	if len(after.UIMeta.Layers) > 0 && after.UIMeta.Layers[0].HasVectorMask {
		t.Fatal("expected vector mask to be removed after DeleteVectorMask")
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(undone.UIMeta.Layers) == 0 || !undone.UIMeta.Layers[0].HasVectorMask {
		t.Fatal("expected vector mask restored after undo of DeleteVectorMask")
	}
}

func TestMaskEditModeNotTrackedInHistory(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{W: 4, H: 4},
		Pixels:    make([]byte, 4*4*4),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	if _, err := DispatchCommand(h, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{
		LayerID: layerID,
		Mode:    AddLayerMaskRevealAll,
	})); err != nil {
		t.Fatalf("add mask: %v", err)
	}

	historyBefore := instances[h].history.CurrentIndex()

	after, err := DispatchCommand(h, commandSetMaskEditMode, mustJSON(t, SetMaskEditModePayload{
		LayerID: layerID,
		Editing: true,
	}))
	if err != nil {
		t.Fatalf("set mask edit mode: %v", err)
	}
	if after.UIMeta.MaskEditLayerID != layerID {
		t.Fatalf("maskEditLayerId = %q, want %q", after.UIMeta.MaskEditLayerID, layerID)
	}

	if instances[h].history.CurrentIndex() != historyBefore {
		t.Fatal("SetMaskEditMode should not add a history entry")
	}

	exit, err := DispatchCommand(h, commandSetMaskEditMode, mustJSON(t, SetMaskEditModePayload{
		LayerID: layerID,
		Editing: false,
	}))
	if err != nil {
		t.Fatalf("exit mask edit mode: %v", err)
	}
	if exit.UIMeta.MaskEditLayerID != "" {
		t.Fatalf("maskEditLayerId after exit = %q, want empty", exit.UIMeta.MaskEditLayerID)
	}
}

func TestVectorMaskRendersWithoutError(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	_, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{CanvasW: 8, CanvasH: 8, DevicePixelRatio: 1}))
	if err != nil {
		t.Fatalf("resize: %v", err)
	}

	result, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{W: 8, H: 8},
		Pixels:    filledPixels(8, 8, [4]byte{255, 0, 0, 255}),
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := result.UIMeta.ActiveLayerID

	if _, err := DispatchCommand(h, commandAddVectorMask, mustJSON(t, AddVectorMaskPayload{LayerID: layerID})); err != nil {
		t.Fatalf("add vector mask: %v", err)
	}

	if _, err := RenderFrame(h); err != nil {
		t.Fatalf("RenderFrame with vector mask: %v", err)
	}
}

func TestDocumentAndSnapshotHelpersCoverMismatchBranches(t *testing.T) {
	doc := testDocumentFixture("doc-helper", "Helpers", 64, 32)
	layer := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, filledPixels(2, 2, [4]byte{1, 2, 3, 255}))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()

	if !documentsEqual(nil, nil) {
		t.Fatal("documentsEqual(nil, nil) = false, want true")
	}
	if documentsEqual(nil, doc) {
		t.Fatal("documentsEqual(nil, doc) = true, want false")
	}

	same := cloneDocument(doc)
	if same == doc || same.LayerRoot == doc.LayerRoot {
		t.Fatal("cloneDocument should deep clone document and layer root")
	}
	if !documentsEqual(doc, same) {
		t.Fatal("documentsEqual(clone) = false, want true")
	}

	widthMismatch := cloneDocument(doc)
	widthMismatch.Width++
	if documentsEqual(doc, widthMismatch) {
		t.Fatal("documentsEqual should detect width mismatch")
	}

	metadataMismatch := cloneDocument(doc)
	metadataMismatch.ModifiedAt = "2026-03-27T11:00:00Z"
	if documentsEqual(doc, metadataMismatch) {
		t.Fatal("documentsEqual should detect metadata mismatch")
	}

	activeLayerMismatch := cloneDocument(doc)
	activeLayerMismatch.ActiveLayerID = "different-layer"
	if documentsEqual(doc, activeLayerMismatch) {
		t.Fatal("documentsEqual should detect active layer mismatch")
	}

	layerMismatch := cloneDocument(doc)
	layerMismatch.LayerRoot.Children()[0].SetName("Renamed")
	if documentsEqual(doc, layerMismatch) {
		t.Fatal("documentsEqual should detect layer tree mismatch")
	}

	baseSnapshot := snapshot{DocumentID: doc.ID, Document: cloneDocument(doc), Viewport: ViewportState{CenterX: 12, CenterY: 8, Zoom: 1.5}}
	if !snapshotsEqual(baseSnapshot, snapshot{DocumentID: doc.ID, Document: cloneDocument(doc), Viewport: baseSnapshot.Viewport}) {
		t.Fatal("snapshotsEqual should accept identical snapshots")
	}
	if snapshotsEqual(baseSnapshot, snapshot{DocumentID: "other", Document: cloneDocument(doc), Viewport: baseSnapshot.Viewport}) {
		t.Fatal("snapshotsEqual should detect document id mismatch")
	}
	if snapshotsEqual(baseSnapshot, snapshot{DocumentID: doc.ID, Document: cloneDocument(doc), Viewport: ViewportState{CenterX: 12, CenterY: 8, Zoom: 2}}) {
		t.Fatal("snapshotsEqual should detect viewport mismatch")
	}
	if snapshotsEqual(baseSnapshot, snapshot{DocumentID: doc.ID, Document: nil, Viewport: baseSnapshot.Viewport}) {
		t.Fatal("snapshotsEqual should detect document nil mismatch")
	}
	if !snapshotsEqual(snapshot{Viewport: ViewportState{Zoom: 1}}, snapshot{Viewport: ViewportState{Zoom: 1}}) {
		t.Fatal("snapshotsEqual(nil docs) = false, want true")
	}
}

func TestRestoreSnapshotAndUtilityHelpers(t *testing.T) {
	inst := &instance{manager: newDocumentManager(), viewport: ViewportState{Zoom: 4}}
	doc := testDocumentFixture("doc-restore", "Restore", 80, 40)
	layer := NewPixelLayer("Layer", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, filledPixels(1, 1, [4]byte{9, 8, 7, 255}))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()

	state := snapshot{
		DocumentID: doc.ID,
		Document:   doc,
		Viewport:   ViewportState{CenterX: 40, CenterY: 20, Zoom: 2, Rotation: 30},
	}
	if err := inst.restoreSnapshot(state); err != nil {
		t.Fatalf("restoreSnapshot with document: %v", err)
	}
	restored := inst.manager.Active()
	if restored == nil {
		t.Fatal("restoreSnapshot should restore active document")
	}
	if !documentsEqual(restored, doc) {
		t.Fatalf("restored document = %+v, want %+v", restored, doc)
	}
	doc.Name = "mutated after restore"
	if inst.manager.Active().Name != "Restore" {
		t.Fatal("restoreSnapshot should clone the restored document")
	}
	if inst.viewport != state.Viewport {
		t.Fatalf("viewport after restore = %+v, want %+v", inst.viewport, state.Viewport)
	}

	clearedState := snapshot{Viewport: ViewportState{CenterX: 1, CenterY: 2, Zoom: 0.5}}
	if err := inst.restoreSnapshot(clearedState); err != nil {
		t.Fatalf("restoreSnapshot with nil document: %v", err)
	}
	if inst.manager.Active() != nil {
		t.Fatal("restoreSnapshot with nil document should clear the active document")
	}
	if inst.viewport != clearedState.Viewport {
		t.Fatalf("viewport after clearing restore = %+v, want %+v", inst.viewport, clearedState.Viewport)
	}

	if got := defaultDocumentName(""); got != "Untitled" {
		t.Fatalf("defaultDocumentName(\"\") = %q, want Untitled", got)
	}
	if got := defaultDocumentName("Poster"); got != "Poster" {
		t.Fatalf("defaultDocumentName(Poster) = %q, want Poster", got)
	}
	if got := parseBackground("white"); got.Kind != "white" || got.Color != [4]uint8{244, 246, 250, 255} {
		t.Fatalf("parseBackground(white) = %+v, want white preset", got)
	}
	if got := parseBackground("color"); got.Kind != "color" || got.Color != [4]uint8{236, 147, 92, 255} {
		t.Fatalf("parseBackground(color) = %+v, want color preset", got)
	}
	if got := parseBackground("unknown"); got.Kind != "transparent" {
		t.Fatalf("parseBackground(default) = %+v, want transparent", got)
	}
	if got := clampZoom(-1); got != 1 {
		t.Fatalf("clampZoom(-1) = %.2f, want 1", got)
	}
	if got := clampZoom(0.01); got != 0.05 {
		t.Fatalf("clampZoom(0.01) = %.2f, want 0.05", got)
	}
	if got := clampZoom(40); got != 32 {
		t.Fatalf("clampZoom(40) = %.2f, want 32", got)
	}
	if got := clampZoom(2.5); got != 2.5 {
		t.Fatalf("clampZoom(2.5) = %.2f, want 2.5", got)
	}
	if got := normalizeRotation(-30); got != 330 {
		t.Fatalf("normalizeRotation(-30) = %.2f, want 330", got)
	}
	if got := normalizeRotation(765); got != 45 {
		t.Fatalf("normalizeRotation(765) = %.2f, want 45", got)
	}
	if got := maxInt(3, 7); got != 7 {
		t.Fatalf("maxInt(3, 7) = %d, want 7", got)
	}
	if got := maxInt(9, 2); got != 9 {
		t.Fatalf("maxInt(9, 2) = %d, want 9", got)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(bytes)
}

func testDocumentFixture(id, name string, width, height int) *Document {
	return &Document{
		Width:      width,
		Height:     height,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		ID:         id,
		Name:       name,
		CreatedAt:  "2026-03-27T10:00:00Z",
		CreatedBy:  "agogo-web-test",
		ModifiedAt: "2026-03-27T10:00:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}
}
