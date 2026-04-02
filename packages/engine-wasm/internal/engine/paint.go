package engine

import (
	"math"

	agglib "github.com/cwbudde/agg_go"
)

func (inst *instance) handleBeginPaintStroke(p BeginPaintStrokePayload) {
	doc := inst.manager.activeMut()
	if doc == nil {
		return
	}
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return
	}
	brushParams := p.Brush
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
		stroke.mixerSource, stroke.mixerSourceW, stroke.mixerSourceH, stroke.mixerSourceX, stroke.mixerSourceY = captureStrokeSourceSurface(doc, layer, brushParams.SampleMerged)
	}
	if brushParams.CloneStamp {
		stroke.cloneSource, stroke.cloneSourceW, stroke.cloneSourceH, stroke.cloneSourceX, stroke.cloneSourceY = captureStrokeSourceSurface(doc, layer, brushParams.SampleMerged)
		stroke.cloneOffsetX = brushParams.CloneSourceX - p.X
		stroke.cloneOffsetY = brushParams.CloneSourceY - p.Y
	}
	if brushParams.HistoryBrush {
		if state, ok := inst.history.PreviousSnapshot(inst); ok {
			stroke.historySource, stroke.historySourceW, stroke.historySourceH, stroke.historySourceX, stroke.historySourceY = captureHistorySourceSurface(state, brushParams.SampleMerged)
		}
	}

	inst.paintStroke = stroke

	pressure := p.Pressure
	if pressure == 0 {
		pressure = 0.5
	}
	effective := applyPressure(brushParams, pressure)
	azimuth, squish := applyTilt(p.TiltX, p.TiltY)
	sx, sy := inst.paintStroke.stabilizer.Push(p.X, p.Y)
	dabs := inst.paintStroke.strokeState.AddPoint(sx, sy, 0.25, effective.Size)
	for _, dab := range dabs {
		dx, dy := applyScatter(dab[0], dab[1], effective)
		dabParams := effective
		stroke.saveRowsBeforeDab(layer, dx, dy, effective.Size, &inst.undoRowBuf)
		if brushParams.EraseBackground {
			EraseBackgroundDab(layer, dx, dy, dabParams, inst.paintStroke.bgEraseBaseColor)
		} else if dabParams.CloneStamp {
			CloneStampDab(layer, inst.paintStroke.cloneSource, inst.paintStroke.cloneSourceW, inst.paintStroke.cloneSourceH, inst.paintStroke.cloneSourceX, inst.paintStroke.cloneSourceY, dx, dy, dabParams, inst.paintStroke.cloneOffsetX, inst.paintStroke.cloneOffsetY)
		} else if dabParams.HistoryBrush {
			CloneStampDab(layer, inst.paintStroke.historySource, inst.paintStroke.historySourceW, inst.paintStroke.historySourceH, inst.paintStroke.historySourceX, inst.paintStroke.historySourceY, dx, dy, dabParams, 0, 0)
		} else {
			if dabParams.MixerBrush {
				dabParams.Color = resolveMixerBrushColor(stroke.mixerSource, stroke.mixerSourceW, stroke.mixerSourceH, stroke.mixerSourceX, stroke.mixerSourceY, dx, dy, dabParams.Color, dabParams.MixerMix)
			}
			paintDabReuse(stroke.renderer, layer, dx, dy, dabParams, azimuth, squish)
		}
		inst.paintStroke.expandDirty(layer, dx, dy, effective.Size)
	}
	doc.ContentVersion++
}

func (inst *instance) handleContinuePaintStroke(p ContinuePaintStrokePayload) {
	if inst.paintStroke == nil {
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
	pressure := p.Pressure
	if pressure == 0 {
		pressure = 0.5
	}
	effective := applyPressure(inst.paintStroke.params, pressure)
	azimuth, squish := applyTilt(p.TiltX, p.TiltY)
	sx, sy := inst.paintStroke.stabilizer.Push(p.X, p.Y)
	dabs := inst.paintStroke.strokeState.AddPoint(sx, sy, 0.25, effective.Size)
	for _, dab := range dabs {
		dx, dy := applyScatter(dab[0], dab[1], effective)
		dabParams := effective
		inst.paintStroke.saveRowsBeforeDab(layer, dx, dy, effective.Size, &inst.undoRowBuf)
		if inst.paintStroke.params.EraseBackground {
			EraseBackgroundDab(layer, dx, dy, dabParams, inst.paintStroke.bgEraseBaseColor)
		} else if dabParams.CloneStamp {
			CloneStampDab(layer, inst.paintStroke.cloneSource, inst.paintStroke.cloneSourceW, inst.paintStroke.cloneSourceH, inst.paintStroke.cloneSourceX, inst.paintStroke.cloneSourceY, dx, dy, dabParams, inst.paintStroke.cloneOffsetX, inst.paintStroke.cloneOffsetY)
		} else if dabParams.HistoryBrush {
			CloneStampDab(layer, inst.paintStroke.historySource, inst.paintStroke.historySourceW, inst.paintStroke.historySourceH, inst.paintStroke.historySourceX, inst.paintStroke.historySourceY, dx, dy, dabParams, 0, 0)
		} else {
			if dabParams.MixerBrush {
				dabParams.Color = resolveMixerBrushColor(inst.paintStroke.mixerSource, inst.paintStroke.mixerSourceW, inst.paintStroke.mixerSourceH, inst.paintStroke.mixerSourceX, inst.paintStroke.mixerSourceY, dx, dy, dabParams.Color, dabParams.MixerMix)
			}
			paintDabReuse(inst.paintStroke.renderer, layer, dx, dy, dabParams, azimuth, squish)
		}
		inst.paintStroke.expandDirty(layer, dx, dy, effective.Size)
	}
	if len(dabs) > 0 {
		doc.ContentVersion++
	}
}

func (inst *instance) handleEndPaintStroke() {
	if inst.paintStroke == nil {
		return
	}
	doc := inst.manager.activeMut()
	stroke := inst.paintStroke
	inst.paintStroke = nil

	if doc == nil || !stroke.hasDirty {
		return
	}
	layer := findPixelLayer(doc, stroke.layerID)
	if layer == nil {
		return
	}

	rect := DirtyRect{
		X: stroke.dirtyMin[0], Y: stroke.dirtyMin[1],
		W: stroke.dirtyMax[0] - stroke.dirtyMin[0],
		H: stroke.dirtyMax[1] - stroke.dirtyMin[1],
	}
	delta, err := newPixelDeltaFromRows(
		stroke.beforeRowBuf, stroke.beforeRowStart, stroke.layerW,
		layer.Pixels, layer.Bounds.W, layer.Bounds.H, rect,
	)
	if err != nil {
		return
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
		delta: delta,
	}
	inst.history.push(cmd)
}

// handleMagicErase implements the Magic Eraser: flood-fills (or global-selects)
// pixels within tolerance of the clicked color and clears their alpha to 0.
// The operation is undoable.
func (inst *instance) handleMagicErase(p MagicErasePayload, doc *Document, layer *PixelLayer) error {
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
	lw := layer.Bounds.W
	lh := layer.Bounds.H
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
			idx := (ly*lw + lx) * 4
			newAlpha := float64(layer.Pixels[idx+3]) * (1.0 - coverage)
			if newAlpha < 0 {
				newAlpha = 0
			}
			layer.Pixels[idx+3] = uint8(newAlpha)
		}
	}
	doc.ContentVersion++

	// Record undo.
	layerID := layer.ID()
	delta, err := NewPixelDelta(before, layer.Pixels, lw, lh, DirtyRect{0, 0, lw, lh})
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
		delta: delta,
	})
	return nil
}
