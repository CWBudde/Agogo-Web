package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

// surfacePool recycles document-sized RGBA scratch buffers to cut GC churn
// during recompositing (PLAN.md S.4). A single recomposite of a 4000x3000 doc
// with grouped/styled layers otherwise allocates tens of MB of transient
// buffers that immediately become garbage, which is the dominant GC-hitch
// source on the paint path.
//
// OWNERSHIP RULE (read before using):
//
//	acquireSurface(n) returns a ZEROED []byte of exactly n bytes (compositing
//	assumes a cleared destination). releaseSurface returns it to the pool.
//
//	A buffer obtained from acquireSurface MUST be released by the SAME function
//	that acquired it, and ONLY when it provably does NOT escape that function —
//	i.e. it is used purely as transient compositing scratch and dropped before
//	return. NEVER release a buffer that is:
//	  - returned to the caller (e.g. renderLayer(s)ToSurface* results),
//	  - stored on a struct or cached (inst.cachedDocSurface in render.go),
//	  - wrapped into a layer (NewPixelLayer keeps the slice).
//	Those escape and must keep allocating with make(). Releasing an escaped
//	buffer would let a later acquireSurface hand the same memory to another
//	frame and corrupt it.
//
// The pool stores *[]byte (not []byte) so Put does not box-allocate on every
// release (staticcheck SA6002).
var surfacePool = sync.Pool{New: func() any { return new([]byte) }}

// acquireSurface returns a zeroed byte slice of exactly size bytes, reusing a
// pooled buffer when one of sufficient capacity is available.
func acquireSurface(size int) []byte {
	buf := *surfacePool.Get().(*[]byte)
	if cap(buf) >= size {
		buf = buf[:size]
		clear(buf)
		return buf
	}
	return make([]byte, size)
}

// releaseSurface returns a transient scratch buffer to the pool. See the
// ownership rule on surfacePool — only non-escaping buffers may be released.
func releaseSurface(buf []byte) {
	if buf == nil {
		return
	}
	full := buf[:cap(buf)]
	surfacePool.Put(&full)
}

func (doc *Document) ensureLayerRoot() *GroupLayer {
	doc.LayerRoot = docpkg.EnsureLayerRoot(doc.LayerRoot)
	return doc.LayerRoot
}

func (doc *Document) Layers() []LayerNode {
	return docpkg.Layers(doc.ensureLayerRoot())
}

func (doc *Document) ActiveLayer() LayerNode {
	if doc == nil || doc.ActiveLayerID == "" {
		return nil
	}
	return docpkg.ActiveLayer(doc.ensureLayerRoot(), doc.ActiveLayerID)
}

func (doc *Document) AddLayer(layer LayerNode, parentLayerID string, index int) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if err := docpkg.AddLayer(doc.ensureLayerRoot(), layer, parentLayerID, index); err != nil {
		return err
	}
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	doc.ActiveLayerID = layer.ID()
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) DeleteLayer(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, nextActive, activeChanged, err := docpkg.DeleteLayer(doc.ensureLayerRoot(), layerID, doc.ActiveLayerID)
	if err != nil {
		return err
	}
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	if activeChanged {
		doc.ActiveLayerID = nextActive
	}
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) DuplicateLayer(layerID, parentLayerID string, index int) (LayerNode, error) {
	clone, err := docpkg.DuplicateLayer(doc.ensureLayerRoot(), layerID, parentLayerID, index)
	if err != nil {
		return nil, err
	}
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	doc.ActiveLayerID = clone.ID()
	doc.touchModifiedAtLayer(clone)
	return clone, nil
}

func (doc *Document) MoveLayer(layerID, parentLayerID string, index int) error {
	if err := docpkg.MoveLayer(doc.ensureLayerRoot(), layerID, parentLayerID, index); err != nil {
		return err
	}
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerVisibility(layerID string, visible bool) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetVisible(visible)
	doc.invalidateLayerSolo(false)
	doc.touchModifiedAtLayer(layer)
	return nil
}

// SoloLayerVisibility atomically isolates one row. A group keeps the baseline
// visibility of its complete subtree; a leaf keeps only itself, its ancestors,
// and the clipping base required to render it. Repeating the gesture on the
// same target restores the guarded baseline, while choosing another target
// re-isolates from that original baseline.
func (doc *Document) SoloLayerVisibility(layerID string) error {
	root := doc.ensureLayerRoot()
	state := &root.VisibilitySolo
	treeSignature := layerTreeStructureSignature(root)
	guardValid := state.GuardVersion == state.TreeVersion && state.TreeSignature == treeSignature
	target, parent, targetIndex, ok := findLayerByID(root, layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if state.TargetID == layerID && state.Visibility != nil && guardValid {
		walkLayerTree(doc.LayerRoot, func(layer LayerNode) {
			if visible, exists := state.Visibility[layer.ID()]; exists {
				layer.SetVisible(visible)
			}
		})
		doc.clearLayerSolo()
		doc.touchModifiedAt()
		return nil
	}
	if state.Visibility == nil || !guardValid {
		state.Visibility = make(map[string]bool)
		walkLayerTree(doc.LayerRoot, func(layer LayerNode) {
			if layer != doc.LayerRoot {
				state.Visibility[layer.ID()] = layer.Visible()
			}
		})
		state.GuardVersion = state.TreeVersion
		state.TreeSignature = treeSignature
	}

	keep := make(map[string]bool)
	forceVisible := make(map[string]bool)
	keep[layerID] = true
	forceVisible[layerID] = true
	for ancestor := target.Parent(); ancestor != nil && ancestor != doc.LayerRoot; ancestor = ancestor.Parent() {
		keep[ancestor.ID()] = true
		forceVisible[ancestor.ID()] = true
	}
	if target.LayerType() == LayerTypeGroup {
		walkLayerTree(target, func(layer LayerNode) { keep[layer.ID()] = true })
	} else if target.ClipToBelow() && parent != nil {
		children := parent.Children()
		if baseIndex := clippingBaseIndex(children, targetIndex); baseIndex >= 0 {
			base := children[baseIndex]
			keep[base.ID()] = true
			forceVisible[base.ID()] = true
			for ancestor := base.Parent(); ancestor != nil && ancestor != doc.LayerRoot; ancestor = ancestor.Parent() {
				keep[ancestor.ID()] = true
				forceVisible[ancestor.ID()] = true
			}
		}
	}
	baseline := state.Visibility
	walkLayerTree(doc.LayerRoot, func(layer LayerNode) {
		if layer == doc.LayerRoot {
			return
		}
		if !keep[layer.ID()] {
			layer.SetVisible(false)
			return
		}
		if forceVisible[layer.ID()] {
			layer.SetVisible(true)
			return
		}
		layer.SetVisible(baseline[layer.ID()])
	})
	state.TargetID = layerID
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) clearLayerSolo() {
	state := &doc.ensureLayerRoot().VisibilitySolo
	state.TargetID = ""
	state.GuardVersion = 0
	state.TreeSignature = 0
	state.Visibility = nil
}

func (doc *Document) invalidateLayerSolo(structural bool) {
	state := &doc.ensureLayerRoot().VisibilitySolo
	if structural {
		state.TreeVersion++
	}
	doc.clearLayerSolo()
}

func layerTreeStructureSignature(root LayerNode) uint64 {
	hash := fnv.New64a()
	walkLayerTree(root, func(layer LayerNode) {
		parentID := ""
		if parent := layer.Parent(); parent != nil {
			parentID = parent.ID()
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%s\x00", layer.ID(), layer.LayerType(), parentID)
	})
	return hash.Sum64()
}

func (doc *Document) SetLayerOpacity(layerID string, opacity, fillOpacity *float64) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if opacity != nil {
		layer.SetOpacity(*opacity)
	}
	if fillOpacity != nil {
		layer.SetFillOpacity(*fillOpacity)
	}
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) SetLayerBlendMode(layerID string, mode BlendMode) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetBlendMode(mode)
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) SetAdjustmentLayerParams(layerID, adjustmentKind string, params json.RawMessage) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	typed, ok := layer.(*AdjustmentLayer)
	if !ok {
		return fmt.Errorf("layer %q is not an adjustment layer", layer.Name())
	}
	if adjustmentKind != "" {
		typed.AdjustmentKind = adjustmentKind
	}
	if typed.AdjustmentKind == "" {
		return fmt.Errorf("adjustment layer %q requires adjustmentKind", layer.Name())
	}
	typed.Params = cloneJSONRawMessage(params)
	doc.touchModifiedAtLayer(typed)
	return nil
}

func (doc *Document) SetLayerLock(layerID string, mode LayerLockMode) error {
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetLockMode(mode)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetActiveLayer(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if layerID == "" {
		doc.ActiveLayerID = ""
		return nil
	}
	if _, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID); !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	doc.ActiveLayerID = layerID
	return nil
}

func (doc *Document) SetLayerName(layerID, name string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	layer.SetName(name)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) TranslateLayer(layerID string, dx, dy int) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if dx == 0 && dy == 0 {
		return nil
	}
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	if layerID == "" {
		return fmt.Errorf("no active layer")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if err := ensureLayerEditable(layer, editLayerPosition); err != nil {
		return err
	}
	beforeRect, hasBefore := doc.layerCompositeDirtyRect(layer)
	if err := translateLayerNode(layer, dx, dy); err != nil {
		return err
	}
	afterRect, hasAfter := doc.layerCompositeDirtyRect(layer)
	switch {
	case hasBefore && hasAfter:
		doc.touchModifiedAtRect(docpkg.UnionDirtyRects(beforeRect, afterRect))
	case hasBefore:
		doc.touchModifiedAtRect(beforeRect)
	case hasAfter:
		doc.touchModifiedAtRect(afterRect)
	default:
		doc.touchModifiedAt()
	}
	return nil
}

func (doc *Document) PickLayerAtPoint(x, y int) (LayerNode, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	if x < 0 || x >= doc.Width || y < 0 || y >= doc.Height {
		doc.ActiveLayerID = ""
		return nil, nil
	}
	layer := topmostLayerAtPoint(doc, doc.ensureLayerRoot().Children(), x, y)
	if layer == nil {
		doc.ActiveLayerID = ""
		return nil, nil
	}
	doc.ActiveLayerID = layer.ID()
	return layer, nil
}

func (doc *Document) AddLayerMask(layerID string, mode AddLayerMaskMode) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.Mask() != nil {
		return fmt.Errorf("layer %q already has a mask", layer.Name())
	}
	var fill byte
	switch mode {
	case "", AddLayerMaskRevealAll:
		fill = 255
	case AddLayerMaskHideAll:
		fill = 0
	case AddLayerMaskFromSelection:
		selection := normalizeSelection(cloneSelection(doc.Selection))
		if selection == nil {
			return fmt.Errorf("layer %q cannot create a mask without an active selection", layer.Name())
		}
		layer.SetMask(newLayerMaskFromSelection(selection))
		doc.touchModifiedAtLayer(layer)
		return nil
	default:
		return fmt.Errorf("unsupported layer mask mode %q", mode)
	}
	layer.SetMask(newFilledLayerMask(doc.Width, doc.Height, fill))
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) DeleteLayerMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.Mask() == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	layer.SetMask(nil)
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) ApplyLayerMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	// Applying a mask bakes it into the raster — a destructive pixel edit.
	if err := ensureLayerEditable(layer, editLayerPixels); err != nil {
		return err
	}
	mask = effectiveRasterMask(mask)
	switch typed := layer.(type) {
	case *PixelLayer:
		if err := applyMaskToLayerRaster(typed.Bounds, typed.Pixels, mask); err != nil {
			return err
		}
	case *TextLayer:
		if err := applyMaskToLayerRaster(typed.Bounds, typed.CachedRaster, mask); err != nil {
			return err
		}
	case *VectorLayer:
		if err := applyMaskToLayerRaster(typed.Bounds, typed.CachedRaster, mask); err != nil {
			return err
		}
	case *GroupLayer:
		return fmt.Errorf("layer %q cannot apply a group mask without rasterizing the group", layer.Name())
	case *AdjustmentLayer:
		return fmt.Errorf("layer %q cannot apply a mask without raster content", layer.Name())
	default:
		return fmt.Errorf("unsupported layer type %T", layer)
	}
	layer.SetMask(nil)
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) InvertLayerMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	inverted := cloneLayerMask(mask)
	for index := range inverted.Data {
		inverted.Data[index] = 255 - inverted.Data[index]
	}
	layer.SetMask(inverted)
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) SetLayerMaskEnabled(layerID string, enabled bool) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	updated := cloneLayerMask(mask)
	updated.Enabled = enabled
	layer.SetMask(updated)
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) SetLayerMaskProperties(layerID string, density, feather *int) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	mask := layer.Mask()
	if mask == nil {
		return fmt.Errorf("layer %q has no mask", layer.Name())
	}
	updated := cloneLayerMask(mask)
	updated.SetProperties(density, feather)
	layer.SetMask(updated)
	doc.touchModifiedAtLayer(layer)
	return nil
}

// AddVectorMask attaches a vector mask to the layer. Without a seed the mask
// is an EMPTY placeholder path (no anchor points), which renders as
// "reveal all" — byte-identical to having no vector mask. When fromActivePath
// is true and the document has a valid active stored path, the mask is seeded
// with a deep clone of that path instead.
func (doc *Document) AddVectorMask(layerID string, fromActivePath bool) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.VectorMask() != nil {
		return fmt.Errorf("layer %q already has a vector mask", layer.Name())
	}
	if fromActivePath && doc.ActivePathIdx >= 0 && doc.ActivePathIdx < len(doc.Paths) {
		// SetVectorMask deep-clones, so the mask does not alias the stored path.
		layer.SetVectorMask(&doc.Paths[doc.ActivePathIdx].Path)
	} else {
		// Reveal-all placeholder (regression-pinned to render byte-identical
		// to no mask); the path is edited later via SetVectorMaskPath.
		layer.SetVectorMask(&Path{Subpaths: []Subpath{{Closed: true}}})
	}
	doc.touchModifiedAtLayer(layer)
	return nil
}

// SetVectorMaskPath replaces the path of an EXISTING vector mask. The layer
// must already carry a vector mask (use AddVectorMask first).
func (doc *Document) SetVectorMaskPath(layerID string, path *Path) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.VectorMask() == nil {
		return fmt.Errorf("layer %q has no vector mask", layer.Name())
	}
	if path == nil {
		return fmt.Errorf("vector mask path is required")
	}
	layer.SetVectorMask(path) // SetVectorMask deep-clones the payload path
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) DeleteVectorMask(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if layer.VectorMask() == nil {
		return fmt.Errorf("layer %q has no vector mask", layer.Name())
	}
	layer.SetVectorMask(nil)
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) SetLayerClipToBelow(layerID string, clipToBelow bool) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || parent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if clipToBelow {
		children := parent.Children()
		if clippingBaseIndex(children, index) < 0 {
			return fmt.Errorf("layer %q cannot clip without a base layer below it", layer.Name())
		}
	}
	layer.SetClipToBelow(clipToBelow)
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	if clipToBelow {
		children := parent.Children()
		baseIndex := clippingBaseIndex(children, index)
		if baseIndex >= 0 {
			doc.touchModifiedAtLayers(layer, children[baseIndex])
			return nil
		}
	}
	doc.touchModifiedAtLayer(layer)
	return nil
}

func (doc *Document) FlattenLayer(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || parent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	flattened, err := doc.rasterizeAsPixelLayer(layer, layer.Name())
	if err != nil {
		return err
	}
	replaceChild(parent, index, flattened)
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	doc.ActiveLayerID = flattened.ID()
	doc.touchModifiedAtLayers(layer, flattened)
	return nil
}

func (doc *Document) MergeDown(layerID string) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok || parent == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	if index == 0 {
		return fmt.Errorf("layer %q has no layer below it", layerID)
	}
	children := parent.Children()
	below := children[index-1]
	merged, err := doc.mergeNodesToPixelLayer(below, layer, fmt.Sprintf("%s + %s", below.Name(), layer.Name()))
	if err != nil {
		return err
	}
	children[index-1] = merged
	children = append(children[:index], children[index+1:]...)
	parent.SetChildren(children)
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	doc.ActiveLayerID = merged.ID()
	doc.touchModifiedAtLayers(below, layer, merged)
	return nil
}

func (doc *Document) MergeVisible() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	root := doc.ensureLayerRoot()
	children := root.Children()

	hasVisible := false
	for _, child := range children {
		if child.Visible() {
			hasVisible = true
			break
		}
	}
	if !hasVisible {
		return fmt.Errorf("no visible layers to merge")
	}

	// Render the full document composite so that all nested visible content at every
	// level of the layer hierarchy is included, not just root-level layers.
	surface, err := doc.renderCompositeSurfaceChecked()
	if err != nil {
		return fmt.Errorf("render composite surface: %w", err)
	}
	if surface == nil {
		return fmt.Errorf("failed to render composite surface")
	}
	merged := NewPixelLayer("Merged Visible", LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, surface)

	// Preserve root-level hidden layers at their original stack positions; the
	// merged composite (which represents all visible content, including groups
	// and their nested children) takes the slot of the BOTTOMMOST visible
	// layer — Photoshop's Merge Visible collapses into the Background / lowest
	// visible layer, so hidden layers survive above the merged result (same
	// direction as Merge Down, which lands in the lower layer's slot).
	bottomVisible := -1
	for index, child := range children {
		if child.Visible() {
			bottomVisible = index
			break
		}
	}
	next := make([]LayerNode, 0, len(children))
	for index, child := range children {
		switch {
		case index == bottomVisible:
			next = append(next, merged)
		case !child.Visible():
			next = append(next, child)
		}
	}
	root.SetChildren(next)
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	doc.ActiveLayerID = merged.ID()
	doc.touchModifiedAt()
	return nil
}

// FlattenImage merges all visible layers into a single "Background" pixel layer,
// discarding hidden layers. This is the standard Photoshop "Flatten Image" behaviour.
func (doc *Document) FlattenImage() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	root := doc.ensureLayerRoot()
	children := root.Children()

	hasVisible := false
	for _, child := range children {
		if child.Visible() {
			hasVisible = true
			break
		}
	}
	if !hasVisible {
		return fmt.Errorf("no visible layers to flatten")
	}

	surface, err := doc.renderCompositeSurfaceChecked()
	if err != nil {
		return fmt.Errorf("render composite surface: %w", err)
	}
	if surface == nil {
		return fmt.Errorf("failed to render composite surface")
	}
	flattened := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, surface)
	root.SetChildren([]LayerNode{flattened})
	doc.normalizeClippingState()
	doc.invalidateLayerSolo(true)
	doc.ActiveLayerID = flattened.ID()
	doc.touchModifiedAt()
	return nil
}

// generateAllThumbnails builds thumbnailSize×thumbnailSize previews for every
// layer in the active document. The result is a map of layer ID → ThumbnailEntry.
func (doc *Document) generateAllThumbnails(thumbW, thumbH int) (map[string]ThumbnailEntry, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	all := flattenLayerTree(doc.ensureLayerRoot().Children())
	result := make(map[string]ThumbnailEntry, len(all))
	for _, layer := range all {
		layerRGBA, maskRGBA := doc.generateLayerThumbnail(layer, thumbW, thumbH)
		entry := ThumbnailEntry{
			LayerRGBA: base64.StdEncoding.EncodeToString(layerRGBA),
		}
		if len(maskRGBA) > 0 {
			entry.MaskRGBA = base64.StdEncoding.EncodeToString(maskRGBA)
		}
		result[layer.ID()] = entry
	}
	return result, nil
}

// generateLayerThumbnail returns RGBA pixel data for the layer thumbnail and (if
// a mask is present) the mask thumbnail. Both buffers are thumbW×thumbH×4 bytes.
func (doc *Document) generateLayerThumbnail(layer LayerNode, thumbW, thumbH int) (layerRGBA, maskRGBA []byte) {
	switch typed := layer.(type) {
	case *PixelLayer:
		if len(typed.Pixels) > 0 && typed.Bounds.W > 0 && typed.Bounds.H > 0 {
			layerRGBA = scaleRGBA(typed.Pixels, typed.Bounds.W, typed.Bounds.H, thumbW, thumbH)
		}
	case *TextLayer:
		if len(typed.CachedRaster) > 0 && typed.Bounds.W > 0 && typed.Bounds.H > 0 {
			layerRGBA = scaleRGBA(typed.CachedRaster, typed.Bounds.W, typed.Bounds.H, thumbW, thumbH)
		}
	case *VectorLayer:
		if len(typed.CachedRaster) > 0 && typed.Bounds.W > 0 && typed.Bounds.H > 0 {
			layerRGBA = scaleRGBA(typed.CachedRaster, typed.Bounds.W, typed.Bounds.H, thumbW, thumbH)
		}
	case *GroupLayer:
		if len(typed.Children()) > 0 {
			buf, err := doc.renderLayerToSurface(typed)
			if err == nil {
				layerRGBA = scaleRGBA(buf, doc.Width, doc.Height, thumbW, thumbH)
			}
		}
	}

	if mask := layer.Mask(); mask != nil && len(mask.Data) > 0 && mask.Width > 0 && mask.Height > 0 {
		maskRGBA = scaleGrayToRGBA(mask.Data, mask.Width, mask.Height, thumbW, thumbH)
	}
	return layerRGBA, maskRGBA
}

// flattenLayerTree returns all layers in depth-first order, recursing into groups.
func flattenLayerTree(layers []LayerNode) []LayerNode {
	result := make([]LayerNode, 0, len(layers))
	for _, layer := range layers {
		result = append(result, layer)
		if children := layer.Children(); len(children) > 0 {
			result = append(result, flattenLayerTree(children)...)
		}
	}
	return result
}

// scaleRGBA downsamples an RGBA pixel buffer using nearest-neighbour sampling.
// The source buffer must be srcW×srcH×4 bytes; the result is dstW×dstH×4 bytes.
func scaleRGBA(src []byte, srcW, srcH, dstW, dstH int) []byte {
	dst := make([]byte, dstW*dstH*4)
	for y := range dstH {
		for x := range dstW {
			sx := x * srcW / dstW
			sy := y * srcH / dstH
			si := (sy*srcW + sx) * 4
			di := (y*dstW + x) * 4
			if si+4 <= len(src) {
				copy(dst[di:di+4], src[si:si+4])
			}
		}
	}
	return dst
}

// scaleGrayToRGBA downsamples a grayscale mask buffer and converts it to RGBA.
// The source buffer must be srcW×srcH×1 bytes (one byte per pixel, 0=black, 255=white).
func scaleGrayToRGBA(src []byte, srcW, srcH, dstW, dstH int) []byte {
	dst := make([]byte, dstW*dstH*4)
	for y := range dstH {
		for x := range dstW {
			sx := x * srcW / dstW
			sy := y * srcH / dstH
			si := sy*srcW + sx
			di := (y*dstW + x) * 4
			var gray byte
			if si < len(src) {
				gray = src[si]
			}
			dst[di] = gray
			dst[di+1] = gray
			dst[di+2] = gray
			dst[di+3] = 255
		}
	}
	return dst
}

func (doc *Document) touchModifiedAt() {
	doc.touchModifiedAtRect(doc.fullDocumentDirtyRect())
}

func (doc *Document) touchModifiedAtRect(rect DirtyRect) {
	doc.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	doc.ContentVersion = atomic.AddInt64(&nextDocVersion, 1)
	doc.markDirtyCompositeRect(rect)
}

func (doc *Document) bumpContentVersionRect(rect DirtyRect) {
	doc.ContentVersion = atomic.AddInt64(&nextDocVersion, 1)
	doc.markDirtyCompositeRect(rect)
}

func (doc *Document) fullDocumentDirtyRect() DirtyRect {
	if doc == nil {
		return DirtyRect{}
	}
	return docpkg.FullDocumentDirtyRect(doc.Width, doc.Height)
}

func (doc *Document) markDirtyCompositeRect(rect DirtyRect) {
	if doc == nil {
		return
	}
	doc.dirtyComposite, doc.hasDirtyComposite = docpkg.MarkDirtyCompositeRect(doc.dirtyComposite, doc.hasDirtyComposite, rect, doc.Width, doc.Height)
}

func (doc *Document) currentDirtyCompositeRect() *DirtyRect {
	if doc == nil {
		return nil
	}
	rect, ok := docpkg.CurrentDirtyCompositeRect(doc.dirtyComposite, doc.hasDirtyComposite)
	if !ok {
		return nil
	}
	return &rect
}

func (doc *Document) clearDirtyCompositeRect() {
	if doc == nil {
		return
	}
	doc.dirtyComposite = DirtyRect{}
	doc.hasDirtyComposite = false
	// From here on, newly marked dirty rects describe the delta relative to
	// the document content at this version (see dirtyCompositeBase).
	doc.dirtyCompositeBase = doc.ContentVersion
}

func (doc *Document) touchModifiedAtLayer(layer LayerNode) {
	if rect, ok := doc.layerCompositeDirtyRect(layer); ok {
		doc.touchModifiedAtRect(rect)
		return
	}
	doc.touchModifiedAt()
}

func (doc *Document) touchModifiedAtLayers(layers ...LayerNode) {
	combined, hasCombined := docpkg.CombineLayerCompositeDirtyRects(layers, doc.Width, doc.Height, hasAnyEnabledLayerStyleEntry)
	if hasCombined {
		doc.touchModifiedAtRect(combined)
		return
	}
	doc.touchModifiedAt()
}

func (doc *Document) touchModifiedAtBounds(before, after LayerBounds) {
	if rect, ok := docpkg.DirtyRectForBounds(before, after, doc.Width, doc.Height); ok {
		doc.touchModifiedAtRect(rect)
		return
	}
	doc.touchModifiedAt()
}

func (doc *Document) layerCompositeDirtyRect(layer LayerNode) (DirtyRect, bool) {
	if doc == nil {
		return DirtyRect{}, false
	}
	return docpkg.LayerCompositeDirtyRect(layer, doc.Width, doc.Height, hasAnyEnabledLayerStyleEntry)
}

func (doc *Document) SetArtboard(layerID string, bounds LayerBounds, background *[4]uint8) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if err := docpkg.SetArtboard(doc.ensureLayerRoot(), layerID, bounds, background); err != nil {
		return err
	}
	doc.touchModifiedAt()
	return nil
}

func (doc *Document) newLayerFromPayload(payload AddLayerPayload) (LayerNode, error) {
	switch payload.LayerType {
	case LayerTypePixel:
		if payload.Bounds.W <= 0 || payload.Bounds.H <= 0 {
			return nil, fmt.Errorf("pixel layer requires valid bounds, got %dx%d", payload.Bounds.W, payload.Bounds.H)
		}
		pixels := payload.Pixels
		if len(pixels) == 0 {
			pixels = make([]byte, payload.Bounds.W*payload.Bounds.H*4)
		}
		return NewPixelLayer(payload.Name, payload.Bounds, pixels), nil
	case LayerTypeGroup:
		group := NewGroupLayer(payload.Name)
		group.Isolated = payload.Isolated
		if payload.IsArtboard {
			bounds := LayerBounds{}
			if payload.ArtboardBounds != nil {
				bounds = *payload.ArtboardBounds
			}
			if bounds.W <= 0 || bounds.H <= 0 {
				return nil, fmt.Errorf("artboard group requires valid bounds, got %dx%d", bounds.W, bounds.H)
			}
			background := docpkg.DefaultArtboardBackground()
			if payload.ArtboardBackground != nil {
				background = *payload.ArtboardBackground
			}
			group.Artboard = &ArtboardData{
				Bounds:     bounds,
				Background: background,
			}
		}
		return group, nil
	case LayerTypeAdjustment:
		if payload.AdjustmentKind == "" {
			return nil, fmt.Errorf("adjustment layer requires adjustmentKind")
		}
		return NewAdjustmentLayer(payload.Name, payload.AdjustmentKind, payload.Params), nil
	case LayerTypeText:
		layer := NewTextLayer(payload.Name, payload.Bounds, payload.Text, payload.CachedRaster)
		// Payloads carry no anchor; the payload bounds origin is the pen origin.
		layer.AnchorX = float64(payload.Bounds.X)
		layer.AnchorY = float64(payload.Bounds.Y)
		layer.AnchorSet = true
		if payload.FontFamily != "" {
			layer.FontFamily = payload.FontFamily
		}
		if payload.FontSize > 0 {
			layer.FontSize = payload.FontSize
		}
		if payload.Color != [4]uint8{} {
			layer.Color = payload.Color
		}
		// Auto-rasterize when created via payload with text but no pre-baked raster.
		if len(layer.CachedRaster) == 0 {
			if err := rasterizeTextLayer(layer); err != nil {
				return nil, err
			}
		}
		return layer, nil
	case LayerTypeVector:
		layer := NewVectorLayer(payload.Name, payload.Bounds, payload.Path, payload.CachedRaster)
		if payload.FillColor != [4]uint8{} {
			layer.FillColor = payload.FillColor
		}
		if payload.StrokeColor != [4]uint8{} {
			layer.StrokeColor = payload.StrokeColor
		}
		if payload.StrokeWidth > 0 {
			layer.StrokeWidth = payload.StrokeWidth
		}
		return layer, nil
	default:
		return nil, fmt.Errorf("unsupported layer type %q", payload.LayerType)
	}
}

func (doc *Document) rasterizeAsPixelLayer(layer LayerNode, name string) (*PixelLayer, error) {
	if _, ok := layer.(*AdjustmentLayer); ok {
		return nil, fmt.Errorf("layer %q cannot be flattened without raster content", layer.Name())
	}
	buffer, err := doc.renderLayerToSurface(layer)
	if err != nil {
		return nil, err
	}
	pixelLayer := NewPixelLayer(name, LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, buffer)
	// Opacity, fill opacity, and the layer mask are already baked into the
	// rendered surface by renderLayerToSurface — the same convention used by
	// MergeDown/MergeVisible. Copying them onto the result would apply them a
	// second time at composite (a 50% layer would render at 25%), so the
	// flattened layer keeps the defaults (1.0). The blend mode is NOT baked
	// (there is no backdrop when rendering the layer alone), so preserve it.
	pixelLayer.SetBlendMode(layer.BlendMode())
	pixelLayer.SetVisible(layer.Visible())
	pixelLayer.SetLockMode(layer.LockMode())
	return pixelLayer, nil
}

func (doc *Document) mergeNodesToPixelLayer(bottom, top LayerNode, name string) (*PixelLayer, error) {
	buffer, err := doc.renderLayersToSurface([]LayerNode{bottom, top})
	if err != nil {
		return nil, err
	}
	return NewPixelLayer(name, LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, buffer), nil
}

func (doc *Document) renderLayerToSurface(layer LayerNode) ([]byte, error) {
	return doc.renderLayerToSurfaceWithOptions(layer, false)
}

func (doc *Document) renderLayerToSurfaceWithOptions(layer LayerNode, allowAdjustmentCache bool) ([]byte, error) {
	buffer := make([]byte, doc.Width*doc.Height*4)
	clipAlpha, err := doc.clippingBaseSurfaceForLayer(layer)
	if err != nil {
		return nil, err
	}
	if err := doc.compositeLayerOntoWithClipOptions(buffer, layer, clipAlpha, allowAdjustmentCache, nil); err != nil {
		return nil, err
	}
	return buffer, nil
}

func (doc *Document) renderLayersToSurface(layers []LayerNode) ([]byte, error) {
	return doc.renderLayersToSurfaceWithOptions(layers, false)
}

func (doc *Document) renderLayersToSurfaceWithOptions(layers []LayerNode, allowAdjustmentCache bool) ([]byte, error) {
	buffer := make([]byte, doc.Width*doc.Height*4)
	if err := doc.compositeLayerStackOntoWithOptions(buffer, layers, nil, allowAdjustmentCache, nil); err != nil {
		return nil, err
	}
	return buffer, nil
}

// compositeLayerOnto is retained for test coverage and delegates to the
// clip-aware compositor used by production code.
//
//nolint:unused
func (doc *Document) compositeLayerOnto(dest []byte, layer LayerNode) error {
	return doc.compositeLayerOntoWithClipOptions(dest, layer, nil, false, nil)
}

// compositeLayerOntoWithClipOptions composites a single layer onto dest.
//
// clip, when non-nil, restricts every per-pixel write to the given doc-space
// rectangle (the incremental dirty-rect recomposite path, PLAN.md S.4). A nil
// clip preserves the historical full-surface behavior. Layer-style effect
// surfaces are still rendered unclipped — their content is backdrop-independent
// and spatially extended (drop shadows, glows), so only their final composite
// onto dest is clipped, which keeps the result byte-identical to a full
// recomposite inside the clip rect.
func (doc *Document) compositeLayerOntoWithClipOptions(dest []byte, layer LayerNode, clipAlpha []byte, allowAdjustmentCache bool, clip *DirtyRect) error {
	if layer == nil || !layer.Visible() {
		return nil
	}
	if err := ensureRasterizableLayer(layer); err != nil {
		return err
	}
	switch typed := layer.(type) {
	case *PixelLayer:
		if !hasSupportedEnabledLayerStyleStack(typed.StyleStack()) {
			return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.Pixels, typed.BlendMode(), clampUnit(effectiveLayerOpacity(typed)*effectiveContentOpacity(typed)), doc.effectiveLayerMask(typed), clipAlpha, typed.BlendIf(), clip)
		}
		surface, err := doc.renderStyledLayerSurface(typed, clipAlpha)
		if err != nil {
			return err
		}
		compositeDocumentSurfaceClipped(dest, surface, doc.Width, typed.BlendMode(), effectiveLayerOpacity(typed), typed.BlendIf(), clip)
		return nil
	case *TextLayer:
		if !hasSupportedEnabledLayerStyleStack(typed.StyleStack()) {
			return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.CachedRaster, typed.BlendMode(), clampUnit(effectiveLayerOpacity(typed)*effectiveContentOpacity(typed)), doc.effectiveLayerMask(typed), clipAlpha, typed.BlendIf(), clip)
		}
		surface, err := doc.renderStyledLayerSurface(typed, clipAlpha)
		if err != nil {
			return err
		}
		compositeDocumentSurfaceClipped(dest, surface, doc.Width, typed.BlendMode(), effectiveLayerOpacity(typed), typed.BlendIf(), clip)
		return nil
	case *VectorLayer:
		if !hasSupportedEnabledLayerStyleStack(typed.StyleStack()) {
			return compositeRasterIntoDocument(dest, doc.Width, doc.Height, typed.Bounds, typed.CachedRaster, typed.BlendMode(), clampUnit(effectiveLayerOpacity(typed)*effectiveContentOpacity(typed)), doc.effectiveLayerMask(typed), clipAlpha, typed.BlendIf(), clip)
		}
		surface, err := doc.renderStyledLayerSurface(typed, clipAlpha)
		if err != nil {
			return err
		}
		compositeDocumentSurfaceClipped(dest, surface, doc.Width, typed.BlendMode(), effectiveLayerOpacity(typed), typed.BlendIf(), clip)
		return nil
	case *AdjustmentLayer:
		rect := doc.currentDirtyCompositeRect()
		if clip != nil {
			// Defensive only: the incremental gate (canRecompositeIncrementally)
			// bails on visible adjustment layers, so a clipped composite should
			// never reach here. If it does, scope the adjustment to the clip and
			// keep the cache untouched — outside the clip dest holds the FINAL
			// previous composite, not this adjustment's backdrop, so both
			// copySurfaceOutsideRect and updateAdjustmentCache would corrupt it.
			rect = clip
			allowAdjustmentCache = false
		}
		return applyAdjustmentLayerToSurface(dest, doc.Width, doc.Height, typed, clipAlpha, rect, allowAdjustmentCache)
	case *GroupLayer:
		// effectiveLayerMask folds the vector mask into the raster mask; the
		// pass-through gate uses it so an EMPTY vector mask (reveal all)
		// keeps the fast path byte-identical while a real vector mask forces
		// the isolated-surface path where it can be applied.
		effectiveMask := doc.effectiveLayerMask(typed)
		if !typed.Isolated && typed.BlendMode() == BlendModeNormal && effectiveLayerOpacity(typed) >= 1 && effectiveMask == nil {
			return doc.compositeLayerStackOntoWithOptions(dest, typed.Children(), clipAlpha, allowAdjustmentCache, clip)
		}
		// temp is transient compositing scratch: children are composited into
		// it, then blended onto dest and dropped. It never escapes this
		// function, so it is safe to draw from (and return to) surfacePool.
		temp := acquireSurface(len(dest))
		defer releaseSurface(temp)
		if err := doc.compositeLayerStackOntoWithOptions(temp, typed.Children(), nil, allowAdjustmentCache, clip); err != nil {
			return err
		}
		applyLayerMaskToSurface(temp, doc.Width, doc.Height, effectiveMask, clip)
		applyClipSurfaceToSurfaceClipped(temp, clipAlpha, doc.Width, clip)
		compositeDocumentSurfaceClipped(dest, temp, doc.Width, typed.BlendMode(), effectiveLayerOpacity(typed), typed.BlendIf(), clip)
		return nil
	default:
		return fmt.Errorf("unsupported layer type %T", layer)
	}
}

func (doc *Document) compositeLayerStackOntoWithOptions(dest []byte, layers []LayerNode, clipAlpha []byte, allowAdjustmentCache bool, clip *DirtyRect) error {
	for index := 0; index < len(layers); index++ {
		layer := layers[index]
		if layer == nil {
			continue
		}
		if layer.ClipToBelow() {
			if err := doc.compositeLayerOntoWithClipOptions(dest, layer, clipAlpha, allowAdjustmentCache, clip); err != nil {
				return err
			}
			continue
		}
		if err := doc.compositeLayerOntoWithClipOptions(dest, layer, clipAlpha, allowAdjustmentCache, clip); err != nil {
			return err
		}
		if index+1 >= len(layers) || !layers[index+1].ClipToBelow() {
			continue
		}
		baseIndex := clippingBaseRenderableIndex(layers, index)
		if baseIndex < 0 {
			continue
		}
		baseSurface, err := doc.renderClipBaseSurface(layers[baseIndex])
		if err != nil {
			return err
		}
		combinedClip := combineClipSurface(baseSurface, clipAlpha)
		for next := index + 1; next < len(layers) && layers[next].ClipToBelow(); next++ {
			if err := doc.compositeLayerOntoWithClipOptions(dest, layers[next], combinedClip, allowAdjustmentCache, clip); err != nil {
				return err
			}
			index = next
		}
	}
	return nil
}

func ensureRasterizableLayer(layer LayerNode) error {
	if !hasAnyEnabledLayerStyleEntry(layer.StyleStack()) {
		return nil
	}
	switch layer.(type) {
	case *PixelLayer, *TextLayer, *VectorLayer:
		return nil
	}
	return fmt.Errorf("layer %q cannot be merged while layer styles are not rasterized", layer.Name())
}

// effectiveLayerOpacity returns the whole-layer composite opacity, which applies
// to the complete layer surface including any layer effects (Phase 5). Use this
// when compositing groups or checking pass-through conditions.
func effectiveLayerOpacity(layer LayerNode) float64 {
	return layer.Opacity()
}

// effectiveContentOpacity returns the opacity used when compositing pixel content.
// Fill opacity reduces only the layer's own pixels, not its effects (drop shadows,
// strokes, etc. added in Phase 5). Use this for PixelLayer, TextLayer, VectorLayer.
func effectiveContentOpacity(layer LayerNode) float64 {
	return clampUnit(layer.FillOpacity())
}

// compositeRasterIntoDocument composites a BOUNDS-LOCAL raster into the
// doc-sized dest surface: src must be exactly bounds.W × bounds.H × 4 bytes and
// its pixel (0,0) is placed at document pixel (bounds.X, bounds.Y). This is the
// canonical geometry contract for PixelLayer.Pixels and the CachedRaster fields
// of TextLayer/VectorLayer (see internal/model/layers.go).
//
// clip, when non-nil, restricts writes to the given doc-space rectangle by
// intersecting the iteration range; a nil clip composites the full bounds.
func compositeRasterIntoDocument(dest []byte, docW, docH int, bounds LayerBounds, src []byte, blendMode BlendMode, opacity float64, mask *LayerMask, clipAlpha []byte, blendIf *BlendIfConfig, clip *DirtyRect) error {
	if bounds.W <= 0 || bounds.H <= 0 || len(src) == 0 || opacity <= 0 {
		return nil
	}
	expectedLen := bounds.W * bounds.H * 4
	if len(src) != expectedLen {
		return fmt.Errorf("raster length %d does not match bounds %dx%d", len(src), bounds.W, bounds.H)
	}
	yStart, yEnd := 0, bounds.H
	xStart, xEnd := 0, bounds.W
	if clip != nil {
		yStart = maxInt(yStart, clip.Y-bounds.Y)
		yEnd = minInt(yEnd, clip.Y+clip.H-bounds.Y)
		xStart = maxInt(xStart, clip.X-bounds.X)
		xEnd = minInt(xEnd, clip.X+clip.W-bounds.X)
		if yStart >= yEnd || xStart >= xEnd {
			return nil
		}
	}
	identityBlendIf := blendIfIsIdentity(blendIf)
	for y := yStart; y < yEnd; y++ {
		docY := bounds.Y + y
		if docY < 0 || docY >= docH {
			continue
		}
		for x := xStart; x < xEnd; x++ {
			docX := bounds.X + x
			if docX < 0 || docX >= docW {
				continue
			}
			srcIndex := (y*bounds.W + x) * 4
			maskAlpha := layerMaskAlphaAt(mask, docX, docY)
			maskAlpha = scaleMaskedAlpha(maskAlpha, clipSurfaceAlphaAt(clipAlpha, docW, docX, docY))
			if maskAlpha == 0 {
				continue
			}
			destIndex := (docY*docW + docX) * 4
			srcPixel := src[srcIndex : srcIndex+4]
			pixelOpacity := opacity
			var origDest [4]uint8
			if !identityBlendIf {
				srcRGBA := [4]uint8{srcPixel[0], srcPixel[1], srcPixel[2], srcPixel[3]}
				copy(origDest[:], dest[destIndex:destIndex+4])
				pixelOpacity *= blendIfAlpha(srcRGBA, origDest, blendIf)
				if pixelOpacity <= 0 {
					continue
				}
			}
			if maskAlpha == 255 {
				compositePixelWithBlend(dest[destIndex:destIndex+4], srcPixel, blendMode, pixelOpacity, pixelNoiseSeed(docX, docY))
			} else {
				var masked [4]byte
				copy(masked[:], srcPixel)
				masked[3] = scaleMaskedAlpha(srcPixel[3], maskAlpha)
				if masked[3] == 0 {
					continue
				}
				compositePixelWithBlend(dest[destIndex:destIndex+4], masked[:], blendMode, pixelOpacity, pixelNoiseSeed(docX, docY))
			}
			if !identityBlendIf {
				var after [4]uint8
				copy(after[:], dest[destIndex:destIndex+4])
				applyChannelsMask(&origDest, &after, blendIf)
				copy(dest[destIndex:destIndex+4], after[:])
			}
		}
	}
	return nil
}

// applyLayerMaskToSurface multiplies the surface alpha by the layer mask.
// clip, when non-nil, restricts the pass to the given doc-space rectangle.
func applyLayerMaskToSurface(surface []byte, docW, docH int, mask *LayerMask, clip *DirtyRect) {
	if len(surface) == 0 || docW <= 0 || docH <= 0 || mask == nil || !mask.Enabled {
		return
	}
	yStart, yEnd := 0, docH
	xStart, xEnd := 0, docW
	if clip != nil {
		yStart = maxInt(yStart, clip.Y)
		yEnd = minInt(yEnd, clip.Y+clip.H)
		xStart = maxInt(xStart, clip.X)
		xEnd = minInt(xEnd, clip.X+clip.W)
	}
	for docY := yStart; docY < yEnd; docY++ {
		for docX := xStart; docX < xEnd; docX++ {
			maskAlpha := layerMaskAlphaAt(mask, docX, docY)
			if maskAlpha == 255 {
				continue
			}
			index := (docY*docW + docX) * 4
			surface[index+3] = scaleMaskedAlpha(surface[index+3], maskAlpha)
		}
	}
}

func applyClipSurfaceToSurface(surface, clipAlpha []byte) {
	if len(surface) == 0 || len(clipAlpha) != len(surface) {
		return
	}
	for offset := 0; offset < len(surface); offset += 4 {
		surface[offset+3] = scaleMaskedAlpha(surface[offset+3], clipAlpha[offset+3])
	}
}

// applyClipSurfaceToSurfaceClipped is applyClipSurfaceToSurface restricted to a
// doc-space rectangle. A nil clip delegates to the full-surface variant.
func applyClipSurfaceToSurfaceClipped(surface, clipAlpha []byte, docW int, clip *DirtyRect) {
	if clip == nil {
		applyClipSurfaceToSurface(surface, clipAlpha)
		return
	}
	if len(surface) == 0 || len(clipAlpha) != len(surface) || docW <= 0 {
		return
	}
	for y := clip.Y; y < clip.Y+clip.H; y++ {
		rowStart := (y*docW + clip.X) * 4
		rowEnd := rowStart + clip.W*4
		if rowStart < 0 || rowEnd > len(surface) {
			continue
		}
		for offset := rowStart; offset < rowEnd; offset += 4 {
			surface[offset+3] = scaleMaskedAlpha(surface[offset+3], clipAlpha[offset+3])
		}
	}
}

func layerMaskAlphaAt(mask *LayerMask, docX, docY int) uint8 {
	if mask == nil || !mask.Enabled || mask.Width <= 0 || mask.Height <= 0 {
		return 255
	}
	expectedLen := mask.Width * mask.Height
	if len(mask.Data) < expectedLen {
		return 255
	}
	if docX < 0 || docX >= mask.Width || docY < 0 || docY >= mask.Height {
		return 0
	}
	return mask.Data[docY*mask.Width+docX]
}

func scaleMaskedAlpha(alpha, maskAlpha uint8) uint8 {
	return uint8((uint16(alpha)*uint16(maskAlpha) + 127) / 255)
}

func clipSurfaceAlphaAt(surface []byte, width, x, y int) uint8 {
	if len(surface) == 0 || width <= 0 || x < 0 || y < 0 {
		return 255
	}
	index := (y*width + x) * 4
	if index < 0 || index+3 >= len(surface) {
		return 0
	}
	return surface[index+3]
}

func combineClipSurface(baseSurface, clipAlpha []byte) []byte {
	if len(baseSurface) == 0 {
		return clipAlpha
	}
	if len(clipAlpha) == 0 {
		return baseSurface
	}
	combined := append([]byte(nil), baseSurface...)
	applyClipSurfaceToSurface(combined, clipAlpha)
	return combined
}

func newFilledLayerMask(width, height int, fill byte) *LayerMask {
	if width <= 0 || height <= 0 {
		mask := &LayerMask{Enabled: true, Width: width, Height: height}
		density := 100
		mask.SetProperties(&density, nil)
		return mask
	}
	data := make([]byte, width*height)
	if fill != 0 {
		for index := range data {
			data[index] = fill
		}
	}
	mask := &LayerMask{Enabled: true, Width: width, Height: height, Data: data}
	density := 100
	mask.SetProperties(&density, nil)
	return mask
}

func applyMaskToLayerRaster(bounds LayerBounds, raster []byte, mask *LayerMask) error {
	if bounds.W <= 0 || bounds.H <= 0 || len(raster) == 0 || mask == nil {
		return nil
	}
	expectedLen := bounds.W * bounds.H * 4
	if len(raster) != expectedLen {
		return fmt.Errorf("raster length %d does not match bounds %dx%d", len(raster), bounds.W, bounds.H)
	}
	for y := 0; y < bounds.H; y++ {
		docY := bounds.Y + y
		for x := 0; x < bounds.W; x++ {
			docX := bounds.X + x
			alpha := layerMaskDataAlphaAt(mask, docX, docY)
			if alpha == 255 {
				continue
			}
			pixelIndex := (y*bounds.W + x) * 4
			raster[pixelIndex+3] = scaleMaskedAlpha(raster[pixelIndex+3], alpha)
		}
	}
	return nil
}

func layerMaskDataAlphaAt(mask *LayerMask, docX, docY int) uint8 {
	if mask == nil || mask.Width <= 0 || mask.Height <= 0 {
		return 255
	}
	expectedLen := mask.Width * mask.Height
	if len(mask.Data) < expectedLen {
		return 255
	}
	if docX < 0 || docX >= mask.Width || docY < 0 || docY >= mask.Height {
		return 0
	}
	return mask.Data[docY*mask.Width+docX]
}

func (doc *Document) normalizeClippingState() {
	if doc == nil {
		return
	}
	normalizeGroupClipping(doc.ensureLayerRoot())
}

func normalizeGroupClipping(group *GroupLayer) {
	if group == nil {
		return
	}
	children := group.Children()
	for _, child := range children {
		child.SetClippingBase(false)
		if nested, ok := child.(*GroupLayer); ok {
			normalizeGroupClipping(nested)
		}
	}
	for index, child := range children {
		baseIndex := clippingBaseIndex(children, index)
		if !child.ClipToBelow() {
			continue
		}
		if baseIndex < 0 {
			child.SetClipToBelow(false)
			continue
		}
		children[baseIndex].SetClippingBase(true)
	}
	group.SetChildren(children)
}

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
	identity := blendIfIsIdentity(blendIf)
	if clip == nil {
		compositeDocumentSurfaceSpan(dest, src, docW, 0, len(dest), blendMode, opacity, identity, blendIf)
		return
	}
	for y := clip.Y; y < clip.Y+clip.H; y++ {
		rowStart := (y*docW + clip.X) * 4
		rowEnd := rowStart + clip.W*4
		if rowStart < 0 || rowEnd > len(dest) {
			continue
		}
		compositeDocumentSurfaceSpan(dest, src, docW, rowStart, rowEnd, blendMode, opacity, identity, blendIf)
	}
}

// compositeDocumentSurfaceSpan composites the byte range [from, to) of two
// document-sized surfaces. The dissolve noise seed follows the
// document-coordinate convention pixelNoiseSeed(docX, docY); every caller
// threads a positive docW (compositeDocumentSurfaceClipped guards it).
func compositeDocumentSurfaceSpan(dest, src []byte, docW, from, to int, blendMode BlendMode, opacity float64, identity bool, blendIf *BlendIfConfig) {
	for offset := from; offset < to; offset += 4 {
		pixelOpacity := opacity
		var origDest [4]uint8
		if !identity {
			srcRGBA := [4]uint8{src[offset], src[offset+1], src[offset+2], src[offset+3]}
			copy(origDest[:], dest[offset:offset+4])
			pixelOpacity *= blendIfAlpha(srcRGBA, origDest, blendIf)
			if pixelOpacity <= 0 {
				continue
			}
		}
		var noiseSeed uint32
		if blendMode == BlendModeDissolve {
			pixelIndex := offset / 4
			noiseSeed = pixelNoiseSeed(pixelIndex%docW, pixelIndex/docW)
		}
		compositePixelWithBlend(dest[offset:offset+4], src[offset:offset+4], blendMode, pixelOpacity, noiseSeed)
		if !identity {
			var after [4]uint8
			copy(after[:], dest[offset:offset+4])
			applyChannelsMask(&origDest, &after, blendIf)
			copy(dest[offset:offset+4], after[:])
		}
	}
}
