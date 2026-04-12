package engine

import agglib "github.com/cwbudde/agg_go"

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

func appendOutlinedText(subpaths *[]Subpath, text string, x, y, fontSize, tracking float64) {
	if text == "" {
		return
	}
	outlines := agglib.BuildGSVTextOutlinePath(text, x, y, fontSize, 0, fontSize*0.08, tracking, 0, true)
	for _, outline := range outlines {
		points := make([]PathPoint, 0, len(outline.Points))
		for _, point := range outline.Points {
			points = append(points, PathPoint{
				X:          point.X,
				Y:          point.Y,
				InX:        point.X,
				InY:        point.Y,
				OutX:       point.X,
				OutY:       point.Y,
				HandleType: HandleCorner,
			})
		}
		if len(points) == 0 {
			continue
		}
		*subpaths = append(*subpaths, Subpath{
			Closed: outline.Closed,
			Points: points,
		})
	}
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
