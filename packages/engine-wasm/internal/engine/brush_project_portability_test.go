package engine

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"

	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

func TestImportedBrushTipFirstUseAddsDocumentScopedDependency(t *testing.T) {
	used := testBrushTipResource([]byte{255, 128, 64, 0})
	unused := testBrushTipResource([]byte{0, 64, 128, 255})
	preset := ImportedBrushPreset{
		ID: used.ID, Name: "Portable Sample", TipResourceID: used.ID,
		ThumbnailRGBA: "thumbnail", Warnings: []string{"fixture warning"},
	}
	inst := &instance{
		manager: newDocumentManager(), history: newHistoryStack(defaultHistoryMax),
		viewport:       ViewportState{Zoom: 1, CanvasW: 16, CanvasH: 16, DevicePixelRatio: 1},
		brushResources: map[string]*brushTipResource{used.ID: used, unused.ID: unused},
		brushPresets:   map[string]ImportedBrushPreset{used.ID: preset},
	}
	doc := testDocumentFixture("brush-portable", "Portable Brush", 16, 16)
	layer := NewPixelLayer("Paint", LayerBounds{W: 16, H: 16}, make([]byte, 16*16*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{
		X: 8, Y: 8, Pressure: 1,
		Brush: BrushParams{
			Size: 4, Hardness: 1, Flow: 1,
			Color: [4]uint8{12, 34, 56, 255}, TipResourceID: used.ID,
		},
	})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end sampled brush stroke: %v", err)
	}

	stored := inst.manager.activeMut()
	if stored.BrushResources[used.ID] == nil {
		t.Fatal("used imported tip was not added to the document")
	}
	if stored.BrushResources[unused.ID] != nil {
		t.Fatal("unused imported tip was added to the document")
	}
	if got := stored.BrushPresets[used.ID]; got.ID != preset.ID || got.Name != preset.Name || got.ThumbnailRGBA != preset.ThumbnailRGBA {
		t.Fatalf("document preset metadata = %+v, want %+v", got, preset)
	}
	if stored.BrushResources[used.ID] == inst.brushResources[used.ID] {
		t.Fatal("document must own a distinct brush-tip resource")
	}
	inst.brushResources[used.ID].Alpha[0] = 1
	inst.brushPresets[used.ID] = ImportedBrushPreset{ID: "mutated", TipResourceID: used.ID}
	if stored.BrushResources[used.ID].Alpha[0] != 255 || stored.BrushPresets[used.ID].ID != preset.ID {
		t.Fatal("document brush dependency changed with the instance registry")
	}
}

func TestCloneAndDocumentsEqualIncludeBrushDependencies(t *testing.T) {
	resource := testBrushTipResource([]byte{255, 192, 128, 64})
	doc := testDocumentFixture("brush-clone", "Brush Clone", 2, 2)
	doc.BrushResources = map[string]*brushTipResource{resource.ID: resource}
	doc.BrushPresets = map[string]ImportedBrushPreset{
		resource.ID: {
			ID: resource.ID, Name: "Clone Sample", TipResourceID: resource.ID,
			Warnings: []string{"preserved"},
		},
	}

	cloned := cloneDocument(doc)
	if !documentsEqual(doc, cloned) {
		t.Fatal("clone with identical brush dependencies should compare equal")
	}
	if cloned.BrushResources[resource.ID] == doc.BrushResources[resource.ID] {
		t.Fatal("cloneDocument must deep-copy brush-tip resources")
	}
	cloned.BrushResources[resource.ID].Alpha[0] = 0
	if documentsEqual(doc, cloned) {
		t.Fatal("documentsEqual did not detect brush-tip byte mutation")
	}
	cloned = cloneDocument(doc)
	preset := cloned.BrushPresets[resource.ID]
	preset.Warnings[0] = "changed"
	cloned.BrushPresets[resource.ID] = preset
	if documentsEqual(doc, cloned) {
		t.Fatal("documentsEqual did not detect brush preset metadata mutation")
	}
}

func TestProjectArchivesEmbedReferencedBrushDependencies(t *testing.T) {
	resource := testBrushTipResource([]byte{255, 160, 80, 0})
	doc := portableBrushProjectFixture(resource)

	legacy, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	legacyRestored, _, err := LoadProject(legacy)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if !docpkg.BrushTipResourcesEqual(doc.BrushResources, legacyRestored.BrushResources) || !docpkg.ImportedBrushPresetsEqual(doc.BrushPresets, legacyRestored.BrushPresets) {
		t.Fatal("legacy project round trip did not preserve brush dependencies")
	}
	assertProjectArchiveEquivalent(t, legacyRestored, doc)

	data, err := SaveProjectZip(doc, nil)
	if err != nil {
		t.Fatalf("SaveProjectZip: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open project zip: %v", err)
	}
	var manifest []byte
	brushEntries := []string(nil)
	for _, file := range zr.File {
		switch {
		case file.Name == "manifest.json":
			rc, openErr := file.Open()
			if openErr != nil {
				t.Fatalf("open manifest: %v", openErr)
			}
			manifest, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("read manifest: %v", err)
			}
		case strings.HasPrefix(file.Name, "brushes/"):
			brushEntries = append(brushEntries, file.Name)
		}
	}
	wantEntry := "brushes/" + resource.ID + ".alpha.bin"
	if len(brushEntries) != 1 || brushEntries[0] != wantEntry {
		t.Fatalf("brush zip entries = %v, want [%s]", brushEntries, wantEntry)
	}
	var archive ProjectArchive
	if err := json.Unmarshal(manifest, &archive); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if len(archive.Document.BrushTips) != 1 || len(archive.Document.BrushPresets) != 1 {
		t.Fatalf("manifest brush dependencies = %d tips, %d presets", len(archive.Document.BrushTips), len(archive.Document.BrushPresets))
	}
	if len(archive.Document.BrushTips[0].Alpha) != 0 {
		t.Fatal("brush alpha should be stored as a ZIP blob, not inline in the manifest")
	}

	zipRestored, _, err := LoadProjectZip(data)
	if err != nil {
		t.Fatalf("LoadProjectZip: %v", err)
	}
	if !docpkg.BrushTipResourcesEqual(doc.BrushResources, zipRestored.BrushResources) || !docpkg.ImportedBrushPresetsEqual(doc.BrushPresets, zipRestored.BrushPresets) {
		t.Fatal("ZIP project round trip did not preserve brush dependencies")
	}
	assertProjectArchiveEquivalent(t, zipRestored, doc)
}

func TestImportProjectReregistersEmbeddedBrushDependencies(t *testing.T) {
	resource := testBrushTipResource([]byte{0, 90, 180, 255})
	doc := portableBrushProjectFixture(resource)
	data, err := SaveProjectZip(doc, nil)
	if err != nil {
		t.Fatalf("SaveProjectZip: %v", err)
	}

	handle := Init("")
	defer Free(handle)
	if _, err := ImportProject(handle, base64.StdEncoding.EncodeToString(data)); err != nil {
		t.Fatalf("ImportProject: %v", err)
	}
	inst := instances[handle]
	imported := inst.manager.Active()
	registered := inst.brushResources[resource.ID]
	if registered == nil || !bytes.Equal(registered.Alpha, resource.Alpha) {
		t.Fatal("embedded brush tip was not re-registered on project import")
	}
	if registered == imported.BrushResources[resource.ID] {
		t.Fatal("instance registry must not alias the imported document resource")
	}
	if got := inst.brushPresets[resource.ID]; got.Name != "Portable Sample" || got.TipResourceID != resource.ID {
		t.Fatalf("re-registered preset metadata = %+v", got)
	}

	// Prove the restored instance-level registry can seed another document.
	other := testDocumentFixture("other-brush-doc", "Other", 2, 2)
	resolved := inst.documentBrushResource(other, resource.ID)
	if resolved == nil || other.BrushResources[resource.ID] == nil || other.BrushPresets[resource.ID].Name != "Portable Sample" {
		t.Fatal("re-registered embedded brush dependency was not reusable")
	}
}

func testBrushTipResource(alpha []byte) *brushTipResource {
	resource := &brushTipResource{Width: 2, Height: 2, Alpha: append([]byte(nil), alpha...)}
	resource.ID = brushTipResourceID(resource.Width, resource.Height, resource.Alpha)
	return resource
}

func portableBrushProjectFixture(resource *brushTipResource) *Document {
	doc := testDocumentFixture("portable-brush-project", "Portable Brush Project", 2, 2)
	doc.BrushResources = map[string]*brushTipResource{resource.ID: resource}
	doc.BrushPresets = map[string]ImportedBrushPreset{
		resource.ID: {
			ID: resource.ID, Name: "Portable Sample", TipResourceID: resource.ID,
			ThumbnailRGBA: "thumbnail-rgba", TipShape: "round", Size: 48,
			Hardness: 0.65, Spacing: 0.18, Angle: 32, Roundness: 0.72,
			SizeJitter: 0.25, OpacityJitter: 0.15, FlowJitter: 0.05,
			ControlSource: "pressure", FadeDabs: 240, Warnings: []string{"fixture warning"},
		},
	}
	return doc
}
