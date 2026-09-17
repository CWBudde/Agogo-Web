package engine

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"testing"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture"
)

// loadFixtureCorpus returns the external fixture corpus. It never skips: an
// unreadable or empty corpus is a failure, because a PSD interop test that
// quietly does nothing is the exact situation Phase S.10.1 exists to end.
func loadFixtureCorpus(t *testing.T) []psdfixture.Fixture {
	t.Helper()
	fixtures, err := psdfixture.Load()
	if err != nil {
		t.Fatalf("load fixture corpus: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("fixture corpus is empty; import assertions would be vacuous")
	}
	return fixtures
}

// actualFromDocument builds the neutral view psdfixture compares against. The
// composite surface is rendered only when the sidecar asks for pixel samples,
// because rendering is the expensive part.
func actualFromDocument(data []byte, doc *Document, warnings []string, spec psdfixture.Expectation) psdfixture.Actual {
	actual := psdfixture.Actual{
		IsPSB:      isPSBStream(data),
		Width:      doc.Width,
		Height:     doc.Height,
		Resolution: doc.Resolution,
		ColorMode:  doc.ColorMode,
		BitDepth:   doc.BitDepth,
		Root:       doc.LayerRoot,
		Warnings:   warnings,
	}
	if spec.Asserts(psdfixture.ScopeCompositePixels) {
		actual.Composite = doc.renderCompositeSurface()
	}
	return actual
}

// isPSBStream reads the container version straight from the header, because the
// engine Document does not carry it: a PSB imports into the same Document type
// as a PSD, so the only honest source for the isPSB expectation is the bytes.
func isPSBStream(data []byte) bool {
	const versionOffset = 4
	if len(data) < versionOffset+2 {
		return false
	}
	return binary.BigEndian.Uint16(data[versionOffset:versionOffset+2]) == 2
}

// assertImportRefused checks a negative fixture: the import must fail, and it
// must fail with the error the sidecar names. Any-error-will-do is not an
// assertion — LoadPSDWithOptions turns a panic into "invalid PSD data: ...",
// which would otherwise satisfy a fixture that exists to prove a deliberate,
// actionable refusal.
func assertImportRefused(t *testing.T, spec psdfixture.Expectation, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("LoadPSDWithOptions succeeded, but this fixture must be refused with an error containing %q", spec.Import.ErrorContains)
	}
	if !strings.Contains(err.Error(), spec.Import.ErrorContains) {
		t.Fatalf("LoadPSDWithOptions failed with %q, which does not contain the expected %q", err.Error(), spec.Import.ErrorContains)
	}
}

// TestPSDFixtureCorpusImportsAsExternallyDerived is the honest import test: every
// assertion comes from a sidecar derived by a non-Agogo reader from a file
// written by a non-Agogo writer, and the AgogoProject bypass is disabled so the
// result can only come from parsing the PSD itself.
func TestPSDFixtureCorpusImportsAsExternallyDerived(t *testing.T) {
	for _, fixture := range loadFixtureCorpus(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			doc, warnings, err := LoadPSDWithOptions(fixture.Data, PSDLoadOptions{
				IgnoreEmbeddedProject: true,
			})
			if fixture.Spec.ExpectsImportError() {
				assertImportRefused(t, fixture.Spec, err)
				return
			}
			if err != nil {
				t.Fatalf("LoadPSDWithOptions: %v", err)
			}
			// Errorf, not Fatalf: one fixture should report every mismatch it has
			// in a single run rather than one per iteration.
			for _, mismatch := range psdfixture.CompareDocument(fixture.Spec, actualFromDocument(fixture.Data, doc, warnings, fixture.Spec)) {
				t.Errorf("%s", mismatch)
			}
		})
	}
}

// TestPSDFixtureCorpusCarriesNoEmbeddedAgogoProject keeps the corpus foreign. A
// fixture carrying an AgogoProject resource would let the import succeed through
// the embedded archive and silently restore the vacuous round trip.
func TestPSDFixtureCorpusCarriesNoEmbeddedAgogoProject(t *testing.T) {
	for _, fixture := range loadFixtureCorpus(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			if bytes.Contains(fixture.Data, []byte("AgogoProject")) {
				t.Errorf("fixture carries an AgogoProject resource; it is not a foreign file")
			}
			// A rejection fixture is refused before the resource block is
			// reached, so the byte scan above is the whole check for it.
			if fixture.Spec.ExpectsImportError() {
				return
			}
			result, err := psdio.Parse(fixture.Data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(result.Resources.AgogoProject) > 0 {
				t.Errorf("fixture carries %d bytes of embedded Agogo project data", len(result.Resources.AgogoProject))
			}
		})
	}
}

// TestPSDFixtureImportIgnoresEmbeddedProjectFlagForForeignFiles proves the bypass
// is inert here: with a genuinely foreign file there is nothing to bypass, so
// both code paths must agree. If they ever diverge, a fixture stopped being
// foreign.
func TestPSDFixtureImportIgnoresEmbeddedProjectFlagForForeignFiles(t *testing.T) {
	for _, fixture := range loadFixtureCorpus(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			strict, strictWarnings, strictErr := LoadPSDWithOptions(fixture.Data, PSDLoadOptions{IgnoreEmbeddedProject: true})
			relaxed, relaxedWarnings, relaxedErr := LoadPSDWithOptions(fixture.Data, PSDLoadOptions{})
			if (strictErr == nil) != (relaxedErr == nil) {
				t.Fatalf("bypass changed the outcome: strict err = %v, relaxed err = %v", strictErr, relaxedErr)
			}
			if fixture.Spec.ExpectsImportError() {
				if strictErr == nil {
					t.Fatalf("both code paths imported a fixture that must be refused; want an error containing %q", fixture.Spec.Import.ErrorContains)
				}
				assertImportRefused(t, fixture.Spec, strictErr)
				assertImportRefused(t, fixture.Spec, relaxedErr)
				return
			}
			if strictErr != nil {
				return
			}
			if got, want := psdfixture.RenderTree(relaxed.LayerRoot), psdfixture.RenderTree(strict.LayerRoot); got != want {
				t.Errorf("bypass changed the layer tree:\n%s", psdfixture.UnifiedDiff(want, got))
			}
			// Compare contents, not just the count: two different warnings of the
			// same length would otherwise pass. Order is not significant.
			strictSorted := slices.Sorted(slices.Values(strictWarnings))
			relaxedSorted := slices.Sorted(slices.Values(relaxedWarnings))
			if !slices.Equal(strictSorted, relaxedSorted) {
				t.Errorf("bypass changed the warning set:\n strict  %q\n relaxed %q", strictSorted, relaxedSorted)
			}
		})
	}
}
