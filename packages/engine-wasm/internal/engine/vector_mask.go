// Package engine — vector mask rendering (Phase S.6, Batch V1).
//
// Semantics (Photoshop behavior):
//   - Raster AND vector masks both apply: their coverages are MULTIPLIED
//     (intersection).
//   - An EMPTY vector mask — no anchor points in any subpath, which is exactly
//     the placeholder AddVectorMask creates — means "reveal all": it yields nil
//     coverage and contributes nothing, keeping the placeholder byte-identical
//     to having no vector mask at all.
//   - Vector masks have no enabled flag; a DISABLED raster mask does not
//     disable the vector mask.
package engine

import "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"

// commandSetVectorMaskPath mirrors the ABI id declared in
// internal/command/layer.go (and CommandID.SetVectorMaskPath in
// packages/proto/src/commands.ts).
//
//nolint:unused // part of the engine ABI, referenced from tests.
const commandSetVectorMaskPath = 0x012a

// pathHasAnchorPoints reports whether any subpath carries at least one anchor
// point. Paths without anchors (e.g. the AddVectorMask placeholder) are
// treated as "reveal all".
func pathHasAnchorPoints(p *Path) bool {
	if p == nil {
		return false
	}
	for i := range p.Subpaths {
		if len(p.Subpaths[i].Points) > 0 {
			return true
		}
	}
	return false
}

// vectorMaskCoverage returns the doc-sized 8-bit coverage (255 = inside) of
// the layer's vector mask, or nil when the layer has no vector mask or an
// empty one (reveal all). Results are memoized on the layer via
// model.VectorMaskRasterCache; the cache is validated by content
// (dimensions + PathEqual) because transforms and crop mutate the mask path
// in place through the VectorMask() pointer.
//
// The returned slice is the cache's backing store: treat it as read-only.
func vectorMaskCoverage(layer LayerNode, docW, docH int) []byte {
	vm := layer.VectorMask()
	if vm == nil || !pathHasAnchorPoints(vm) {
		return nil
	}
	if cache := layer.VectorMaskRaster(); cache != nil && cache.W == docW && cache.H == docH && model.PathEqual(cache.Path, vm) {
		return cache.Data
	}
	data, err := rasterizePathToMask(vm, docW, docH)
	if err != nil {
		// Defensive: rasterizePathToMask only errors for nil/empty-subpath
		// paths, which are excluded above. Treat a failure as reveal-all
		// rather than aborting the composite.
		return nil
	}
	layer.SetVectorMaskRaster(&model.VectorMaskRasterCache{
		W:    docW,
		H:    docH,
		Path: model.ClonePath(vm),
		Data: data,
	})
	return data
}

// effectiveLayerMask resolves the mask compositing should apply to the layer:
//
//   - no (or empty) vector mask → the raster mask unchanged (zero-cost common
//     path, byte-identical to historical behavior);
//   - vector mask only, or vector mask with a nil/DISABLED raster mask → the
//     vector coverage wrapped as an enabled doc-sized LayerMask. The wrapper
//     SHARES the cached coverage slice — consumers are read-only
//     (layerMaskAlphaAt / applyLayerMaskToSurface never write mask data);
//   - both enabled → a freshly allocated multiply of the two coverages
//     (Photoshop intersection). Never cached: mask brush strokes mutate
//     LayerMask.Data in place mid-stroke, so the combination must be rebuilt
//     per composite.
func (doc *Document) effectiveLayerMask(layer LayerNode) *LayerMask {
	coverage := vectorMaskCoverage(layer, doc.Width, doc.Height)
	if coverage == nil {
		return layer.Mask()
	}
	raster := layer.Mask()
	if raster == nil || !raster.Enabled {
		return &LayerMask{Enabled: true, Width: doc.Width, Height: doc.Height, Data: coverage}
	}
	combined := make([]byte, doc.Width*doc.Height)
	for y := 0; y < doc.Height; y++ {
		row := y * doc.Width
		for x := 0; x < doc.Width; x++ {
			combined[row+x] = scaleMaskedAlpha(layerMaskAlphaAt(raster, x, y), coverage[row+x])
		}
	}
	return &LayerMask{Enabled: true, Width: doc.Width, Height: doc.Height, Data: combined}
}
