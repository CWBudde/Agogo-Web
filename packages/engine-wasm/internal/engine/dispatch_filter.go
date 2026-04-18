package engine

import (
	"encoding/json"
	"fmt"
)

// ApplyFilterPayload is the JSON payload for the ApplyFilter command.
type ApplyFilterPayload struct {
	LayerID  string          `json:"layerId"`
	FilterID string          `json:"filterId"`
	Params   json.RawMessage `json:"params"`
}

// lastFilterState records the most recently applied filter so that
// ReapplyFilter can replay it without user interaction.
type lastFilterState struct {
	FilterID string
	Params   json.RawMessage
}

// filterPreviewState holds transient state while a filter dialog shows a
// live preview. The original pixels are saved so they can be restored on
// cancel or replaced on commit.
type filterPreviewState struct {
	LayerID      string // target pixel layer
	FilterID     string
	OrigPixels   []byte // snapshot before any preview mutation
	PreviewScale int    // denominator: 1 = full, 2 = half, 4 = quarter
}

// fadeSnapshot stores the pixel data before the last committed filter so
// that "Filter > Fade" can blend the result with the original.
type fadeSnapshot struct {
	LayerID    string
	OrigPixels []byte
}

// PreviewFilterPayload is the JSON payload for the PreviewFilter command.
type PreviewFilterPayload struct {
	LayerID  string          `json:"layerId"`
	FilterID string          `json:"filterId"`
	Params   json.RawMessage `json:"params"`
	Scale    int             `json:"scale"` // 1, 2, or 4 — preview resolution divisor
}

// FadeFilterPayload is the JSON payload for the FadeFilter command.
type FadeFilterPayload struct {
	Opacity   float64   `json:"opacity"`   // 0-100
	BlendMode BlendMode `json:"blendMode"` // blend mode for fade
}

func (inst *instance) dispatchFilterCommand(commandID int32, payloadJSON string) (bool, error) {
	switch commandID {
	case commandApplyFilter:
		return inst.handleApplyFilter(payloadJSON)

	case commandReapplyFilter:
		return inst.handleReapplyFilter()

	case commandPreviewFilter:
		return inst.handlePreviewFilter(payloadJSON)

	case commandCancelFilterPreview:
		return inst.handleCancelFilterPreview()

	case commandCommitFilterPreview:
		return inst.handleCommitFilterPreview()

	case commandFadeFilter:
		return inst.handleFadeFilter(payloadJSON)

	default:
		return false, nil
	}
}

// ---------------------------------------------------------------------------
// Apply Filter (destructive, with undo + fade snapshot)
// ---------------------------------------------------------------------------

func (inst *instance) handleApplyFilter(payloadJSON string) (bool, error) {
	var payload ApplyFilterPayload
	if err := decodePayload(payloadJSON, &payload); err != nil {
		return true, err
	}

	doc := inst.manager.Active()
	if doc == nil {
		return true, fmt.Errorf("apply filter: no active document")
	}

	layerID := payload.LayerID
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}

	entry := lookupFilter(payload.FilterID)
	if entry == nil {
		return true, fmt.Errorf("apply filter: unknown filter %q", payload.FilterID)
	}

	// Check for Smart Object placeholder — non-pixel layers cannot have
	// destructive filters applied; they would need Smart Filters (Phase 7+).
	node := doc.findLayer(layerID)
	if node == nil {
		return true, fmt.Errorf("apply filter: layer %q not found", layerID)
	}
	pl, ok := node.(*PixelLayer)
	if !ok {
		return true, fmt.Errorf("apply filter: layer %q is %s — destructive filters require a pixel layer (Smart Filters planned for Phase 7+)", layerID, node.LayerType())
	}

	// Save pre-filter snapshot for Fade.
	preFade := &fadeSnapshot{
		LayerID:    layerID,
		OrigPixels: append([]byte(nil), pl.Pixels...),
	}

	// Cancel any outstanding preview before applying.
	inst.filterPreview = nil

	if err := inst.executeDocCommand(entry.Def.Name, func(doc *Document) error {
		return doc.ApplyFilter(layerID, payload.FilterID, payload.Params)
	}); err != nil {
		return true, err
	}

	inst.lastFilter = &lastFilterState{
		FilterID: payload.FilterID,
		Params:   payload.Params,
	}
	inst.preFadeSnapshot = preFade
	return true, nil
}

// ---------------------------------------------------------------------------
// Reapply Filter
// ---------------------------------------------------------------------------

func (inst *instance) handleReapplyFilter() (bool, error) {
	if inst.lastFilter == nil {
		return true, fmt.Errorf("reapply filter: no previous filter to reapply")
	}

	doc := inst.manager.Active()
	if doc == nil {
		return true, fmt.Errorf("reapply filter: no active document")
	}

	entry := lookupFilter(inst.lastFilter.FilterID)
	if entry == nil {
		return true, fmt.Errorf("reapply filter: last filter %q no longer registered", inst.lastFilter.FilterID)
	}

	layerID := doc.ActiveLayerID
	node := doc.findLayer(layerID)
	if node == nil {
		return true, fmt.Errorf("reapply filter: active layer %q not found", layerID)
	}
	pl, ok := node.(*PixelLayer)
	if !ok {
		return true, fmt.Errorf("reapply filter: active layer %q is %s, not a pixel layer", layerID, node.LayerType())
	}

	preFade := &fadeSnapshot{
		LayerID:    layerID,
		OrigPixels: append([]byte(nil), pl.Pixels...),
	}

	params := inst.lastFilter.Params
	if err := inst.executeDocCommand(entry.Def.Name, func(doc *Document) error {
		return doc.ApplyFilter(layerID, inst.lastFilter.FilterID, params)
	}); err != nil {
		return true, err
	}

	inst.preFadeSnapshot = preFade
	return true, nil
}

// ---------------------------------------------------------------------------
// Preview Filter (live preview in filter dialog)
// ---------------------------------------------------------------------------

func (inst *instance) handlePreviewFilter(payloadJSON string) (bool, error) {
	var payload PreviewFilterPayload
	if err := decodePayload(payloadJSON, &payload); err != nil {
		return true, fmt.Errorf("preview filter decode: %w", err)
	}

	doc := inst.manager.activeMut()
	if doc == nil {
		return true, fmt.Errorf("preview filter: no active document")
	}

	layerID := payload.LayerID
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}

	entry := lookupFilter(payload.FilterID)
	if entry == nil {
		return true, fmt.Errorf("preview filter: unknown filter %q", payload.FilterID)
	}

	node := doc.findLayer(layerID)
	if node == nil {
		return true, fmt.Errorf("preview filter: layer %q not found", layerID)
	}
	pl, ok := node.(*PixelLayer)
	if !ok {
		return true, fmt.Errorf("preview filter: layer %q is not a pixel layer", layerID)
	}

	// On first preview call, snapshot the original pixels. On subsequent
	// calls (parameter tweak), restore from the snapshot before re-applying.
	if inst.filterPreview == nil || inst.filterPreview.LayerID != layerID || inst.filterPreview.FilterID != payload.FilterID {
		// New preview session — save original.
		inst.filterPreview = &filterPreviewState{
			LayerID:      layerID,
			FilterID:     payload.FilterID,
			OrigPixels:   append([]byte(nil), pl.Pixels...),
			PreviewScale: payload.Scale,
		}
	} else {
		// Same filter/layer — restore from snapshot before re-applying.
		copy(pl.Pixels, inst.filterPreview.OrigPixels)
	}

	scale := payload.Scale
	if scale < 1 {
		scale = 1
	}

	if scale > 1 {
		// Reduced-resolution preview: downsample, filter, upsample back.
		origW, origH := pl.Bounds.W, pl.Bounds.H
		prevW := max(origW/scale, 1)
		prevH := max(origH/scale, 1)

		small := scaleRGBA(pl.Pixels, origW, origH, prevW, prevH)
		selMask := doc.selectionMaskForLayer(pl)

		// Scale selection mask if present.
		var smallMask []byte
		if selMask != nil {
			smallMask = make([]byte, prevW*prevH)
			for y := 0; y < prevH; y++ {
				for x := 0; x < prevW; x++ {
					sx := x * origW / prevW
					sy := y * origH / prevH
					smallMask[y*prevW+x] = selMask[sy*origW+sx]
				}
			}
		}

		if err := entry.Fn(small, prevW, prevH, smallMask, payload.Params); err != nil {
			return true, fmt.Errorf("preview filter %q: %w", payload.FilterID, err)
		}

		// Upsample back to original size.
		upscaled := scaleRGBA(small, prevW, prevH, origW, origH)
		copy(pl.Pixels, upscaled)
	} else {
		// Full-resolution preview.
		selMask := doc.selectionMaskForLayer(pl)
		if err := entry.Fn(pl.Pixels, pl.Bounds.W, pl.Bounds.H, selMask, payload.Params); err != nil {
			return true, fmt.Errorf("preview filter %q: %w", payload.FilterID, err)
		}
	}

	doc.bumpContentVersionRect(DirtyRect{X: pl.Bounds.X, Y: pl.Bounds.Y, W: pl.Bounds.W, H: pl.Bounds.H})
	return true, nil
}

// ---------------------------------------------------------------------------
// Cancel Filter Preview
// ---------------------------------------------------------------------------

func (inst *instance) handleCancelFilterPreview() (bool, error) {
	if inst.filterPreview == nil {
		return true, nil // no-op if no preview active
	}

	doc := inst.manager.activeMut()
	if doc == nil {
		inst.filterPreview = nil
		return true, nil
	}

	node := doc.findLayer(inst.filterPreview.LayerID)
	if node != nil {
		if pl, ok := node.(*PixelLayer); ok {
			copy(pl.Pixels, inst.filterPreview.OrigPixels)
			doc.bumpContentVersionRect(DirtyRect{X: pl.Bounds.X, Y: pl.Bounds.Y, W: pl.Bounds.W, H: pl.Bounds.H})
		}
	}

	inst.filterPreview = nil
	return true, nil
}

// ---------------------------------------------------------------------------
// Commit Filter Preview
// ---------------------------------------------------------------------------

func (inst *instance) handleCommitFilterPreview() (bool, error) {
	if inst.filterPreview == nil {
		return true, fmt.Errorf("commit filter preview: no active preview")
	}

	preview := inst.filterPreview
	inst.filterPreview = nil

	doc := inst.manager.activeMut()
	if doc == nil {
		return true, fmt.Errorf("commit filter preview: no active document")
	}

	node := doc.findLayer(preview.LayerID)
	if node == nil {
		return true, fmt.Errorf("commit filter preview: layer %q not found", preview.LayerID)
	}
	pl, ok := node.(*PixelLayer)
	if !ok {
		return true, fmt.Errorf("commit filter preview: layer %q is not a pixel layer", preview.LayerID)
	}

	// If previewed at reduced scale, the current pixels are approximate.
	// Re-apply at full resolution for the final commit.
	entry := lookupFilter(preview.FilterID)
	if entry == nil {
		return true, fmt.Errorf("commit filter preview: filter %q no longer registered", preview.FilterID)
	}

	// Save pre-filter snapshot for Fade.
	preFade := &fadeSnapshot{
		LayerID:    preview.LayerID,
		OrigPixels: preview.OrigPixels,
	}

	// Restore original pixels and apply filter at full quality via undo-wrapped command.
	copy(pl.Pixels, preview.OrigPixels)

	if err := inst.executeDocCommand(entry.Def.Name, func(doc *Document) error {
		return doc.ApplyFilter(preview.LayerID, preview.FilterID, nil)
	}); err != nil {
		return true, err
	}

	inst.lastFilter = &lastFilterState{
		FilterID: preview.FilterID,
	}
	inst.preFadeSnapshot = preFade
	return true, nil
}

// ---------------------------------------------------------------------------
// Fade Filter
// ---------------------------------------------------------------------------

func (inst *instance) handleFadeFilter(payloadJSON string) (bool, error) {
	var payload FadeFilterPayload
	if err := decodePayload(payloadJSON, &payload); err != nil {
		return true, err
	}

	if inst.preFadeSnapshot == nil {
		return true, fmt.Errorf("fade filter: no recent filter to fade")
	}

	snap := inst.preFadeSnapshot

	opacity := payload.Opacity / 100.0
	if opacity < 0 {
		opacity = 0
	} else if opacity > 1 {
		opacity = 1
	}

	blendMode := payload.BlendMode
	if blendMode == "" {
		blendMode = BlendModeNormal
	}

	if err := inst.executeDocCommand("Fade Filter", func(doc *Document) error {
		node := doc.findLayer(snap.LayerID)
		if node == nil {
			return fmt.Errorf("fade filter: layer %q not found", snap.LayerID)
		}
		pl, ok := node.(*PixelLayer)
		if !ok {
			return fmt.Errorf("fade filter: layer %q is not a pixel layer", snap.LayerID)
		}

		orig := snap.OrigPixels
		filtered := append([]byte(nil), pl.Pixels...)

		// Start with original pixels as the base, then composite the filtered
		// result on top using the requested opacity and blend mode.
		copy(pl.Pixels, orig)
		for i := 0; i < len(pl.Pixels); i += 4 {
			compositePixelWithBlend(pl.Pixels[i:i+4], filtered[i:i+4], blendMode, opacity, uint32(i))
		}

		doc.touchModifiedAtLayer(pl)
		return nil
	}); err != nil {
		return true, err
	}

	// Fade is a one-shot operation — clear the snapshot.
	inst.preFadeSnapshot = nil
	return true, nil
}
