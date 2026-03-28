package engine

import (
	"encoding/json"
	"math"
	"strings"
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

func TestPickLayerAtPointAndTranslateLayer(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Move Tool",
		Width:      8,
		Height:     8,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "transparent",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	base, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 1, Y: 1, W: 1, H: 1},
		Pixels:    []byte{255, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add base layer: %v", err)
	}
	baseID := base.UIMeta.ActiveLayerID

	top, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Top",
		Bounds:    LayerBounds{X: 1, Y: 1, W: 1, H: 1},
		Pixels:    []byte{0, 255, 0, 255},
	}))
	if err != nil {
		t.Fatalf("add top layer: %v", err)
	}
	topID := top.UIMeta.ActiveLayerID
	historyBeforePick := len(top.UIMeta.History)

	picked, err := DispatchCommand(h, commandPickLayerAtPoint, mustJSON(t, PickLayerAtPointPayload{X: 1, Y: 1}))
	if err != nil {
		t.Fatalf("pick layer at point: %v", err)
	}
	if picked.UIMeta.ActiveLayerID != topID {
		t.Fatalf("picked active layer = %q, want %q", picked.UIMeta.ActiveLayerID, topID)
	}
	if len(picked.UIMeta.History) != historyBeforePick {
		t.Fatal("pick layer at point should not add a history entry")
	}

	if _, err := DispatchCommand(h, commandBeginTxn, mustJSON(t, BeginTransactionPayload{Description: "Move layer"})); err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if _, err := DispatchCommand(h, commandTranslateLayer, mustJSON(t, TranslateLayerPayload{DX: 2, DY: 1})); err != nil {
		t.Fatalf("translate layer: %v", err)
	}
	committed, err := DispatchCommand(h, commandEndTxn, mustJSON(t, EndTransactionPayload{Commit: true}))
	if err != nil {
		t.Fatalf("end transaction: %v", err)
	}
	if committed.UIMeta.ActiveLayerID != topID {
		t.Fatalf("active layer after move = %q, want %q", committed.UIMeta.ActiveLayerID, topID)
	}

	doc := instances[h].manager.Active()
	surface := doc.renderCompositeSurface()
	if r, g, b, a := pixelAt(surface, doc.Width, 1, 1); [4]byte{r, g, b, a} != [4]byte{255, 0, 0, 255} {
		t.Fatalf("pixel at old top position = [%d %d %d %d], want [255 0 0 255]", r, g, b, a)
	}
	if r, g, b, a := pixelAt(surface, doc.Width, 3, 2); [4]byte{r, g, b, a} != [4]byte{0, 255, 0, 255} {
		t.Fatalf("pixel at moved top position = [%d %d %d %d], want [0 255 0 255]", r, g, b, a)
	}
	if doc.ActiveLayerID != topID {
		t.Fatalf("doc active layer = %q, want %q", doc.ActiveLayerID, topID)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo move: %v", err)
	}
	if undone.UIMeta.ActiveLayerID != topID {
		t.Fatalf("active layer after undo = %q, want %q", undone.UIMeta.ActiveLayerID, topID)
	}
	doc = instances[h].manager.Active()
	surface = doc.renderCompositeSurface()
	if r, g, b, a := pixelAt(surface, doc.Width, 1, 1); [4]byte{r, g, b, a} != [4]byte{0, 255, 0, 255} {
		t.Fatalf("pixel after undo = [%d %d %d %d], want [0 255 0 255]", r, g, b, a)
	}
	if _, _, _, ok := findLayerByID(doc.ensureLayerRoot(), baseID); !ok {
		t.Fatalf("base layer %q missing after move workflow", baseID)
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

	wantCenterX := before.Viewport.CenterX * 0.5
	wantCenterY := before.Viewport.CenterY * 0.5
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

func TestTransactionRedoRestoresGroupedCommandState(t *testing.T) {
	h := Init("")
	defer Free(h)

	before, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("render before: %v", err)
	}

	if _, err := DispatchCommand(h, commandBeginTxn, mustJSON(t, BeginTransactionPayload{Description: "Viewport drag"})); err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	if _, err := DispatchCommand(h, commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 1.5})); err != nil {
		t.Fatalf("zoom in transaction: %v", err)
	}
	afterCommit, err := DispatchCommand(h, commandPanSet, mustJSON(t, PanPayload{CenterX: 240, CenterY: 180}))
	if err != nil {
		t.Fatalf("pan in transaction: %v", err)
	}
	if _, err := DispatchCommand(h, commandEndTxn, mustJSON(t, EndTransactionPayload{Commit: true})); err != nil {
		t.Fatalf("end transaction: %v", err)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo transaction: %v", err)
	}
	if undone.Viewport.Zoom != before.Viewport.Zoom || undone.Viewport.CenterX != before.Viewport.CenterX || undone.Viewport.CenterY != before.Viewport.CenterY {
		t.Fatalf("undo transaction viewport = %+v, want %+v", undone.Viewport, before.Viewport)
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo transaction: %v", err)
	}
	if redone.Viewport.Zoom != afterCommit.Viewport.Zoom || redone.Viewport.CenterX != afterCommit.Viewport.CenterX || redone.Viewport.CenterY != afterCommit.Viewport.CenterY {
		t.Fatalf("redo transaction viewport = %+v, want %+v", redone.Viewport, afterCommit.Viewport)
	}
	if redone.UIMeta.CurrentHistoryIndex != 1 || len(redone.UIMeta.History) != 1 || redone.UIMeta.History[0].State != "current" {
		t.Fatalf("unexpected history after redo transaction: %+v index=%d", redone.UIMeta.History, redone.UIMeta.CurrentHistoryIndex)
	}
}

func TestHistoryStackHandlesDiscardedTransactionsAndNoopNavigation(t *testing.T) {
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: 100, CanvasH: 100, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("history-doc", "History", 100, 100)
	inst.manager.Create(doc)

	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo on empty history: %v", err)
	}
	if err := inst.history.Redo(inst); err != nil {
		t.Fatalf("Redo on empty history: %v", err)
	}
	if err := inst.history.JumpTo(inst, -3); err != nil {
		t.Fatalf("JumpTo negative index: %v", err)
	}

	inst.history.BeginTransaction(inst, "outer")
	inst.history.BeginTransaction(inst, "inner ignored")
	if inst.history.active == nil || inst.history.active.description != "outer" {
		t.Fatalf("nested BeginTransaction should preserve the original transaction, got %+v", inst.history.active)
	}

	command := &snapshotCommand{
		description: "Zoom without commit",
		applyFn: func(inst *instance) (snapshot, error) {
			inst.viewport.Zoom = 2
			return inst.captureSnapshot(), nil
		},
	}
	if err := inst.history.Execute(inst, command); err != nil {
		t.Fatalf("Execute in active transaction: %v", err)
	}
	inst.history.EndTransaction(false)
	if inst.history.CurrentIndex() != 0 || len(inst.history.Entries()) != 0 {
		t.Fatalf("discarded transaction should not add history entries, got index=%d entries=%+v", inst.history.CurrentIndex(), inst.history.Entries())
	}
	if inst.viewport.Zoom != 2 {
		t.Fatalf("discarded transaction should keep the current state change, zoom=%.2f", inst.viewport.Zoom)
	}

	inst.history.BeginTransaction(inst, "noop")
	inst.history.EndTransaction(true)
	if inst.history.CurrentIndex() != 0 || len(inst.history.Entries()) != 0 {
		t.Fatalf("no-op committed transaction should not add entries, got index=%d entries=%+v", inst.history.CurrentIndex(), inst.history.Entries())
	}

	if err := inst.history.JumpTo(inst, 99); err != nil {
		t.Fatalf("JumpTo out-of-range index should clamp, got error: %v", err)
	}
	if inst.history.CurrentIndex() != 0 {
		t.Fatalf("JumpTo on empty history should keep current index at 0, got %d", inst.history.CurrentIndex())
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
	h := Init("")
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
	h := Init("")
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
