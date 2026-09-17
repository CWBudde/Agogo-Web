//go:build !js

package psdfixture

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

// Actual is the neutral view of an imported document. The caller builds it from
// whatever it has (psdimport, engine.Document, a hand-made tree); this package
// deliberately knows nothing about those types so that it cannot pull
// internal/io/psd into an import cycle with its own test package.
type Actual struct {
	Width      int
	Height     int
	Resolution float64
	ColorMode  string
	BitDepth   int
	IsPSB      bool
	Root       *model.GroupLayer
	Warnings   []string
	// Composite is the flattened RGBA surface, Width*Height*4 bytes. It may be
	// nil when the caller does not assert compositePixels.
	Composite []byte
}

// RecordView is the neutral view of one flat PSD layer record. It carries the
// detail the engine model throws away: the section divider type, the mask
// rectangle and default fill, and the raw channel IDs.
type RecordView struct {
	Name        string
	SectionType int
	PassThrough bool
	BlendMode   string
	// BlendKey is the raw four-character PSD key, trailing spaces included.
	BlendKey string
	// Opacity is in [0,1]; sidecars store the 0..255 byte and this package
	// converts at compare time.
	Opacity           float64
	Visible           bool
	ClipToBelow       bool
	Bounds            model.LayerBounds
	ChannelIDs        []int
	HasMask           bool
	MaskEnabled       bool
	MaskBounds        model.LayerBounds
	MaskDefault       int
	MaskInverted      bool
	UnsupportedBlocks []string
}

// Mismatch is one failed assertion. Path addresses the sidecar line that made
// the claim, so a failure message points at the JSON to fix or at the engine
// behaviour to correct.
type Mismatch struct {
	// Path is JSON-ish: "layers[1].children[0].blendMode".
	Path string
	// Want is the sidecar's value, rendered in the sidecar's own units.
	Want string
	// Got is the engine's value, rendered in the same units.
	Got string
	// Detail carries extra context: a known-gap reason, or the tree diff, which
	// is attached to the first structural mismatch only.
	Detail string
}

func (m Mismatch) String() string {
	if m.Want == "" && m.Got == "" {
		if m.Detail == "" {
			return m.Path
		}
		return m.Path + ": " + m.Detail
	}
	out := fmt.Sprintf("%s: want %s, got %s", m.Path, m.Want, m.Got)
	if m.Detail != "" {
		out += "\n  " + m.Detail
	}
	return out
}

// CompareDocument checks an imported document against the sidecar: header,
// warnings, layer tree, layer pixels, mask samples and composite pixels.
//
// A nil expectation field asserts nothing. Scopes are honoured by presence, not
// by the "assert" list — corpus_test.go separately enforces that the two agree,
// which keeps the comparison logic free of a second source of truth.
func CompareDocument(exp Expectation, actual Actual) []Mismatch {
	return compareImport(exp, actual, false)
}

// CompareReexport checks a document that was written by Agogo and read back.
// It applies the same assertions as CompareDocument with two differences:
// paths on the reviewed writer.lossy allowlist are not asserted, and any lossy
// path that unexpectedly matched is reported so the allowlist cannot go stale.
func CompareReexport(exp Expectation, actual Actual) []Mismatch {
	return compareImport(exp, actual, true)
}

// CompareRecords checks the flat PSD layer records against the psdRecords
// scope. Records are compared positionally, in file order.
func CompareRecords(exp Expectation, records []RecordView) []Mismatch {
	return compareRecords(exp, records, false)
}

// CompareReexportRecords checks the flat layer records of a document Agogo
// WROTE against the same external psdRecords expectation the reader is held to.
//
// This is the only assertion that can catch a writer which drops record-level
// detail: the section-divider type, the mask rectangle and default fill, and
// the channel IDs never reach the engine model, so CompareReexport — which only
// sees the reconstructed model — passes whatever the writer does with them.
// Paths on the reviewed writer.lossy allowlist are not asserted, and a lossy
// path that unexpectedly matched is reported so the allowlist cannot go stale.
func CompareReexportRecords(exp Expectation, records []RecordView) []Mismatch {
	return compareRecords(exp, records, true)
}

func compareRecords(exp Expectation, records []RecordView, reexport bool) []Mismatch {
	if exp.Records == nil {
		return nil
	}
	c := newCollector(exp, reexport)
	compareRecordsInto(c, exp, records)
	if reexport {
		c.reportStaleLossy()
	}
	return c.out
}

func compareRecordsInto(c *collector, exp Expectation, records []RecordView) {
	if exp.Records == nil {
		return
	}
	want := *exp.Records
	if len(want) != len(records) {
		c.addStructural(Mismatch{
			Path: "psdRecords",
			Want: strconv.Itoa(len(want)) + " records",
			Got:  strconv.Itoa(len(records)) + " records",
		})
	}
	for i := range want {
		path := "psdRecords[" + strconv.Itoa(i) + "]"
		if i >= len(records) {
			c.addStructural(Mismatch{Path: path, Want: recordLabel(want[i]), Got: "<absent>"})
			continue
		}
		compareRecord(c, path, want[i], i, records[i])
	}
}

func compareImport(exp Expectation, actual Actual, reexport bool) []Mismatch {
	c := newCollector(exp, reexport)
	compareImportInto(c, exp, actual)
	if reexport {
		c.reportStaleLossy()
	}
	return c.out
}

func compareImportInto(c *collector, exp Expectation, actual Actual) {
	c.treeDiff = func() string {
		if exp.Layers == nil {
			return ""
		}
		return UnifiedDiff(RenderExpectedTree(*exp.Layers), RenderTree(actual.Root))
	}

	compareDocumentHeader(c, exp.Document, actual)
	compareWarnings(c, exp.Warnings, actual.Warnings)
	if exp.Layers != nil {
		compareLayerLevel(c, "layers", *exp.Layers, childNodes(actual.Root))
	}
	compareLayerPixels(c, exp.LayerPixels, actual)
	compareMaskSamples(c, exp.MaskSamples, actual)
	compareCompositePixels(c, exp.CompositePixels, actual)
}

// CompareReexportAll is the whole writer check: the reconstructed document AND
// the flat layer records, judged together against the external expectation.
//
// The writer harness calls this rather than the two legs separately, because
// only a single pass over both can account for the ENTIRE writer.lossy
// allowlist. Split across two calls, each leg sees paths it never asserts —
// the document leg never touches psdRecords[...], the record leg never touches
// layers[...] — so neither can tell a path it merely does not own from a path
// that addresses nothing at all. Together they can, and a path that addresses
// nothing is reported: a typo, or an entry outliving the field it excused,
// silently excuses nothing while looking like a reviewed exemption.
//
// Pass nil records when the sidecar does not assert the psdRecords scope.
func CompareReexportAll(exp Expectation, actual Actual, records []RecordView) []Mismatch {
	c := newCollector(exp, true)
	compareImportInto(c, exp, actual)
	compareRecordsInto(c, exp, records)
	c.reportStaleLossy()
	c.reportUnusedLossy(exp)
	return c.out
}

// collector accumulates mismatches while applying the known-gap overlay and the
// re-export lossy allowlist. Every assertion — passing or failing — goes through
// emit, because detecting a stale allowlist entry needs the passes too.
type collector struct {
	out   []Mismatch
	gaps  map[string]KnownGap
	lossy map[string]struct{}
	// staleLossy holds lossy paths whose assertion PASSED: the writer no longer
	// loses them and the entry should go.
	staleLossy []string
	// usedLossy holds every lossy path an assertion was attempted on, passing or
	// failing. A lossy path missing from it addresses nothing in this sidecar.
	usedLossy map[string]struct{}
	treeDiff  func() string
	diffTaken bool
}

func newCollector(exp Expectation, reexport bool) *collector {
	c := &collector{
		gaps:      make(map[string]KnownGap, len(exp.KnownGaps)),
		usedLossy: make(map[string]struct{}),
	}
	for _, gap := range exp.KnownGaps {
		c.gaps[gap.Path] = gap
	}
	if reexport && exp.Writer != nil && len(exp.Writer.Lossy) > 0 {
		c.lossy = make(map[string]struct{}, len(exp.Writer.Lossy))
		for _, path := range exp.Writer.Lossy {
			c.lossy[path] = struct{}{}
		}
	}
	return c
}

func (c *collector) add(m Mismatch) {
	c.out = append(c.out, m)
}

// reportStaleLossy turns every lossy path that actually matched into a failure.
// An allowlist entry for a path the writer now reproduces is the same kind of
// lie as a missing assertion, so it is reported rather than tolerated.
// reportUnusedLossy fails every writer.lossy path that no assertion in this
// comparison ever reached. Such an entry looks like a reviewed exemption and is
// inert: it excuses nothing, and because nothing asserts it, it can never be
// reported stale either. Only a pass covering both legs can judge this, so it
// runs from CompareReexportAll alone.
func (c *collector) reportUnusedLossy(exp Expectation) {
	if exp.Writer == nil {
		return
	}
	for _, path := range exp.Writer.Lossy {
		if _, used := c.usedLossy[path]; used {
			continue
		}
		c.add(Mismatch{
			Path:   path,
			Detail: "listed in writer.lossy but no assertion addresses this path; it excuses nothing and can never be reported stale - fix the path or remove the entry",
		})
	}
}

func (c *collector) reportStaleLossy() {
	for _, path := range c.staleLossy {
		c.add(Mismatch{
			Path:   path,
			Detail: "listed in writer.lossy but matched on re-export; the allowlist is stale, remove this path",
		})
	}
}

// isLossy reports whether the path is allowlisted, and records that an
// assertion on it was reached. Every caller is about to assert the path, so
// asking the question IS the evidence that the entry addresses something.
func (c *collector) isLossy(path string) bool {
	if c.lossy == nil {
		return false
	}
	if _, ok := c.lossy[path]; !ok {
		return false
	}
	c.usedLossy[path] = struct{}{}
	return true
}

// emit records one assertion. A known gap on the path replaces the assertion
// entirely; a lossy path on a re-export suppresses it and tracks the pass.
func (c *collector) emit(path string, ok bool, want, got string) {
	if gap, isGap := c.gaps[path]; isGap {
		// A known gap overrides the allowlist, but the path is still reachable,
		// so the lossy entry is redundant rather than dead. Record it as used;
		// reporting it as addressing nothing would be wrong and confusing.
		c.isLossy(path)
		c.emitGap(path, gap, got)
		return
	}
	if c.isLossy(path) {
		if ok {
			c.staleLossy = append(c.staleLossy, path)
		}
		return
	}
	if !ok {
		c.add(Mismatch{Path: path, Want: want, Got: got})
	}
}

// emitStrict records an assertion that writer.lossy must never be able to
// suppress. It is for invariants a reviewed exemption has no business excusing:
// an allowlist entry justifies a known DIFFERENCE, never the loss of the data
// the difference is a spelling of.
func (c *collector) emitStrict(path string, ok bool, want, got string) {
	if gap, isGap := c.gaps[path]; isGap {
		c.emitGap(path, gap, got)
		return
	}
	if !ok {
		c.add(Mismatch{Path: path, Want: want, Got: got})
	}
}

func (c *collector) emitGap(path string, gap KnownGap, got string) {
	gapActual := formatRaw(gap.Actual)
	if got == gapActual {
		return
	}
	detail := "known gap value changed; recorded reason: " + gap.Reason
	if got == formatRaw(gap.Want) {
		detail = "known gap closed; update the sidecar"
	}
	c.add(Mismatch{Path: path, Want: gapActual, Got: got, Detail: detail})
}

// addStructural records a shape mismatch (wrong child count, missing node) and
// attaches the rendered tree diff to the FIRST one only. Attaching it to every
// field mismatch would bury the actual failure under repeated dumps.
func (c *collector) addStructural(m Mismatch) {
	if c.isLossy(m.Path) {
		return
	}
	if !c.diffTaken && c.treeDiff != nil {
		if diff := c.treeDiff(); diff != "" {
			m.Detail = diff
			c.diffTaken = true
		}
	}
	c.add(m)
}

func compareDocumentHeader(c *collector, exp *DocumentExpect, actual Actual) {
	if exp == nil {
		return
	}
	if exp.Width != nil {
		c.emit("document.width", *exp.Width == actual.Width, fmtInt(*exp.Width), fmtInt(actual.Width))
	}
	if exp.Height != nil {
		c.emit("document.height", *exp.Height == actual.Height, fmtInt(*exp.Height), fmtInt(actual.Height))
	}
	if exp.ColorMode != nil {
		c.emit("document.colorMode", *exp.ColorMode == actual.ColorMode, *exp.ColorMode, actual.ColorMode)
	}
	if exp.BitDepth != nil {
		c.emit("document.bitDepth", *exp.BitDepth == actual.BitDepth, fmtInt(*exp.BitDepth), fmtInt(actual.BitDepth))
	}
	if exp.Resolution != nil {
		ok := math.Abs(*exp.Resolution-actual.Resolution) <= 1e-9
		c.emit("document.resolution", ok, fmtFloat(*exp.Resolution), fmtFloat(actual.Resolution))
	}
	if exp.IsPSB != nil {
		c.emit("document.isPSB", *exp.IsPSB == actual.IsPSB, fmtBool(*exp.IsPSB), fmtBool(actual.IsPSB))
	}
}

// compareWarnings compares the warning sets as order-insensitive multisets and
// reports each difference on its own line. One extra warning must not produce a
// wall of diff against an otherwise identical list.
func compareWarnings(c *collector, exp *[]string, actual []string) {
	if exp == nil {
		return
	}
	want := multiset(*exp)
	got := multiset(actual)
	unexpected := excess(got, want)
	missing := excess(want, got)

	if c.isLossy("warnings") {
		if len(unexpected) == 0 && len(missing) == 0 {
			c.staleLossy = append(c.staleLossy, "warnings")
		}
		return
	}
	for _, warning := range unexpected {
		c.add(Mismatch{Path: "warnings", Detail: fmt.Sprintf("unexpected %q", warning)})
	}
	for _, warning := range missing {
		c.add(Mismatch{Path: "warnings", Detail: fmt.Sprintf("missing %q", warning)})
	}
}

func compareLayerLevel(c *collector, prefix string, exp []LayerExpect, actual []model.LayerNode) {
	if len(exp) != len(actual) {
		c.addStructural(Mismatch{
			Path: prefix,
			Want: strconv.Itoa(len(exp)) + " layers",
			Got:  strconv.Itoa(len(actual)) + " layers",
		})
	}
	for i := range exp {
		path := prefix + "[" + strconv.Itoa(i) + "]"
		if i >= len(actual) {
			c.addStructural(Mismatch{Path: path, Want: layerLabel(exp[i]), Got: "<absent>"})
			continue
		}
		compareLayerNode(c, path, exp[i], actual[i])
	}
}

func compareLayerNode(c *collector, path string, exp LayerExpect, node model.LayerNode) {
	if exp.Name != nil {
		c.emit(path+".name", *exp.Name == node.Name(), *exp.Name, node.Name())
	}
	if exp.Type != nil {
		got := string(node.LayerType())
		c.emit(path+".type", *exp.Type == got, *exp.Type, got)
	}
	if exp.BlendMode != nil {
		got := string(node.BlendMode())
		c.emit(path+".blendMode", *exp.BlendMode == got, *exp.BlendMode, got)
	}
	if exp.Opacity255 != nil {
		compareOpacity(c, path+".opacity255", *exp.Opacity255, node.Opacity())
	}
	if exp.FillOpacity255 != nil {
		compareOpacity(c, path+".fillOpacity255", *exp.FillOpacity255, node.FillOpacity())
	}
	if exp.Visible != nil {
		c.emit(path+".visible", *exp.Visible == node.Visible(), fmtBool(*exp.Visible), fmtBool(node.Visible()))
	}
	if exp.ClipToBelow != nil {
		c.emit(path+".clipToBelow", *exp.ClipToBelow == node.ClipToBelow(), fmtBool(*exp.ClipToBelow), fmtBool(node.ClipToBelow()))
	}
	if exp.ClippingBase != nil {
		c.emit(path+".clippingBase", *exp.ClippingBase == node.ClippingBase(), fmtBool(*exp.ClippingBase), fmtBool(node.ClippingBase()))
	}
	if exp.Bounds != nil {
		want := exp.Bounds.Bounds()
		got, hasBounds := nodeBounds(node)
		c.emit(path+".bounds", hasBounds && got == want, fmtBounds(want), boundsCell(node))
	}
	if exp.Isolated != nil {
		group, isGroup := node.(*model.GroupLayer)
		got := isGroup && group.Isolated
		c.emit(path+".isolated", isGroup && group.Isolated == *exp.Isolated, fmtBool(*exp.Isolated), fmtBool(got))
	}
	if exp.StyleKinds != nil {
		want := *exp.StyleKinds
		got := styleKinds(node)
		c.emit(path+".styleKinds", equalStrings(want, got), fmtList(want), fmtList(got))
	}
	if exp.Mask != nil {
		compareModelMask(c, path+".mask", *exp.Mask, node.Mask())
	}
	if exp.Children != nil {
		compareLayerLevel(c, path+".children", *exp.Children, node.Children())
	}
}

// compareOpacity converts the sidecar byte to the model's [0,1] float. The
// reported values stay in the sidecar's units so that a failure message can be
// pasted straight back into the JSON.
func compareOpacity(c *collector, path string, want255 int, got float64) {
	want := float64(want255) / 255
	c.emit(path, math.Abs(got-want) <= 1e-9, fmtInt(want255), fmtByte(got))
}

// compareModelMask asserts only what survives into the engine model. Rect,
// defaultFill and inverted are PSD-record scope: psdimport rasterizes the mask
// into a document-sized buffer and discards them.
func compareModelMask(c *collector, path string, exp MaskExpect, mask *model.LayerMask) {
	if exp.Present != nil {
		got := mask != nil
		c.emit(path+".present", *exp.Present == got, fmtBool(*exp.Present), fmtBool(got))
	}
	if exp.Enabled != nil {
		got := mask != nil && mask.Enabled
		c.emit(path+".enabled", *exp.Enabled == got, fmtBool(*exp.Enabled), fmtBool(got))
	}
}

func compareRecord(c *collector, path string, exp RecordExpect, slot int, record RecordView) {
	c.emit(path+".index", exp.Index == slot, fmtInt(exp.Index), fmtInt(slot))
	if exp.Name != nil {
		c.emit(path+".name", *exp.Name == record.Name, *exp.Name, record.Name)
	}
	if exp.SectionType != nil {
		c.emit(path+".sectionType", *exp.SectionType == record.SectionType, fmtInt(*exp.SectionType), fmtInt(record.SectionType))
	}
	if exp.PassThrough != nil {
		c.emit(path+".passThrough", *exp.PassThrough == record.PassThrough, fmtBool(*exp.PassThrough), fmtBool(record.PassThrough))
	}
	if exp.Bounds != nil {
		want := exp.Bounds.Bounds()
		c.emit(path+".bounds", want == record.Bounds, fmtBounds(want), fmtBounds(record.Bounds))
	}
	if exp.ChannelIDs != nil {
		want := *exp.ChannelIDs
		c.emit(path+".channelIds", equalInts(want, record.ChannelIDs), fmtInts(want), fmtInts(record.ChannelIDs))
		compareChannelIDSet(c, path+".channelIds.preserved", want, record.ChannelIDs)
	}
	if exp.Opacity255 != nil {
		compareOpacity(c, path+".opacity255", *exp.Opacity255, record.Opacity)
	}
	if exp.Visible != nil {
		c.emit(path+".visible", *exp.Visible == record.Visible, fmtBool(*exp.Visible), fmtBool(record.Visible))
	}
	if exp.Clipping != nil {
		c.emit(path+".clipping", *exp.Clipping == record.ClipToBelow, fmtBool(*exp.Clipping), fmtBool(record.ClipToBelow))
	}
	if exp.BlendMode != nil {
		c.emit(path+".blendMode", *exp.BlendMode == record.BlendMode, *exp.BlendMode, record.BlendMode)
	}
	if exp.PSDBlendKey != nil {
		c.emit(path+".psdBlendKey", *exp.PSDBlendKey == record.BlendKey, quoteKey(*exp.PSDBlendKey), quoteKey(record.BlendKey))
	}
	if exp.UnsupportedBlocks != nil {
		want := *exp.UnsupportedBlocks
		c.emit(path+".unsupportedBlocks", equalStrings(want, record.UnsupportedBlocks), fmtList(want), fmtList(record.UnsupportedBlocks))
	}
	compareRecordMask(c, path+".mask", exp.Mask, record)
}

// addedAlphaID is the only channel psdexport may introduce that the source file
// did not have: the engine model is RGBA, so a flattened source without alpha
// gains one on write.
const addedAlphaID = -1

// compareChannelIDSet asserts what the channel allowlist must never excuse: that
// every channel the source had is still there, and that the only one gained is
// alpha.
//
// The ordered channelIds assertion above is allowlistable, because psdexport
// emits a fixed order and the PSD spec does not prescribe one. But allowlisting
// a path suppresses the WHOLE assertion, so on its own it would also excuse a
// dropped -2 user mask or a corrupted ID — the exact regressions the sidecars
// claim are still caught. This path is emitted strictly, so no writer.lossy
// entry can reach it.
func compareChannelIDSet(c *collector, path string, want, got []int) {
	present := make(map[int]int, len(got))
	for _, id := range got {
		present[id]++
	}
	expected := make(map[int]int, len(want))
	for _, id := range want {
		expected[id]++
	}

	var missing, gained []int
	for _, id := range want {
		if expected[id] > present[id] {
			expected[id]--
			missing = append(missing, id)
		}
	}
	for _, id := range got {
		if id == addedAlphaID {
			continue
		}
		if present[id] > countInt(want, id) {
			present[id]--
			gained = append(gained, id)
		}
	}

	var detail []string
	if len(missing) > 0 {
		detail = append(detail, "missing "+fmtInts(missing))
	}
	if len(gained) > 0 {
		detail = append(detail, "unexpected "+fmtInts(gained))
	}
	if len(detail) == 0 {
		c.emitStrict(path, true, "", "")
		return
	}
	c.emitStrict(path, false, fmtInts(want)+" preserved", fmtInts(got)+" ("+strings.Join(detail, ", ")+")")
}

func countInt(values []int, want int) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func compareRecordMask(c *collector, path string, exp *MaskExpect, record RecordView) {
	if exp == nil {
		return
	}
	if exp.Present != nil {
		c.emit(path+".present", *exp.Present == record.HasMask, fmtBool(*exp.Present), fmtBool(record.HasMask))
	}
	if exp.Enabled != nil {
		c.emit(path+".enabled", *exp.Enabled == record.MaskEnabled, fmtBool(*exp.Enabled), fmtBool(record.MaskEnabled))
	}
	if exp.Rect != nil {
		want := exp.Rect.Bounds()
		c.emit(path+".rect", want == record.MaskBounds, fmtBounds(want), fmtBounds(record.MaskBounds))
	}
	if exp.DefaultFill != nil {
		c.emit(path+".defaultFill", *exp.DefaultFill == record.MaskDefault, fmtInt(*exp.DefaultFill), fmtInt(record.MaskDefault))
	}
	if exp.Inverted != nil {
		c.emit(path+".inverted", *exp.Inverted == record.MaskInverted, fmtBool(*exp.Inverted), fmtBool(record.MaskInverted))
	}
}

func compareLayerPixels(c *collector, samples []LayerPixelExpect, actual Actual) {
	for i, sample := range samples {
		path := "layerPixels[" + strconv.Itoa(i) + "]"
		node := findLayer(actual.Root, sample.Layer)
		if node == nil {
			c.emit(path+".layer", false, sample.Layer, "<no such layer>")
			continue
		}
		pixels, bounds, ok := layerRaster(node)
		if !ok {
			c.emit(path+".layer", false, sample.Layer+" (rasterized)", string(node.LayerType())+" layer has no raster")
			continue
		}
		x, y := sample.X, sample.Y
		if sample.Space == "document" {
			x -= bounds.X
			y -= bounds.Y
		}
		got, inside := samplePixel(pixels, bounds.W, bounds.H, x, y)
		if !inside {
			c.emit(path, false, fmtRGBA(sample.RGBA), fmt.Sprintf("<outside %dx%d raster>", bounds.W, bounds.H))
			continue
		}
		c.emit(path, rgbaWithin(sample.RGBA, got, sample.Tolerance), fmtRGBA(sample.RGBA), fmtRGBA(got))
	}
}

func compareMaskSamples(c *collector, samples []MaskSampleExpect, actual Actual) {
	for i, sample := range samples {
		path := "maskSamples[" + strconv.Itoa(i) + "]"
		node := findLayer(actual.Root, sample.Layer)
		if node == nil {
			c.emit(path+".layer", false, sample.Layer, "<no such layer>")
			continue
		}
		mask := node.Mask()
		if mask == nil {
			c.emit(path+".layer", false, sample.Layer+" (masked)", "<layer has no mask>")
			continue
		}
		index := sample.Y*mask.Width + sample.X
		if sample.X < 0 || sample.Y < 0 || sample.X >= mask.Width || sample.Y >= mask.Height || index >= len(mask.Data) {
			c.emit(path, false, fmtInt(sample.Value), fmt.Sprintf("<outside %dx%d mask>", mask.Width, mask.Height))
			continue
		}
		got := int(mask.Data[index])
		c.emit(path, withinTolerance(sample.Value, got, sample.Tolerance), fmtInt(sample.Value), fmtInt(got))
	}
}

func compareCompositePixels(c *collector, samples []PixelExpect, actual Actual) {
	for i, sample := range samples {
		path := "compositePixels[" + strconv.Itoa(i) + "]"
		got, inside := samplePixel(actual.Composite, actual.Width, actual.Height, sample.X, sample.Y)
		if !inside {
			c.emit(path, false, fmtRGBA(sample.RGBA), fmt.Sprintf("<outside %dx%d composite>", actual.Width, actual.Height))
			continue
		}
		c.emit(path, rgbaWithin(sample.RGBA, got, sample.Tolerance), fmtRGBA(sample.RGBA), fmtRGBA(got))
	}
}

// ---- helpers -------------------------------------------------------------

func childNodes(root *model.GroupLayer) []model.LayerNode {
	if root == nil {
		return nil
	}
	return root.Children()
}

// findLayer resolves a slash-joined name path ("Group A/Fill") against the
// tree. Names are the only stable identity a sidecar can use: layer IDs are
// regenerated on every import.
func findLayer(root *model.GroupLayer, path string) model.LayerNode {
	if root == nil || path == "" {
		return nil
	}
	segments := strings.Split(path, "/")
	nodes := root.Children()
	var found model.LayerNode
	for _, segment := range segments {
		found = nil
		for _, node := range nodes {
			if node.Name() == segment {
				found = node
				break
			}
		}
		if found == nil {
			return nil
		}
		nodes = found.Children()
	}
	return found
}

func nodeBounds(node model.LayerNode) (model.LayerBounds, bool) {
	switch typed := node.(type) {
	case *model.PixelLayer:
		return typed.Bounds, true
	case *model.TextLayer:
		return typed.Bounds, true
	case *model.VectorLayer:
		return typed.Bounds, true
	}
	return model.LayerBounds{}, false
}

func layerRaster(node model.LayerNode) ([]byte, model.LayerBounds, bool) {
	switch typed := node.(type) {
	case *model.PixelLayer:
		return typed.Pixels, typed.Bounds, true
	case *model.TextLayer:
		return typed.CachedRaster, typed.Bounds, true
	case *model.VectorLayer:
		return typed.CachedRaster, typed.Bounds, true
	}
	return nil, model.LayerBounds{}, false
}

func styleKinds(node model.LayerNode) []string {
	styles := node.StyleStack()
	if len(styles) == 0 {
		return nil
	}
	kinds := make([]string, 0, len(styles))
	for _, style := range styles {
		kinds = append(kinds, style.Kind)
	}
	return kinds
}

func samplePixel(buffer []byte, width, height, x, y int) ([4]int, bool) {
	if width <= 0 || height <= 0 || x < 0 || y < 0 || x >= width || y >= height {
		return [4]int{}, false
	}
	offset := (y*width + x) * 4
	if offset+4 > len(buffer) {
		return [4]int{}, false
	}
	return [4]int{int(buffer[offset]), int(buffer[offset+1]), int(buffer[offset+2]), int(buffer[offset+3])}, true
}

func rgbaWithin(want, got [4]int, tolerance int) bool {
	for i := range want {
		if !withinTolerance(want[i], got[i], tolerance) {
			return false
		}
	}
	return true
}

func withinTolerance(want, got, tolerance int) bool {
	delta := want - got
	if delta < 0 {
		delta = -delta
	}
	return delta <= tolerance
}

func multiset(values []string) map[string]int {
	counts := make(map[string]int, len(values))
	for _, value := range values {
		counts[value]++
	}
	return counts
}

// excess returns the values present in a more often than in b, expanded and
// sorted so the report is deterministic.
func excess(a, b map[string]int) []string {
	var out []string
	for value, count := range a {
		for i := count - b[value]; i > 0; i-- {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func layerLabel(exp LayerExpect) string {
	if exp.Path != "" {
		return exp.Path
	}
	if exp.Name != nil {
		return *exp.Name
	}
	return "<unnamed>"
}

func recordLabel(exp RecordExpect) string {
	if exp.Name != nil {
		return *exp.Name
	}
	return "record " + strconv.Itoa(exp.Index)
}

// quoteKey renders a raw blend key in quotes. The keys are space-padded to four
// characters, so an unquoted "mul " and "mul" would look identical in a failure
// message and hide the exact defect this assertion exists to catch.
func quoteKey(value string) string { return strconv.Quote(value) }

func fmtInt(value int) string { return strconv.Itoa(value) }

func fmtBool(value bool) string { return strconv.FormatBool(value) }

// fmtByte renders a model opacity in [0,1] as the 0..255 byte a sidecar stores.
func fmtByte(value float64) string {
	return strconv.Itoa(int(math.Round(value * 255)))
}

// fmtFloat renders integral floats without a decimal point so that a
// resolution of 72 reads as "72", matching the sidecar.
func fmtFloat(value float64) string {
	if value == math.Trunc(value) && math.Abs(value) < 1e15 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func fmtBounds(bounds model.LayerBounds) string {
	return fmt.Sprintf("%d,%d %dx%d", bounds.X, bounds.Y, bounds.W, bounds.H)
}

func fmtRGBA(rgba [4]int) string {
	return fmt.Sprintf("[%d,%d,%d,%d]", rgba[0], rgba[1], rgba[2], rgba[3])
}

func fmtList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return "[" + strings.Join(values, ",") + "]"
}

func fmtInts(values []int) string {
	if len(values) == 0 {
		return "[]"
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// formatRaw renders a known-gap JSON value in the same spelling the comparison
// uses for its Got strings, so the two can be compared as text.
func formatRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "<absent>"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return strings.TrimSpace(string(raw))
	}
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return fmtFloat(typed)
	default:
		compact, err := json.Marshal(typed)
		if err != nil {
			return strings.TrimSpace(string(raw))
		}
		return string(compact)
	}
}
