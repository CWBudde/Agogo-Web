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

func applyLayerStylesToSurface(baseSurface, sourceSurface []byte, docW, docH int, styles []DecodedLayerStyle) []byte {
	finalSurface := append([]byte(nil), baseSurface...)
	for _, style := range styles {
		if !style.Enabled {
			continue
		}

		switch LayerStyleKind(style.Kind) {
		case LayerStyleKindColorOverlay:
			overlaySurface := buildColorOverlaySurface(sourceSurface, style.ColorOverlay)
			compositeDocumentSurface(finalSurface, overlaySurface, style.ColorOverlay.BlendMode, style.ColorOverlay.Opacity)
		case LayerStyleKindDropShadow:
			shadowSurface := buildDropShadowSurface(sourceSurface, docW, docH, style.DropShadow)
			compositeDocumentSurface(finalSurface, shadowSurface, style.DropShadow.BlendMode, style.DropShadow.Opacity)
		}
	}
	return finalSurface
}

func hasSupportedEnabledLayerStyles(styles []DecodedLayerStyle) bool {
	for _, style := range styles {
		if !style.Enabled {
			continue
		}
		switch LayerStyleKind(style.Kind) {
		case LayerStyleKindColorOverlay, LayerStyleKindDropShadow:
			return true
		}
	}
	return false
}

func hasSupportedEnabledLayerStyleStack(styles []LayerStyle) bool {
	return hasSupportedEnabledLayerStyles(decodeLayerStyles(styles))
}

func buildColorOverlaySurface(sourceSurface []byte, params ColorOverlayParams) []byte {
	overlay := make([]byte, len(sourceSurface))
	for offset := 0; offset < len(sourceSurface); offset += 4 {
		sourceAlpha := sourceSurface[offset+3]
		if sourceAlpha == 0 {
			continue
		}
		copy(overlay[offset:offset+4], params.Color[:])
		overlay[offset+3] = scaleMaskedAlpha(sourceAlpha, params.Color[3])
	}
	return overlay
}

func buildDropShadowSurface(sourceSurface []byte, docW, docH int, params DropShadowParams) []byte {
	shadow := make([]byte, len(sourceSurface))
	if len(sourceSurface) == 0 || docW <= 0 || docH <= 0 {
		return shadow
	}

	dx, dy := dropShadowOffset(params.Angle, params.Distance)
	for offset := 0; offset < len(sourceSurface); offset += 4 {
		sourceAlpha := sourceSurface[offset+3]
		if sourceAlpha == 0 {
			continue
		}

		pixelIndex := offset / 4
		srcX := pixelIndex % docW
		srcY := pixelIndex / docW
		dstX := srcX + dx
		dstY := srcY + dy
		if dstX < 0 || dstX >= docW || dstY < 0 || dstY >= docH {
			continue
		}

		destIndex := (dstY*docW + dstX) * 4

		pixel := [4]byte{params.Color[0], params.Color[1], params.Color[2], scaleMaskedAlpha(sourceAlpha, params.Color[3])}
		compositePixelWithBlend(shadow[destIndex:destIndex+4], pixel[:], BlendModeNormal, 1, pixelNoiseSeed(dstX, dstY))
	}

	return shadow
}

func dropShadowOffset(angle, distance float64) (int, int) {
	radians := angle * math.Pi / 180
	dx := int(math.Round(math.Cos(radians) * distance))
	dy := int(math.Round(-math.Sin(radians) * distance))
	return dx, dy
}

func fmtUnsupportedStyledLayer(layer LayerNode) error {
	return fmt.Errorf("unsupported styled layer type %T", layer)
}

func errRasterLengthMismatch(bounds LayerBounds, got int) error {
	return fmt.Errorf("raster length %d does not match bounds %dx%d", got, bounds.W, bounds.H)
}
