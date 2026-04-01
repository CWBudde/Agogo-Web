// Package agg wraps the Go port of Anti-Grain Geometry (AGG).
package agg

import (
	"math"

	agglib "github.com/cwbudde/agg_go"
)

type Document struct {
	Width      int
	Height     int
	Background string
}

type Viewport struct {
	CenterX    float64
	CenterY    float64
	Zoom       float64
	Rotation   float64
	CanvasW    int
	CanvasH    int
	ShowGuides bool
}

const checkerTile = 24

var (
	canvasBackground = agglib.NewColor(17, 22, 30, 255)
	checkerA         = agglib.NewColor(57, 64, 76, 255)
	checkerB         = agglib.NewColor(42, 48, 59, 255)
	docWhite         = agglib.NewColor(244, 246, 250, 255)
	docColor         = agglib.NewColor(236, 147, 92, 255)
	docBorder        = agglib.NewColor(73, 195, 255, 255)
	centerGuide      = agglib.NewColor(255, 255, 255, 48)
)

// RenderViewport renders the current document shell and overlays using the public
// agg_go Agg2D facade from v0.2.6.
func RenderViewport(doc *Document, vp *Viewport, reuse []byte) []byte {
	reuse = RenderViewportBase(doc, vp, reuse)
	return RenderViewportOverlays(doc, vp, reuse)
}

func RenderViewportBase(doc *Document, vp *Viewport, reuse []byte) []byte {
	width := maxInt(vp.CanvasW, 1)
	height := maxInt(vp.CanvasH, 1)
	size := width * height * 4
	if len(reuse) != size {
		reuse = make([]byte, size)
	}

	renderer := agglib.NewAgg2D()
	renderer.Attach(reuse, width, height, width*4)
	renderer.ClearAll(canvasBackground)
	renderer.NoLine()

	configureViewportTransform(renderer, width, height, vp)

	minX, minY, maxX, maxY := visibleWorldBounds(renderer, width, height, doc)
	renderDocumentBackground(renderer, doc, minX, minY, maxX, maxY)

	return reuse
}

func RenderViewportOverlays(doc *Document, vp *Viewport, reuse []byte) []byte {
	width := maxInt(vp.CanvasW, 1)
	height := maxInt(vp.CanvasH, 1)
	size := width * height * 4
	if len(reuse) != size {
		reuse = make([]byte, size)
	}

	renderer := agglib.NewAgg2D()
	renderer.Attach(reuse, width, height, width*4)
	renderer.NoLine()
	renderDocumentBorder(renderer, doc, vp)
	if vp.ShowGuides {
		configureViewportTransform(renderer, width, height, vp)
		renderGuides(renderer, doc, vp)
	}

	return reuse
}

func configureViewportTransform(renderer *agglib.Agg2D, width, height int, vp *Viewport) {
	const degToRad = math.Pi / 180
	renderer.ResetTransformations()
	renderer.Translate(-vp.CenterX, -vp.CenterY)
	renderer.Scale(vp.Zoom, vp.Zoom)
	renderer.Rotate(vp.Rotation * degToRad)
	renderer.Translate(float64(width)*0.5, float64(height)*0.5)
}

func renderDocumentBackground(renderer *agglib.Agg2D, doc *Document, minX, minY, maxX, maxY float64) {
	renderer.NoLine()

	switch doc.Background {
	case "white":
		renderer.FillColor(docWhite)
		renderer.Rectangle(0, 0, float64(doc.Width), float64(doc.Height))
	case "color":
		renderer.FillColor(docColor)
		renderer.Rectangle(0, 0, float64(doc.Width), float64(doc.Height))
	default:
		drawCheckerboard(renderer, minX, minY, maxX, maxY, doc)
	}
}

func renderDocumentBorder(renderer *agglib.Agg2D, doc *Document, vp *Viewport) {
	if shouldRenderBorderInScreenSpace(doc, vp) {
		renderDocumentBorderScreenSpace(renderer, doc, vp)
		return
	}

	lineWidth := 1.5
	if vp.Zoom > 0 {
		lineWidth = math.Max(1.0/vp.Zoom, 0.75)
	}
	configureViewportTransform(renderer, maxInt(vp.CanvasW, 1), maxInt(vp.CanvasH, 1), vp)
	renderer.LineWidth(lineWidth)
	renderer.LineColor(docBorder)
	renderer.NoFill()
	renderer.Rectangle(0, 0, float64(doc.Width), float64(doc.Height))
}

func shouldRenderBorderInScreenSpace(doc *Document, vp *Viewport) bool {
	if doc == nil || vp == nil {
		return false
	}
	return vp.Zoom >= 4
}

func renderDocumentBorderScreenSpace(renderer *agglib.Agg2D, doc *Document, vp *Viewport) {
	width := maxInt(vp.CanvasW, 1)
	height := maxInt(vp.CanvasH, 1)
	corners := [4][2]float64{
		projectDocToScreen(0, 0, vp),
		projectDocToScreen(float64(doc.Width), 0, vp),
		projectDocToScreen(float64(doc.Width), float64(doc.Height), vp),
		projectDocToScreen(0, float64(doc.Height), vp),
	}

	renderer.ResetTransformations()
	renderer.LineColor(docBorder)
	renderer.NoFill()
	renderer.LineWidth(0.75)

	for i := 0; i < len(corners); i++ {
		start := corners[i]
		end := corners[(i+1)%len(corners)]
		x0, y0, x1, y1, ok := clipLineToCanvas(start[0], start[1], end[0], end[1], width, height)
		if !ok {
			continue
		}
		renderer.Line(x0, y0, x1, y1)
	}
}

func projectDocToScreen(x, y float64, vp *Viewport) [2]float64 {
	rotation := vp.Rotation * (math.Pi / 180)
	cosTheta := math.Cos(rotation)
	sinTheta := math.Sin(rotation)
	dx := x - vp.CenterX
	dy := y - vp.CenterY
	return [2]float64{
		dx*cosTheta*vp.Zoom - dy*sinTheta*vp.Zoom + float64(maxInt(vp.CanvasW, 1))*0.5,
		dx*sinTheta*vp.Zoom + dy*cosTheta*vp.Zoom + float64(maxInt(vp.CanvasH, 1))*0.5,
	}
}

func clipLineToCanvas(x0, y0, x1, y1 float64, width, height int) (float64, float64, float64, float64, bool) {
	const (
		clipLeft = 1 << iota
		clipRight
		clipBottom
		clipTop
	)

	minX := 0.0
	minY := 0.0
	maxX := float64(maxInt(width, 1) - 1)
	maxY := float64(maxInt(height, 1) - 1)

	outCode := func(x, y float64) int {
		code := 0
		if x < minX {
			code |= clipLeft
		} else if x > maxX {
			code |= clipRight
		}
		if y < minY {
			code |= clipTop
		} else if y > maxY {
			code |= clipBottom
		}
		return code
	}

	code0 := outCode(x0, y0)
	code1 := outCode(x1, y1)

	for {
		if code0|code1 == 0 {
			return x0, y0, x1, y1, true
		}
		if code0&code1 != 0 {
			return 0, 0, 0, 0, false
		}

		codeOut := code0
		if codeOut == 0 {
			codeOut = code1
		}

		var x, y float64
		switch {
		case codeOut&clipTop != 0:
			if y1 == y0 {
				return 0, 0, 0, 0, false
			}
			x = x0 + (x1-x0)*(minY-y0)/(y1-y0)
			y = minY
		case codeOut&clipBottom != 0:
			if y1 == y0 {
				return 0, 0, 0, 0, false
			}
			x = x0 + (x1-x0)*(maxY-y0)/(y1-y0)
			y = maxY
		case codeOut&clipRight != 0:
			if x1 == x0 {
				return 0, 0, 0, 0, false
			}
			y = y0 + (y1-y0)*(maxX-x0)/(x1-x0)
			x = maxX
		default:
			if x1 == x0 {
				return 0, 0, 0, 0, false
			}
			y = y0 + (y1-y0)*(minX-x0)/(x1-x0)
			x = minX
		}

		if codeOut == code0 {
			x0, y0 = x, y
			code0 = outCode(x0, y0)
		} else {
			x1, y1 = x, y
			code1 = outCode(x1, y1)
		}
	}
}

func drawCheckerboard(renderer *agglib.Agg2D, minX, minY, maxX, maxY float64, doc *Document) {
	startX := maxInt(int(math.Floor(minX/float64(checkerTile)))*checkerTile, 0)
	startY := maxInt(int(math.Floor(minY/float64(checkerTile)))*checkerTile, 0)
	endX := minInt(int(math.Ceil(maxX/float64(checkerTile)))*checkerTile, doc.Width)
	endY := minInt(int(math.Ceil(maxY/float64(checkerTile)))*checkerTile, doc.Height)

	for y := startY; y < endY; y += checkerTile {
		for x := startX; x < endX; x += checkerTile {
			if ((x/checkerTile)+(y/checkerTile))%2 == 0 {
				renderer.FillColor(checkerA)
			} else {
				renderer.FillColor(checkerB)
			}

			x2 := minInt(x+checkerTile, doc.Width)
			y2 := minInt(y+checkerTile, doc.Height)
			renderer.Rectangle(float64(x), float64(y), float64(x2), float64(y2))
		}
	}
}

func renderGuides(renderer *agglib.Agg2D, doc *Document, vp *Viewport) {
	renderer.LineColor(centerGuide)
	if vp.Zoom > 0 {
		renderer.LineWidth(math.Max(0.5/vp.Zoom, 0.5))
	}
	renderer.Line(float64(doc.Width)/2, 0, float64(doc.Width)/2, float64(doc.Height))
	renderer.Line(0, float64(doc.Height)/2, float64(doc.Width), float64(doc.Height)/2)
}

func visibleWorldBounds(renderer *agglib.Agg2D, width, height int, doc *Document) (float64, float64, float64, float64) {
	corners := [4][2]float64{
		{0, 0},
		{float64(width), 0},
		{float64(width), float64(height)},
		{0, float64(height)},
	}

	minX := math.Inf(1)
	minY := math.Inf(1)
	maxX := math.Inf(-1)
	maxY := math.Inf(-1)

	for _, corner := range corners {
		x, y := corner[0], corner[1]
		renderer.ScreenToWorld(&x, &y)
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}

	minX = math.Max(minX, 0)
	minY = math.Max(minY, 0)
	maxX = math.Min(maxX, float64(doc.Width))
	maxY = math.Min(maxY, float64(doc.Height))
	return minX, minY, maxX, maxY
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
