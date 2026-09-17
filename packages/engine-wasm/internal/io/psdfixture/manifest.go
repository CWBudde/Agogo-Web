//go:build !js

package psdfixture

// ManifestFile is testdata/manifest.json: the capability matrix the corpus is
// meant to cover, plus the inventory of the fixtures that cover it.
type ManifestFile struct {
	SchemaVersion int             `json:"schemaVersion"`
	Capabilities  []Capability    `json:"capabilities"`
	Fixtures      []ManifestEntry `json:"fixtures"`
}

// Capability is one row of the PSD compatibility matrix.
type Capability struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Deferred is empty when the capability is expected to be covered by a
	// fixture. A non-empty value is a PLAN.md reference explaining why no
	// fixture covers it yet — typically "only Photoshop can author this".
	// TestCorpusMatrixCoverage requires every non-deferred capability to be
	// claimed by at least one fixture.
	Deferred string `json:"deferred,omitempty"`
}

// Covered reports whether the capability must be claimed by a fixture.
func (c Capability) Covered() bool {
	return c.Deferred == ""
}

// ManifestEntry is one fixture's inventory row. Bytes and SHA256 are recorded
// so that a silently re-generated or truncated binary fails the corpus tests
// instead of quietly changing what the suite asserts.
type ManifestEntry struct {
	ID           string   `json:"id"`
	File         string   `json:"file"`
	Capabilities []string `json:"capabilities"`
	Bytes        int      `json:"bytes"`
	SHA256       string   `json:"sha256"`
}
