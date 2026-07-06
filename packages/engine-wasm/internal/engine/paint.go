package engine

import (
	"fmt"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

// buildStrokeDelta builds the undo delta for a finished brush stroke. It is a
// package variable (defaulting to newPixelDeltaFromRows) so tests can force the
// otherwise hard-to-trigger failure path of handleEndPaintStroke.
var buildStrokeDelta = newPixelDeltaFromRows

func (inst *instance) handleBeginPaintStroke(p BeginPaintStrokePayload) {
	doc := inst.manager.activeMut()
	if doc == nil {
		return
	}
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		// The active layer exists but is not a pixel layer (text, vector,
		// adjustment, group). The begin/continue dispatch path discards return
		// values, so record a rejected stroke; handleEndPaintStroke surfaces
		// the "must be rasterized" error over the ABI instead of silently
		// succeeding. A missing layer stays a silent no-op as before.
		if node := doc.findLayer(doc.ActiveLayerID); node != nil {
			inst.paintStroke = &activePaintStroke{layerID: doc.ActiveLayerID}
		}
		return
	}
	if err := ensureLayerEditable(layer, editLayerPixels); err != nil {
		// The layer's pixels are locked. Record a rejected stroke so continue
		// points stay no-ops and handleEndPaintStroke surfaces the lock error
		// over the ABI, mirroring the non-pixel-layer rejection above.
		inst.paintStroke = &activePaintStroke{layerID: layer.ID(), rejected: err}
		return
	}
	brushParams := p.Brush
	brushParams = normalizeMixerBrushParams(brushParams)
	brushParams = normalizeCloneStampParams(brushParams)
	brushParams = normalizeHistoryBrushParams(brushParams)
	if brushParams.AutoErase {
		// Sample the active layer pixel at the stroke start.
		// If it matches the brush (foreground) color, switch to background color.
		px := int(math.Round(p.X)) - layer.Bounds.X
		py := int(math.Round(p.Y)) - layer.Bounds.Y
		if px >= 0 && py >= 0 && px < layer.Bounds.W && py < layer.Bounds.H {
			idx := (py*layer.Bounds.W + px) * 4
			fg := brushParams.Color
			if layer.Pixels[idx] == fg[0] && layer.Pixels[idx+1] == fg[1] && layer.Pixels[idx+2] == fg[2] {
				brushParams.Color = inst.backgroundColor
			}
		}
	}

	stroke := &activePaintStroke{
		layerID:    layer.ID(),
		params:     brushParams,
		stabilizer: newStabilizer(brushParams.Stabilizer),
	}

	// Background eraser: sample the pixel under the pointer once at stroke begin.
	if brushParams.EraseBackground {
		px := int(math.Round(p.X)) - layer.Bounds.X
		py := int(math.Round(p.Y)) - layer.Bounds.Y
		if px >= 0 && py >= 0 && px < layer.Bounds.W && py < layer.Bounds.H {
			idx := (py*layer.Bounds.W + px) * 4
			stroke.bgEraseBaseColor = [4]uint8{layer.Pixels[idx], layer.Pixels[idx+1], layer.Pixels[idx+2], layer.Pixels[idx+3]}
		}
	}

	// Pre-create the AGG renderer for the stroke's layer so dab rendering
	// reuses the rasterizer's allocated cell blocks instead of re-allocating.
	stroke.renderer = agglib.NewAgg2D()
	if brushParams.MixerBrush {
		stroke.mixer = inst.beginMixerBrushStroke(doc.ID, brushParams)
		stroke.mixerSource, stroke.mixerSourceW, stroke.mixerSourceH, stroke.mixerSourceX, stroke.mixerSourceY = captureStrokeSourceSurface(doc, layer, brushParams.SampleMerged)
	}
	if brushParams.CloneStamp {
		if brushParams.CloneHistory && brushParams.CloneHistoryIdx > 0 {
			if state, ok := inst.history.SnapshotAtIndex(inst, brushParams.CloneHistoryIdx); ok {
				stroke.cloneSource, stroke.cloneSourceW, stroke.cloneSourceH, stroke.cloneSourceX, stroke.cloneSourceY = captureHistorySourceSurface(state, brushParams.SampleMerged)
			}
		}
		if len(stroke.cloneSource) == 0 {
			stroke.cloneSource, stroke.cloneSourceW, stroke.cloneSourceH, stroke.cloneSourceX, stroke.cloneSourceY = captureStrokeSourceSurface(doc, layer, brushParams.SampleMerged)
		}
		stroke.cloneOffsetX, stroke.cloneOffsetY = inst.beginCloneStampStroke(doc.ID, p.X, p.Y, brushParams)
		stroke.cloneRemainingLoad = brushParams.CloneLoad
	}
	if brushParams.HistoryBrush {
		if brushParams.HistorySourceIdx > 0 {
			if state, ok := inst.history.SnapshotAtIndex(inst, brushParams.HistorySourceIdx); ok {
				stroke.historySource, stroke.historySourceW, stroke.historySourceH, stroke.historySourceX, stroke.historySourceY = captureHistorySourceSurface(state, brushParams.SampleMerged)
			}
		}
		if len(stroke.historySource) == 0 {
			if state, ok := inst.history.PreviousSnapshot(inst); ok {
				stroke.historySource, stroke.historySourceW, stroke.historySourceH, stroke.historySourceX, stroke.historySourceY = captureHistorySourceSurface(state, brushParams.SampleMerged)
			}
		}
		stroke.historyRemainingLoad = brushParams.HistoryLoad
	}

	inst.paintStroke = stroke

	pressure := p.Pressure
	if pressure == 0 {
		// No pressure reported (mouse input) → dynamics neutral, not weakened.
		pressure = 1
	}
	effective := applyPressure(brushParams, pressure)
	azimuth, squish := applyTilt(p.TiltX, p.TiltY)
	sx, sy := inst.paintStroke.stabilizer.Push(p.X, p.Y)
	dabs := inst.paintStroke.strokeState.AddPoint(sx, sy, 0.25, effective.Size)
	footprint := dabFootprintSize(effective)
	var selScratch []byte
	for _, dab := range dabs {
		dx, dy := applyScatter(dab[0], dab[1], effective)
		dabParams := effective
		stroke.saveRowsBeforeDab(layer, dx, dy, footprint, &inst.undoRowBuf)
		paintDabClippedToSelection(layer, doc.Selection, dx, dy, footprint, &selScratch, func() {
			if brushParams.EraseBackground {
				EraseBackgroundDab(layer, dx, dy, dabParams, inst.paintStroke.bgEraseBaseColor)
			} else if dabParams.CloneStamp {
				CloneStampDab(layer, inst.paintStroke.cloneSource, inst.paintStroke.cloneSourceW, inst.paintStroke.cloneSourceH, inst.paintStroke.cloneSourceX, inst.paintStroke.cloneSourceY, dx, dy, dabParams, inst.paintStroke.cloneOffsetX, inst.paintStroke.cloneOffsetY, &inst.paintStroke.cloneRemainingLoad)
			} else if dabParams.HistoryBrush {
				CloneStampDab(layer, inst.paintStroke.historySource, inst.paintStroke.historySourceW, inst.paintStroke.historySourceH, inst.paintStroke.historySourceX, inst.paintStroke.historySourceY, dx, dy, dabParams, 0, 0, &inst.paintStroke.historyRemainingLoad)
			} else {
				if dabParams.MixerBrush {
					dirX, dirY := mixerStrokeDirection(stroke, dx, dy, azimuth)
					directionAzimuth := math.Atan2(dirY, dirX)
					paintMixerBrushDab(stroke.renderer, layer, &stroke.mixer, stroke.mixerSource, stroke.mixerSourceW, stroke.mixerSourceH, stroke.mixerSourceX, stroke.mixerSourceY, dx, dy, dabParams, directionAzimuth, squish)
					updateMixerStrokeDirection(stroke, dx, dy)
				} else {
					paintDabReuse(stroke.renderer, layer, dx, dy, dabParams, azimuth, squish)
				}
			}
		})
		inst.paintStroke.expandDirty(layer, dx, dy, footprint)
	}
	if rect, ok := strokeDirtyRectInDocument(stroke, layer); ok {
		doc.bumpContentVersionRect(rect)
	}
}

// StrokePoint is a single continue-stroke sample fed through the per-point
// paint pipeline. A coalesced batch is a slice of these.
type StrokePoint struct {
	X        float64
	Y        float64
	Pressure float64
	TiltX    float64
	TiltY    float64
}

func (inst *instance) handleContinuePaintStroke(p ContinuePaintStrokePayload) {
	inst.handleContinuePaintStrokePoints([]StrokePoint{{
		X:        p.X,
		Y:        p.Y,
		Pressure: p.Pressure,
		TiltX:    p.TiltX,
		TiltY:    p.TiltY,
	}})
}

// handleContinuePaintStrokePoints processes a coalesced batch of continue-stroke
// samples in order. Each sample runs through the same per-point pipeline as a
// single legacy point; the content-version dirty rect is bumped once per batch
// (not per point) to mirror the pre-batching single-point behavior.
func (inst *instance) handleContinuePaintStrokePoints(points []StrokePoint) {
	if inst.paintStroke == nil || inst.paintStroke.rejected != nil {
		return
	}
	doc := inst.manager.activeMut()
	if doc == nil {
		return
	}
	layer := findPixelLayer(doc, inst.paintStroke.layerID)
	if layer == nil {
		return
	}
	var selScratch []byte
	for i := range points {
		inst.continuePaintStrokePoint(doc, layer, points[i], &selScratch)
	}
	if rect, ok := strokeDirtyRectInDocument(inst.paintStroke, layer); ok {
		doc.bumpContentVersionRect(rect)
	}
}

// continuePaintStrokePoint feeds one sample through the stroke pipeline:
// pressure default, applyPressure, applyTilt, stabilizer.Push, strokeState
// AddPoint, then the dab loop. It does not bump the content version; callers
// do that once per batch.
func (inst *instance) continuePaintStrokePoint(doc *Document, layer *PixelLayer, p StrokePoint, selScratch *[]byte) {
	pressure := p.Pressure
	if pressure == 0 {
		// No pressure reported (mouse input) → dynamics neutral, not weakened.
		pressure = 1
	}
	effective := applyPressure(inst.paintStroke.params, pressure)
	azimuth, squish := applyTilt(p.TiltX, p.TiltY)
	sx, sy := inst.paintStroke.stabilizer.Push(p.X, p.Y)
	dabs := inst.paintStroke.strokeState.AddPoint(sx, sy, 0.25, effective.Size)
	footprint := dabFootprintSize(effective)
	for _, dab := range dabs {
		dx, dy := applyScatter(dab[0], dab[1], effective)
		dabParams := effective
		inst.paintStroke.saveRowsBeforeDab(layer, dx, dy, footprint, &inst.undoRowBuf)
		paintDabClippedToSelection(layer, doc.Selection, dx, dy, footprint, selScratch, func() {
			if inst.paintStroke.params.EraseBackground {
				EraseBackgroundDab(layer, dx, dy, dabParams, inst.paintStroke.bgEraseBaseColor)
			} else if dabParams.CloneStamp {
				CloneStampDab(layer, inst.paintStroke.cloneSource, inst.paintStroke.cloneSourceW, inst.paintStroke.cloneSourceH, inst.paintStroke.cloneSourceX, inst.paintStroke.cloneSourceY, dx, dy, dabParams, inst.paintStroke.cloneOffsetX, inst.paintStroke.cloneOffsetY, &inst.paintStroke.cloneRemainingLoad)
			} else if dabParams.HistoryBrush {
				CloneStampDab(layer, inst.paintStroke.historySource, inst.paintStroke.historySourceW, inst.paintStroke.historySourceH, inst.paintStroke.historySourceX, inst.paintStroke.historySourceY, dx, dy, dabParams, 0, 0, &inst.paintStroke.historyRemainingLoad)
			} else {
				if dabParams.MixerBrush {
					dirX, dirY := mixerStrokeDirection(inst.paintStroke, dx, dy, azimuth)
					directionAzimuth := math.Atan2(dirY, dirX)
					paintMixerBrushDab(inst.paintStroke.renderer, layer, &inst.paintStroke.mixer, inst.paintStroke.mixerSource, inst.paintStroke.mixerSourceW, inst.paintStroke.mixerSourceH, inst.paintStroke.mixerSourceX, inst.paintStroke.mixerSourceY, dx, dy, dabParams, directionAzimuth, squish)
					updateMixerStrokeDirection(inst.paintStroke, dx, dy)
				} else {
					paintDabReuse(inst.paintStroke.renderer, layer, dx, dy, dabParams, azimuth, squish)
				}
			}
		})
		inst.paintStroke.expandDirty(layer, dx, dy, footprint)
	}
}

// handleEndPaintStroke finalizes the active stroke and records it as an
// undoable history entry. It returns an error when the stroke pixels were
// already committed to the layer but the undo delta could not be built: in that
// case the document has diverged from the recorded history, so the stroke is not
// undoable. When that happens we drop the redo stack (ClearRedo) — the live
// document no longer matches the base a stale redo entry would replay onto, so
// keeping it would corrupt the document — and surface the error so the dispatch
// layer can report the failure over the ABI instead of silently swallowing it.
func (inst *instance) handleEndPaintStroke() error {
	if inst.paintStroke == nil {
		return nil
	}
	doc := inst.manager.activeMut()
	stroke := inst.paintStroke
	inst.paintStroke = nil

	if doc == nil {
		if stroke.params.MixerBrush {
			inst.mixerBrush = stroke.mixer
		}
		return nil
	}
	if stroke.rejected != nil {
		// The stroke was refused at begin time (locked layer) and never
		// painted a single dab — surface the recorded error over the ABI.
		return stroke.rejected
	}
	layer := findPixelLayer(doc, stroke.layerID)
	if layer == nil {
		if stroke.params.MixerBrush {
			inst.mixerBrush = stroke.mixer
		}
		// The stroke targeted a non-pixel layer (rejected at begin time):
		// surface a clear error over the ABI so the frontend can offer to
		// rasterize. A layer that vanished mid-stroke stays a silent no-op.
		if node := doc.findLayer(stroke.layerID); node != nil {
			return paintTargetError(node)
		}
		return nil
	}
	if !stroke.hasDirty {
		if stroke.params.MixerBrush {
			inst.mixerBrush = stroke.mixer
		}
		return nil
	}

	rect := DirtyRect{
		X: stroke.dirtyMin[0], Y: stroke.dirtyMin[1],
		W: stroke.dirtyMax[0] - stroke.dirtyMin[0],
		H: stroke.dirtyMax[1] - stroke.dirtyMin[1],
	}
	delta, err := buildStrokeDelta(
		stroke.beforeRowBuf, stroke.beforeRowStart, stroke.layerW,
		layer.Pixels, layer.Bounds.W, layer.Bounds.H, rect,
	)
	if err != nil {
		// The stroke pixels are already on the layer but cannot be recorded as an
		// undo step. Discard any redo entries (the document has diverged from
		// history) and surface the error rather than leaving a non-undoable stroke.
		if stroke.params.MixerBrush {
			inst.mixerBrush = stroke.mixer
		}
		inst.history.ClearRedo()
		return fmt.Errorf("record brush stroke: %w", err)
	}
	layerID := stroke.layerID
	cmd := &pixelDeltaCommand{
		description: "Brush stroke",
		target: func(inst *instance) []byte {
			l := findPixelLayer(inst.manager.activeMut(), layerID)
			if l == nil {
				return nil
			}
			return l.Pixels
		},
		bump:  bumpLayerContentVersion(layerID, delta.Rect),
		delta: delta,
	}
	inst.history.push(cmd)
	if stroke.params.MixerBrush {
		inst.mixerBrush = stroke.mixer
	}
	return nil
}

// paintTargetError describes why the given layer node cannot be painted on.
// The wording travels over the ABI so the frontend can offer to rasterize the
// layer.
func paintTargetError(node LayerNode) error {
	if node == nil {
		return fmt.Errorf("no active layer to paint on")
	}
	return fmt.Errorf("cannot paint on %s layer %q: layer must be rasterized before painting", node.LayerType(), node.Name())
}

// revertActivePaintStroke discards an in-progress brush stroke and restores the
// pixel rows it has painted so far from the stroke's lazily saved before-rows.
// The row snapshot is exactly what handleEndPaintStroke would have used to
// build the undo delta, so the revert is byte-exact. Called when a history
// restore interrupts a stroke (see revertInFlightPreviewMutations): the stored
// document may be referenced by pointer snapshots, so the half-finished stroke
// must not survive on it.
func (inst *instance) revertActivePaintStroke() {
	stroke := inst.paintStroke
	if stroke == nil {
		return
	}
	inst.paintStroke = nil
	if !stroke.hasDirty || stroke.layerW == 0 {
		return
	}
	doc := inst.manager.activeMut()
	if doc == nil {
		return
	}
	layer := findPixelLayer(doc, stroke.layerID)
	if layer == nil || layer.Bounds.W != stroke.layerW {
		return
	}
	rowBytes := stroke.layerW * 4
	start := stroke.beforeRowStart * rowBytes
	end := stroke.beforeRowEnd * rowBytes
	if start < 0 || end > len(layer.Pixels) || len(stroke.beforeRowBuf) != end-start {
		return
	}
	copy(layer.Pixels[start:end], stroke.beforeRowBuf)
	if rect, ok := strokeDirtyRectInDocument(stroke, layer); ok {
		doc.bumpContentVersionRect(rect)
	}
}

func strokeDirtyRectInDocument(stroke *activePaintStroke, layer *PixelLayer) (DirtyRect, bool) {
	if stroke == nil || layer == nil || !stroke.hasDirty {
		return DirtyRect{}, false
	}
	rect := DirtyRect{
		X: layer.Bounds.X + stroke.dirtyMin[0],
		Y: layer.Bounds.Y + stroke.dirtyMin[1],
		W: stroke.dirtyMax[0] - stroke.dirtyMin[0],
		H: stroke.dirtyMax[1] - stroke.dirtyMin[1],
	}
	if rect.W <= 0 || rect.H <= 0 {
		return DirtyRect{}, false
	}
	return rect, true
}

// handleMagicErase implements the Magic Eraser: flood-fills (or global-selects)
// pixels within tolerance of the clicked color and clears their alpha to 0.
// The operation is undoable.
func (inst *instance) handleMagicErase(p MagicErasePayload, doc *Document, layer *PixelLayer) error {
	if err := ensureLayerEditable(layer, editLayerPixels); err != nil {
		return err
	}
	// Determine the source surface for color sampling.
	var surface []byte
	if p.SampleMerged {
		surface = inst.compositeSurface(doc)
	} else {
		surface = layer.Pixels
	}

	// Convert document-space click to pixel coordinates on the source surface.
	var srcW, srcH int
	var offX, offY int
	if p.SampleMerged {
		srcW, srcH = doc.Width, doc.Height
	} else {
		srcW, srcH = layer.Bounds.W, layer.Bounds.H
		offX, offY = layer.Bounds.X, layer.Bounds.Y
	}
	px := int(math.Round(p.X)) - offX
	py := int(math.Round(p.Y)) - offY
	if px < 0 || py < 0 || px >= srcW || py >= srcH {
		return nil
	}

	// Sample the target color.
	targetColor, ok := sampleSurfaceColor(surface, srcW, srcH, px, py)
	if !ok {
		return nil
	}

	// Build a mask of pixels to erase (reuse selection logic, then apply to layer).
	var mask *Selection
	if p.Contiguous {
		mask = magicWandFloodFill(surface, srcW, srcH, px, py, p.Tolerance)
	} else {
		mask = selectColorRange(surface, srcW, srcH, targetColor, p.Tolerance)
	}
	if mask == nil {
		return nil
	}

	// Snapshot layer pixels for undo.
	before := make([]byte, len(layer.Pixels))
	copy(before, layer.Pixels)

	// Apply mask to layer alpha: multiply dest alpha by (1 - mask/255).
	// Track the bounding box of pixels that actually changed so the dirty
	// rect (and composite invalidation) stays as tight as possible.
	lw := layer.Bounds.W
	lh := layer.Bounds.H
	minX, minY := lw, lh
	maxX, maxY := -1, -1
	for ly := range lh {
		for lx := range lw {
			// Map layer-local coordinates to mask coordinates.
			maskX := lx + layer.Bounds.X - offX
			maskY := ly + layer.Bounds.Y - offY
			if maskX < 0 || maskY < 0 || maskX >= mask.Width || maskY >= mask.Height {
				continue
			}
			coverage := float64(mask.Mask[maskY*mask.Width+maskX]) / 255.0
			if coverage <= 0 {
				continue
			}
			// Clip to the active selection with coverage weighting — the same
			// 0–255 convention brush dabs and fillRasterWithMask use.
			if sel := selectionCoverageAt(doc.Selection, lx+layer.Bounds.X, ly+layer.Bounds.Y); sel == 0 {
				continue
			} else if sel < 255 {
				coverage *= float64(sel) / 255.0
			}
			idx := (ly*lw + lx) * 4
			newAlpha := float64(layer.Pixels[idx+3]) * (1.0 - coverage)
			if newAlpha < 0 {
				newAlpha = 0
			}
			if na := uint8(newAlpha); na != layer.Pixels[idx+3] {
				layer.Pixels[idx+3] = na
				if lx < minX {
					minX = lx
				}
				if lx > maxX {
					maxX = lx
				}
				if ly < minY {
					minY = ly
				}
				if ly > maxY {
					maxY = ly
				}
			}
		}
	}
	// Use the shared atomic version counter and dirty-rect marking (S.2/S.4)
	// like every other pixel edit — a bare ContentVersion++ neither advances
	// the global counter nor invalidates the composite cache region.
	dirty := DirtyRect{X: layer.Bounds.X, Y: layer.Bounds.Y, W: lw, H: lh}
	if maxX >= minX && maxY >= minY {
		dirty = DirtyRect{X: layer.Bounds.X + minX, Y: layer.Bounds.Y + minY, W: maxX - minX + 1, H: maxY - minY + 1}
	}
	doc.bumpContentVersionRect(dirty)

	// Record undo.
	layerID := layer.ID()
	delta, err := NewPixelDelta(before, layer.Pixels, lw, lh, DirtyRect{X: 0, Y: 0, W: lw, H: lh})
	if err != nil {
		return nil
	}
	inst.history.push(&pixelDeltaCommand{
		description: "Magic Eraser",
		target: func(inst *instance) []byte {
			l := findPixelLayer(inst.manager.activeMut(), layerID)
			if l == nil {
				return nil
			}
			return l.Pixels
		},
		bump:  bumpLayerContentVersion(layerID, delta.Rect),
		delta: delta,
	})
	return nil
}
