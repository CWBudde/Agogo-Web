package engine

import "fmt"

// SetGroupExpanded flips a group's open/closed flag — the PSD section divider's
// lsct 1 (open folder) vs lsct 2 (closed folder), which the Layers panel shows
// as the twirl-down chevron.
//
// It deliberately does NOT call touchModifiedAt: expansion is chrome, not
// content, and bumping ModifiedAt would make merely twirling a folder open mark
// the document dirty. It is still real document state, not view state — it
// round-trips through the project archive and through the PSD file — which is
// why it lives on the model rather than in the panel's local React state.
func (doc *Document) SetGroupExpanded(layerID string, expanded bool) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return fmt.Errorf("layer %q not found", layerID)
	}
	group, isGroup := layer.(*GroupLayer)
	if !isGroup {
		return fmt.Errorf("layer %q is not a group", layer.Name())
	}
	group.Expanded = expanded
	return nil
}
