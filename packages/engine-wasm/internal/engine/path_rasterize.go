package engine

import (
	"fmt"

	agglib "github.com/cwbudde/agg_go"
)

// rasterizePathToMask renders a Path to an 8-bit alpha mask.
// The mask is width*height bytes, where 255 = inside, 0 = outside.
func rasterizePathToMask(p *Path, width, height int) ([]byte, error) {
	if p == nil || len(p.Subpaths) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	// Create an RGBA buffer (Agg2D needs RGBA), render white-on-black,
	// then extract the red channel as the mask.
	stride := width * 4
	buf := make([]byte, stride*height)

	r := agglib.NewAgg2D()
	r.Attach(buf, width, height, stride)
	r.ClearAll(agglib.NewColor(0, 0, 0, 255))
	r.FillColor(agglib.NewColor(255, 255, 255, 255))
	r.NoLine()
	r.FillEvenOdd(true) // compound paths with holes
	r.ResetPath()
	applyPathToAgg2D(r, p)
	r.DrawPath(agglib.FillOnly)

	// Extract red channel as mask (white = 255, black = 0).
	mask := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		mask[i] = buf[i*4] // Red channel
	}
	return mask, nil
}

// makeSelectionFromPath creates a Selection from the given path.
func (doc *Document) makeSelectionFromPath(pathIdx int) error {
	if pathIdx < 0 || pathIdx >= len(doc.Paths) {
		return fmt.Errorf("path index %d out of range", pathIdx)
	}
	p := &doc.Paths[pathIdx].Path
	mask, err := rasterizePathToMask(p, doc.Width, doc.Height)
	if err != nil {
		return err
	}
	doc.Selection = &Selection{
		Width:  doc.Width,
		Height: doc.Height,
		Mask:   mask,
	}
	return nil
}

// fillPathOnDoc fills the given path with a color onto the active pixel layer
// of the provided document.
func fillPathOnDoc(doc *Document, pathIdx int, color [4]uint8) error {
	if pathIdx < 0 || pathIdx >= len(doc.Paths) {
		return fmt.Errorf("path index %d out of range", pathIdx)
	}

	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return fmt.Errorf("no active pixel layer")
	}

	bounds := layer.Bounds
	stride := bounds.W * 4
	if len(layer.Pixels) < stride*bounds.H {
		return fmt.Errorf("layer pixel buffer too small")
	}

	r := agglib.NewAgg2D()
	r.Attach(layer.Pixels, bounds.W, bounds.H, stride)
	r.FillColor(agglib.NewColor(color[0], color[1], color[2], color[3]))
	r.NoLine()
	r.FillEvenOdd(true)

	// Path coordinates are in document space; offset by layer bounds origin.
	r.ResetTransformations()
	r.Translate(-float64(bounds.X), -float64(bounds.Y))
	r.ResetPath()
	applyPathToAgg2D(r, &doc.Paths[pathIdx].Path)
	r.DrawPath(agglib.FillOnly)

	return nil
}

// strokePathOnDoc strokes the given path onto the active pixel layer
// of the provided document.
func strokePathOnDoc(doc *Document, pathIdx int, width float64, color [4]uint8) error {
	if pathIdx < 0 || pathIdx >= len(doc.Paths) {
		return fmt.Errorf("path index %d out of range", pathIdx)
	}

	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return fmt.Errorf("no active pixel layer")
	}

	bounds := layer.Bounds
	stride := bounds.W * 4

	r := agglib.NewAgg2D()
	r.Attach(layer.Pixels, bounds.W, bounds.H, stride)
	r.NoFill()
	r.LineColor(agglib.NewColor(color[0], color[1], color[2], color[3]))
	r.LineWidth(width)

	// Path coordinates are in document space; offset by layer bounds origin.
	r.ResetTransformations()
	r.Translate(-float64(bounds.X), -float64(bounds.Y))
	r.ResetPath()
	applyPathToAgg2D(r, &doc.Paths[pathIdx].Path)
	r.DrawPath(agglib.StrokeOnly)

	return nil
}
