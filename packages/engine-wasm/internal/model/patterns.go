package model

import "bytes"

// PatternResource is a document-scoped repeatable RGBA tile used by pattern
// fills and (in later batches) pattern overlay/stroke layer styles. Data holds
// straight RGBA bytes, len = Width*Height*4.
type PatternResource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Data   []byte `json:"data"`
}

// Clone returns a deep copy of the pattern (the tile bytes are copied).
func (p PatternResource) Clone() PatternResource {
	clone := p
	clone.Data = append([]byte(nil), p.Data...)
	return clone
}

// ClonePatterns deep-copies a pattern list. A nil input stays nil so archives
// keep omitting the field for documents without patterns.
func ClonePatterns(patterns []PatternResource) []PatternResource {
	if patterns == nil {
		return nil
	}
	clones := make([]PatternResource, len(patterns))
	for i := range patterns {
		clones[i] = patterns[i].Clone()
	}
	return clones
}

// PatternsEqual reports whether two pattern lists are identical, including the
// tile bytes (tiles are small — capped at 1024x1024 — so bytes.Equal is cheap
// relative to the layer-tree comparisons around it).
func PatternsEqual(a, b []PatternResource) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Name != b[i].Name {
			return false
		}
		if a[i].Width != b[i].Width || a[i].Height != b[i].Height {
			return false
		}
		if !bytes.Equal(a[i].Data, b[i].Data) {
			return false
		}
	}
	return true
}

// NewPatternID returns a unique pattern resource ID. It reuses the layer ID
// generator ("pattern-<uuid>") so IDs stay collision-free per process even
// when entropy is unavailable.
func NewPatternID() string {
	return "pattern-" + NewLayerID()
}
