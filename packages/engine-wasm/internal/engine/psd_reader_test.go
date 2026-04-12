package engine

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"io"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestLoadPSDImportsCompositeImageAndResolution(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:      1,
		height:     1,
		channels:   3,
		resolution: 300,
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{255},
				{0},
				{0},
			},
		},
	})

	doc, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if doc.Width != 1 || doc.Height != 1 {
		t.Fatalf("doc size = %dx%d, want 1x1", doc.Width, doc.Height)
	}
	if doc.ColorMode != "rgb" {
		t.Fatalf("doc color mode = %q, want rgb", doc.ColorMode)
	}
	if doc.BitDepth != 8 {
		t.Fatalf("doc bit depth = %d, want 8", doc.BitDepth)
	}
	if doc.Resolution != 300 {
		t.Fatalf("doc resolution = %v, want 300", doc.Resolution)
	}

	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	layer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if layer.Name() != "Background" {
		t.Fatalf("layer name = %q, want Background", layer.Name())
	}
	if !bytes.Equal(layer.Pixels, []byte{255, 0, 0, 255}) {
		t.Fatalf("layer pixels = %v, want red RGBA pixel", layer.Pixels)
	}
}

func TestLoadPSDImportsLayerRasterDataAndWarnsForUnsupportedTextMetadata(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Pixel",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRLE, data: []byte{10}},
					{id: 1, compression: psdCompressionRLE, data: []byte{20}},
					{id: 2, compression: psdCompressionRLE, data: []byte{30}},
					{id: -1, compression: psdCompressionRLE, data: []byte{255}},
				},
			},
			{
				name: "Title",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				extraBlocks: []psdTaggedBlock{
					{signature: "8BIM", key: "TySh", data: buildTypeToolInfoBlock()},
				},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{200}},
					{id: 1, compression: psdCompressionRaw, data: []byte{150}},
					{id: 2, compression: psdCompressionRaw, data: []byte{100}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{200},
				{150},
				{100},
			},
		},
	})

	doc, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one warning", warnings)
	}
	if !strings.Contains(warnings[0], "TySh") {
		t.Fatalf("warning = %q, want TySh context", warnings[0])
	}

	children := doc.LayerRoot.Children()
	if len(children) != 2 {
		t.Fatalf("imported child count = %d, want 2", len(children))
	}

	first, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("first layer = %T, want *PixelLayer", children[0])
	}
	if first.Name() != "Pixel" {
		t.Fatalf("first layer name = %q, want Pixel", first.Name())
	}
	if !bytes.Equal(first.Pixels, []byte{10, 20, 30, 255}) {
		t.Fatalf("first layer pixels = %v, want [10 20 30 255]", first.Pixels)
	}

	second, ok := children[1].(*PixelLayer)
	if !ok {
		t.Fatalf("second layer = %T, want *PixelLayer fallback", children[1])
	}
	if second.Name() != "Title" {
		t.Fatalf("second layer name = %q, want Title", second.Name())
	}
	if !bytes.Equal(second.Pixels, []byte{200, 150, 100, 255}) {
		t.Fatalf("second layer pixels = %v, want [200 150 100 255]", second.Pixels)
	}
}

func TestLoadPSDParsesImageResourceMetadataWithoutBlockingImport(t *testing.T) {
	testCases := []struct {
		name string
		id   uint16
		data []byte
	}{
		{name: "ICC profile", id: psdImageResourceICCProfile, data: []byte("icc-profile")},
		{name: "guides", id: psdImageResourceGuides, data: []byte("guide-data")},
		{name: "slices", id: psdImageResourceSlices, data: []byte("slice-data")},
		{name: "layer comps", id: psdImageResourceLayerComps, data: []byte("layer-comps")},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := buildMinimalPSD(t, minimalPSDConfig{
				width:      1,
				height:     1,
				channels:   3,
				resolution: 72,
				resources: []psdImageResource{
					{id: tc.id, name: "meta", data: tc.data},
				},
				composite: psdImageData{
					compression: psdCompressionRaw,
					planes: [][]byte{
						{0},
						{0},
						{0},
					},
				},
			})
			doc, warnings, err := LoadPSD(data)
			if err != nil {
				t.Fatalf("LoadPSD: %v", err)
			}
			if len(warnings) != 0 {
				t.Fatalf("unexpected warnings = %v", warnings)
			}
			if len(doc.LayerRoot.Children()) != 1 {
				t.Fatalf("imported child count = %d, want 1", len(doc.LayerRoot.Children()))
			}
		})
	}
}

func TestLoadPSDParsesSectionDividerBlocksIntoGroups(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Group",
				rect: LayerBounds{X: 0, Y: 0, W: 0, H: 0},
				extraBlocks: []psdTaggedBlock{
					{signature: "8BIM", key: "lsct", data: buildSectionDividerData(psdLayerSectionOpenFolder)},
				},
			},
			{
				name: "Child",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{10}},
					{id: 1, compression: psdCompressionRaw, data: []byte{20}},
					{id: 2, compression: psdCompressionRaw, data: []byte{30}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
			{
				name: "Group End",
				rect: LayerBounds{X: 0, Y: 0, W: 0, H: 0},
				extraBlocks: []psdTaggedBlock{
					{signature: "8BIM", key: "lsct", data: buildSectionDividerData(psdLayerSectionCloseFolder)},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{0},
				{0},
				{0},
			},
		},
	})
	doc, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings = %v", warnings)
	}
	rootChildren := doc.LayerRoot.Children()
	if len(rootChildren) != 1 {
		t.Fatalf("root child count = %d, want 1", len(rootChildren))
	}
	group, ok := rootChildren[0].(*GroupLayer)
	if !ok {
		t.Fatalf("root child type = %T, want *GroupLayer", rootChildren[0])
	}
	groupChildren := group.Children()
	if len(groupChildren) != 1 {
		t.Fatalf("group child count = %d, want 1", len(groupChildren))
	}
	if _, ok := groupChildren[0].(*PixelLayer); !ok {
		t.Fatalf("group child type = %T, want *PixelLayer", groupChildren[0])
	}
}

func TestLoadPSDParsesLayerMaskMetadata(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Masked",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{10}},
					{id: 1, compression: psdCompressionRaw, data: []byte{20}},
					{id: 2, compression: psdCompressionRaw, data: []byte{30}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{10},
				{20},
				{30},
			},
		},
	})
	doc, _, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("child count = %d, want 1", len(children))
	}
	layer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("root child type = %T, want *PixelLayer", children[0])
	}
	mask := layer.Mask()
	if mask == nil {
		t.Fatal("expected parsed layer mask")
	}
	if mask.Width != 1 || mask.Height != 1 {
		t.Fatalf("mask size = %dx%d, want 1x1", mask.Width, mask.Height)
	}
}

func TestParsePSDLayerExtraDataCapturesLayerEffectsMetadata(t *testing.T) {
	extraData := buildLayerExtraData(t, []psdTaggedBlock{
		{
			signature: "8BIM",
			key:       "lrFX",
			data:      buildLayerLegacyEffectsBlock([]string{"drSh", "dsSh"}),
		},
		{
			signature: "8BIM",
			key:       "lfx2",
			data:      buildLayerObjectEffectsBlock(2, 3),
		},
		{
			signature: "8BIM",
			key:       "levl",
			data:      buildLayerAdjustmentPayload(0),
		},
		{
			signature: "8BIM",
			key:       "PlLd",
			data:      buildLayerSmartObjectPayload("uid-1"),
		},
	})

	var record psdLayerRecord
	if err := parsePSDLayerExtraData(extraData, &record); err != nil {
		t.Fatalf("parsePSDLayerExtraData: %v", err)
	}
	if record.Effects == nil || record.Effects.Legacy == nil {
		t.Fatal("expected legacy effects metadata")
	}
	if got, want := len(record.Effects.Legacy.EffectKeys), 2; got != want {
		t.Fatalf("legacy effect count = %d, want %d", got, want)
	}
	if record.Effects.Legacy.EffectCount != 2 {
		t.Fatalf("legacy effect count field = %d, want 2", record.Effects.Legacy.EffectCount)
	}
	if record.Effects.Object == nil {
		t.Fatal("expected object effects metadata")
	}
	if got, want := len(record.Adjustments), 1; got != want {
		t.Fatalf("adjustment metadata count = %d, want 1", got)
	}
	if record.Adjustments[0].Kind != "levels" {
		t.Fatalf("adjustment kind = %q, want levels", record.Adjustments[0].Kind)
	}
	if record.SmartObject == nil {
		t.Fatal("expected smart object metadata")
	}
	if record.SmartObject.Identifier != "uid-1" {
		t.Fatalf("smart object identifier = %q, want uid-1", record.SmartObject.Identifier)
	}
}

func TestParsePSDLayerExtraDataMapsObjectEffectKeysToStyleStack(t *testing.T) {
	extraData := buildLayerExtraData(t, []psdTaggedBlock{
		{
			signature: "8BIM",
			key:       "lfx2",
			data:      buildLayerObjectEffectsBlockWithKeys(2, 3, []string{"drSh", "outerGlow"}),
		},
	})

	var record psdLayerRecord
	if err := parsePSDLayerExtraData(extraData, &record); err != nil {
		t.Fatalf("parsePSDLayerExtraData: %v", err)
	}
	styles := record.Effects.GetStyleStack()
	if len(styles) != 2 {
		t.Fatalf("effect style stack size = %d, want 2", len(styles))
	}
	seen := map[string]bool{}
	for _, style := range styles {
		seen[style.Kind] = true
		if style.Enabled {
			t.Fatalf("style %q should import disabled", style.Kind)
		}
	}
	if !seen[string(LayerStyleKindDropShadow)] {
		t.Fatal("expected drop shadow style in object metadata")
	}
	if !seen[string(LayerStyleKindOuterGlow)] {
		t.Fatal("expected outer glow style in object metadata")
	}
}

func TestParsePSDLayerExtraDataCapturesVectorMaskMetadata(t *testing.T) {
	extraData := buildLayerExtraData(t, []psdTaggedBlock{
		{
			signature: "8BIM",
			key:       "vmsk",
			data:      buildLayerVectorMaskPayload(LayerBounds{X: 1, Y: 2, W: 3, H: 4}),
		},
	})

	var record psdLayerRecord
	if err := parsePSDLayerExtraData(extraData, &record); err != nil {
		t.Fatalf("parsePSDLayerExtraData: %v", err)
	}
	if !record.HasVectorMask {
		t.Fatal("expected vector mask flag")
	}
	if record.VectorMask == nil {
		t.Fatal("expected vector mask metadata")
	}
	if record.VectorMask.Bounds.W != 3 || record.VectorMask.Bounds.H != 4 {
		t.Fatalf("vector mask bounds = %dx%d, want 3x4", record.VectorMask.Bounds.W, record.VectorMask.Bounds.H)
	}
	if record.VectorMask.Bounds.X != 1 || record.VectorMask.Bounds.Y != 2 {
		t.Fatalf("vector mask origin = (%d,%d), want (1,2)", record.VectorMask.Bounds.X, record.VectorMask.Bounds.Y)
	}
}

func TestParsePSDLayerExtraDataCapturesTextMetadata(t *testing.T) {
	extraData := buildLayerExtraData(t, []psdTaggedBlock{
		{
			signature: "8BIM",
			key:       "TySh",
			data:      buildLayerTextPayload("Layer Title"),
		},
	})

	var record psdLayerRecord
	if err := parsePSDLayerExtraData(extraData, &record); err != nil {
		t.Fatalf("parsePSDLayerExtraData: %v", err)
	}
	if record.Text == nil {
		t.Fatal("expected text layer metadata")
	}
	if !record.Text.HasDescriptor {
		t.Fatal("expected text descriptor metadata")
	}
	if record.Text.ParsedText != "Layer Title" {
		t.Fatalf("parsed text = %q, want Layer Title", record.Text.ParsedText)
	}
}

func TestParsePSDLayerExtraDataCapturesSmartObjectMetadata(t *testing.T) {
	extraData := buildLayerExtraData(t, []psdTaggedBlock{
		{
			signature: "8BIM",
			key:       "SoLd",
			data:      buildLayerSmartObjectPayloadWithSoLd("uid-2", 2, 3, 1),
		},
	})

	var record psdLayerRecord
	if err := parsePSDLayerExtraData(extraData, &record); err != nil {
		t.Fatalf("parsePSDLayerExtraData: %v", err)
	}
	if record.SmartObject == nil {
		t.Fatal("expected smart object metadata")
	}
	if record.SmartObject.PageNumber == nil {
		t.Fatal("expected smart object page number")
	}
	if got, want := *record.SmartObject.PageNumber, uint32(2); got != want {
		t.Fatalf("smart object page = %d, want %d", got, want)
	}
	if record.SmartObject.TotalPages == nil || *record.SmartObject.TotalPages != 3 {
		t.Fatalf("smart object total pages = %v, want 3", record.SmartObject.TotalPages)
	}
}

func TestLoadPSDImportsLegacyLayerEffectsAsDisabledStyles(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Styled",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				extraBlocks: []psdTaggedBlock{
					{
						signature: "8BIM",
						key:       "lrFX",
						data:      buildLayerLegacyEffectsBlock([]string{"drSh"}),
					},
				},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{10}},
					{id: 1, compression: psdCompressionRaw, data: []byte{20}},
					{id: 2, compression: psdCompressionRaw, data: []byte{30}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{10},
				{20},
				{30},
			},
		},
	})

	doc, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}

	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	layer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	styles := layer.StyleStack()
	if len(styles) != 1 {
		t.Fatalf("style stack size = %d, want 1", len(styles))
	}
	if got, want := styles[0].Kind, string(LayerStyleKindDropShadow); got != want {
		t.Fatalf("style kind = %q, want %q", got, want)
	}
	if styles[0].Enabled {
		t.Fatal("expected imported legacy effect style to be disabled")
	}
}

func TestLoadPSDParsesMalformedLayerEffectsMetadataAsWarning(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Effects",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				extraBlocks: []psdTaggedBlock{
					{
						signature: "8BIM",
						key:       "lfx2",
						data:      []byte{0x01},
					},
					{
						signature: "8BIM",
						key:       "lrFX",
						data:      []byte{0x00, 0x01},
					},
				},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{11}},
					{id: 1, compression: psdCompressionRaw, data: []byte{22}},
					{id: 2, compression: psdCompressionRaw, data: []byte{33}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{11},
				{22},
				{33},
			},
		},
	})
	_, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for malformed layer effects metadata")
	}
	if !strings.Contains(strings.Join(warnings, " "), "lfx2") && !strings.Contains(strings.Join(warnings, " "), "lrFX") {
		t.Fatalf("expected malformed layer effects warning, got %v", warnings)
	}
}

func TestLoadPSDPartialCompositeErrorReportsWarning(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Only",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{40}},
					{id: 1, compression: psdCompressionRaw, data: []byte{50}},
					{id: 2, compression: psdCompressionRaw, data: []byte{60}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: 99,
			planes: [][]byte{
				{40},
				{50},
				{60},
			},
		},
	})
	doc, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for unsupported composite compression")
	}
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("child count = %d, want 1", len(children))
	}
}

func TestImportProjectAcceptsBase64PSD(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{0},
				{255},
				{0},
			},
		},
	})

	h := Init("")
	defer Free(h)

	result, err := ImportProject(h, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatalf("ImportProject: %v", err)
	}
	if result.UIMeta.ActiveDocumentName == "" {
		t.Fatal("active document name should not be empty after PSD import")
	}
	if result.UIMeta.DocumentWidth != 1 || result.UIMeta.DocumentHeight != 1 {
		t.Fatalf("imported dimensions = %dx%d, want 1x1", result.UIMeta.DocumentWidth, result.UIMeta.DocumentHeight)
	}

	doc := instances[h].manager.Active()
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	layer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if !bytes.Equal(layer.Pixels, []byte{0, 255, 0, 255}) {
		t.Fatalf("layer pixels = %v, want green RGBA pixel", layer.Pixels)
	}
}

func TestImportProjectSurfacesPSDWarnings(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Title",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				extraBlocks: []psdTaggedBlock{
					{signature: "8BIM", key: "TySh", data: buildTypeToolInfoBlock()},
				},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{40}},
					{id: 1, compression: psdCompressionRaw, data: []byte{50}},
					{id: 2, compression: psdCompressionRaw, data: []byte{60}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{40},
				{50},
				{60},
			},
		},
	})

	h := Init("")
	defer Free(h)

	result, err := ImportProject(h, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatalf("ImportProject: %v", err)
	}
	if len(result.UIMeta.ImportWarnings) != 1 {
		t.Fatalf("import warnings = %v, want single warning", result.UIMeta.ImportWarnings)
	}
	if !strings.Contains(result.UIMeta.ImportWarnings[0], "TySh") {
		t.Fatalf("warning = %q, want TySh context", result.UIMeta.ImportWarnings[0])
	}
}

func TestLoadPSDImportsZipPredictionLayerData(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    2,
		height:   2,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Predicted",
				rect: LayerBounds{X: 0, Y: 0, W: 2, H: 2},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionZipPrediction, data: []byte{20, 10, 30, 40}},
					{id: 1, compression: psdCompressionZipPrediction, data: []byte{5, 8, 12, 16}},
					{id: 2, compression: psdCompressionZipPrediction, data: []byte{60, 70, 80, 90}},
					{id: -1, compression: psdCompressionZipPrediction, data: []byte{255, 255, 255, 255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{20, 10, 30, 40},
				{5, 8, 12, 16},
				{60, 70, 80, 90},
			},
		},
	})

	doc, _, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if doc.Width != 2 || doc.Height != 2 {
		t.Fatalf("doc size = %dx%d, want 2x2", doc.Width, doc.Height)
	}
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	pixelLayer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if !bytes.Equal(pixelLayer.Pixels, []byte{
		20, 5, 60, 255,
		10, 8, 70, 255,
		30, 12, 80, 255,
		40, 16, 90, 255,
	}) {
		t.Fatalf("unexpected pixels: %v", pixelLayer.Pixels)
	}
}

func TestLoadPSDImportsZipCompositeData(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    2,
		height:   2,
		channels: 3,
		composite: psdImageData{
			compression: psdCompressionZip,
			planes: [][]byte{
				{20, 5, 30, 40},
				{10, 8, 12, 16},
				{60, 70, 80, 90},
			},
		},
	})
	doc, _, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	pixelLayer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if !bytes.Equal(pixelLayer.Pixels, []byte{
		20, 10, 60, 255,
		5, 8, 70, 255,
		30, 12, 80, 255,
		40, 16, 90, 255,
	}) {
		t.Fatalf("unexpected pixels: %v", pixelLayer.Pixels)
	}
}

type minimalPSDConfig struct {
	width      int
	height     int
	channels   int
	resolution int
	resources  []psdImageResource
	layers     []minimalPSDLayer
	composite  psdImageData
}

type psdImageResource struct {
	id   uint16
	name string
	data []byte
}

type minimalPSDLayer struct {
	name        string
	rect        LayerBounds
	channels    []psdLayerChannel
	extraBlocks []psdTaggedBlock
	maskData    []byte
	blendRanges []byte
}

type psdLayerChannel struct {
	id          int16
	compression uint16
	data        []byte
}

type psdImageData struct {
	compression uint16
	planes      [][]byte
}

type psdTaggedBlock struct {
	signature string
	key       string
	data      []byte
}

func buildMinimalPSD(t *testing.T, cfg minimalPSDConfig) []byte {
	t.Helper()

	if cfg.width <= 0 || cfg.height <= 0 {
		t.Fatalf("invalid PSD dimensions: %dx%d", cfg.width, cfg.height)
	}
	if cfg.channels <= 0 {
		t.Fatal("PSD must have at least one channel")
	}

	var out bytes.Buffer
	out.WriteString("8BPS")
	writeUint16(&out, 1)
	out.Write(make([]byte, 6))
	writeUint16(&out, uint16(cfg.channels))
	writeUint32(&out, uint32(cfg.height))
	writeUint32(&out, uint32(cfg.width))
	writeUint16(&out, 8)
	writeUint16(&out, 3)

	writeUint32(&out, 0)

	var resources bytes.Buffer
	if cfg.resolution > 0 {
		writeImageResourceBlock(&resources, 0x03ed, nil, buildResolutionInfoBlock(cfg.resolution))
	}
	for _, resource := range cfg.resources {
		writeImageResourceBlock(&resources, resource.id, []byte(resource.name), resource.data)
	}
	writeUint32(&out, uint32(resources.Len()))
	out.Write(resources.Bytes())

	layerMask := buildLayerAndMaskSection(t, cfg.layers)
	writeUint32(&out, uint32(len(layerMask)))
	out.Write(layerMask)

	out.Write(buildImageDataSection(t, cfg.composite, cfg.width, cfg.height))
	return out.Bytes()
}

func buildLayerAndMaskSection(t *testing.T, layers []minimalPSDLayer) []byte {
	t.Helper()

	var layerRecords bytes.Buffer
	var channelData bytes.Buffer
	writeInt16(&layerRecords, int16(len(layers)))
	for _, layer := range layers {
		top := layer.rect.Y
		left := layer.rect.X
		bottom := layer.rect.Y + layer.rect.H
		right := layer.rect.X + layer.rect.W
		writeUint32(&layerRecords, uint32(top))
		writeUint32(&layerRecords, uint32(left))
		writeUint32(&layerRecords, uint32(bottom))
		writeUint32(&layerRecords, uint32(right))
		writeUint16(&layerRecords, uint16(len(layer.channels)))

		encodedChannels := make([][]byte, len(layer.channels))
		for i, channel := range layer.channels {
			writeInt16(&layerRecords, channel.id)
			encoded := buildChannelImageData(t, channel.compression, channel.data, layer.rect.W, layer.rect.H)
			encodedChannels[i] = encoded
			writeUint32(&layerRecords, uint32(len(encoded)))
		}

		layerRecords.WriteString("8BIM")
		layerRecords.WriteString("norm")
		layerRecords.WriteByte(255)
		layerRecords.WriteByte(0)
		layerRecords.WriteByte(0)
		layerRecords.WriteByte(0)

		var extra bytes.Buffer
		if len(layer.maskData) == 0 {
			layer.maskData = buildLayerMaskData(t, layer.rect)
		}
		if len(layer.blendRanges) == 0 {
			layer.blendRanges = buildLayerBlendRangeData()
		}
		writeUint32(&extra, uint32(len(layer.maskData)))
		extra.Write(layer.maskData)
		writeUint32(&extra, uint32(len(layer.blendRanges)))
		extra.Write(layer.blendRanges)
		writePascalString4(&extra, layer.name)
		for _, block := range layer.extraBlocks {
			writeTaggedBlock(&extra, block)
		}
		writeUint32(&layerRecords, uint32(extra.Len()))
		layerRecords.Write(extra.Bytes())

		for _, encoded := range encodedChannels {
			channelData.Write(encoded)
		}
	}

	var layerInfo bytes.Buffer
	layerInfo.Write(layerRecords.Bytes())
	layerInfo.Write(channelData.Bytes())
	if layerInfo.Len()%2 != 0 {
		layerInfo.WriteByte(0)
	}

	var section bytes.Buffer
	writeUint32(&section, uint32(layerInfo.Len()))
	section.Write(layerInfo.Bytes())
	writeUint32(&section, 0)
	return section.Bytes()
}

func buildLayerExtraData(t *testing.T, blocks []psdTaggedBlock) []byte {
	t.Helper()

	var out bytes.Buffer
	writeUint32(&out, 0)
	writeUint32(&out, 0)
	writePascalString4(&out, "Layer")
	for _, block := range blocks {
		writeTaggedBlock(&out, block)
	}
	return out.Bytes()
}

func buildImageDataSection(t *testing.T, data psdImageData, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	writeUint16(&out, data.compression)
	switch data.compression {
	case psdCompressionRaw:
		for _, plane := range data.planes {
			out.Write(plane)
		}
	case psdCompressionRLE:
		for _, plane := range data.planes {
			encodedRows := encodeRLEByRow(t, plane, width, height)
			for _, row := range encodedRows {
				writeUint16(&out, uint16(len(row)))
			}
			for _, row := range encodedRows {
				out.Write(row)
			}
		}
	case psdCompressionZip, psdCompressionZipPrediction:
		composite := make([]byte, 0, width*height*len(data.planes))
		for _, plane := range data.planes {
			composite = append(composite, plane...)
		}
		if len(composite) != width*height*len(data.planes) {
			t.Fatalf("invalid composite plane data for zip: planes=%d width=%d height=%d", len(data.planes), width, height)
		}
		if data.compression == psdCompressionZipPrediction {
			for i := 0; i < len(data.planes); i++ {
				start := i * width * height
				end := start + width*height
				if end > len(composite) {
					t.Fatalf("invalid composite plane bounds start=%d end=%d len=%d", start, end, len(composite))
				}
				applyZipPredictionEncodeInPlace(composite[start:end], width, height)
			}
		}
		compressed, err := compressZipData(composite)
		if err != nil {
			t.Fatalf("compress zip: %v", err)
		}
		out.Write(compressed)
	default:
		// Intentionally emit unsupported compression markers for resilience tests.
		return out.Bytes()
	}
	return out.Bytes()
}

func buildLayerAdjustmentPayload(version uint16) []byte {
	var out bytes.Buffer
	writeUint16(&out, version)
	out.WriteString("level-data")
	return out.Bytes()
}

func buildLayerVectorMaskPayload(bounds LayerBounds) []byte {
	var out bytes.Buffer
	right := bounds.X + bounds.W
	bottom := bounds.Y + bounds.H
	writeInt32(&out, int32(bounds.Y))
	writeInt32(&out, int32(bounds.X))
	writeInt32(&out, int32(bottom))
	writeInt32(&out, int32(right))
	writeUint16(&out, 0)
	writeUint16(&out, 0)
	out.WriteByte(0)
	out.WriteByte(0)
	return out.Bytes()
}

func buildLayerTextPayload(text string) []byte {
	var out bytes.Buffer
	writeUint32(&out, 1)
	writeUTF16String(&out, text)
	return out.Bytes()
}

func buildLayerSmartObjectPayloadWithSoLd(identifier string, pageNumber, totalPages, placedType uint32) []byte {
	var out bytes.Buffer
	writeUint32(&out, 1)
	out.WriteByte(byte(len(identifier)))
	out.WriteString(identifier)
	writeUint32(&out, pageNumber)
	writeUint32(&out, totalPages)
	writeUint32(&out, placedType)
	return out.Bytes()
}

func buildLayerLegacyEffectsBlock(effectKeys []string) []byte {
	var out bytes.Buffer
	writeUint16(&out, 1)
	writeUint16(&out, uint16(len(effectKeys)))
	for _, key := range effectKeys {
		out.WriteString("8BIM")
		out.WriteString(key)
		payload := []byte{0x10, 0x20, 0x30, 0x40}
		writeUint32(&out, uint32(len(payload)))
		out.Write(payload)
	}
	return out.Bytes()
}

func buildLayerObjectEffectsBlock(objectVersion, descriptorVersion uint32) []byte {
	var out bytes.Buffer
	writeUint32(&out, objectVersion)
	writeUint32(&out, descriptorVersion)
	out.WriteString("desc")
	return out.Bytes()
}

func buildLayerObjectEffectsBlockWithKeys(objectVersion, descriptorVersion uint32, keys []string) []byte {
	var out bytes.Buffer
	writeUint32(&out, objectVersion)
	writeUint32(&out, descriptorVersion)
	out.WriteString("desc")
	for _, key := range keys {
		out.WriteString(key)
	}
	return out.Bytes()
}

func buildLayerSmartObjectPayload(identifier string) []byte {
	var out bytes.Buffer
	writeUint32(&out, 1)
	out.WriteByte(byte(len(identifier)))
	out.WriteString(identifier)
	return out.Bytes()
}

func buildChannelImageData(t *testing.T, compression uint16, data []byte, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	writeUint16(&out, compression)
	switch compression {
	case psdCompressionRaw:
		out.Write(data)
	case psdCompressionRLE:
		encodedRows := encodeRLEByRow(t, data, maxInt(len(data)/maxInt(height, 1), 1), height)
		for _, row := range encodedRows {
			writeUint16(&out, uint16(len(row)))
		}
		for _, row := range encodedRows {
			out.Write(row)
		}
	case psdCompressionZip, psdCompressionZipPrediction:
		payload := append([]byte(nil), data...)
		if compression == psdCompressionZipPrediction {
			applyZipPredictionEncodeInPlace(payload, width, height)
		}
		compressed, err := compressZipData(payload)
		if err != nil {
			t.Fatalf("compress zip channel: %v", err)
		}
		out.Write(compressed)
	default:
		t.Fatalf("unsupported compression %d in test fixture", compression)
	}
	return out.Bytes()
}

func applyZipPredictionEncodeInPlace(data []byte, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	for row := 0; row < height; row++ {
		rowStart := row * width
		rowEnd := rowStart + width
		if rowEnd > len(data) {
			return
		}
		for col := width - 1; col >= 1; col-- {
			data[rowStart+col] = data[rowStart+col] - data[rowStart+col-1]
		}
	}
}

func compressZipData(data []byte) ([]byte, error) {
	var out bytes.Buffer
	z := zlib.NewWriter(&out)
	if _, err := io.Copy(z, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encodeRLEByRow(t *testing.T, data []byte, width, height int) [][]byte {
	t.Helper()
	if height <= 0 {
		t.Fatal("height must be positive")
	}
	if len(data) != width*height {
		t.Fatalf("RLE source length = %d, want %d", len(data), width*height)
	}
	rows := make([][]byte, 0, height)
	for row := 0; row < height; row++ {
		start := row * width
		rows = append(rows, encodePackBitsLiteral(data[start:start+width]))
	}
	return rows
}

func encodePackBitsLiteral(data []byte) []byte {
	out := []byte{byte(len(data) - 1)}
	out = append(out, data...)
	return out
}

func buildResolutionInfoBlock(dpi int) []byte {
	var out bytes.Buffer
	writeUint32(&out, uint32(dpi<<16))
	writeUint16(&out, 1)
	writeUint16(&out, 1)
	writeUint32(&out, uint32(dpi<<16))
	writeUint16(&out, 1)
	writeUint16(&out, 1)
	return out.Bytes()
}

func buildLayerMaskData(t *testing.T, bounds LayerBounds) []byte {
	t.Helper()

	var out bytes.Buffer
	writeInt32(&out, int32(bounds.Y))
	writeInt32(&out, int32(bounds.X))
	writeInt32(&out, int32(bounds.Y+bounds.H))
	writeInt32(&out, int32(bounds.X+bounds.W))
	writeUint16(&out, 0) // default color
	writeUint16(&out, 0) // flags
	return out.Bytes()
}

func buildLayerBlendRangeData() []byte {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0}
}

func buildSectionDividerData(sectionType uint32) []byte {
	var out bytes.Buffer
	writeUint32(&out, sectionType)
	return out.Bytes()
}

func buildTypeToolInfoBlock() []byte {
	var out bytes.Buffer
	writeUint16(&out, 1)
	for range 6 {
		writeFloat64(&out, 0)
	}
	writeUint16(&out, 50)
	writeUint32(&out, 16)
	writeUint32(&out, 0)
	writeUint32(&out, 16)
	writeUint32(&out, 0)
	return out.Bytes()
}

func writeImageResourceBlock(out *bytes.Buffer, id uint16, name []byte, data []byte) {
	out.WriteString("8BIM")
	writeUint16(out, id)
	if len(name) == 0 {
		out.Write([]byte{0, 0})
	} else {
		out.WriteByte(byte(len(name)))
		out.Write(name)
		if (1+len(name))%2 != 0 {
			out.WriteByte(0)
		}
	}
	writeUint32(out, uint32(len(data)))
	out.Write(data)
	if len(data)%2 != 0 {
		out.WriteByte(0)
	}
}

func writeTaggedBlock(out *bytes.Buffer, block psdTaggedBlock) {
	out.WriteString(block.signature)
	out.WriteString(block.key)
	writeUint32(out, uint32(len(block.data)))
	out.Write(block.data)
	if len(block.data)%2 != 0 {
		out.WriteByte(0)
	}
}

func writePascalString4(out *bytes.Buffer, value string) {
	if len(value) > 255 {
		value = value[:255]
	}
	out.WriteByte(byte(len(value)))
	out.WriteString(value)
	for out.Len()%4 != 0 {
		out.WriteByte(0)
	}
}

func writeUTF16String(out *bytes.Buffer, value string) {
	runes := utf16.Encode([]rune(value))
	writeUint32(out, uint32(len(runes)))
	for _, r := range runes {
		writeUint16(out, r)
	}
}

func writeUint16(out *bytes.Buffer, value uint16) {
	_ = binary.Write(out, binary.BigEndian, value)
}

func writeInt16(out *bytes.Buffer, value int16) {
	_ = binary.Write(out, binary.BigEndian, value)
}

func writeInt32(out *bytes.Buffer, value int32) {
	_ = binary.Write(out, binary.BigEndian, value)
}

func writeUint32(out *bytes.Buffer, value uint32) {
	_ = binary.Write(out, binary.BigEndian, value)
}

func writeFloat64(out *bytes.Buffer, value float64) {
	_ = binary.Write(out, binary.BigEndian, value)
}
