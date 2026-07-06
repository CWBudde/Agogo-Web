package model

// Phase S.5 regression test: NewLayerID must never panic when the entropy
// source fails — library code degrades to a process-unique fallback ID.

import (
	"errors"
	"regexp"
	"testing"
)

var layerIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestNewLayerIDDoesNotPanicWhenEntropyFails(t *testing.T) {
	original := layerIDRandRead
	layerIDRandRead = func(b []byte) (int, error) {
		return 0, errors.New("entropy source exhausted")
	}
	defer func() { layerIDRandRead = original }()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewLayerID panicked on entropy failure: %v", r)
		}
	}()

	first := NewLayerID()
	second := NewLayerID()
	if !layerIDPattern.MatchString(first) {
		t.Fatalf("fallback layer id %q does not match UUID format", first)
	}
	if !layerIDPattern.MatchString(second) {
		t.Fatalf("fallback layer id %q does not match UUID format", second)
	}
	if first == second {
		t.Fatalf("fallback layer ids must stay unique, got %q twice", first)
	}
}

func TestNewLayerIDFormat(t *testing.T) {
	id := NewLayerID()
	if !layerIDPattern.MatchString(id) {
		t.Fatalf("layer id %q does not match UUID format", id)
	}
	if other := NewLayerID(); other == id {
		t.Fatalf("layer ids must be unique, got %q twice", id)
	}
}
