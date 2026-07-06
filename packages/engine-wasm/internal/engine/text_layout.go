package engine

import (
	"math"
	"strings"

	text "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/text"
	"golang.org/x/image/font/sfnt"
)

// textDefaultLeading is the default line-height multiplier when a TextLayer
// carries no explicit Leading value.
const textDefaultLeading = 1.2

// textBoundsPad is the anti-aliasing padding (in pixels) added on every side
// of the computed tight bounds of point text.
const textBoundsPad = 2.0

// positionedGlyph is a single glyph placed in ANCHOR-RELATIVE document space
// (y-down): x/baselineY are offsets from the layer's (AnchorX, AnchorY) pen
// origin, so translating a layer never changes its layout.
type positionedGlyph struct {
	glyph     sfnt.GlyphIndex
	x         float64
	baselineY float64
	size      float64 // per-glyph render size (small-caps lowercase shrink)
}

// textDecorSpan is the horizontal extent of one rendered line, used to place
// underline/strikethrough bars. Coordinates are anchor-relative.
type textDecorSpan struct {
	x0, x1    float64
	baselineY float64
}

// textLayout is the pure (agg-free) result of laying out a TextLayer: shaped
// glyph positions, per-line decoration spans and the overall extents, all in
// anchor-relative document space.
type textLayout struct {
	face       *text.Face
	fontSize   float64          // layer font size (unscaled by super/subscript)
	metrics    text.FaceMetrics // at fontSize
	lineHeight float64

	glyphs []positionedGlyph
	spans  []textDecorSpan

	minX, minY, maxX, maxY float64 // union of line extents; valid iff hasExtent
	hasExtent              bool
}

// textLayerStyleFlags resolves the effective bold/italic request of a layer.
// The boolean fields are authoritative; a FontStyle string that encodes the
// style (e.g. "Bold Italic", "oblique") is OR-ed in, case-insensitively.
func textLayerStyleFlags(tl *TextLayer) (bold, italic bool) {
	style := strings.ToLower(tl.FontStyle)
	bold = tl.Bold || strings.Contains(style, "bold")
	italic = tl.Italic || strings.Contains(style, "italic") || strings.Contains(style, "oblique")
	return bold, italic
}

// resolveTextFace resolves the layer's font through the default registry.
// Unknown families fall back to DejaVu Sans (see text.Registry.Resolve); the
// default registry never returns nil.
func resolveTextFace(tl *TextLayer) *text.Face {
	bold, italic := textLayerStyleFlags(tl)
	return text.DefaultRegistry().Resolve(tl.FontFamily, bold, italic)
}

// textShapeParams derives the shared shaping parameters of a layer:
// the resolved face, effective font size, metrics at that size, line height,
// shaping options and the per-glyph baseline adjustment.
//
// Superscript/Subscript follow the Photoshop-ish convention: glyphs render at
// 2/3 of the font size and the baseline shifts up/down by FontSize/3 (when
// both flags are set, superscript wins). BaselineShift raises text for
// positive values (y-down document space, so it subtracts).
func textShapeParams(tl *TextLayer) (face *text.Face, fontSize float64, metrics text.FaceMetrics, lineHeight float64, opts text.ShapeOptions, baselineAdjust float64) {
	fontSize = tl.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}
	face = resolveTextFace(tl)
	metrics = face.Metrics(fontSize)
	leading := tl.Leading
	if leading <= 0 {
		leading = textDefaultLeading
	}
	lineHeight = fontSize * leading

	glyphSize := fontSize
	baselineAdjust = -tl.BaselineShift
	switch {
	case tl.Superscript:
		glyphSize *= 2.0 / 3.0
		baselineAdjust -= fontSize / 3
	case tl.Subscript:
		glyphSize *= 2.0 / 3.0
		baselineAdjust += fontSize / 3
	}
	opts = text.ShapeOptions{
		Size:      glyphSize,
		Tracking:  tl.Tracking,
		Kerning:   tl.Kerning,
		SmallCaps: tl.SmallCaps,
	}
	return face, fontSize, metrics, lineHeight, opts, baselineAdjust
}

// layoutTextLayer lays out a TextLayer's text into positioned glyphs and
// decoration spans in anchor-relative document space. Point text splits on
// explicit newlines and aligns each line relative to the anchor (center/right
// alignment produces negative x — the tight bounds absorb it); area text
// wraps within the layer's frame width with indent/justify support.
//
// AllCaps is a plain uppercase pre-pass; SmallCaps is handled by the shaper
// (uppercase glyphs at reduced size) and must NOT also uppercase the string.
func layoutTextLayer(tl *TextLayer) *textLayout {
	face, fontSize, metrics, lineHeight, opts, baselineAdjust := textShapeParams(tl)
	l := &textLayout{
		face:       face,
		fontSize:   fontSize,
		metrics:    metrics,
		lineHeight: lineHeight,
	}

	content := tl.Text
	if tl.AllCaps {
		content = strings.ToUpper(content)
	}
	if content == "" {
		return l
	}

	if tl.TextType == "area" && tl.Bounds.W > 0 {
		l.layoutArea(tl, content, opts, baselineAdjust)
	} else {
		l.layoutPoint(tl, content, opts, baselineAdjust)
	}
	return l
}

// layoutPoint lays out point text: one line per explicit newline, first
// baseline at Ascent below the anchor top.
func (l *textLayout) layoutPoint(tl *TextLayer, content string, opts text.ShapeOptions, baselineAdjust float64) {
	y := l.metrics.Ascent
	for _, line := range strings.Split(content, "\n") {
		l.addLine(line, 0, y+baselineAdjust, tl.Alignment, 0, opts)
		y += l.lineHeight
	}
}

// layoutArea lays out area text: paragraphs split on blank lines, words
// wrapped to the frame width minus indents, optional justification.
func (l *textLayout) layoutArea(tl *TextLayer, content string, opts text.ShapeOptions, baselineAdjust float64) {
	maxWidth := float64(tl.Bounds.W)
	y := l.metrics.Ascent

	for pi, para := range splitParagraphs(content) {
		if pi > 0 {
			y += tl.SpaceBefore + tl.SpaceAfter
		}
		words := splitWords(para)
		if len(words) == 0 {
			// Empty paragraph — just advance by one line height.
			y += l.lineHeight
			continue
		}

		availWidth := maxWidth - tl.IndentLeft - tl.IndentRight
		firstAvail := availWidth - tl.IndentFirst
		lines := l.wrapWords(words, firstAvail, availWidth, opts)

		for li, line := range lines {
			lineX := tl.IndentLeft
			curAvail := availWidth
			if li == 0 {
				lineX += tl.IndentFirst
				curAvail = firstAvail
			}
			isLastLine := li == len(lines)-1

			if tl.Alignment == "justify" && !isLastLine && len(splitWords(line)) > 1 {
				l.addJustifiedLine(line, lineX, y+baselineAdjust, curAvail, opts)
			} else {
				l.addLine(line, lineX, y+baselineAdjust, tl.Alignment, curAvail, opts)
			}
			y += l.lineHeight
		}
	}
}

// addLine shapes one line and appends its glyphs, aligned at x within
// availWidth (or relative to x itself when availWidth <= 0 — point text).
func (l *textLayout) addLine(line string, x, baselineY float64, alignment string, availWidth float64, opts text.ShapeOptions) {
	glyphs, width := l.face.ShapeLine(line, opts)
	if len(glyphs) == 0 {
		return
	}
	startX := x
	switch alignment {
	case "center":
		if availWidth > 0 {
			startX = x + (availWidth-width)/2
		} else {
			startX = x - width/2
		}
	case "right":
		if availWidth > 0 {
			startX = x + availWidth - width
		} else {
			startX = x - width
		}
	}
	for _, g := range glyphs {
		l.glyphs = append(l.glyphs, positionedGlyph{glyph: g.Glyph, x: startX + g.X, baselineY: baselineY, size: g.Size})
	}
	l.addSpan(startX, startX+width, baselineY)
}

// addJustifiedLine distributes the extra space of availWidth between words.
func (l *textLayout) addJustifiedLine(line string, x, baselineY, availWidth float64, opts text.ShapeOptions) {
	words := splitWords(line)
	if len(words) <= 1 {
		l.addLine(line, x, baselineY, "left", availWidth, opts)
		return
	}
	widths := make([]float64, len(words))
	total := 0.0
	for i, w := range words {
		widths[i] = l.measure(w, opts)
		total += widths[i]
	}
	gap := (availWidth - total) / float64(len(words)-1)

	tx := x
	startX := tx
	for i, w := range words {
		glyphs, _ := l.face.ShapeLine(w, opts)
		for _, g := range glyphs {
			l.glyphs = append(l.glyphs, positionedGlyph{glyph: g.Glyph, x: tx + g.X, baselineY: baselineY, size: g.Size})
		}
		tx += widths[i]
		if i < len(words)-1 {
			tx += gap
		}
	}
	l.addSpan(startX, tx, baselineY)
}

// addSpan records a line's decoration span and folds its line box (x-extent ×
// [baseline-ascent, baseline+descent]) into the layout extents.
func (l *textLayout) addSpan(x0, x1, baselineY float64) {
	l.spans = append(l.spans, textDecorSpan{x0: x0, x1: x1, baselineY: baselineY})
	l.extend(x0, x1, baselineY-l.metrics.Ascent, baselineY+l.metrics.Descent)
}

func (l *textLayout) extend(x0, x1, y0, y1 float64) {
	if !l.hasExtent {
		l.minX, l.maxX, l.minY, l.maxY = x0, x1, y0, y1
		l.hasExtent = true
		return
	}
	l.minX = math.Min(l.minX, x0)
	l.maxX = math.Max(l.maxX, x1)
	l.minY = math.Min(l.minY, y0)
	l.maxY = math.Max(l.maxY, y1)
}

// measure returns the advance width of s under the layout's face and options.
func (l *textLayout) measure(s string, opts text.ShapeOptions) float64 {
	_, w := l.face.ShapeLine(s, opts)
	return w
}

// wrapWords breaks words into lines, with a distinct width for the first line
// (first-line indent support). Words carrying explicit newlines flush the
// current line, mirroring splitWords' token convention.
func (l *textLayout) wrapWords(words []string, firstLineWidth, otherLineWidth float64, opts text.ShapeOptions) []string {
	var lines []string
	current := ""
	lineIdx := 0
	for _, word := range words {
		maxW := otherLineWidth
		if lineIdx == 0 {
			maxW = firstLineWidth
		}
		parts := strings.Split(word, "\n")
		for pi, part := range parts {
			if pi > 0 {
				// Newline encountered — flush current line.
				lines = append(lines, current)
				current = ""
				lineIdx++
				maxW = otherLineWidth
			}
			if part == "" {
				continue
			}
			candidate := part
			if current != "" {
				candidate = current + " " + part
			}
			if current != "" && l.measure(candidate, opts) > maxW {
				lines = append(lines, current)
				current = part
				lineIdx++
			} else {
				current = candidate
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// underlineThickness returns the decoration bar thickness, clamped to at
// least one pixel so decorations stay visible at small sizes.
func (l *textLayout) underlineThickness() float64 {
	return math.Max(1, l.metrics.UnderlineThickness)
}

// computePointTextBounds returns the tight doc-space bounds of a laid-out
// point-text layer: the union of per-line boxes around the anchor, extended
// by the underline when enabled, padded by textBoundsPad on every side for
// anti-aliasing (plus extra horizontal room for oblique overhang), and
// rounded outward to integers. Empty text yields a minimal but non-degenerate
// box at the anchor (about one line high) so the layer stays hittable.
func computePointTextBounds(tl *TextLayer, l *textLayout) LayerBounds {
	if !l.hasExtent {
		side := int(textBoundsPad)
		return LayerBounds{
			X: int(math.Floor(tl.AnchorX - textBoundsPad)),
			Y: int(math.Floor(tl.AnchorY - textBoundsPad)),
			W: 1 + 2*side,
			H: int(math.Ceil(l.lineHeight)) + 2*side,
		}
	}
	minX, minY, maxX, maxY := l.minX, l.minY, l.maxX, l.maxY
	if tl.Underline {
		for _, s := range l.spans {
			maxY = math.Max(maxY, s.baselineY+l.metrics.UnderlinePosition+l.underlineThickness())
		}
	}
	padX := textBoundsPad
	if _, italic := textLayerStyleFlags(tl); italic {
		// Oblique glyph ink overhangs the advance width; widen the horizontal
		// pad so slanted ascenders are not clipped at the tight box edge.
		padX += math.Ceil(l.fontSize * 0.25)
	}
	x0 := math.Floor(tl.AnchorX + minX - padX)
	y0 := math.Floor(tl.AnchorY + minY - textBoundsPad)
	x1 := math.Ceil(tl.AnchorX + maxX + padX)
	y1 := math.Ceil(tl.AnchorY + maxY + textBoundsPad)
	return LayerBounds{X: int(x0), Y: int(y0), W: int(x1 - x0), H: int(y1 - y0)}
}

// textLayerCursor returns the doc-space text-insertion cursor position for a
// layer being edited with the given working text: x sits after the last line's
// advance from the anchor, y on that line's baseline.
func textLayerCursor(tl *TextLayer, working string) (x, y float64) {
	face, _, metrics, lineHeight, opts, _ := textShapeParams(tl)
	if tl.AllCaps {
		working = strings.ToUpper(working)
	}
	lineIdx := strings.Count(working, "\n")
	lastLine := working[strings.LastIndexByte(working, '\n')+1:]
	_, width := face.ShapeLine(lastLine, opts)
	return tl.AnchorX + width, tl.AnchorY + metrics.Ascent + float64(lineIdx)*lineHeight
}

// splitParagraphs splits text on double newlines as paragraph boundaries.
// Single newlines are preserved as line breaks within a paragraph.
func splitParagraphs(text string) []string {
	return strings.Split(text, "\n\n")
}

// splitWords splits text on space/tab boundaries, preserving newlines as
// separate tokens attached to the preceding word.
func splitWords(text string) []string {
	var words []string
	word := ""
	for _, ch := range text {
		switch ch {
		case ' ', '\t', '\r':
			if word != "" {
				words = append(words, word)
				word = ""
			}
		case '\n':
			// Preserve newlines by attaching them to the previous word
			// or emitting them as standalone tokens.
			if word != "" {
				word += "\n"
			} else if len(words) > 0 {
				words[len(words)-1] += "\n"
			} else {
				words = append(words, "\n")
			}
		default:
			word += string(ch)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}
