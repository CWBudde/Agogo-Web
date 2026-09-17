package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"slices"
	"testing"
)

func TestSavePSDAndLoadPSDRoundTripPreservesAgogoDocument(t *testing.T) {
	doc := newArchiveOnlyProjectFixture()
	doc.Paths = []NamedPath{
		{
			Name: "Triangle",
			Path: Path{Subpaths: []Subpath{{
				Closed: true,
				Points: []PathPoint{
					{X: 0, Y: 0},
					{X: 2, Y: 0},
					{X: 1, Y: 2},
				},
			}}},
		},
	}
	doc.ActivePathIdx = 0
	doc.StylePresets = []DocumentStylePreset{
		{
			ID:   "preset-1",
			Name: "Thin Stroke",
			Styles: []LayerStyle{
				{Kind: "stroke", Enabled: true, Params: []byte(`{"width":1}`)},
			},
		},
	}

	data, err := SavePSD(doc)
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}

	header, err := (&psdParser{r: bytes.NewReader(data)}).parseHeader()
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if header.Version != 1 {
		t.Fatalf("header version = %d, want 1", header.Version)
	}

	restored, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	// Both blocks Agogo writes for non-raster state are lost on the way back in,
	// and both now say so. TySh has always been reported; AgAJ is the private
	// JSON block, and its silence used to hide the fact that an adjustment layer
	// survives an Agogo round trip only through the embedded project archive.
	wantWarnings := []string{
		"layer \"Title\": metadata block TySh imported as a flattened pixel layer",
		"layer \"Curves\": metadata block AgAJ imported as a flattened pixel layer",
	}
	if !slices.Equal(warnings, wantWarnings) {
		t.Fatalf("warnings = %v, want %v", warnings, wantWarnings)
	}
	assertProjectArchiveEquivalent(t, restored, doc)

	parsed, parsedWarnings, err := LoadPSDWithOptions(data, PSDLoadOptions{IgnoreEmbeddedProject: true})
	if err != nil {
		t.Fatalf("LoadPSDWithOptions(ignore embedded project): %v", err)
	}
	if !slices.Equal(parsedWarnings, wantWarnings) {
		t.Fatalf("spec-parser warnings = %v, want %v", parsedWarnings, wantWarnings)
	}
	if parsed.Name != "Imported PSD" || parsed.ID == doc.ID {
		t.Fatalf("spec-parser document identity = (%q, %q), want a newly imported PSD", parsed.Name, parsed.ID)
	}
	if len(parsed.Paths) != 0 || len(parsed.StylePresets) != 0 {
		t.Fatalf("spec-parser restored Agogo-only data: paths=%d stylePresets=%d", len(parsed.Paths), len(parsed.StylePresets))
	}
	assertParsedPSDRoundTripLayerTree(t, parsed)
}

func assertParsedPSDRoundTripLayerTree(t *testing.T, doc *Document) {
	t.Helper()

	children := doc.LayerRoot.Children()
	if len(children) != 3 {
		t.Fatalf("spec-parser root child count = %d, want 3", len(children))
	}
	if children[0].Name() != "Base" {
		t.Fatalf("spec-parser first layer = %q, want Base", children[0].Name())
	}
	wantGroups := []struct {
		index    int
		name     string
		children []string
	}{
		{index: 1, name: "Overlay Group", children: []string{"Title", "Shape"}},
		{index: 2, name: "Archive Group", children: []string{"Curves", "Archive Vector"}},
	}
	for _, want := range wantGroups {
		group, ok := children[want.index].(*GroupLayer)
		if !ok {
			t.Fatalf("spec-parser layer %d = %T, want *GroupLayer", want.index, children[want.index])
		}
		if group.Name() != want.name {
			t.Fatalf("spec-parser group %d name = %q, want %q", want.index, group.Name(), want.name)
		}
		groupChildren := group.Children()
		if len(groupChildren) != len(want.children) {
			t.Fatalf("spec-parser group %q child count = %d, want %d", group.Name(), len(groupChildren), len(want.children))
		}
		for index, childName := range want.children {
			if groupChildren[index].Name() != childName {
				t.Fatalf("spec-parser group %q child %d = %q, want %q", group.Name(), index, groupChildren[index].Name(), childName)
			}
		}
	}
}

func TestSavePSDUsesPSBForOversizedDocuments(t *testing.T) {
	doc := &Document{
		Width:      psdPSDMaxDimension + 1,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		ID:         "oversized-doc",
		Name:       "Oversized",
		CreatedAt:  "2026-04-17T10:00:00Z",
		CreatedBy:  "agogo-web-test",
		ModifiedAt: "2026-04-17T10:00:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}

	data, err := SavePSD(doc)
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}

	header, err := (&psdParser{r: bytes.NewReader(data)}).parseHeader()
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if header.Version != 2 {
		t.Fatalf("header version = %d, want 2", header.Version)
	}
}

func TestExportDocumentSupportsPSDAndPSB(t *testing.T) {
	h := Init("")
	defer Free(h)

	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "Export PSD Fixture",
		Width:      4,
		Height:     4,
		Resolution: 144,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "transparent",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}

	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Base",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 4, H: 4},
		Pixels:    filledPixels(4, 4, [4]byte{20, 40, 60, 255}),
	})); err != nil {
		t.Fatalf("add layer: %v", err)
	}

	exportedPSD, err := ExportDocument(h, "psd")
	if err != nil {
		t.Fatalf("ExportDocument(psd): %v", err)
	}
	psdBytes, err := base64.StdEncoding.DecodeString(exportedPSD)
	if err != nil {
		t.Fatalf("DecodeString(psd): %v", err)
	}
	psdHeader, err := (&psdParser{r: bytes.NewReader(psdBytes)}).parseHeader()
	if err != nil {
		t.Fatalf("parseHeader(psd): %v", err)
	}
	if psdHeader.Version != 1 {
		t.Fatalf("PSD export version = %d, want 1", psdHeader.Version)
	}

	exportedPSB, err := ExportDocument(h, "psb")
	if err != nil {
		t.Fatalf("ExportDocument(psb): %v", err)
	}
	psbBytes, err := base64.StdEncoding.DecodeString(exportedPSB)
	if err != nil {
		t.Fatalf("DecodeString(psb): %v", err)
	}
	psbHeader, err := (&psdParser{r: bytes.NewReader(psbBytes)}).parseHeader()
	if err != nil {
		t.Fatalf("parseHeader(psb): %v", err)
	}
	if psbHeader.Version != 2 {
		t.Fatalf("PSB export version = %d, want 2", psbHeader.Version)
	}
}

func TestSavePSDSerializesLayerEffectsTextAndAdjustmentMetadata(t *testing.T) {
	doc := &Document{
		Width:      8,
		Height:     8,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		ID:         "metadata-doc",
		Name:       "Metadata",
		CreatedAt:  "2026-04-17T10:00:00Z",
		CreatedBy:  "agogo-web-test",
		ModifiedAt: "2026-04-17T10:00:00Z",
		LayerRoot:  NewGroupLayer("Root"),
	}

	styled := NewPixelLayer("Styled", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, filledPixels(2, 2, [4]byte{10, 20, 30, 255}))
	styled.SetStyleStack([]LayerStyle{
		{Kind: string(LayerStyleKindDropShadow), Enabled: true, Params: json.RawMessage(`{"distance":4}`)},
		{Kind: string(LayerStyleKindColorOverlay), Enabled: false, Params: json.RawMessage(`{"opacity":0.5}`)},
	})

	text := NewTextLayer("Title", LayerBounds{X: 1, Y: 1, W: 4, H: 2}, "Hello PSD", filledPixels(4, 2, [4]byte{200, 100, 50, 255}))
	text.FontFamily = "Work Sans"
	text.FontStyle = "Bold"
	text.FontSize = 24
	text.Alignment = "center"
	text.AntiAlias = "smooth"

	adjustment := NewAdjustmentLayer("Levels", "levels", json.RawMessage(`{"inputBlack":10,"inputWhite":240}`))
	doc.LayerRoot.SetChildren([]LayerNode{styled, text, adjustment})

	data, err := SavePSD(doc)
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}

	layers := parsePSDExportedLayers(t, data)
	if got, want := len(layers), 3; got != want {
		t.Fatalf("layer record count = %d, want %d", got, want)
	}

	if layers[0].Effects == nil || layers[0].Effects.Object == nil {
		t.Fatal("expected object effects metadata on styled layer")
	}
	if got := layers[0].Effects.Object.EffectKeys; len(got) < 2 {
		t.Fatalf("effect keys = %v, want at least 2 entries", got)
	}

	if layers[1].Text == nil {
		t.Fatal("expected text metadata on text layer")
	}
	if !layers[1].Text.HasDescriptor {
		t.Fatal("expected text descriptor metadata")
	}
	if got, want := layers[1].Text.ParsedText, "Hello PSD"; got != want {
		t.Fatalf("parsed text = %q, want %q", got, want)
	}

	if got, want := len(layers[2].Adjustments), 1; got != want {
		t.Fatalf("adjustment metadata count = %d, want %d", got, want)
	}
	if got, want := layers[2].Adjustments[0].Kind, "levels"; got != want {
		t.Fatalf("adjustment kind = %q, want %q", got, want)
	}
	if !layers[2].Adjustments[0].HasVersion || layers[2].Adjustments[0].Version != 1 {
		t.Fatalf("adjustment version = %+v, want version 1", layers[2].Adjustments[0])
	}
}

func parsePSDExportedLayers(t *testing.T, data []byte) []psdLayerRecord {
	t.Helper()

	parser := &psdParser{r: bytes.NewReader(data)}
	header, err := parser.parseHeader()
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if err := parser.skipColorModeData(); err != nil {
		t.Fatalf("skipColorModeData: %v", err)
	}
	if _, err := parser.parseImageResources(); err != nil {
		t.Fatalf("parseImageResources: %v", err)
	}
	layers, err := parser.parseLayerAndMaskInfo(header)
	if err != nil {
		t.Fatalf("parseLayerAndMaskInfo: %v", err)
	}
	return layers
}

// A masked or clipped layer must reach the file with its own pixels intact.
// PSD keeps the mask as the -2 channel and the clip as the record's clipping
// byte and re-evaluates both when compositing, so baking them into the stored
// raster destroyed content and attenuated the survivors twice (PLAN.md
// S.10.3/S.10.4).
func TestSavePSDKeepsMaskedAndClippedLayerPixelsIntact(t *testing.T) {
	doc := testDocumentFixture("nd", "non-destructive", 16, 16)

	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 6, H: 6}, makeSolidPixels(6, 6, 10, 20, 30, 255))

	clipped := NewPixelLayer("Clipped", LayerBounds{X: 0, Y: 0, W: 16, H: 16}, makeSolidPixels(16, 16, 40, 50, 60, 255))
	clipped.SetClipToBelow(true)

	maskData := make([]byte, 16*16)
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			maskData[y*16+x] = 255
		}
	}
	masked := NewPixelLayer("Masked", LayerBounds{X: 0, Y: 0, W: 16, H: 16}, makeSolidPixels(16, 16, 70, 80, 90, 255))
	masked.SetMask(&LayerMask{Enabled: true, Width: 16, Height: 16, Data: maskData})

	doc.ensureLayerRoot().SetChildren([]LayerNode{base, clipped, masked})

	data, err := SavePSD(doc)
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}
	reloaded, warnings, err := LoadPSDWithOptions(data, PSDLoadOptions{IgnoreEmbeddedProject: true})
	if err != nil {
		t.Fatalf("LoadPSDWithOptions: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected import warnings: %v", warnings)
	}

	byName := map[string]*PixelLayer{}
	for _, node := range reloaded.ensureLayerRoot().Children() {
		if pixel, ok := node.(*PixelLayer); ok {
			byName[pixel.Name()] = pixel
		}
	}

	for _, name := range []string{"Clipped", "Masked"} {
		layer, ok := byName[name]
		if !ok {
			t.Fatalf("layer %q missing after the round trip", name)
		}
		want := LayerBounds{X: 0, Y: 0, W: 16, H: 16}
		if layer.Bounds != want {
			// Fatal, not Errorf: the pixel assertions below index the full
			// 16x16 raster and would panic on a cropped one.
			t.Fatalf("%s bounds = %+v, want %+v; the writer cropped the layer", name, layer.Bounds, want)
		}
	}

	// The clip base covers only 6x6, so a pixel well outside it proves the
	// clipped layer's own content survived rather than being cut to the base.
	if got := byName["Clipped"].Pixels[(12*16+12)*4+3]; got != 255 {
		t.Errorf("clipped layer alpha at (12,12) = %d, want 255; pixels outside the clip base were destroyed", got)
	}
	if !byName["Clipped"].ClipToBelow() {
		t.Errorf("clipping flag was lost, so the reader will never re-apply the clip")
	}

	// The mask must attenuate once, at composite time - not be pre-multiplied
	// into the stored raster and then applied again from the -2 channel.
	masked2 := byName["Masked"]
	if got := masked2.Pixels[(12*16+12)*4+3]; got != 255 {
		t.Errorf("masked layer alpha at (12,12) = %d, want 255; the mask was baked into the pixels", got)
	}
	if got := masked2.Pixels[(4*16+4)*4]; got != 70 {
		t.Errorf("masked layer red at (4,4) = %d, want 70 (unattenuated)", got)
	}
	if masked2.Mask() == nil {
		t.Fatalf("mask was not written as the -2 channel")
	}
	if got := masked2.Mask().Data[4*16+4]; got != 255 {
		t.Errorf("mask coverage at (4,4) = %d, want 255", got)
	}
	if got := masked2.Mask().Data[12*16+12]; got != 0 {
		t.Errorf("mask coverage at (12,12) = %d, want 0", got)
	}
}

// Vector masks have no LayerMask of their own, so before the S.10.3/S.10.4 fix
// they were neither baked into the exported raster nor written as a channel:
// they simply vanished. ResolveMask routes the engine's effective coverage -
// raster mask, vector mask, density and feather - into the -2 channel.
func TestSavePSDWritesVectorMaskCoverage(t *testing.T) {
	doc := testDocumentFixture("vm", "vector-mask", 16, 16)
	layer := NewPixelLayer("VectorMasked", LayerBounds{W: 16, H: 16}, makeSolidPixels(16, 16, 200, 200, 200, 255))
	layer.SetVectorMask(&Path{Subpaths: []Subpath{{
		Closed: true,
		Points: []PathPoint{
			{X: 2, Y: 2, InX: 2, InY: 2, OutX: 2, OutY: 2},
			{X: 12, Y: 2, InX: 12, InY: 2, OutX: 12, OutY: 2},
			{X: 12, Y: 10, InX: 12, InY: 10, OutX: 12, OutY: 10},
			{X: 2, Y: 10, InX: 2, InY: 10, OutX: 2, OutY: 10},
		},
	}}})
	doc.ensureLayerRoot().SetChildren([]LayerNode{layer})

	data, err := SavePSD(doc)
	if err != nil {
		t.Fatalf("SavePSD: %v", err)
	}
	reloaded, warnings, err := LoadPSDWithOptions(data, PSDLoadOptions{IgnoreEmbeddedProject: true})
	if err != nil {
		t.Fatalf("LoadPSDWithOptions: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected import warnings: %v", warnings)
	}
	children := reloaded.ensureLayerRoot().Children()
	if len(children) != 1 {
		t.Fatalf("layer count = %d, want 1", len(children))
	}
	mask := children[0].Mask()
	if mask == nil {
		t.Fatalf("vector mask coverage did not reach the PSD at all")
	}
	if got := mask.Data[5*16+5]; got != 255 {
		t.Errorf("coverage inside the vector mask at (5,5) = %d, want 255", got)
	}
	if got := mask.Data[14*16+14]; got != 0 {
		t.Errorf("coverage outside the vector mask at (14,14) = %d, want 0", got)
	}
}
