package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// FilterCategory groups filters in the UI menu.
type FilterCategory string

const (
	FilterCategoryBlur    FilterCategory = "blur"
	FilterCategorySharpen FilterCategory = "sharpen"
	FilterCategoryNoise   FilterCategory = "noise"
	FilterCategoryDistort FilterCategory = "distort"
	FilterCategoryStylize FilterCategory = "stylize"
	FilterCategoryRender  FilterCategory = "render"
	FilterCategoryOther   FilterCategory = "other"
)

// FilterDef describes a registered filter for UI discovery.
type FilterDef struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Category  FilterCategory `json:"category"`
	HasDialog bool           `json:"hasDialog"`
}

// FilterFunc is the signature for a destructive pixel filter.
// pixels is row-major RGBA8 (len = w*h*4).
// selMask is single-channel alpha (len = w*h) or nil for full layer.
type FilterFunc func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error

// registeredFilter pairs a filter definition with its implementation.
type registeredFilter struct {
	Def FilterDef
	Fn  FilterFunc
}

var filterRegistry = struct {
	sync.RWMutex
	entries map[string]*registeredFilter
}{
	entries: make(map[string]*registeredFilter),
}

// RegisterFilter registers a filter. Passing a nil fn removes the registration.
func RegisterFilter(def FilterDef, fn FilterFunc) {
	key := normalizeFilterID(def.ID)
	if key == "" {
		return
	}

	filterRegistry.Lock()
	defer filterRegistry.Unlock()

	if fn == nil {
		delete(filterRegistry.entries, key)
		return
	}
	filterRegistry.entries[key] = &registeredFilter{Def: def, Fn: fn}
}

// lookupFilter finds a registered filter by its ID.
func lookupFilter(id string) *registeredFilter {
	key := normalizeFilterID(id)
	if key == "" {
		return nil
	}

	filterRegistry.RLock()
	defer filterRegistry.RUnlock()

	return filterRegistry.entries[key]
}

// ApplyFilter applies a registered filter to the pixel layer identified by layerID.
// The document's selection (if any) is converted into a per-pixel alpha mask
// clipped to the layer's bounds and passed to the filter function.
func (doc *Document) ApplyFilter(layerID, filterID string, params json.RawMessage) error {
	node := doc.findLayer(layerID)
	if node == nil {
		return fmt.Errorf("apply filter: layer %q not found", layerID)
	}

	pl, ok := node.(*PixelLayer)
	if !ok {
		return fmt.Errorf("apply filter: layer %q is %s, not a pixel layer", layerID, node.LayerType())
	}

	rf := lookupFilter(filterID)
	if rf == nil {
		return fmt.Errorf("apply filter: unknown filter %q", filterID)
	}

	selMask := doc.selectionMaskForLayer(pl)

	if err := rf.Fn(pl.Pixels, pl.Bounds.W, pl.Bounds.H, selMask, params); err != nil {
		return fmt.Errorf("apply filter %q: %w", filterID, err)
	}

	doc.touchModifiedAtLayer(pl)
	return nil
}

// selectionMaskForLayer extracts a single-channel alpha mask from the document's
// selection, clipped to the given pixel layer's bounds. Returns nil when there
// is no active selection (meaning the filter should affect the entire layer).
func (doc *Document) selectionMaskForLayer(pl *PixelLayer) []byte {
	if doc.Selection == nil || len(doc.Selection.Mask) == 0 {
		return nil
	}

	w, h := pl.Bounds.W, pl.Bounds.H
	mask := make([]byte, w*h)
	for y := range h {
		for x := range w {
			mask[y*w+x] = selectionAlphaAt(doc.Selection, pl.Bounds.X+x, pl.Bounds.Y+y)
		}
	}
	return mask
}

func normalizeFilterID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
