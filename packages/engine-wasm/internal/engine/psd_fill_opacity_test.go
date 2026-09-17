package engine

import (
	"bytes"
	"math"
	"testing"
)

// newFillOpacityFixture builds a document covering the three cases the iOpa
// block has to distinguish: a partial fill, a fully transparent fill, and a
// layer left at the default that must carry no block at all. The group leg is
// here rather than in the fixture corpus because psd-tools applies a group's
// fill opacity when compositing and Agogo does not (PLAN.md S.10.7), so only a
// model-scope assertion is honest about it.
func newFillOpacityFixture() *Document {
	doc := &Document{
		Width:      4,
		Height:     4,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		ID:         "fixture-fill-opacity",
		Name:       "Fill Opacity Fixture",
		CreatedAt:  "2026-09-17T10:00:00Z",
		CreatedBy:  "agogo-web-test",
		ModifiedAt: "2026-09-17T10:00:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}

	half := NewPixelLayer("Half Fill", LayerBounds{X: 0, Y: 0, W: 4, H: 4}, filledPixels(4, 4, [4]byte{200, 40, 60, 255}))
	half.SetFillOpacity(128.0 / 255.0)

	empty := NewPixelLayer("Empty Fill", LayerBounds{X: 0, Y: 0, W: 4, H: 4}, filledPixels(4, 4, [4]byte{40, 200, 60, 255}))
	empty.SetFillOpacity(0)

	plain := NewPixelLayer("Plain", LayerBounds{X: 0, Y: 0, W: 4, H: 4}, filledPixels(4, 4, [4]byte{60, 40, 200, 255}))

	child := NewPixelLayer("Group Child", LayerBounds{X: 0, Y: 0, W: 4, H: 4}, filledPixels(4, 4, [4]byte{200, 200, 40, 255}))
	group := NewGroupLayer("Quarter Fill Folder")
	group.Isolated = true
	group.SetFillOpacity(64.0 / 255.0)
	group.SetChildren([]LayerNode{child})

	doc.LayerRoot.SetChildren([]LayerNode{half, empty, plain, group})
	return doc
}

func fillOpacityByte(node LayerNode) int {
	return int(math.Round(node.FillOpacity() * 255))
}

func layerByName(t *testing.T, doc *Document, name string) LayerNode {
	t.Helper()
	var found LayerNode
	walkLayerTree(doc.ensureLayerRoot(), func(node LayerNode) {
		if node.Name() == name {
			found = node
		}
	})
	if found == nil {
		t.Fatalf("layer %q not found", name)
	}
	return found
}

// TestPSDRoundTripPreservesFillOpacity covers the read and write halves of the
// iOpa block together. IgnoreEmbeddedProject is what makes it real: without it
// the embedded AgogoProject archive would carry fill opacity past a writer that
// never emitted the block.
func TestPSDRoundTripPreservesFillOpacity(t *testing.T) {
	doc := newFillOpacityFixture()

	data, err := SavePSD(doc)
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}
	reimported, warnings, err := LoadPSDWithOptions(data, PSDLoadOptions{IgnoreEmbeddedProject: true})
	if err != nil {
		t.Fatalf("LoadPSDWithOptions: %v", err)
	}
	// iOpa used to reach psdimport as an unsupported block, one warning per
	// layer carrying it.
	if len(warnings) != 0 {
		t.Fatalf("import warnings = %q, want none", warnings)
	}

	testCases := []struct {
		name string
		want int
	}{
		{name: "Half Fill", want: 128},
		{name: "Empty Fill", want: 0},
		{name: "Plain", want: 255},
		{name: "Quarter Fill Folder", want: 64},
		{name: "Group Child", want: 255},
	}
	for _, testCase := range testCases {
		if got := fillOpacityByte(layerByName(t, reimported, testCase.name)); got != testCase.want {
			t.Errorf("%s fill opacity = %d, want %d", testCase.name, got, testCase.want)
		}
	}
}

// A re-import cannot tell an absent block from one carrying 255, so the
// omission has to be asserted on the bytes.
func TestSavePSDWritesIOpaOnlyBelowFullFillOpacity(t *testing.T) {
	data, err := SavePSD(newFillOpacityFixture())
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}
	if got, want := bytes.Count(data, []byte("iOpa")), 3; got != want {
		t.Fatalf("iOpa block count = %d, want %d (half fill, empty fill, folder)", got, want)
	}
}
