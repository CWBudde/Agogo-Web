package engine

import (
	"math"
	"strings"

	agglib "github.com/cwbudde/agg_go"
)

const textDefaultLeading = 1.2

// rasterizeTextLayer renders a TextLayer's text string into an RGBA buffer
// sized docW×docH. The result is suitable for storing in TextLayer.CachedRaster.
// Returns a transparent buffer (no error) when Text is empty.
func rasterizeTextLayer(layer *TextLayer, docW, docH int) ([]byte, error) {
	stride := docW * 4
	buf := make([]byte, stride*docH)

	if layer.Text == "" || docW <= 0 || docH <= 0 {
		return buf, nil
	}

	fontSize := layer.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}

	r := agglib.NewAgg2D()
	r.Attach(buf, docW, docH, stride)
	r.ResetTransformations()

	// Use WASM-safe GSV font (no CGO, no font files required).
	r.FontGSV(fontSize)
	r.FlipText(true)

	c := layer.Color
	r.FillColor(agglib.NewColor(c[0], c[1], c[2], c[3]))
	r.NoLine()

	leading := layer.Leading
	if leading <= 0 {
		leading = textDefaultLeading
	}
	lineHeight := fontSize * leading

	x := float64(layer.Bounds.X)
	baseY := float64(layer.Bounds.Y) + fontSize

	// Apply caps transformation to text.
	text := applyCapsTransform(layer.Text, layer.AllCaps, layer.SmallCaps)

	if layer.TextType == "area" && layer.Bounds.W > 0 {
		renderAreaText(r, layer, text, x, baseY, float64(layer.Bounds.W), lineHeight, fontSize)
	} else {
		renderPointText(r, layer, text, x, baseY, fontSize)
	}

	return buf, nil
}

// applyCapsTransform transforms text based on AllCaps/SmallCaps settings.
func applyCapsTransform(text string, allCaps, smallCaps bool) string {
	if allCaps || smallCaps {
		return strings.ToUpper(text)
	}
	return text
}

// renderPointText renders a single-line text string at (x, y).
func renderPointText(r *agglib.Agg2D, layer *TextLayer, text string, x, baseY, fontSize float64) {
	if layer.Tracking != 0 {
		lineWidth := renderTextWithTracking(r, text, x, baseY, layer.Tracking, layer.Alignment, 0)
		drawTextDecorations(r, layer, x, baseY, lineWidth, fontSize)
	} else {
		tx := alignedX(r, text, x, layer.Alignment)
		r.TextDefault(tx, baseY, text)
		drawTextDecorations(r, layer, tx, baseY, r.TextWidth(text), fontSize)
	}
}

// renderAreaText wraps text within maxWidth and renders each line.
func renderAreaText(r *agglib.Agg2D, layer *TextLayer, text string, x, baseY, maxWidth, lineHeight, fontSize float64) {
	paragraphs := splitParagraphs(text)
	y := baseY

	for pi, para := range paragraphs {
		if pi > 0 {
			y += layer.SpaceBefore + layer.SpaceAfter
		}

		words := splitWords(para)
		if len(words) == 0 {
			// Empty paragraph — just advance by one line height.
			y += lineHeight
			continue
		}

		// Compute available widths considering indents.
		availWidth := maxWidth - layer.IndentLeft - layer.IndentRight
		firstLineAvailWidth := maxWidth - layer.IndentLeft - layer.IndentFirst - layer.IndentRight

		lines := wrapWordsVariable(r, words, firstLineAvailWidth, availWidth)

		for li, line := range lines {
			lineX := x + layer.IndentLeft
			curAvailWidth := availWidth
			if li == 0 {
				lineX += layer.IndentFirst
				curAvailWidth = firstLineAvailWidth
			}

			isLastLine := li == len(lines)-1

			if layer.Alignment == "justify" && !isLastLine && len(splitWords(line)) > 1 {
				renderJustifiedLine(r, layer, line, lineX, y, curAvailWidth, fontSize)
			} else if layer.Tracking != 0 {
				lineWidth := renderTextWithTracking(r, line, lineX, y, layer.Tracking, layer.Alignment, curAvailWidth)
				drawTextDecorations(r, layer, lineX, y, lineWidth, fontSize)
			} else {
				tx := alignedXWidth(r, line, lineX, layer.Alignment, curAvailWidth)
				r.TextDefault(tx, y, line)
				drawTextDecorations(r, layer, tx, y, r.TextWidth(line), fontSize)
			}
			y += lineHeight
		}
	}
}

// renderJustifiedLine renders a single justified line by distributing extra space between words.
func renderJustifiedLine(r *agglib.Agg2D, layer *TextLayer, line string, x, y, availWidth, fontSize float64) {
	words := splitWords(line)
	if len(words) <= 1 {
		// Single word — just left-align.
		renderLineWithDecorations(r, layer, line, x, y, fontSize)
		return
	}

	totalTextWidth := 0.0
	for _, w := range words {
		totalTextWidth += r.TextWidth(w)
	}
	extraSpace := availWidth - totalTextWidth
	gapCount := float64(len(words) - 1)
	wordGap := extraSpace / gapCount

	tx := x
	startX := tx
	for wi, w := range words {
		if layer.Tracking != 0 {
			renderCharsWithTracking(r, w, tx, y, layer.Tracking)
			tx += textWidthWithTracking(r, w, layer.Tracking)
		} else {
			r.TextDefault(tx, y, w)
			tx += r.TextWidth(w)
		}
		if wi < len(words)-1 {
			tx += wordGap
		}
	}
	drawTextDecorations(r, layer, startX, y, tx-startX, fontSize)
}

// renderLineWithDecorations renders a line and draws underline/strikethrough.
func renderLineWithDecorations(r *agglib.Agg2D, layer *TextLayer, text string, x, y, fontSize float64) {
	r.TextDefault(x, y, text)
	drawTextDecorations(r, layer, x, y, r.TextWidth(text), fontSize)
}

// renderTextWithTracking renders text character by character with extra tracking,
// taking alignment into account. Returns the total rendered width.
func renderTextWithTracking(r *agglib.Agg2D, text string, x, y, tracking float64, alignment string, availWidth float64) float64 {
	totalWidth := textWidthWithTracking(r, text, tracking)

	tx := x
	switch alignment {
	case "center":
		if availWidth > 0 {
			tx = x + (availWidth-totalWidth)/2
		} else {
			tx = x - totalWidth/2
		}
	case "right":
		if availWidth > 0 {
			tx = x + availWidth - totalWidth
		} else {
			tx = x - totalWidth
		}
	}

	renderCharsWithTracking(r, text, tx, y, tracking)
	return totalWidth
}

// renderCharsWithTracking renders each character advancing by charWidth + tracking.
func renderCharsWithTracking(r *agglib.Agg2D, text string, x, y, tracking float64) {
	tx := x
	for _, ch := range text {
		s := string(ch)
		r.TextDefault(tx, y, s)
		tx += r.TextWidth(s) + tracking
	}
}

// textWidthWithTracking computes the total width of text with tracking applied.
func textWidthWithTracking(r *agglib.Agg2D, text string, tracking float64) float64 {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	total := 0.0
	for _, ch := range runes {
		total += r.TextWidth(string(ch))
	}
	// Tracking is added between characters, not after the last one.
	total += tracking * float64(len(runes)-1)
	return total
}

// drawTextDecorations draws underline and/or strikethrough lines.
func drawTextDecorations(r *agglib.Agg2D, layer *TextLayer, x, y, width, fontSize float64) {
	if !layer.Underline && !layer.Strikethrough {
		return
	}

	c := layer.Color
	lw := math.Max(1.0, fontSize/20)

	// Save current state and configure line drawing.
	r.LineColor(agglib.NewColor(c[0], c[1], c[2], c[3]))
	r.LineWidth(lw)

	if layer.Underline {
		lineY := y + fontSize*0.15
		r.Line(x, lineY, x+width, lineY)
	}
	if layer.Strikethrough {
		lineY := y - fontSize*0.3
		r.Line(x, lineY, x+width, lineY)
	}

	// Restore to no-line state for subsequent text rendering.
	r.NoLine()
}

// splitParagraphs splits text on double newlines as paragraph boundaries.
// Single newlines are preserved as line breaks within a paragraph.
func splitParagraphs(text string) []string {
	return strings.Split(text, "\n\n")
}

// alignedX returns the starting X position for a text string given alignment.
func alignedX(r *agglib.Agg2D, text string, x float64, alignment string) float64 {
	switch alignment {
	case "center":
		return x - r.TextWidth(text)/2
	case "right":
		return x - r.TextWidth(text)
	default: // "left" or "justify"
		return x
	}
}

// alignedXWidth returns the starting X for text within an available width region.
func alignedXWidth(r *agglib.Agg2D, text string, x float64, alignment string, availWidth float64) float64 {
	switch alignment {
	case "center":
		return x + (availWidth-r.TextWidth(text))/2
	case "right":
		return x + availWidth - r.TextWidth(text)
	default: // "left" or "justify"
		return x
	}
}

// wrapWordsVariable breaks words into lines with potentially different widths
// for the first line vs subsequent lines (to support first-line indent).
func wrapWordsVariable(r *agglib.Agg2D, words []string, firstLineWidth, otherLineWidth float64) []string {
	var lines []string
	current := ""
	lineIdx := 0
	for _, word := range words {
		maxW := otherLineWidth
		if lineIdx == 0 {
			maxW = firstLineWidth
		}

		// Handle explicit line breaks within a paragraph.
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
			if current != "" && r.TextWidth(candidate) > maxW {
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

// splitWords splits text on space/tab boundaries, preserving newlines as separate tokens.
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

// measureTextWidth returns the rendered width of text at the given font size
// using the GSV vector font. Used for cursor position estimation.
func measureTextWidth(text string, fontSize float64) float64 {
	if text == "" || fontSize <= 0 {
		return 0
	}
	r := agglib.NewAgg2D()
	r.FontGSV(fontSize)
	return r.TextWidth(text)
}
