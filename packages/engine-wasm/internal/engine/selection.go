package engine

import (
	"bytes"
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

type Selection struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Mask   []byte `json:"mask,omitempty"`
}

type SelectionMeta struct {
	Active                 bool       `json:"active"`
	Bounds                 *DirtyRect `json:"bounds,omitempty"`
	PixelCount             int        `json:"pixelCount"`
	LastSelectionAvailable bool       `json:"lastSelectionAvailable"`
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

func cloneSelection(selection *Selection) *Selection {
	if selection == nil {
		return nil
	}
	cloned := *selection
	cloned.Mask = append([]byte(nil), selection.Mask...)
	return &cloned
}

func selectionEqual(a, b *Selection) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Width == b.Width && a.Height == b.Height && bytes.Equal(a.Mask, b.Mask)
}

func (selection *Selection) bounds() (DirtyRect, bool) {
	if selection == nil || selection.Width <= 0 || selection.Height <= 0 || len(selection.Mask) < selection.Width*selection.Height {
		return DirtyRect{}, false
	}
	minX := selection.Width
	minY := selection.Height
	maxX := -1
	maxY := -1
	for y := range selection.Height {
		rowOffset := y * selection.Width
		for x := range selection.Width {
			if selection.Mask[rowOffset+x] == 0 {
				continue
			}
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
	}
	if maxX < minX || maxY < minY {
		return DirtyRect{}, false
	}
	return DirtyRect{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1}, true
}

func (selection *Selection) pixelCount() int {
	if selection == nil {
		return 0
	}
	count := 0
	for _, alpha := range selection.Mask {
		if alpha != 0 {
			count++
		}
	}
	return count
}

func normalizeSelection(selection *Selection) *Selection {
	if selection == nil || selection.Width <= 0 || selection.Height <= 0 {
		return nil
	}
	expectedLen := selection.Width * selection.Height
	if len(selection.Mask) < expectedLen {
		return nil
	}
	selection.Mask = selection.Mask[:expectedLen]
	for _, alpha := range selection.Mask {
		if alpha != 0 {
			return selection
		}
	}
	return nil
}

func newSelection(width, height int) *Selection {
	if width <= 0 || height <= 0 {
		return &Selection{Width: width, Height: height}
	}
	return &Selection{Width: width, Height: height, Mask: make([]byte, width*height)}
}

func newLayerMaskFromSelection(selection *Selection) *LayerMask {
	if selection == nil {
		return nil
	}
	return &LayerMask{
		Enabled: true,
		Width:   selection.Width,
		Height:  selection.Height,
		Data:    append([]byte(nil), selection.Mask...),
	}
}

func (doc *Document) selectionMeta() SelectionMeta {
	meta := SelectionMeta{}
	if doc == nil {
		return meta
	}
	meta.LastSelectionAvailable = normalizeSelection(cloneSelection(doc.LastSelection)) != nil
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return meta
	}
	meta.Active = true
	meta.PixelCount = selection.pixelCount()
	if bounds, ok := selection.bounds(); ok {
		meta.Bounds = &bounds
	}
	return meta
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

func (doc *Document) SelectAll() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	selection := newSelection(doc.Width, doc.Height)
	for index := range selection.Mask {
		selection.Mask[index] = 255
	}
	doc.Selection = selection
	return nil
}

func (doc *Document) Deselect() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if selection := normalizeSelection(cloneSelection(doc.Selection)); selection != nil {
		doc.LastSelection = selection
	}
	doc.Selection = nil
	return nil
}

func (doc *Document) Reselect() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	selection := normalizeSelection(cloneSelection(doc.LastSelection))
	if selection == nil {
		return fmt.Errorf("no stored selection")
	}
	doc.Selection = selection
	return nil
}

func (doc *Document) InvertSelection() error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return doc.SelectAll()
	}
	for index := range selection.Mask {
		selection.Mask[index] = 255 - selection.Mask[index]
	}
	doc.Selection = normalizeSelection(selection)
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
		next = quickSelect(surface, doc.Width, doc.Height, x, y, tolerance, tolerance)
	} else {
		next = selectColorRange(surface, doc.Width, doc.Height, targetColor, tolerance)
	}
	if antiAlias {
		next = normalizeSelection(&Selection{
			Width:  next.Width,
			Height: next.Height,
			Mask:   smoothMask(next.Mask, next.Width, next.Height, 1),
		})
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

func featherSelection(selection *Selection, radius float64) *Selection {
	if selection == nil || radius <= 0 {
		return cloneSelection(selection)
	}
	kernelRadius := maxInt(int(math.Ceil(radius*2)), 1)
	sigma := math.Max(radius/2, 0.5)
	kernel := make([]float64, kernelRadius*2+1)
	sum := 0.0
	for index := -kernelRadius; index <= kernelRadius; index++ {
		value := math.Exp(-(float64(index * index)) / (2 * sigma * sigma))
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
	bounds, ok := selection.bounds()
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
	queue := []int{seedY*width + seedX}
	visited[seedY*width+seedX] = true
	directions := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
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

func RenderSelectionOverlay(doc *Document, vp *ViewportState, reuse []byte, selection *Selection, animationFrame int64) []byte {
	if doc == nil || vp == nil || selection == nil || len(reuse) == 0 {
		return reuse
	}
	bounds, ok := selection.bounds()
	if !ok {
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
	phase := int(animationFrame/2) & 7
	for y := bounds.Y; y < bounds.Y+bounds.H; y++ {
		for x := bounds.X; x < bounds.X+bounds.W; x++ {
			if !selectionEdgeAt(selection, x, y) {
				continue
			}
			docX := float64(x) + 0.5 - vp.CenterX
			docY := float64(y) + 0.5 - vp.CenterY
			screenX := docX*cosTheta*zoom - docY*sinTheta*zoom + halfCanvasW
			screenY := docX*sinTheta*zoom + docY*cosTheta*zoom + halfCanvasH
			canvasX := int(math.Floor(screenX))
			canvasY := int(math.Floor(screenY))
			if canvasX < 0 || canvasX >= canvasW || canvasY < 0 || canvasY >= canvasH {
				continue
			}
			pattern := (x + y + phase) & 7
			color := byte(0)
			if pattern >= 4 {
				color = 255
			}
			index := (canvasY*canvasW + canvasX) * 4
			reuse[index] = color
			reuse[index+1] = color
			reuse[index+2] = color
			reuse[index+3] = 255
		}
	}
	return reuse
}

func selectionEdgeAt(selection *Selection, x, y int) bool {
	if selection == nil || x < 0 || x >= selection.Width || y < 0 || y >= selection.Height {
		return false
	}
	if selection.Mask[y*selection.Width+x] == 0 {
		return false
	}
	if x == 0 || x == selection.Width-1 || y == 0 || y == selection.Height-1 {
		return true
	}
	return selection.Mask[y*selection.Width+x-1] == 0 ||
		selection.Mask[y*selection.Width+x+1] == 0 ||
		selection.Mask[(y-1)*selection.Width+x] == 0 ||
		selection.Mask[(y+1)*selection.Width+x] == 0
}

func bilinearSelectionSample(selection *Selection, x, y float64) byte {
	fx := x - 0.5
	fy := y - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	a00 := float64(selectionAlphaAt(selection, x0, y0))
	a10 := float64(selectionAlphaAt(selection, x0+1, y0))
	a01 := float64(selectionAlphaAt(selection, x0, y0+1))
	a11 := float64(selectionAlphaAt(selection, x0+1, y0+1))
	top := a00*(1-tx) + a10*tx
	bottom := a01*(1-tx) + a11*tx
	return uint8(math.Round(clampFloat(top*(1-ty)+bottom*ty, 0, 255)))
}

func selectionAlphaAt(selection *Selection, x, y int) byte {
	if selection == nil || x < 0 || y < 0 || x >= selection.Width || y >= selection.Height {
		return 0
	}
	return selection.Mask[y*selection.Width+x]
}

func sampledCoverage(antiAlias bool, inside func(sampleX, sampleY float64) bool, pixelX, pixelY int) byte {
	if !antiAlias {
		if inside(float64(pixelX)+0.5, float64(pixelY)+0.5) {
			return 255
		}
		return 0
	}
	covered := 0
	sampleOffsets := [4]float64{0.125, 0.375, 0.625, 0.875}
	for _, sampleY := range sampleOffsets {
		for _, sampleX := range sampleOffsets {
			if inside(float64(pixelX)+sampleX, float64(pixelY)+sampleY) {
				covered++
			}
		}
	}
	return uint8(math.Round(float64(covered) / 16 * 255))
}

func pointInPolygon(points []SelectionPoint, x, y float64) bool {
	inside := false
	for left, right := len(points)-1, 0; right < len(points); left, right = right, right+1 {
		ax := points[left].X
		ay := points[left].Y
		bx := points[right].X
		by := points[right].Y
		intersects := ((ay > y) != (by > y)) && (x < (bx-ax)*(y-ay)/(by-ay+1e-9)+ax)
		if intersects {
			inside = !inside
		}
	}
	return inside
}

func normalizeSelectionRect(rect LayerBounds) LayerBounds {
	if rect.W < 0 {
		rect.X += rect.W
		rect.W = -rect.W
	}
	if rect.H < 0 {
		rect.Y += rect.H
		rect.H = -rect.H
	}
	return rect
}

func colorDistance(pixel []byte, target [4]uint8) float64 {
	red := float64(pixel[0]) - float64(target[0])
	green := float64(pixel[1]) - float64(target[1])
	blue := float64(pixel[2]) - float64(target[2])
	return math.Sqrt(red*red + green*green + blue*blue)
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
