package psdimport

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func rgbHeader() psdio.Header {
	return psdio.Header{Width: 4, Height: 4, Depth: 8, ColorMode: psdio.ColorModeRGB}
}

// adjustmentRecord is a Photoshop-shaped adjustment layer record: an empty rect
// and no colour channels. The shape is the point — this is what used to be
// dropped, because flattenLayerPixels has nothing to work with.
func adjustmentRecord(name, kind, params string) psdio.LayerRecord {
	return psdio.LayerRecord{
		Name:        name,
		Visible:     true,
		Opacity:     1,
		FillOpacity: 1,
		BlendMode:   model.BlendModeNormal,
		Adjustment: &psdio.AdjustmentPayload{
			Kind:   kind,
			Params: json.RawMessage(params),
		},
	}
}

// TestBuildLayerNodesReconstructsAdjustmentLayers is the regression this batch
// exists for: before it, a record with no colour channels failed
// flattenLayerPixels and the layer vanished with a "skipped" warning.
func TestBuildLayerNodesReconstructsAdjustmentLayers(t *testing.T) {
	record := adjustmentRecord("Levels", "levels", `{"gamma":1.2,"inputBlack":10}`)
	record.Opacity = 160.0 / 255.0
	record.BlendMode = model.BlendModeMultiply
	record.ClipToBelow = true

	nodes, warnings, err := BuildLayerNodes(rgbHeader(), []psdio.LayerRecord{record})
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}

	layer, ok := nodes[0].(*model.AdjustmentLayer)
	if !ok {
		t.Fatalf("node type = %T, want *model.AdjustmentLayer", nodes[0])
	}
	if layer.AdjustmentKind != "levels" {
		t.Errorf("AdjustmentKind = %q, want levels", layer.AdjustmentKind)
	}
	if string(layer.Params) != `{"gamma":1.2,"inputBlack":10}` {
		t.Errorf("Params = %s, want them carried through verbatim", layer.Params)
	}

	// The record's own fields are not the adjustment's, and an importer that
	// special-cases adjustment layers is exactly where they get dropped.
	if layer.BlendMode() != model.BlendModeMultiply {
		t.Errorf("BlendMode = %q, want multiply", layer.BlendMode())
	}
	if !layer.ClipToBelow() {
		t.Error("ClipToBelow = false, want true")
	}
	if got := layer.Opacity(); got < 0.62 || got > 0.63 {
		t.Errorf("Opacity = %v, want ~160/255", got)
	}
}

// TestZeroAreaRecordWithNoReconstructionIsReported pins the shape that used to
// be lost invisibly. A Photoshop adjustment layer has a zero rect, and
// flattenLayerPixels returns (nil, nil) for that rather than an error — so the
// layer arrived as an empty pixel layer with the right name and nothing else,
// and nothing anywhere said so.
//
// The record here carries a block the reader cannot reconstruct, so it still
// takes the raster path; what must not happen again is that it does so in
// silence.
func TestZeroAreaRecordWithNoReconstructionIsReported(t *testing.T) {
	record := psdio.LayerRecord{
		Name:              "Hue/Saturation 1",
		Visible:           true,
		Opacity:           1,
		BlendMode:         model.BlendModeNormal,
		UnsupportedBlocks: []string{"vibA"},
	}

	nodes, warnings, err := BuildLayerNodes(rgbHeader(), []psdio.LayerRecord{record})
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	if _, isPixel := nodes[0].(*model.PixelLayer); !isPixel {
		t.Fatalf("node type = %T, want *model.PixelLayer", nodes[0])
	}
	if len(warnings) == 0 {
		t.Fatal("an empty pixel layer was produced with no warning at all")
	}
	if !strings.Contains(warnings[0], "vibA") {
		t.Errorf("warning %q does not name the block that was lost", warnings[0])
	}
}

// TestAdjustmentLayerKeepsItsMask: a PSD adjustment layer can carry a mask, and
// the mask is what scopes the adjustment.
func TestAdjustmentLayerKeepsItsMask(t *testing.T) {
	record := adjustmentRecord("Curves", "curves", `{}`)
	record.HasLayerMask = true
	record.LayerMaskBounds = model.LayerBounds{X: 1, Y: 1, W: 2, H: 2}
	record.LayerMaskEnabled = true
	record.LayerMaskDefault = 255

	nodes, _, err := BuildLayerNodes(rgbHeader(), []psdio.LayerRecord{record})
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	layer, ok := nodes[0].(*model.AdjustmentLayer)
	if !ok {
		t.Fatalf("node type = %T, want *model.AdjustmentLayer", nodes[0])
	}
	if layer.Mask() == nil {
		t.Fatal("the adjustment layer lost its mask")
	}
}

// TestImportWarnsForEveryUnreconstructedBlock is the fallback contract: a block
// that did not become engine state is named, and the message says what actually
// happened to the layer it was on.
func TestImportWarnsForEveryUnreconstructedBlock(t *testing.T) {
	tests := []struct {
		name        string
		record      psdio.LayerRecord
		wantPhrases []string
	}{
		{
			name: "a raster layer keeps its pixels",
			record: func() psdio.LayerRecord {
				record := pixelRecord("Textured", 40)
				record.UnsupportedBlocks = []string{"SoCo"}
				return record
			}(),
			wantPhrases: []string{`layer "Textured": metadata block SoCo imported as a flattened pixel layer`},
		},
		{
			// A non-zero rect whose channels are missing is the case that
			// really fails: flattenLayerPixels returns "missing RGB channels"
			// and the layer is gone.
			name: "a layer with neither pixels nor a reconstruction is dropped",
			record: psdio.LayerRecord{
				Name:              "Gone",
				Bounds:            model.LayerBounds{W: 2, H: 2},
				Visible:           true,
				Opacity:           1,
				BlendMode:         model.BlendModeNormal,
				UnsupportedBlocks: []string{"GdFl"},
			},
			wantPhrases: []string{
				`layer "Gone" skipped`,
				`layer "Gone": metadata block GdFl not imported; the layer carried no pixels and was dropped`,
			},
		},
		{
			name: "a reconstructed layer still reports its other blocks",
			record: func() psdio.LayerRecord {
				record := adjustmentRecord("Levels", "levels", `{}`)
				record.UnsupportedBlocks = []string{"lnsr"}
				return record
			}(),
			wantPhrases: []string{
				`layer "Levels": metadata block lnsr not imported; the layer was reconstructed from another block`,
			},
		},
		{
			name: "a group reports its blocks too",
			record: psdio.LayerRecord{
				Name:              "Folder",
				SectionType:       psdio.LayerSectionOpenFolder,
				Visible:           true,
				Opacity:           1,
				BlendMode:         model.BlendModeNormal,
				UnsupportedBlocks: []string{"PtFl"},
			},
			wantPhrases: []string{
				`layer "Folder": metadata block PtFl not imported; the group itself was preserved`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			records := []psdio.LayerRecord{test.record}
			if test.record.SectionType == psdio.LayerSectionOpenFolder {
				records = []psdio.LayerRecord{
					{SectionType: psdio.LayerSectionBoundingDivider},
					pixelRecord("Child", 10),
					test.record,
				}
			}
			_, warnings, err := BuildLayerNodes(rgbHeader(), records)
			if err != nil {
				t.Fatalf("BuildLayerNodes: %v", err)
			}
			for _, phrase := range test.wantPhrases {
				if !slices.ContainsFunc(warnings, func(w string) bool {
					return strings.Contains(w, phrase)
				}) {
					t.Errorf("no warning contains %q\ngot: %v", phrase, warnings)
				}
			}
		})
	}
}

// TestLostAdjustmentFieldsAreWarnedAbout: a value the engine cannot hold is
// reported rather than rounded into a field that means something else.
func TestLostAdjustmentFieldsAreWarnedAbout(t *testing.T) {
	record := adjustmentRecord("Levels", "levels", `{}`)
	record.Adjustment.Lost = []string{"levl per-channel level records"}

	_, warnings, err := BuildLayerNodes(rgbHeader(), []psdio.LayerRecord{record})
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	want := `layer "Levels": levl per-channel level records has no engine equivalent and was not imported`
	if !slices.Contains(warnings, want) {
		t.Fatalf("warnings = %v, want one reading %q", warnings, want)
	}
}

// TestImportSmartObjectWarningNamesWhatWasLost pins item 4 of the batch: smart
// objects stay out of scope, but the warning has to be accurate about the cost.
// The generic wording was wrong in both directions — the layer's flattened
// composite DOES arrive, and the placed document does not.
func TestImportSmartObjectWarningNamesWhatWasLost(t *testing.T) {
	record := pixelRecord("Placed", 60)
	record.SmartObject = &psdio.SmartObjectMeta{Key: "SoLd"}

	_, warnings, err := BuildLayerNodes(rgbHeader(), []psdio.LayerRecord{record})
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	for _, phrase := range []string{"SoLd", "smart object", "flattened composite was imported", "placed source document"} {
		if !strings.Contains(warnings[0], phrase) {
			t.Errorf("warning %q does not mention %q", warnings[0], phrase)
		}
	}
}
