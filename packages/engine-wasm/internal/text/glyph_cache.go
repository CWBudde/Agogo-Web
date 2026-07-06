package text

import (
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// glyphCacheCap bounds each per-face cache map. Eviction is drop-all: when
// a cache reaches the cap it is cleared and rebuilt from subsequent lookups.
// This is deliberately simple — a text document cycles through a small,
// stable working set of {glyph, ppem} pairs, so wholesale eviction is rare
// and an LRU would add bookkeeping for no practical gain.
const glyphCacheCap = 4096

type segKey struct {
	glyph sfnt.GlyphIndex
	ppem  fixed.Int26_6
}

type advKey struct {
	glyph sfnt.GlyphIndex
	ppem  fixed.Int26_6
}

type kernKey struct {
	left, right sfnt.GlyphIndex
	ppem        fixed.Int26_6
}

// segmentsLocked returns the outline segments for a glyph at a ppem,
// caching a deep copy (sfnt.LoadGlyph output is invalidated when the
// buffer is reused). The caller must hold f.mu. Load errors are not cached.
func (f *Face) segmentsLocked(glyph sfnt.GlyphIndex, ppem fixed.Int26_6) (sfnt.Segments, error) {
	key := segKey{glyph: glyph, ppem: ppem}
	if segs, ok := f.segCache[key]; ok {
		return segs, nil
	}
	raw, err := f.font.LoadGlyph(&f.buf, glyph, ppem, nil)
	f.segLoads++
	if err != nil {
		return nil, err
	}
	segs := append(sfnt.Segments(nil), raw...) // detach from f.buf
	if len(f.segCache) >= glyphCacheCap {
		f.segCache = make(map[segKey]sfnt.Segments)
	}
	f.segCache[key] = segs
	return segs, nil
}

// advanceLocked returns the cached horizontal advance for a glyph at a
// ppem. Lookup errors (out-of-range glyph index) are cached as zero: the
// glyph set of a font is immutable, so retrying cannot succeed. The caller
// must hold f.mu.
func (f *Face) advanceLocked(glyph sfnt.GlyphIndex, ppem fixed.Int26_6) fixed.Int26_6 {
	key := advKey{glyph: glyph, ppem: ppem}
	if adv, ok := f.advCache[key]; ok {
		return adv
	}
	adv, err := f.font.GlyphAdvance(&f.buf, glyph, ppem, font.HintingNone)
	if err != nil {
		adv = 0
	}
	if len(f.advCache) >= glyphCacheCap {
		f.advCache = make(map[advKey]fixed.Int26_6)
	}
	f.advCache[key] = adv
	return adv
}

// kernLocked returns the cached pair kern for (left, right) at a ppem.
// Errors — including the ErrNotFound a font without a kern table reports —
// are treated and cached as zero kern. The caller must hold f.mu.
func (f *Face) kernLocked(left, right sfnt.GlyphIndex, ppem fixed.Int26_6) fixed.Int26_6 {
	key := kernKey{left: left, right: right, ppem: ppem}
	if k, ok := f.kernCache[key]; ok {
		return k
	}
	k, err := f.font.Kern(&f.buf, left, right, ppem, font.HintingNone)
	if err != nil {
		k = 0
	}
	if len(f.kernCache) >= glyphCacheCap {
		f.kernCache = make(map[kernKey]fixed.Int26_6)
	}
	f.kernCache[key] = k
	return k
}

// segmentLoads reports how many times the face has called sfnt.LoadGlyph
// (as opposed to serving segments from cache). Test seam.
//
//nolint:unused // referenced only from _test.go files; lint runs with --tests=false
func (f *Face) segmentLoads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.segLoads
}
