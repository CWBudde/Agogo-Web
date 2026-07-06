package engine

import (
	"fmt"
	"math"
)

type SelectionCombineMode string

const (
	SelectionCombineReplace   SelectionCombineMode = "replace"
	SelectionCombineAdd       SelectionCombineMode = "add"
	SelectionCombineSubtract  SelectionCombineMode = "subtract"
	SelectionCombineIntersect SelectionCombineMode = "intersect"
)

type SelectionShape string

const (
	SelectionShapeRect    SelectionShape = "rect"
	SelectionShapeEllipse SelectionShape = "ellipse"
	SelectionShapePolygon SelectionShape = "polygon"
)

type SelectionPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type CreateSelectionPayload struct {
	Shape     SelectionShape       `json:"shape"`
	Mode      SelectionCombineMode `json:"mode"`
	Rect      LayerBounds          `json:"rect"`
	Polygon   []SelectionPoint     `json:"polygon,omitempty"`
	AntiAlias bool                 `json:"antiAlias,omitempty"`
}

type FeatherSelectionPayload struct {
	Radius float64 `json:"radius"`
}

type ExpandSelectionPayload struct {
	Pixels int `json:"pixels"`
}

type ContractSelectionPayload struct {
	Pixels int `json:"pixels"`
}

type SmoothSelectionPayload struct {
	Radius int `json:"radius"`
}

type BorderSelectionPayload struct {
	Width int `json:"width"`
}

type TransformSelectionPayload struct {
	A  float64 `json:"a"`
	B  float64 `json:"b"`
	C  float64 `json:"c"`
	D  float64 `json:"d"`
	TX float64 `json:"tx"`
	TY float64 `json:"ty"`
}

type SelectColorRangePayload struct {
	LayerID      string               `json:"layerId"`
	TargetColor  [4]uint8             `json:"targetColor"`
	Fuzziness    float64              `json:"fuzziness"`
	SampleMerged bool                 `json:"sampleMerged"`
	Mode         SelectionCombineMode `json:"mode"`
}

type QuickSelectPayload struct {
	X               int                  `json:"x"`
	Y               int                  `json:"y"`
	Tolerance       float64              `json:"tolerance"`
	EdgeSensitivity float64              `json:"edgeSensitivity"`
	LayerID         string               `json:"layerId"`
	SampleMerged    bool                 `json:"sampleMerged"`
	Mode            SelectionCombineMode `json:"mode"`
}

type MagicWandPayload struct {
	X            int                  `json:"x"`
	Y            int                  `json:"y"`
	Tolerance    float64              `json:"tolerance"`
	LayerID      string               `json:"layerId"`
	SampleMerged bool                 `json:"sampleMerged"`
	Contiguous   bool                 `json:"contiguous"`
	AntiAlias    bool                 `json:"antiAlias"`
	Mode         SelectionCombineMode `json:"mode"`
}

type SaveSelectionToChannelPayload struct {
	Name string `json:"name"`
}

type LoadSelectionFromChannelPayload struct {
	Name string               `json:"name"`
	Mode SelectionCombineMode `json:"mode"`
}

type RefineSelectionPayload struct {
	SmartRadius  float64 `json:"smartRadius,omitempty"`
	Contrast     float64 `json:"contrast,omitempty"`
	LayerID      string  `json:"layerId,omitempty"`
	SampleMerged bool    `json:"sampleMerged,omitempty"`
}

type SelectionViewMode string

const (
	SelectionViewModeOnionSkin    SelectionViewMode = "onion-skin"
	SelectionViewModeMarchingAnts SelectionViewMode = "marching-ants"
	SelectionViewModeOverlay      SelectionViewMode = "overlay"
	SelectionViewModeBlackWhite   SelectionViewMode = "black-white"
	SelectionViewModeBlack        SelectionViewMode = "black"
	SelectionViewModeWhite        SelectionViewMode = "white"
	SelectionViewModeLayer        SelectionViewMode = "layer"
)

type SetSelectionViewModePayload struct {
	Mode SelectionViewMode `json:"mode"`
}

type OutputSelectionMode string

const (
	OutputSelectionSelection        OutputSelectionMode = "selection"
	OutputSelectionLayerMask        OutputSelectionMode = "layer-mask"
	OutputSelectionNewLayer         OutputSelectionMode = "new-layer"
	OutputSelectionNewLayerWithMask OutputSelectionMode = "new-layer-with-mask"
	OutputSelectionDocument         OutputSelectionMode = "document"
)

type OutputSelectionPayload struct {
	Mode         OutputSelectionMode `json:"mode"`
	LayerID      string              `json:"layerId,omitempty"`
	Name         string              `json:"name,omitempty"`
	SampleMerged bool                `json:"sampleMerged,omitempty"`
}

func (doc *Document) CreateSelection(shape SelectionShape, rect LayerBounds, polygon []SelectionPoint, mode SelectionCombineMode, antiAlias bool) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	var next *Selection
	switch shape {
	case SelectionShapeRect:
		next = newRectSelection(doc.Width, doc.Height, rect)
	case SelectionShapeEllipse:
		next = newEllipseSelection(doc.Width, doc.Height, rect, antiAlias)
	case SelectionShapePolygon:
		if len(polygon) < 3 {
			return fmt.Errorf("polygon selection requires at least 3 points")
		}
		next = newPolygonSelection(doc.Width, doc.Height, polygon, antiAlias)
	default:
		return fmt.Errorf("unsupported selection shape %q", shape)
	}
	doc.Selection = combineSelection(doc.Selection, next, mode)
	return nil
}

func (doc *Document) FeatherSelection(radius float64) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	doc.Selection = normalizeSelection(featherSelection(selection, radius))
	return nil
}

func (doc *Document) ExpandSelection(pixels int) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	doc.Selection = normalizeSelection(&Selection{Width: selection.Width, Height: selection.Height, Mask: dilateMask(selection.Mask, selection.Width, selection.Height, pixels)})
	return nil
}

func (doc *Document) ContractSelection(pixels int) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	doc.Selection = normalizeSelection(&Selection{Width: selection.Width, Height: selection.Height, Mask: erodeMask(selection.Mask, selection.Width, selection.Height, pixels)})
	return nil
}

func (doc *Document) SmoothSelection(radius int) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	doc.Selection = normalizeSelection(&Selection{Width: selection.Width, Height: selection.Height, Mask: smoothMask(selection.Mask, selection.Width, selection.Height, radius)})
	return nil
}

func (doc *Document) BorderSelection(width int) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	doc.Selection = normalizeSelection(&Selection{Width: selection.Width, Height: selection.Height, Mask: borderMask(selection.Mask, selection.Width, selection.Height, width)})
	return nil
}

func (doc *Document) RefineSelectionEdges(smartRadius, contrast float64, layerID string, sampleMerged bool) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	refined := cloneSelection(selection)
	if smartRadius > 0 {
		surface, err := doc.selectionSourceSurface(layerID, sampleMerged)
		if err != nil {
			return err
		}
		refined = smartRefineSelection(refined, smartRadius, surface, doc.Width, doc.Height)
	}
	if contrast != 0 {
		refined = applySelectionContrast(refined, contrast)
	}
	doc.Selection = normalizeSelection(refined)
	return nil
}

func (doc *Document) TransformSelection(a, b, c, d, tx, ty float64) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	transformed, err := transformSelection(selection, a, b, c, d, tx, ty)
	if err != nil {
		return err
	}
	doc.Selection = normalizeSelection(transformed)
	return nil
}

func (doc *Document) SelectColorRange(layerID string, targetColor [4]uint8, fuzziness float64, sampleMerged bool, mode SelectionCombineMode) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	surface, err := doc.selectionSourceSurface(layerID, sampleMerged)
	if err != nil {
		return err
	}
	doc.Selection = combineSelection(doc.Selection, selectColorRange(surface, doc.Width, doc.Height, targetColor, fuzziness), mode)
	return nil
}

func (doc *Document) QuickSelect(x, y int, tolerance, edgeSensitivity float64, layerID string, sampleMerged bool, mode SelectionCombineMode) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	surface, err := doc.selectionSourceSurface(layerID, sampleMerged)
	if err != nil {
		return err
	}
	doc.Selection = combineSelection(doc.Selection, quickSelect(surface, doc.Width, doc.Height, x, y, tolerance, edgeSensitivity), mode)
	return nil
}

func (doc *Document) MagicWand(x, y int, tolerance float64, layerID string, sampleMerged, contiguous, antiAlias bool, mode SelectionCombineMode) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	surface, err := doc.selectionSourceSurface(layerID, sampleMerged)
	if err != nil {
		return err
	}
	if x < 0 || x >= doc.Width || y < 0 || y >= doc.Height {
		doc.Selection = combineSelection(doc.Selection, newSelection(doc.Width, doc.Height), mode)
		return nil
	}
	targetColor, ok := sampleSurfaceColor(surface, doc.Width, doc.Height, x, y)
	if !ok {
		doc.Selection = combineSelection(doc.Selection, newSelection(doc.Width, doc.Height), mode)
		return nil
	}
	var next *Selection
	if contiguous {
		next = magicWandFloodFill(surface, doc.Width, doc.Height, x, y, tolerance)
	} else {
		next = selectColorRange(surface, doc.Width, doc.Height, targetColor, tolerance)
	}
	if antiAlias && next != nil {
		next = &Selection{
			Width:  next.Width,
			Height: next.Height,
			Mask:   antiAliasSelectionMask(next.Mask, next.Width, next.Height),
		}
	}
	doc.Selection = combineSelection(doc.Selection, next, mode)
	return nil
}

func (doc *Document) selectionSourceSurface(layerID string, sampleMerged bool) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	if sampleMerged {
		return doc.renderCompositeSurface(), nil
	}
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	if layerID == "" {
		return doc.renderCompositeSurface(), nil
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !ok {
		return nil, fmt.Errorf("layer %q not found", layerID)
	}
	return doc.renderLayerToSurface(layer)
}

func sampleSurfaceColor(surface []byte, width, height, x, y int) ([4]uint8, bool) {
	if x < 0 || x >= width || y < 0 || y >= height || len(surface) < width*height*4 {
		return [4]uint8{}, false
	}
	index := (y*width + x) * 4
	if index < 0 || index+3 >= len(surface) {
		return [4]uint8{}, false
	}
	return [4]uint8{surface[index], surface[index+1], surface[index+2], surface[index+3]}, true
}

func newRectSelection(width, height int, rect LayerBounds) *Selection {
	rect = normalizeSelectionRect(rect)
	selection := newSelection(width, height)
	if rect.W <= 0 || rect.H <= 0 {
		return selection
	}
	minX := clampInt(rect.X, 0, width)
	maxX := clampInt(rect.X+rect.W, 0, width)
	minY := clampInt(rect.Y, 0, height)
	maxY := clampInt(rect.Y+rect.H, 0, height)
	for y := minY; y < maxY; y++ {
		rowOffset := y * width
		for x := minX; x < maxX; x++ {
			selection.Mask[rowOffset+x] = 255
		}
	}
	return selection
}

func newEllipseSelection(width, height int, rect LayerBounds, antiAlias bool) *Selection {
	rect = normalizeSelectionRect(rect)
	selection := newSelection(width, height)
	if rect.W <= 0 || rect.H <= 0 {
		return selection
	}
	cx := float64(rect.X) + float64(rect.W)*0.5
	cy := float64(rect.Y) + float64(rect.H)*0.5
	rx := float64(rect.W) * 0.5
	ry := float64(rect.H) * 0.5
	if rx <= 0 || ry <= 0 {
		return selection
	}
	minX := clampInt(rect.X, 0, width)
	maxX := clampInt(rect.X+rect.W, 0, width)
	minY := clampInt(rect.Y, 0, height)
	maxY := clampInt(rect.Y+rect.H, 0, height)
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			coverage := sampledCoverage(antiAlias, func(sampleX, sampleY float64) bool {
				dx := (sampleX - cx) / rx
				dy := (sampleY - cy) / ry
				return dx*dx+dy*dy <= 1
			}, x, y)
			selection.Mask[y*width+x] = coverage
		}
	}
	return selection
}

func newPolygonSelection(width, height int, points []SelectionPoint, antiAlias bool) *Selection {
	selection := newSelection(width, height)
	if len(points) < 3 {
		return selection
	}
	minX := math.Inf(1)
	minY := math.Inf(1)
	maxX := math.Inf(-1)
	maxY := math.Inf(-1)
	for _, point := range points {
		if point.X < minX {
			minX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}
	startX := clampInt(int(math.Floor(minX)), 0, width)
	endX := clampInt(int(math.Ceil(maxX)), 0, width)
	startY := clampInt(int(math.Floor(minY)), 0, height)
	endY := clampInt(int(math.Ceil(maxY)), 0, height)
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			coverage := sampledCoverage(antiAlias, func(sampleX, sampleY float64) bool {
				return pointInPolygon(points, sampleX, sampleY)
			}, x, y)
			selection.Mask[y*width+x] = coverage
		}
	}
	return selection
}

func combineSelection(current, next *Selection, mode SelectionCombineMode) *Selection {
	next = normalizeSelection(cloneSelection(next))
	current = normalizeSelection(cloneSelection(current))
	switch mode {
	case "", SelectionCombineReplace:
		return next
	case SelectionCombineAdd:
		if current == nil {
			return next
		}
		if next == nil {
			return current
		}
	case SelectionCombineSubtract:
		if current == nil {
			return nil
		}
		if next == nil {
			return current
		}
	case SelectionCombineIntersect:
		if current == nil || next == nil {
			return nil
		}
	default:
		return combineSelection(current, next, SelectionCombineReplace)
	}
	if current.Width != next.Width || current.Height != next.Height {
		return next
	}
	combined := newSelection(current.Width, current.Height)
	for index := range combined.Mask {
		currentAlpha := current.Mask[index]
		nextAlpha := next.Mask[index]
		switch mode {
		case SelectionCombineAdd:
			if nextAlpha > currentAlpha {
				combined.Mask[index] = nextAlpha
			} else {
				combined.Mask[index] = currentAlpha
			}
		case SelectionCombineSubtract:
			combined.Mask[index] = scaleMaskedAlpha(currentAlpha, 255-nextAlpha)
		case SelectionCombineIntersect:
			if nextAlpha < currentAlpha {
				combined.Mask[index] = nextAlpha
			} else {
				combined.Mask[index] = currentAlpha
			}
		}
	}
	return normalizeSelection(combined)
}

func smartRefineSelection(selection *Selection, radius float64, surface []byte, width, height int) *Selection {
	if selection == nil || radius <= 0 {
		return cloneSelection(selection)
	}
	edges := &Selection{
		Width:  selection.Width,
		Height: selection.Height,
		Mask:   edgeMask(selection.Mask, selection.Width, selection.Height),
	}
	edgeInfluence := featherSelection(edges, radius)
	softened := featherSelection(selection, radius)
	refined := cloneSelection(selection)
	for i := range refined.Mask {
		influence := float64(edgeInfluence.Mask[i]) / 255
		if influence <= 0 {
			continue
		}
		gradient := localSurfaceEdgeStrength(surface, width, height, i%selection.Width, i/selection.Width)
		adapt := influence * (1 - gradient)
		if adapt <= 0 {
			continue
		}
		base := float64(selection.Mask[i])
		soft := float64(softened.Mask[i])
		refined.Mask[i] = byte(math.Round(clampFloat(base+(soft-base)*adapt, 0, 255)))
	}
	return refined
}

func applySelectionContrast(selection *Selection, contrast float64) *Selection {
	if selection == nil || contrast == 0 {
		return cloneSelection(selection)
	}
	factor := 1 + clampFloat(contrast, -100, 100)/100
	refined := cloneSelection(selection)
	for i, alpha := range refined.Mask {
		normalized := float64(alpha) / 255
		value := (normalized-0.5)*factor + 0.5
		refined.Mask[i] = byte(math.Round(clampFloat(value*255, 0, 255)))
	}
	return refined
}

func localSurfaceEdgeStrength(surface []byte, width, height, x, y int) float64 {
	if len(surface) < width*height*4 || x < 0 || y < 0 || x >= width || y >= height {
		return 0
	}
	center := localSurfaceLuminance(surface[(y*width+x)*4:])
	maxDelta := 0.0
	for _, delta := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		sx := x + delta[0]
		sy := y + delta[1]
		if sx < 0 || sy < 0 || sx >= width || sy >= height {
			continue
		}
		neighbor := localSurfaceLuminance(surface[(sy*width+sx)*4:])
		diff := math.Abs(center - neighbor)
		if diff > maxDelta {
			maxDelta = diff
		}
	}
	return clampFloat(maxDelta/255, 0, 1)
}

func localSurfaceLuminance(px []byte) float64 {
	if len(px) < 3 {
		return 0
	}
	return 0.299*float64(px[0]) + 0.587*float64(px[1]) + 0.114*float64(px[2])
}

func selectionBounds(selection *Selection) (LayerBounds, bool) {
	bounds, ok := selection.Bounds()
	if !ok {
		return LayerBounds{}, false
	}
	return LayerBounds(bounds), true
}

func extractSelectionFromSurface(surface []byte, width, height int, selection *Selection) ([]byte, LayerBounds, bool) {
	selection = normalizeSelection(cloneSelection(selection))
	if selection == nil || width <= 0 || height <= 0 || len(surface) < width*height*4 {
		return nil, LayerBounds{}, false
	}
	bounds, ok := selectionBounds(selection)
	if !ok {
		return nil, LayerBounds{}, false
	}
	pixels := make([]byte, bounds.W*bounds.H*4)
	for y := 0; y < bounds.H; y++ {
		for x := 0; x < bounds.W; x++ {
			docX := bounds.X + x
			docY := bounds.Y + y
			if docX < 0 || docY < 0 || docX >= width || docY >= height {
				continue
			}
			srcIndex := (docY*width + docX) * 4
			dstIndex := (y*bounds.W + x) * 4
			alpha := selection.Mask[docY*selection.Width+docX]
			if alpha == 0 {
				continue
			}
			copy(pixels[dstIndex:dstIndex+4], surface[srcIndex:srcIndex+4])
			pixels[dstIndex+3] = scaleMaskedAlpha(surface[srcIndex+3], alpha)
		}
	}
	return pixels, bounds, true
}

func cropSurfaceBounds(surface []byte, width, height int, bounds LayerBounds) []byte {
	if bounds.W <= 0 || bounds.H <= 0 || len(surface) < width*height*4 {
		return nil
	}
	cropped := make([]byte, bounds.W*bounds.H*4)
	for y := 0; y < bounds.H; y++ {
		for x := 0; x < bounds.W; x++ {
			docX := bounds.X + x
			docY := bounds.Y + y
			if docX < 0 || docY < 0 || docX >= width || docY >= height {
				continue
			}
			srcIndex := (docY*width + docX) * 4
			dstIndex := (y*bounds.W + x) * 4
			copy(cropped[dstIndex:dstIndex+4], surface[srcIndex:srcIndex+4])
		}
	}
	return cropped
}

func featherSelection(selection *Selection, radius float64) *Selection {
	if selection == nil || radius <= 0 {
		return cloneSelection(selection)
	}
	kernelRadius := maxInt(int(math.Ceil(radius*2)), 1)
	sigma := math.Max(radius/2, 0.5)
	kernel := make([]float64, kernelRadius*2+1)
	sum := 0.0
	for index := -kernelRadius; index <= kernelRadius; index++ {
		value := math.Exp(-float64(index*index) / (2 * sigma * sigma))
		kernel[index+kernelRadius] = value
		sum += value
	}
	for index := range kernel {
		kernel[index] /= sum
	}
	horizontal := make([]float64, selection.Width*selection.Height)
	for y := range selection.Height {
		rowOffset := y * selection.Width
		for x := range selection.Width {
			value := 0.0
			for kernelIndex := -kernelRadius; kernelIndex <= kernelRadius; kernelIndex++ {
				sampleX := clampInt(x+kernelIndex, 0, selection.Width-1)
				value += kernel[kernelIndex+kernelRadius] * float64(selection.Mask[rowOffset+sampleX])
			}
			horizontal[rowOffset+x] = value
		}
	}
	blurred := newSelection(selection.Width, selection.Height)
	for y := range selection.Height {
		for x := range selection.Width {
			value := 0.0
			for kernelIndex := -kernelRadius; kernelIndex <= kernelRadius; kernelIndex++ {
				sampleY := clampInt(y+kernelIndex, 0, selection.Height-1)
				value += kernel[kernelIndex+kernelRadius] * horizontal[sampleY*selection.Width+x]
			}
			blurred.Mask[y*selection.Width+x] = uint8(math.Round(clampFloat(value, 0, 255)))
		}
	}
	return blurred
}

func dilateMask(mask []byte, width, height, radius int) []byte {
	if radius <= 0 {
		return append([]byte(nil), mask...)
	}
	dilated := make([]byte, len(mask))
	radiusSquared := radius * radius
	for y := range height {
		for x := range width {
			maxAlpha := byte(0)
			for sampleY := maxInt(y-radius, 0); sampleY <= minInt(y+radius, height-1); sampleY++ {
				for sampleX := maxInt(x-radius, 0); sampleX <= minInt(x+radius, width-1); sampleX++ {
					dx := sampleX - x
					dy := sampleY - y
					if dx*dx+dy*dy > radiusSquared {
						continue
					}
					alpha := mask[sampleY*width+sampleX]
					if alpha > maxAlpha {
						maxAlpha = alpha
						if maxAlpha == 255 {
							break
						}
					}
				}
				if maxAlpha == 255 {
					break
				}
			}
			dilated[y*width+x] = maxAlpha
		}
	}
	return dilated
}

func erodeMask(mask []byte, width, height, radius int) []byte {
	if radius <= 0 {
		return append([]byte(nil), mask...)
	}
	eroded := make([]byte, len(mask))
	radiusSquared := radius * radius
	for y := range height {
		for x := range width {
			minAlpha := byte(255)
			for sampleY := maxInt(y-radius, 0); sampleY <= minInt(y+radius, height-1); sampleY++ {
				for sampleX := maxInt(x-radius, 0); sampleX <= minInt(x+radius, width-1); sampleX++ {
					dx := sampleX - x
					dy := sampleY - y
					if dx*dx+dy*dy > radiusSquared {
						continue
					}
					alpha := mask[sampleY*width+sampleX]
					if alpha < minAlpha {
						minAlpha = alpha
						if minAlpha == 0 {
							break
						}
					}
				}
				if minAlpha == 0 {
					break
				}
			}
			eroded[y*width+x] = minAlpha
		}
	}
	return eroded
}

func smoothMask(mask []byte, width, height, radius int) []byte {
	if radius <= 0 {
		return append([]byte(nil), mask...)
	}
	smoothed := make([]byte, len(mask))
	for y := range height {
		for x := range width {
			minAlpha := byte(255)
			maxAlpha := byte(0)
			sum := 0
			count := 0
			for sampleY := maxInt(y-radius, 0); sampleY <= minInt(y+radius, height-1); sampleY++ {
				for sampleX := maxInt(x-radius, 0); sampleX <= minInt(x+radius, width-1); sampleX++ {
					alpha := mask[sampleY*width+sampleX]
					if alpha < minAlpha {
						minAlpha = alpha
					}
					if alpha > maxAlpha {
						maxAlpha = alpha
					}
					sum += int(alpha)
					count++
				}
			}
			if minAlpha == maxAlpha {
				smoothed[y*width+x] = mask[y*width+x]
				continue
			}
			smoothed[y*width+x] = byte(sum / maxInt(count, 1))
		}
	}
	return smoothed
}

func borderMask(mask []byte, width, height, borderWidth int) []byte {
	if borderWidth <= 1 {
		return edgeMask(mask, width, height)
	}
	radius := maxInt(borderWidth/2, 1)
	outer := dilateMask(mask, width, height, radius)
	inner := erodeMask(mask, width, height, radius)
	border := make([]byte, len(mask))
	for index := range border {
		border[index] = scaleMaskedAlpha(outer[index], 255-inner[index])
	}
	return border
}

func edgeMask(mask []byte, width, height int) []byte {
	edges := make([]byte, len(mask))
	for y := range height {
		for x := range width {
			alpha := mask[y*width+x]
			if alpha == 0 {
				continue
			}
			if x == 0 || x == width-1 || y == 0 || y == height-1 {
				edges[y*width+x] = alpha
				continue
			}
			if mask[y*width+x-1] == 0 || mask[y*width+x+1] == 0 || mask[(y-1)*width+x] == 0 || mask[(y+1)*width+x] == 0 {
				edges[y*width+x] = alpha
			}
		}
	}
	return edges
}

func transformSelection(selection *Selection, a, b, c, d, tx, ty float64) (*Selection, error) {
	if selection == nil {
		return nil, fmt.Errorf("selection is required")
	}
	determinant := a*d - b*c
	if math.Abs(determinant) < 1e-8 {
		return nil, fmt.Errorf("selection transform is singular")
	}
	bounds, ok := selection.Bounds()
	if !ok {
		return cloneSelection(selection), nil
	}
	invA := d / determinant
	invB := -b / determinant
	invC := -c / determinant
	invD := a / determinant
	corners := [4]SelectionPoint{
		{X: float64(bounds.X), Y: float64(bounds.Y)},
		{X: float64(bounds.X + bounds.W), Y: float64(bounds.Y)},
		{X: float64(bounds.X + bounds.W), Y: float64(bounds.Y + bounds.H)},
		{X: float64(bounds.X), Y: float64(bounds.Y + bounds.H)},
	}
	minX := math.Inf(1)
	minY := math.Inf(1)
	maxX := math.Inf(-1)
	maxY := math.Inf(-1)
	for _, corner := range corners {
		x := a*corner.X + c*corner.Y + tx
		y := b*corner.X + d*corner.Y + ty
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	startX := clampInt(int(math.Floor(minX)), 0, selection.Width)
	endX := clampInt(int(math.Ceil(maxX)), 0, selection.Width)
	startY := clampInt(int(math.Floor(minY)), 0, selection.Height)
	endY := clampInt(int(math.Ceil(maxY)), 0, selection.Height)
	transformed := newSelection(selection.Width, selection.Height)
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			destX := float64(x) + 0.5 - tx
			destY := float64(y) + 0.5 - ty
			sourceX := invA*destX + invC*destY
			sourceY := invB*destX + invD*destY
			transformed.Mask[y*selection.Width+x] = bilinearSelectionSample(selection, sourceX, sourceY)
		}
	}
	return transformed, nil
}

func selectColorRange(surface []byte, width, height int, targetColor [4]uint8, fuzziness float64) *Selection {
	selection := newSelection(width, height)
	if len(surface) < width*height*4 {
		return selection
	}
	threshold := clampFloat(fuzziness, 0, 442)
	for y := range height {
		for x := range width {
			index := (y*width + x) * 4
			alpha := surface[index+3]
			if alpha == 0 {
				continue
			}
			distance := colorDistance(surface[index:index+4], targetColor)
			var coverage float64
			switch {
			case threshold == 0 && distance == 0:
				coverage = 255
			case threshold == 0:
				coverage = 0
			case distance <= threshold:
				coverage = 255 * (1 - distance/threshold)
			default:
				coverage = 0
			}
			selection.Mask[y*width+x] = scaleMaskedAlpha(alpha, uint8(math.Round(clampFloat(coverage, 0, 255))))
		}
	}
	return selection
}

func quickSelect(surface []byte, width, height, seedX, seedY int, tolerance, edgeSensitivity float64) *Selection {
	selection := newSelection(width, height)
	if len(surface) < width*height*4 || seedX < 0 || seedX >= width || seedY < 0 || seedY >= height {
		return selection
	}
	seedIndex := (seedY*width + seedX) * 4
	seedColor := [4]uint8{surface[seedIndex], surface[seedIndex+1], surface[seedIndex+2], surface[seedIndex+3]}
	if seedColor[3] == 0 {
		return selection
	}
	colorThreshold := clampFloat(tolerance, 0, 442)
	edgeThreshold := clampFloat(edgeSensitivity, 0, 442)
	if edgeThreshold == 0 {
		edgeThreshold = colorThreshold
	}
	visited := make([]bool, width*height)
	seed := seedY*width + seedX
	queue := []int{seed}
	visited[seed] = true
	directions := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		currentX := current % width
		currentY := current / width
		currentIndex := current * 4
		currentColor := [4]uint8{surface[currentIndex], surface[currentIndex+1], surface[currentIndex+2], surface[currentIndex+3]}
		if currentColor[3] == 0 || colorDistance(currentColor[:], seedColor) > colorThreshold {
			continue
		}
		selection.Mask[current] = 255
		for _, direction := range directions {
			nextX := currentX + direction[0]
			nextY := currentY + direction[1]
			if nextX < 0 || nextX >= width || nextY < 0 || nextY >= height {
				continue
			}
			next := nextY*width + nextX
			if visited[next] {
				continue
			}
			visited[next] = true
			nextIndex := next * 4
			nextColor := [4]uint8{surface[nextIndex], surface[nextIndex+1], surface[nextIndex+2], surface[nextIndex+3]}
			if nextColor[3] == 0 {
				continue
			}
			if colorDistance(nextColor[:], seedColor) > colorThreshold {
				continue
			}
			if colorDistance(nextColor[:], currentColor) > edgeThreshold {
				continue
			}
			queue = append(queue, next)
		}
	}
	return selection
}

// magicWandFloodFill selects all 4-connected pixels within tolerance of the
// seed color. Unlike quickSelect, it performs no edge detection — every visited
// pixel is accepted or rejected based solely on its distance to the seed color.
func magicWandFloodFill(surface []byte, width, height, seedX, seedY int, tolerance float64) *Selection {
	selection := newSelection(width, height)
	if len(surface) < width*height*4 || seedX < 0 || seedX >= width || seedY < 0 || seedY >= height {
		return selection
	}
	seedIndex := (seedY*width + seedX) * 4
	seedColor := [4]uint8{surface[seedIndex], surface[seedIndex+1], surface[seedIndex+2], surface[seedIndex+3]}
	if seedColor[3] == 0 {
		return selection
	}
	colorThreshold := clampFloat(tolerance, 0, 442)
	visited := make([]bool, width*height)
	seed := seedY*width + seedX
	queue := []int{seed}
	visited[seed] = true
	selection.Mask[seed] = 255
	directions := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		currentX := current % width
		currentY := current / width
		for _, dir := range directions {
			nextX := currentX + dir[0]
			nextY := currentY + dir[1]
			if nextX < 0 || nextX >= width || nextY < 0 || nextY >= height {
				continue
			}
			next := nextY*width + nextX
			if visited[next] {
				continue
			}
			visited[next] = true
			nextIndex := next * 4
			nextColor := [4]uint8{surface[nextIndex], surface[nextIndex+1], surface[nextIndex+2], surface[nextIndex+3]}
			if nextColor[3] == 0 || colorDistance(nextColor[:], seedColor) > colorThreshold {
				continue
			}
			selection.Mask[next] = 255
			queue = append(queue, next)
		}
	}
	return selection
}

// antiAliasSelectionMask softens only the boundary pixels of a hard selection
// mask. Interior pixels (all selected 4-connected neighbours) are left at 255.
// Exterior pixels (no selected neighbours) stay at 0. Only boundary pixels —
// those selected but adjacent to at least one unselected pixel — are replaced
// with a coverage value computed from their 8-connected neighbourhood.
func antiAliasSelectionMask(mask []byte, width, height int) []byte {
	result := append([]byte(nil), mask...)
	for y := range height {
		for x := range width {
			idx := y*width + x
			if mask[idx] == 0 {
				continue
			}
			isBoundary := (x > 0 && mask[idx-1] == 0) ||
				(x < width-1 && mask[idx+1] == 0) ||
				(y > 0 && mask[idx-width] == 0) ||
				(y < height-1 && mask[idx+width] == 0)
			if !isBoundary {
				continue
			}
			selected := 0
			total := 0
			for dy := -1; dy <= 1; dy++ {
				ny := y + dy
				if ny < 0 || ny >= height {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					nx := x + dx
					if nx < 0 || nx >= width {
						continue
					}
					total++
					if mask[ny*width+nx] != 0 {
						selected++
					}
				}
			}
			if total > 0 {
				result[idx] = uint8(selected * 255 / total)
			}
		}
	}
	return result
}
