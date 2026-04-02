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
