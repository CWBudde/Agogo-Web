package engine

import (
	"strings"

	agglib "github.com/cwbudde/agg_go"
)

func buildTextOutlinePath(layer *TextLayer) *Path {
	if layer == nil || layer.Text == "" {
		return nil
	}

	fontSize := layer.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}

	measurer := agglib.NewAgg2D()
	measurer.FontGSV(fontSize)

	leading := layer.Leading
	if leading <= 0 {
		leading = textDefaultLeading
	}
	lineHeight := fontSize * leading

	x := float64(layer.Bounds.X)
	baseY := float64(layer.Bounds.Y) + fontSize
	text := applyCapsTransform(layer.Text, layer.AllCaps, layer.SmallCaps)

	subpaths := make([]Subpath, 0)
	if layer.TextType == "area" && layer.Bounds.W > 0 {
		appendAreaTextOutline(&subpaths, measurer, layer, text, x, baseY, float64(layer.Bounds.W), lineHeight, fontSize)
	} else {
		appendPointTextOutline(&subpaths, measurer, layer, text, x, baseY, fontSize)
	}

	if len(subpaths) == 0 {
		return nil
	}
	return &Path{Subpaths: subpaths}
}

func appendPointTextOutline(subpaths *[]Subpath, measurer *agglib.Agg2D, layer *TextLayer, text string, x, baseY, fontSize float64) {
	if text == "" {
		return
	}
	if layer.Tracking != 0 {
		totalWidth := textWidthWithTracking(measurer, text, layer.Tracking)
		appendOutlinedText(subpaths, text, alignedTrackedX(x, totalWidth, layer.Alignment, 0), baseY, fontSize, layer.Tracking)
		return
	}
	appendOutlinedText(subpaths, text, alignedX(measurer, text, x, layer.Alignment), baseY, fontSize, 0)
}

func appendAreaTextOutline(subpaths *[]Subpath, measurer *agglib.Agg2D, layer *TextLayer, text string, x, baseY, maxWidth, lineHeight, fontSize float64) {
	paragraphs := splitParagraphs(text)
	y := baseY

	for pi, para := range paragraphs {
		if pi > 0 {
			y += layer.SpaceBefore + layer.SpaceAfter
		}

		words := splitWords(para)
		if len(words) == 0 {
			y += lineHeight
			continue
		}

		availWidth := maxWidth - layer.IndentLeft - layer.IndentRight
		firstLineAvailWidth := maxWidth - layer.IndentLeft - layer.IndentFirst - layer.IndentRight
		lines := wrapWordsVariable(measurer, words, firstLineAvailWidth, availWidth)

		for li, line := range lines {
			lineX := x + layer.IndentLeft
			curAvailWidth := availWidth
			if li == 0 {
				lineX += layer.IndentFirst
				curAvailWidth = firstLineAvailWidth
			}

			isLastLine := li == len(lines)-1
			if layer.Alignment == "justify" && !isLastLine && len(splitWords(line)) > 1 {
				appendJustifiedTextOutline(subpaths, measurer, layer, line, lineX, y, curAvailWidth, fontSize)
			} else if layer.Tracking != 0 {
				totalWidth := textWidthWithTracking(measurer, line, layer.Tracking)
				appendOutlinedText(subpaths, line, alignedTrackedX(lineX, totalWidth, layer.Alignment, curAvailWidth), y, fontSize, layer.Tracking)
			} else {
				appendOutlinedText(subpaths, line, alignedXWidth(measurer, line, lineX, layer.Alignment, curAvailWidth), y, fontSize, 0)
			}
			y += lineHeight
		}
	}
}

func appendJustifiedTextOutline(subpaths *[]Subpath, measurer *agglib.Agg2D, layer *TextLayer, line string, x, y, availWidth, fontSize float64) {
	words := splitWords(line)
	if len(words) <= 1 {
		appendOutlinedText(subpaths, line, x, y, fontSize, layer.Tracking)
		return
	}

	totalTextWidth := 0.0
	for _, word := range words {
		if layer.Tracking != 0 {
			totalTextWidth += textWidthWithTracking(measurer, word, layer.Tracking)
		} else {
			totalTextWidth += measurer.TextWidth(word)
		}
	}
	wordGap := (availWidth - totalTextWidth) / float64(len(words)-1)

	tx := x
	for index, word := range words {
		appendOutlinedText(subpaths, word, tx, y, fontSize, layer.Tracking)
		if layer.Tracking != 0 {
			tx += textWidthWithTracking(measurer, word, layer.Tracking)
		} else {
			tx += measurer.TextWidth(word)
		}
		if index < len(words)-1 {
			tx += wordGap
		}
	}
}

// appendOutlinedText traces GSV glyph centerlines into open subpaths.
// GSV is a stroke font: each subpath is a pen stroke, not a closed fill
// contour, so the resulting vector layer must render stroke-only with
// gsvOutlineStrokeWidth(fontSize) to match rasterized GSV text.
func appendOutlinedText(subpaths *[]Subpath, text string, x, y, fontSize, tracking float64) {
	if text == "" {
		return
	}

	gsv := agglib.NewGSVText()
	// flip=true matches Agg2D's configuration for top-down buffers
	// (document space, Y grows down): glyphs ascend above the baseline.
	gsv.SetFlip(true)
	gsv.SetSize(fontSize, 0)
	if tracking != 0 {
		gsv.SetSpace(tracking)
	}
	gsv.SetStartPoint(x, y)
	gsv.SetText(text)
	gsv.Rewind(0)

	var current []PathPoint
	flush := func() {
		if len(current) >= 2 {
			*subpaths = append(*subpaths, Subpath{Closed: false, Points: current})
		}
		current = nil
	}
	for {
		vx, vy, cmd := gsv.Vertex()
		switch cmd {
		case agglib.GSVPathCmdMoveTo:
			flush()
			current = append(current, PathPoint{X: vx, Y: vy, HandleType: HandleCorner})
		case agglib.GSVPathCmdLineTo:
			current = append(current, PathPoint{X: vx, Y: vy, HandleType: HandleCorner})
		default:
			flush()
			return
		}
	}
}

// gsvOutlineStrokeWidth returns the stroke width Agg2D uses when rasterizing
// GSV text (~8% of glyph height), so converted outlines match rendered text.
func gsvOutlineStrokeWidth(fontSize float64) float64 {
	return fontSize * 0.08
}

func alignedTrackedX(x, totalWidth float64, alignment string, availWidth float64) float64 {
	switch alignment {
	case "center":
		if availWidth > 0 {
			return x + (availWidth-totalWidth)/2
		}
		return x - totalWidth/2
	case "right":
		if availWidth > 0 {
			return x + availWidth - totalWidth
		}
		return x - totalWidth
	default:
		return x
	}
}

// ---------------------------------------------------------------------------
// Legacy GSV measuring helpers.
//
// These support the GSV-based outline conversion above and are the LAST GSV
// users in the engine: the raster text pipeline now shapes and measures via
// internal/text (see text_layout.go / text_render.go). They are scheduled for
// deletion when Create Outlines is rewritten on the shared layout pass (S.6
// batch F3).
// ---------------------------------------------------------------------------

// applyCapsTransform transforms text based on AllCaps/SmallCaps settings.
// GSV has no small-caps rendering, so SmallCaps degrades to plain uppercase
// here (the raster pipeline handles SmallCaps in the shaper instead).
func applyCapsTransform(text string, allCaps, smallCaps bool) string {
	if allCaps || smallCaps {
		return strings.ToUpper(text)
	}
	return text
}

// textWidthWithTracking computes the GSV width of text with tracking applied.
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
// for the first line vs subsequent lines (to support first-line indent),
// measuring through the GSV font.
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
