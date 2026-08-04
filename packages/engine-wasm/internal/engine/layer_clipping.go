package engine

import agg "github.com/cwbudde/agg_go"

func clippingBaseIndex(children []LayerNode, index int) int {
	for candidate := index - 1; candidate >= 0; candidate-- {
		if children[candidate] == nil {
			continue
		}
		if _, ok := children[candidate].(*AdjustmentLayer); ok {
			continue
		}
		if !children[candidate].ClipToBelow() {
			return candidate
		}
	}
	return -1
}

func clippingBaseRenderableIndex(children []LayerNode, index int) int {
	if index < 0 || index >= len(children) {
		return -1
	}
	if _, ok := children[index].(*AdjustmentLayer); !ok {
		return index
	}
	for candidate := index - 1; candidate >= 0; candidate-- {
		layer := children[candidate]
		if layer == nil || layer.ClipToBelow() {
			continue
		}
		if _, ok := layer.(*AdjustmentLayer); ok {
			continue
		}
		return candidate
	}
	return -1
}

func (doc *Document) clippingBaseSurfaceForLayer(layer LayerNode) ([]byte, error) {
	if doc == nil || layer == nil || !layer.ClipToBelow() {
		return nil, nil
	}
	parent := layer.Parent()
	group, ok := parent.(*GroupLayer)
	if !ok || group == nil {
		return nil, nil
	}
	children := group.Children()
	for index, candidate := range children {
		if candidate == nil || candidate.ID() != layer.ID() {
			continue
		}
		baseIndex := clippingBaseIndex(children, index)
		if baseIndex < 0 {
			return nil, nil
		}
		return doc.renderClipBaseSurface(children[baseIndex])
	}
	return nil, nil
}

// compositeDocumentSurfaceClipped composites src onto dest restricted to a
// doc-space rectangle. A nil clip composites the full surface. The per-pixel
// dissolve noise seed is pixelNoiseSeed(docX, docY) — the same convention as
// compositeRasterIntoDocument — so clipped output is byte-identical to the
// full pass inside the clip rect (dissolve blending).
func compositeDocumentSurfaceClipped(dest, src []byte, docW int, blendMode BlendMode, opacity float64, blendIf *BlendIfConfig, clip *DirtyRect) {
	if len(dest) != len(src) || opacity <= 0 || docW <= 0 {
		return
	}
	docH := len(dest) / (docW * 4)
	if docH <= 0 || docW*docH*4 != len(dest) {
		return
	}
	identity := blendIfIsIdentity(blendIf)
	var original []byte
	if !identity {
		original = acquireSurface(len(dest))
		defer releaseSurface(original)
		copy(original, dest)
	}

	coverage := buildDocumentCompositeMask(docW, docH, src, dest, blendIf)
	defer releaseCompositeMask(coverage)
	_ = compositeImageStraight(
		dest, docW, docH,
		src, docW, docH,
		agg.Rect{X2: docW, Y2: docH}, agg.PointI{},
		blendMode, opacity,
		coverage, agg.PointI{}, aggRectFromDirty(clip), engineDissolveSeed,
	)
	if !identity {
		applyBlendIfChannelsClipped(dest, original, docW, docH, LayerBounds{W: docW, H: docH}, blendIf, clip)
	}
}
