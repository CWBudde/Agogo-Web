package engine

import (
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture/psdrecords"
)

// newGroupExpansionFixture builds a two-group document: one folder left open
// (the NewGroupLayer default) and one explicitly closed.
func newGroupExpansionFixture() *Document {
	doc := &Document{
		Width:      4,
		Height:     4,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		ID:         "fixture-group-expansion",
		Name:       "Group Expansion Fixture",
		CreatedAt:  "2026-09-17T10:00:00Z",
		CreatedBy:  "agogo-web-test",
		ModifiedAt: "2026-09-17T10:00:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}

	openChild := NewPixelLayer("Open Child", LayerBounds{X: 0, Y: 0, W: 4, H: 4}, filledPixels(4, 4, [4]byte{200, 40, 60, 255}))
	openGroup := NewGroupLayer("Open Folder")
	openGroup.Isolated = true
	openGroup.SetChildren([]LayerNode{openChild})

	closedChild := NewPixelLayer("Closed Child", LayerBounds{X: 0, Y: 0, W: 4, H: 4}, filledPixels(4, 4, [4]byte{40, 200, 60, 255}))
	closedGroup := NewGroupLayer("Closed Folder")
	closedGroup.Isolated = true
	closedGroup.Expanded = false
	closedGroup.SetChildren([]LayerNode{closedChild})

	doc.LayerRoot.SetChildren([]LayerNode{openGroup, closedGroup})
	doc.ActiveLayerID = openGroup.ID()
	return doc
}

func groupByName(t *testing.T, doc *Document, name string) *GroupLayer {
	t.Helper()
	var found *GroupLayer
	walkLayerTree(doc.ensureLayerRoot(), func(node LayerNode) {
		if group, ok := node.(*GroupLayer); ok && group.Name() == name {
			found = group
		}
	})
	if found == nil {
		t.Fatalf("group %q not found", name)
	}
	return found
}

// TestNewGroupLayerDefaultsToExpanded pins the default that every other
// assertion here leans on: Photoshop's default folder is the OPEN one (lsct 1),
// so a group the engine creates must not silently export as closed.
func TestNewGroupLayerDefaultsToExpanded(t *testing.T) {
	if !NewGroupLayer("Group").Expanded {
		t.Fatal("NewGroupLayer must default Expanded to true (PSD lsct 1)")
	}
}

// TestPSDRoundTripPreservesGroupExpandedAtRecordScope asserts the open/closed
// folder flag at RECORD scope. A model-scope assertion alone cannot see it: the
// same mistake in reader and writer cancels out, which is exactly how
// rgb8-group-closed-folder passed while psdexport was hardcoding lsct 1.
func TestPSDRoundTripPreservesGroupExpandedAtRecordScope(t *testing.T) {
	doc := newGroupExpansionFixture()

	data, err := SavePSD(doc)
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}

	records, err := psdrecords.Parse(data)
	if err != nil {
		t.Fatalf("parse re-exported records: %v", err)
	}
	sectionTypes := map[string]int{}
	for _, record := range records {
		if record.SectionType == 1 || record.SectionType == 2 {
			sectionTypes[record.Name] = record.SectionType
		}
	}
	if got := sectionTypes["Open Folder"]; got != 1 {
		t.Errorf("Open Folder section type = %d, want 1 (open folder)", got)
	}
	if got := sectionTypes["Closed Folder"]; got != 2 {
		t.Errorf("Closed Folder section type = %d, want 2 (closed folder)", got)
	}

	// The embedded Agogo project archive is deliberately bypassed, so this leg
	// exercises the spec parser rather than replaying the archive.
	reimported, _, err := LoadPSDWithOptions(data, PSDLoadOptions{IgnoreEmbeddedProject: true})
	if err != nil {
		t.Fatalf("LoadPSDWithOptions: %v", err)
	}
	if !groupByName(t, reimported, "Open Folder").Expanded {
		t.Error("re-imported Open Folder is collapsed; want expanded")
	}
	if groupByName(t, reimported, "Closed Folder").Expanded {
		t.Error("re-imported Closed Folder is expanded; want collapsed")
	}
}

// TestProjectArchiveRoundTripPreservesGroupExpanded covers the .agp leg.
func TestProjectArchiveRoundTripPreservesGroupExpanded(t *testing.T) {
	doc := newGroupExpansionFixture()

	raw, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	restored, _, err := LoadProject(raw)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if !groupByName(t, restored, "Open Folder").Expanded {
		t.Error("restored Open Folder is collapsed; want expanded")
	}
	if groupByName(t, restored, "Closed Folder").Expanded {
		t.Error("restored Closed Folder is expanded; want collapsed")
	}
	assertProjectArchiveEquivalent(t, restored, doc)
}

// TestProjectArchiveWithoutExpandedFieldDefaultsToExpanded protects existing
// .agp files: ArchiveVersion stays 1, so an archive written before the field
// existed must still load, and its folders must come back OPEN rather than all
// collapsed (which is what a plain `bool` with `omitempty` would have done).
func TestProjectArchiveWithoutExpandedFieldDefaultsToExpanded(t *testing.T) {
	legacy := []byte(`{
		"version": 1,
		"document": {
			"width": 4,
			"height": 4,
			"resolution": 72,
			"colorMode": "rgb",
			"bitDepth": 8,
			"background": {"kind": "transparent"},
			"id": "doc-legacy-groups",
			"name": "Legacy Groups",
			"createdAt": "2026-04-11T08:00:00Z",
			"createdBy": "agogo-web",
			"modifiedAt": "2026-04-11T08:00:00Z",
			"layers": [
				{
					"id": "group-legacy",
					"layerType": "group",
					"name": "Legacy Folder",
					"visible": true,
					"lockMode": "none",
					"opacity": 1,
					"fillOpacity": 1,
					"blendMode": "normal",
					"clipToBelow": false,
					"clippingBase": false
				}
			]
		}
	}`)

	restored, _, err := LoadProject(legacy)
	if err != nil {
		t.Fatalf("LoadProject(legacy archive): %v", err)
	}
	if !groupByName(t, restored, "Legacy Folder").Expanded {
		t.Error("a group from an archive with no \"expanded\" key must load as expanded")
	}
}

// TestSetGroupExpandedCommandIsNotUndoable records the design decision: group
// expansion is chrome that happens to be persisted, so it follows
// SetActiveLayer (no history entry) rather than executeDocCommand. It must
// still land on the active document.
func TestSetGroupExpandedCommandIsNotUndoable(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{LayerType: LayerTypeGroup, Name: "Folder"}))
	if err != nil {
		t.Fatalf("add group layer: %v", err)
	}
	groupID := added.UIMeta.ActiveLayerID

	inst := instances[h]
	group, ok := inst.manager.Active().findLayer(groupID).(*GroupLayer)
	if !ok {
		t.Fatalf("layer %q is not a group", groupID)
	}
	if !group.Expanded {
		t.Fatal("a newly added group must start expanded")
	}

	before := len(inst.history.Entries())
	if _, err := DispatchCommand(h, commandSetGroupExpanded, mustJSON(t, map[string]any{
		"layerId":  groupID,
		"expanded": false,
	})); err != nil {
		t.Fatalf("dispatch SetGroupExpanded: %v", err)
	}
	if collapsed, ok := inst.manager.Active().findLayer(groupID).(*GroupLayer); !ok || collapsed.Expanded {
		t.Fatal("SetGroupExpanded did not reach the active document")
	}
	if after := len(inst.history.Entries()); after != before {
		t.Fatalf("history entries = %d, want %d: group expansion must not be undoable", after, before)
	}

	if _, err := DispatchCommand(h, commandSetGroupExpanded, mustJSON(t, map[string]any{
		"layerId":  "missing-layer",
		"expanded": true,
	})); err == nil {
		t.Fatal("SetGroupExpanded on an unknown layer should fail")
	}
}
