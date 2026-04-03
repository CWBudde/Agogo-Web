package engine

import (
	"unsafe"

	aggrender "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/agg"
)

func (inst *instance) compositeSurface(doc *Document) []byte {
	if doc == nil {
		inst.cachedDocSurface = nil
		inst.cachedDocID = ""
		inst.cachedDocContentVersion = 0
		return nil
	}
	if inst.cachedDocID == doc.ID && inst.cachedDocContentVersion == doc.ContentVersion && len(inst.cachedDocSurface) > 0 {
		return inst.cachedDocSurface
	}
	inst.cachedDocSurface = doc.renderCompositeSurface()
	inst.cachedDocID = doc.ID
	inst.cachedDocContentVersion = doc.ContentVersion
	return inst.cachedDocSurface
}

func (inst *instance) render() RenderResult {
	raw := inst.renderRaw()
	uiMeta := inst.renderUIMeta()
	return RenderResult{
		FrameID:     raw.FrameID,
		Viewport:    raw.Viewport,
		DirtyRects:  []DirtyRect{{X: 0, Y: 0, W: inst.viewport.CanvasW, H: inst.viewport.CanvasH}},
		PixelFormat: "rgba8-premultiplied",
		BufferPtr:   raw.BufferPtr,
		BufferLen:   raw.BufferLen,
		UIMeta:      uiMeta,
	}
}

func (inst *instance) renderRaw() RawRenderResult {
	frameID := inst.nextFrameID()
	doc := inst.manager.Active()
	if doc == nil {
		inst.pixels = inst.pixels[:0]
		inst.hasCachedRawFrame = false
		return RawRenderResult{FrameID: frameID, Viewport: inst.viewport}
	}

	key := rawFrameKey{
		DocID:            doc.ID,
		ContentVersion:   doc.ContentVersion,
		CenterX:          inst.viewport.CenterX,
		CenterY:          inst.viewport.CenterY,
		Zoom:             inst.viewport.Zoom,
		Rotation:         inst.viewport.Rotation,
		CanvasW:          inst.viewport.CanvasW,
		CanvasH:          inst.viewport.CanvasH,
		DevicePixelRatio: inst.viewport.DevicePixelRatio,
		ShowGuides:       inst.viewport.ShowGuides,
	}
	if inst.canReuseRawFrame(doc) && inst.hasCachedRawFrame && inst.cachedRawFrameKey == key && len(inst.pixels) > 0 {
		return RawRenderResult{
			FrameID:   frameID,
			Viewport:  inst.viewport,
			BufferPtr: int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:unsafeptr
			BufferLen: int32(len(inst.pixels)),
			Reused:    true,
		}
	}

	inst.pixels = inst.renderViewportWithCache(doc, inst.compositeSurface(doc))
	inst.pixels = RenderSelectionOverlay(doc, &inst.viewport, inst.pixels, doc.Selection, frameID)
	inst.pixels = RenderTransformHandlesOverlay(inst.freeTransform, &inst.viewport, inst.pixels)
	inst.pixels = RenderCropOverlay(inst.crop, &inst.viewport, inst.pixels)
	inst.cachedRawFrameKey = key
	inst.hasCachedRawFrame = inst.canReuseRawFrame(doc)
	return RawRenderResult{
		FrameID:   frameID,
		Viewport:  inst.viewport,
		BufferPtr: int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:unsafeptr
		BufferLen: int32(len(inst.pixels)),
		Reused:    false,
	}
}

func (inst *instance) canReuseRawFrame(doc *Document) bool {
	if doc == nil || len(inst.pixels) == 0 {
		return false
	}
	if doc.Selection != nil {
		return false
	}
	if inst.freeTransform != nil && inst.freeTransform.Active {
		return false
	}
	if inst.crop != nil && inst.crop.Active {
		return false
	}
	return true
}

func (inst *instance) renderUIMeta() UIMeta {
	doc := inst.manager.Active()
	if doc == nil {
		return UIMeta{
			CursorType:          "default",
			StatusText:          "No active document",
			History:             inst.history.Entries(),
			CurrentHistoryIndex: inst.history.CurrentIndex(),
			CanUndo:             inst.history.CanUndo(),
			CanRedo:             inst.history.CanRedo(),
			MaskEditLayerID:     inst.maskEditLayerID,
		}
	}

	activeLayerName := ""
	if activeLayer := doc.ActiveLayer(); activeLayer != nil {
		activeLayerName = activeLayer.Name()
	}

	return UIMeta{
		ActiveLayerID:       doc.ActiveLayerID,
		ActiveLayerName:     activeLayerName,
		CursorType:          inst.cursorType(),
		StatusText:          inst.statusText(doc),
		RulerOriginX:        0,
		RulerOriginY:        0,
		History:             inst.history.Entries(),
		CurrentHistoryIndex: inst.history.CurrentIndex(),
		CanUndo:             inst.history.CanUndo(),
		CanRedo:             inst.history.CanRedo(),
		ActiveDocumentID:    doc.ID,
		ActiveDocumentName:  doc.Name,
		DocumentWidth:       doc.Width,
		DocumentHeight:      doc.Height,
		DocumentBackground:  doc.Background.Kind,
		Layers:              doc.LayerMeta(),
		ContentVersion:      doc.ContentVersion,
		MaskEditLayerID:     inst.maskEditLayerID,
		Selection:           doc.selectionMeta(),
		FreeTransform:       inst.freeTransform.meta(),
		Crop:                inst.crop.meta(),
		Paths:               doc.pathsMeta(),
		PathOverlay:          inst.buildPathOverlay(),
	}
}

func (inst *instance) renderViewportWithCache(doc *Document, documentSurface []byte) []byte {
	vp := &inst.viewport
	key := viewportBaseKey{
		DocWidth:   doc.Width,
		DocHeight:  doc.Height,
		Background: doc.Background.Kind,
		CenterX:    vp.CenterX,
		CenterY:    vp.CenterY,
		Zoom:       clampZoom(vp.Zoom),
		Rotation:   vp.Rotation,
		CanvasW:    vp.CanvasW,
		CanvasH:    vp.CanvasH,
	}

	canvasSize := maxInt(vp.CanvasW, 1) * maxInt(vp.CanvasH, 1) * 4

	if key == inst.cachedViewportBaseKey && len(inst.cachedViewportBase) == canvasSize {
		if len(inst.pixels) != canvasSize {
			inst.pixels = make([]byte, canvasSize)
		}
		copy(inst.pixels, inst.cachedViewportBase)
	} else {
		inst.pixels = aggrender.RenderViewportBase(
			&aggrender.Document{
				Width:      doc.Width,
				Height:     doc.Height,
				Background: doc.Background.Kind,
			},
			&aggrender.Viewport{
				CenterX:  key.CenterX,
				CenterY:  key.CenterY,
				Zoom:     key.Zoom,
				Rotation: key.Rotation,
				CanvasW:  key.CanvasW,
				CanvasH:  key.CanvasH,
			},
			inst.pixels,
		)
		if len(inst.cachedViewportBase) != canvasSize {
			inst.cachedViewportBase = make([]byte, canvasSize)
		}
		copy(inst.cachedViewportBase, inst.pixels)
		inst.cachedViewportBaseKey = key
	}

	if len(documentSurface) > 0 {
		compositeDocumentToViewport(inst.pixels, maxInt(vp.CanvasW, 1), maxInt(vp.CanvasH, 1), doc, vp, documentSurface)
	}

	return aggrender.RenderViewportOverlays(
		&aggrender.Document{
			Width:      doc.Width,
			Height:     doc.Height,
			Background: doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:    vp.CenterX,
			CenterY:    vp.CenterY,
			Zoom:       clampZoom(vp.Zoom),
			Rotation:   vp.Rotation,
			CanvasW:    vp.CanvasW,
			CanvasH:    vp.CanvasH,
			ShowGuides: vp.ShowGuides,
		},
		inst.pixels,
	)
}

// RenderViewport renders the document shell and the current composited layer tree.
// documentSurface is the precomputed RGBA composite for the full document; pass nil
// to skip layer compositing (e.g. when there are no layers).
func RenderViewport(doc *Document, vp *ViewportState, reuse []byte, documentSurface []byte) []byte {
	reuse = aggrender.RenderViewportBase(
		&aggrender.Document{
			Width:      doc.Width,
			Height:     doc.Height,
			Background: doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:  vp.CenterX,
			CenterY:  vp.CenterY,
			Zoom:     clampZoom(vp.Zoom),
			Rotation: vp.Rotation,
			CanvasW:  vp.CanvasW,
			CanvasH:  vp.CanvasH,
		},
		reuse,
	)

	if len(documentSurface) > 0 {
		compositeDocumentToViewport(reuse, maxInt(vp.CanvasW, 1), maxInt(vp.CanvasH, 1), doc, vp, documentSurface)
	}

	return aggrender.RenderViewportOverlays(
		&aggrender.Document{
			Width:      doc.Width,
			Height:     doc.Height,
			Background: doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:    vp.CenterX,
			CenterY:    vp.CenterY,
			Zoom:       clampZoom(vp.Zoom),
			Rotation:   vp.Rotation,
			CanvasW:    vp.CanvasW,
			CanvasH:    vp.CanvasH,
			ShowGuides: vp.ShowGuides,
		},
		reuse,
	)
}
