package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	assertProjectArchiveEquivalent(t, restored, doc)
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
