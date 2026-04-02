package engine

import (
	"encoding/json"
	"strings"
	"sync"
)

// FilterCategory groups filters in the UI menu.
type FilterCategory string

const (
	FilterCategoryBlur     FilterCategory = "blur"
	FilterCategorySharpen  FilterCategory = "sharpen"
	FilterCategoryNoise    FilterCategory = "noise"
	FilterCategoryDistort  FilterCategory = "distort"
	FilterCategoryStylize  FilterCategory = "stylize"
	FilterCategoryRender   FilterCategory = "render"
	FilterCategoryOther    FilterCategory = "other"
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

func normalizeFilterID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
