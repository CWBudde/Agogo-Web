// Package buildinfo exposes version metadata injected at build time via -ldflags.
package buildinfo

import "runtime"

// These variables are set by the linker via -ldflags "-X ..." during wasm-build.
var (
	BuildTime = "dev"
	GoVersion = runtime.Version()
)
