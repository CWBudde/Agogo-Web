package engine

import "math"

// renderCompositeSurfaceChecked renders the document's full layer stack into a
// doc-sized RGBA surface and propagates any compositing error (for example a
// CachedRaster whose length does not match its layer bounds). The interactive
// render path must use this variant so failures reach the frontend instead of
// silently producing — and caching — a blank document.
func (doc *Document) renderCompositeSurfaceChecked() ([]byte, error) {
	if doc == nil || doc.Width <= 0 || doc.Height <= 0 {
		return nil, nil
	}
	return doc.renderLayersToSurfaceWithOptions(doc.ensureLayerRoot().Children(), true)
}

// renderCompositeSurface is a compatibility wrapper around
// renderCompositeSurfaceChecked for call sites that cannot propagate an error;
// it returns nil on failure. New code should prefer the checked variant.
func (doc *Document) renderCompositeSurface() []byte {
	buffer, err := doc.renderCompositeSurfaceChecked()
	if err != nil {
		return nil
	}
	return buffer
}

func compositeDocumentToViewport(canvas []byte, canvasW, canvasH int, doc *Document, vp *ViewportState, documentSurface []byte) {
	compositeDocumentToViewportClipped(canvas, canvasW, canvasH, doc, vp, documentSurface, nil)
}

// compositeDocumentToViewportClipped composites the document surface onto the
// canvas, optionally restricted to a canvas-space clip rectangle (the partial
// viewport resample path, PLAN.md S.4). The sampling loops compute the
// document coordinate of each canvas pixel directly from its absolute canvas
// position (no per-row float accumulation), so the output inside the clip is
// byte-identical to an unclipped pass regardless of where iteration starts.
func compositeDocumentToViewportClipped(canvas []byte, canvasW, canvasH int, doc *Document, vp *ViewportState, documentSurface []byte, clip *DirtyRect) {
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
	if clip != nil {
		clipX0 = maxInt(clipX0, clip.X)
		clipY0 = maxInt(clipY0, clip.Y)
		clipX1 = minInt(clipX1, clip.X+clip.W)
		clipY1 = minInt(clipY1, clip.Y+clip.H)
	}
	if clipX0 >= clipX1 || clipY0 >= clipY1 {
		return
	}

	// Identity fast path: at exactly 1:1 with no rotation and an integer-aligned
	// doc→canvas mapping, each canvas pixel maps to exactly one doc pixel. This
	// avoids the needless (and, at 1:1, slightly blurry) bilinear resample.
	// The mapping is derived from compositeViewportBilinearUnrotated:
	//   docX = (canvasX + 0.5 - halfCanvasW)/zoom + CenterX - 0.5
	// At zoom == 1 this reduces to docX = canvasX + (CenterX - halfCanvasW), so
	// the mapping is integer-aligned iff CenterX - halfCanvasW is an integer
	// (and likewise in Y).
	if zoom == 1.0 && vp.Rotation == 0 {
		offsetX := vp.CenterX - halfCanvasW
		offsetY := vp.CenterY - halfCanvasH
		baseX := int(math.Round(offsetX))
		baseY := int(math.Round(offsetY))
		if math.Abs(offsetX-float64(baseX)) < 1e-9 && math.Abs(offsetY-float64(baseY)) < 1e-9 {
			compositeViewportIdentity(canvas, canvasW, doc, documentSurface, baseX, baseY, clipX0, clipY0, clipX1, clipY1)
			return
		}
	}

	if useBilinear && math.Abs(sinTheta) < 1e-10 {
		compositeViewportBilinearUnrotated(canvas, canvasW, doc, documentSurface, vp, zoom, clipX0, clipY0, clipX1, clipY1, halfCanvasW, halfCanvasH)
		return
	}

	if useBilinear {
		compositeViewportBilinearRotated(canvas, canvasW, doc, documentSurface, clipX0, clipY0, clipX1, clipY1, cosTheta, sinTheta, zoom, halfCanvasW, halfCanvasH, vp)
		return
	}

	for canvasY := clipY0; canvasY < clipY1; canvasY++ {
		deltaY := (float64(canvasY) + 0.5) - halfCanvasH
		destIndex := (canvasY*canvasW + clipX0) * 4

		for canvasX := clipX0; canvasX < clipX1; canvasX++ {
			deltaX := (float64(canvasX) + 0.5) - halfCanvasW
			docX := (deltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX
			docY := (-deltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY
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

			destIndex += 4
		}
	}
}

// compositeViewportIdentity composites the document surface onto the canvas at
// exactly 1:1 with an integer-aligned mapping: canvas pixel (canvasX, canvasY)
// maps directly to doc pixel (canvasX+baseX, canvasY+baseY). Per-pixel semantics
// match the nearest-neighbour loop: opaque → direct RGBA copy, srcAlpha == 0 →
// skip, otherwise blend via compositePixelWithBlend.
func compositeViewportIdentity(canvas []byte, canvasW int, doc *Document, surf []byte, baseX, baseY, clipX0, clipY0, clipX1, clipY1 int) {
	docW := doc.Width
	docH := doc.Height
	stride := docW * 4

	for canvasY := clipY0; canvasY < clipY1; canvasY++ {
		sourceY := canvasY + baseY
		if sourceY < 0 || sourceY >= docH {
			continue
		}
		srcRow := sourceY * stride
		destIndex := (canvasY*canvasW + clipX0) * 4

		for canvasX := clipX0; canvasX < clipX1; canvasX++ {
			sourceX := canvasX + baseX
			if sourceX >= 0 && sourceX < docW {
				sourceIndex := srcRow + sourceX*4
				srcAlpha := surf[sourceIndex+3]
				if srcAlpha != 0 {
					if srcAlpha == 255 {
						copy(canvas[destIndex:destIndex+4], surf[sourceIndex:sourceIndex+4])
					} else {
						compositePixelWithBlend(canvas[destIndex:destIndex+4], surf[sourceIndex:sourceIndex+4], BlendModeNormal, 1, pixelNoiseSeed(canvasX, canvasY))
					}
				}
			}
			destIndex += 4
		}
	}
}

// bilinearPremultSample interpolates the four taps in premultiplied-alpha space
// and un-premultiplies the result. The document surface uses straight
// (non-premultiplied) alpha, so weighting RGB by alpha before interpolation
// prevents a transparent neighbour's (usually black 0,0,0) colour from bleeding
// into the interpolated pixel — the cause of dark fringes at layer/document
// edges. Weights are 16-bit fixed point (Σ = 65536); multiplying by an 8-bit
// alpha adds 8 bits, so intermediates stay well under 2^40 (int is 64-bit on the
// wasm/amd64 targets). ok is false when the interpolated alpha rounds to zero.
func bilinearPremultSample(surf []byte, off00, off10, off01, off11, w00, w10, w01, w11 int) (r, g, b, a int, ok bool) {
	a00 := int(surf[off00+3])
	a10 := int(surf[off10+3])
	a01 := int(surf[off01+3])
	a11 := int(surf[off11+3])

	sumA := a00*w00 + a10*w10 + a01*w01 + a11*w11
	a = (sumA + 32768) >> 16
	if a == 0 {
		// sumA < 32768 here (may still be > 0); nothing visible to store.
		return 0, 0, 0, 0, false
	}

	wa00 := a00 * w00
	wa10 := a10 * w10
	wa01 := a01 * w01
	wa11 := a11 * w11

	sumRA := int(surf[off00])*wa00 + int(surf[off10])*wa10 + int(surf[off01])*wa01 + int(surf[off11])*wa11
	sumGA := int(surf[off00+1])*wa00 + int(surf[off10+1])*wa10 + int(surf[off01+1])*wa01 + int(surf[off11+1])*wa11
	sumBA := int(surf[off00+2])*wa00 + int(surf[off10+2])*wa10 + int(surf[off01+2])*wa01 + int(surf[off11+2])*wa11

	// Un-premultiply with the existing +half rounding convention.
	half := sumA >> 1
	r = (sumRA + half) / sumA
	g = (sumGA + half) / sumA
	b = (sumBA + half) / sumA
	return r, g, b, a, true
}

// storeBilinearPixel writes an interpolated pixel to the canvas, taking the
// direct-store fast path when the pixel is fully opaque and otherwise blending.
func storeBilinearPixel(canvas []byte, destIndex, canvasX, canvasY, r, g, b, a int) {
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

		destIndex := (canvasY*canvasW + clipX0) * 4

		for canvasX := clipX0; canvasX < clipX1; canvasX++ {
			// Computed directly from the absolute canvas X (not accumulated
			// across the row) so a clipped pass starting mid-row produces the
			// exact same coordinates as a full pass.
			docX := ((float64(canvasX)+0.5)-halfCanvasW)*invZoom + vp.CenterX - 0.5
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

				if r, g, b, a, ok := bilinearPremultSample(surf, off00, off10, off01, off11, w00, w10, w01, w11); ok {
					storeBilinearPixel(canvas, destIndex, canvasX, canvasY, r, g, b, a)
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

				if r, g, b, a, ok := bilinearPremultSample(surf, off00, off10, off01, off11, w00, w10, w01, w11); ok {
					storeBilinearPixel(canvas, destIndex, canvasX, canvasY, r, g, b, a)
				}
			}

			destIndex += 4
		}
	}
}

// compositeViewportBilinearRotated handles bilinear sampling when the viewport
// is rotated. Both docX and docY vary per pixel. Sampling is inlined with
// fixed-point weights but Y weights cannot be hoisted. Coordinates are computed
// directly from the absolute canvas position (not accumulated) so a clipped
// pass is byte-identical to a full pass inside the clip rect.
func compositeViewportBilinearRotated(canvas []byte, canvasW int, doc *Document, surf []byte, clipX0, clipY0, clipX1, clipY1 int, cosTheta, sinTheta, zoom, halfCanvasW, halfCanvasH float64, vp *ViewportState) {
	docW := doc.Width
	docH := doc.Height
	stride := docW * 4
	maxX := docW - 1
	maxY := docH - 1

	for canvasY := clipY0; canvasY < clipY1; canvasY++ {
		deltaY := (float64(canvasY) + 0.5) - halfCanvasH
		destIndex := (canvasY*canvasW + clipX0) * 4

		for canvasX := clipX0; canvasX < clipX1; canvasX++ {
			deltaX := (float64(canvasX) + 0.5) - halfCanvasW
			docX := (deltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX - 0.5
			docY := (-deltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY - 0.5
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

			if r, g, b, a, ok := bilinearPremultSample(surf, off00, off10, off01, off11, w00, w10, w01, w11); ok {
				storeBilinearPixel(canvas, destIndex, canvasX, canvasY, r, g, b, a)
			}

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
