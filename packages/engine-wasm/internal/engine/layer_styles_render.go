package engine

import (
	"fmt"
	"math"
)

func (doc *Document) renderStyledLayerSurface(layer LayerNode, clipAlpha []byte) ([]byte, error) {
	baseSurface, err := doc.renderRasterizableContentSurface(layer, clipAlpha, effectiveContentOpacity(layer))
	if err != nil {
		return nil, err
	}

	decoded := decodeLayerStyles(layer.StyleStack())
	if !hasSupportedEnabledLayerStyles(decoded) {
		return baseSurface, nil
	}

	sourceSurface, err := doc.renderRasterizableContentSurface(layer, clipAlpha, 1)
	if err != nil {
		return nil, err
	}
	return applyLayerStylesToSurface(baseSurface, sourceSurface, doc.Width, doc.Height, decoded), nil
}

func (doc *Document) renderRasterizableContentSurface(layer LayerNode, clipAlpha []byte, opacity float64) ([]byte, error) {
	bounds, raster, mask, err := rasterizableLayerSource(layer)
	if err != nil {
		return nil, err
	}
	return buildDocumentSurfaceFromRaster(doc.Width, doc.Height, bounds, raster, mask, clipAlpha, opacity)
}

func (doc *Document) renderClipBaseSurface(layer LayerNode) ([]byte, error) {
	switch typed := layer.(type) {
	case *PixelLayer, *TextLayer, *VectorLayer:
		return doc.renderRasterizableContentSurface(typed, nil, effectiveContentOpacity(typed))
	default:
		return doc.renderLayerToSurface(layer)
	}
}

func rasterizableLayerSource(layer LayerNode) (LayerBounds, []byte, *LayerMask, error) {
	switch typed := layer.(type) {
	case *PixelLayer:
		return typed.Bounds, typed.Pixels, typed.Mask(), nil
	case *TextLayer:
		return typed.Bounds, typed.CachedRaster, typed.Mask(), nil
	case *VectorLayer:
		return typed.Bounds, typed.CachedRaster, typed.Mask(), nil
	default:
		return LayerBounds{}, nil, nil, fmtUnsupportedStyledLayer(layer)
	}
}

func buildDocumentSurfaceFromRaster(docW, docH int, bounds LayerBounds, src []byte, mask *LayerMask, clipAlpha []byte, opacity float64) ([]byte, error) {
	surface := make([]byte, docW*docH*4)
	if docW <= 0 || docH <= 0 || bounds.W <= 0 || bounds.H <= 0 || len(src) == 0 || opacity <= 0 {
		return surface, nil
	}

	expectedLen := bounds.W * bounds.H * 4
	if len(src) != expectedLen {
		return nil, errRasterLengthMismatch(bounds, len(src))
	}

	for y := 0; y < bounds.H; y++ {
		docY := bounds.Y + y
		if docY < 0 || docY >= docH {
			continue
		}
		for x := 0; x < bounds.W; x++ {
			docX := bounds.X + x
			if docX < 0 || docX >= docW {
				continue
			}

			srcIndex := (y*bounds.W + x) * 4
			alpha := src[srcIndex+3]
			if alpha == 0 {
				continue
			}

			maskAlpha := layerMaskAlphaAt(mask, docX, docY)
			maskAlpha = scaleMaskedAlpha(maskAlpha, clipSurfaceAlphaAt(clipAlpha, docW, docX, docY))
			if maskAlpha == 0 {
				continue
			}

			effectiveAlpha := scaleMaskedAlpha(alpha, maskAlpha)
			effectiveAlpha = scaleMaskedAlpha(effectiveAlpha, uint8(math.Round(clampUnit(opacity)*255)))
			if effectiveAlpha == 0 {
				continue
			}

			destIndex := (docY*docW + docX) * 4
			copy(surface[destIndex:destIndex+4], src[srcIndex:srcIndex+4])
			surface[destIndex+3] = effectiveAlpha
		}
	}

	return surface, nil
}

// applyLayerStylesToSurface assembles the self-contained styled-layer
// surface in Photoshop z-order: drop shadow and outer glow first (beneath
// the layer's own pixels), then the layer content, then the remaining
// effects above it. The result depends only on baseSurface/sourceSurface,
// never on the document backdrop, so incremental compositing stays valid.
func applyLayerStylesToSurface(baseSurface, sourceSurface []byte, docW, docH int, styles []DecodedLayerStyle) []byte {
	if !hasEnabledBehindContentStyles(styles) {
		// No drop shadow / outer glow enabled: skip the full-document
		// composite pass and paint the top-side effects straight onto a
		// copy of the layer content, exactly as before.
		finalSurface := append([]byte(nil), baseSurface...)
		applyLayerStyleEffectsForPlacement(finalSurface, sourceSurface, docW, docH, styles, false)
		return finalSurface
	}

	finalSurface := make([]byte, len(baseSurface))
	applyLayerStyleEffectsForPlacement(finalSurface, sourceSurface, docW, docH, styles, true)
	compositeDocumentSurface(finalSurface, baseSurface, BlendModeNormal, 1, nil)
	applyLayerStyleEffectsForPlacement(finalSurface, sourceSurface, docW, docH, styles, false)
	return finalSurface
}

func hasEnabledBehindContentStyles(styles []DecodedLayerStyle) bool {
	for _, style := range styles {
		if style.Enabled && layerStyleKindRendersBehindContent(LayerStyleKind(style.Kind)) {
			return true
		}
	}
	return false
}

func applyLayerStyleEffectsForPlacement(dst, sourceSurface []byte, docW, docH int, styles []DecodedLayerStyle, behindContent bool) {
	for _, kind := range orderedLayerStyleKinds() {
		if layerStyleKindRendersBehindContent(kind) != behindContent {
			continue
		}
		for _, style := range styles {
			if !style.Enabled || LayerStyleKind(style.Kind) != kind {
				continue
			}
			applyLayerStyleEffect(dst, sourceSurface, docW, docH, style)
		}
	}
}

func hasSupportedEnabledLayerStyles(styles []DecodedLayerStyle) bool {
	for _, style := range styles {
		if style.Enabled && isSupportedLayerStyleKind(LayerStyleKind(style.Kind)) {
			return true
		}
	}
	return false
}

func hasSupportedEnabledLayerStyleStack(styles []LayerStyle) bool {
	return hasSupportedEnabledLayerStyles(decodeLayerStyles(styles))
}

func hasAnyEnabledLayerStyleEntry(styles []LayerStyle) bool {
	for _, style := range styles {
		if style.Enabled {
			return true
		}
	}
	return false
}

func fmtUnsupportedStyledLayer(layer LayerNode) error {
	return fmt.Errorf("unsupported styled layer type %T", layer)
}

func errRasterLengthMismatch(bounds LayerBounds, got int) error {
	return fmt.Errorf("raster length %d does not match bounds %dx%d", got, bounds.W, bounds.H)
}
