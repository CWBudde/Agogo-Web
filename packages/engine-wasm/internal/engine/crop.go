package engine

import (
	"fmt"
	"math"
)

// CropState holds the live state while the crop tool is active.
type CropState struct {
	Active bool
	X      float64
	Y      float64
	W      float64
	H      float64
}

// CropMeta is serialized into UIMeta so the frontend can render
// the crop overlay and handles.
type CropMeta struct {
	Active bool    `json:"active"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
}

// meta builds the UIMeta representation of the current state.
func (s *CropState) meta() *CropMeta {
	if s == nil || !s.Active {
		return nil
	}
	return &CropMeta{
		Active: true,
		X:      s.X,
		Y:      s.Y,
		W:      s.W,
		H:      s.H,
	}
}

// UpdateCropPayload defines the parameters for updating the crop box.
type UpdateCropPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
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

	const darkenFactor = 0.5
	for cy := 0; cy < canvasH; cy++ {
		for cx := 0; cx < canvasW; cx++ {
			dx, dy := canvasToDoc(cx, cy)
			if dx < state.X || dx > state.X+state.W || dy < state.Y || dy > state.Y+state.H {
				i := (cy*canvasW + cx) * 4
				reuse[i] = byte(float64(reuse[i]) * darkenFactor)
				reuse[i+1] = byte(float64(reuse[i+1]) * darkenFactor)
				reuse[i+2] = byte(float64(reuse[i+2]) * darkenFactor)
			}
		}
	}

	// 2. Draw crop box and grid
	x0, y0 := docToCanvas(state.X, state.Y)
	x1, y1 := docToCanvas(state.X+state.W, state.Y)
	x2, y2 := docToCanvas(state.X+state.W, state.Y+state.H)
	x3, y3 := docToCanvas(state.X, state.Y+state.H)

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
	for i := 1; i < 3; i++ {
		t := float64(i) / 3.0
		ax, ay := docToCanvas(state.X+state.W*t, state.Y)
		bx, by := docToCanvas(state.X+state.W*t, state.Y+state.H)
		drawLine(ax, ay, bx, by, gridColor)
		cx, cy := docToCanvas(state.X, state.Y+state.H*t)
		dx, dy := docToCanvas(state.X+state.W, state.Y+state.H*t)
		drawLine(cx, cy, dx, dy, gridColor)
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

	mx0, my0 := docToCanvas(state.X+state.W*0.5, state.Y)
	mx1, my1 := docToCanvas(state.X+state.W, state.Y+state.H*0.5)
	mx2, my2 := docToCanvas(state.X+state.W*0.5, state.Y+state.H)
	mx3, my3 := docToCanvas(state.X, state.Y+state.H*0.5)
	drawHandle(mx0, my0)
	drawHandle(mx1, my1)
	drawHandle(mx2, my2)
	drawHandle(mx3, my3)

	return reuse
}
