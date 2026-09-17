// This file is in package psd_test, not package psd: the record view it needs
// is built by internal/io/psdfixture/psdrecords, which imports this package, and
// an in-package test file importing it would close an import cycle.
package psd_test

import (
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture/psdrecords"
)

// TestPSDFixtureCorpusParsesToExternallyDerivedRecords asserts the psdRecords
// scope: the facts that do not survive into the engine model. Mask rectangles,
// mask default fill, section-divider types and channel IDs are all discarded or
// transformed by psdimport, so this is the only place they can be checked
// against an externally derived expectation.
func TestPSDFixtureCorpusParsesToExternallyDerivedRecords(t *testing.T) {
	fixtures, err := psdfixture.Load()
	if err != nil {
		t.Fatalf("load fixture corpus: %v", err)
	}
	for _, fixture := range fixtures {
		if !fixture.Spec.Asserts(psdfixture.ScopePSDRecords) {
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			records, parseErr := psdrecords.Parse(fixture.Data)
			if parseErr != nil {
				t.Fatalf("Parse: %v", parseErr)
			}
			for _, mismatch := range psdfixture.CompareRecords(fixture.Spec, records) {
				t.Errorf("%s", mismatch)
			}
		})
	}
}
