package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	agg "github.com/cwbudde/agg_go"
)

// AdjustmentPixelFunc transforms a single RGBA pixel using the adjustment's
// JSON params payload. The hook is intentionally generic so later phases can
// register core adjustment kinds without changing the render pipeline again.
type AdjustmentPixelFunc func(r, g, b, a uint8, params json.RawMessage) (uint8, uint8, uint8, uint8, error)

type AdjustmentFactory func(params json.RawMessage) (AdjustmentPixelFunc, error)

var adjustmentRegistry = struct {
	sync.RWMutex
	entries map[string]AdjustmentFactory
}{
	entries: make(map[string]AdjustmentFactory),
}

// RegisterAdjustmentTransform registers or replaces the transform for a given
// adjustment kind. Passing a nil function removes the registration.
func RegisterAdjustmentTransform(kind string, fn AdjustmentPixelFunc) {
	RegisterAdjustmentFactory(kind, func(json.RawMessage) (AdjustmentPixelFunc, error) {
		return fn, nil
	})
}

// RegisterAdjustmentFactory registers a factory that compiles a per-layer pixel
// transform from the layer's params payload.
func RegisterAdjustmentFactory(kind string, factory AdjustmentFactory) {
	key := normalizeAdjustmentKind(kind)
	if key == "" {
		return
	}

	adjustmentRegistry.Lock()
	defer adjustmentRegistry.Unlock()

	if factory == nil {
		delete(adjustmentRegistry.entries, key)
		return
	}
	adjustmentRegistry.entries[key] = factory
}

func lookupAdjustmentTransform(kind string, params json.RawMessage) (AdjustmentPixelFunc, error) {
	key := normalizeAdjustmentKind(kind)
	if key == "" {
		return nil, nil
	}

	adjustmentRegistry.RLock()
	defer adjustmentRegistry.RUnlock()

	factory, ok := adjustmentRegistry.entries[key]
	if !ok {
		return nil, nil
	}
	return factory(params)
}

func normalizeAdjustmentKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}

func applyAdjustmentLayerToSurface(surface []byte, docW, docH int, layer *AdjustmentLayer, clipAlpha []byte, dirtyRect *DirtyRect, allowCache bool) error {
	if layer == nil || len(surface) == 0 || docW <= 0 || docH <= 0 {
		return nil
	}

	kind := normalizeAdjustmentKind(layer.AdjustmentKind)
	resolvedParams, err := resolveAdjustmentParamsForSurface(surface, docW, docH, layer, clipAlpha)
	if err != nil {
		return err
	}

	transform, err := lookupAdjustmentTransform(kind, resolvedParams)
	if err != nil {
		return err
	}
	if transform == nil {
		return nil
	}

	rect := DirtyRect{X: 0, Y: 0, W: docW, H: docH}
	if normalized, ok := normalizeAdjustmentDirtyRect(dirtyRect, docW, docH); ok {
		rect = normalized
	}

	canReuseDirtyRegion := allowCache &&
		rect != (DirtyRect{X: 0, Y: 0, W: docW, H: docH}) &&
		adjustmentCacheMatches(layer, kind, resolvedParams, docW, docH) &&
		adjustmentSupportsDirtyRegionCache(layer)
	if canReuseDirtyRegion {
		copySurfaceOutsideRect(surface, layer.Cache.Output, rect, docW, docH)
	}

	if err := applyAdjustmentLayerRectToSurface(surface, docW, docH, layer, clipAlpha, resolvedParams, transform, rect); err != nil {
		return err
	}

	if allowCache {
		updateAdjustmentCache(layer, kind, resolvedParams, docW, docH, surface)
	}
	return nil
}

// applyAdjustmentLayerRectToSurface applies the transform to the backdrop and
// merges the result back, honouring the layer's opacity and blend mode.
//
// An adjustment layer RECOLOURS the backdrop; it is not a layer composited over
// it. The difference is the alpha channel: compositing an adjusted image
// source-over would give a half-transparent backdrop an alpha of 0.75 where it
// had 0.5, so the adjustment would add coverage that no pixel of it ever had.
// The model instead is
//
//	blended = blend(backdrop, adjusted, mode)          // colour only
//	out.rgb = lerp(backdrop.rgb, blended.rgb, coverage * opacity)
//	out.a   = lerp(backdrop.a,  adjusted.a,  coverage)
//
// which leaves alpha to the backdrop and the transform, exactly as before this
// function learned about opacity and blend modes.
func applyAdjustmentLayerRectToSurface(surface []byte, docW, docH int, layer *AdjustmentLayer, clipAlpha []byte, resolvedParams json.RawMessage, transform AdjustmentPixelFunc, rect DirtyRect) error {
	opacity := clampUnit(effectiveLayerOpacity(layer))
	if opacity <= 0 {
		return nil
	}
	mode := layer.BlendMode()

	// Normal at full opacity is the overwhelmingly common case and is exactly
	// "replace the backdrop within the coverage", so it keeps the direct loop:
	// the general path below needs two rect-sized buffers and a pass through the
	// compositor, and this is the hot path S.4 spent its budget on.
	if mode == BlendModeNormal && opacity >= 1 {
		return applyAdjustmentRectDirect(surface, docW, layer, clipAlpha, resolvedParams, transform, rect)
	}
	return applyAdjustmentRectBlended(surface, docW, layer, clipAlpha, resolvedParams, transform, rect, mode, opacity)
}

// applyAdjustmentRectDirect is the Normal-at-100% path: the transformed pixel
// replaces the backdrop, scaled by coverage alone.
func applyAdjustmentRectDirect(surface []byte, docW int, layer *AdjustmentLayer, clipAlpha []byte, resolvedParams json.RawMessage, transform AdjustmentPixelFunc, rect DirtyRect) error {
	mask := layer.Mask()
	for y := rect.Y; y < rect.Y+rect.H; y++ {
		for x := rect.X; x < rect.X+rect.W; x++ {
			index := (y*docW + x) * 4
			if index < 0 || index+3 >= len(surface) {
				continue
			}

			coverage := clipSurfaceAlphaAt(clipAlpha, docW, x, y)
			coverage = scaleMaskedAlpha(coverage, layerMaskAlphaAt(mask, x, y))
			if coverage == 0 {
				continue
			}

			r, g, b, a, err := transform(surface[index], surface[index+1], surface[index+2], surface[index+3], resolvedParams)
			if err != nil {
				return fmt.Errorf("adjustment layer %q: %w", layer.Name(), err)
			}

			if coverage == 255 {
				surface[index] = r
				surface[index+1] = g
				surface[index+2] = b
				surface[index+3] = a
				continue
			}

			surface[index] = blendByte(surface[index], r, coverage)
			surface[index+1] = blendByte(surface[index+1], g, coverage)
			surface[index+2] = blendByte(surface[index+2], b, coverage)
			surface[index+3] = blendByte(surface[index+3], a, coverage)
		}
	}
	return nil
}

// applyAdjustmentRectBlended handles a non-Normal blend mode or a reduced
// opacity.
//
// The blend itself goes through the same agg_go compositor every other layer
// uses rather than a hand-written mode switch: the 27 modes and their edge cases
// are exactly what must not be reimplemented here. It is fed two fully opaque
// rect-sized images — the backdrop's colour and the adjusted colour — because
// with both opaque, source-over reduces to `lerp(dest, blend(dest, src),
// opacity)`, which is the colour half of the model above. The surviving loop
// then merges that back under the coverage and restores the backdrop's alpha.
func applyAdjustmentRectBlended(surface []byte, docW int, layer *AdjustmentLayer, clipAlpha []byte, resolvedParams json.RawMessage, transform AdjustmentPixelFunc, rect DirtyRect, mode BlendMode, opacity float64) error {
	pixels := rect.W * rect.H
	if pixels <= 0 {
		return nil
	}

	adjusted := acquireSurface(pixels * 4)
	defer releaseSurface(adjusted)
	backdrop := acquireSurface(pixels * 4)
	defer releaseSurface(backdrop)

	for y := range rect.H {
		for x := range rect.W {
			index := ((rect.Y+y)*docW + rect.X + x) * 4
			if index < 0 || index+3 >= len(surface) {
				continue
			}
			offset := (y*rect.W + x) * 4

			r, g, b, a, err := transform(surface[index], surface[index+1], surface[index+2], surface[index+3], resolvedParams)
			if err != nil {
				return fmt.Errorf("adjustment layer %q: %w", layer.Name(), err)
			}
			adjusted[offset], adjusted[offset+1], adjusted[offset+2], adjusted[offset+3] = r, g, b, a

			// Opaque copies: the blend is a colour operation, and letting the
			// backdrop's own alpha reach the compositor would turn it into a
			// coverage operation as well.
			backdrop[offset] = surface[index]
			backdrop[offset+1] = surface[index+1]
			backdrop[offset+2] = surface[index+2]
			backdrop[offset+3] = 255
		}
	}

	// Blend into a copy of the adjusted colours so `adjusted` keeps the
	// transform's alpha for the merge below.
	blended := acquireSurface(pixels * 4)
	defer releaseSurface(blended)
	copy(blended, adjusted)
	for offset := 3; offset < len(blended); offset += 4 {
		blended[offset] = 255
	}

	if err := compositeImageStraight(
		backdrop, rect.W, rect.H,
		blended, rect.W, rect.H,
		agg.Rect{X2: rect.W, Y2: rect.H},
		agg.PointI{},
		mode, opacity,
		nil, agg.PointI{},
		nil, engineDissolveSeed,
	); err != nil {
		return fmt.Errorf("adjustment layer %q: blend: %w", layer.Name(), err)
	}

	mask := layer.Mask()
	for y := range rect.H {
		docY := rect.Y + y
		for x := range rect.W {
			docX := rect.X + x
			index := (docY*docW + docX) * 4
			if index < 0 || index+3 >= len(surface) {
				continue
			}

			coverage := clipSurfaceAlphaAt(clipAlpha, docW, docX, docY)
			coverage = scaleMaskedAlpha(coverage, layerMaskAlphaAt(mask, docX, docY))
			if coverage == 0 {
				continue
			}
			offset := (y*rect.W + x) * 4

			surface[index] = blendByte(surface[index], backdrop[offset], coverage)
			surface[index+1] = blendByte(surface[index+1], backdrop[offset+1], coverage)
			surface[index+2] = blendByte(surface[index+2], backdrop[offset+2], coverage)
			// Alpha follows the transform, never the blend: an adjustment layer
			// recolours what is there and must not add or remove coverage.
			surface[index+3] = blendByte(surface[index+3], adjusted[offset+3], coverage)
		}
	}
	return nil
}

func normalizeAdjustmentDirtyRect(rect *DirtyRect, docW, docH int) (DirtyRect, bool) {
	if rect == nil {
		return DirtyRect{}, false
	}
	normalized, err := normalizeDirtyRect(*rect, docW, docH)
	if err != nil {
		return DirtyRect{}, false
	}
	return normalized, true
}

func adjustmentSupportsDirtyRegionCache(layer *AdjustmentLayer) bool {
	if layer == nil {
		return false
	}
	switch normalizeAdjustmentKind(layer.AdjustmentKind) {
	case "levels":
		cfg, err := decodeAdjustmentParams[levelsParams](layer.Params)
		return err == nil && !cfg.Auto
	case "black-white", "blackandwhite", "black & white", "black/white":
		cfg, err := decodeAdjustmentParams[blackWhiteParams](layer.Params)
		return err == nil && !cfg.Auto
	default:
		return true
	}
}

func adjustmentCacheMatches(layer *AdjustmentLayer, kind string, resolvedParams json.RawMessage, docW, docH int) bool {
	if layer == nil {
		return false
	}
	return layer.Cache.Kind == kind &&
		layer.Cache.DocW == docW &&
		layer.Cache.DocH == docH &&
		layer.Cache.Opacity == effectiveLayerOpacity(layer) &&
		layer.Cache.BlendMode == layer.BlendMode() &&
		len(layer.Cache.Output) == docW*docH*4 &&
		bytes.Equal(layer.Cache.ResolvedParams, resolvedParams)
}

func updateAdjustmentCache(layer *AdjustmentLayer, kind string, resolvedParams json.RawMessage, docW, docH int, surface []byte) {
	if layer == nil {
		return
	}
	layer.Cache.Kind = kind
	layer.Cache.DocW = docW
	layer.Cache.DocH = docH
	layer.Cache.Opacity = effectiveLayerOpacity(layer)
	layer.Cache.BlendMode = layer.BlendMode()
	layer.Cache.ResolvedParams = cloneJSONRawMessage(resolvedParams)
	if len(layer.Cache.Output) != len(surface) {
		layer.Cache.Output = make([]byte, len(surface))
	}
	copy(layer.Cache.Output, surface)
}

func copySurfaceOutsideRect(dest, src []byte, rect DirtyRect, width, height int) {
	if len(dest) != len(src) || width <= 0 || height <= 0 {
		return
	}
	rowBytes := width * 4
	rectStartY := rect.Y
	rectEndY := rect.Y + rect.H
	rectStartXBytes := rect.X * 4
	rectEndXBytes := (rect.X + rect.W) * 4

	for y := 0; y < height; y++ {
		rowStart := y * rowBytes
		rowEnd := rowStart + rowBytes
		if y < rectStartY || y >= rectEndY {
			copy(dest[rowStart:rowEnd], src[rowStart:rowEnd])
			continue
		}
		if rectStartXBytes > 0 {
			copy(dest[rowStart:rowStart+rectStartXBytes], src[rowStart:rowStart+rectStartXBytes])
		}
		if rectEndXBytes < rowBytes {
			copy(dest[rowStart+rectEndXBytes:rowEnd], src[rowStart+rectEndXBytes:rowEnd])
		}
	}
}

func resolveAdjustmentParamsForSurface(surface []byte, docW, docH int, layer *AdjustmentLayer, clipAlpha []byte) (json.RawMessage, error) {
	if layer == nil {
		return nil, nil
	}
	switch normalizeAdjustmentKind(layer.AdjustmentKind) {
	case "levels":
		return resolveLevelsParamsForSurface(surface, docW, docH, layer, clipAlpha)
	case "black-white", "blackandwhite", "black & white", "black/white":
		return resolveBlackWhiteParamsForSurface(surface, docW, docH, layer, clipAlpha)
	default:
		return layer.Params, nil
	}
}

func blendByte(base, target, alpha uint8) uint8 {
	return uint8((uint32(base)*(255-uint32(alpha)) + uint32(target)*uint32(alpha) + 127) / 255)
}
