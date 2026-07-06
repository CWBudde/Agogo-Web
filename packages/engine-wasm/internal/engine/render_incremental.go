package engine

import "math"

// Incremental document recomposite (PLAN.md S.4).
//
// When only a sub-rectangle of the document changed since the last composite
// (tracked by doc.dirtyComposite), the cached full-document surface is updated
// in place: the dirty rect is zeroed and the complete layer stack is
// re-composited clipped to that rect. Every compositing primitive in the stack
// is per-pixel (blend modes, masks, blend-if, clipping bases, group blending),
// so clipping the iteration bounds yields output byte-identical to a full
// recomposite inside the rect. Layer styles render spatially-extended effect
// surfaces, but those depend only on the styled layer's own content — the full
// effect surface is still rendered and only its composite onto the destination
// is clipped, which stays byte-identical too.
//
// The one exception is adjustment layers: their dirty-region cache
// (AdjustmentLayer.Cache) stores the mid-stack backdrop captured during FULL
// recomposites, and auto-parameter kinds (levels auto, black & white auto)
// derive parameters from the full backdrop at their stack position. During an
// incremental pass the destination outside the dirty rect holds the FINAL
// previous composite — not that backdrop — so any visible adjustment layer
// forces a full recomposite (see canRecompositeIncrementally).

// incrementalRecompositeRect reports the doc-space rectangle to recomposite in
// place, or ok=false when the full recomposite path must run instead: no dirty
// rect is tracked, the rect covers the whole document (a full pass is equally
// cheap and keeps the adjustment-cache semantics), or the layer stack contains
// a visible adjustment layer.
func (doc *Document) incrementalRecompositeRect() (DirtyRect, bool) {
	rectPtr := doc.currentDirtyCompositeRect()
	if rectPtr == nil {
		return DirtyRect{}, false
	}
	rect := *rectPtr
	if rect.W <= 0 || rect.H <= 0 {
		return DirtyRect{}, false
	}
	if rect.X <= 0 && rect.Y <= 0 && rect.X+rect.W >= doc.Width && rect.Y+rect.H >= doc.Height {
		return DirtyRect{}, false
	}
	if !layersAllowIncrementalRecomposite(doc.ensureLayerRoot().Children()) {
		return DirtyRect{}, false
	}
	return rect, true
}

// layersAllowIncrementalRecomposite walks the visible layer tree and reports
// whether a clipped recomposite is guaranteed byte-identical to a full one.
// Only visible adjustment layers disqualify (see the package comment above);
// invisible subtrees are skipped because they never composite.
func layersAllowIncrementalRecomposite(layers []LayerNode) bool {
	for _, layer := range layers {
		if layer == nil || !layer.Visible() {
			continue
		}
		if _, ok := layer.(*AdjustmentLayer); ok {
			return false
		}
		if group, ok := layer.(*GroupLayer); ok {
			if !layersAllowIncrementalRecomposite(group.Children()) {
				return false
			}
		}
	}
	return true
}

// renderCompositeSurfaceInto composites the document's full layer stack into
// dest, which must be a zeroed doc-sized RGBA buffer. It is the in-place
// variant of renderCompositeSurfaceChecked used to recycle the engine's cached
// document surface instead of allocating a fresh W×H×4 buffer per frame.
func (doc *Document) renderCompositeSurfaceInto(dest []byte) error {
	return doc.compositeLayerStackOntoWithOptions(dest, doc.ensureLayerRoot().Children(), nil, true, nil)
}

// recompositeSurfaceRect updates dest — which must hold the full composite for
// the document's previous content — in place: the rect region is zeroed and
// the complete layer stack is re-composited clipped to rect. Callers must have
// validated the rect via incrementalRecompositeRect.
//
// allowAdjustmentCache is false for the clipped stack pass: the gate excludes
// visible adjustment layers, and if one slipped through, updating its cache
// from a partially-recomposited surface would poison later full passes.
func (doc *Document) recompositeSurfaceRect(dest []byte, rect DirtyRect) error {
	zeroSurfaceRect(dest, doc.Width, rect)
	return doc.compositeLayerStackOntoWithOptions(dest, doc.ensureLayerRoot().Children(), nil, false, &rect)
}

// zeroSurfaceRect clears the RGBA bytes of a doc-space rectangle. The rect must
// already be clamped to the surface (markDirtyCompositeRect normalizes rects).
func zeroSurfaceRect(surface []byte, docW int, rect DirtyRect) {
	for y := rect.Y; y < rect.Y+rect.H; y++ {
		start := (y*docW + rect.X) * 4
		end := start + rect.W*4
		if start < 0 || end > len(surface) {
			continue
		}
		clear(surface[start:end])
	}
}

// Partial viewport resample (PLAN.md S.4): when only a doc-space sub-rect
// changed and the viewport is unchanged, renderRaw re-renders just the
// projected canvas rectangle instead of the whole canvas.

// partialOverlaySafe reports whether a partial redraw of the dirty doc rect is
// guaranteed not to overlap any viewport overlay. The partial path rebuilds
// base + document content inside the projected canvas rect but does NOT redraw
// overlays there, so it is only valid when the document border (a stroke along
// the doc outline) and the center guides stay clear of the redrawn region.
// The margin is conservative: it covers the 1-doc-px bilinear inflation and
// the +2-canvas-px rounding inflation of docRectToCanvasRect plus the border/
// guide stroke width and its anti-aliased falloff.
func partialOverlaySafe(doc *Document, vp *ViewportState, zoom float64, rect DirtyRect) bool {
	margin := int(math.Ceil(6.0/zoom)) + 2
	if rect.X-margin < 0 || rect.Y-margin < 0 ||
		rect.X+rect.W+margin > doc.Width || rect.Y+rect.H+margin > doc.Height {
		return false
	}
	if vp.ShowGuides {
		guideX := float64(doc.Width) / 2
		guideY := float64(doc.Height) / 2
		if float64(rect.X-margin) <= guideX && guideX <= float64(rect.X+rect.W+margin) {
			return false
		}
		if float64(rect.Y-margin) <= guideY && guideY <= float64(rect.Y+rect.H+margin) {
			return false
		}
	}
	return true
}

// docRectToCanvasRect maps a doc-space dirty rect to the canvas-space pixel
// rectangle that must be redrawn, using the same forward transform as
// docBoundsOnCanvas. The doc rect is inflated by 1 px (bilinear taps read a
// 2×2 neighbourhood) and the projected rect by 2 px (rounding/AA safety),
// then clamped to the canvas. ok is false when the result is empty (the
// change is entirely off-canvas).
func docRectToCanvasRect(vp *ViewportState, zoom float64, rect DirtyRect, canvasW, canvasH int) (DirtyRect, bool) {
	rotation := vp.Rotation * (math.Pi / 180)
	cosTheta := math.Cos(rotation)
	sinTheta := math.Sin(rotation)
	halfCanvasW := float64(maxInt(canvasW, 1)) * 0.5
	halfCanvasH := float64(maxInt(canvasH, 1)) * 0.5

	x0 := float64(rect.X - 1)
	y0 := float64(rect.Y - 1)
	x1 := float64(rect.X + rect.W + 1)
	y1 := float64(rect.Y + rect.H + 1)

	minSX := math.MaxFloat64
	minSY := math.MaxFloat64
	maxSX := -math.MaxFloat64
	maxSY := -math.MaxFloat64
	corners := [4][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	for _, c := range corners {
		dx := c[0] - vp.CenterX
		dy := c[1] - vp.CenterY
		sx := dx*cosTheta*zoom - dy*sinTheta*zoom + halfCanvasW
		sy := dx*sinTheta*zoom + dy*cosTheta*zoom + halfCanvasH
		if sx < minSX {
			minSX = sx
		}
		if sx > maxSX {
			maxSX = sx
		}
		if sy < minSY {
			minSY = sy
		}
		if sy > maxSY {
			maxSY = sy
		}
	}

	cx0 := clampInt(int(math.Floor(minSX))-2, 0, canvasW)
	cy0 := clampInt(int(math.Floor(minSY))-2, 0, canvasH)
	cx1 := clampInt(int(math.Ceil(maxSX))+3, 0, canvasW)
	cy1 := clampInt(int(math.Ceil(maxSY))+3, 0, canvasH)
	if cx1 <= cx0 || cy1 <= cy0 {
		return DirtyRect{}, false
	}
	return DirtyRect{X: cx0, Y: cy0, W: cx1 - cx0, H: cy1 - cy0}, true
}
