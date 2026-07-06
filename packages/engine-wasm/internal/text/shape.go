package text

import (
	"unicode"

	"golang.org/x/image/font/sfnt"
)

// ShapedGlyph is one positioned glyph produced by ShapeLine.
type ShapedGlyph struct {
	Rune  rune            // source rune (original case, before small caps)
	Glyph sfnt.GlyphIndex // glyph to render; 0 is .notdef, still rendered
	X     float64         // pen offset from the line start, in pixels
	Size  float64         // per-glyph render size; differs from the line
	// size only for small-caps lowercase glyphs (0.7 * ShapeOptions.Size)
}

// ShapeOptions controls ShapeLine.
type ShapeOptions struct {
	// Size is the render size in pixels per em.
	Size float64
	// Tracking is extra spacing in pixels inserted between consecutive
	// glyphs (not after the last one), matching the engine's existing
	// pixel-tracking semantics.
	Tracking float64
	// Kerning is manual kerning in 1/1000 em added between every glyph
	// pair, on top of the font's own pair kerning. This mirrors
	// Photoshop's "Metrics" kerning plus a manual kerning value: the
	// font's kern table is always applied, and Kerning shifts every pair
	// by Kerning/1000 em (em = Size).
	Kerning float64
	// SmallCaps renders lowercase runes as their uppercase glyph at 0.7x
	// size.
	SmallCaps bool
}

// ShapeLine converts a single line of text into positioned glyphs and
// returns the total advance width in pixels. Line breaking, wrapping, and
// alignment are the caller's responsibility; text should contain no
// newlines.
//
// Per rune: the glyph index is looked up (missing runes map to .notdef,
// index 0, and are still emitted); the advance comes from GlyphAdvance at
// the glyph's own ppem; the font's pair kerning is always applied via
// Font.Kern (fonts without a kern table report an error there, which is
// treated as zero kern — GPOS-only fonts therefore show no pair kerning).
// For small-caps pairs where the sizes differ, the kern value is computed
// at the ppem of the LEFT glyph — a simplification that keeps mixed-size
// pairs cheap and deterministic.
//
// Ligatures are not supported: x/image/font/sfnt exposes no GSUB table, so
// substitutions like "fi" render as separate glyphs. Future work if a
// shaping engine with GSUB support is adopted.
func (f *Face) ShapeLine(text string, opts ShapeOptions) ([]ShapedGlyph, float64) {
	if text == "" || opts.Size <= 0 {
		return nil, 0
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	var (
		glyphs   []ShapedGlyph
		pen      float64
		prev     sfnt.GlyphIndex
		prevSize float64
		havePrev bool
	)
	manualKern := opts.Kerning / 1000 * opts.Size

	for _, r := range text {
		shapeRune := r
		renderSize := opts.Size
		if opts.SmallCaps && unicode.IsLower(r) {
			shapeRune = unicode.ToUpper(r)
			renderSize = 0.7 * opts.Size
		}

		glyph, err := f.font.GlyphIndex(&f.buf, shapeRune)
		if err != nil {
			glyph = 0 // broken cmap lookup: render .notdef
		}

		if havePrev {
			// Font pair kerning at the LEFT glyph's ppem (see doc above).
			pen += floatFromFixed(f.kernLocked(prev, glyph, fixedFromFloat(prevSize)))
			pen += manualKern
			pen += opts.Tracking
		}

		glyphs = append(glyphs, ShapedGlyph{Rune: r, Glyph: glyph, X: pen, Size: renderSize})
		pen += floatFromFixed(f.advanceLocked(glyph, fixedFromFloat(renderSize)))

		prev, prevSize, havePrev = glyph, renderSize, true
	}
	return glyphs, pen
}
