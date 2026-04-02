package engine

import (
	"encoding/json"
	"testing"
)

// initWithDefaultDoc creates an engine instance with a pre-existing 1920×1080 document.
// Use this in tests that require an active document to be present from the start.
func initWithDefaultDoc(t *testing.T) int32 {
	t.Helper()
	h := Init(`{"documentWidth":1920,"documentHeight":1080,"background":"transparent","resolution":72}`)
	if h <= 0 {
		t.Fatalf("initWithDefaultDoc: invalid handle %d", h)
	}
	return h
}

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
	h := initWithDefaultDoc(t)
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

func TestRenderCompositeSurfaceAppliesAdjustmentLayersNonDestructively(t *testing.T) {
	RegisterAdjustmentTransform("test-lift-red", func(r, g, b, a uint8, params json.RawMessage) (uint8, uint8, uint8, uint8, error) {
		var payload struct {
			Add [4]int `json:"add"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &payload); err != nil {
				return 0, 0, 0, 0, err
			}
		}
		return clampByteInt(int(r) + payload.Add[0]), clampByteInt(int(g) + payload.Add[1]), clampByteInt(int(b) + payload.Add[2]), clampByteInt(int(a) + payload.Add[3]), nil
	})
	defer RegisterAdjustmentTransform("test-lift-red", nil)

	doc := &Document{
		Width:      1,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Adjusted",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{10, 20, 30, 255})
	adjustment := NewAdjustmentLayer("Lift Red", "test-lift-red", json.RawMessage(`{"add":[20,0,0,0]}`))
	doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

	adjusted := doc.renderCompositeSurface()
	if got := adjusted[:4]; got[0] != 30 || got[1] != 20 || got[2] != 30 || got[3] != 255 {
		t.Fatalf("adjusted pixel = %v, want [30 20 30 255]", got)
	}

	versionBefore := doc.ContentVersion
	if err := doc.SetAdjustmentLayerParams(adjustment.ID(), "test-lift-red", json.RawMessage(`{"add":[40,0,0,0]}`)); err != nil {
		t.Fatalf("SetAdjustmentLayerParams: %v", err)
	}
	if doc.ContentVersion <= versionBefore {
		t.Fatalf("content version did not advance after adjustment params change: before=%d after=%d", versionBefore, doc.ContentVersion)
	}

	changed := doc.renderCompositeSurface()
	if got := changed[:4]; got[0] != 50 || got[1] != 20 || got[2] != 30 || got[3] != 255 {
		t.Fatalf("changed adjusted pixel = %v, want [50 20 30 255]", got)
	}

	adjustment.SetVisible(false)
	hidden := doc.renderCompositeSurface()
	if got := hidden[:4]; got[0] != 10 || got[1] != 20 || got[2] != 30 || got[3] != 255 {
		t.Fatalf("hidden adjustment pixel = %v, want original [10 20 30 255]", got)
	}
}

func TestAdjustmentParamsUndoRedoRestoresRenderAndMetadata(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	if _, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{
		CanvasW:          1,
		CanvasH:          1,
		DevicePixelRatio: 1,
	})); err != nil {
		t.Fatalf("resize: %v", err)
	}

	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{90, 70, 40, 255},
	}))
	if err != nil {
		t.Fatalf("add base layer: %v", err)
	}
	adjustment, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:      LayerTypeAdjustment,
		Name:           "Threshold",
		AdjustmentKind: "threshold",
		Params:         json.RawMessage(`{"threshold":255}`),
	}))
	if err != nil {
		t.Fatalf("add adjustment layer: %v", err)
	}
	adjustmentID := adjustment.UIMeta.ActiveLayerID

	doc := instances[h].manager.Active()
	black := doc.renderCompositeSurface()
	if got := black[:4]; got[0] != 0 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("initial render = %v, want black output", got)
	}

	updated, err := DispatchCommand(h, commandSetAdjustmentParams, mustJSON(t, SetAdjustmentParamsPayload{
		LayerID:        adjustmentID,
		AdjustmentKind: "threshold",
		Params:         json.RawMessage(`{"threshold":0}`),
	}))
	if err != nil {
		t.Fatalf("set adjustment params: %v", err)
	}
	if pixel, ok := findLayerMetaByID(updated.UIMeta.Layers, adjustmentID); !ok {
		t.Fatalf("adjustment layer %q missing after update", adjustmentID)
	} else if string(pixel.Params) != `{"threshold":0}` {
		t.Fatalf("updated params = %s, want threshold 0", string(pixel.Params))
	}
	doc = instances[h].manager.Active()
	white := doc.renderCompositeSurface()
	if got := white[:4]; got[0] != 255 || got[1] != 255 || got[2] != 255 || got[3] != 255 {
		t.Fatalf("updated render = %v, want white output", got)
	}

	undone, err := DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo adjustment params: %v", err)
	}
	if pixel, ok := findLayerMetaByID(undone.UIMeta.Layers, adjustmentID); !ok {
		t.Fatalf("adjustment layer %q missing after undo", adjustmentID)
	} else if string(pixel.Params) != `{"threshold":255}` {
		t.Fatalf("undo params = %s, want threshold 255", string(pixel.Params))
	}
	doc = instances[h].manager.Active()
	blackAgain := doc.renderCompositeSurface()
	if got := blackAgain[:4]; got[0] != 0 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("undo render = %v, want black output", got)
	}

	redone, err := DispatchCommand(h, commandRedo, "")
	if err != nil {
		t.Fatalf("redo adjustment params: %v", err)
	}
	if pixel, ok := findLayerMetaByID(redone.UIMeta.Layers, adjustmentID); !ok {
		t.Fatalf("adjustment layer %q missing after redo", adjustmentID)
	} else if string(pixel.Params) != `{"threshold":0}` {
		t.Fatalf("redo params = %s, want threshold 0", string(pixel.Params))
	}
	doc = instances[h].manager.Active()
	whiteAgain := doc.renderCompositeSurface()
	if got := whiteAgain[:4]; got[0] != 255 || got[1] != 255 || got[2] != 255 || got[3] != 255 {
		t.Fatalf("redo render = %v, want white output", got)
	}
}

func TestAdjustmentParamsLivePreviewInvalidatesCachedFrame(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	if _, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{
		CanvasW:          1,
		CanvasH:          1,
		DevicePixelRatio: 1,
	})); err != nil {
		t.Fatalf("resize: %v", err)
	}

	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 1, H: 1},
		Pixels:    []byte{90, 70, 40, 255},
	})); err != nil {
		t.Fatalf("add base layer: %v", err)
	}
	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType:      LayerTypeAdjustment,
		Name:           "Threshold",
		AdjustmentKind: "threshold",
		Params:         json.RawMessage(`{"threshold":255}`),
	}))
	if err != nil {
		t.Fatalf("add adjustment layer: %v", err)
	}
	adjustmentID := added.UIMeta.ActiveLayerID

	doc := instances[h].manager.Active()
	before := doc.renderCompositeSurface()
	if got := before[:4]; got[0] != 0 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("initial preview = %v, want black output", got)
	}
	versionBefore := doc.ContentVersion

	updated, err := DispatchCommand(h, commandSetAdjustmentParams, mustJSON(t, SetAdjustmentParamsPayload{
		LayerID:        adjustmentID,
		AdjustmentKind: "threshold",
		Params:         json.RawMessage(`{"threshold":0}`),
	}))
	if err != nil {
		t.Fatalf("set adjustment params: %v", err)
	}
	if pixel, ok := findLayerMetaByID(updated.UIMeta.Layers, adjustmentID); !ok {
		t.Fatalf("adjustment layer %q missing after update", adjustmentID)
	} else if string(pixel.Params) != `{"threshold":0}` {
		t.Fatalf("updated params = %s, want threshold 0", string(pixel.Params))
	}
	if updated.UIMeta.ContentVersion <= versionBefore {
		t.Fatalf("content version did not advance after adjustment change: before=%d after=%d", versionBefore, updated.UIMeta.ContentVersion)
	}

	afterRaw, err := RenderFrameRaw(h)
	if err != nil {
		t.Fatalf("RenderFrameRaw after change: %v", err)
	}
	if afterRaw.BufferLen != 4 {
		t.Fatalf("render after change length = %d, want 4", afterRaw.BufferLen)
	}
	doc = instances[h].manager.Active()
	pixels := doc.renderCompositeSurface()
	if got := pixels[:4]; got[0] != 255 || got[1] != 255 || got[2] != 255 || got[3] != 255 {
		t.Fatalf("render after change = %v, want white output", got)
	}
}

func TestLayerMaskCommandsUpdateMetadataAndUndo(t *testing.T) {
	h := initWithDefaultDoc(t)
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
	h := initWithDefaultDoc(t)
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

func clampByteInt(value int) uint8 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(value)
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
	h := initWithDefaultDoc(t)
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
