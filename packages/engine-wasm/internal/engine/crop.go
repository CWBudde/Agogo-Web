package engine

import (
	"fmt"
	"math"
)

const (
	cropOverlayThirds   = "thirds"
	cropOverlayGrid     = "grid"
	cropOverlayDiagonal = "diagonal"
	cropOverlayNone     = "none"
)

// CropState holds the live state while the crop tool is active.
type CropState struct {
	Active       bool
	X            float64
	Y            float64
	W            float64
	H            float64
	Rotation     float64 // degrees, 0 = no rotation
	DeletePixels bool
	Resolution   float64
	OverlayType  string
}

// CropMeta is serialized into UIMeta so the frontend can render
// the crop overlay and handles.
type CropMeta struct {
	Active       bool    `json:"active"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	W            float64 `json:"w"`
	H            float64 `json:"h"`
	Rotation     float64 `json:"rotation"`
	DeletePixels bool    `json:"deletePixels"`
	Resolution   float64 `json:"resolution"`
	OverlayType  string  `json:"overlayType"`
}

// meta builds the UIMeta representation of the current state.
func (s *CropState) meta() *CropMeta {
	if s == nil || !s.Active {
		return nil
	}
	return &CropMeta{
		Active:       true,
		X:            s.X,
		Y:            s.Y,
		W:            s.W,
		H:            s.H,
		Rotation:     s.Rotation,
		DeletePixels: s.DeletePixels,
		Resolution:   s.Resolution,
		OverlayType:  normalizeCropOverlayType(s.OverlayType),
	}
}

// UpdateCropPayload defines the parameters for updating the crop box.
type UpdateCropPayload struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	W            float64 `json:"w"`
	H            float64 `json:"h"`
	Rotation     float64 `json:"rotation"`
	DeletePixels bool    `json:"deletePixels"`
	Resolution   float64 `json:"resolution"`
	OverlayType  string  `json:"overlayType"`
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

// applyResizeCanvas resizes the document and shifts layers based on the anchor.
func applyResizeCanvas(doc *Document, w, h int, anchor string) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid canvas dimensions: %dx%d", w, h)
	}

	dx := 0
	dy := 0

	oldW := doc.Width
	oldH := doc.Height

	switch anchor {
	case "top-left":
		dx, dy = 0, 0
	case "top-center":
		dx = (w - oldW) / 2
		dy = 0
	case "top-right":
		dx = w - oldW
		dy = 0
	case "middle-left":
		dx = 0
		dy = (h - oldH) / 2
	case "center":
		dx = (w - oldW) / 2
		dy = (h - oldH) / 2
	case "middle-right":
		dx = w - oldW
		dy = (h - oldH) / 2
	case "bottom-left":
		dx = 0
		dy = h - oldH
	case "bottom-center":
		dx = (w - oldW) / 2
		dy = h - oldH
	case "bottom-right":
		dx = w - oldW
		dy = h - oldH
	default:
		dx, dy = 0, 0 // fallback to top-left
	}

	doc.Width = w
	doc.Height = h

	// Shift all pixel layers by dx, dy
	if dx != 0 || dy != 0 {
		walkLayerTree(doc.LayerRoot, func(n LayerNode) {
			if pl, ok := n.(*PixelLayer); ok {
				pl.Bounds.X += dx
				pl.Bounds.Y += dy
			}
		})
	}
	return nil
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

	for oy := range outH {
		for ox := range outW {
			// Crop-local position (relative to crop center in output space)
			lx := float64(ox) + 0.5 - w/2
			ly := float64(oy) + 0.5 - h/2
			// Inverse-rotate to original doc space (rotate by rotRad)
			srcX := cx + lx*cosR - ly*sinR
			srcY := cy + lx*sinR + ly*cosR
			// Transform to layer-local space
			layerX := srcX - float64(pl.Bounds.X)
			layerY := srcY - float64(pl.Bounds.Y)
			// sampleBilinear uses pixel-center convention (lx+0.5, ly+0.5)
			pix := sampleBilinear(pl.Pixels, pl.Bounds.W, pl.Bounds.H, layerX+0.5, layerY+0.5)
			i := (oy*outW + ox) * 4
			newPixels[i] = pix[0]
			newPixels[i+1] = pix[1]
			newPixels[i+2] = pix[2]
			newPixels[i+3] = pix[3]
		}
	}
	return newPixels, LayerBounds{X: 0, Y: 0, W: outW, H: outH}
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
