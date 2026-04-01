package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

func applyAdjustmentLayerToSurface(surface []byte, docW, docH int, layer *AdjustmentLayer, clipAlpha []byte) error {
	if layer == nil || len(surface) == 0 || docW <= 0 || docH <= 0 {
		return nil
	}

	resolvedParams, err := resolveAdjustmentParamsForSurface(surface, docW, docH, layer, clipAlpha)
	if err != nil {
		return err
	}

	transform, err := lookupAdjustmentTransform(layer.AdjustmentKind, resolvedParams)
	if err != nil {
		return err
	}
	if transform == nil {
		return nil
	}

	mask := layer.Mask()
	for y := 0; y < docH; y++ {
		for x := 0; x < docW; x++ {
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
