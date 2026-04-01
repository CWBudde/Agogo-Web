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
	command := &snapshotCommand{
		description: "Fill",
		applyFn: func(inst *instance) (snapshot, error) {
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
		},
	}
	return inst.history.Execute(inst, command)
}

func (inst *instance) handleApplyGradient(p ApplyGradientPayload) error {
	command := &snapshotCommand{
		description: "Gradient fill",
		applyFn: func(inst *instance) (snapshot, error) {
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
		},
	}
	return inst.history.Execute(inst, command)
}

func applyFillToDocument(inst *instance, doc *Document, p FillPayload) error {
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return fmt.Errorf("no active pixel layer")
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
				return patternFillColor(inst, docX, docY, p)
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
				return patternFillColor(inst, docX, docY, p)
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
			if coverage < 255 {
				color[3] = uint8((uint16(color[3]) * uint16(coverage)) / 255)
			}
			compositePixelWithBlend(dst[idx:idx+4], color[:], BlendModeNormal, 1, 0)
		}
	}
}

func applyGradientBufferToLayer(layer *PixelLayer, doc *Document, buffer []byte) {
	if layer == nil || len(buffer) < doc.Width*doc.Height*4 {
		return
	}
	lw := layer.Bounds.W
	lh := layer.Bounds.H
	for y := 0; y < lh; y++ {
		for x := 0; x < lw; x++ {
			docX := x + layer.Bounds.X
			docY := y + layer.Bounds.Y
			if docX < 0 || docY < 0 || docX >= doc.Width || docY >= doc.Height {
				continue
			}
			mask := selectionCoverageAt(doc.Selection, docX, docY)
			if mask == 0 {
				continue
			}
			srcIdx := (docY*doc.Width + docX) * 4
			src := buffer[srcIdx : srcIdx+4]
			if mask < 255 {
				src = append([]byte(nil), src...)
				src[3] = uint8((uint16(src[3]) * uint16(mask)) / 255)
			}
			dstIdx := (y*lw + x) * 4
			compositePixelWithBlend(layer.Pixels[dstIdx:dstIdx+4], src, BlendModeNormal, 1, 0)
		}
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

func patternFillColor(inst *instance, docX, docY int, p FillPayload) [4]uint8 {
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
	renderCustomGradient(buffer, width, height, p, startColor, endColor)
	if p.Dither {
		applyGradientDither(buffer, width, height)
	}
	return buffer
}

func renderGradientWithAgg(buffer []byte, width, height int, p ApplyGradientPayload, startColor, endColor [4]uint8) {
	renderer := agglib.NewAgg2D()
	renderer.Attach(buffer, width, height, width*4)
	renderer.NoLine()
	renderer.ResetTransformations()
	c1 := agglib.NewColor(startColor[0], startColor[1], startColor[2], startColor[3])
	c2 := agglib.NewColor(endColor[0], endColor[1], endColor[2], endColor[3])
	if p.Reverse {
		c1, c2 = c2, c1
	}

	renderer.ResetPath()
	switch p.Type {
	case GradientTypeRadial:
		cx := (p.StartX + p.EndX) * 0.5
		cy := (p.StartY + p.EndY) * 0.5
		radius := math.Hypot(p.EndX-p.StartX, p.EndY-p.StartY) * 0.5
		if radius < 1 {
			radius = 1
		}
		renderer.FillRadialGradient(cx, cy, radius, c1, c2, 1.0)
	default:
		renderer.FillLinearGradient(p.StartX, p.StartY, p.EndX, p.EndY, c1, c2, 1.0)
	}
	renderer.Rectangle(0, 0, float64(width), float64(height))
	renderer.DrawPath(agglib.FillOnly)
}

func renderCustomGradient(buffer []byte, width, height int, p ApplyGradientPayload, startColor, endColor [4]uint8) {
	dx := p.EndX - p.StartX
	dy := p.EndY - p.StartY
	length := math.Hypot(dx, dy)
	if length < 1 {
		length = 1
	}
	ux := dx / length
	uy := dy / length
	px := -uy
	py := ux
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			relX := float64(x) - p.StartX
			relY := float64(y) - p.StartY
			var t float64
			switch p.Type {
			case GradientTypeAngle:
				theta := math.Atan2(relY, relX)
				t = (theta + math.Pi) / (2 * math.Pi)
			case GradientTypeDiamond:
				u := math.Abs(relX*ux + relY*uy)
				v := math.Abs(relX*px + relY*py)
				t = (u + v) / length
			case GradientTypeReflected:
				proj := (relX*ux + relY*uy) / length
				proj = proj - math.Floor(proj)
				t = math.Abs(proj*2 - 1)
			default:
				t = (relX*ux + relY*uy) / length
			}
			if p.Reverse {
				t = 1 - t
			}
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			idx := (y*width + x) * 4
			writeGradientPixel(buffer[idx:idx+4], startColor, endColor, t)
		}
	}
}

func writeGradientPixel(dst []byte, c1, c2 [4]uint8, t float64) {
	if len(dst) < 4 {
		return
	}
	dst[0] = uint8(math.Round(float64(c1[0]) + (float64(c2[0])-float64(c1[0]))*t))
	dst[1] = uint8(math.Round(float64(c1[1]) + (float64(c2[1])-float64(c1[1]))*t))
	dst[2] = uint8(math.Round(float64(c1[2]) + (float64(c2[2])-float64(c1[2]))*t))
	dst[3] = uint8(math.Round(float64(c1[3]) + (float64(c2[3])-float64(c1[3]))*t))
}

func applyGradientDither(buffer []byte, width, height int) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 4
			noise := uint32(x*1103515245 ^ y*12345)
			jitter := float64((noise>>24)&0x7) / 255.0
			for channel := 0; channel < 3; channel++ {
				value := float64(buffer[idx+channel]) / 255.0
				value += (jitter - 0.014) * 0.25
				if value < 0 {
					value = 0
				} else if value > 1 {
					value = 1
				}
				buffer[idx+channel] = uint8(math.Round(value * 255))
			}
		}
	}
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
