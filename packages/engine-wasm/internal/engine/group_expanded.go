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

// captureGroupExpansion records every group's open/closed flag in doc, keyed by
// layer ID.
//
// Expansion is deliberately not undoable (see SetGroupExpanded), but it lives
// inside the Document that history snapshots capture wholesale. Without
// carrying the live flags across a restore, undoing an edit made while a folder
// was open would silently re-open a folder the user has since collapsed — and
// redo would reinstate stale expansion the same way.
func captureGroupExpansion(doc *Document) map[string]bool {
	if doc == nil {
		return nil
	}
	expanded := make(map[string]bool)
	walkLayers(doc.ensureLayerRoot(), func(node LayerNode) bool {
		if group, ok := node.(*GroupLayer); ok {
			expanded[group.ID()] = group.Expanded
		}
		return true
	})
	if len(expanded) == 0 {
		return nil
	}
	return expanded
}

// applyGroupExpansion reinstates flags captured by captureGroupExpansion.
//
// A group the map does not mention keeps whatever the restored document says:
// that is a group the snapshot has and the live document does not, so there is
// no live flag to preserve and the snapshot's own value is the only answer.
func applyGroupExpansion(doc *Document, expanded map[string]bool) {
	if doc == nil || len(expanded) == 0 {
		return
	}
	walkLayers(doc.ensureLayerRoot(), func(node LayerNode) bool {
		group, ok := node.(*GroupLayer)
		if !ok {
			return true
		}
		if live, found := expanded[group.ID()]; found {
			group.Expanded = live
		}
		return true
	})
}

// captureStoredGroupExpansion reads the stored (uncloned) document's group
// expansion without taking a clone of the whole document.
func (inst *instance) captureStoredGroupExpansion(docID string) map[string]bool {
	if inst == nil || inst.manager == nil || docID == "" {
		return nil
	}
	var expanded map[string]bool
	inst.manager.Inspect(docID, func(stored *Document) {
		expanded = captureGroupExpansion(stored)
	})
	return expanded
}

// applyStoredGroupExpansion writes the flags back onto the stored document in
// place. Inspect hands back the stored pointer, which is what makes this reach
// the copy the manager actually serves.
func (inst *instance) applyStoredGroupExpansion(docID string, expanded map[string]bool) {
	if inst == nil || inst.manager == nil || docID == "" || len(expanded) == 0 {
		return
	}
	inst.manager.Inspect(docID, func(stored *Document) {
		applyGroupExpansion(stored, expanded)
	})
}
