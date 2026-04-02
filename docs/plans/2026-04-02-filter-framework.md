# Phase 5.4: Filter Framework — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the backend filter infrastructure — a registry, dispatch, undo integration, and selection-awareness — so destructive pixel filters can be applied to pixel layers with full undo/redo support.

**Architecture:** Filters are destructive: they mutate `PixelLayer.Pixels` in-place inside a `snapshotCommand` (full undo). A `FilterRegistry` maps string IDs to `FilterFunc` implementations. Each filter receives the layer's RGBA pixel slice, dimensions, a selection mask (nil = full layer), and a JSON params blob. The dispatch uses command range `0x0500–0x05FF`. agg_go's `StackBlur` and `PixelReadWriter` are used for blur; per-pixel transforms (invert, brightness/contrast, posterize) use direct byte loops. A "last filter" slot stores the most recent filter ID + params for Ctrl+F replay.

**Tech Stack:** Go 1.25, agg_go (StackBlur, PixelReadWriter, color.RGBA8), engine snapshot history

---

## Task 1: Filter Registry

**Files:**
- Create: `packages/engine-wasm/internal/engine/filters.go`
- Test: `packages/engine-wasm/internal/engine/filters_test.go`

The registry maps a filter ID string to a `FilterFunc`. Mirrors the adjustment registry pattern but operates destructively on pixel data.

**Step 1: Write the failing test**

```go
// filters_test.go
package engine

import (
	"encoding/json"
	"testing"
)

func TestFilterRegistryRegisterAndLookup(t *testing.T) {
	// Register a no-op filter
	RegisterFilter(FilterDef{
		ID:       "test-noop",
		Name:     "Test Noop",
		Category: FilterCategoryOther,
	}, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		return nil
	})

	f := lookupFilter("test-noop")
	if f == nil {
		t.Fatal("expected to find registered filter")
	}
	if f.Def.Name != "Test Noop" {
		t.Errorf("got name %q, want %q", f.Def.Name, "Test Noop")
	}

	// Unknown filter returns nil.
	if lookupFilter("does-not-exist") != nil {
		t.Fatal("expected nil for unknown filter")
	}
}

func TestFilterRegistryDeregister(t *testing.T) {
	RegisterFilter(FilterDef{
		ID:       "test-temp",
		Name:     "Temp",
		Category: FilterCategoryOther,
	}, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		return nil
	})
	if lookupFilter("test-temp") == nil {
		t.Fatal("expected filter to exist")
	}

	RegisterFilter(FilterDef{ID: "test-temp"}, nil)
	if lookupFilter("test-temp") != nil {
		t.Fatal("expected filter to be removed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test -run TestFilterRegistry ./internal/engine/ -v`
Expected: FAIL — types not defined

**Step 3: Write minimal implementation**

```go
// filters.go
package engine

import (
	"encoding/json"
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

// FilterDef describes a filter's metadata.
type FilterDef struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Category  FilterCategory `json:"category"`
	HasDialog bool           `json:"hasDialog"`
}

// FilterFunc applies a destructive filter to an RGBA8 pixel buffer.
// pixels is row-major RGBA (len = w*h*4).
// selMask is a single-channel alpha mask (len = w*h) or nil for full layer.
// params is the JSON-encoded filter parameters (may be nil for immediate filters).
type FilterFunc func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error

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

// RegisterFilter registers or replaces a filter. Passing a nil fn removes the registration.
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
```

**Step 4: Run test to verify it passes**

Run: `cd packages/engine-wasm && go test -run TestFilterRegistry ./internal/engine/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add filter registry for Phase 5.4 filter framework
```

---

## Task 2: ApplyFilter on Document + selection masking

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters.go`
- Test: `packages/engine-wasm/internal/engine/filters_test.go`

Add `Document.ApplyFilter(layerID, filterID string, params json.RawMessage) error` that:
1. Finds the pixel layer
2. Builds a selection mask (from doc selection, scaled to layer bounds)
3. Calls the registered `FilterFunc`
4. Bumps `ContentVersion`

**Step 1: Write the failing test**

```go
func TestApplyFilterInvertsPixels(t *testing.T) {
	// Register a simple "invert" filter for the test.
	RegisterFilter(FilterDef{
		ID:       "test-invert",
		Name:     "Invert",
		Category: FilterCategoryOther,
	}, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		for i := 0; i < len(pixels); i += 4 {
			if selMask != nil {
				idx := i / 4
				if idx < len(selMask) && selMask[idx] == 0 {
					continue
				}
			}
			pixels[i] = 255 - pixels[i]
			pixels[i+1] = 255 - pixels[i+1]
			pixels[i+2] = 255 - pixels[i+2]
		}
		return nil
	})

	doc := &Document{Width: 2, Height: 2}
	doc.ensureLayerRoot()
	pixels := []byte{
		255, 0, 0, 255, 0, 255, 0, 255,
		0, 0, 255, 255, 128, 128, 128, 255,
	}
	layer := NewPixelLayer("bg", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, pixels)
	doc.AddLayer(layer, "", 0)
	doc.ActiveLayerID = layer.ID()

	if err := doc.ApplyFilter(layer.ID(), "test-invert", nil); err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0, 255, 255, 255, 255, 0, 255, 255,
		255, 255, 0, 255, 127, 127, 127, 255,
	}
	for i, b := range layer.Pixels {
		if b != want[i] {
			t.Errorf("pixel[%d] = %d, want %d", i, b, want[i])
		}
	}
}

func TestApplyFilterUnknownFilterReturnsError(t *testing.T) {
	doc := &Document{Width: 2, Height: 2}
	doc.ensureLayerRoot()
	layer := NewPixelLayer("bg", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, make([]byte, 16))
	doc.AddLayer(layer, "", 0)
	err := doc.ApplyFilter(layer.ID(), "nonexistent-filter", nil)
	if err == nil {
		t.Fatal("expected error for unknown filter")
	}
}

func TestApplyFilterNonPixelLayerReturnsError(t *testing.T) {
	doc := &Document{Width: 2, Height: 2}
	doc.ensureLayerRoot()
	adj := NewAdjustmentLayer("adj", "brightness-contrast", nil)
	doc.AddLayer(adj, "", 0)
	err := doc.ApplyFilter(adj.ID(), "test-invert", nil)
	if err == nil {
		t.Fatal("expected error for non-pixel layer")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test -run TestApplyFilter ./internal/engine/ -v`
Expected: FAIL — `ApplyFilter` not defined

**Step 3: Write minimal implementation**

Add to `filters.go`:

```go
import "fmt"

// ApplyFilter destructively applies a registered filter to a pixel layer.
func (doc *Document) ApplyFilter(layerID, filterID string, params json.RawMessage) error {
	layer := doc.findLayer(layerID)
	if layer == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	pl, ok := layer.(*PixelLayer)
	if !ok {
		return fmt.Errorf("filter can only be applied to pixel layers")
	}

	entry := lookupFilter(filterID)
	if entry == nil {
		return fmt.Errorf("unknown filter %q", filterID)
	}

	// Build selection mask clipped to layer bounds.
	var selMask []byte
	if doc.Selection != nil && doc.Selection.HasSelection() {
		selMask = doc.selectionMaskForLayer(pl)
	}

	if err := entry.Fn(pl.Pixels, pl.Bounds.W, pl.Bounds.H, selMask, params); err != nil {
		return fmt.Errorf("filter %q: %w", filterID, err)
	}
	doc.ContentVersion++
	doc.touchModifiedAt()
	return nil
}

// selectionMaskForLayer extracts the selection channel clipped to a pixel
// layer's bounds, producing a single-channel alpha buffer (len = w*h).
func (doc *Document) selectionMaskForLayer(pl *PixelLayer) []byte {
	if doc.Selection == nil || !doc.Selection.HasSelection() {
		return nil
	}
	lw, lh := pl.Bounds.W, pl.Bounds.H
	mask := make([]byte, lw*lh)
	for y := 0; y < lh; y++ {
		for x := 0; x < lw; x++ {
			docX := pl.Bounds.X + x
			docY := pl.Bounds.Y + y
			mask[y*lw+x] = doc.Selection.AlphaAt(docX, docY)
		}
	}
	return mask
}
```

**Step 4: Run test to verify it passes**

Run: `cd packages/engine-wasm && go test -run TestApplyFilter ./internal/engine/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add Document.ApplyFilter with selection masking
```

---

## Task 3: Command dispatch + undo integration

**Files:**
- Modify: `packages/engine-wasm/internal/engine/engine.go` (add command constants + dispatch case)
- Create: `packages/engine-wasm/internal/engine/dispatch_filter.go`
- Modify: `packages/proto/src/commands.ts` (add filter command IDs)
- Test: `packages/engine-wasm/internal/engine/filters_test.go`

**Step 1: Write the failing test**

```go
func TestDispatchApplyFilterWithUndo(t *testing.T) {
	// Register invert for this test.
	RegisterFilter(FilterDef{
		ID:       "invert",
		Name:     "Invert",
		Category: FilterCategoryOther,
	}, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		for i := 0; i < len(pixels); i += 4 {
			pixels[i] = 255 - pixels[i]
			pixels[i+1] = 255 - pixels[i+1]
			pixels[i+2] = 255 - pixels[i+2]
		}
		return nil
	})

	handle := mustCreateEngine(t, 4, 4)
	mustAddPixelLayer(t, handle, 4, 4, []byte{255, 0, 0, 255}) // red fill

	result, err := DispatchCommand(handle, commandApplyFilter, `{"filterID":"invert"}`)
	if err != nil {
		t.Fatal(err)
	}
	_ = result

	// Verify pixels are inverted (cyan).
	doc := mustActiveDoc(t, handle)
	pl := firstPixelLayer(t, doc)
	if pl.Pixels[0] != 0 || pl.Pixels[1] != 255 || pl.Pixels[2] != 255 {
		t.Errorf("expected cyan (0,255,255), got (%d,%d,%d)", pl.Pixels[0], pl.Pixels[1], pl.Pixels[2])
	}

	// Undo → back to red.
	_, err = DispatchCommand(handle, commandUndo, "")
	if err != nil {
		t.Fatal(err)
	}
	doc = mustActiveDoc(t, handle)
	pl = firstPixelLayer(t, doc)
	if pl.Pixels[0] != 255 || pl.Pixels[1] != 0 || pl.Pixels[2] != 0 {
		t.Errorf("expected red after undo, got (%d,%d,%d)", pl.Pixels[0], pl.Pixels[1], pl.Pixels[2])
	}
}
```

Note: `mustCreateEngine`, `mustAddPixelLayer`, `mustActiveDoc`, `firstPixelLayer` are test helpers. If they don't exist, create minimal versions that use `EngineInit`, `DispatchCommand`, and type-assert through the layer tree.

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test -run TestDispatchApplyFilter ./internal/engine/ -v`
Expected: FAIL — `commandApplyFilter` not defined

**Step 3: Write implementation**

Add to `engine.go` constants:

```go
// Phase 5.4: Filters
commandApplyFilter  = 0x0500
commandReapplyFilter = 0x0501
```

Create `dispatch_filter.go`:

```go
package engine

import (
	"encoding/json"
	"fmt"
)

type ApplyFilterPayload struct {
	LayerID  string          `json:"layerId,omitempty"`
	FilterID string          `json:"filterID"`
	Params   json.RawMessage `json:"params,omitempty"`
}

func (inst *instance) dispatchFilterCommand(commandID int32, payloadJSON string) (bool, error) {
	switch commandID {
	case commandApplyFilter:
		var payload ApplyFilterPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		layerID := payload.LayerID
		if layerID == "" {
			doc := inst.manager.Active()
			if doc == nil {
				return true, fmt.Errorf("no active document")
			}
			layerID = doc.ActiveLayerID
		}

		entry := lookupFilter(payload.FilterID)
		if entry == nil {
			return true, fmt.Errorf("unknown filter %q", payload.FilterID)
		}

		err := inst.executeDocCommand(entry.Def.Name, func(doc *Document) error {
			return doc.ApplyFilter(layerID, payload.FilterID, payload.Params)
		})
		if err != nil {
			return true, err
		}

		// Store as last filter for re-apply.
		inst.lastFilter = &lastFilterState{
			FilterID: payload.FilterID,
			Params:   payload.Params,
		}
		return true, nil

	case commandReapplyFilter:
		if inst.lastFilter == nil {
			return true, fmt.Errorf("no filter to re-apply")
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		entry := lookupFilter(inst.lastFilter.FilterID)
		if entry == nil {
			return true, fmt.Errorf("last filter %q no longer registered", inst.lastFilter.FilterID)
		}
		err := inst.executeDocCommand(entry.Def.Name, func(doc *Document) error {
			return doc.ApplyFilter(doc.ActiveLayerID, inst.lastFilter.FilterID, inst.lastFilter.Params)
		})
		return true, err

	default:
		return false, nil
	}
}
```

Add `lastFilter` field and struct to `state.go` (or `engine.go` near `instance`):

```go
type lastFilterState struct {
	FilterID string
	Params   json.RawMessage
}
```

Add to the `instance` struct: `lastFilter *lastFilterState`

Wire into `DispatchCommand` in `engine.go`:

```go
case commandApplyFilter, commandReapplyFilter:
	if handled, err := inst.dispatchFilterCommand(commandID, payloadJSON); handled || err != nil {
		if err != nil {
			return RenderResult{}, err
		}
	}
```

Add to `packages/proto/src/commands.ts`:

```typescript
// Phase 5.4: Filters
ApplyFilter = 0x0500,
ReapplyFilter = 0x0501,
```

**Step 4: Run test to verify it passes**

Run: `cd packages/engine-wasm && go test -run TestDispatchApplyFilter ./internal/engine/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add filter dispatch commands with undo and last-filter re-apply
```

---

## Task 4: Invert filter (immediate, no dialog)

**Files:**
- Create: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

**Step 1: Write the failing test**

```go
// filters_builtin_test.go
package engine

import "testing"

func TestFilterInvert(t *testing.T) {
	pixels := []byte{
		255, 0, 0, 255,     // red
		0, 255, 0, 128,     // green, half-alpha
		0, 0, 0, 255,       // black
		255, 255, 255, 255, // white
	}
	err := filterInvert(pixels, 2, 2, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0, 255, 255, 255,
		255, 0, 255, 128,   // alpha unchanged
		255, 255, 255, 255,
		0, 0, 0, 255,
	}
	for i, b := range pixels {
		if b != want[i] {
			t.Errorf("pixel[%d] = %d, want %d", i, b, want[i])
		}
	}
}

func TestFilterInvertWithSelectionMask(t *testing.T) {
	pixels := []byte{
		255, 0, 0, 255, // red
		0, 255, 0, 255, // green
	}
	mask := []byte{255, 0} // first pixel selected, second not
	err := filterInvert(pixels, 2, 1, mask, nil)
	if err != nil {
		t.Fatal(err)
	}
	// First pixel inverted, second untouched.
	if pixels[0] != 0 || pixels[1] != 255 || pixels[2] != 255 {
		t.Errorf("selected pixel not inverted: got (%d,%d,%d)", pixels[0], pixels[1], pixels[2])
	}
	if pixels[4] != 0 || pixels[5] != 255 || pixels[6] != 0 {
		t.Errorf("unselected pixel should be untouched: got (%d,%d,%d)", pixels[4], pixels[5], pixels[6])
	}
}

func TestFilterInvertPartialSelection(t *testing.T) {
	pixels := []byte{255, 0, 0, 255} // red
	mask := []byte{128}              // 50% selected
	err := filterInvert(pixels, 1, 1, mask, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Blended: 50% of 255→0 = ~128, 50% of 0→255 = ~128
	if pixels[0] > 130 || pixels[0] < 126 {
		t.Errorf("R = %d, want ~128", pixels[0])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test -run TestFilterInvert ./internal/engine/ -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// filters_builtin.go
package engine

import "encoding/json"

func init() {
	RegisterFilter(FilterDef{
		ID:       "invert",
		Name:     "Invert",
		Category: FilterCategoryOther,
	}, filterInvert)
}

func filterInvert(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	for i := 0; i < len(pixels); i += 4 {
		nr := 255 - pixels[i]
		ng := 255 - pixels[i+1]
		nb := 255 - pixels[i+2]

		idx := i / 4
		if selMask != nil && idx < len(selMask) {
			a := selMask[idx]
			if a == 0 {
				continue
			}
			if a < 255 {
				pixels[i] = blendByte(pixels[i], nr, a)
				pixels[i+1] = blendByte(pixels[i+1], ng, a)
				pixels[i+2] = blendByte(pixels[i+2], nb, a)
				continue
			}
		}
		pixels[i] = nr
		pixels[i+1] = ng
		pixels[i+2] = nb
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd packages/engine-wasm && go test -run TestFilterInvert ./internal/engine/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add Invert filter (immediate, no dialog)
```

---

## Task 5: Gaussian Blur filter via agg_go StackBlur

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

This is the key agg_go integration. We wrap `PixelLayer.Pixels` in a `PixelReadWriter` adapter and call `agg_go.StackBlur.Blur()`.

**Step 1: Write the failing test**

```go
func TestFilterGaussianBlur(t *testing.T) {
	// 5x5 image: single white pixel at center, rest black.
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	// Set center pixel (2,2) to white.
	idx := (2*w + 2) * 4
	pixels[idx] = 255
	pixels[idx+1] = 255
	pixels[idx+2] = 255
	pixels[idx+3] = 255
	// Set all other alpha to 255 so blur doesn't produce zero-alpha.
	for i := 3; i < len(pixels); i += 4 {
		pixels[i] = 255
	}

	params, _ := json.Marshal(map[string]any{"radius": 1})
	err := filterGaussianBlur(pixels, w, h, nil, params)
	if err != nil {
		t.Fatal(err)
	}

	// After blur, center pixel should no longer be pure white.
	if pixels[idx] == 255 && pixels[idx+1] == 255 && pixels[idx+2] == 255 {
		t.Error("center pixel should have been spread by blur")
	}
	// Neighbours should have gained some brightness.
	nIdx := (2*w + 3) * 4 // right neighbour
	if pixels[nIdx] == 0 && pixels[nIdx+1] == 0 && pixels[nIdx+2] == 0 {
		t.Error("neighbour should have received blur spread")
	}
}

func TestFilterGaussianBlurZeroRadiusIsNoop(t *testing.T) {
	pixels := []byte{255, 0, 0, 255, 0, 255, 0, 255}
	orig := append([]byte(nil), pixels...)
	params, _ := json.Marshal(map[string]any{"radius": 0})
	err := filterGaussianBlur(pixels, 2, 1, nil, params)
	if err != nil {
		t.Fatal(err)
	}
	for i := range pixels {
		if pixels[i] != orig[i] {
			t.Errorf("pixel[%d] changed with radius 0", i)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test -run TestFilterGaussianBlur ./internal/engine/ -v`
Expected: FAIL

**Step 3: Write implementation**

Add to `filters_builtin.go`:

```go
import (
	agglib "github.com/cwbudde/agg_go"
)

func init() {
	// ... (previous invert registration) ...

	RegisterFilter(FilterDef{
		ID:        "gaussian-blur",
		Name:      "Gaussian Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}, filterGaussianBlur)
}

type gaussianBlurParams struct {
	Radius int `json:"radius"`
}

// pixelRW adapts a flat RGBA8 byte slice to agg_go's PixelReadWriter interface.
type pixelRW struct {
	pixels []byte
	w, h   int
}

func (p *pixelRW) Width() int  { return p.w }
func (p *pixelRW) Height() int { return p.h }

func (p *pixelRW) Pixel(x, y int) agglib.Color {
	if x < 0 || x >= p.w || y < 0 || y >= p.h {
		return agglib.NewColor(0, 0, 0, 0)
	}
	i := (y*p.w + x) * 4
	return agglib.NewColor(p.pixels[i], p.pixels[i+1], p.pixels[i+2], p.pixels[i+3])
}

func (p *pixelRW) SetPixel(x, y int, c agglib.Color) {
	if x < 0 || x >= p.w || y < 0 || y >= p.h {
		return
	}
	i := (y*p.w + x) * 4
	r, g, b, a := c.RGBA8()
	p.pixels[i] = r
	p.pixels[i+1] = g
	p.pixels[i+2] = b
	p.pixels[i+3] = a
}

func filterGaussianBlur(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p gaussianBlurParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}

	// If there's a selection mask, we need to blur a copy and blend back.
	if selMask != nil {
		orig := append([]byte(nil), pixels...)
		rw := &pixelRW{pixels: pixels, w: w, h: h}
		sb := agglib.NewStackBlur()
		sb.Blur(rw, p.Radius)
		// Blend blurred result with original using selection mask.
		for i := 0; i < len(pixels); i += 4 {
			idx := i / 4
			a := selMask[idx]
			if a == 0 {
				pixels[i] = orig[i]
				pixels[i+1] = orig[i+1]
				pixels[i+2] = orig[i+2]
				pixels[i+3] = orig[i+3]
			} else if a < 255 {
				pixels[i] = blendByte(orig[i], pixels[i], a)
				pixels[i+1] = blendByte(orig[i+1], pixels[i+1], a)
				pixels[i+2] = blendByte(orig[i+2], pixels[i+2], a)
				pixels[i+3] = blendByte(orig[i+3], pixels[i+3], a)
			}
		}
		return nil
	}

	rw := &pixelRW{pixels: pixels, w: w, h: h}
	sb := agglib.NewStackBlur()
	sb.Blur(rw, p.Radius)
	return nil
}
```

Note: The `pixelRW` adapter needs to implement `agglib.PixelReadWriter[color.RGBA8[color.Linear]]`. The exact interface may require `Pixel(x, y int)` and `CopyColorHspan(x, y, len int, colors []Color)` — adapt the adapter to match the actual interface signature in agg_go. Check `agg_go/analysis.go` for the exact `PixelReadWriter` definition and adjust accordingly.

**Step 4: Run test to verify it passes**

Run: `cd packages/engine-wasm && go test -run TestFilterGaussianBlur ./internal/engine/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add Gaussian Blur filter backed by agg_go StackBlur
```

---

## Task 6: Brightness/Contrast filter

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

**Step 1: Write the failing test**

```go
func TestFilterBrightnessContrast(t *testing.T) {
	tests := []struct {
		name       string
		brightness int
		contrast   int
		input      [4]byte
		wantR      byte
	}{
		{"brightness +50", 50, 0, [4]byte{100, 100, 100, 255}, 150},
		{"brightness -50", -50, 0, [4]byte{100, 100, 100, 255}, 50},
		{"brightness clamps high", 200, 0, [4]byte{200, 200, 200, 255}, 255},
		{"brightness clamps low", -200, 0, [4]byte{50, 50, 50, 255}, 0},
		{"contrast +50 on 200", 0, 50, [4]byte{200, 200, 200, 255}, 237}, // Photoshop-style
		{"no change", 0, 0, [4]byte{100, 100, 100, 255}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixels := []byte{tt.input[0], tt.input[1], tt.input[2], tt.input[3]}
			params, _ := json.Marshal(map[string]any{
				"brightness": tt.brightness,
				"contrast":   tt.contrast,
			})
			err := filterBrightnessContrast(pixels, 1, 1, nil, params)
			if err != nil {
				t.Fatal(err)
			}
			diff := int(pixels[0]) - int(tt.wantR)
			if diff < -2 || diff > 2 {
				t.Errorf("R = %d, want ~%d", pixels[0], tt.wantR)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test -run TestFilterBrightnessContrast ./internal/engine/ -v`
Expected: FAIL

**Step 3: Write implementation**

```go
func init() {
	RegisterFilter(FilterDef{
		ID:        "brightness-contrast",
		Name:      "Brightness/Contrast",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterBrightnessContrast)
}

type brightnessContrastParams struct {
	Brightness int `json:"brightness"` // -150 to +150
	Contrast   int `json:"contrast"`   // -100 to +100
}

func filterBrightnessContrast(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p brightnessContrastParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Brightness == 0 && p.Contrast == 0 {
		return nil
	}

	// Build LUT for speed.
	var lut [256]byte
	// Photoshop-style contrast: factor = (259*(contrast+255)) / (255*(259-contrast))
	contrastFactor := float64(259*(p.Contrast+255)) / float64(255*(259-p.Contrast))
	for i := 0; i < 256; i++ {
		v := float64(i) + float64(p.Brightness)
		v = contrastFactor*(v-128) + 128
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		lut[i] = byte(v)
	}

	for i := 0; i < len(pixels); i += 4 {
		nr, ng, nb := lut[pixels[i]], lut[pixels[i+1]], lut[pixels[i+2]]
		idx := i / 4
		if selMask != nil && idx < len(selMask) {
			a := selMask[idx]
			if a == 0 {
				continue
			}
			if a < 255 {
				pixels[i] = blendByte(pixels[i], nr, a)
				pixels[i+1] = blendByte(pixels[i+1], ng, a)
				pixels[i+2] = blendByte(pixels[i+2], nb, a)
				continue
			}
		}
		pixels[i] = nr
		pixels[i+1] = ng
		pixels[i+2] = nb
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd packages/engine-wasm && go test -run TestFilterBrightnessContrast ./internal/engine/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add Brightness/Contrast filter with LUT optimization
```

---

## Task 7: Sharpen filter (Unsharp Mask via agg_go StackBlur)

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

Unsharp mask = `original + amount * (original - blurred)`. Uses agg_go StackBlur to generate the blurred version.

**Step 1: Write the failing test**

```go
func TestFilterUnsharpMask(t *testing.T) {
	// 5x5 image with an edge: left half dark, right half bright.
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if x < 3 {
				pixels[i], pixels[i+1], pixels[i+2] = 50, 50, 50
			} else {
				pixels[i], pixels[i+1], pixels[i+2] = 200, 200, 200
			}
			pixels[i+3] = 255
		}
	}
	orig := append([]byte(nil), pixels...)

	params, _ := json.Marshal(map[string]any{"amount": 100, "radius": 1, "threshold": 0})
	err := filterUnsharpMask(pixels, w, h, nil, params)
	if err != nil {
		t.Fatal(err)
	}

	// The edge should be enhanced: dark side darker, bright side brighter.
	darkIdx := (2*w + 1) * 4
	brightIdx := (2*w + 3) * 4
	if pixels[darkIdx] >= orig[darkIdx] {
		t.Errorf("dark side should be darker after sharpen: was %d, now %d", orig[darkIdx], pixels[darkIdx])
	}
	if pixels[brightIdx] <= orig[brightIdx] {
		t.Errorf("bright side should be brighter after sharpen: was %d, now %d", orig[brightIdx], pixels[brightIdx])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test -run TestFilterUnsharpMask ./internal/engine/ -v`
Expected: FAIL

**Step 3: Write implementation**

```go
func init() {
	RegisterFilter(FilterDef{
		ID:        "unsharp-mask",
		Name:      "Unsharp Mask",
		Category:  FilterCategorySharpen,
		HasDialog: true,
	}, filterUnsharpMask)
}

type unsharpMaskParams struct {
	Amount    int `json:"amount"`    // percentage, 1-500
	Radius    int `json:"radius"`    // blur radius, 1-250
	Threshold int `json:"threshold"` // 0-255, ignore edges below this
}

func filterUnsharpMask(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p unsharpMaskParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 || p.Amount <= 0 {
		return nil
	}

	// Create blurred copy using agg_go StackBlur.
	blurred := append([]byte(nil), pixels...)
	rw := &pixelRW{pixels: blurred, w: w, h: h}
	sb := agglib.NewStackBlur()
	sb.Blur(rw, p.Radius)

	amt := float64(p.Amount) / 100.0
	for i := 0; i < len(pixels); i += 4 {
		idx := i / 4
		// Check threshold: skip if difference is below threshold.
		dr := abs8(pixels[i], blurred[i])
		dg := abs8(pixels[i+1], blurred[i+1])
		db := abs8(pixels[i+2], blurred[i+2])
		if int(dr)+int(dg)+int(db) < p.Threshold*3 {
			continue
		}

		nr := clamp8(float64(pixels[i]) + amt*float64(int(pixels[i])-int(blurred[i])))
		ng := clamp8(float64(pixels[i+1]) + amt*float64(int(pixels[i+1])-int(blurred[i+1])))
		nb := clamp8(float64(pixels[i+2]) + amt*float64(int(pixels[i+2])-int(blurred[i+2])))

		if selMask != nil && idx < len(selMask) {
			a := selMask[idx]
			if a == 0 {
				continue
			}
			if a < 255 {
				pixels[i] = blendByte(pixels[i], nr, a)
				pixels[i+1] = blendByte(pixels[i+1], ng, a)
				pixels[i+2] = blendByte(pixels[i+2], nb, a)
				continue
			}
		}
		pixels[i] = nr
		pixels[i+1] = ng
		pixels[i+2] = nb
	}
	return nil
}

func abs8(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}

func clamp8(v float64) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}
```

**Step 4: Run test to verify it passes**

Run: `cd packages/engine-wasm && go test -run TestFilterUnsharpMask ./internal/engine/ -v`
Expected: PASS

**Step 5: Commit**

```
feat: add Unsharp Mask filter using agg_go StackBlur
```

---

## Task 8: Add Noise filter

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

**Step 1: Write the failing test**

```go
func TestFilterAddNoise(t *testing.T) {
	pixels := make([]byte, 100*100*4)
	for i := 3; i < len(pixels); i += 4 {
		pixels[i] = 255
	}
	// Fill with mid-gray.
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 128
		pixels[i+1] = 128
		pixels[i+2] = 128
	}
	orig := append([]byte(nil), pixels...)

	params, _ := json.Marshal(map[string]any{"amount": 25, "distribution": "gaussian", "monochromatic": false})
	err := filterAddNoise(pixels, 100, 100, nil, params)
	if err != nil {
		t.Fatal(err)
	}

	// At least some pixels should differ.
	changed := 0
	for i := 0; i < len(pixels); i += 4 {
		if pixels[i] != orig[i] || pixels[i+1] != orig[i+1] || pixels[i+2] != orig[i+2] {
			changed++
		}
	}
	if changed < 100 {
		t.Errorf("expected many changed pixels, got %d", changed)
	}
}

func TestFilterAddNoiseMonochromatic(t *testing.T) {
	pixels := []byte{128, 128, 128, 255}
	params, _ := json.Marshal(map[string]any{"amount": 50, "distribution": "uniform", "monochromatic": true})
	err := filterAddNoise(pixels, 1, 1, nil, params)
	if err != nil {
		t.Fatal(err)
	}
	// Monochromatic: all channels shifted by the same amount.
	dr := int(pixels[0]) - 128
	dg := int(pixels[1]) - 128
	db := int(pixels[2]) - 128
	if dr != dg || dg != db {
		t.Errorf("monochromatic noise should shift all channels equally: dr=%d dg=%d db=%d", dr, dg, db)
	}
}
```

**Step 2: Run test to verify it fails**

**Step 3: Write implementation**

```go
import (
	"math"
	"math/rand/v2"
)

func init() {
	RegisterFilter(FilterDef{
		ID:        "add-noise",
		Name:      "Add Noise",
		Category:  FilterCategoryNoise,
		HasDialog: true,
	}, filterAddNoise)
}

type addNoiseParams struct {
	Amount       int    `json:"amount"`       // 0-400
	Distribution string `json:"distribution"` // "uniform" or "gaussian"
	Monochromatic bool  `json:"monochromatic"`
}

func filterAddNoise(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p addNoiseParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Amount <= 0 {
		return nil
	}

	rng := rand.New(rand.NewPCG(42, 0)) // deterministic seed for reproducibility
	amt := float64(p.Amount)

	noise := func() float64 {
		if p.Distribution == "gaussian" {
			return rng.NormFloat64() * amt * 0.5
		}
		return (rng.Float64()*2 - 1) * amt
	}

	for i := 0; i < len(pixels); i += 4 {
		idx := i / 4
		if selMask != nil && idx < len(selMask) && selMask[idx] == 0 {
			continue
		}

		var nr, ng, nb byte
		if p.Monochromatic {
			n := noise()
			nr = clamp8(float64(pixels[i]) + n)
			ng = clamp8(float64(pixels[i+1]) + n)
			nb = clamp8(float64(pixels[i+2]) + n)
		} else {
			nr = clamp8(float64(pixels[i]) + noise())
			ng = clamp8(float64(pixels[i+1]) + noise())
			nb = clamp8(float64(pixels[i+2]) + noise())
		}

		if selMask != nil && idx < len(selMask) {
			a := selMask[idx]
			if a < 255 {
				pixels[i] = blendByte(pixels[i], nr, a)
				pixels[i+1] = blendByte(pixels[i+1], ng, a)
				pixels[i+2] = blendByte(pixels[i+2], nb, a)
				continue
			}
		}
		pixels[i] = nr
		pixels[i+1] = ng
		pixels[i+2] = nb
	}
	return nil
}
```

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```
feat: add Add Noise filter with uniform/gaussian distribution
```

---

## Task 9: High Pass filter (via agg_go StackBlur)

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

High Pass = `original - blurred + 128` (per channel). Very useful with Overlay blend mode for sharpening.

**Step 1: Write the failing test**

```go
func TestFilterHighPass(t *testing.T) {
	// Uniform gray → high pass should produce 128,128,128 (no edges).
	w, h := 5, 5
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 100
		pixels[i+1] = 100
		pixels[i+2] = 100
		pixels[i+3] = 255
	}

	params, _ := json.Marshal(map[string]any{"radius": 2})
	err := filterHighPass(pixels, w, h, nil, params)
	if err != nil {
		t.Fatal(err)
	}

	// Center pixel should be ~128 (neutral gray = no edge).
	idx := (2*w + 2) * 4
	diff := int(pixels[idx]) - 128
	if diff < -2 || diff > 2 {
		t.Errorf("uniform area should be ~128, got %d", pixels[idx])
	}
}
```

**Step 2: Run test to verify it fails**

**Step 3: Write implementation**

```go
func init() {
	RegisterFilter(FilterDef{
		ID:        "high-pass",
		Name:      "High Pass",
		Category:  FilterCategoryOther,
		HasDialog: true,
	}, filterHighPass)
}

type highPassParams struct {
	Radius int `json:"radius"`
}

func filterHighPass(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p highPassParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Radius <= 0 {
		return nil
	}

	blurred := append([]byte(nil), pixels...)
	rw := &pixelRW{pixels: blurred, w: w, h: h}
	sb := agglib.NewStackBlur()
	sb.Blur(rw, p.Radius)

	for i := 0; i < len(pixels); i += 4 {
		nr := clamp8(float64(pixels[i]) - float64(blurred[i]) + 128)
		ng := clamp8(float64(pixels[i+1]) - float64(blurred[i+1]) + 128)
		nb := clamp8(float64(pixels[i+2]) - float64(blurred[i+2]) + 128)

		idx := i / 4
		if selMask != nil && idx < len(selMask) {
			a := selMask[idx]
			if a == 0 {
				continue
			}
			if a < 255 {
				pixels[i] = blendByte(pixels[i], nr, a)
				pixels[i+1] = blendByte(pixels[i+1], ng, a)
				pixels[i+2] = blendByte(pixels[i+2], nb, a)
				continue
			}
		}
		pixels[i] = nr
		pixels[i+1] = ng
		pixels[i+2] = nb
	}
	return nil
}
```

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```
feat: add High Pass filter using agg_go StackBlur
```

---

## Task 10: Emboss filter

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

Emboss uses a directional 3x3 convolution kernel. The kernel depends on the angle parameter.

**Step 1: Write the failing test**

```go
func TestFilterEmboss(t *testing.T) {
	// 3x3 image: bright center, dark surround.
	w, h := 3, 3
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 50
		pixels[i+1] = 50
		pixels[i+2] = 50
		pixels[i+3] = 255
	}
	// Bright center.
	idx := (1*w + 1) * 4
	pixels[idx] = 200
	pixels[idx+1] = 200
	pixels[idx+2] = 200

	params, _ := json.Marshal(map[string]any{"angle": 135, "height": 1, "amount": 100})
	err := filterEmboss(pixels, w, h, nil, params)
	if err != nil {
		t.Fatal(err)
	}

	// Result should have non-uniform values (edges visible).
	allSame := true
	first := pixels[0]
	for i := 4; i < len(pixels); i += 4 {
		if pixels[i] != first {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("emboss should produce varying pixel values at edges")
	}
}
```

**Step 2: Run test to verify it fails**

**Step 3: Write implementation**

```go
func init() {
	RegisterFilter(FilterDef{
		ID:        "emboss",
		Name:      "Emboss",
		Category:  FilterCategoryStylize,
		HasDialog: true,
	}, filterEmboss)
}

type embossParams struct {
	Angle  int `json:"angle"`  // degrees, 0-360
	Height int `json:"height"` // 1-10 pixels
	Amount int `json:"amount"` // 1-500 percent
}

func filterEmboss(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
	var p embossParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return err
		}
	}
	if p.Height <= 0 {
		p.Height = 1
	}
	if p.Amount <= 0 {
		p.Amount = 100
	}

	// Build directional emboss kernel based on angle.
	rad := float64(p.Angle) * math.Pi / 180.0
	dx := math.Cos(rad)
	dy := math.Sin(rad)
	scale := float64(p.Amount) / 100.0 * float64(p.Height)

	orig := append([]byte(nil), pixels...)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			idx := i / 4

			// Sample in emboss direction.
			sx := x + int(math.Round(dx))
			sy := y + int(math.Round(dy))
			si := sampleIdx(sx, sy, w, h)

			var nr, ng, nb byte
			if si >= 0 {
				nr = clamp8(float64(orig[i])-float64(orig[si])*scale + 128)
				ng = clamp8(float64(orig[i+1])-float64(orig[si+1])*scale + 128)
				nb = clamp8(float64(orig[i+2])-float64(orig[si+2])*scale + 128)
			} else {
				nr, ng, nb = 128, 128, 128
			}

			if selMask != nil && idx < len(selMask) {
				a := selMask[idx]
				if a == 0 {
					pixels[i] = orig[i]
					pixels[i+1] = orig[i+1]
					pixels[i+2] = orig[i+2]
					continue
				}
				if a < 255 {
					pixels[i] = blendByte(orig[i], nr, a)
					pixels[i+1] = blendByte(orig[i+1], ng, a)
					pixels[i+2] = blendByte(orig[i+2], nb, a)
					continue
				}
			}
			pixels[i] = nr
			pixels[i+1] = ng
			pixels[i+2] = nb
		}
	}
	return nil
}

func sampleIdx(x, y, w, h int) int {
	if x < 0 || x >= w || y < 0 || y >= h {
		return -1
	}
	return (y*w + x) * 4
}
```

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```
feat: add Emboss filter with directional kernel
```

---

## Task 11: Solarize and Find Edges (immediate filters)

**Files:**
- Modify: `packages/engine-wasm/internal/engine/filters_builtin.go`
- Test: `packages/engine-wasm/internal/engine/filters_builtin_test.go`

**Step 1: Write the failing tests**

```go
func TestFilterSolarize(t *testing.T) {
	pixels := []byte{
		200, 200, 200, 255, // above 128 → inverted
		50, 50, 50, 255,    // below 128 → unchanged
	}
	err := filterSolarize(pixels, 2, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pixels[0] != 55 { // 255 - 200
		t.Errorf("R = %d, want 55", pixels[0])
	}
	if pixels[4] != 50 {
		t.Errorf("R = %d, want 50 (unchanged)", pixels[4])
	}
}

func TestFilterFindEdges(t *testing.T) {
	// 3x3: uniform → no edges, should produce dark.
	w, h := 3, 3
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 128
		pixels[i+1] = 128
		pixels[i+2] = 128
		pixels[i+3] = 255
	}

	err := filterFindEdges(pixels, w, h, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Center pixel should be ~0 (no edge in uniform area).
	idx := (1*w + 1) * 4
	if pixels[idx] > 10 {
		t.Errorf("uniform area edge = %d, want ~0", pixels[idx])
	}
}
```

**Step 2: Run test to verify it fails**

**Step 3: Write implementation**

```go
func init() {
	RegisterFilter(FilterDef{
		ID:       "solarize",
		Name:     "Solarize",
		Category: FilterCategoryStylize,
	}, filterSolarize)

	RegisterFilter(FilterDef{
		ID:       "find-edges",
		Name:     "Find Edges",
		Category: FilterCategoryStylize,
	}, filterFindEdges)
}

func filterSolarize(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	for i := 0; i < len(pixels); i += 4 {
		idx := i / 4
		if selMask != nil && idx < len(selMask) && selMask[idx] == 0 {
			continue
		}
		for c := 0; c < 3; c++ {
			if pixels[i+c] >= 128 {
				pixels[i+c] = 255 - pixels[i+c]
			}
		}
	}
	return nil
}

func filterFindEdges(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
	orig := append([]byte(nil), pixels...)
	// Sobel operator.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			idx := i / 4
			if selMask != nil && idx < len(selMask) && selMask[idx] == 0 {
				continue
			}

			for c := 0; c < 3; c++ {
				gx := -sample(orig, x-1, y-1, c, w, h) - 2*sample(orig, x-1, y, c, w, h) - sample(orig, x-1, y+1, c, w, h) +
					sample(orig, x+1, y-1, c, w, h) + 2*sample(orig, x+1, y, c, w, h) + sample(orig, x+1, y+1, c, w, h)
				gy := -sample(orig, x-1, y-1, c, w, h) - 2*sample(orig, x, y-1, c, w, h) - sample(orig, x+1, y-1, c, w, h) +
					sample(orig, x-1, y+1, c, w, h) + 2*sample(orig, x, y+1, c, w, h) + sample(orig, x+1, y+1, c, w, h)
				mag := math.Sqrt(float64(gx*gx + gy*gy))
				pixels[i+c] = clamp8(mag)
			}
		}
	}
	return nil
}

func sample(pixels []byte, x, y, c, w, h int) int {
	if x < 0 {
		x = 0
	}
	if x >= w {
		x = w - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= h {
		y = h - 1
	}
	return int(pixels[(y*w+x)*4+c])
}
```

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```
feat: add Solarize and Find Edges filters
```

---

## Task 12: Run full test suite + lint

**Step 1:** Run all Go tests:
```
cd packages/engine-wasm && go test ./internal/engine/ -v -count=1
```
Expected: All PASS

**Step 2:** Run linter:
```
cd packages/engine-wasm && go vet ./...
```
Expected: Clean

**Step 3:** Run `just test-go` from project root:
```
just test-go
```
Expected: All PASS

**Step 4:** Commit any cleanup if needed.

---

## Summary of deliverables

| Task | Filter | agg_go usage | Dialog |
|------|--------|-------------|--------|
| 1 | — | — | — |
| 2 | — | — | — |
| 3 | — | — | — |
| 4 | Invert | none (byte loop) | no |
| 5 | Gaussian Blur | StackBlur | yes |
| 6 | Brightness/Contrast | none (LUT) | yes |
| 7 | Unsharp Mask | StackBlur | yes |
| 8 | Add Noise | none (rand) | yes |
| 9 | High Pass | StackBlur | yes |
| 10 | Emboss | none (kernel) | yes |
| 11 | Solarize, Find Edges | none (Sobel) | no |

**agg_go integration points:** Tasks 5, 7, 9 use `StackBlur` via `PixelReadWriter` adapter. The `pixelRW` adapter type (Task 5) is reused by all three.
