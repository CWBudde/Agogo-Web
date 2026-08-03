package engine

import (
	"math"

	agglib "github.com/cwbudde/agg_go"
)

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
// viewport resample path, PLAN.md S.4). Sampling is delegated to agg_go's
// affine image pipeline so rotation, scale, interpolation, alpha handling, and
// clipping share one transform-driven implementation.
func compositeDocumentToViewportClipped(canvas []byte, canvasW, canvasH int, doc *Document, vp *ViewportState, documentSurface []byte, clip *DirtyRect) {
	compositeDocumentToViewportClippedAlpha(canvas, canvasW, canvasH, doc, vp, documentSurface, clip, agglib.AlphaStraight)
}

func compositeDocumentToViewportClippedAlpha(canvas []byte, canvasW, canvasH int, doc *Document, vp *ViewportState, documentSurface []byte, clip *DirtyRect, sourceAlpha agglib.AlphaMode) {
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
	scaleCos := zoom * cosTheta
	scaleSin := zoom * sinTheta
	documentToCanvas := agglib.NewTransformationsFromValues(
		scaleCos,
		scaleSin,
		-scaleSin,
		scaleCos,
		halfCanvasW-scaleCos*vp.CenterX+scaleSin*vp.CenterY,
		halfCanvasH-scaleSin*vp.CenterX-scaleCos*vp.CenterY,
	)

	clipX0, clipY0, clipX1, clipY1 := docBoundsOnCanvasTransformed(doc, documentToCanvas, canvasW, canvasH)
	visibleX0, visibleY0, visibleX1, visibleY1 := clipX0, clipY0, clipX1, clipY1
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
			compositeViewportIdentity(canvas, canvasW, canvasH, doc, documentSurface, baseX, baseY, clipX0, clipY0, clipX1, clipY1, sourceAlpha)
			return
		}
	}

	filter := agglib.ImageFilterNoFilter
	resample := agglib.NoResample
	if useBilinear {
		filter = agglib.ImageFilterBilinear
	}

	// Map document-space rectangle edges into canvas space. AGG performs its
	// inverse sampling at destination pixel centres, preserving the viewport's
	// existing center and rotation convention without separate rotated and
	// unrotated code paths.
	destination := agglib.NewImage(canvas, canvasW, canvasH, canvasW*4)
	source := agglib.NewImage(documentSurface, doc.Width, doc.Height, doc.Width*4)
	aggClip := agglib.Rect{X1: clipX0, Y1: clipY0, X2: clipX1, Y2: clipY1}
	// Bound the source to the document area visible in the full viewport, not
	// the dirty clip. Full and partial renders therefore use identical span
	// origins (byte-identical DDA sampling) while high zoom avoids rasterizing a
	// transformed polygon for the entire document.
	sourceRect := viewportVisibleDocumentRect(
		doc,
		vp,
		visibleX0,
		visibleY0,
		visibleX1,
		visibleY1,
		cosTheta,
		sinTheta,
		zoom,
		halfCanvasW,
		halfCanvasH,
	)
	if err := agglib.DrawImageAffine(
		destination,
		source,
		sourceRect,
		documentToCanvas,
		agglib.ImageTransformOptions{
			Filter:           filter,
			Resample:         resample,
			EdgeMode:         agglib.ImageEdgeClamp,
			SourceAlpha:      sourceAlpha,
			DestinationAlpha: agglib.AlphaStraight,
			BlendMode:        agglib.BlendSrcOver,
			Opacity:          1,
			Clip:             &aggClip,
		},
	); err != nil {
		// Dimensions, strides, alpha modes, and the transform are validated
		// above, so this is only reachable if the library rejects a future
		// option combination. Leave the already-rendered viewport base intact.
		return
	}
}

func viewportVisibleDocumentRect(doc *Document, vp *ViewportState, clipX0, clipY0, clipX1, clipY1 int, cosTheta, sinTheta, zoom, halfCanvasW, halfCanvasH float64) agglib.Rect {
	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64
	corners := [4][2]float64{
		{float64(clipX0), float64(clipY0)},
		{float64(clipX1), float64(clipY0)},
		{float64(clipX1), float64(clipY1)},
		{float64(clipX0), float64(clipY1)},
	}
	for _, corner := range corners {
		deltaX := corner[0] - halfCanvasW
		deltaY := corner[1] - halfCanvasH
		docX := (deltaX*cosTheta+deltaY*sinTheta)/zoom + vp.CenterX
		docY := (-deltaX*sinTheta+deltaY*cosTheta)/zoom + vp.CenterY
		minX = math.Min(minX, docX)
		minY = math.Min(minY, docY)
		maxX = math.Max(maxX, docX)
		maxY = math.Max(maxY, docY)
	}
	const filterAndRasterMargin = 2
	return agglib.Rect{
		X1: clampInt(int(math.Floor(minX))-filterAndRasterMargin, 0, doc.Width),
		Y1: clampInt(int(math.Floor(minY))-filterAndRasterMargin, 0, doc.Height),
		X2: clampInt(int(math.Ceil(maxX))+filterAndRasterMargin, 0, doc.Width),
		Y2: clampInt(int(math.Ceil(maxY))+filterAndRasterMargin, 0, doc.Height),
	}
}

// compositeViewportIdentity composites the document surface onto the canvas at
// exactly 1:1 with an integer-aligned mapping: canvas pixel (canvasX, canvasY)
// maps directly to doc pixel (canvasX+baseX, canvasY+baseY). The fast path
// remains a single clipped source-over composite and avoids affine setup.
func compositeViewportIdentity(canvas []byte, canvasW, canvasH int, doc *Document, surf []byte, baseX, baseY, clipX0, clipY0, clipX1, clipY1 int, sourceAlpha agglib.AlphaMode) {
	sourceX0 := clampInt(clipX0+baseX, 0, doc.Width)
	sourceY0 := clampInt(clipY0+baseY, 0, doc.Height)
	sourceX1 := clampInt(clipX1+baseX, 0, doc.Width)
	sourceY1 := clampInt(clipY1+baseY, 0, doc.Height)
	if sourceX0 >= sourceX1 || sourceY0 >= sourceY1 {
		return
	}
	if rgbaRectOpaque(surf, doc.Width, sourceX0, sourceY0, sourceX1, sourceY1) {
		widthBytes := (sourceX1 - sourceX0) * 4
		destinationX := sourceX0 - baseX
		destinationY := sourceY0 - baseY
		for row := 0; row < sourceY1-sourceY0; row++ {
			sourceOffset := ((sourceY0+row)*doc.Width + sourceX0) * 4
			destinationOffset := ((destinationY+row)*canvasW + destinationX) * 4
			copy(canvas[destinationOffset:destinationOffset+widthBytes], surf[sourceOffset:sourceOffset+widthBytes])
		}
		return
	}
	destination := agglib.NewImage(canvas, canvasW, canvasH, canvasW*4)
	source := agglib.NewImage(surf, doc.Width, doc.Height, doc.Width*4)
	if err := agglib.CompositeImage(
		destination, source,
		agglib.Rect{X1: sourceX0, Y1: sourceY0, X2: sourceX1, Y2: sourceY1},
		agglib.PointI{X: sourceX0 - baseX, Y: sourceY0 - baseY},
		agglib.CompositeOptions{BlendMode: agglib.BlendSrcOver, Opacity: 1, AlphaMode: sourceAlpha},
	); err != nil {
		return
	}
}

func rgbaRectOpaque(pixels []byte, stridePixels, x0, y0, x1, y1 int) bool {
	for y := y0; y < y1; y++ {
		rowOffset := y * stridePixels * 4
		for x := x0; x < x1; x++ {
			if pixels[rowOffset+x*4+3] != 255 {
				return false
			}
		}
	}
	return true
}

// docBoundsOnCanvas returns the canvas pixel rectangle that bounds the document,
// clamped to [0,canvasW) × [0,canvasH). The forward transform maps document
// coordinates to canvas coordinates, so we project the four document corners.
func docBoundsOnCanvas(doc *Document, vp *ViewportState, canvasW, canvasH int, zoom, cosTheta, sinTheta, halfCanvasW, halfCanvasH float64) (x0, y0, x1, y1 int) {
	scaleCos := zoom * cosTheta
	scaleSin := zoom * sinTheta
	documentToCanvas := agglib.NewTransformationsFromValues(
		scaleCos,
		scaleSin,
		-scaleSin,
		scaleCos,
		halfCanvasW-scaleCos*vp.CenterX+scaleSin*vp.CenterY,
		halfCanvasH-scaleSin*vp.CenterX-scaleCos*vp.CenterY,
	)
	return docBoundsOnCanvasTransformed(doc, documentToCanvas, canvasW, canvasH)
}

func docBoundsOnCanvasTransformed(doc *Document, documentToCanvas *agglib.Transformations, canvasW, canvasH int) (x0, y0, x1, y1 int) {
	docW := float64(doc.Width)
	docH := float64(doc.Height)

	minSX := math.MaxFloat64
	minSY := math.MaxFloat64
	maxSX := -math.MaxFloat64
	maxSY := -math.MaxFloat64

	corners := [4][2]float64{{0, 0}, {docW, 0}, {docW, docH}, {0, docH}}
	for _, c := range corners {
		sx, sy := documentToCanvas.Transform(c[0], c[1])
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
