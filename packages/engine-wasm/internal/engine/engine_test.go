package engine

import (
	"encoding/json"
	"testing"
)

func TestInitFree(t *testing.T) {
	h := Init("")
	if h <= 0 {
		t.Fatalf("Init() returned invalid handle %d", h)
	}
	Free(h)

	if got := GetBufferLen(h); got != 0 {
		t.Errorf("GetBufferLen after Free = %d, want 0", got)
	}
}

func TestRenderFrameIncludesViewportBuffer(t *testing.T) {
	h := Init("")
	defer Free(h)

	result, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{
		CanvasW:          640,
		CanvasH:          480,
		DevicePixelRatio: 2,
	}))
	if err != nil {
		t.Fatalf("DispatchCommand resize: %v", err)
	}

	if result.BufferLen != 640*480*4 {
		t.Fatalf("bufferLen = %d, want %d", result.BufferLen, 640*480*4)
	}
	if result.BufferPtr == 0 {
		t.Fatal("bufferPtr = 0, want non-zero")
	}
	if result.Viewport.CanvasW != 640 || result.Viewport.CanvasH != 480 {
		t.Fatalf("viewport size = %dx%d, want 640x480", result.Viewport.CanvasW, result.Viewport.CanvasH)
	}
}

func TestCreateDocumentUpdatesStatusAndMetadata(t *testing.T) {
	h := Init("")
	defer Free(h)

	result, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Poster",
		Width:      2048,
		Height:     2048,
		Resolution: 300,
		ColorMode:  "rgb",
		BitDepth:   16,
		Background: "white",
	}))
	if err != nil {
		t.Fatalf("DispatchCommand create document: %v", err)
	}

	if result.UIMeta.ActiveDocumentName != "Poster" {
		t.Fatalf("activeDocumentName = %q, want Poster", result.UIMeta.ActiveDocumentName)
	}
	if result.UIMeta.DocumentWidth != 2048 || result.UIMeta.DocumentHeight != 2048 {
		t.Fatalf("document size = %dx%d, want 2048x2048", result.UIMeta.DocumentWidth, result.UIMeta.DocumentHeight)
	}
	if len(result.UIMeta.History) == 0 {
		t.Fatal("history should contain the create document entry")
	}
}

func TestUndoRedoRestoresViewportState(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 2})); err != nil {
		t.Fatalf("zoom: %v", err)
	}
	if _, err := DispatchCommand(h, commandPanSet, mustJSON(t, PanPayload{CenterX: 400, CenterY: 240})); err != nil {
		t.Fatalf("pan: %v", err)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if undone.Viewport.CenterX == 400 && undone.Viewport.CenterY == 240 {
		t.Fatal("undo did not restore the previous viewport center")
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo: %v", err)
	}
	if redone.Viewport.CenterX != 400 || redone.Viewport.CenterY != 240 {
		t.Fatalf("redo viewport center = %.2f, %.2f, want 400, 240", redone.Viewport.CenterX, redone.Viewport.CenterY)
	}
}

func TestRenderViewportProducesOpaqueBuffer(t *testing.T) {
	doc := &Document{
		Width:      100,
		Height:     80,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Unit Test",
	}
	vp := &ViewportState{
		CenterX:          50,
		CenterY:          40,
		Zoom:             1,
		CanvasW:          128,
		CanvasH:          96,
		DevicePixelRatio: 1,
	}

	pixels := RenderViewport(doc, vp, nil, nil)
	if got, want := len(pixels), 128*96*4; got != want {
		t.Fatalf("len(pixels) = %d, want %d", got, want)
	}
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] != 255 {
			t.Fatalf("alpha[%d] = %d, want 255", i, pixels[i])
		}
	}
}

func TestRenderViewportIncludesLayerComposite(t *testing.T) {
	doc := &Document{
		Width:      8,
		Height:     8,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Layered",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 8, H: 8}, filledPixels(8, 8, [4]byte{0, 0, 255, 255}))
	top := NewPixelLayer("Top", LayerBounds{X: 0, Y: 0, W: 8, H: 8}, filledPixels(8, 8, [4]byte{255, 0, 0, 255}))
	top.SetBlendMode(BlendModeScreen)
	doc.LayerRoot.SetChildren([]LayerNode{base, top})
	vp := &ViewportState{CenterX: 4, CenterY: 4, Zoom: 1, CanvasW: 8, CanvasH: 8, DevicePixelRatio: 1}

	pixels := RenderViewport(doc, vp, nil, doc.renderCompositeSurface())
	red, green, blue, alpha := pixelAt(pixels, 8, 1, 1)
	if red < 250 || green > 5 || blue < 250 || alpha != 255 {
		t.Fatalf("viewport pixel = [%d %d %d %d], want screen blend close to [255 0 255 255]", red, green, blue, alpha)
	}
}

func TestRenderViewportRespectsGroupIsolation(t *testing.T) {
	buildDoc := func(isolated bool) *Document {
		doc := &Document{
			Width:      8,
			Height:     8,
			Resolution: 72,
			ColorMode:  "rgb",
			BitDepth:   8,
			Background: parseBackground("transparent"),
			Name:       "Groups",
			LayerRoot:  NewGroupLayer("Root"),
		}
		bottom := NewPixelLayer("Bottom", LayerBounds{X: 0, Y: 0, W: 8, H: 8}, filledPixels(8, 8, [4]byte{0, 0, 255, 255}))
		multiply := NewPixelLayer("Multiply", LayerBounds{X: 0, Y: 0, W: 8, H: 8}, filledPixels(8, 8, [4]byte{255, 0, 0, 255}))
		multiply.SetBlendMode(BlendModeMultiply)
		screen := NewPixelLayer("Screen", LayerBounds{X: 0, Y: 0, W: 8, H: 8}, filledPixels(8, 8, [4]byte{0, 255, 0, 255}))
		screen.SetBlendMode(BlendModeScreen)
		group := NewGroupLayer("Group")
		group.Isolated = isolated
		group.SetChildren([]LayerNode{multiply, screen})
		doc.LayerRoot.SetChildren([]LayerNode{bottom, group})
		return doc
	}

	vp := &ViewportState{CenterX: 4, CenterY: 4, Zoom: 1, CanvasW: 8, CanvasH: 8, DevicePixelRatio: 1}
	passThroughDoc := buildDoc(false)
	isolatedDoc := buildDoc(true)
	passThrough := RenderViewport(passThroughDoc, vp, nil, passThroughDoc.renderCompositeSurface())
	isolated := RenderViewport(isolatedDoc, vp, nil, isolatedDoc.renderCompositeSurface())
	passRed, _, passBlue, _ := pixelAt(passThrough, 8, 1, 1)
	isoRed, _, isoBlue, _ := pixelAt(isolated, 8, 1, 1)
	if passRed == isoRed && passBlue == isoBlue {
		t.Fatal("expected isolated and pass-through groups to render differently in the viewport")
	}
}

func TestRenderCompositeSurfaceAppliesRasterMask(t *testing.T) {
	doc := &Document{
		Width:      2,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Masked",
		LayerRoot:  NewGroupLayer("Root"),
	}
	group := NewGroupLayer("Group")
	group.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 1, Data: []byte{255, 0}})
	child := NewPixelLayer("Fill", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	group.SetChildren([]LayerNode{child})
	doc.LayerRoot.SetChildren([]LayerNode{group})

	surface := doc.renderCompositeSurface()
	if got := surface[:4]; got[0] != 255 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("first composite pixel = %v, want opaque red", got)
	}
	if got := surface[4:8]; got[0] != 0 || got[1] != 0 || got[2] != 0 || got[3] != 0 {
		t.Fatalf("second composite pixel = %v, want fully masked out", got)
	}
}

func TestLayerMaskCommandsUpdateMetadataAndUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Masked",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{255, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	layerID := added.UIMeta.ActiveLayerID

	withMask, err := DispatchCommand(h, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{LayerID: layerID, Mode: AddLayerMaskRevealAll}))
	if err != nil {
		t.Fatalf("add mask command: %v", err)
	}
	meta, ok := findLayerMetaByID(withMask.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q not found after add mask", layerID)
	}
	if !meta.HasMask || !meta.MaskEnabled {
		t.Fatalf("mask metadata = %+v, want enabled mask", meta)
	}

	disabled, err := DispatchCommand(h, commandSetMaskEnabled, mustJSON(t, SetLayerMaskEnabledPayload{LayerID: layerID, Enabled: false}))
	if err != nil {
		t.Fatalf("disable mask command: %v", err)
	}
	meta, ok = findLayerMetaByID(disabled.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q not found after disable mask", layerID)
	}
	if !meta.HasMask || meta.MaskEnabled {
		t.Fatalf("mask metadata after disable = %+v, want disabled mask", meta)
	}

	applied, err := DispatchCommand(h, commandApplyLayerMask, mustJSON(t, ApplyLayerMaskPayload{LayerID: layerID}))
	if err != nil {
		t.Fatalf("apply mask command: %v", err)
	}
	meta, ok = findLayerMetaByID(applied.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q not found after apply mask", layerID)
	}
	if meta.HasMask || meta.MaskEnabled {
		t.Fatalf("mask metadata after apply = %+v, want no mask", meta)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo apply mask: %v", err)
	}
	meta, ok = findLayerMetaByID(undone.UIMeta.Layers, layerID)
	if !ok {
		t.Fatalf("layer %q not found after undo", layerID)
	}
	if !meta.HasMask || meta.MaskEnabled {
		t.Fatalf("mask metadata after undo = %+v, want disabled mask restored", meta)
	}
}

func TestLayerClipCommandUpdatesMetadataAndUndo(t *testing.T) {
	h := Init("")
	defer Free(h)

	base, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 1},
		Pixels: []byte{
			0, 0, 255, 255,
			0, 0, 255, 0,
		},
	}))
	if err != nil {
		t.Fatalf("add base layer: %v", err)
	}
	baseID := base.UIMeta.ActiveLayerID

	top, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Top",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 1},
		Pixels: []byte{
			255, 0, 0, 255,
			255, 0, 0, 255,
		},
	}))
	if err != nil {
		t.Fatalf("add top layer: %v", err)
	}
	topID := top.UIMeta.ActiveLayerID

	clipped, err := DispatchCommand(h, commandSetLayerClip, mustJSON(t, SetLayerClipToBelowPayload{LayerID: topID, ClipToBelow: true}))
	if err != nil {
		t.Fatalf("set clip command: %v", err)
	}
	baseMeta, ok := findLayerMetaByID(clipped.UIMeta.Layers, baseID)
	if !ok {
		t.Fatalf("base layer %q not found", baseID)
	}
	topMeta, ok := findLayerMetaByID(clipped.UIMeta.Layers, topID)
	if !ok {
		t.Fatalf("top layer %q not found", topID)
	}
	if !baseMeta.ClippingBase || !topMeta.ClipToBelow {
		t.Fatalf("unexpected clipping metadata: base=%+v top=%+v", baseMeta, topMeta)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo clip command: %v", err)
	}
	baseMeta, ok = findLayerMetaByID(undone.UIMeta.Layers, baseID)
	if !ok {
		t.Fatalf("base layer %q not found after undo", baseID)
	}
	topMeta, ok = findLayerMetaByID(undone.UIMeta.Layers, topID)
	if !ok {
		t.Fatalf("top layer %q not found after undo", topID)
	}
	if baseMeta.ClippingBase || topMeta.ClipToBelow {
		t.Fatalf("unexpected metadata after undo: base=%+v top=%+v", baseMeta, topMeta)
	}
}

func TestRenderCompositeSurfaceAppliesClipToBelow(t *testing.T) {
	doc := &Document{
		Width:      2,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Clipped",
		LayerRoot:  NewGroupLayer("Root"),
	}
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

	surface := doc.renderCompositeSurface()
	if got := surface[:4]; got[0] != 255 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("first composite pixel = %v, want opaque red", got)
	}
	if got := surface[4:8]; got[0] != 0 || got[1] != 0 || got[2] != 0 || got[3] != 0 {
		t.Fatalf("second composite pixel = %v, want fully clipped transparent pixel", got)
	}
}

func filledPixels(width, height int, color [4]byte) []byte {
	pixels := make([]byte, width*height*4)
	for index := 0; index < len(pixels); index += 4 {
		copy(pixels[index:index+4], color[:])
	}
	return pixels
}

func pixelAt(pixels []byte, width, x, y int) (byte, byte, byte, byte) {
	index := (y*width + x) * 4
	return pixels[index], pixels[index+1], pixels[index+2], pixels[index+3]
}

func TestInvalidHandleFails(t *testing.T) {
	if _, err := DispatchCommand(9999, commandResize, mustJSON(t, ResizePayload{CanvasW: 10, CanvasH: 10})); err == nil {
		t.Fatal("expected invalid handle error")
	}
}

func TestPointerEventPanUpdatesViewportCenter(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{
		CanvasW: 800,
		CanvasH: 600,
	})); err != nil {
		t.Fatalf("resize: %v", err)
	}

	before, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("render before: %v", err)
	}

	if _, err := DispatchCommand(h, commandPointerEvent, mustJSON(t, PointerEventPayload{
		Phase:     "down",
		PointerID: 1,
		X:         400,
		Y:         300,
		PanMode:   true,
	})); err != nil {
		t.Fatalf("pointer down: %v", err)
	}

	afterMove, err := DispatchCommand(h, commandPointerEvent, mustJSON(t, PointerEventPayload{
		Phase:     "move",
		PointerID: 1,
		X:         500,
		Y:         300,
		PanMode:   true,
	}))
	if err != nil {
		t.Fatalf("pointer move: %v", err)
	}

	if afterMove.Viewport.CenterX >= before.Viewport.CenterX {
		t.Fatalf("centerX = %.2f, want less than %.2f after dragging right", afterMove.Viewport.CenterX, before.Viewport.CenterX)
	}

	if afterMove.UIMeta.CursorType != "grabbing" {
		t.Fatalf("cursorType = %q, want grabbing", afterMove.UIMeta.CursorType)
	}

	afterUp, err := DispatchCommand(h, commandPointerEvent, mustJSON(t, PointerEventPayload{
		Phase:     "up",
		PointerID: 1,
		X:         500,
		Y:         300,
		PanMode:   true,
	}))
	if err != nil {
		t.Fatalf("pointer up: %v", err)
	}

	if afterUp.UIMeta.CursorType != "default" {
		t.Fatalf("cursorType after up = %q, want default", afterUp.UIMeta.CursorType)
	}
}

func TestZoomAnchorKeepsAnchorStable(t *testing.T) {
	h := Init("")
	defer Free(h)

	before, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("render before: %v", err)
	}

	after, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{
		Zoom:      2,
		HasAnchor: true,
		AnchorX:   0,
		AnchorY:   0,
	}))
	if err != nil {
		t.Fatalf("zoom: %v", err)
	}

	wantCenterX := before.Viewport.CenterX / 2
	wantCenterY := before.Viewport.CenterY / 2
	if after.Viewport.CenterX != wantCenterX || after.Viewport.CenterY != wantCenterY {
		t.Fatalf("center after anchored zoom = %.2f, %.2f, want %.2f, %.2f", after.Viewport.CenterX, after.Viewport.CenterY, wantCenterX, wantCenterY)
	}
}

func TestTransactionGroupsMultipleViewportChangesIntoOneHistoryEntry(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandBeginTxn, mustJSON(t, BeginTransactionPayload{
		Description: "Zoom drag",
	})); err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 1.5})); err != nil {
		t.Fatalf("zoom 1: %v", err)
	}
	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 2})); err != nil {
		t.Fatalf("zoom 2: %v", err)
	}

	afterEnd, err := DispatchCommand(h, commandEndTxn, mustJSON(t, EndTransactionPayload{
		Commit: true,
	}))
	if err != nil {
		t.Fatalf("end transaction: %v", err)
	}

	if len(afterEnd.UIMeta.History) != 1 {
		t.Fatalf("history length = %d, want 1", len(afterEnd.UIMeta.History))
	}
	if afterEnd.UIMeta.History[0].Description != "Zoom drag" {
		t.Fatalf("history description = %q, want Zoom drag", afterEnd.UIMeta.History[0].Description)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if undone.Viewport.Zoom != 1 {
		t.Fatalf("zoom after undo = %.2f, want 1", undone.Viewport.Zoom)
	}
}

func TestJumpHistoryMovesLinearlyToTargetState(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 1.5})); err != nil {
		t.Fatalf("zoom: %v", err)
	}
	if _, err := DispatchCommand(h, commandRotateViewSet, mustJSON(t, RotatePayload{Rotation: 30})); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	latest, err := DispatchCommand(h, commandPanSet, mustJSON(t, PanPayload{CenterX: 200, CenterY: 150}))
	if err != nil {
		t.Fatalf("pan: %v", err)
	}
	if len(latest.UIMeta.History) != 3 || latest.UIMeta.CurrentHistoryIndex != 3 {
		t.Fatalf("history len/index = %d/%d, want 3/3", len(latest.UIMeta.History), latest.UIMeta.CurrentHistoryIndex)
	}

	jumpedBack, err := DispatchCommand(h, commandJumpHistory, mustJSON(t, JumpHistoryPayload{HistoryIndex: 1}))
	if err != nil {
		t.Fatalf("jump back: %v", err)
	}
	if jumpedBack.Viewport.Zoom != 1.5 || jumpedBack.Viewport.Rotation != 0 {
		t.Fatalf("jump back state = zoom %.2f rotation %.2f, want 1.5 / 0", jumpedBack.Viewport.Zoom, jumpedBack.Viewport.Rotation)
	}
	if jumpedBack.UIMeta.CurrentHistoryIndex != 1 {
		t.Fatalf("currentHistoryIndex = %d, want 1", jumpedBack.UIMeta.CurrentHistoryIndex)
	}
	if jumpedBack.UIMeta.History[0].State != "current" || jumpedBack.UIMeta.History[1].State != "undone" {
		t.Fatalf("unexpected history states after jump back: %+v", jumpedBack.UIMeta.History)
	}

	jumpedForward, err := DispatchCommand(h, commandJumpHistory, mustJSON(t, JumpHistoryPayload{HistoryIndex: 3}))
	if err != nil {
		t.Fatalf("jump forward: %v", err)
	}
	if jumpedForward.Viewport.CenterX != 200 || jumpedForward.Viewport.CenterY != 150 || jumpedForward.Viewport.Rotation != 30 {
		t.Fatalf("jump forward viewport = %+v, want restored latest state", jumpedForward.Viewport)
	}
}

func TestClearHistoryDropsUndoRedoButKeepsCurrentState(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 2})); err != nil {
		t.Fatalf("zoom: %v", err)
	}
	current, err := DispatchCommand(h, commandPanSet, mustJSON(t, PanPayload{CenterX: 320, CenterY: 180}))
	if err != nil {
		t.Fatalf("pan: %v", err)
	}
	if len(current.UIMeta.History) == 0 {
		t.Fatal("expected history entries before clear")
	}

	cleared, err := DispatchCommand(h, commandClearHistory, "")
	if err != nil {
		t.Fatalf("clear history: %v", err)
	}

	if len(cleared.UIMeta.History) != 0 {
		t.Fatalf("history length after clear = %d, want 0", len(cleared.UIMeta.History))
	}
	if cleared.UIMeta.CanUndo || cleared.UIMeta.CanRedo {
		t.Fatalf("canUndo/canRedo after clear = %v/%v, want false/false", cleared.UIMeta.CanUndo, cleared.UIMeta.CanRedo)
	}
	if cleared.Viewport.Zoom != 2 || cleared.Viewport.CenterX != 320 || cleared.Viewport.CenterY != 180 {
		t.Fatalf("viewport after clear = %+v, want preserved current state", cleared.Viewport)
	}
}

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
	h := Init("")
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
	h := Init("")
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

	// Add vector mask.
	after, err := DispatchCommand(h, commandAddVectorMask, mustJSON(t, AddVectorMaskPayload{LayerID: layerID}))
	if err != nil {
		t.Fatalf("add vector mask: %v", err)
	}
	layers := after.UIMeta.Layers
	if len(layers) == 0 || !layers[0].HasVectorMask {
		t.Fatal("expected layer to have a vector mask after AddVectorMask")
	}

	// Delete vector mask.
	after, err = DispatchCommand(h, commandDeleteVectorMask, mustJSON(t, DeleteVectorMaskPayload{LayerID: layerID}))
	if err != nil {
		t.Fatalf("delete vector mask: %v", err)
	}
	if len(after.UIMeta.Layers) > 0 && after.UIMeta.Layers[0].HasVectorMask {
		t.Fatal("expected vector mask to be removed after DeleteVectorMask")
	}

	// Undo delete restores mask.
	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(undone.UIMeta.Layers) == 0 || !undone.UIMeta.Layers[0].HasVectorMask {
		t.Fatal("expected vector mask restored after undo of DeleteVectorMask")
	}
}

func TestMaskEditModeNotTrackedInHistory(t *testing.T) {
	h := Init("")
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

	// Add a mask so we can enter mask edit mode.
	if _, err := DispatchCommand(h, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{
		LayerID: layerID,
		Mode:    AddLayerMaskRevealAll,
	})); err != nil {
		t.Fatalf("add mask: %v", err)
	}

	historyBefore := instances[h].history.CurrentIndex()

	// Enter mask edit mode.
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

	// History must not have grown — mask edit is not undoable.
	if instances[h].history.CurrentIndex() != historyBefore {
		t.Fatal("SetMaskEditMode should not add a history entry")
	}

	// Exit mask edit mode.
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
	h := Init("")
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

	// Rendering with a vector mask should succeed (mask silently ignored as placeholder).
	if _, err := RenderFrame(h); err != nil {
		t.Fatalf("RenderFrame with vector mask: %v", err)
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
