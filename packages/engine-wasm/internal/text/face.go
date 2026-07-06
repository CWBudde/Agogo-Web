// Package text implements a pure-Go, WASM-safe font engine for the
// rendering pipeline: font registration with style fallback (registry.go),
// glyph shaping with kerning and tracking (shape.go), face metrics
// (metrics.go), and glyph outline extraction into an abstract PathSink
// (outline.go). The package is model-free: it depends only on
// golang.org/x/image/font/sfnt and the embedded DejaVu fonts, and emits
// geometry through the PathSink interface, so callers own all rasterization.
//
// Coordinate convention: all emitted geometry is y-down document space
// (Photoshop convention). Glyph ink above the baseline has y < originY.
package text

import (
	"math"
	"sync"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Face wraps a parsed sfnt.Font together with its scratch buffer and
// per-face caches.
//
// Concurrency: sfnt.Buffer is not safe for concurrent use, so every method
// that touches the buffer or the caches serializes through an internal
// mutex. Calls are therefore safe from multiple goroutines (relevant for
// host-side tests) but never run in parallel on the same Face; in the
// single-threaded wasm build the mutex is uncontended. Tests must still not
// assume parallel speedup from sharing one Face across parallel subtests.
type Face struct {
	font       *sfnt.Font
	unitsPerEm float64

	mu  sync.Mutex // guards buf, caches, and post-table lazy init
	buf sfnt.Buffer

	// Lazily-read post-table underline values, normalized to positive
	// distance below the baseline (y-down), in font units.
	postRead           bool
	postUnderlinePos   float64
	postUnderlineThick float64
	postValid          bool

	// Per-face glyph caches; see glyph_cache.go.
	segCache  map[segKey]sfnt.Segments
	advCache  map[advKey]fixed.Int26_6
	kernCache map[kernKey]fixed.Int26_6
	segLoads  int // number of real sfnt LoadGlyph calls (test seam)
}

// newFace wraps a parsed font. It is the only Face constructor; registries
// create faces via Register.
func newFace(f *sfnt.Font) *Face {
	upem := float64(f.UnitsPerEm())
	if upem <= 0 {
		upem = 2048
	}
	return &Face{
		font:       f,
		unitsPerEm: upem,
		segCache:   make(map[segKey]sfnt.Segments),
		advCache:   make(map[advKey]fixed.Int26_6),
		kernCache:  make(map[kernKey]fixed.Int26_6),
	}
}

// UnitsPerEm returns the font's design units per em.
func (f *Face) UnitsPerEm() float64 { return f.unitsPerEm }

// GlyphIndex returns the glyph index for a rune. A rune the font does not
// cover maps to index 0 (.notdef) without error; errors indicate a broken
// font table.
func (f *Face) GlyphIndex(r rune) (sfnt.GlyphIndex, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.font.GlyphIndex(&f.buf, r)
}

// fixedFromFloat converts pixels to 26.6 fixed point. Together with
// floatFromFixed it is the only 26.6 conversion point in this package.
func fixedFromFloat(v float64) fixed.Int26_6 {
	return fixed.Int26_6(math.Round(v * 64))
}

// floatFromFixed converts 26.6 fixed point to pixels.
func floatFromFixed(v fixed.Int26_6) float64 {
	return float64(v) / 64
}
