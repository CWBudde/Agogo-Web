//go:build !js

// Package psdfixture holds the external PSD fixture corpus and the machinery
// that judges Agogo against it (Phase S.10.1).
//
// # What the corpus is
//
// Every binary under testdata/corpus was produced by a PSD writer that is not
// Agogo, and every number asserted against it was derived by a PSD reader that
// is not Agogo. A fixture therefore states what a third party believes the file
// contains, not what Agogo happens to do with it. That is the whole point: the
// pre-existing round-trip tests passed because the reader and the writer shared
// their bugs, so they could not fail. A fixture whose expectations came out of
// Agogo would reproduce exactly that blind spot.
//
// Each binary <id>.psd (or .psb) is accompanied by <id>.expected.json, a
// sidecar matching the schema in schema.go. testdata/manifest.json lists the
// fixtures and the capability matrix they are meant to cover; a capability with
// no fixture yet carries a "deferred" reference into PLAN.md explaining why.
//
// # Rule 1: every non-test file in this package is //go:build !js
//
// The corpus is embedded. Production code must never reach it, because an
// accidental import would silently add the whole corpus to the engine.wasm
// binary that the browser downloads. The build tag turns that mistake into a
// loud `just wasm-build` failure instead of a quietly fatter download.
//
// # Rule 2: this package imports only stdlib and internal/model
//
// It must never import internal/io/psd, internal/io/psdimport,
// internal/io/psdexport or internal/engine. internal/io/psd's own tests live in
// `package psd` and are seeded from this corpus, so an import of psd here would
// be an import cycle. The comparison API is therefore expressed over neutral
// view structs (Actual, RecordView) that the calling test fills in; this
// package never parses a PSD itself.
//
// Corollary: no non-test file may import "testing" or accept a *testing.T.
// Comparison returns []Mismatch and the caller turns those into t.Errorf.
package psdfixture
