//go:build !js

package psdfixture

import (
	"encoding/json"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

// SchemaVersion is the sidecar schema this package understands. Load rejects
// any sidecar or manifest that declares a different version rather than
// silently mis-reading it.
const SchemaVersion = 1

// Assertion scopes. A sidecar lists the scopes it covers in "assert"; a scope
// that is not listed is not checked, and a scope that is listed must carry its
// data (enforced by corpus_test.go).
const (
	ScopeDocument        = "document"
	ScopeTree            = "tree"
	ScopeWarnings        = "warnings"
	ScopePSDRecords      = "psdRecords"
	ScopeLayerPixels     = "layerPixels"
	ScopeMaskSamples     = "maskSamples"
	ScopeCompositePixels = "compositePixels"
	ScopeWriter          = "writer"
	ScopeImport          = "import"
)

// KnownScopes lists every scope name accepted in Expectation.Assert.
func KnownScopes() []string {
	return []string{
		ScopeDocument,
		ScopeTree,
		ScopeWarnings,
		ScopePSDRecords,
		ScopeLayerPixels,
		ScopeMaskSamples,
		ScopeCompositePixels,
		ScopeWriter,
		ScopeImport,
	}
}

// RequiredScopes are the scopes every importing sidecar must assert. They are
// the cheap ones: if a fixture cannot even state its document header, its layer
// tree and its warning set, it is not carrying its weight. A fixture that
// expects the import to fail is exempt — see ImportExpect.
func RequiredScopes() []string {
	return []string{ScopeDocument, ScopeTree, ScopeWarnings}
}

// Expectation is one <id>.expected.json sidecar.
//
// Optional fields are pointers throughout so that "the sidecar said nothing" is
// distinguishable from "the sidecar said zero". `Warnings: nil` means warnings
// are not asserted; `Warnings: &[]string{}` means the file must import clean.
// A nil field asserts nothing at all.
type Expectation struct {
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id"`
	File          string `json:"file"`
	Description   string `json:"description"`

	Provenance        Provenance        `json:"provenance"`
	ExpectationSource ExpectationSource `json:"expectationSource"`

	Assert []string `json:"assert"`

	Document *DocumentExpect `json:"document"`

	// Warnings is the EXACT set of import warnings, compared as an
	// order-insensitive multiset. Pointer so that [] ("must import clean")
	// differs from absent ("not asserted").
	Warnings *[]string `json:"warnings"`

	// Layers is the model-scope layer tree: the children of doc.LayerRoot,
	// bottom-to-top. A non-nil value makes the root level exhaustive in count
	// and order; see LayerExpect.Children for nested levels.
	Layers *[]LayerExpect `json:"layers"`

	// Records is the PSD-record scope: the flat layer records in file order.
	// Mask rectangles, default fills, section dividers and channel IDs live
	// here because the engine model is lossy for them.
	Records *[]RecordExpect `json:"psdRecords"`

	LayerPixels     []LayerPixelExpect `json:"layerPixels"`
	MaskSamples     []MaskSampleExpect `json:"maskSamples"`
	CompositePixels []PixelExpect      `json:"compositePixels"`

	Writer *WriterExpect `json:"writer"`
	Fuzz   *FuzzExpect   `json:"fuzz"`

	// Import pins the outcome of the import itself. Absent means "must import
	// successfully", which is what every other field in this struct assumes.
	Import *ImportExpect `json:"import,omitempty"`

	// KnownGaps records places where Agogo is knowingly wrong. The harness
	// asserts the recorded Actual value and fails when the value has become
	// Want, because the gap closed and the sidecar is now stale.
	KnownGaps []KnownGap `json:"knownGaps"`
}

// Asserts reports whether the sidecar claims to cover the given scope.
func (e Expectation) Asserts(scope string) bool {
	for _, declared := range e.Assert {
		if declared == scope {
			return true
		}
	}
	return false
}

// Accepted values for ImportExpect.Expect.
const (
	ImportExpectSuccess = "success"
	ImportExpectError   = "error"
)

// ImportExpect pins the outcome of the import itself. It exists for fixtures
// whose whole point is that Agogo must refuse them: a valid PSD that uses a
// feature the engine deliberately does not support.
type ImportExpect struct {
	// Expect is "success" (the default when the block is absent) or "error".
	Expect string `json:"expect"`
	// ErrorContains is a substring the returned error must contain. It is
	// required when Expect is "error", so a fixture cannot pass on ANY error -
	// a panic converted to a generic message must not satisfy it.
	ErrorContains string `json:"errorContains,omitempty"`
}

// ExpectsImportError reports whether the fixture must be refused by the engine.
// Such a fixture has no document, no layer tree and no re-export, so every
// other scope is meaningless for it.
func (e Expectation) ExpectsImportError() bool {
	return e.Import != nil && e.Import.Expect == ImportExpectError
}

// Provenance records who produced the binary. Never Agogo.
type Provenance struct {
	SourceTool        string `json:"sourceTool"`
	SourceToolVersion string `json:"sourceToolVersion"`
	GeneratedBy       string `json:"generatedBy"`
	GeneratedOn       string `json:"generatedOn"`
	Author            string `json:"author"`
	License           string `json:"license"`
	LicenseNote       string `json:"licenseNote"`
	Redistributable   bool   `json:"redistributable"`
}

// ExpectationSource records who derived the numbers. Never Agogo, and
// deliberately a different tool from the generator.
type ExpectationSource struct {
	Tool         string   `json:"tool"`
	ToolVersion  string   `json:"toolVersion"`
	Script       string   `json:"script"`
	DerivedOn    string   `json:"derivedOn"`
	Method       string   `json:"method"`
	ManualChecks []string `json:"manualChecks"`
}

// DocumentExpect is the document header scope.
type DocumentExpect struct {
	Width      *int     `json:"width"`
	Height     *int     `json:"height"`
	ColorMode  *string  `json:"colorMode"`
	BitDepth   *int     `json:"bitDepth"`
	Resolution *float64 `json:"resolution"`
	IsPSB      *bool    `json:"isPSB"`
}

// BoundsExpect is the sidecar spelling of a rectangle.
type BoundsExpect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Bounds converts the sidecar rectangle to the engine model's spelling.
func (b BoundsExpect) Bounds() model.LayerBounds {
	return model.LayerBounds{X: b.X, Y: b.Y, W: b.W, H: b.H}
}

// LayerExpect is one node of the model-scope layer tree.
type LayerExpect struct {
	// Path is the slash-joined name path ("Group A/Fill"). It is the stable
	// identity used by layerPixels and maskSamples.
	Path string `json:"path"`

	Name   *string       `json:"name"`
	Type   *string       `json:"type"`
	Bounds *BoundsExpect `json:"bounds"`

	BlendMode *string `json:"blendMode"`
	// PSDBlendKey is the raw four-character PSD key. Documentation only in this
	// scope; the engine model does not keep it, so it is asserted on
	// RecordExpect instead.
	PSDBlendKey string `json:"psdBlendKey"`

	Opacity255     *int  `json:"opacity255"`
	FillOpacity255 *int  `json:"fillOpacity255"`
	Visible        *bool `json:"visible"`
	ClipToBelow    *bool `json:"clipToBelow"`
	ClippingBase   *bool `json:"clippingBase"`

	// Isolated applies to groups only: a PSD "Normal" group is isolated,
	// a "Pass Through" group is not.
	Isolated *bool `json:"isolated"`

	// Expanded applies to groups only: the section divider's open/closed flag
	// (lsct 1 vs lsct 2). Optional, so the sidecars written before the engine
	// model carried the flag stay valid under DisallowUnknownFields.
	Expanded *bool `json:"expanded"`

	// AdjustmentKind and AdjustmentParams apply to adjustment layers only.
	// Both are optional, so the sidecars written before the corpus carried an
	// adjustment fixture stay valid under DisallowUnknownFields.
	//
	// AdjustmentParams is compared as JSON with the EXPECTED object's keys
	// driving the comparison, not as raw bytes: the engine writes a
	// json.RawMessage whose key order and whitespace are its own business,
	// while the values are the thing being asserted. A key the expectation
	// does not mention is not checked, which is what lets a sidecar pin the
	// fields that have an engine home without also pinning every default the
	// importer happens to fill in.
	AdjustmentKind   *string          `json:"adjustmentKind"`
	AdjustmentParams *json.RawMessage `json:"adjustmentParams"`

	Mask       *MaskExpect    `json:"mask"`
	StyleKinds *[]string      `json:"styleKinds"`
	Children   *[]LayerExpect `json:"children"`
}

// MaskExpect describes a layer mask. Present and Enabled are meaningful in
// both scopes; Rect, DefaultFill and Inverted are PSD-record scope only,
// because psdimport discards them when it builds the document-sized raster.
type MaskExpect struct {
	Present     *bool         `json:"present"`
	Enabled     *bool         `json:"enabled"`
	Rect        *BoundsExpect `json:"rect"`
	DefaultFill *int          `json:"defaultFill"`
	Inverted    *bool         `json:"inverted"`
}

// RecordExpect is one flat PSD layer record, in file order.
type RecordExpect struct {
	// Index is the record's position in the file. It is asserted against the
	// slot the record occupies, which keeps a hand-edited sidecar honest.
	Index int `json:"index"`

	Name        *string       `json:"name"`
	SectionType *int          `json:"sectionType"`
	PassThrough *bool         `json:"passThrough"`
	Bounds      *BoundsExpect `json:"bounds"`
	ChannelIDs  *[]int        `json:"channelIds"`
	Opacity255  *int          `json:"opacity255"`
	Visible     *bool         `json:"visible"`
	Clipping    *bool         `json:"clipping"`

	BlendMode *string `json:"blendMode"`
	// PSDBlendKey is the raw four-character key, trailing spaces significant
	// ("mul ", not "mul"). It is asserted here and nowhere else: BlendMode is
	// the normalised form and maps every unrecognised key onto "normal", so a
	// writer that emitted the wrong key for the right mode would pass on
	// BlendMode alone.
	PSDBlendKey *string `json:"psdBlendKey"`

	Mask              *MaskExpect `json:"mask"`
	UnsupportedBlocks *[]string   `json:"unsupportedBlocks"`
}

// PixelExpect is a single sampled pixel with a per-channel tolerance.
type PixelExpect struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	RGBA      [4]int `json:"rgba"`
	Tolerance int    `json:"tolerance"`
	Note      string `json:"note,omitempty"`
}

// LayerPixelExpect samples a pixel out of one layer's own raster.
type LayerPixelExpect struct {
	Layer string `json:"layer"`
	// Space is "layer" (the default) for layer-local coordinates, or
	// "document" for document coordinates.
	Space string `json:"space,omitempty"`
	PixelExpect
}

// MaskSampleExpect samples the document-sized mask raster of one layer.
type MaskSampleExpect struct {
	Layer     string `json:"layer"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Value     int    `json:"value"`
	Tolerance int    `json:"tolerance"`
	Note      string `json:"note,omitempty"`
}

// WriterExpect describes the re-export leg of the round trip.
type WriterExpect struct {
	// Format is "psd" or "psb".
	Format string `json:"format"`
	// Expect is "match" (re-export must reproduce every asserted field) or
	// "lossy" (the Lossy allowlist below is expected to differ).
	Expect string `json:"expect"`
	// Lossy is the reviewed allowlist of field paths that the writer is known
	// not to reproduce. A path that unexpectedly matches is itself reported,
	// so the allowlist cannot go stale silently.
	Lossy []string `json:"lossy"`
	// LossyReason records WHY those paths differ, with a PLAN.md reference. A
	// lossy allowlist without a stated reason is indistinguishable from data
	// being dropped on purpose, which is the failure mode this corpus exists to
	// prevent; corpus_test.go requires it whenever Lossy is non-empty.
	LossyReason          string                `json:"lossyReason,omitempty"`
	ExternalVerification *ExternalVerification `json:"externalVerification"`
}

// ExternalVerification records that a non-Agogo reader opened Agogo's output.
type ExternalVerification struct {
	Tool        string `json:"tool"`
	ToolVersion string `json:"toolVersion"`
	VerifiedOn  string `json:"verifiedOn"`
	Result      string `json:"result"`
	Notes       string `json:"notes"`
}

// FuzzExpect marks a fixture as a seed for the internal/io/psd fuzz targets.
type FuzzExpect struct {
	Seed bool `json:"seed"`
}

// KnownGap pins a field where Agogo is knowingly wrong. The harness asserts
// Actual; when the field has become Want the gap closed and the sidecar must
// be updated, which is reported as a mismatch rather than passing quietly.
type KnownGap struct {
	Path   string          `json:"path"`
	Want   json.RawMessage `json:"want"`
	Actual json.RawMessage `json:"actual"`
	Reason string          `json:"reason"`
}
