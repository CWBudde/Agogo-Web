package psd

import (
	"slices"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture"
)

// Truncation budgets. Cutting every fixture at every byte offset is what this
// test would like to do, and it is too slow: measured over the 20-fixture /
// 114,594-byte corpus of 2026-09-17 it cost ~1.7s plain but ~14s under
// `-race`, which is what CI runs. The sweep is therefore budgeted per fixture.
// A fixture smaller than the budget still gets every offset, so the small files
// are covered exhaustively for free.
//
// truncationOffsetBudget is the cap on offsets sampled from one fixture beyond
// the dense prefix; truncationDensePrefix is the leading run of offsets that is
// always cut at every byte, because the 26-byte header and the first section
// length prefixes live there and are where a bounds bug is most likely.
//
// This is the repository's first use of `testing.Short`. The convention it
// introduces: the default run is the thorough one, and `-short` trades sweep
// width for wall time. Nothing skips entirely under `-short` — every fixture is
// still cut — so a `-short` run is a weaker version of the same assertion, not
// a different one.
const (
	truncationOffsetBudget      = 384
	truncationOffsetBudgetShort = 48
	truncationDensePrefix       = 64
	truncationDensePrefixShort  = 32
)

// TestParseSurvivesTruncationOfEveryFixture cuts every corpus fixture short at
// many offsets and asserts the parser's hostile-input contract: a truncated
// file either fails with an error, or returns a partial result that carries
// warnings — and never panics, hangs, or returns a silently-clean document.
//
// The assertion is deliberately looser than the every-offset synthetic test in
// hardening_test.go, which asserts an error at every offset of a 43-byte file.
// A real file is large enough that a cut inside the layer section can leave a
// usable header, resources and layer list behind, and returning that partial
// document plus a warning is a legitimate outcome here.
func TestParseSurvivesTruncationOfEveryFixture(t *testing.T) {
	fixtures, err := psdfixture.Load()
	if err != nil {
		t.Fatalf("load fixture corpus: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("fixture corpus is empty")
	}

	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			offsets := truncationOffsets(fixture.Data, testing.Short())
			for _, offset := range offsets {
				// Parse recovers internally, so a panic escaping here means the
				// panic happened outside that guard — which is exactly the Wasm
				// trap this contract exists to prevent.
				result, err := Parse(fixture.Data[:offset])
				if err != nil {
					continue
				}
				if len(result.Warnings) == 0 {
					t.Errorf("Parse accepted a truncation at offset %d/%d with no error and no warnings",
						offset, len(fixture.Data))
				}
			}
			t.Logf("cut at %d of %d offsets", len(offsets), len(fixture.Data))
		})
	}
}

// truncationOffsets picks the cut points for one fixture: every offset in the
// dense prefix, every section boundary (and its immediate neighbours), and then
// an even stride across the rest up to the budget. Offsets are strictly below
// len(data) — cutting at len(data) is not a truncation.
func truncationOffsets(data []byte, short bool) []int {
	budget, dense := truncationOffsetBudget, truncationDensePrefix
	if short {
		budget, dense = truncationOffsetBudgetShort, truncationDensePrefixShort
	}

	size := len(data)
	selected := make(map[int]struct{}, budget+dense)
	add := func(offset int) {
		if offset > 0 && offset < size {
			selected[offset] = struct{}{}
		}
	}

	for offset := range min(dense, size) {
		add(offset)
	}
	// The section boundaries are where a length field hands the parser a cursor;
	// a cut one byte either side of one is worth more than a cut mid-payload.
	if sections, ok := splitSections(data); ok {
		for _, boundary := range []int{psdHeaderLen, sections.layerAndMaskStart, sections.layerAndMaskEnd} {
			add(boundary - 1)
			add(boundary)
			add(boundary + 1)
		}
	}
	if remaining := size - dense; remaining > 0 && budget > 0 {
		stride := max(remaining/budget, 1)
		for offset := dense; offset < size; offset += stride {
			add(offset)
		}
	}

	offsets := make([]int, 0, len(selected))
	for offset := range selected {
		offsets = append(offsets, offset)
	}
	// Sorting is not required for correctness, but a failure message that walks
	// the file in order is far easier to read.
	slices.Sort(offsets)
	return offsets
}
