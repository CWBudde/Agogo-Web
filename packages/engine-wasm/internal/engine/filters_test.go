package engine

import (
	"encoding/json"
	"testing"
)

func TestFilterRegistryRegisterAndLookup(t *testing.T) {
	// Register a no-op filter.
	def := FilterDef{
		ID:        "gaussian-blur",
		Name:      "Gaussian Blur",
		Category:  FilterCategoryBlur,
		HasDialog: true,
	}
	RegisterFilter(def, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		return nil
	})
	t.Cleanup(func() {
		RegisterFilter(FilterDef{ID: "gaussian-blur"}, nil)
	})

	// Lookup should return the registered filter.
	got := lookupFilter("gaussian-blur")
	if got == nil {
		t.Fatal("expected registered filter, got nil")
	}
	if got.Def.Name != "Gaussian Blur" {
		t.Errorf("expected name %q, got %q", "Gaussian Blur", got.Def.Name)
	}
	if got.Def.Category != FilterCategoryBlur {
		t.Errorf("expected category %q, got %q", FilterCategoryBlur, got.Def.Category)
	}
	if !got.Def.HasDialog {
		t.Error("expected HasDialog to be true")
	}

	// Lookup with different casing and whitespace should still match.
	got2 := lookupFilter("  Gaussian-Blur  ")
	if got2 == nil {
		t.Fatal("expected normalized lookup to succeed")
	}

	// Unknown filter should return nil.
	unknown := lookupFilter("unknown-filter")
	if unknown != nil {
		t.Errorf("expected nil for unknown filter, got %+v", unknown)
	}
}

func TestApplyFilterInvertsPixels(t *testing.T) {
	// Register a test invert filter.
	invertDef := FilterDef{ID: "test-invert", Name: "Test Invert", Category: FilterCategoryOther}
	RegisterFilter(invertDef, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		for i := 0; i < w*h*4; i += 4 {
			alpha := byte(255)
			if selMask != nil {
				alpha = selMask[i/4]
			}
			if alpha == 0 {
				continue
			}
			pixels[i+0] = 255 - pixels[i+0]
			pixels[i+1] = 255 - pixels[i+1]
			pixels[i+2] = 255 - pixels[i+2]
			// leave alpha channel alone
		}
		return nil
	})
	t.Cleanup(func() { RegisterFilter(FilterDef{ID: "test-invert"}, nil) })

	// Create a 2x2 document with a pixel layer containing known pixels.
	layer := NewPixelLayer("bg", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, []byte{
		255, 0, 0, 255, // red
		0, 255, 0, 255, // green
		0, 0, 255, 255, // blue
		128, 128, 128, 255, // grey
	})
	root := NewGroupLayer("Root")
	root.SetChildren([]LayerNode{layer})
	doc := &Document{
		Width: 2, Height: 2, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8,
		ID: "filter-test", Name: "Filter Test",
		LayerRoot:     root,
		ActiveLayerID: layer.ID(),
	}

	versionBefore := doc.ContentVersion
	if err := doc.ApplyFilter(layer.ID(), "test-invert", nil); err != nil {
		t.Fatalf("ApplyFilter failed: %v", err)
	}

	// Verify pixels were inverted.
	want := []byte{
		0, 255, 255, 255,
		255, 0, 255, 255,
		255, 255, 0, 255,
		127, 127, 127, 255,
	}
	for i, b := range layer.Pixels {
		if b != want[i] {
			t.Errorf("pixel[%d] = %d, want %d", i, b, want[i])
		}
	}

	// Content version must have advanced.
	if doc.ContentVersion <= versionBefore {
		t.Errorf("ContentVersion did not advance: before=%d after=%d", versionBefore, doc.ContentVersion)
	}
}

func TestApplyFilterWithSelectionMask(t *testing.T) {
	// Register a test invert filter.
	invertDef := FilterDef{ID: "test-invert-sel", Name: "Test Invert Sel", Category: FilterCategoryOther}
	RegisterFilter(invertDef, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		for i := 0; i < w*h*4; i += 4 {
			alpha := byte(255)
			if selMask != nil {
				alpha = selMask[i/4]
			}
			if alpha == 0 {
				continue
			}
			pixels[i+0] = 255 - pixels[i+0]
			pixels[i+1] = 255 - pixels[i+1]
			pixels[i+2] = 255 - pixels[i+2]
		}
		return nil
	})
	t.Cleanup(func() { RegisterFilter(FilterDef{ID: "test-invert-sel"}, nil) })

	// 2x2 layer at origin, all white.
	layer := NewPixelLayer("bg", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, []byte{
		255, 255, 255, 255,
		255, 255, 255, 255,
		255, 255, 255, 255,
		255, 255, 255, 255,
	})
	root := NewGroupLayer("Root")
	root.SetChildren([]LayerNode{layer})

	// Selection covers only top-left pixel.
	sel := &Selection{Width: 2, Height: 2, Mask: []byte{255, 0, 0, 0}}

	doc := &Document{
		Width: 2, Height: 2, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8,
		ID: "filter-sel-test", Name: "Filter Sel Test",
		LayerRoot:     root,
		ActiveLayerID: layer.ID(),
		Selection:     sel,
	}

	if err := doc.ApplyFilter(layer.ID(), "test-invert-sel", nil); err != nil {
		t.Fatalf("ApplyFilter failed: %v", err)
	}

	// Only the top-left pixel should be inverted.
	if layer.Pixels[0] != 0 || layer.Pixels[1] != 0 || layer.Pixels[2] != 0 {
		t.Errorf("top-left pixel should be inverted to black, got %v", layer.Pixels[0:4])
	}
	// Other pixels should remain white.
	for i := 4; i < 16; i += 4 {
		if layer.Pixels[i] != 255 || layer.Pixels[i+1] != 255 || layer.Pixels[i+2] != 255 {
			t.Errorf("pixel at offset %d should remain white, got %v", i, layer.Pixels[i:i+4])
		}
	}
}

func TestApplyFilterUnknownFilterReturnsError(t *testing.T) {
	layer := NewPixelLayer("bg", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{0, 0, 0, 255})
	root := NewGroupLayer("Root")
	root.SetChildren([]LayerNode{layer})
	doc := &Document{
		Width: 1, Height: 1, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8,
		ID: "err-test", Name: "Err Test",
		LayerRoot:     root,
		ActiveLayerID: layer.ID(),
	}

	err := doc.ApplyFilter(layer.ID(), "nonexistent-filter", nil)
	if err == nil {
		t.Fatal("expected error for unknown filter, got nil")
	}
}

func TestApplyFilterNonPixelLayerReturnsError(t *testing.T) {
	adj := NewAdjustmentLayer("levels", "levels", nil)
	root := NewGroupLayer("Root")
	root.SetChildren([]LayerNode{adj})
	doc := &Document{
		Width: 1, Height: 1, Resolution: 72,
		ColorMode: "rgb", BitDepth: 8,
		ID: "nonpx-test", Name: "NonPx Test",
		LayerRoot:     root,
		ActiveLayerID: adj.ID(),
	}

	// Register a dummy filter so we get past the lookup.
	RegisterFilter(FilterDef{ID: "dummy-for-nonpx", Name: "Dummy", Category: FilterCategoryOther},
		func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error { return nil })
	t.Cleanup(func() { RegisterFilter(FilterDef{ID: "dummy-for-nonpx"}, nil) })

	err := doc.ApplyFilter(adj.ID(), "dummy-for-nonpx", nil)
	if err == nil {
		t.Fatal("expected error for non-pixel layer, got nil")
	}
}

func TestDispatchApplyFilterWithUndo(t *testing.T) {
	// Register a test invert filter.
	invertDef := FilterDef{ID: "dispatch-invert", Name: "Dispatch Invert", Category: FilterCategoryOther}
	RegisterFilter(invertDef, func(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
		for i := 0; i < w*h*4; i += 4 {
			pixels[i+0] = 255 - pixels[i+0] // R
			pixels[i+1] = 255 - pixels[i+1] // G
			pixels[i+2] = 255 - pixels[i+2] // B
		}
		return nil
	})
	t.Cleanup(func() { RegisterFilter(FilterDef{ID: "dispatch-invert"}, nil) })

	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Add a 2x2 pixel layer filled with red.
	redPixels := make([]byte, 2*2*4)
	for i := 0; i < len(redPixels); i += 4 {
		redPixels[i+0] = 255 // R
		redPixels[i+1] = 0   // G
		redPixels[i+2] = 0   // B
		redPixels[i+3] = 255 // A
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "red-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    redPixels,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Apply the invert filter via dispatch.
	_, err = DispatchCommand(h, commandApplyFilter, mustJSON(t, ApplyFilterPayload{
		FilterID: "dispatch-invert",
	}))
	if err != nil {
		t.Fatalf("apply filter: %v", err)
	}

	// Verify pixels are now cyan (0, 255, 255, 255).
	mu.Lock()
	inst := instances[h]
	mu.Unlock()
	doc := inst.manager.Active()
	pl := doc.findLayer(doc.ActiveLayerID).(*PixelLayer)
	for i := 0; i < len(pl.Pixels); i += 4 {
		if pl.Pixels[i+0] != 0 || pl.Pixels[i+1] != 255 || pl.Pixels[i+2] != 255 {
			t.Fatalf("pixel[%d] after invert = [%d,%d,%d], want [0,255,255]",
				i/4, pl.Pixels[i+0], pl.Pixels[i+1], pl.Pixels[i+2])
		}
	}

	// Undo and verify pixels are restored to red.
	_, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	mu.Lock()
	inst = instances[h]
	mu.Unlock()
	doc = inst.manager.Active()
	pl = doc.findLayer(doc.ActiveLayerID).(*PixelLayer)
	for i := 0; i < len(pl.Pixels); i += 4 {
		if pl.Pixels[i+0] != 255 || pl.Pixels[i+1] != 0 || pl.Pixels[i+2] != 0 {
			t.Fatalf("pixel[%d] after undo = [%d,%d,%d], want [255,0,0]",
				i/4, pl.Pixels[i+0], pl.Pixels[i+1], pl.Pixels[i+2])
		}
	}
}

func TestDispatchReapplyFilter(t *testing.T) {
	// Register a test invert filter.
	invertDef := FilterDef{ID: "reapply-invert", Name: "Reapply Invert", Category: FilterCategoryOther}
	RegisterFilter(invertDef, func(pixels []byte, w, h int, selMask []byte, _ json.RawMessage) error {
		for i := 0; i < w*h*4; i += 4 {
			pixels[i+0] = 255 - pixels[i+0]
			pixels[i+1] = 255 - pixels[i+1]
			pixels[i+2] = 255 - pixels[i+2]
		}
		return nil
	})
	t.Cleanup(func() { RegisterFilter(FilterDef{ID: "reapply-invert"}, nil) })

	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// Add a 2x2 pixel layer filled with red.
	redPixels := make([]byte, 2*2*4)
	for i := 0; i < len(redPixels); i += 4 {
		redPixels[i+0] = 255
		redPixels[i+1] = 0
		redPixels[i+2] = 0
		redPixels[i+3] = 255
	}
	_, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: "pixel",
		Name:      "red-layer",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels:    redPixels,
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	// Apply the invert filter (red -> cyan).
	_, err = DispatchCommand(h, commandApplyFilter, mustJSON(t, ApplyFilterPayload{
		FilterID: "reapply-invert",
	}))
	if err != nil {
		t.Fatalf("apply filter: %v", err)
	}

	// Reapply the same filter (cyan -> red again via double invert).
	_, err = DispatchCommand(h, commandReapplyFilter, "")
	if err != nil {
		t.Fatalf("reapply filter: %v", err)
	}

	// Verify pixels are back to red.
	mu.Lock()
	inst := instances[h]
	mu.Unlock()
	doc := inst.manager.Active()
	pl := doc.findLayer(doc.ActiveLayerID).(*PixelLayer)
	for i := 0; i < len(pl.Pixels); i += 4 {
		if pl.Pixels[i+0] != 255 || pl.Pixels[i+1] != 0 || pl.Pixels[i+2] != 0 {
			t.Fatalf("pixel[%d] after double invert = [%d,%d,%d], want [255,0,0]",
				i/4, pl.Pixels[i+0], pl.Pixels[i+1], pl.Pixels[i+2])
		}
	}
}

func TestDispatchReapplyFilterWithoutPriorFilter(t *testing.T) {
	h := initWithDefaultDoc(t)
	t.Cleanup(func() { Free(h) })

	// ReapplyFilter without a prior ApplyFilter should return an error.
	_, err := DispatchCommand(h, commandReapplyFilter, "")
	if err == nil {
		t.Fatal("expected error for reapply without prior filter, got nil")
	}
}

func TestFilterRegistryDeregister(t *testing.T) {
	def := FilterDef{
		ID:       "sharpen-test",
		Name:     "Sharpen",
		Category: FilterCategorySharpen,
	}
	RegisterFilter(def, func(pixels []byte, w, h int, selMask []byte, params json.RawMessage) error {
		return nil
	})

	// Should exist after registration.
	if got := lookupFilter("sharpen-test"); got == nil {
		t.Fatal("expected filter to be registered")
	}

	// Deregister by passing nil fn.
	RegisterFilter(FilterDef{ID: "sharpen-test"}, nil)

	// Should be gone now.
	if got := lookupFilter("sharpen-test"); got != nil {
		t.Errorf("expected nil after deregister, got %+v", got)
	}
}
