package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestExportProjectZipRoundTripAndImportClearsHistory(t *testing.T) {
	exporter := Init("")
	defer Free(exporter)

	if _, err := DispatchCommand(exporter, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Export Fixture",
		Width:      4,
		Height:     4,
		Resolution: 144,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "transparent",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	base, err := DispatchCommand(exporter, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    filledPixels(4, 4, [4]byte{20, 40, 60, 255}),
	}))
	if err != nil {
		t.Fatalf("add base layer: %v", err)
	}
	baseID := base.UIMeta.ActiveLayerID

	if _, err := DispatchCommand(exporter, commandAddLayerMask, mustJSON(t, AddLayerMaskPayload{
		LayerID: baseID,
		Mode:    AddLayerMaskRevealAll,
	})); err != nil {
		t.Fatalf("add base mask: %v", err)
	}

	if _, err := DispatchCommand(exporter, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Title",
		Bounds:    LayerBounds{X: 1, Y: 1, W: 2, H: 1},
		Text:      "Hi",
		CachedRaster: []byte{
			255, 0, 0, 255,
			255, 0, 0, 255,
		},
	})); err != nil {
		t.Fatalf("add text layer: %v", err)
	}

	exported, err := ExportProject(exporter)
	if err != nil {
		t.Fatalf("ExportProject: %v", err)
	}
	if exported == "" {
		t.Fatal("ExportProject returned an empty payload")
	}

	archiveBytes, err := base64.StdEncoding.DecodeString(exported)
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}
	restoredDoc, history, err := LoadProjectZip(archiveBytes)
	if err != nil {
		t.Fatalf("LoadProjectZip: %v", err)
	}
	if restoredDoc.Name != "Export Fixture" {
		t.Fatalf("restored document name = %q, want Export Fixture", restoredDoc.Name)
	}
	if len(history) < 3 {
		t.Fatalf("restored history length = %d, want at least 3 entries", len(history))
	}

	importer := Init("")
	defer Free(importer)

	imported, err := ImportProject(importer, exported)
	if err != nil {
		t.Fatalf("ImportProject: %v", err)
	}
	if len(imported.UIMeta.History) != 0 {
		t.Fatalf("imported history length = %d, want 0", len(imported.UIMeta.History))
	}

	importedDoc := instances[importer].manager.Active()
	assertProjectArchiveEquivalent(t, importedDoc, restoredDoc)
	if imported.UIMeta.ActiveDocumentName != restoredDoc.Name {
		t.Fatalf("active document name = %q, want %q", imported.UIMeta.ActiveDocumentName, restoredDoc.Name)
	}
}

func TestImportProjectAcceptsLegacyJSONFallback(t *testing.T) {
	fixture := newRenderableProjectFixture()
	raw, err := SaveProject(fixture, []HistoryEntry{{Description: "legacy history"}})
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	h := Init("")
	defer Free(h)

	result, err := ImportProject(h, string(raw))
	if err != nil {
		t.Fatalf("ImportProject legacy JSON: %v", err)
	}
	if len(result.UIMeta.History) != 0 {
		t.Fatalf("imported history length = %d, want 0", len(result.UIMeta.History))
	}

	importedDoc := instances[h].manager.Active()
	assertProjectArchiveEquivalent(t, importedDoc, fixture)
	if result.UIMeta.ActiveLayerID != fixture.ActiveLayerID {
		t.Fatalf("active layer id = %q, want %q", result.UIMeta.ActiveLayerID, fixture.ActiveLayerID)
	}
}

func TestProjectIOErrorsForNilAndInvalidPayloads(t *testing.T) {
	var nilInst *instance
	if _, err := nilInst.exportProject(); err == nil {
		t.Fatal("expected exportProject to fail for nil instance")
	}
	if _, err := nilInst.importProject("irrelevant"); err == nil {
		t.Fatal("expected importProject to fail for nil instance")
	}

	inst := &instance{manager: newDocumentManager(), history: newHistoryStack(defaultHistoryMax)}
	if _, err := inst.exportProject(); err == nil {
		t.Fatal("expected exportProject to fail without an active document")
	}

	h := Init("")
	defer Free(h)

	if _, err := ImportProject(h, "definitely not a project archive"); err == nil {
		t.Fatal("expected ImportProject to reject invalid payload")
	} else if !strings.Contains(err.Error(), "load project") {
		t.Fatalf("ImportProject error = %q, want load project context", err)
	}
}

func TestSaveAndLoadProjectPreservesComplexLayerTree(t *testing.T) {
	fixture := newArchiveOnlyProjectFixture()
	history := []HistoryEntry{{Description: "Save archive"}, {Description: "More history"}}

	raw, err := SaveProject(fixture, history)
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	restored, restoredHistory, err := LoadProject(raw)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	assertProjectArchiveEquivalent(t, restored, fixture)
	if len(restoredHistory) != len(history) {
		t.Fatalf("restored history length = %d, want %d", len(restoredHistory), len(history))
	}
	if data, err := json.Marshal(restoredHistory); err != nil || len(data) == 0 {
		t.Fatalf("restored history should remain serializable, err=%v len=%d", err, len(data))
	}
}

func TestSaveAndLoadProjectPreservesExtendedAdjustmentLayerParams(t *testing.T) {
	doc := &Document{
		Width:      6,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		ID:         "extended-adjustments",
		Name:       "Extended Adjustments",
		CreatedAt:  "2026-03-27T10:00:00Z",
		CreatedBy:  "agogo-web-test",
		ModifiedAt: "2026-03-27T10:05:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}

	adjustments := []struct {
		name   string
		kind   string
		params string
	}{
		{name: "Channel Mixer", kind: "channel-mixer", params: `{"monochrome":true,"red":[0,0,100],"green":[0,100,0],"blue":[100,0,0]}`},
		{name: "Selective Color", kind: "selective-color", params: `{"mode":"absolute","blues":{"cyanRed":0,"magentaGreen":0,"yellowBlue":50,"black":-20}}`},
		{name: "Photo Filter", kind: "photo-filter", params: `{"color":[255,190,120,255],"density":40,"preserveLuminosity":true}`},
		{name: "Threshold", kind: "threshold", params: `{"threshold":70}`},
		{name: "Posterize", kind: "posterize", params: `{"levels":4}`},
		{name: "Invert", kind: "invert", params: `{}`},
		{name: "Gradient Map", kind: "gradient-map", params: `{"stops":[{"position":0,"color":[0,0,0,255]},{"position":1,"color":[255,0,0,255]}],"reverse":true}`},
	}

	children := make([]LayerNode, 0, len(adjustments))
	for _, adjustment := range adjustments {
		children = append(children, NewAdjustmentLayer(adjustment.name, adjustment.kind, json.RawMessage(adjustment.params)))
	}
	doc.LayerRoot.SetChildren(children)
	doc.normalizeClippingState()
	doc.ActiveLayerID = children[0].ID()

	raw, err := SaveProject(doc, []HistoryEntry{{Description: "save adjustments"}})
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	restored, history, err := LoadProject(raw)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(history) != 1 || history[0].Description != "save adjustments" {
		t.Fatalf("restored history = %+v, want one preserved entry", history)
	}
	if len(restored.LayerRoot.Children()) != len(adjustments) {
		t.Fatalf("restored child count = %d, want %d", len(restored.LayerRoot.Children()), len(adjustments))
	}
	for index, child := range restored.LayerRoot.Children() {
		adjustment, ok := child.(*AdjustmentLayer)
		if !ok {
			t.Fatalf("restored child %d = %T, want *AdjustmentLayer", index, child)
		}
		want := adjustments[index]
		if adjustment.Name() != want.name {
			t.Fatalf("restored name[%d] = %q, want %q", index, adjustment.Name(), want.name)
		}
		if adjustment.AdjustmentKind != want.kind {
			t.Fatalf("restored kind[%d] = %q, want %q", index, adjustment.AdjustmentKind, want.kind)
		}
		if string(adjustment.Params) != want.params {
			t.Fatalf("restored params[%d] = %s, want %s", index, string(adjustment.Params), want.params)
		}
	}

	assertProjectArchiveEquivalent(t, restored, doc)
}

func newRenderableProjectFixture() *Document {
	doc := &Document{
		Width:      4,
		Height:     4,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		ID:         "fixture-renderable",
		Name:       "Renderable Fixture",
		CreatedAt:  "2026-03-27T10:00:00Z",
		CreatedBy:  "agogo-web-test",
		ModifiedAt: "2026-03-27T10:05:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}

	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 4, H: 4}, filledPixels(4, 4, [4]byte{30, 40, 50, 255}))
	base.SetMask(&LayerMask{Enabled: true, Width: 4, Height: 4, Data: []byte{
		255, 255, 255, 255,
		255, 255, 255, 255,
		255, 0, 0, 255,
		255, 255, 255, 255,
	}})

	group := NewGroupLayer("Overlay Group")
	group.Isolated = true

	text := NewTextLayer("Title", LayerBounds{X: 1, Y: 1, W: 2, H: 1}, "Hi", []byte{
		255, 0, 0, 255,
		255, 0, 0, 255,
	})
	text.FontFamily = "Recursive"
	text.FontSize = 24
	text.SetBlendMode(BlendModeScreen)

	vector := NewVectorLayer("Shape", LayerBounds{X: 1, Y: 1, W: 2, H: 1}, &Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{{X: 1, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 2}}}}}, []byte{
		0, 0, 255, 128,
		0, 0, 255, 128,
	})
	vector.FillColor = [4]uint8{0, 0, 255, 255}
	vector.SetClipToBelow(true)
	vector.SetVectorMask(&Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}}}}})

	group.SetChildren([]LayerNode{text, vector})
	doc.LayerRoot.SetChildren([]LayerNode{base, group})
	doc.normalizeClippingState()
	doc.ActiveLayerID = vector.ID()
	return doc
}

func assertProjectArchiveEquivalent(t *testing.T, got, want *Document) {
	t.Helper()

	gotArchive, err := SaveProject(got, nil)
	if err != nil {
		t.Fatalf("SaveProject(got): %v", err)
	}
	wantArchive, err := SaveProject(want, nil)
	if err != nil {
		t.Fatalf("SaveProject(want): %v", err)
	}
	if !bytes.Equal(gotArchive, wantArchive) {
		t.Fatalf("project archives differ:\ngot=%s\nwant=%s", gotArchive, wantArchive)
	}
}

func newArchiveOnlyProjectFixture() *Document {
	doc := newRenderableProjectFixture()
	adjustment := NewAdjustmentLayer("Curves", "curves", json.RawMessage(`{"points":[[0,0],[255,255]]}`))
	adjustment.SetStyleStack([]LayerStyle{{Kind: "shadow", Enabled: true, Params: json.RawMessage(`{"distance":4}`)}})

	vector := NewVectorLayer("Archive Vector", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, &Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}}}}}, []byte{
		10, 20, 30, 255,
		40, 50, 60, 255,
		70, 80, 90, 255,
		100, 110, 120, 255,
	})
	vector.SetStyleStack([]LayerStyle{{Kind: "stroke", Enabled: true, Params: json.RawMessage(`{"width":2}`)}})

	group := NewGroupLayer("Archive Group")
	group.Isolated = true
	group.SetChildren([]LayerNode{adjustment, vector})

	rootChildren := doc.LayerRoot.Children()
	rootChildren = append(rootChildren, group)
	doc.LayerRoot.SetChildren(rootChildren)
	doc.normalizeClippingState()
	doc.ActiveLayerID = vector.ID()
	return doc
}
