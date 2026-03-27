package engine

import (
	"testing"
)

func TestProjectZipRoundTripPreservesDocument(t *testing.T) {
	doc := &Document{
		Width: 4, Height: 2,
		Resolution: 72, ColorMode: "rgb", BitDepth: 8,
		Background: parseBackground("white"),
		ID:         "zip-test", Name: "Zip Test",
		CreatedAt: "2026-03-27T10:00:00Z", CreatedBy: "agogo-web",
		ModifiedAt: "2026-03-27T10:05:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}
	pixels := make([]byte, 4*2*4) // 4×2 RGBA
	for i := range pixels {
		pixels[i] = byte(i % 256)
	}
	layer := NewPixelLayer("BG", LayerBounds{X: 0, Y: 0, W: 4, H: 2}, pixels)
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()

	archive, err := SaveProjectZip(doc, nil)
	if err != nil {
		t.Fatalf("SaveProjectZip: %v", err)
	}
	if len(archive) == 0 {
		t.Fatal("SaveProjectZip returned empty bytes")
	}

	restored, _, err := LoadProjectZip(archive)
	if err != nil {
		t.Fatalf("LoadProjectZip: %v", err)
	}

	if restored.Width != doc.Width || restored.Height != doc.Height {
		t.Fatalf("size mismatch: got %dx%d want %dx%d", restored.Width, restored.Height, doc.Width, doc.Height)
	}
	restoredChildren := restored.LayerRoot.Children()
	if len(restoredChildren) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(restoredChildren))
	}
	pl, ok := restoredChildren[0].(*PixelLayer)
	if !ok {
		t.Fatalf("expected PixelLayer, got %T", restoredChildren[0])
	}
	if len(pl.Pixels) != len(pixels) {
		t.Fatalf("pixel count mismatch: got %d want %d", len(pl.Pixels), len(pixels))
	}
	for i, b := range pl.Pixels {
		if b != pixels[i] {
			t.Fatalf("pixel[%d] = %d, want %d", i, b, pixels[i])
		}
	}
}

func TestLoadProjectZipRejectsLegacyJSON(t *testing.T) {
	// LoadProjectZip should return an error on non-ZIP data
	_, _, err := LoadProjectZip([]byte(`{"version":1,"document":{"width":1}}`))
	if err == nil {
		t.Fatal("expected error on legacy JSON input, got nil")
	}
}
