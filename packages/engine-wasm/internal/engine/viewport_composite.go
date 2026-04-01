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

	clipX0, clipY0, clipX1, clipY1 := docBoundsOnCanvas(doc, vp, canvasW, canvasH, zoom, cosTheta, sinTheta, halfCanvasW, halfCanvasH)

	if useBilinear && math.Abs(sinTheta) < 1e-10 {
		compositeViewportBilinearUnrotated(canvas, canvasW, doc, documentSurface, vp, zoom, clipX0, clipY0, clipX1, clipY1, halfCanvasW, halfCanvasH)
		return
	}

	stepDocX := cosTheta / zoom
	stepDocY := -sinTheta / zoom
	startDeltaX := (float64(clipX0) + 0.5) - halfCanvasW

	if useBilinear {
		compositeViewportBilinearRotated(canvas, canvasW, doc, documentSurface, clipX0, clipY0, clipX1, clipY1, startDeltaX, stepDocX, stepDocY, cosTheta, sinTheta, zoom, halfCanvasH, vp)
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

// compositeViewportBilinearUnrotated is the fast path for unrotated viewports.
// Because rotation is zero, docY is constant across each scanline, so Y weights
// and row offsets are hoisted out of the inner loop. Bilinear sampling is fully
// inlined using fixed-point (8-bit fractional) weights to avoid float64
// multiplications per channel.
func compositeViewportBilinearUnrotated(canvas []byte, canvasW int, doc *Document, surf []byte, vp *ViewportState, zoom float64, clipX0, clipY0, clipX1, clipY1 int, halfCanvasW, halfCanvasH float64) {
	docW := doc.Width
	docH := doc.Height
	invZoom := 1.0 / zoom
	stride := docW * 4
	maxX := docW - 1
	maxY := docH - 1

	for canvasY := clipY0; canvasY < clipY1; canvasY++ {
		deltaY := (float64(canvasY) + 0.5) - halfCanvasH
		docY := deltaY*invZoom + vp.CenterY - 0.5

		// Precompute Y weights (constant across the entire scanline).
		iy := int(docY)
		if docY < 0 {
			iy = int(docY) - 1
		}
		fy := docY - float64(iy)
		// Fixed-point Y weights (8-bit fraction, 0–256 range).
		wy1 := int(fy*256 + 0.5)
		wy0 := 256 - wy1

		y0 := iy
		y1 := iy + 1
		if y0 < 0 {
			y0 = 0
		} else if y0 > maxY {
			y0 = maxY
		}
		if y1 < 0 {
			y1 = 0
		} else if y1 > maxY {
			y1 = maxY
		}
		row0 := y0 * stride
		row1 := y1 * stride

		deltaX0 := (float64(clipX0) + 0.5) - halfCanvasW
		docX := deltaX0*invZoom + vp.CenterX - 0.5
		stepX := invZoom

		destIndex := (canvasY*canvasW + clipX0) * 4

		for canvasX := clipX0; canvasX < clipX1; canvasX++ {
			ix := int(docX)
			if docX < 0 {
				ix = int(docX) - 1
			}
			fx := docX - float64(ix)

			x0 := ix
			x1 := ix + 1

			// Fast interior check: skip clamping when fully inside.
			if x0 >= 0 && x1 <= maxX {
				off00 := row0 + x0*4
				off10 := row0 + x1*4
				off01 := row1 + x0*4
				off11 := row1 + x1*4

				wx1 := int(fx*256 + 0.5)
				wx0 := 256 - wx1

				w00 := wx0 * wy0
				w10 := wx1 * wy0
				w01 := wx0 * wy1
				w11 := wx1 * wy1

				a := (int(surf[off00+3])*w00 + int(surf[off10+3])*w10 + int(surf[off01+3])*w01 + int(surf[off11+3])*w11 + 32768) >> 16
				if a != 0 {
					r := (int(surf[off00])*w00 + int(surf[off10])*w10 + int(surf[off01])*w01 + int(surf[off11])*w11 + 32768) >> 16
					g := (int(surf[off00+1])*w00 + int(surf[off10+1])*w10 + int(surf[off01+1])*w01 + int(surf[off11+1])*w11 + 32768) >> 16
					b := (int(surf[off00+2])*w00 + int(surf[off10+2])*w10 + int(surf[off01+2])*w01 + int(surf[off11+2])*w11 + 32768) >> 16
					if a >= 255 {
						canvas[destIndex] = byte(r)
						canvas[destIndex+1] = byte(g)
						canvas[destIndex+2] = byte(b)
						canvas[destIndex+3] = 255
					} else {
						pix := [4]byte{byte(r), byte(g), byte(b), byte(a)}
						compositePixelWithBlend(canvas[destIndex:destIndex+4], pix[:], BlendModeNormal, 1, pixelNoiseSeed(canvasX, canvasY))
					}
				}
			} else {
				// Edge: clamp coordinates.
				if x0 < 0 {
					x0 = 0
				} else if x0 > maxX {
					x0 = maxX
				}
				if x1 < 0 {
					x1 = 0
				} else if x1 > maxX {
					x1 = maxX
				}
				off00 := row0 + x0*4
				off10 := row0 + x1*4
				off01 := row1 + x0*4
				off11 := row1 + x1*4

				wx1 := int(fx*256 + 0.5)
				wx0 := 256 - wx1

				w00 := wx0 * wy0
				w10 := wx1 * wy0
				w01 := wx0 * wy1
				w11 := wx1 * wy1

				a := (int(surf[off00+3])*w00 + int(surf[off10+3])*w10 + int(surf[off01+3])*w01 + int(surf[off11+3])*w11 + 32768) >> 16
				if a != 0 {
					r := (int(surf[off00])*w00 + int(surf[off10])*w10 + int(surf[off01])*w01 + int(surf[off11])*w11 + 32768) >> 16
					g := (int(surf[off00+1])*w00 + int(surf[off10+1])*w10 + int(surf[off01+1])*w01 + int(surf[off11+1])*w11 + 32768) >> 16
					b := (int(surf[off00+2])*w00 + int(surf[off10+2])*w10 + int(surf[off01+2])*w01 + int(surf[off11+2])*w11 + 32768) >> 16
					if a >= 255 {
						canvas[destIndex] = byte(r)
						canvas[destIndex+1] = byte(g)
						canvas[destIndex+2] = byte(b)
						canvas[destIndex+3] = 255
					} else {
						pix := [4]byte{byte(r), byte(g), byte(b), byte(a)}
						compositePixelWithBlend(canvas[destIndex:destIndex+4], pix[:], BlendModeNormal, 1, pixelNoiseSeed(canvasX, canvasY))
					}
				}
			}

			docX += stepX
			destIndex += 4
		}
	}
}

// compositeViewportBilinearRotated handles bilinear sampling when the viewport
// is rotated. Both docX and docY vary per pixel. Sampling is inlined with
// fixed-point weights but Y weights cannot be hoisted.
func compositeViewportBilinearRotated(canvas []byte, canvasW int, doc *Document, surf []byte, clipX0, clipY0, clipX1, clipY1 int, startDeltaX, stepDocX, stepDocY, cosTheta, sinTheta, zoom, halfCanvasH float64, vp *ViewportState) {
	docW := doc.Width
	docH := doc.Height
	stride := docW * 4
	maxX := docW - 1
	maxY := docH - 1

	for canvasY := clipY0; canvasY < clipY1; canvasY++ {
		deltaY := (float64(canvasY) + 0.5) - halfCanvasH
		docX := (startDeltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX - 0.5
		docY := (-startDeltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY - 0.5
		destIndex := (canvasY*canvasW + clipX0) * 4

		for canvasX := clipX0; canvasX < clipX1; canvasX++ {
			ix := int(docX)
			if docX < 0 {
				ix = int(docX) - 1
			}
			iy := int(docY)
			if docY < 0 {
				iy = int(docY) - 1
			}

			x0, x1 := ix, ix+1
			y0, y1 := iy, iy+1

			interior := x0 >= 0 && x1 <= maxX && y0 >= 0 && y1 <= maxY
			if !interior {
				if x0 < 0 {
					x0 = 0
				} else if x0 > maxX {
					x0 = maxX
				}
				if x1 < 0 {
					x1 = 0
				} else if x1 > maxX {
					x1 = maxX
				}
				if y0 < 0 {
					y0 = 0
				} else if y0 > maxY {
					y0 = maxY
				}
				if y1 < 0 {
					y1 = 0
				} else if y1 > maxY {
					y1 = maxY
				}
			}

			fx := docX - float64(ix)
			fy := docY - float64(iy)
			wx1 := int(fx*256 + 0.5)
			wx0 := 256 - wx1
			wy1 := int(fy*256 + 0.5)
			wy0 := 256 - wy1

			w00 := wx0 * wy0
			w10 := wx1 * wy0
			w01 := wx0 * wy1
			w11 := wx1 * wy1

			off00 := y0*stride + x0*4
			off10 := y0*stride + x1*4
			off01 := y1*stride + x0*4
			off11 := y1*stride + x1*4

			a := (int(surf[off00+3])*w00 + int(surf[off10+3])*w10 + int(surf[off01+3])*w01 + int(surf[off11+3])*w11 + 32768) >> 16
			if a != 0 {
				r := (int(surf[off00])*w00 + int(surf[off10])*w10 + int(surf[off01])*w01 + int(surf[off11])*w11 + 32768) >> 16
				g := (int(surf[off00+1])*w00 + int(surf[off10+1])*w10 + int(surf[off01+1])*w01 + int(surf[off11+1])*w11 + 32768) >> 16
				b := (int(surf[off00+2])*w00 + int(surf[off10+2])*w10 + int(surf[off01+2])*w01 + int(surf[off11+2])*w11 + 32768) >> 16
				if a >= 255 {
					canvas[destIndex] = byte(r)
					canvas[destIndex+1] = byte(g)
					canvas[destIndex+2] = byte(b)
					canvas[destIndex+3] = 255
				} else {
					pix := [4]byte{byte(r), byte(g), byte(b), byte(a)}
					compositePixelWithBlend(canvas[destIndex:destIndex+4], pix[:], BlendModeNormal, 1, pixelNoiseSeed(canvasX, canvasY))
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
