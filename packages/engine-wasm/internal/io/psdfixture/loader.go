//go:build !js

package psdfixture

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// CorpusDir is the embedded directory holding the fixture binaries and their
// sidecars, relative to this package's source directory.
const CorpusDir = "testdata/corpus"

// SidecarSuffix is appended to a fixture id to form its sidecar filename.
const SidecarSuffix = ".expected.json"

// manifestJSON is the capability matrix and fixture inventory.
//
//go:embed testdata/manifest.json
var manifestJSON []byte

// corpusFS holds the fixture binaries and sidecars. The corpus directory
// always contains a README.md so this pattern matches even before the first
// binary lands; go:embed fails the build on a pattern that matches nothing.
//
//go:embed testdata/corpus
var corpusFS embed.FS

// Fixture is one corpus entry: the binary plus the expectations a non-Agogo
// reader derived from it.
type Fixture struct {
	// ID is the fixture's stable name, equal to the binary's base name.
	ID string
	// File is the binary's filename inside CorpusDir.
	File string
	// Data is the embedded binary.
	Data []byte
	// Spec is the parsed sidecar.
	Spec Expectation
}

// SidecarFile returns the fixture's sidecar filename inside CorpusDir.
func (f Fixture) SidecarFile() string {
	return f.ID + SidecarSuffix
}

// IsPSB reports whether the binary is a large-document (.psb) file.
func (f Fixture) IsPSB() bool {
	return strings.EqualFold(filepath.Ext(f.File), ".psb")
}

// Load reads every fixture out of the embedded corpus, sorted by ID.
//
// It is strict on purpose: a sidecar whose schemaVersion, id or file disagrees
// with the file it sits next to is an error, not a warning, because a corpus
// that silently drops or mismatches a fixture is exactly the kind of quiet
// pass this phase exists to remove.
func Load() ([]Fixture, error) {
	entries, err := corpusFS.ReadDir(CorpusDir)
	if err != nil {
		return nil, fmt.Errorf("psdfixture: read corpus: %w", err)
	}

	binaries := make(map[string]string, len(entries)) // id -> filename
	sidecars := make(map[string]string, len(entries)) // id -> filename
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("psdfixture: %s/%s: the corpus is flat; subdirectories are not supported", CorpusDir, entry.Name())
		}
		name := entry.Name()
		switch {
		case strings.HasSuffix(name, SidecarSuffix):
			sidecars[strings.TrimSuffix(name, SidecarSuffix)] = name
		case isFixtureBinary(name):
			id := strings.TrimSuffix(name, filepath.Ext(name))
			if previous, exists := binaries[id]; exists {
				return nil, fmt.Errorf("psdfixture: fixture id %q is claimed by both %q and %q", id, previous, name)
			}
			binaries[id] = name
		default:
			// Documentation (README.md) and nothing else.
			if name != "README.md" {
				return nil, fmt.Errorf("psdfixture: %s/%s: unexpected file; expected <id>.psd, <id>.psb, <id>%s or README.md", CorpusDir, name, SidecarSuffix)
			}
		}
	}

	ids := make([]string, 0, len(binaries))
	for id := range binaries {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	fixtures := make([]Fixture, 0, len(ids))
	for _, id := range ids {
		fixture, err := loadFixture(id, binaries[id], sidecars[id])
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture)
		delete(sidecars, id)
	}

	if len(sidecars) > 0 {
		orphans := make([]string, 0, len(sidecars))
		for _, name := range sidecars {
			orphans = append(orphans, name)
		}
		sort.Strings(orphans)
		return nil, fmt.Errorf("psdfixture: sidecar without a binary: %s", strings.Join(orphans, ", "))
	}

	return fixtures, nil
}

func loadFixture(id, binaryName, sidecarName string) (Fixture, error) {
	if sidecarName == "" {
		return Fixture{}, fmt.Errorf("psdfixture: %s has no sidecar; expected %s%s", binaryName, id, SidecarSuffix)
	}

	data, err := corpusFS.ReadFile(path.Join(CorpusDir, binaryName))
	if err != nil {
		return Fixture{}, fmt.Errorf("psdfixture: read %s: %w", binaryName, err)
	}
	raw, err := corpusFS.ReadFile(path.Join(CorpusDir, sidecarName))
	if err != nil {
		return Fixture{}, fmt.Errorf("psdfixture: read %s: %w", sidecarName, err)
	}

	var spec Expectation
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return Fixture{}, fmt.Errorf("psdfixture: parse %s: %w", sidecarName, err)
	}

	if spec.SchemaVersion != SchemaVersion {
		return Fixture{}, fmt.Errorf("psdfixture: %s declares schemaVersion %d, this build understands %d", sidecarName, spec.SchemaVersion, SchemaVersion)
	}
	if spec.ID != id {
		return Fixture{}, fmt.Errorf("psdfixture: %s declares id %q but its filename says %q", sidecarName, spec.ID, id)
	}
	if spec.File != binaryName {
		return Fixture{}, fmt.Errorf("psdfixture: %s declares file %q but sits next to %q", sidecarName, spec.File, binaryName)
	}

	return Fixture{ID: id, File: binaryName, Data: data, Spec: spec}, nil
}

func isFixtureBinary(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".psd", ".psb":
		return true
	}
	return false
}

// LoadManifest parses the embedded testdata/manifest.json.
func LoadManifest() (ManifestFile, error) {
	var manifest ManifestFile
	decoder := json.NewDecoder(strings.NewReader(string(manifestJSON)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return ManifestFile{}, fmt.Errorf("psdfixture: parse manifest.json: %w", err)
	}
	if manifest.SchemaVersion != SchemaVersion {
		return ManifestFile{}, fmt.Errorf("psdfixture: manifest.json declares schemaVersion %d, this build understands %d", manifest.SchemaVersion, SchemaVersion)
	}
	return manifest, nil
}

// SourceDir returns this package's directory on disk. Tests use it to reach
// the corpus through the real filesystem — the embedded copy cannot show a
// file that was added without rebuilding, which is precisely what the
// manifest/corpus agreement test needs to catch.
func SourceDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("psdfixture: runtime.Caller failed; cannot locate the package source directory")
	}
	return filepath.Dir(file), nil
}

var (
	dumpOnce sync.Once
	dumpPath string
	dumpErr  error
)

// DumpDir returns <SourceDir>/_dump, creating it on first use. Failing
// comparisons write their artifacts (re-exported files, rendered trees, pixel
// diffs) there so a CI failure can be reproduced locally.
//
// The directory is created exactly once per process: the suite runs with -race
// and parallel subtests, and a naked MkdirAll from every subtest is a data race
// on the same path. The leading underscore keeps _dump invisible to the go tool
// and to go:embed, so nothing in it can ever be compiled or embedded.
func DumpDir() (string, error) {
	dumpOnce.Do(func() {
		dir, err := SourceDir()
		if err != nil {
			dumpErr = err
			return
		}
		dir = filepath.Join(dir, "_dump")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			dumpErr = fmt.Errorf("psdfixture: create dump dir: %w", err)
			return
		}
		dumpPath = dir
	})
	return dumpPath, dumpErr
}
