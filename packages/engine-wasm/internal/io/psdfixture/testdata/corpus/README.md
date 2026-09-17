# Corpus

Fixture binaries and their expectation sidecars live directly in this
directory, flat:

- `<id>.psd` or `<id>.psb` — the binary, written by a tool that is not Agogo
- `<id>.expected.json` — the sidecar, derived by a reader that is not Agogo

Provenance, licensing and the capability matrix are documented one level up in
[`../README.md`](../README.md); the inventory is [`../manifest.json`](../manifest.json).

This file also keeps the directory non-empty: `loader.go` embeds it with
`//go:embed testdata/corpus`, and go:embed fails the build on a pattern that
matches nothing. Do not delete it, even once binaries land.
