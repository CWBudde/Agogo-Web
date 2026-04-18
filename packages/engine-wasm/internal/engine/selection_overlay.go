package engine

import "math"

func RenderSelectionOverlay(doc *Document, vp *ViewportState, reuse []byte, selection *Selection, animationFrame int64, viewMode SelectionViewMode) []byte {
	if doc == nil || vp == nil || selection == nil || len(reuse) == 0 {
		return reuse
	}
	bounds, ok := selection.Bounds()
	if !ok {
		return reuse
	}
	if viewMode == "" {
		viewMode = SelectionViewModeMarchingAnts
	}
	canvasW := maxInt(vp.CanvasW, 1)
	canvasH := maxInt(vp.CanvasH, 1)
	zoom := clampZoom(vp.Zoom)
	radians := vp.Rotation * (math.Pi / 180)
	cosTheta := math.Cos(radians)
	sinTheta := math.Sin(radians)
	halfCanvasW := float64(canvasW) * 0.5
	halfCanvasH := float64(canvasH) * 0.5
	clipX0, clipY0, clipX1, clipY1 := docBoundsOnCanvas(doc, vp, canvasW, canvasH, zoom, cosTheta, sinTheta, halfCanvasW, halfCanvasH)
	if viewMode != SelectionViewModeMarchingAnts {
		for canvasY := clipY0; canvasY < clipY1; canvasY++ {
			for canvasX := clipX0; canvasX < clipX1; canvasX++ {
				docX, docY := viewportPixelToDocument(vp, canvasX, canvasY, zoom, cosTheta, sinTheta, halfCanvasW, halfCanvasH)
				alpha := bilinearSelectionSample(selection, docX, docY)
				index := (canvasY*canvasW + canvasX) * 4
				applySelectionViewModePixel(reuse[index:index+4], alpha, viewMode)
			}
		}
		if viewMode != SelectionViewModeLayer {
			return reuse
		}
	}
	phase := int(animationFrame/2) & 7
	for y := bounds.Y; y < bounds.Y+bounds.H; y++ {
		for x := bounds.X; x < bounds.X+bounds.W; x++ {
			if !selectionEdgeAt(selection, x, y) {
				continue
			}
			docX := float64(x) + 0.5 - vp.CenterX
			docY := float64(y) + 0.5 - vp.CenterY
			screenX := docX*cosTheta*zoom - docY*sinTheta*zoom + halfCanvasW
			screenY := docX*sinTheta*zoom + docY*cosTheta*zoom + halfCanvasH
			canvasX := int(math.Floor(screenX))
			canvasY := int(math.Floor(screenY))
			if canvasX < 0 || canvasX >= canvasW || canvasY < 0 || canvasY >= canvasH {
				continue
			}
			pattern := (x + y - phase) & 7
			color := byte(0)
			if pattern >= 4 {
				color = 255
			}
			index := (canvasY*canvasW + canvasX) * 4
			reuse[index] = color
			reuse[index+1] = color
			reuse[index+2] = color
			reuse[index+3] = 255
		}
	}
	return reuse
}

func viewportPixelToDocument(vp *ViewportState, canvasX, canvasY int, zoom, cosTheta, sinTheta, halfCanvasW, halfCanvasH float64) (float64, float64) {
	deltaX := (float64(canvasX) + 0.5) - halfCanvasW
	deltaY := (float64(canvasY) + 0.5) - halfCanvasH
	docX := (deltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX
	docY := (-deltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY
	return docX, docY
}

func applySelectionViewModePixel(pixel []byte, alpha byte, viewMode SelectionViewMode) {
	if len(pixel) < 4 {
		return
	}
	alphaF := float64(alpha) / 255
	switch viewMode {
	case SelectionViewModeOnionSkin:
		factor := 0.2 + 0.8*alphaF
		pixel[0] = byte(math.Round(float64(pixel[0]) * factor))
		pixel[1] = byte(math.Round(float64(pixel[1]) * factor))
		pixel[2] = byte(math.Round(float64(pixel[2]) * factor))
	case SelectionViewModeOverlay:
		tint := 0.7 * (1 - alphaF)
		pixel[0] = byte(math.Round(float64(pixel[0])*(1-tint) + 255*tint))
		pixel[1] = byte(math.Round(float64(pixel[1]) * (1 - tint)))
		pixel[2] = byte(math.Round(float64(pixel[2]) * (1 - tint)))
	case SelectionViewModeBlackWhite:
		pixel[0] = alpha
		pixel[1] = alpha
		pixel[2] = alpha
		pixel[3] = 255
	case SelectionViewModeBlack:
		pixel[0] = byte(math.Round(float64(pixel[0]) * alphaF))
		pixel[1] = byte(math.Round(float64(pixel[1]) * alphaF))
		pixel[2] = byte(math.Round(float64(pixel[2]) * alphaF))
		pixel[3] = 255
	case SelectionViewModeWhite:
		pixel[0] = byte(math.Round(float64(pixel[0])*alphaF + 255*(1-alphaF)))
		pixel[1] = byte(math.Round(float64(pixel[1])*alphaF + 255*(1-alphaF)))
		pixel[2] = byte(math.Round(float64(pixel[2])*alphaF + 255*(1-alphaF)))
		pixel[3] = 255
	case SelectionViewModeLayer:
		pixel[3] = scaleMaskedAlpha(pixel[3], alpha)
		if pixel[3] == 0 {
			pixel[0] = 0
			pixel[1] = 0
			pixel[2] = 0
		}
	}
}

func selectionEdgeAt(selection *Selection, x, y int) bool {
	if selection == nil || x < 0 || x >= selection.Width || y < 0 || y >= selection.Height {
		return false
	}
	if selection.Mask[y*selection.Width+x] == 0 {
		return false
	}
	if x == 0 || x == selection.Width-1 || y == 0 || y == selection.Height-1 {
		return true
	}
	return selection.Mask[y*selection.Width+x-1] == 0 ||
		selection.Mask[y*selection.Width+x+1] == 0 ||
		selection.Mask[(y-1)*selection.Width+x] == 0 ||
		selection.Mask[(y+1)*selection.Width+x] == 0
}

func bilinearSelectionSample(selection *Selection, x, y float64) byte {
	fx := x - 0.5
	fy := y - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	a00 := float64(selectionAlphaAt(selection, x0, y0))
	a10 := float64(selectionAlphaAt(selection, x0+1, y0))
	a01 := float64(selectionAlphaAt(selection, x0, y0+1))
	a11 := float64(selectionAlphaAt(selection, x0+1, y0+1))
	top := a00*(1-tx) + a10*tx
	bottom := a01*(1-tx) + a11*tx
	return uint8(math.Round(clampFloat(top*(1-ty)+bottom*ty, 0, 255)))
}

func selectionAlphaAt(selection *Selection, x, y int) byte {
	if selection == nil || x < 0 || y < 0 || x >= selection.Width || y >= selection.Height {
		return 0
	}
	return selection.Mask[y*selection.Width+x]
}

func sampledCoverage(antiAlias bool, inside func(sampleX, sampleY float64) bool, pixelX, pixelY int) byte {
	if !antiAlias {
		if inside(float64(pixelX)+0.5, float64(pixelY)+0.5) {
			return 255
		}
		return 0
	}
	covered := 0
	sampleOffsets := [4]float64{0.125, 0.375, 0.625, 0.875}
	for _, sampleY := range sampleOffsets {
		for _, sampleX := range sampleOffsets {
			if inside(float64(pixelX)+sampleX, float64(pixelY)+sampleY) {
				covered++
			}
		}
	}
	return uint8(math.Round(float64(covered) / 16 * 255))
}

func pointInPolygon(points []SelectionPoint, x, y float64) bool {
	inside := false
	for left, right := len(points)-1, 0; right < len(points); left, right = right, right+1 {
		ax := points[left].X
		ay := points[left].Y
		bx := points[right].X
		by := points[right].Y
		intersects := ((ay > y) != (by > y)) && (x < (bx-ax)*(y-ay)/(by-ay+1e-9)+ax)
		if intersects {
			inside = !inside
		}
	}
	return inside
}

func normalizeSelectionRect(rect LayerBounds) LayerBounds {
	if rect.W < 0 {
		rect.X += rect.W
		rect.W = -rect.W
	}
	if rect.H < 0 {
		rect.Y += rect.H
		rect.H = -rect.H
	}
	return rect
}

func colorDistance(pixel []byte, target [4]uint8) float64 {
	red := float64(pixel[0]) - float64(target[0])
	green := float64(pixel[1]) - float64(target[1])
	blue := float64(pixel[2]) - float64(target[2])
	return math.Sqrt(red*red + green*green + blue*blue)
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
