package psdfixture

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func strPtr(value string) *string              { return &value }
func intPtr(value int) *int                    { return &value }
func boolPtr(value bool) *bool                 { return &value }
func floatPtr(value float64) *float64          { return &value }
func strsPtr(values []string) *[]string        { return &values }
func intsPtr(values []int) *[]int              { return &values }
func layersPtr(v []LayerExpect) *[]LayerExpect { return &v }

// solid returns a w*h RGBA buffer filled with one colour.
func solid(w, h int, rgba [4]byte) []byte {
	buffer := make([]byte, w*h*4)
	for i := 0; i < len(buffer); i += 4 {
		copy(buffer[i:i+4], rgba[:])
	}
	return buffer
}

// sampleTree builds the model tree the annotated sidecar example describes:
// a 64x48 background plus an isolated, half-opaque "Group A" holding a
// multiplying 32x32 fill at (8,8).
func sampleTree() *model.GroupLayer {
	root := model.NewGroupLayer("Root")

	background := model.NewPixelLayer("Background", model.LayerBounds{X: 0, Y: 0, W: 64, H: 48}, solid(64, 48, [4]byte{255, 255, 255, 255}))

	group := model.NewGroupLayer("Group A")
	group.SetOpacity(128.0 / 255.0)
	group.Isolated = true
	mask := make([]byte, 64*48)
	for y := 6; y < 36; y++ {
		for x := 4; x < 44; x++ {
			mask[y*64+x] = 255
		}
	}
	group.SetMask(&model.LayerMask{Enabled: true, Width: 64, Height: 48, Data: mask})

	fill := model.NewPixelLayer("Fill", model.LayerBounds{X: 8, Y: 8, W: 32, H: 32}, solid(32, 32, [4]byte{220, 30, 30, 255}))
	fill.SetBlendMode(model.BlendModeMultiply)
	group.SetChildren([]model.LayerNode{fill})

	root.SetChildren([]model.LayerNode{background, group})
	return root
}

func sampleActual() Actual {
	return Actual{
		Width:      64,
		Height:     48,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Root:       sampleTree(),
		Warnings:   nil,
		Composite:  solid(64, 48, [4]byte{255, 255, 255, 255}),
	}
}

// sampleExpectation mirrors sampleActual exactly, so every test below starts
// from a passing comparison and breaks exactly one thing.
func sampleExpectation() Expectation {
	return Expectation{
		SchemaVersion: SchemaVersion,
		ID:            "sample",
		File:          "sample.psd",
		Assert:        []string{ScopeDocument, ScopeTree, ScopeWarnings},
		Document: &DocumentExpect{
			Width:      intPtr(64),
			Height:     intPtr(48),
			ColorMode:  strPtr("rgb"),
			BitDepth:   intPtr(8),
			Resolution: floatPtr(72),
			IsPSB:      boolPtr(false),
		},
		Warnings: strsPtr(nil),
		Layers: layersPtr([]LayerExpect{
			{
				Path: "Background", Name: strPtr("Background"), Type: strPtr("pixel"),
				Bounds:    &BoundsExpect{X: 0, Y: 0, W: 64, H: 48},
				BlendMode: strPtr("normal"), Opacity255: intPtr(255), FillOpacity255: intPtr(255),
				Visible: boolPtr(true), ClipToBelow: boolPtr(false),
				Mask: &MaskExpect{Present: boolPtr(false)},
			},
			{
				Path: "Group A", Name: strPtr("Group A"), Type: strPtr("group"),
				BlendMode: strPtr("normal"), Opacity255: intPtr(128), FillOpacity255: intPtr(255),
				Visible: boolPtr(true), ClipToBelow: boolPtr(false), Isolated: boolPtr(true),
				Mask: &MaskExpect{Present: boolPtr(true), Enabled: boolPtr(true)},
				Children: layersPtr([]LayerExpect{
					{
						Path: "Group A/Fill", Name: strPtr("Fill"), Type: strPtr("pixel"),
						Bounds:    &BoundsExpect{X: 8, Y: 8, W: 32, H: 32},
						BlendMode: strPtr("multiply"), Opacity255: intPtr(255), FillOpacity255: intPtr(255),
						Visible: boolPtr(true), ClipToBelow: boolPtr(false),
						Mask: &MaskExpect{Present: boolPtr(false)},
					},
				}),
			},
		}),
	}
}

func paths(mismatches []Mismatch) []string {
	out := make([]string, 0, len(mismatches))
	for _, mismatch := range mismatches {
		out = append(out, mismatch.Path)
	}
	return out
}

func findMismatch(t *testing.T, mismatches []Mismatch, path string) Mismatch {
	t.Helper()
	for _, mismatch := range mismatches {
		if mismatch.Path == path {
			return mismatch
		}
	}
	t.Fatalf("no mismatch at %q; got %v", path, paths(mismatches))
	return Mismatch{}
}

func assertClean(t *testing.T, mismatches []Mismatch) {
	t.Helper()
	if len(mismatches) != 0 {
		for _, mismatch := range mismatches {
			t.Errorf("unexpected mismatch: %s", mismatch)
		}
	}
}

func TestCompareDocumentMatchingFixtureIsClean(t *testing.T) {
	assertClean(t, CompareDocument(sampleExpectation(), sampleActual()))
}

func TestCompareDocumentNilFieldAssertsNothing(t *testing.T) {
	exp := sampleExpectation()
	// Wipe every optional field; nothing may be reported even though the
	// actual document is wildly different from the defaults.
	exp.Document = &DocumentExpect{}
	exp.Warnings = nil
	exp.Layers = nil

	actual := sampleActual()
	actual.Width = 1
	actual.Height = 1
	actual.ColorMode = "gray"
	actual.Warnings = []string{"something happened"}
	actual.Root = model.NewGroupLayer("Root")

	assertClean(t, CompareDocument(exp, actual))
}

func TestCompareDocumentNilLayerFieldsAssertNothing(t *testing.T) {
	exp := sampleExpectation()
	exp.Layers = layersPtr([]LayerExpect{
		{Path: "Background"},
		{Path: "Group A"},
	})
	assertClean(t, CompareDocument(exp, sampleActual()))
}

func TestCompareDocumentReportsFieldMismatches(t *testing.T) {
	exp := sampleExpectation()
	actual := sampleActual()
	actual.ColorMode = "gray"

	mismatches := CompareDocument(exp, actual)
	mismatch := findMismatch(t, mismatches, "document.colorMode")
	if mismatch.Want != "rgb" || mismatch.Got != "gray" {
		t.Fatalf("want rgb/gray, got %q/%q", mismatch.Want, mismatch.Got)
	}
}

func TestCompareDocumentOpacityUsesSidecarBytes(t *testing.T) {
	exp := sampleExpectation()
	(*exp.Layers)[1].Opacity255 = intPtr(129)

	mismatch := findMismatch(t, CompareDocument(exp, sampleActual()), "layers[1].opacity255")
	if mismatch.Want != "129" || mismatch.Got != "128" {
		t.Fatalf("want 129/128, got %q/%q", mismatch.Want, mismatch.Got)
	}
}

func TestCompareDocumentPathsAddressTheSidecar(t *testing.T) {
	exp := sampleExpectation()
	(*(*exp.Layers)[1].Children)[0].BlendMode = strPtr("screen")

	mismatch := findMismatch(t, CompareDocument(exp, sampleActual()), "layers[1].children[0].blendMode")
	if mismatch.Want != "screen" || mismatch.Got != "multiply" {
		t.Fatalf("want screen/multiply, got %q/%q", mismatch.Want, mismatch.Got)
	}
}

func TestCompareDocumentChildrenAreExhaustive(t *testing.T) {
	exp := sampleExpectation()
	*exp.Layers = (*exp.Layers)[:1] // drop "Group A"

	mismatches := CompareDocument(exp, sampleActual())
	mismatch := findMismatch(t, mismatches, "layers")
	if mismatch.Want != "1 layers" || mismatch.Got != "2 layers" {
		t.Fatalf("want 1/2 layers, got %q/%q", mismatch.Want, mismatch.Got)
	}
	if !strings.Contains(mismatch.Detail, "tree diff") {
		t.Fatalf("the first structural mismatch should carry the tree diff, got %q", mismatch.Detail)
	}
}

func TestCompareDocumentAttachesTheTreeDiffOnlyOnce(t *testing.T) {
	exp := sampleExpectation()
	*exp.Layers = append(
		*exp.Layers,
		LayerExpect{Path: "Extra One", Name: strPtr("Extra One")},
		LayerExpect{Path: "Extra Two", Name: strPtr("Extra Two")},
	)

	mismatches := CompareDocument(exp, sampleActual())
	withDiff := 0
	for _, mismatch := range mismatches {
		if strings.Contains(mismatch.Detail, "tree diff") {
			withDiff++
		}
	}
	if withDiff != 1 {
		t.Fatalf("%d mismatches carry the tree diff, want exactly 1 (%v)", withDiff, paths(mismatches))
	}
}

func TestCompareDocumentNilChildrenSkipsThatLevel(t *testing.T) {
	exp := sampleExpectation()
	(*exp.Layers)[1].Children = nil // group contents are not asserted

	group, ok := sampleActual().Root.Children()[1].(*model.GroupLayer)
	if !ok {
		t.Fatal("expected a group layer")
	}
	group.SetChildren(nil)

	actual := sampleActual()
	root := model.NewGroupLayer("Root")
	emptyGroup := model.NewGroupLayer("Group A")
	emptyGroup.SetOpacity(128.0 / 255.0)
	emptyGroup.Isolated = true
	emptyGroup.SetMask(&model.LayerMask{Enabled: true, Width: 64, Height: 48, Data: make([]byte, 64*48)})
	root.SetChildren([]model.LayerNode{
		model.NewPixelLayer("Background", model.LayerBounds{W: 64, H: 48}, solid(64, 48, [4]byte{255, 255, 255, 255})),
		emptyGroup,
	})
	actual.Root = root

	assertClean(t, CompareDocument(exp, actual))
}

func TestCompareWarningsMultisetBothDirections(t *testing.T) {
	exp := sampleExpectation()
	exp.Warnings = strsPtr([]string{"shared", "only-expected"})

	actual := sampleActual()
	actual.Warnings = []string{"only-actual", "shared"}

	mismatches := CompareDocument(exp, actual)
	if len(mismatches) != 2 {
		t.Fatalf("want 2 warning mismatches, got %d: %v", len(mismatches), mismatches)
	}
	rendered := []string{mismatches[0].String(), mismatches[1].String()}
	if rendered[0] != `warnings: unexpected "only-actual"` {
		t.Errorf("first mismatch = %q", rendered[0])
	}
	if rendered[1] != `warnings: missing "only-expected"` {
		t.Errorf("second mismatch = %q", rendered[1])
	}
}

func TestCompareWarningsIgnoresOrder(t *testing.T) {
	exp := sampleExpectation()
	exp.Warnings = strsPtr([]string{"a", "b"})
	actual := sampleActual()
	actual.Warnings = []string{"b", "a"}
	assertClean(t, CompareDocument(exp, actual))
}

func TestCompareWarningsCountsDuplicates(t *testing.T) {
	exp := sampleExpectation()
	exp.Warnings = strsPtr([]string{"dup"})
	actual := sampleActual()
	actual.Warnings = []string{"dup", "dup"}

	mismatches := CompareDocument(exp, actual)
	if len(mismatches) != 1 || mismatches[0].String() != `warnings: unexpected "dup"` {
		t.Fatalf("want one unexpected duplicate, got %v", mismatches)
	}
}

func TestCompareEmptyWarningsListDemandsACleanImport(t *testing.T) {
	exp := sampleExpectation()
	exp.Warnings = strsPtr([]string{})
	actual := sampleActual()
	actual.Warnings = []string{"unsupported block"}

	mismatches := CompareDocument(exp, actual)
	if len(mismatches) != 1 {
		t.Fatalf("want 1 mismatch, got %v", mismatches)
	}
}

func TestKnownGapSuppressesTheNormalAssertion(t *testing.T) {
	exp := sampleExpectation()
	(*exp.Layers)[0].FillOpacity255 = intPtr(128)
	exp.KnownGaps = []KnownGap{{
		Path:   "layers[0].fillOpacity255",
		Want:   json.RawMessage(`128`),
		Actual: json.RawMessage(`255`),
		Reason: "PLAN.md S.10.7: the iOpa tagged block is not parsed",
	}}

	// The engine reports 255, which is what the gap records, so nothing fires.
	assertClean(t, CompareDocument(exp, sampleActual()))
}

func TestKnownGapFailsWhenTheGapCloses(t *testing.T) {
	exp := sampleExpectation()
	(*exp.Layers)[0].FillOpacity255 = intPtr(128)
	exp.KnownGaps = []KnownGap{{
		Path:   "layers[0].fillOpacity255",
		Want:   json.RawMessage(`128`),
		Actual: json.RawMessage(`255`),
		Reason: "PLAN.md S.10.7: the iOpa tagged block is not parsed",
	}}

	actual := sampleActual()
	actual.Root.Children()[0].SetFillOpacity(128.0 / 255.0)
	// SetFillOpacity on the copy returned by Children() would be lost; rebuild.
	root := model.NewGroupLayer("Root")
	background := model.NewPixelLayer("Background", model.LayerBounds{W: 64, H: 48}, solid(64, 48, [4]byte{255, 255, 255, 255}))
	background.SetFillOpacity(128.0 / 255.0)
	root.SetChildren([]model.LayerNode{background})
	actual.Root = root
	exp.Layers = layersPtr((*exp.Layers)[:1])

	mismatch := findMismatch(t, CompareDocument(exp, actual), "layers[0].fillOpacity255")
	if mismatch.Detail != "known gap closed; update the sidecar" {
		t.Fatalf("detail = %q", mismatch.Detail)
	}
	if mismatch.Want != "255" || mismatch.Got != "128" {
		t.Fatalf("want 255/128, got %q/%q", mismatch.Want, mismatch.Got)
	}
}

func TestKnownGapReportsAThirdValue(t *testing.T) {
	exp := sampleExpectation()
	exp.KnownGaps = []KnownGap{{
		Path:   "document.colorMode",
		Want:   json.RawMessage(`"rgb"`),
		Actual: json.RawMessage(`"gray"`),
		Reason: "documented gap",
	}}
	actual := sampleActual()
	actual.ColorMode = "indexed"

	mismatch := findMismatch(t, CompareDocument(exp, actual), "document.colorMode")
	if !strings.Contains(mismatch.Detail, "known gap value changed") {
		t.Fatalf("detail = %q", mismatch.Detail)
	}
	if mismatch.Want != "gray" || mismatch.Got != "indexed" {
		t.Fatalf("want gray/indexed, got %q/%q", mismatch.Want, mismatch.Got)
	}
}

func TestCompareReexportSkipsLossyPaths(t *testing.T) {
	exp := sampleExpectation()
	exp.Assert = append(exp.Assert, ScopeWriter)
	exp.Writer = &WriterExpect{Format: "psd", Expect: "lossy", Lossy: []string{"layers[1].mask.enabled"}}

	actual := sampleActual()
	root := actual.Root
	group, ok := root.Children()[1].(*model.GroupLayer)
	if !ok {
		t.Fatal("expected a group layer")
	}
	group.SetMask(&model.LayerMask{Enabled: false, Width: 64, Height: 48, Data: make([]byte, 64*48)})
	root.SetChildren([]model.LayerNode{root.Children()[0], group})

	// Import-time comparison still reports it...
	findMismatch(t, CompareDocument(exp, actual), "layers[1].mask.enabled")
	// ...but the re-export leg does not, because the loss is on the allowlist.
	assertClean(t, CompareReexport(exp, actual))
}

func TestCompareReexportReportsStaleLossyEntries(t *testing.T) {
	exp := sampleExpectation()
	exp.Assert = append(exp.Assert, ScopeWriter)
	exp.Writer = &WriterExpect{
		Format: "psd",
		Expect: "lossy",
		Lossy:  []string{"layers[1].mask.enabled", "document.resolution"},
	}

	mismatches := CompareReexport(exp, sampleActual())
	if len(mismatches) != 2 {
		t.Fatalf("want 2 stale-allowlist reports, got %v", mismatches)
	}
	for _, mismatch := range mismatches {
		if !strings.Contains(mismatch.Detail, "the allowlist is stale") {
			t.Errorf("%s: detail = %q", mismatch.Path, mismatch.Detail)
		}
	}
}

func TestCompareLayerPixelsHonoursSpaceAndTolerance(t *testing.T) {
	exp := sampleExpectation()
	exp.Assert = append(exp.Assert, ScopeLayerPixels)
	exp.LayerPixels = []LayerPixelExpect{
		{Layer: "Group A/Fill", PixelExpect: PixelExpect{X: 0, Y: 0, RGBA: [4]int{220, 30, 30, 255}}},
		{Layer: "Group A/Fill", Space: "document", PixelExpect: PixelExpect{X: 8, Y: 8, RGBA: [4]int{220, 30, 30, 255}}},
		{Layer: "Group A/Fill", PixelExpect: PixelExpect{X: 1, Y: 1, RGBA: [4]int{218, 32, 30, 255}, Tolerance: 2}},
	}
	assertClean(t, CompareDocument(exp, sampleActual()))
}

func TestCompareLayerPixelsReportsOutOfTolerance(t *testing.T) {
	exp := sampleExpectation()
	exp.LayerPixels = []LayerPixelExpect{
		{Layer: "Group A/Fill", PixelExpect: PixelExpect{X: 0, Y: 0, RGBA: [4]int{219, 30, 30, 255}, Tolerance: 1}},
		{Layer: "Group A/Fill", PixelExpect: PixelExpect{X: 0, Y: 0, RGBA: [4]int{217, 30, 30, 255}, Tolerance: 1}},
		{Layer: "No Such Layer", PixelExpect: PixelExpect{X: 0, Y: 0}},
		{Layer: "Group A/Fill", Space: "document", PixelExpect: PixelExpect{X: 0, Y: 0}},
	}
	mismatches := CompareDocument(exp, sampleActual())
	want := []string{"layerPixels[1]", "layerPixels[2].layer", "layerPixels[3]"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

func TestCompareMaskSamples(t *testing.T) {
	exp := sampleExpectation()
	exp.MaskSamples = []MaskSampleExpect{
		{Layer: "Group A", X: 0, Y: 0, Value: 0},
		{Layer: "Group A", X: 20, Y: 20, Value: 255},
		{Layer: "Group A", X: 0, Y: 0, Value: 255},
		{Layer: "Background", X: 0, Y: 0, Value: 0},
	}
	mismatches := CompareDocument(exp, sampleActual())
	want := []string{"maskSamples[2]", "maskSamples[3].layer"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

func TestCompareCompositePixels(t *testing.T) {
	exp := sampleExpectation()
	exp.CompositePixels = []PixelExpect{
		{X: 0, Y: 0, RGBA: [4]int{255, 255, 255, 255}, Tolerance: 1},
		{X: 63, Y: 47, RGBA: [4]int{0, 0, 0, 255}},
		{X: 64, Y: 0, RGBA: [4]int{0, 0, 0, 0}},
	}
	mismatches := CompareDocument(exp, sampleActual())
	want := []string{"compositePixels[1]", "compositePixels[2]"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	if !strings.Contains(findMismatch(t, mismatches, "compositePixels[2]").Got, "outside") {
		t.Error("an out-of-range sample should say so")
	}
}

func TestCompareRecordsIsPositional(t *testing.T) {
	exp := sampleExpectation()
	exp.Records = &[]RecordExpect{
		{
			Index: 0, Name: strPtr("Background"), SectionType: intPtr(0),
			Bounds:     &BoundsExpect{W: 64, H: 48},
			ChannelIDs: intsPtr([]int{0, 1, 2, -1}),
			Opacity255: intPtr(255), Visible: boolPtr(true), Clipping: boolPtr(false),
			BlendMode: strPtr("normal"),
			Mask:      &MaskExpect{Present: boolPtr(false)},
		},
		{
			Index: 1, Name: strPtr("Group A"), SectionType: intPtr(1), PassThrough: boolPtr(false),
			Opacity255: intPtr(128),
			Mask: &MaskExpect{
				Present: boolPtr(true), Enabled: boolPtr(true),
				Rect: &BoundsExpect{X: 4, Y: 6, W: 40, H: 30}, DefaultFill: intPtr(0), Inverted: boolPtr(false),
			},
		},
	}
	records := []RecordView{
		{
			Name: "Background", SectionType: 0, BlendMode: "normal", Opacity: 1,
			Visible: true, Bounds: model.LayerBounds{W: 64, H: 48}, ChannelIDs: []int{0, 1, 2, -1},
		},
		{
			Name: "Group A", SectionType: 1, Opacity: 128.0 / 255.0,
			HasMask: true, MaskEnabled: true, MaskBounds: model.LayerBounds{X: 4, Y: 6, W: 40, H: 30},
		},
	}
	assertClean(t, CompareRecords(exp, records))

	records[1].MaskBounds = model.LayerBounds{X: 0, Y: 0, W: 40, H: 30}
	records[1].PassThrough = true
	mismatches := CompareRecords(exp, records)
	want := []string{"psdRecords[1].passThrough", "psdRecords[1].mask.rect"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

func TestCompareRecordsReportsMissingRecords(t *testing.T) {
	exp := sampleExpectation()
	exp.Records = &[]RecordExpect{
		{Index: 0, Name: strPtr("Background")},
		{Index: 1, Name: strPtr("Group A")},
	}
	mismatches := CompareRecords(exp, []RecordView{{Name: "Background"}})
	want := []string{"psdRecords", "psdRecords[1]"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

func TestCompareRecordsNilExpectationAssertsNothing(t *testing.T) {
	assertClean(t, CompareRecords(sampleExpectation(), []RecordView{{Name: "anything"}}))
}

// reexportRecordCase is the closed-folder scenario in miniature: the writer
// re-opens a closed folder because the engine model cannot store the flag, and
// only the record leg can see it.
func reexportRecordCase() (Expectation, []RecordView) {
	exp := sampleExpectation()
	exp.Records = &[]RecordExpect{
		{Index: 0, Name: strPtr("Open Folder"), SectionType: intPtr(1)},
		{Index: 1, Name: strPtr("Closed Folder"), SectionType: intPtr(2)},
	}
	exp.Writer = &WriterExpect{
		Format: "psd",
		Expect: "lossy",
		Lossy:  []string{"psdRecords[1].sectionType"},
	}
	records := []RecordView{
		{Name: "Open Folder", SectionType: 1},
		{Name: "Closed Folder", SectionType: 1},
	}
	return exp, records
}

// TestCompareReexportRecordsAppliesLossyAllowlist is the assertion the writer
// harness leans on: the record-scope fields the engine model cannot carry are
// compared against the EXTERNAL expectation, with the reviewed losses excused
// and everything else still asserted.
func TestCompareReexportRecordsAppliesLossyAllowlist(t *testing.T) {
	exp, records := reexportRecordCase()
	assertClean(t, CompareReexportRecords(exp, records))

	// The same input without the allowlist must fail, or the allowlist is
	// excusing nothing and the test above is vacuous.
	if len(CompareRecords(exp, records)) == 0 {
		t.Fatal("CompareRecords must still report the lossy path; the fixture proves nothing otherwise")
	}
}

func TestCompareReexportRecordsStillAssertsUnlistedFields(t *testing.T) {
	exp, records := reexportRecordCase()
	records[0].SectionType = 2

	mismatches := CompareReexportRecords(exp, records)
	want := []string{"psdRecords[0].sectionType"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

// TestCompareReexportRecordsReportsStaleLossy closes the loop the other way: a
// writer fix that makes an allowlisted path match must fail the suite, so the
// allowlist cannot outlive the defect it documents.
func TestCompareReexportRecordsReportsStaleLossy(t *testing.T) {
	exp, records := reexportRecordCase()
	records[1].SectionType = 2

	mismatches := CompareReexportRecords(exp, records)
	want := []string{"psdRecords[1].sectionType"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	if !strings.Contains(findMismatch(t, mismatches, "psdRecords[1].sectionType").Detail, "stale") {
		t.Error("a matched lossy path should be reported as a stale allowlist entry")
	}
}

// TestCompareRecordsIgnoresLossyAllowlist keeps the reader leg strict: the
// allowlist excuses what the WRITER loses, and must never soften the assertion
// on the fixture as it was authored.
func TestCompareRecordsIgnoresLossyAllowlist(t *testing.T) {
	exp, records := reexportRecordCase()
	want := []string{"psdRecords[1].sectionType"}
	if got := paths(CompareRecords(exp, records)); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

// TestCompareRecordsAssertsRawBlendKey pins the raw four-character key. The
// normalised blendMode maps every unrecognised key onto "normal", so a writer
// emitting the wrong key for the right mode passes on blendMode alone.
func TestCompareRecordsAssertsRawBlendKey(t *testing.T) {
	exp := sampleExpectation()
	exp.Records = &[]RecordExpect{
		{Index: 0, BlendMode: strPtr("multiply"), PSDBlendKey: strPtr("mul ")},
	}
	assertClean(t, CompareRecords(exp, []RecordView{{BlendMode: "multiply", BlendKey: "mul "}}))

	// Right mode, wrong key: blendMode alone would not catch this.
	mismatches := CompareRecords(exp, []RecordView{{BlendMode: "multiply", BlendKey: "mul"}})
	want := []string{"psdRecords[0].psdBlendKey"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	// The padding must be visible, or the failure message reads "mul" vs "mul".
	if got := findMismatch(t, mismatches, "psdRecords[0].psdBlendKey"); got.Want != `"mul "` || got.Got != `"mul"` {
		t.Errorf("want/got = %s/%s, want the keys quoted so trailing spaces show", got.Want, got.Got)
	}
}

// TestCompareReexportAllReportsUnusedLossy is the guard on the allowlist
// itself: an entry that addresses no assertion excuses nothing and can never be
// reported stale, so it would sit there looking like a reviewed exemption.
func TestCompareReexportAllReportsUnusedLossy(t *testing.T) {
	exp, records := reexportRecordCase()
	exp.Writer.Lossy = append(exp.Writer.Lossy, "psdRecords[1].sectionTyp", "layers[99].name")

	mismatches := CompareReexportAll(exp, sampleActual(), records)
	want := []string{"psdRecords[1].sectionTyp", "layers[99].name"}
	if got := paths(mismatches); !equalStrings(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	if !strings.Contains(findMismatch(t, mismatches, "layers[99].name").Detail, "excuses nothing") {
		t.Error("an unused lossy path should say that it excuses nothing")
	}
}

// TestCompareReexportAllAcceptsPathsEitherLegAsserts is why the two legs must
// share one pass: split apart, the document leg would call every psdRecords
// entry unused and the record leg every layers entry unused.
func TestCompareReexportAllAcceptsPathsEitherLegAsserts(t *testing.T) {
	exp, records := reexportRecordCase()
	exp.Layers = &[]LayerExpect{{Name: strPtr("wrong")}, {Name: strPtr("Group A")}}
	exp.Writer.Lossy = append(exp.Writer.Lossy, "layers[0].name")

	// layers[0].name is asserted by the document leg, psdRecords[1].sectionType
	// by the record leg; neither may be reported unused.
	assertClean(t, CompareReexportAll(exp, sampleActual(), records))
}

func TestMismatchString(t *testing.T) {
	cases := []struct {
		mismatch Mismatch
		want     string
	}{
		{Mismatch{Path: "document.width", Want: "64", Got: "32"}, "document.width: want 64, got 32"},
		{Mismatch{Path: "warnings", Detail: `unexpected "x"`}, `warnings: unexpected "x"`},
		{Mismatch{Path: "layers[0].opacity255", Want: "255", Got: "128", Detail: "why"}, "layers[0].opacity255: want 255, got 128\n  why"},
	}
	for _, testCase := range cases {
		if got := testCase.mismatch.String(); got != testCase.want {
			t.Errorf("String() = %q, want %q", got, testCase.want)
		}
	}
}

func TestExpectationAsserts(t *testing.T) {
	exp := Expectation{Assert: []string{ScopeDocument, ScopeWriter}}
	if !exp.Asserts(ScopeWriter) {
		t.Error("Asserts(writer) = false")
	}
	if exp.Asserts(ScopeMaskSamples) {
		t.Error("Asserts(maskSamples) = true")
	}
}

func TestBoundsExpectConvertsToModel(t *testing.T) {
	got := BoundsExpect{X: 1, Y: 2, W: 3, H: 4}.Bounds()
	if got != (model.LayerBounds{X: 1, Y: 2, W: 3, H: 4}) {
		t.Fatalf("Bounds() = %+v", got)
	}
}
