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
		inst.dropDocSurfaceCache()
		return nil, nil
	}
	if inst.cachedDocID == doc.ID && inst.cachedDocContentVersion == doc.ContentVersion && len(inst.cachedDocSurface) > 0 {
		return inst.cachedDocSurface, nil
	}

	surfaceSize := doc.Width * doc.Height * 4
	canReuseBuffer := inst.cachedDocID == doc.ID && surfaceSize > 0 && len(inst.cachedDocSurface) == surfaceSize

	// Incremental path (PLAN.md S.4): the cached surface holds the previous
	// full composite for this document, and only a sub-rect changed since —
	// zero that rect and recomposite the layer stack clipped to it, in place.
	// The dirty rect is only trusted when it accumulated exactly since the
	// cached composite (dirtyCompositeBase); snapshot restores can install a
	// clone whose dirty state is unrelated to the cached surface.
	if canReuseBuffer && doc.dirtyCompositeBase == inst.cachedDocContentVersion {
		if rect, ok := doc.incrementalRecompositeRect(); ok {
			if err := doc.recompositeSurfaceRect(inst.cachedDocSurface, rect); err != nil {
				// Never cache a failed composite: the rect region is now
				// partially written, so the surface must be rebuilt from
				// scratch once the underlying problem is fixed.
				inst.dropDocSurfaceCache()
				return nil, err
			}
			inst.incrementalCompositeCount++
			inst.cachedDocContentVersion = doc.ContentVersion
			doc.clearDirtyCompositeRect()
			return inst.cachedDocSurface, nil
		}
	}

	// Full recomposite. Reuse the cached buffer in place when the dimensions
	// still match (zero + composite) instead of allocating a fresh W×H×4
	// buffer per content change; other renderLayersToSurface* callers
	// (merge/flatten/PSD export) keep allocating because their results escape.
	var surface []byte
	var err error
	if canReuseBuffer {
		surface = inst.cachedDocSurface
		clear(surface)
		err = doc.renderCompositeSurfaceInto(surface)
	} else {
		surface, err = doc.renderCompositeSurfaceChecked()
	}
	if err != nil {
		// Never cache a failed composite: drop any stale cached surface so the
		// next render after the underlying problem is fixed recomputes instead
		// of serving a blank frame keyed to the same ContentVersion.
		inst.dropDocSurfaceCache()
		return nil, err
	}
	inst.cachedDocSurface = surface
	inst.cachedDocID = doc.ID
	inst.cachedDocContentVersion = doc.ContentVersion
	doc.clearDirtyCompositeRect()
	return inst.cachedDocSurface, nil
}

// dropDocSurfaceCache invalidates the cached document composite surface.
func (inst *instance) dropDocSurfaceCache() {
	inst.cachedDocSurface = nil
	inst.cachedDocID = ""
	inst.cachedDocContentVersion = 0
}

func (inst *instance) render() RenderResult {
	raw := inst.renderRaw()
	uiMeta := inst.renderUIMeta()
	dirtyRects := raw.DirtyRects
	if len(dirtyRects) == 0 && !raw.Reused {
		// Non-reused frames without explicit rects (e.g. the no-document
		// frame) fall back to the historical full-canvas rect.
		dirtyRects = []DirtyRect{{X: 0, Y: 0, W: inst.viewport.CanvasW, H: inst.viewport.CanvasH}}
	}
	return RenderResult{
		FrameID:       raw.FrameID,
		Viewport:      raw.Viewport,
		DirtyRects:    dirtyRects,
		PixelFormat:   "rgba8-premultiplied",
		BufferPtr:     raw.BufferPtr,
		BufferLen:     raw.BufferLen,
		UIMeta:        &uiMeta,
		UIMetaVersion: inst.uiMetaVersion,
		Error:         raw.Error,
	}
}

// fullCanvasDirtyRects returns the single full-canvas dirty rect used by
// render paths that rewrite the whole frame.
func (inst *instance) fullCanvasDirtyRects() []DirtyRect {
	return []DirtyRect{{X: 0, Y: 0, W: inst.viewport.CanvasW, H: inst.viewport.CanvasH}}
}

func (inst *instance) renderRaw() RawRenderResult {
	frameID := inst.nextFrameID()
	doc := inst.manager.activeMut()
	if doc == nil {
		inst.pixels = inst.pixels[:0]
		inst.hasCachedRawFrame = false
		inst.hasCachedAnimBase = false
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
	// Zero-copy reuse: no selection and no active transform/crop, so the frame
	// is byte-for-byte identical to the last one. The frontend skips the blit.
	if inst.canReuseRawFrame(doc) && inst.hasCachedRawFrame && inst.cachedRawFrameKey == key && len(inst.pixels) > 0 {
		return RawRenderResult{
			FrameID:   frameID,
			Viewport:  inst.viewport,
			BufferPtr: int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
			BufferLen: int32(len(inst.pixels)),
			Reused:    true,
		}
	}

	// Cheap ants path: content/viewport unchanged and only the animated
	// selection overlay differs frame-to-frame. Copy the cached base (which
	// excludes the animated overlay) and stamp the current ants phase on top —
	// a memcpy + one overlay pass instead of a full recomposite + resample.
	// The frame changes each frame (ants phase), so Reused MUST stay false or
	// the frontend would skip blitting the animation.
	if inst.canCacheAnimBase(doc) && inst.hasCachedAnimBase && inst.cachedAnimBaseKey == key && len(inst.cachedAnimBase) > 0 {
		if len(inst.pixels) != len(inst.cachedAnimBase) {
			inst.pixels = make([]byte, len(inst.cachedAnimBase))
		}
		copy(inst.pixels, inst.cachedAnimBase)
		inst.pixels = RenderSelectionOverlay(doc, &inst.viewport, inst.pixels, doc.Selection, frameID, inst.selectionViewMode)
		// Transform/crop overlays are inactive whenever canCacheAnimBase is
		// true, so applying them here is a no-op; call them anyway to preserve
		// the exact z-order of the full-render path.
		inst.pixels = RenderTransformHandlesOverlay(inst.freeTransform, &inst.viewport, inst.pixels)
		inst.pixels = RenderCropOverlay(inst.crop, &inst.viewport, inst.pixels)
		return RawRenderResult{
			FrameID:  frameID,
			Viewport: inst.viewport,
			// Tracking the union of previous + current ants rects is not
			// worth the complexity yet; report the full canvas.
			DirtyRects: inst.fullCanvasDirtyRects(),
			BufferPtr:  int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
			BufferLen:  int32(len(inst.pixels)),
			Reused:     false,
		}
	}

	// Partial viewport resample: only a doc-space sub-rect changed and the
	// viewport is identical to the previously rendered frame — redraw just the
	// projected canvas rectangle on top of the previous frame.
	if result, ok := inst.tryPartialContentRender(doc, key, frameID); ok {
		return result
	}

	inst.fullRecompositeCount++
	surface, compositeErr := inst.compositeSurfaceChecked(doc)
	inst.pixels = inst.renderViewportWithCache(doc, surface)
	// Snapshot the base BEFORE the animated selection overlay so future frames
	// can stamp only the ants on top. Only cache when eligible (a selection is
	// active and no transform/crop drag is in progress).
	if compositeErr == nil && inst.canCacheAnimBase(doc) {
		if len(inst.cachedAnimBase) != len(inst.pixels) {
			inst.cachedAnimBase = make([]byte, len(inst.pixels))
		}
		copy(inst.cachedAnimBase, inst.pixels)
		inst.cachedAnimBaseKey = key
		inst.hasCachedAnimBase = true
	} else {
		inst.hasCachedAnimBase = false
	}
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
			FrameID:    frameID,
			Viewport:   inst.viewport,
			DirtyRects: inst.fullCanvasDirtyRects(),
			BufferPtr:  int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
			BufferLen:  int32(len(inst.pixels)),
			Reused:     false,
			Error:      compositeErr.Error(),
		}
	}
	inst.cachedRawFrameKey = key
	inst.hasCachedRawFrame = inst.canReuseRawFrame(doc)
	return RawRenderResult{
		FrameID:    frameID,
		Viewport:   inst.viewport,
		DirtyRects: inst.fullCanvasDirtyRects(),
		BufferPtr:  int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
		BufferLen:  int32(len(inst.pixels)),
		Reused:     false,
	}
}

// tryPartialContentRender implements the partial viewport resample tier of
// renderRaw (PLAN.md S.4). It succeeds only when it is provably safe to
// redraw a sub-rectangle of the previous frame in place:
//
//   - the previous frame is cached and overlay-free (no selection, no active
//     transform/crop — canReuseRawFrame),
//   - the viewport (pan/zoom/rotation/canvas/DPR/guides) is unchanged and only
//     ContentVersion differs,
//   - the cached document surface matches the previous frame's content version,
//     so the accumulated dirty rect covers exactly what changed on screen,
//   - the dirty rect stays clear of the document border and guides
//     (partialOverlaySafe) — overlays are NOT redrawn inside the rect,
//   - the cached viewport base is valid (base rows are copied from it).
//
// On success the redrawn canvas rect is reported via DirtyRects so the
// frontend can blit just that region.
func (inst *instance) tryPartialContentRender(doc *Document, key rawFrameKey, frameID int64) (RawRenderResult, bool) {
	if !inst.hasCachedRawFrame || !inst.canReuseRawFrame(doc) {
		return RawRenderResult{}, false
	}
	prevKey := inst.cachedRawFrameKey
	if prevKey.ContentVersion == key.ContentVersion {
		return RawRenderResult{}, false
	}
	viewportOnly := prevKey
	viewportOnly.ContentVersion = key.ContentVersion
	if viewportOnly != key {
		return RawRenderResult{}, false
	}
	canvasW := maxInt(key.CanvasW, 1)
	canvasH := maxInt(key.CanvasH, 1)
	canvasSize := canvasW * canvasH * 4
	if len(inst.pixels) != canvasSize {
		return RawRenderResult{}, false
	}
	// The dirty rect accumulates changes since the last successful composite.
	// It describes the delta to the previous FRAME only if the cached document
	// surface still matches that frame's content version (a mid-command
	// compositeSurface call would have consumed part of the delta otherwise)
	// AND the rect provably accumulated from that version on
	// (dirtyCompositeBase — snapshot restores clone stale dirty state).
	if inst.cachedDocID != doc.ID || inst.cachedDocContentVersion != prevKey.ContentVersion {
		return RawRenderResult{}, false
	}
	if doc.dirtyCompositeBase != prevKey.ContentVersion {
		return RawRenderResult{}, false
	}
	dirtyPtr := doc.currentDirtyCompositeRect()
	if dirtyPtr == nil {
		return RawRenderResult{}, false
	}
	dirtyDoc := *dirtyPtr
	vp := &inst.viewport
	zoom := clampZoom(vp.Zoom)
	if !partialOverlaySafe(doc, vp, zoom, dirtyDoc) {
		return RawRenderResult{}, false
	}
	baseKey := viewportBaseKey{
		DocWidth:   doc.Width,
		DocHeight:  doc.Height,
		Background: doc.Background.Kind,
		CenterX:    vp.CenterX,
		CenterY:    vp.CenterY,
		Zoom:       zoom,
		Rotation:   vp.Rotation,
		CanvasW:    vp.CanvasW,
		CanvasH:    vp.CanvasH,
	}
	if baseKey != inst.cachedViewportBaseKey || len(inst.cachedViewportBase) != canvasSize {
		return RawRenderResult{}, false
	}

	surface, err := inst.compositeSurfaceChecked(doc)
	if err != nil || len(surface) != doc.Width*doc.Height*4 {
		// Fall back to the full path, which re-runs compositeSurfaceChecked
		// (cache hit on success, error surfaced to the frontend on failure).
		return RawRenderResult{}, false
	}

	canvasRect, ok := docRectToCanvasRect(vp, zoom, dirtyDoc, canvasW, canvasH)
	if !ok {
		// The change is entirely off-canvas: the previous frame is still
		// byte-exact for the new content version.
		inst.cachedRawFrameKey = key
		return RawRenderResult{
			FrameID:   frameID,
			Viewport:  inst.viewport,
			BufferPtr: int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
			BufferLen: int32(len(inst.pixels)),
			Reused:    true,
		}, true
	}

	// Restore the background/base inside the rect, then re-composite the
	// document restricted to it. Overlays are guaranteed not to intersect
	// (partialOverlaySafe), so the surrounding frame pixels stay valid.
	rowBytes := canvasW * 4
	for y := canvasRect.Y; y < canvasRect.Y+canvasRect.H; y++ {
		start := y*rowBytes + canvasRect.X*4
		copy(inst.pixels[start:start+canvasRect.W*4], inst.cachedViewportBase[start:start+canvasRect.W*4])
	}
	compositeDocumentToViewportClipped(inst.pixels, canvasW, canvasH, doc, vp, surface, &canvasRect)

	inst.partialViewportUpdateCount++
	inst.cachedRawFrameKey = key
	inst.hasCachedRawFrame = true
	inst.hasCachedAnimBase = false
	return RawRenderResult{
		FrameID:    frameID,
		Viewport:   inst.viewport,
		DirtyRects: []DirtyRect{canvasRect},
		BufferPtr:  int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:govet // intentional Wasm ABI pointer handoff to JS
		BufferLen:  int32(len(inst.pixels)),
		Reused:     false,
	}, true
}

// canReuseRawFrame reports whether the last rendered frame can be handed back
// byte-for-byte (zero-copy, Reused=true). An active selection animates marching
// ants each frame, so it disqualifies zero-copy reuse — the cheaper ants path
// (see canCacheAnimBase) handles that case instead.
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

// canCacheAnimBase reports whether the animated-selection base cache is usable
// for the current frame: a selection must be active (otherwise the zero-copy
// path applies) and no transform/crop drag may be in progress, since those
// overlays change geometry without bumping ContentVersion and are therefore not
// captured by rawFrameKey.
func (inst *instance) canCacheAnimBase(doc *Document) bool {
	if doc == nil || doc.Selection == nil {
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
			Version:             inst.uiMetaVersion,
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
		Version:                inst.uiMetaVersion,
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
