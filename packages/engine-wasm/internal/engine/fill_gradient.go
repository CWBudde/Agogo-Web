package engine

import (
	"fmt"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

type fillDestinationMode string

const (
	fillDestinationPaint fillDestinationMode = "paint"
	fillDestinationLayer fillDestinationMode = "layer"
)

func (inst *instance) handleFill(p FillPayload) error {
	command := newSnapshotCommand("Fill", func(inst *instance) (snapshot, error) {
		doc := inst.manager.Active()
		if doc == nil {
			return snapshot{}, fmt.Errorf("no active document")
		}
		if err := applyFillToDocument(inst, doc, p); err != nil {
			return snapshot{}, err
		}
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return snapshot{}, err
		}
		return inst.captureSnapshot(), nil
	})
	return inst.history.Execute(inst, command)
}

func (inst *instance) handleApplyGradient(p ApplyGradientPayload) error {
	command := newSnapshotCommand("Gradient fill", func(inst *instance) (snapshot, error) {
		doc := inst.manager.Active()
		if doc == nil {
			return snapshot{}, fmt.Errorf("no active document")
		}
		if err := applyGradientToDocument(inst, doc, p); err != nil {
			return snapshot{}, err
		}
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return snapshot{}, err
		}
		return inst.captureSnapshot(), nil
	})
	return inst.history.Execute(inst, command)
}

func applyFillToDocument(inst *instance, doc *Document, p FillPayload) error {
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return fmt.Errorf("no active pixel layer")
	}
	// Filling into a new layer leaves the (possibly locked) active layer
	// untouched, so only the paint-onto-layer destination checks the lock.
	if !p.CreateLayer {
		if err := ensureLayerEditable(layer, editLayerPixels); err != nil {
			return err
		}
	}

	mode := fillDestinationPaint
	if p.CreateLayer {
		mode = fillDestinationLayer
	}

	surface, srcW, srcH := fillSourceSurface(inst, doc, layer, p.SampleMerged)
	sourceOriginX, sourceOriginY := 0, 0
	if !p.SampleMerged {
		sourceOriginX = layer.Bounds.X
		sourceOriginY = layer.Bounds.Y
	}
	mask := buildFillMask(surface, srcW, srcH, sourceOriginX, sourceOriginY, p)
	if mask == nil {
		return nil
	}

	fillColor := resolveFillColor(inst, p)
	if p.Source == "pattern" {
		fillColor = [4]uint8{0, 0, 0, 0}
	}

	switch mode {
	case fillDestinationLayer:
		raster := make([]byte, doc.Width*doc.Height*4)
		maskOriginX, maskOriginY := 0, 0
		if !p.SampleMerged {
			maskOriginX = layer.Bounds.X
			maskOriginY = layer.Bounds.Y
		}
		fillRasterWithMask(raster, doc.Width, doc.Height, mask, doc.Selection, maskOriginX, maskOriginY, 0, 0, func(docX, docY int) [4]uint8 {
			if p.Source == "pattern" {
				return patternFillColor(inst, doc, docX, docY, p)
			}
			return fillColor
		})
		newLayer := NewPixelLayer(fillLayerName(p.Source), LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, raster)
		parentID := ""
		insertIndex := -1
		if _, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID); ok && parent != nil {
			parentID = parent.ID()
			insertIndex = index + 1
		}
		if err := doc.AddLayer(newLayer, parentID, insertIndex); err != nil {
			return err
		}
		return nil
	default:
		maskOriginX, maskOriginY := 0, 0
		if !p.SampleMerged {
			maskOriginX = layer.Bounds.X
			maskOriginY = layer.Bounds.Y
		}
		fillRasterWithMask(layer.Pixels, layer.Bounds.W, layer.Bounds.H, mask, doc.Selection, maskOriginX, maskOriginY, layer.Bounds.X, layer.Bounds.Y, func(docX, docY int) [4]uint8 {
			if p.Source == "pattern" {
				return patternFillColor(inst, doc, docX, docY, p)
			}
			return fillColor
		})
		doc.touchModifiedAt()
		return nil
	}
}

func applyGradientToDocument(inst *instance, doc *Document, p ApplyGradientPayload) error {
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return fmt.Errorf("no active pixel layer")
	}
	// A gradient rendered into a new layer leaves the (possibly locked)
	// active layer untouched, so only the paint destination checks the lock.
	if !p.CreateLayer {
		if err := ensureLayerEditable(layer, editLayerPixels); err != nil {
			return err
		}
	}

	mode := fillDestinationPaint
	if p.CreateLayer {
		mode = fillDestinationLayer
	}

	buffer := renderGradientSurface(doc.Width, doc.Height, p, inst.foregroundColor, inst.backgroundColor)
	if buffer == nil {
		return fmt.Errorf("failed to render gradient")
	}

	switch mode {
	case fillDestinationLayer:
		applySelectionMaskToDocBuffer(buffer, doc, doc.Selection)
		newLayer := NewPixelLayer("Gradient Fill", LayerBounds{X: 0, Y: 0, W: doc.Width, H: doc.Height}, buffer)
		parentID := ""
		insertIndex := -1
		if _, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID); ok && parent != nil {
			parentID = parent.ID()
			insertIndex = index + 1
		}
		if err := doc.AddLayer(newLayer, parentID, insertIndex); err != nil {
			return err
		}
		return nil
	default:
		applyGradientBufferToLayer(layer, doc, buffer)
		doc.touchModifiedAt()
		return nil
	}
}

func fillSourceSurface(inst *instance, doc *Document, layer *PixelLayer, sampleMerged bool) ([]byte, int, int) {
	if sampleMerged {
		return inst.compositeSurface(doc), doc.Width, doc.Height
	}
	return layer.Pixels, layer.Bounds.W, layer.Bounds.H
}

func buildFillMask(surface []byte, width, height, sourceOriginX, sourceOriginY int, p FillPayload) *Selection {
	if len(surface) < width*height*4 {
		return nil
	}

	if !p.HasPoint {
		mask := newSelection(width, height)
		for i := range mask.Mask {
			mask.Mask[i] = 255
		}
		return mask
	}

	px := int(math.Round(p.X)) - sourceOriginX
	py := int(math.Round(p.Y)) - sourceOriginY
	if px < 0 || py < 0 || px >= width || py >= height {
		if p.Tolerance == 0 && !p.Contiguous {
			mask := newSelection(width, height)
			for i := range mask.Mask {
				mask.Mask[i] = 255
			}
			return mask
		}
		return nil
	}

	targetColor, ok := sampleSurfaceColor(surface, width, height, px, py)
	if !ok {
		return nil
	}
	if p.Contiguous {
		return magicWandFloodFill(surface, width, height, px, py, p.Tolerance)
	}
	return selectColorRange(surface, width, height, targetColor, p.Tolerance)
}

func resolveFillColor(inst *instance, p FillPayload) [4]uint8 {
	switch p.Source {
	case "background":
		return inst.backgroundColor
	case "color":
		if p.Color != ([4]uint8{}) {
			return p.Color
		}
		fallthrough
	case "foreground", "":
		return inst.foregroundColor
	default:
		return inst.foregroundColor
	}
}

func fillRasterWithMask(dst []byte, width, height int, fillMask *Selection, selection *Selection, maskOriginX, maskOriginY, dstOriginX, dstOriginY int, colorAt func(docX, docY int) [4]uint8) {
	if len(dst) < width*height*4 {
		return
	}
	sourcePixels := make([]byte, width*height*4)
	mask := agglib.NewAlphaMask(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			docX := x + dstOriginX
			docY := y + dstOriginY
			coverage := selectionCoverageAt(fillMask, docX-maskOriginX, docY-maskOriginY)
			if coverage == 0 {
				continue
			}
			if sel := selectionCoverageAt(selection, docX, docY); sel == 0 {
				continue
			} else if sel < 255 {
				coverage = uint8((uint16(coverage) * uint16(sel)) / 255)
			}
			idx := (y*width + x) * 4
			color := colorAt(docX, docY)
			copy(sourcePixels[idx:idx+4], color[:])
			mask.Pix[y*width+x] = coverage
		}
	}
	destination := agglib.NewImage(dst, width, height, width*4)
	source := agglib.NewImage(sourcePixels, width, height, width*4)
	if err := agglib.CompositeImage(
		destination, source,
		agglib.Rect{X1: 0, Y1: 0, X2: width, Y2: height},
		agglib.PointI{},
		agglib.CompositeOptions{BlendMode: agglib.BlendSrcOver, Opacity: 1, AlphaMode: agglib.AlphaStraight, Mask: &mask},
	); err != nil {
		return
	}
}

func applyGradientBufferToLayer(layer *PixelLayer, doc *Document, buffer []byte) {
	if layer == nil || len(buffer) < doc.Width*doc.Height*4 {
		return
	}
	docX0 := maxInt(layer.Bounds.X, 0)
	docY0 := maxInt(layer.Bounds.Y, 0)
	docX1 := minInt(layer.Bounds.X+layer.Bounds.W, doc.Width)
	docY1 := minInt(layer.Bounds.Y+layer.Bounds.H, doc.Height)
	if docX0 >= docX1 || docY0 >= docY1 {
		return
	}
	var mask *agglib.AlphaMask
	if doc.Selection != nil {
		selectionMask := agglib.NewAlphaMask(layer.Bounds.W, layer.Bounds.H)
		for y := docY0; y < docY1; y++ {
			for x := docX0; x < docX1; x++ {
				selectionMask.Pix[(y-layer.Bounds.Y)*layer.Bounds.W+x-layer.Bounds.X] = selectionCoverageAt(doc.Selection, x, y)
			}
		}
		mask = &selectionMask
	}
	destination := agglib.NewImage(layer.Pixels, layer.Bounds.W, layer.Bounds.H, layer.Bounds.W*4)
	source := agglib.NewImage(buffer, doc.Width, doc.Height, doc.Width*4)
	if err := agglib.CompositeImage(
		destination, source,
		agglib.Rect{X1: docX0, Y1: docY0, X2: docX1, Y2: docY1},
		agglib.PointI{X: docX0 - layer.Bounds.X, Y: docY0 - layer.Bounds.Y},
		agglib.CompositeOptions{BlendMode: agglib.BlendSrcOver, Opacity: 1, AlphaMode: agglib.AlphaStraight, Mask: mask},
	); err != nil {
		return
	}
}

func applySelectionMaskToDocBuffer(buffer []byte, doc *Document, selection *Selection) {
	if doc == nil || selection == nil || len(buffer) < doc.Width*doc.Height*4 {
		return
	}
	for y := 0; y < doc.Height; y++ {
		for x := 0; x < doc.Width; x++ {
			coverage := selectionCoverageAt(selection, x, y)
			if coverage == 0 {
				idx := (y*doc.Width + x) * 4
				buffer[idx+3] = 0
				continue
			}
			if coverage < 255 {
				idx := (y*doc.Width + x) * 4
				buffer[idx+3] = uint8((uint16(buffer[idx+3]) * uint16(coverage)) / 255)
			}
		}
	}
}

// patternFillColor returns the pattern-source fill color at a document
// coordinate. A resolvable PatternID samples the pattern tile; an empty or
// unknown ID keeps the legacy foreground/background 8px checker byte-identical.
func patternFillColor(inst *instance, doc *Document, docX, docY int, p FillPayload) [4]uint8 {
	if pattern := resolvePattern(doc, p.PatternID); pattern != nil {
		return samplePatternColor(pattern, docX, docY, p.PatternScale)
	}
	size := 8
	if (docX/size+docY/size)%2 == 0 {
		return inst.foregroundColor
	}
	return inst.backgroundColor
}

func fillLayerName(source string) string {
	switch source {
	case "background":
		return "Background Fill"
	case "pattern":
		return "Pattern Fill"
	default:
		return "Fill"
	}
}

func selectionCoverageAt(selection *Selection, docX, docY int) byte {
	if selection == nil || selection.Width <= 0 || selection.Height <= 0 {
		return 255
	}
	if docX < 0 || docY < 0 || docX >= selection.Width || docY >= selection.Height {
		return 0
	}
	return selection.Mask[docY*selection.Width+docX]
}

func renderGradientSurface(width, height int, p ApplyGradientPayload, startColor, endColor [4]uint8) []byte {
	if width <= 0 || height <= 0 {
		return nil
	}
	buffer := make([]byte, width*height*4)
	stops := make([]agglib.GradientStop, 0, len(p.Stops)+2)
	if len(p.Stops) == 0 {
		stops = append(
			stops,
			agglib.GradientStop{Position: 0, Color: agglib.NewColor(startColor[0], startColor[1], startColor[2], startColor[3])},
			agglib.GradientStop{Position: 1, Color: agglib.NewColor(endColor[0], endColor[1], endColor[2], endColor[3])},
		)
	} else {
		for _, stop := range p.Stops {
			color := clampGradientColor(stop.Color)
			stops = append(stops, agglib.GradientStop{
				Position: clampGradientPosition(stop.Position),
				Color:    agglib.NewColor(color[0], color[1], color[2], color[3]),
			})
		}
	}
	lut, err := agglib.NewGradientLUT(stops, 256)
	if err != nil {
		return nil
	}
	shape := agglib.GradientShapeLinear
	end := agglib.Point{X: p.EndX, Y: p.EndY}
	switch p.Type {
	case GradientTypeRadial:
		shape = agglib.GradientShapeRadial
	case GradientTypeAngle:
		shape = agglib.GradientShapeAngular
		// The established engine contract uses End only as the angular radius;
		// the zero-angle ray remains the positive document X axis.
		end = agglib.Point{X: p.StartX + math.Hypot(p.EndX-p.StartX, p.EndY-p.StartY), Y: p.StartY}
	case GradientTypeDiamond:
		shape = agglib.GradientShapeDiamond
	case GradientTypeReflected:
		shape = agglib.GradientShapeReflected
	}
	image := agglib.NewImage(buffer, width, height, width*4)
	if err := agglib.RenderGradient(image, lut, agglib.GradientRenderOptions{
		Shape:   shape,
		Start:   agglib.Point{X: p.StartX, Y: p.StartY},
		End:     end,
		Reverse: p.Reverse,
		Dither:  p.Dither,
	}); err != nil {
		return nil
	}
	return buffer
}

func buildGradientLUT(stops []GradientStopPayload, startColor, endColor [4]uint8) [256]agglib.Color {
	gradientStops := make([]agglib.GradientStop, 0, len(stops)+2)
	if len(stops) == 0 {
		gradientStops = append(
			gradientStops,
			agglib.GradientStop{Position: 0, Color: agglib.NewColor(startColor[0], startColor[1], startColor[2], startColor[3])},
			agglib.GradientStop{Position: 1, Color: agglib.NewColor(endColor[0], endColor[1], endColor[2], endColor[3])},
		)
	} else {
		for _, stop := range stops {
			color := clampGradientColor(stop.Color)
			gradientStops = append(gradientStops, agglib.GradientStop{
				Position: clampGradientPosition(stop.Position),
				Color:    agglib.NewColor(color[0], color[1], color[2], color[3]),
			})
		}
	}
	var lut [256]agglib.Color
	publicLUT, err := agglib.NewGradientLUT(gradientStops, len(lut))
	if err != nil {
		return lut
	}
	for index := range lut {
		lut[index] = publicLUT.At(index)
	}
	return lut
}

func clampGradientPosition(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func clampGradientColor(color [4]uint8) [4]uint8 {
	return [4]uint8{
		color[0],
		color[1],
		color[2],
		color[3],
	}
}

func gradientColorAt(lut [256]agglib.Color, t float64) [4]uint8 {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	index := int(math.Round(t * 255.0))
	if index < 0 {
		index = 0
	} else if index > 255 {
		index = 255
	}
	c := lut[index]
	return [4]uint8{c.R, c.G, c.B, c.A}
}

func sampleSurfaceColorAverage(surface []byte, width, height, x, y, sampleSize int) ([4]uint8, bool) {
	if sampleSize < 1 {
		sampleSize = 1
	}
	if sampleSize%2 == 0 {
		sampleSize++
	}
	if len(surface) < width*height*4 || x < 0 || y < 0 || x >= width || y >= height {
		return [4]uint8{}, false
	}
	radius := sampleSize / 2
	var sum [4]int
	var count int
	for sy := y - radius; sy <= y+radius; sy++ {
		if sy < 0 || sy >= height {
			continue
		}
		for sx := x - radius; sx <= x+radius; sx++ {
			if sx < 0 || sx >= width {
				continue
			}
			idx := (sy*width + sx) * 4
			sum[0] += int(surface[idx])
			sum[1] += int(surface[idx+1])
			sum[2] += int(surface[idx+2])
			sum[3] += int(surface[idx+3])
			count++
		}
	}
	if count == 0 {
		return [4]uint8{}, false
	}
	return [4]uint8{
		uint8(sum[0] / count),
		uint8(sum[1] / count),
		uint8(sum[2] / count),
		uint8(sum[3] / count),
	}, true
}
