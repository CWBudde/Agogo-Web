package engine

import (
	"unsafe"

	aggrender "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/agg"
	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

// compositeSurface is a compatibility wrapper around compositeSurfaceChecked
// for sampling/paint-preview call sites that cannot propagate an error; it
// returns nil on failure. The frame render path uses compositeSurfaceChecked
// so composite errors reach the frontend.
func (inst *instance) compositeSurface(doc *Document) []byte {
	surface, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		return nil
	}
	return surface
}

func (inst *instance) compositeSurfaceChecked(doc *Document) ([]byte, error) {
	if doc == nil {
		inst.cachedDocSurface = nil
		inst.cachedDocID = ""
		inst.cachedDocContentVersion = 0
		return nil, nil
	}
	if inst.cachedDocID == doc.ID && inst.cachedDocContentVersion == doc.ContentVersion && len(inst.cachedDocSurface) > 0 {
		return inst.cachedDocSurface, nil
	}
	surface, err := doc.renderCompositeSurfaceChecked()
	if err != nil {
		// Never cache a failed composite: drop any stale cached surface so the
		// next render after the underlying problem is fixed recomputes instead
		// of serving a blank frame keyed to the same ContentVersion.
		inst.cachedDocSurface = nil
		inst.cachedDocID = ""
		inst.cachedDocContentVersion = 0
		return nil, err
	}
	inst.cachedDocSurface = surface
	inst.cachedDocID = doc.ID
	inst.cachedDocContentVersion = doc.ContentVersion
	doc.clearDirtyCompositeRect()
	return inst.cachedDocSurface, nil
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
		Error:       raw.Error,
	}
}

func (inst *instance) renderRaw() RawRenderResult {
	frameID := inst.nextFrameID()
	doc := inst.manager.activeMut()
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
			BufferPtr: int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
			BufferLen: int32(len(inst.pixels)),
			Reused:    true,
		}
	}

	surface, compositeErr := inst.compositeSurfaceChecked(doc)
	inst.pixels = inst.renderViewportWithCache(doc, surface)
	inst.pixels = RenderSelectionOverlay(doc, &inst.viewport, inst.pixels, doc.Selection, frameID, inst.selectionViewMode)
	inst.pixels = RenderTransformHandlesOverlay(inst.freeTransform, &inst.viewport, inst.pixels)
	inst.pixels = RenderCropOverlay(inst.crop, &inst.viewport, inst.pixels)
	if compositeErr != nil {
		// Do not cache a frame rendered from a failed composite: once the
		// underlying problem is fixed the next render must recompute rather
		// than reuse the blank frame. The error is surfaced to the frontend
		// via RenderResult.Error.
		inst.hasCachedRawFrame = false
		return RawRenderResult{
			FrameID:   frameID,
			Viewport:  inst.viewport,
			BufferPtr: int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
			BufferLen: int32(len(inst.pixels)),
			Reused:    false,
			Error:     compositeErr.Error(),
		}
	}
	inst.cachedRawFrameKey = key
	inst.hasCachedRawFrame = inst.canReuseRawFrame(doc)
	return RawRenderResult{
		FrameID:   frameID,
		Viewport:  inst.viewport,
		BufferPtr: int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
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
	doc := inst.manager.activeMut()
	if doc == nil {
		return UIMeta{
			CursorType:          "default",
			StatusText:          "No active document",
			ImportWarnings:      append([]string(nil), inst.importWarnings...),
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
		ActiveLayerID:          doc.ActiveLayerID,
		ActiveLayerName:        activeLayerName,
		CursorType:             inst.cursorType(),
		StatusText:             inst.statusText(doc),
		ImportWarnings:         append([]string(nil), inst.importWarnings...),
		RulerOriginX:           0,
		RulerOriginY:           0,
		History:                inst.history.Entries(),
		CurrentHistoryIndex:    inst.history.CurrentIndex(),
		CanUndo:                inst.history.CanUndo(),
		CanRedo:                inst.history.CanRedo(),
		ActiveDocumentID:       doc.ID,
		ActiveDocumentName:     doc.Name,
		DocumentWidth:          doc.Width,
		DocumentHeight:         doc.Height,
		DocumentBackground:     doc.Background.Kind,
		Layers:                 docpkg.BuildLayerMeta(doc.ensureLayerRoot().Children()),
		ContentVersion:         doc.ContentVersion,
		MaskEditLayerID:        inst.maskEditLayerID,
		Selection:              docpkg.BuildSelectionMeta(doc.Selection, doc.LastSelection),
		SavedSelectionChannels: docpkg.BuildSavedSelectionChannelMeta(doc.SavedSelections),
		FreeTransform:          inst.freeTransform.meta(),
		Crop:                   inst.crop.meta(),
		Paths:                  docpkg.BuildPathsMeta(doc.Paths, doc.ActivePathIdx),
		PathOverlay:            inst.buildPathOverlay(),
		EditingVectorLayerID:   inst.editingVectorLayerID,
		EditingTextLayerID:     inst.textEdit.layerID,
		TextCursorX:            inst.textCursorX(doc),
		TextCursorY:            inst.textCursorY(doc),
		StylePresets:           cloneDocumentStylePresets(doc.StylePresets),
	}
}

// textCursorX returns the doc-space X coordinate of the text insertion cursor.
// Returns 0 when no text layer is being edited.
func (inst *instance) textCursorX(doc *Document) float64 {
	if inst.textEdit.layerID == "" || doc == nil {
		return 0
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), inst.textEdit.layerID)
	if !ok {
		return 0
	}
	tl, ok := layer.(*TextLayer)
	if !ok {
		return 0
	}
	textWidth := measureTextWidth(inst.textEdit.workingText, tl.FontSize)
	return float64(tl.Bounds.X) + textWidth
}

// textCursorY returns the doc-space Y coordinate of the text insertion cursor baseline.
func (inst *instance) textCursorY(doc *Document) float64 {
	if inst.textEdit.layerID == "" || doc == nil {
		return 0
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), inst.textEdit.layerID)
	if !ok {
		return 0
	}
	tl, ok := layer.(*TextLayer)
	if !ok {
		return 0
	}
	return float64(tl.Bounds.Y) + tl.FontSize
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
