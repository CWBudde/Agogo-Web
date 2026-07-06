package text

import (
	"math"

	"golang.org/x/image/font"
)

// FaceMetrics holds face-wide vertical metrics scaled to a given render
// size, in pixels. The coordinate convention is y-down document space:
// Ascent extends above the baseline, Descent is a positive distance below
// it, and UnderlinePosition is a positive distance below the baseline (add
// it to the baseline y to place the underline top).
type FaceMetrics struct {
	Ascent             float64 // baseline to line top, > 0
	Descent            float64 // baseline to line bottom, positive-down
	LineGap            float64 // recommended extra space between lines
	XHeight            float64 // baseline to top of 'x'
	CapHeight          float64 // baseline to top of 'H'
	UnderlinePosition  float64 // baseline to underline top, positive-down
	UnderlineThickness float64 // underline stroke thickness
}

// Metrics returns the face metrics scaled to size pixels per em. When the
// font tables cannot be read it falls back to conventional heuristic
// proportions so callers never observe zero line height.
func (f *Face) Metrics(size float64) FaceMetrics {
	ppem := fixedFromFloat(size)

	f.mu.Lock()
	m, err := f.font.Metrics(&f.buf, ppem, font.HintingNone)
	f.mu.Unlock()

	var fm FaceMetrics
	if err != nil {
		fm = FaceMetrics{
			Ascent:    0.8 * size,
			Descent:   0.2 * size,
			XHeight:   0.5 * size,
			CapHeight: 0.7 * size,
		}
	} else {
		// sfnt reports XHeight and CapHeight in its y-down glyph space
		// (negative above the baseline); normalize to positive distances.
		fm = FaceMetrics{
			Ascent:    floatFromFixed(m.Ascent),
			Descent:   floatFromFixed(m.Descent),
			XHeight:   math.Abs(floatFromFixed(m.XHeight)),
			CapHeight: math.Abs(floatFromFixed(m.CapHeight)),
		}
		if gap := floatFromFixed(m.Height) - fm.Ascent - fm.Descent; gap > 0 {
			fm.LineGap = gap
		}
	}

	fm.UnderlinePosition, fm.UnderlineThickness = f.underline(size)
	return fm
}

// underline returns the underline position (positive-down distance below
// the baseline) and thickness in pixels for the given size. Values come
// from the font's post table; the table stores the position in font units
// with negative meaning below the baseline, so the sign is flipped here to
// this package's positive-down convention. Fonts without a usable post
// table fall back to position 0.15*size below the baseline and thickness
// size/20 (the engine's previous heuristics).
func (f *Face) underline(size float64) (position, thickness float64) {
	f.mu.Lock()
	if !f.postRead {
		f.postRead = true
		if post := f.font.PostTable(); post != nil {
			// post.UnderlinePosition: top of underline relative to the
			// baseline, negative below. Normalize to positive-below.
			f.postUnderlinePos = -float64(post.UnderlinePosition)
			f.postUnderlineThick = float64(post.UnderlineThickness)
			f.postValid = f.postUnderlinePos > 0 && f.postUnderlineThick > 0
		}
	}
	valid, pos, thick := f.postValid, f.postUnderlinePos, f.postUnderlineThick
	f.mu.Unlock()

	if !valid {
		return 0.15 * size, size / 20
	}
	scale := size / f.unitsPerEm
	return pos * scale, thick * scale
}
