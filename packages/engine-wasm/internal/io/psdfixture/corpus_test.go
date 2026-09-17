package psdfixture

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Size budgets. The hard caps fail the build; the soft targets are only logged,
// because a corpus is worth carrying only while it stays cheap to clone.
const (
	hardMaxFixtureBytes = 128 * 1024
	hardMaxSidecarBytes = 32 * 1024
	hardMaxCorpusBytes  = 1024 * 1024

	softMaxFixtureBytes = 24 * 1024
	softMaxSidecarBytes = 8 * 1024
	softMaxCorpusBytes  = 512 * 1024
)

func mustLoad(t *testing.T) []Fixture {
	t.Helper()
	fixtures, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return fixtures
}

func mustLoadManifest(t *testing.T) ManifestFile {
	t.Helper()
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	return manifest
}

func corpusPath(t *testing.T) string {
	t.Helper()
	dir, err := SourceDir()
	if err != nil {
		t.Fatalf("SourceDir: %v", err)
	}
	return filepath.Join(dir, CorpusDir)
}

// TestCorpusIsNotEmpty is the honesty check for the whole phase. Every other
// test in this file passes vacuously on an empty corpus, so without this one a
// deleted corpus would look like a green suite.
func TestCorpusIsNotEmpty(t *testing.T) {
	fixtures := mustLoad(t)
	if len(fixtures) == 0 {
		t.Fatal("the PSD fixture corpus is empty; every fixture-backed assertion in this package is vacuous")
	}
}

// TestManifestAndCorpusAgree checks the inventory in both directions against
// the real filesystem, not the embedded copy: a binary added without rebuilding
// would be invisible to go:embed, and that is exactly the drift to catch.
func TestManifestAndCorpusAgree(t *testing.T) {
	manifest := mustLoadManifest(t)
	dir := corpusPath(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	onDisk := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !isFixtureBinary(entry.Name()) {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		onDisk[id] = entry.Name()
	}

	claimed := make(map[string]bool, len(manifest.Fixtures))
	for _, entry := range manifest.Fixtures {
		if claimed[entry.ID] {
			t.Errorf("manifest lists fixture %q twice", entry.ID)
		}
		claimed[entry.ID] = true

		binary := filepath.Join(dir, entry.File)
		data, err := os.ReadFile(binary) //nolint:gosec // path is built from the checked-in manifest
		if err != nil {
			t.Errorf("manifest entry %q: %v", entry.ID, err)
			continue
		}
		if got, want := strings.TrimSuffix(entry.File, filepath.Ext(entry.File)), entry.ID; got != want {
			t.Errorf("manifest entry %q: file %q does not match the id", entry.ID, entry.File)
		}
		if len(data) != entry.Bytes {
			t.Errorf("manifest entry %q: records %d bytes, file is %d bytes", entry.ID, entry.Bytes, len(data))
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != entry.SHA256 {
			t.Errorf("manifest entry %q: records sha256 %s, file is %s", entry.ID, entry.SHA256, got)
		}
		sidecar := filepath.Join(dir, entry.ID+SidecarSuffix)
		if _, err := os.Stat(sidecar); err != nil {
			t.Errorf("manifest entry %q: missing sidecar: %v", entry.ID, err)
		}
	}

	for id, name := range onDisk {
		if !claimed[id] {
			t.Errorf("%s is in the corpus but not in manifest.json", name)
		}
	}
}

// TestCorpusMatrixCoverage keeps the capability matrix honest: a capability is
// either covered by a fixture or carries a PLAN.md reference explaining why it
// cannot be.
func TestCorpusMatrixCoverage(t *testing.T) {
	manifest := mustLoadManifest(t)
	if len(manifest.Capabilities) == 0 {
		t.Fatal("manifest.json declares no capabilities; the matrix cannot be checked")
	}

	known := make(map[string]Capability, len(manifest.Capabilities))
	for _, capability := range manifest.Capabilities {
		if capability.ID == "" {
			t.Error("manifest.json has a capability with an empty id")
			continue
		}
		if capability.Title == "" {
			t.Errorf("capability %q has no title", capability.ID)
		}
		if _, exists := known[capability.ID]; exists {
			t.Errorf("capability %q is declared twice", capability.ID)
		}
		known[capability.ID] = capability
	}

	claimedBy := make(map[string][]string)
	for _, entry := range manifest.Fixtures {
		for _, id := range entry.Capabilities {
			if _, exists := known[id]; !exists {
				t.Errorf("fixture %q claims unknown capability %q", entry.ID, id)
				continue
			}
			claimedBy[id] = append(claimedBy[id], entry.ID)
		}
	}

	for _, capability := range manifest.Capabilities {
		covered := claimedBy[capability.ID]
		switch {
		case capability.Covered() && len(covered) == 0:
			t.Errorf("capability %q is not deferred but no fixture claims it; add a fixture or record a PLAN.md reason in \"deferred\"", capability.ID)
		case !capability.Covered() && len(covered) > 0:
			t.Errorf("capability %q is claimed by %s but is still marked deferred (%q); clear the deferred field", capability.ID, strings.Join(covered, ", "), capability.Deferred)
		}
	}
}

func TestEveryFixtureDeclaresItsAssertionScopes(t *testing.T) {
	known := make(map[string]bool, len(KnownScopes()))
	for _, scope := range KnownScopes() {
		known[scope] = true
	}

	for _, fixture := range mustLoad(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			spec := fixture.Spec
			if len(spec.Assert) == 0 {
				t.Fatal(`"assert" is empty; a fixture that asserts nothing cannot fail`)
			}
			seen := make(map[string]bool, len(spec.Assert))
			for _, scope := range spec.Assert {
				if !known[scope] {
					t.Errorf("unknown assertion scope %q (known: %s)", scope, strings.Join(KnownScopes(), ", "))
				}
				if seen[scope] {
					t.Errorf("assertion scope %q is listed twice", scope)
				}
				seen[scope] = true
			}
			for _, problem := range scopeDeclarationProblems(spec) {
				t.Error(problem)
			}
		})
	}
}

// scopesForbiddenWithImportError lists the scopes that cannot exist for a file
// the engine refuses to import. There is no document, no layer tree, no parsed
// record set, no pixel and no re-export behind a rejection, so asserting any of
// them would be claiming something the harness can never evaluate.
func scopesForbiddenWithImportError() []string {
	return []string{
		ScopeDocument,
		ScopeTree,
		ScopePSDRecords,
		ScopeLayerPixels,
		ScopeMaskSamples,
		ScopeCompositePixels,
		ScopeWriter,
	}
}

// scopeDeclarationProblems reports everything wrong with a sidecar's "assert"
// list, given what kind of fixture it is. It is a plain function rather than
// inline test code so that both directions of every rule can be unit-tested
// without inventing a fixture on disk.
func scopeDeclarationProblems(spec Expectation) []string {
	if !spec.ExpectsImportError() {
		var problems []string
		for _, scope := range RequiredScopes() {
			if !spec.Asserts(scope) {
				problems = append(problems, fmt.Sprintf("does not assert the required scope %q", scope))
			}
		}
		return problems
	}

	var problems []string
	if !spec.Asserts(ScopeImport) {
		problems = append(problems, fmt.Sprintf(
			`import.expect is %q but %q is missing from "assert"; the rejection is the only thing this fixture can assert, so it must declare it`,
			spec.Import.Expect, ScopeImport,
		))
	}
	for _, scope := range scopesForbiddenWithImportError() {
		if spec.Asserts(scope) {
			problems = append(problems, fmt.Sprintf(
				"asserts scope %q while import.expect is %q; a file that never imports has no %s to compare against",
				scope, spec.Import.Expect, scope,
			))
		}
	}
	if strings.TrimSpace(spec.Import.ErrorContains) == "" {
		problems = append(problems, fmt.Sprintf(
			"import.expect is %q but import.errorContains is empty; without it the fixture would pass on ANY error, including a panic turned into a generic message",
			spec.Import.Expect,
		))
	}
	return problems
}

func TestScopeRulesAcceptAnImportingFixture(t *testing.T) {
	spec := Expectation{Assert: []string{ScopeDocument, ScopeTree, ScopeWarnings, ScopeWriter}}
	if problems := scopeDeclarationProblems(spec); len(problems) != 0 {
		t.Errorf("a fixture asserting every required scope was rejected: %s", strings.Join(problems, "; "))
	}
}

func TestScopeRulesRejectAnImportingFixtureMissingARequiredScope(t *testing.T) {
	spec := Expectation{Assert: []string{ScopeDocument, ScopeWarnings}}
	problems := scopeDeclarationProblems(spec)
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly one about the missing %q scope", problems, ScopeTree)
	}
	if !strings.Contains(problems[0], ScopeTree) {
		t.Errorf("problem %q does not name the missing scope %q", problems[0], ScopeTree)
	}
}

func TestScopeRulesAcceptARejectionFixture(t *testing.T) {
	spec := Expectation{
		Assert: []string{ScopeImport},
		Import: &ImportExpect{Expect: ImportExpectError, ErrorContains: "unsupported PSD bit depth"},
	}
	if problems := scopeDeclarationProblems(spec); len(problems) != 0 {
		t.Errorf("a well-formed rejection fixture was rejected: %s", strings.Join(problems, "; "))
	}
}

// TestScopeRulesRejectARejectionFixtureWithoutTheImportScope covers one
// direction of the rule: stating the outcome in "import" without listing the
// scope would leave the harness free to skip the only assertion there is.
func TestScopeRulesRejectARejectionFixtureWithoutTheImportScope(t *testing.T) {
	spec := Expectation{
		Assert: []string{ScopeWarnings},
		Import: &ImportExpect{Expect: ImportExpectError, ErrorContains: "unsupported PSD bit depth"},
	}
	problems := scopeDeclarationProblems(spec)
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly one about the missing %q scope", problems, ScopeImport)
	}
	if !strings.Contains(problems[0], ScopeImport) {
		t.Errorf("problem %q does not name the missing scope %q", problems[0], ScopeImport)
	}
}

// TestScopeRulesRejectARejectionFixtureAssertingImportedData covers the other
// direction: a fixture cannot both refuse to import and claim facts that only a
// successful import could produce.
func TestScopeRulesRejectARejectionFixtureAssertingImportedData(t *testing.T) {
	for _, scope := range scopesForbiddenWithImportError() {
		t.Run(scope, func(t *testing.T) {
			spec := Expectation{
				Assert: []string{ScopeImport, scope},
				Import: &ImportExpect{Expect: ImportExpectError, ErrorContains: "unsupported PSD bit depth"},
			}
			problems := scopeDeclarationProblems(spec)
			if len(problems) != 1 {
				t.Fatalf("problems = %v, want exactly one about the forbidden scope %q", problems, scope)
			}
			if !strings.Contains(problems[0], scope) {
				t.Errorf("problem %q does not name the forbidden scope %q", problems[0], scope)
			}
		})
	}
}

func TestScopeRulesRejectARejectionFixtureWithoutErrorContains(t *testing.T) {
	spec := Expectation{
		Assert: []string{ScopeImport},
		Import: &ImportExpect{Expect: ImportExpectError, ErrorContains: "   "},
	}
	problems := scopeDeclarationProblems(spec)
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly one about the empty errorContains", problems)
	}
	if !strings.Contains(problems[0], "errorContains") {
		t.Errorf("problem %q does not mention errorContains", problems[0])
	}
}

func TestAssertedScopesHaveData(t *testing.T) {
	for _, fixture := range mustLoad(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			spec := fixture.Spec
			checks := []struct {
				scope   string
				present bool
			}{
				{ScopeDocument, spec.Document != nil},
				{ScopeTree, spec.Layers != nil},
				{ScopeWarnings, spec.Warnings != nil},
				{ScopePSDRecords, spec.Records != nil},
				{ScopeLayerPixels, len(spec.LayerPixels) > 0},
				{ScopeMaskSamples, len(spec.MaskSamples) > 0},
				{ScopeCompositePixels, len(spec.CompositePixels) > 0},
				{ScopeWriter, spec.Writer != nil},
				{ScopeImport, spec.Import != nil},
			}
			for _, check := range checks {
				if spec.Asserts(check.scope) && !check.present {
					t.Errorf("asserts scope %q but carries no data for it", check.scope)
				}
				if !spec.Asserts(check.scope) && check.present {
					t.Errorf("carries data for scope %q but does not list it in \"assert\"", check.scope)
				}
			}
			if spec.Import != nil {
				switch spec.Import.Expect {
				case ImportExpectSuccess, ImportExpectError:
				default:
					t.Errorf("import.expect is %q; want %q or %q", spec.Import.Expect, ImportExpectSuccess, ImportExpectError)
				}
			}
			if spec.Writer != nil {
				switch spec.Writer.Expect {
				case "match", "lossy":
				default:
					t.Errorf("writer.expect is %q; want \"match\" or \"lossy\"", spec.Writer.Expect)
				}
				switch spec.Writer.Format {
				case "psd", "psb":
				default:
					t.Errorf("writer.format is %q; want \"psd\" or \"psb\"", spec.Writer.Format)
				}
				if len(spec.Writer.Lossy) > 0 && strings.TrimSpace(spec.Writer.LossyReason) == "" {
					t.Errorf("writer.lossy lists %d path(s) but writer.lossyReason is empty; "+
						"an allowlisted loss without a recorded reason is indistinguishable "+
						"from data being dropped on purpose", len(spec.Writer.Lossy))
				}
				if spec.Writer.Expect == "match" && len(spec.Writer.Lossy) > 0 {
					t.Errorf("writer.expect is \"match\" but writer.lossy lists %d paths", len(spec.Writer.Lossy))
				}
				if spec.Writer.ExternalVerification == nil {
					t.Error("asserts the writer scope without an externalVerification block; see testdata/VERIFICATION.md")
				}
			}
			for _, gap := range spec.KnownGaps {
				if gap.Path == "" {
					t.Error("knownGaps entry with an empty path")
				}
				if strings.TrimSpace(gap.Reason) == "" {
					t.Errorf("knownGaps entry %q has no reason; an undocumented gap is indistinguishable from a bug", gap.Path)
				}
				if len(gap.Actual) == 0 {
					t.Errorf("knownGaps entry %q has no \"actual\" value to assert", gap.Path)
				}
			}
		})
	}
}

// TestTreeExpectationsAreComplete enforces the absent-field policy: when a
// fixture asserts the tree scope, every node must pin the cheap fields. A node
// that leaves them nil renders as "?" and silently asserts nothing.
func TestTreeExpectationsAreComplete(t *testing.T) {
	for _, fixture := range mustLoad(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			spec := fixture.Spec
			if !spec.Asserts(ScopeTree) || spec.Layers == nil {
				return
			}
			var walk func(nodes []LayerExpect, prefix string)
			walk = func(nodes []LayerExpect, prefix string) {
				for i, node := range nodes {
					path := prefix + "[" + itoa(i) + "]"
					if node.Path == "" {
						t.Errorf("%s: no path", path)
					}
					if node.Name == nil {
						t.Errorf("%s (%s): name is null", path, node.Path)
					}
					if node.Type == nil {
						t.Errorf("%s (%s): type is null", path, node.Path)
					}
					if node.BlendMode == nil {
						t.Errorf("%s (%s): blendMode is null", path, node.Path)
					}
					if node.Opacity255 == nil {
						t.Errorf("%s (%s): opacity255 is null", path, node.Path)
					}
					if node.Visible == nil {
						t.Errorf("%s (%s): visible is null", path, node.Path)
					}
					if node.Opacity255 != nil && (*node.Opacity255 < 0 || *node.Opacity255 > 255) {
						t.Errorf("%s (%s): opacity255 %d is outside 0..255", path, node.Path, *node.Opacity255)
					}
					if node.Children != nil {
						walk(*node.Children, path+".children")
					}
				}
			}
			walk(*spec.Layers, "layers")
		})
	}
}

func TestProvenanceIsComplete(t *testing.T) {
	for _, fixture := range mustLoad(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			p := fixture.Spec.Provenance
			required := map[string]string{
				"sourceTool":        p.SourceTool,
				"sourceToolVersion": p.SourceToolVersion,
				"generatedBy":       p.GeneratedBy,
				"generatedOn":       p.GeneratedOn,
				"author":            p.Author,
				"license":           p.License,
			}
			for field, value := range required {
				if strings.TrimSpace(value) == "" {
					t.Errorf("provenance.%s is empty", field)
				}
			}
			if !p.Redistributable {
				t.Error("provenance.redistributable is false; a fixture that cannot be redistributed must not be checked in")
			}
			if strings.Contains(strings.ToLower(p.SourceTool), "agogo") {
				t.Errorf("provenance.sourceTool is %q; the binary must come from a writer that is not Agogo", p.SourceTool)
			}
		})
	}
}

func TestExpectationSourceIsNotAgogo(t *testing.T) {
	for _, fixture := range mustLoad(t) {
		t.Run(fixture.ID, func(t *testing.T) {
			source := fixture.Spec.ExpectationSource
			if strings.TrimSpace(source.Tool) == "" {
				t.Fatal("expectationSource.tool is empty")
			}
			if strings.Contains(strings.ToLower(source.Tool), "agogo") {
				t.Errorf("expectationSource.tool is %q; expectations derived by Agogo assert only that Agogo agrees with itself", source.Tool)
			}
			if strings.TrimSpace(source.ToolVersion) == "" {
				t.Error("expectationSource.toolVersion is empty; an unversioned deriver is not reproducible")
			}
			if strings.TrimSpace(source.Script) == "" {
				t.Error("expectationSource.script is empty")
			}
			if strings.TrimSpace(source.DerivedOn) == "" {
				t.Error("expectationSource.derivedOn is empty")
			}
			if strings.EqualFold(source.Tool, fixture.Spec.Provenance.SourceTool) {
				t.Errorf("expectationSource.tool and provenance.sourceTool are both %q; the deriver must be a different implementation from the generator", source.Tool)
			}
		})
	}
}

func TestCorpusSizeBudget(t *testing.T) {
	dir := corpusPath(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	var total int64
	var names []string
	sizes := make(map[string]int64, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "README.md" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", entry.Name(), err)
		}
		names = append(names, entry.Name())
		sizes[entry.Name()] = info.Size()
		total += info.Size()
	}
	sort.Strings(names)

	for _, name := range names {
		size := sizes[name]
		hard, soft, kind := int64(hardMaxFixtureBytes), int64(softMaxFixtureBytes), "fixture"
		if strings.HasSuffix(name, SidecarSuffix) {
			hard, soft, kind = hardMaxSidecarBytes, softMaxSidecarBytes, "sidecar"
		}
		if size > hard {
			t.Errorf("%s: %d bytes exceeds the %s hard cap of %d bytes", name, size, kind, hard)
		} else if size > soft {
			t.Logf("%s: %d bytes is over the %s soft target of %d bytes", name, size, kind, soft)
		}
	}

	if total > hardMaxCorpusBytes {
		t.Errorf("corpus is %d bytes, over the hard cap of %d bytes", total, hardMaxCorpusBytes)
	}
	t.Logf("corpus: %d files, %d bytes total (soft target %d bytes, hard cap %d bytes)", len(names), total, softMaxCorpusBytes, hardMaxCorpusBytes)
}

func TestLoadManifestIsParseable(t *testing.T) {
	manifest := mustLoadManifest(t)
	if manifest.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", manifest.SchemaVersion, SchemaVersion)
	}
}

func TestDumpDirIsCreated(t *testing.T) {
	dir, err := DumpDir()
	if err != nil {
		t.Fatalf("DumpDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
	if base := filepath.Base(dir); base != "_dump" {
		t.Fatalf("dump dir base = %q, want %q", base, "_dump")
	}
}

func itoa(value int) string { return fmtInt(value) }
