package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture"
)

// TestPSDFixtureReexportSurvivesSelfReimport is the writer half of the honest
// round trip. Two things keep it from degenerating into a self-consistency test:
//
//  1. the re-import is compared against the EXTERNAL expectation, not against the
//     pre-export document — a bug shared by reader and writer cancels out in the
//     latter and does not in the former;
//  2. the AgogoProject bypass stays disabled, because SavePSD always embeds a
//     project archive and reading it back would restore the vacuity.
//
// The re-exported bytes are also dumped for external verification, since "opens
// in a reader that is not Agogo's own" cannot be asserted from inside Go.
func TestPSDFixtureReexportSurvivesSelfReimport(t *testing.T) {
	for _, fixture := range loadFixtureCorpus(t) {
		// A fixture the engine must refuse never becomes a Document, so there is
		// nothing to re-export; its assertion lives in the import harness.
		if fixture.Spec.ExpectsImportError() {
			continue
		}
		if !fixture.Spec.Asserts(psdfixture.ScopeWriter) {
			t.Errorf("%s: fixture does not assert the writer scope", fixture.ID)
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			imported, _, err := LoadPSDWithOptions(fixture.Data, PSDLoadOptions{IgnoreEmbeddedProject: true})
			if err != nil {
				t.Fatalf("import: %v", err)
			}

			format := "psd"
			if fixture.Spec.Writer != nil && fixture.Spec.Writer.Format != "" {
				format = fixture.Spec.Writer.Format
			}
			var reexported []byte
			if format == "psb" {
				reexported, err = SavePSB(imported)
			} else {
				reexported, err = SavePSD(imported)
			}
			if err != nil {
				t.Fatalf("export: %v", err)
			}

			if path := dumpFixtureBytes(t, fixture.ID, format, reexported); path != "" {
				t.Logf("wrote %s for external verification (just fixtures-verify)", path)
			}

			roundTripped, warnings, err := LoadPSDWithOptions(reexported, PSDLoadOptions{IgnoreEmbeddedProject: true})
			if err != nil {
				t.Fatalf("reimport of re-export: %v", err)
			}
			for _, mismatch := range psdfixture.CompareReexport(fixture.Spec, actualFromDocument(reexported, roundTripped, warnings, fixture.Spec)) {
				t.Errorf("%s", mismatch)
			}
		})
	}
}

// dumpFixtureBytes persists a re-exported fixture so a human or an external
// reader can open it. A dump failure is logged rather than fatal: the assertions
// above are the test, and the dump is an aid to reviewing them.
func dumpFixtureBytes(t *testing.T, id, format string, data []byte) string {
	t.Helper()
	dir, err := psdfixture.DumpDir()
	if err != nil {
		t.Logf("dump directory unavailable, skipping external-verification dump: %v", err)
		return ""
	}
	path := filepath.Join(dir, id+".reexport."+format)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Logf("write dump %s: %v", path, err)
		return ""
	}
	return path
}
