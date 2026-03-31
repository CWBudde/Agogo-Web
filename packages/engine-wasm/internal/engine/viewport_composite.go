package engine

import "math"

func (doc *Document) renderCompositeSurface() []byte {
	if doc == nil || doc.Width <= 0 || doc.Height <= 0 {
		return nil
	}
	buffer, err := doc.renderLayersToSurface(doc.ensureLayerRoot().Children())
	if err != nil {
		return nil
	}
	return buffer
}

func compositeDocumentToViewport(canvas []byte, canvasW, canvasH int, doc *Document, vp *ViewportState, documentSurface []byte) {
	if len(canvas) == 0 || canvasW <= 0 || canvasH <= 0 || doc == nil || len(documentSurface) == 0 {
		return
	}

	zoom := clampZoom(vp.Zoom)
	rotation := vp.Rotation * (math.Pi / 180)
	cosTheta := math.Cos(rotation)
	sinTheta := math.Sin(rotation)
	halfCanvasW := float64(canvasW) * 0.5
	halfCanvasH := float64(canvasH) * 0.5

	// Use bilinear interpolation below 4× zoom or when the viewport is rotated.
	// At 4× and above, nearest-neighbour gives pixel-perfect blocks that are
	// the expected look when inspecting individual document pixels up close.
	useBilinear := zoom < 4.0 || vp.Rotation != 0

	// Compute the canvas-space bounding box of the document rectangle and clamp
	// to canvas bounds. At zoom < 1 this skips large areas of empty canvas; at
	// zoom ≥ 1 the clip equals the full canvas (no extra cost).
	clipX0, clipY0, clipX1, clipY1 := docBoundsOnCanvas(doc, vp, canvasW, canvasH, zoom, cosTheta, sinTheta, halfCanvasW, halfCanvasH)
	stepDocX := cosTheta / zoom
	stepDocY := -sinTheta / zoom
	startDeltaX := (float64(clipX0) + 0.5) - halfCanvasW

	if useBilinear {
		for canvasY := clipY0; canvasY < clipY1; canvasY++ {
			deltaY := (float64(canvasY) + 0.5) - halfCanvasH
			docX := (startDeltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX
			docY := (-startDeltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY
			destIndex := (canvasY*canvasW + clipX0) * 4

			for canvasX := clipX0; canvasX < clipX1; canvasX++ {
				pixel := sampleBilinear(documentSurface, doc.Width, doc.Height, docX, docY)
				if pixel[3] != 0 {
					if pixel[3] == 255 {
						canvas[destIndex] = pixel[0]
						canvas[destIndex+1] = pixel[1]
						canvas[destIndex+2] = pixel[2]
						canvas[destIndex+3] = 255
					} else {
						compositePixelWithBlend(canvas[destIndex:destIndex+4], pixel[:], BlendModeNormal, 1, pixelNoiseSeed(canvasX, canvasY))
					}
				}

				docX += stepDocX
				docY += stepDocY
				destIndex += 4
			}
		}
		return
	}

	for canvasY := clipY0; canvasY < clipY1; canvasY++ {
		deltaY := (float64(canvasY) + 0.5) - halfCanvasH
		docX := (startDeltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX
		docY := (-startDeltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY
		destIndex := (canvasY*canvasW + clipX0) * 4

		for canvasX := clipX0; canvasX < clipX1; canvasX++ {
			sourceX := int(math.Floor(docX))
			sourceY := int(math.Floor(docY))
			if sourceX >= 0 && sourceX < doc.Width && sourceY >= 0 && sourceY < doc.Height {
				sourceIndex := (sourceY*doc.Width + sourceX) * 4
				srcAlpha := documentSurface[sourceIndex+3]
				if srcAlpha != 0 {
					if srcAlpha == 255 {
						canvas[destIndex] = documentSurface[sourceIndex]
						canvas[destIndex+1] = documentSurface[sourceIndex+1]
						canvas[destIndex+2] = documentSurface[sourceIndex+2]
						canvas[destIndex+3] = 255
					} else {
						compositePixelWithBlend(canvas[destIndex:destIndex+4], documentSurface[sourceIndex:sourceIndex+4], BlendModeNormal, 1, pixelNoiseSeed(canvasX, canvasY))
					}
				}
			}

			docX += stepDocX
			docY += stepDocY
			destIndex += 4
		}
	}
}

// docBoundsOnCanvas returns the canvas pixel rectangle that bounds the document,
// clamped to [0,canvasW) × [0,canvasH). The forward transform maps document
// coordinates to canvas coordinates, so we project the four document corners.
func docBoundsOnCanvas(doc *Document, vp *ViewportState, canvasW, canvasH int, zoom, cosTheta, sinTheta, halfCanvasW, halfCanvasH float64) (x0, y0, x1, y1 int) {
	docW := float64(doc.Width)
	docH := float64(doc.Height)
	cx := vp.CenterX
	cy := vp.CenterY

	minSX := math.MaxFloat64
	minSY := math.MaxFloat64
	maxSX := -math.MaxFloat64
	maxSY := -math.MaxFloat64

	corners := [4][2]float64{{0, 0}, {docW, 0}, {docW, docH}, {0, docH}}
	for _, c := range corners {
		dx := c[0] - cx
		dy := c[1] - cy
		sx := dx*cosTheta*zoom - dy*sinTheta*zoom + halfCanvasW
		sy := dx*sinTheta*zoom + dy*cosTheta*zoom + halfCanvasH
		if sx < minSX {
			minSX = sx
		}
		if sx > maxSX {
			maxSX = sx
		}
		if sy < minSY {
			minSY = sy
		}
		if sy > maxSY {
			maxSY = sy
		}
	}

	x0 = clampInt(int(math.Floor(minSX)), 0, canvasW)
	y0 = clampInt(int(math.Floor(minSY)), 0, canvasH)
	x1 = clampInt(int(math.Ceil(maxSX))+1, 0, canvasW)
	y1 = clampInt(int(math.Ceil(maxSY))+1, 0, canvasH)
	return
}
