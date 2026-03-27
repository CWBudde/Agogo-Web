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
	rotation := vp.Rotation * math.Pi / 180
	cosTheta := math.Cos(rotation)
	sinTheta := math.Sin(rotation)
	halfCanvasW := float64(canvasW) / 2
	halfCanvasH := float64(canvasH) / 2

	for canvasY := 0; canvasY < canvasH; canvasY++ {
		for canvasX := 0; canvasX < canvasW; canvasX++ {
			deltaX := (float64(canvasX) + 0.5) - halfCanvasW
			deltaY := (float64(canvasY) + 0.5) - halfCanvasH
			docX := (deltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX
			docY := (-deltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY
			sourceX := int(math.Floor(docX))
			sourceY := int(math.Floor(docY))
			if sourceX < 0 || sourceX >= doc.Width || sourceY < 0 || sourceY >= doc.Height {
				continue
			}
			sourceIndex := (sourceY*doc.Width + sourceX) * 4
			if documentSurface[sourceIndex+3] == 0 {
				continue
			}
			destIndex := (canvasY*canvasW + canvasX) * 4
			compositePixelWithBlend(canvas[destIndex:destIndex+4], documentSurface[sourceIndex:sourceIndex+4], BlendModeNormal, 1, pixelNoiseSeed(canvasX, canvasY))
		}
	}
}
