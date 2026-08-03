package engine

import (
	"fmt"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

const (
	cropOverlayThirds   = "thirds"
	cropOverlayGrid     = "grid"
	cropOverlayDiagonal = "diagonal"
	cropOverlayNone     = "none"
)

// CropState holds the live state while the crop tool is active.
type CropState struct {
	Active           bool
	X                float64
	Y                float64
	W                float64
	H                float64
	Rotation         float64 // degrees, 0 = no rotation
	DeletePixels     bool
	ContentAwareFill bool
	Resolution       float64
	OverlayType      string
}

// CropMeta is serialized into UIMeta so the frontend can render
// the crop overlay and handles.
type CropMeta struct {
	Active           bool    `json:"active"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	W                float64 `json:"w"`
	H                float64 `json:"h"`
	Rotation         float64 `json:"rotation"`
	DeletePixels     bool    `json:"deletePixels"`
	ContentAwareFill bool    `json:"contentAwareFill"`
	Resolution       float64 `json:"resolution"`
	OverlayType      string  `json:"overlayType"`
}

// meta builds the UIMeta representation of the current state.
func (s *CropState) meta() *CropMeta {
	if s == nil || !s.Active {
		return nil
	}
	return &CropMeta{
		Active:           true,
		X:                s.X,
		Y:                s.Y,
		W:                s.W,
		H:                s.H,
		Rotation:         s.Rotation,
		DeletePixels:     s.DeletePixels,
		ContentAwareFill: s.ContentAwareFill,
		Resolution:       s.Resolution,
		OverlayType:      normalizeCropOverlayType(s.OverlayType),
	}
}

// UpdateCropPayload defines the parameters for updating the crop box.
type UpdateCropPayload struct {
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	W                float64 `json:"w"`
	H                float64 `json:"h"`
	Rotation         float64 `json:"rotation"`
	DeletePixels     bool    `json:"deletePixels"`
	ContentAwareFill bool    `json:"contentAwareFill"`
	Resolution       float64 `json:"resolution"`
	OverlayType      string  `json:"overlayType"`
}

func normalizeCropOverlayType(value string) string {
	switch value {
	case cropOverlayGrid, cropOverlayDiagonal, cropOverlayNone:
		return value
	default:
		return cropOverlayThirds
	}
}

func normalizeCropResolution(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	return defaultResolutionDPI
}

// ResizeCanvasPayload defines the parameters for the Canvas Size command.
type ResizeCanvasPayload struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Anchor string `json:"anchor"`
}

// canvasAnchorOffset returns the (dx, dy) translation applied to document-space
// geometry when the canvas is resized from oldW×oldH to newW×newH using the
// given anchor placement.
func canvasAnchorOffset(anchor string, oldW, oldH, newW, newH int) (dx, dy int) {
	switch anchor {
	case "top-left":
		dx, dy = 0, 0
	case "top-center":
		dx, dy = (newW-oldW)/2, 0
	case "top-right":
		dx, dy = newW-oldW, 0
	case "middle-left":
		dx, dy = 0, (newH-oldH)/2
	case "center":
		dx, dy = (newW-oldW)/2, (newH-oldH)/2
	case "middle-right":
		dx, dy = newW-oldW, (newH-oldH)/2
	case "bottom-left":
		dx, dy = 0, newH-oldH
	case "bottom-center":
		dx, dy = (newW-oldW)/2, newH-oldH
	case "bottom-right":
		dx, dy = newW-oldW, newH-oldH
	default:
		dx, dy = 0, 0 // fallback to top-left
	}
	return dx, dy
}

// applyResizeCanvas resizes the document and remaps every layer kind, its masks
// and the document selections into the new document space based on the anchor.
func applyResizeCanvas(doc *Document, w, h int, anchor string) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid canvas dimensions: %dx%d", w, h)
	}

	dx, dy := canvasAnchorOffset(anchor, doc.Width, doc.Height, w, h)

	doc.Width = w
	doc.Height = h

	walkLayerTree(doc.LayerRoot, func(n LayerNode) {
		if pl, ok := n.(*PixelLayer); ok {
			pl.Bounds.X += dx
			pl.Bounds.Y += dy
		}
		remapLayerIntoDocumentSpace(n, w, h, dx, dy)
	})
	remapDocumentSelections(doc, w, h, dx, dy)
	return nil
}

// translatePathInPlace shifts every anchor point and its bezier handles by
// (dx, dy). Handle coordinates are stored in absolute document space, so they
// translate alongside the anchor.
func translatePathInPlace(p *Path, dx, dy float64) {
	if p == nil {
		return
	}
	for si := range p.Subpaths {
		pts := p.Subpaths[si].Points
		for pi := range pts {
			pts[pi].X += dx
			pts[pi].Y += dy
			pts[pi].InX += dx
			pts[pi].InY += dy
			pts[pi].OutX += dx
			pts[pi].OutY += dy
		}
	}
}

// remapDocMask resamples a document-space, single-channel (alpha) buffer of size
// srcW×srcH into a new dstW×dstH buffer translated by (dx, dy). Destination
// pixels that map outside the source are left at 0.
func remapDocMask(data []byte, srcW, srcH, dstW, dstH, dx, dy int) []byte {
	out := make([]byte, dstW*dstH)
	if srcW <= 0 || srcH <= 0 || len(data) < srcW*srcH {
		return out
	}
	for ny := range dstH {
		sy := ny - dy
		if sy < 0 || sy >= srcH {
			continue
		}
		dstRow := ny * dstW
		srcRow := sy * srcW
		for nx := range dstW {
			sx := nx - dx
			if sx < 0 || sx >= srcW {
				continue
			}
			out[dstRow+nx] = data[srcRow+sx]
		}
	}
	return out
}

// remapLayerMaskInPlace crops/extends a document-space layer mask to newW×newH,
// translating its coverage by (dx, dy).
func remapLayerMaskInPlace(mask *LayerMask, newW, newH, dx, dy int) {
	if mask == nil || mask.Width <= 0 || mask.Height <= 0 {
		return
	}
	mask.Data = remapDocMask(mask.Data, mask.Width, mask.Height, newW, newH, dx, dy)
	mask.Width = newW
	mask.Height = newH
}

// remapSelection translates a document-space selection into a newW×newH space by
// (dx, dy), clipping to the new bounds. Returns nil when the selection becomes
// empty (fully outside the new document), which deselects.
func remapSelection(sel *Selection, newW, newH, dx, dy int) *Selection {
	if sel == nil || sel.Width <= 0 || sel.Height <= 0 {
		return nil
	}
	remapped := &Selection{
		Width:  newW,
		Height: newH,
		Mask:   remapDocMask(sel.Mask, sel.Width, sel.Height, newW, newH, dx, dy),
	}
	return normalizeSelection(remapped)
}

// remapDocumentSelections translates the active, last and saved selections into
// the new document space, deselecting any that become empty.
func remapDocumentSelections(doc *Document, newW, newH, dx, dy int) {
	doc.Selection = remapSelection(doc.Selection, newW, newH, dx, dy)
	doc.LastSelection = remapSelection(doc.LastSelection, newW, newH, dx, dy)
	for i := range doc.SavedSelections {
		doc.SavedSelections[i].Selection = remapSelection(doc.SavedSelections[i].Selection, newW, newH, dx, dy)
	}
}

// remapLayerIntoDocumentSpace translates a single layer's document-space
// auxiliary data (raster mask, vector mask) into the new document space and, for
// text and vector layers, shifts their position by (dx, dy) and re-rasterizes
// their cached raster against the new document dimensions so it never mismatches
// the resized document. Pixel-layer bounds and pixel content are handled by the
// caller (crop resamples/trims; canvas-size shifts).
func remapLayerIntoDocumentSpace(n LayerNode, newW, newH, dx, dy int) {
	remapLayerMaskInPlace(n.Mask(), newW, newH, dx, dy)
	translatePathInPlace(n.VectorMask(), float64(dx), float64(dy))

	switch layer := n.(type) {
	case *TextLayer:
		// Text rasters are bounds-local: translate the position — the anchor
		// (pen origin) must shift with the bounds or the next rasterization
		// would snap the text back — and re-rasterize.
		layer.Bounds.X += dx
		layer.Bounds.Y += dy
		layer.AnchorX += float64(dx)
		layer.AnchorY += float64(dy)
		_ = rasterizeTextLayer(layer)
	case *VectorLayer:
		// Vector rasters are document-local (doc-sized, Bounds == full document,
		// Shape points in absolute document space): translate the geometry, resize
		// the bounds to the new document and re-rasterize at the new dimensions.
		translatePathInPlace(layer.Shape, float64(dx), float64(dy))
		layer.Bounds = LayerBounds{X: 0, Y: 0, W: newW, H: newH}
		if layer.Shape != nil && len(layer.Shape.Subpaths) > 0 {
			if raster, err := rasterizeVectorShapeFillRule(layer.Shape, newW, newH, layer.FillColor, layer.StrokeColor, layer.StrokeWidth, layer.FillRule); err == nil {
				layer.CachedRaster = raster
			}
		}
	case *AdjustmentLayer:
		// Cached adjustment output is document-sized; invalidate so it recomputes
		// against the new dimensions.
		layer.Cache = AdjustmentCache{}
	case *GroupLayer:
		if layer.Artboard != nil {
			layer.Artboard.Bounds.X += dx
			layer.Artboard.Bounds.Y += dy
		}
	}
}

// RenderCropOverlay draws the darkened area outside the crop box and the
// crop handles/grid onto the canvas buffer.
func RenderCropOverlay(state *CropState, vp *ViewportState, reuse []byte) []byte {
	if state == nil || !state.Active || len(reuse) == 0 {
		return reuse
	}

	canvasW := maxInt(vp.CanvasW, 1)
	canvasH := maxInt(vp.CanvasH, 1)
	zoom := clampZoom(vp.Zoom)
	radians := vp.Rotation * (math.Pi / 180)
	cosTheta := math.Cos(radians)
	sinTheta := math.Sin(radians)
	halfCanvasW := float64(canvasW) * 0.5
	halfCanvasH := float64(canvasH) * 0.5

	docToCanvas := func(docX, docY float64) (cx, cy int) {
		dx := docX - vp.CenterX
		dy := docY - vp.CenterY
		sx := dx*cosTheta*zoom - dy*sinTheta*zoom + halfCanvasW
		sy := dx*sinTheta*zoom + dy*cosTheta*zoom + halfCanvasH
		return int(math.Round(sx)), int(math.Round(sy))
	}

	setPixelBlend := func(cx, cy int, col overlayColor) {
		if cx < 0 || cx >= canvasW || cy < 0 || cy >= canvasH {
			return
		}
		i := (cy*canvasW + cx) * 4
		a := float64(col.A) / 255
		reuse[i] = byte(float64(reuse[i])*(1-a) + float64(col.R)*a)
		reuse[i+1] = byte(float64(reuse[i+1])*(1-a) + float64(col.G)*a)
		reuse[i+2] = byte(float64(reuse[i+2])*(1-a) + float64(col.B)*a)
		reuse[i+3] = 255
	}

	// 1. Darken the area outside the crop box
	invZoom := 1.0 / zoom
	invCos := math.Cos(-radians)
	invSin := math.Sin(-radians)

	canvasToDoc := func(cx, cy int) (dx, dy float64) {
		sx := float64(cx) - halfCanvasW
		sy := float64(cy) - halfCanvasH
		rx := sx*invCos - sy*invSin
		ry := sx*invSin + sy*invCos
		return rx*invZoom + vp.CenterX, ry*invZoom + vp.CenterY
	}

	// Crop rotation support: precompute crop-space transform
	cropRad := state.Rotation * (math.Pi / 180)
	cropCosR := math.Cos(cropRad)
	cropSinR := math.Sin(cropRad)
	cropCX := state.X + state.W/2
	cropCY := state.Y + state.H/2
	halfW := state.W / 2
	halfH := state.H / 2

	// isInsideCrop returns true if a doc-space point is inside the (possibly rotated) crop box.
	isInsideCrop := func(docX, docY float64) bool {
		// Translate to crop-center-relative
		tx := docX - cropCX
		ty := docY - cropCY
		// Apply inverse rotation (rotate by -cropRad)
		localX := tx*cropCosR + ty*cropSinR
		localY := -tx*cropSinR + ty*cropCosR
		return localX >= -halfW && localX <= halfW && localY >= -halfH && localY <= halfH
	}

	// cropLocalToDoc converts a crop-local offset (relative to crop center) to doc space.
	cropLocalToDoc := func(lx, ly float64) (float64, float64) {
		return cropCX + lx*cropCosR - ly*cropSinR, cropCY + lx*cropSinR + ly*cropCosR
	}

	const darkenFactor = 0.5
	for cy := range canvasH {
		for cx := range canvasW {
			dx, dy := canvasToDoc(cx, cy)
			if !isInsideCrop(dx, dy) {
				i := (cy*canvasW + cx) * 4
				reuse[i] = byte(float64(reuse[i]) * darkenFactor)
				reuse[i+1] = byte(float64(reuse[i+1]) * darkenFactor)
				reuse[i+2] = byte(float64(reuse[i+2]) * darkenFactor)
			}
		}
	}

	// 2. Draw crop box and grid — corners are rotated around the crop center.
	c0dx, c0dy := cropLocalToDoc(-halfW, -halfH) // TL
	c1dx, c1dy := cropLocalToDoc(halfW, -halfH)  // TR
	c2dx, c2dy := cropLocalToDoc(halfW, halfH)   // BR
	c3dx, c3dy := cropLocalToDoc(-halfW, halfH)  // BL
	x0, y0 := docToCanvas(c0dx, c0dy)
	x1, y1 := docToCanvas(c1dx, c1dy)
	x2, y2 := docToCanvas(c2dx, c2dy)
	x3, y3 := docToCanvas(c3dx, c3dy)

	boxColor := overlayColor{255, 255, 255, 200}
	drawLine := func(ax, ay, bx, by int, col overlayColor) {
		dx := bx - ax
		dy := by - ay
		steps := maxInt(absInt(dx), absInt(dy))
		if steps == 0 {
			setPixelBlend(ax, ay, col)
			return
		}
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			cx := ax + int(math.Round(float64(dx)*t))
			cy := ay + int(math.Round(float64(dy)*t))
			setPixelBlend(cx, cy, col)
		}
	}
	drawLine(x0, y0, x1, y1, boxColor)
	drawLine(x1, y1, x2, y2, boxColor)
	drawLine(x2, y2, x3, y3, boxColor)
	drawLine(x3, y3, x0, y0, boxColor)

	gridColor := overlayColor{255, 255, 255, 100}
	drawGuideLine := func(axDoc, ayDoc, bxDoc, byDoc float64) {
		ax, ay := docToCanvas(axDoc, ayDoc)
		bx, by := docToCanvas(bxDoc, byDoc)
		drawLine(ax, ay, bx, by, gridColor)
	}

	drawFractionGuides := func(divisions int) {
		for i := 1; i < divisions; i++ {
			t := float64(i) / float64(divisions)
			lx := -halfW + state.W*t
			topX, topY := cropLocalToDoc(lx, -halfH)
			bottomX, bottomY := cropLocalToDoc(lx, halfH)
			drawGuideLine(topX, topY, bottomX, bottomY)

			ly := -halfH + state.H*t
			leftX, leftY := cropLocalToDoc(-halfW, ly)
			rightX, rightY := cropLocalToDoc(halfW, ly)
			drawGuideLine(leftX, leftY, rightX, rightY)
		}
	}

	switch normalizeCropOverlayType(state.OverlayType) {
	case cropOverlayGrid:
		drawFractionGuides(5)
	case cropOverlayDiagonal:
		drawGuideLine(c0dx, c0dy, c2dx, c2dy)
		drawGuideLine(c1dx, c1dy, c3dx, c3dy)
	case cropOverlayNone:
		// Hide interior guides while keeping the crop bounds and handles visible.
	default:
		drawFractionGuides(3)
	}

	// 3. Draw 8 resize handles
	handleSize := 5
	drawHandle := func(cx, cy int) {
		for dy := -handleSize; dy <= handleSize; dy++ {
			for dx := -handleSize; dx <= handleSize; dx++ {
				if dx == -handleSize || dx == handleSize || dy == -handleSize || dy == handleSize {
					setPixelBlend(cx+dx, cy+dy, overlayColor{0, 0, 0, 200})
				} else {
					setPixelBlend(cx+dx, cy+dy, overlayColor{255, 255, 255, 255})
				}
			}
		}
	}

	drawHandle(x0, y0)
	drawHandle(x1, y1)
	drawHandle(x2, y2)
	drawHandle(x3, y3)

	m0dx, m0dy := cropLocalToDoc(0, -halfH) // top edge mid
	m1dx, m1dy := cropLocalToDoc(halfW, 0)  // right edge mid
	m2dx, m2dy := cropLocalToDoc(0, halfH)  // bottom edge mid
	m3dx, m3dy := cropLocalToDoc(-halfW, 0) // left edge mid
	mx0, my0 := docToCanvas(m0dx, m0dy)
	mx1, my1 := docToCanvas(m1dx, m1dy)
	mx2, my2 := docToCanvas(m2dx, m2dy)
	mx3, my3 := docToCanvas(m3dx, m3dy)
	drawHandle(mx0, my0)
	drawHandle(mx1, my1)
	drawHandle(mx2, my2)
	drawHandle(mx3, my3)

	return reuse
}

// applyRotatedCropToPixelLayer resamples a pixel layer's pixels for a rotated
// crop commit. For each output pixel at (ox, oy) in the new W×H document, it
// computes the source position in the original layer via inverse rotation around
// the crop center, then samples bilinearly.
func applyRotatedCropToPixelLayer(pl *PixelLayer, cx, cy, w, h, rotRad float64) (newPixels []byte, newBounds LayerBounds) {
	outW := int(math.Round(w))
	outH := int(math.Round(h))
	newPixels = make([]byte, outW*outH*4)
	cosR := math.Cos(rotRad)
	sinR := math.Sin(rotRad)
	tx := w/2 + cosR*(float64(pl.Bounds.X)-cx) + sinR*(float64(pl.Bounds.Y)-cy)
	ty := h/2 - sinR*(float64(pl.Bounds.X)-cx) + cosR*(float64(pl.Bounds.Y)-cy)
	transform := agglib.NewTransformationsFromValues(cosR, -sinR, sinR, cosR, tx, ty)
	renderCropAffine(newPixels, outW, outH, pl.Pixels, pl.Bounds.W, pl.Bounds.H, transform)
	return newPixels, LayerBounds{X: 0, Y: 0, W: outW, H: outH}
}

func renderCropAffine(dst []byte, dstW, dstH int, src []byte, srcW, srcH int, transform *agglib.Transformations) {
	if dstW <= 0 || dstH <= 0 || srcW <= 0 || srcH <= 0 || len(dst) < dstW*dstH*4 || len(src) < srcW*srcH*4 {
		return
	}
	dstImage := agglib.NewImage(dst, dstW, dstH, dstW*4)
	srcImage := agglib.NewImage(src, srcW, srcH, srcW*4)
	if err := agglib.DrawImageAffine(dstImage, srcImage, agglib.Rect{X1: 0, Y1: 0, X2: srcW, Y2: srcH}, transform, agglib.ImageTransformOptions{
		Filter:           agglib.Bilinear,
		Resample:         agglib.NoResample,
		EdgeMode:         agglib.ImageEdgeClamp,
		SourceAlpha:      agglib.AlphaStraight,
		DestinationAlpha: agglib.AlphaStraight,
		BlendMode:        agglib.BlendSrc,
		Opacity:          1,
		SampleOffset:     agglib.Point{X: 0.5, Y: 0.5},
	}); err != nil {
		clear(dst)
	}
}

func buildContentAwareCropFillLayer(source []byte, sourceW, sourceH int, cropX, cropY, cropW, cropH, rotRad float64) ([]byte, bool) {
	outW := int(math.Round(cropW))
	outH := int(math.Round(cropH))
	if outW <= 0 || outH <= 0 || len(source) < sourceW*sourceH*4 {
		return nil, false
	}

	fillPixels := make([]byte, outW*outH*4)
	known := make([]bool, outW*outH)
	expansionMask := make([]bool, outW*outH)
	hasExpansion := false

	cx := cropX + cropW/2
	cy := cropY + cropH/2
	cosR := math.Cos(rotRad)
	sinR := math.Sin(rotRad)
	tx := cropW/2 - cosR*cx - sinR*cy
	ty := cropH/2 + sinR*cx - cosR*cy
	transform := agglib.NewTransformationsFromValues(cosR, -sinR, sinR, cosR, tx, ty)
	renderCropAffine(fillPixels, outW, outH, source, sourceW, sourceH, transform)

	for oy := range outH {
		for ox := range outW {
			lx := float64(ox) + 0.5 - cropW/2
			ly := float64(oy) + 0.5 - cropH/2
			srcX := cx + lx*cosR - ly*sinR
			srcY := cy + lx*sinR + ly*cosR
			idx := oy*outW + ox
			if srcX >= 0 && srcX < float64(sourceW) && srcY >= 0 && srcY < float64(sourceH) {
				known[idx] = true
				continue
			}
			expansionMask[idx] = true
			hasExpansion = true
		}
	}

	if !hasExpansion {
		return nil, false
	}

	diffuseCropExpansion(fillPixels, outW, outH, known, expansionMask)

	hasOpaqueFill := false
	for idx := range outW * outH {
		base := idx * 4
		if !expansionMask[idx] {
			fillPixels[base] = 0
			fillPixels[base+1] = 0
			fillPixels[base+2] = 0
			fillPixels[base+3] = 0
			continue
		}
		if fillPixels[base+3] != 0 {
			hasOpaqueFill = true
		}
	}

	if !hasOpaqueFill {
		return nil, false
	}
	return fillPixels, true
}

func diffuseCropExpansion(pixels []byte, width, height int, known, expansionMask []bool) {
	if len(pixels) < width*height*4 || len(known) < width*height || len(expansionMask) < width*height {
		return
	}

	queue := make([]int, 0, width*height/4)
	queued := make([]bool, width*height)
	for idx, masked := range expansionMask {
		if !masked {
			continue
		}
		if !hasKnownCropNeighbor(idx, width, height, known) {
			continue
		}
		queue = append(queue, idx)
		queued[idx] = true
	}

	for head := 0; head < len(queue); head++ {
		idx := queue[head]
		if known[idx] || !expansionMask[idx] {
			continue
		}

		var sum [4]float64
		var total float64
		cx := idx % width
		cy := idx / width
		for ny := maxInt(0, cy-1); ny <= minInt(height-1, cy+1); ny++ {
			for nx := maxInt(0, cx-1); nx <= minInt(width-1, cx+1); nx++ {
				nidx := ny*width + nx
				if nidx == idx || !known[nidx] {
					continue
				}
				weight := 1.0
				if nx != cx && ny != cy {
					weight = 0.70710678118
				}
				base := nidx * 4
				for channel := range 4 {
					sum[channel] += float64(pixels[base+channel]) * weight
				}
				total += weight
			}
		}
		if total == 0 {
			continue
		}

		base := idx * 4
		for channel := range 4 {
			pixels[base+channel] = byte(math.Round(sum[channel] / total))
		}
		known[idx] = true

		for ny := maxInt(0, cy-1); ny <= minInt(height-1, cy+1); ny++ {
			for nx := maxInt(0, cx-1); nx <= minInt(width-1, cx+1); nx++ {
				nidx := ny*width + nx
				if !expansionMask[nidx] || known[nidx] || queued[nidx] {
					continue
				}
				queue = append(queue, nidx)
				queued[nidx] = true
			}
		}
	}
}

func hasKnownCropNeighbor(idx, width, height int, known []bool) bool {
	if idx < 0 || idx >= width*height {
		return false
	}
	x := idx % width
	y := idx / width
	for ny := maxInt(0, y-1); ny <= minInt(height-1, y+1); ny++ {
		for nx := maxInt(0, x-1); nx <= minInt(width-1, x+1); nx++ {
			nidx := ny*width + nx
			if nidx != idx && known[nidx] {
				return true
			}
		}
	}
	return false
}

// trimPixelLayerToBounds zeros out pixel data outside the given document bounds.
// The layer's Bounds are already shifted (post-crop origin shift). Pixels outside
// [0, docW) x [0, docH) in doc space are cleared.
func trimPixelLayerToBounds(pl *PixelLayer, docW, docH int) {
	for ly := range pl.Bounds.H {
		for lx := range pl.Bounds.W {
			dx := pl.Bounds.X + lx
			dy := pl.Bounds.Y + ly
			if dx < 0 || dx >= docW || dy < 0 || dy >= docH {
				i := (ly*pl.Bounds.W + lx) * 4
				pl.Pixels[i] = 0
				pl.Pixels[i+1] = 0
				pl.Pixels[i+2] = 0
				pl.Pixels[i+3] = 0
			}
		}
	}
}
