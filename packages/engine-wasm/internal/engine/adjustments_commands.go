package engine

import (
	"encoding/json"
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Histogram computation
// ---------------------------------------------------------------------------

// HistogramData contains per-channel histogram bins (256 per channel).
type HistogramData struct {
	Red       [256]uint32 `json:"red"`
	Green     [256]uint32 `json:"green"`
	Blue      [256]uint32 `json:"blue"`
	Luminance [256]uint32 `json:"luminance"`
}

// ComputeHistogramPayload is the JSON payload for the ComputeHistogram command.
type ComputeHistogramPayload struct {
	LayerID string `json:"layerId"` // empty = active layer; "merged" = composite
}

// computeHistogram builds histogram data from a pixel layer or the merged composite.
func (inst *instance) computeHistogram(payloadJSON string) (*HistogramData, error) {
	var payload ComputeHistogramPayload
	if payloadJSON != "" {
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return nil, err
		}
	}

	doc := inst.manager.Active()
	if doc == nil {
		return nil, fmt.Errorf("compute histogram: no active document")
	}

	var surface []byte
	var w, h int

	if payload.LayerID == "merged" {
		surface = inst.compositeSurface(doc)
		w, h = doc.Width, doc.Height
	} else {
		layerID := payload.LayerID
		if layerID == "" {
			layerID = doc.ActiveLayerID
		}
		if pl := findPixelLayer(doc, layerID); pl != nil {
			surface = pl.Pixels
			w, h = pl.Bounds.W, pl.Bounds.H
		} else {
			// For non-pixel layers, use the merged composite.
			surface = inst.compositeSurface(doc)
			w, h = doc.Width, doc.Height
		}
	}

	if surface == nil || w <= 0 || h <= 0 {
		return &HistogramData{}, nil
	}

	var hist HistogramData
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if i+3 >= len(surface) {
				continue
			}
			// Skip fully transparent pixels.
			if surface[i+3] == 0 {
				continue
			}
			r, g, b := surface[i], surface[i+1], surface[i+2]
			hist.Red[r]++
			hist.Green[g]++
			hist.Blue[b]++
			lum := histogramChannelValue(r, g, b, "luminance")
			hist.Luminance[lum]++
		}
	}
	return &hist, nil
}

// ---------------------------------------------------------------------------
// Identify Hue Range (for Hue/Saturation eyedropper)
// ---------------------------------------------------------------------------

// IdentifyHueRangePayload is the JSON payload for commandIdentifyHueRange.
type IdentifyHueRangePayload struct {
	X          float64 `json:"x"` // document-space
	Y          float64 `json:"y"`
	SampleSize int     `json:"sampleSize"` // averaging radius, default 1
}

// identifyHueRange samples a color at the given position and returns which
// Hue/Saturation color range it falls into (reds, yellows, greens, etc.).
func (inst *instance) identifyHueRange(payloadJSON string) (string, error) {
	var payload IdentifyHueRangePayload
	if err := decodePayload(payloadJSON, &payload); err != nil {
		return "", err
	}

	doc := inst.manager.Active()
	if doc == nil {
		return "", fmt.Errorf("identify hue range: no active document")
	}

	surface := inst.compositeSurface(doc)
	w, h := doc.Width, doc.Height

	px := int(math.Round(payload.X))
	py := int(math.Round(payload.Y))
	if px < 0 || px >= w || py < 0 || py >= h {
		return "", fmt.Errorf("identify hue range: position (%d,%d) outside document", px, py)
	}

	sampleSize := payload.SampleSize
	if sampleSize <= 0 {
		sampleSize = 1
	}

	color, ok := sampleSurfaceColorAverage(surface, w, h, px, py, sampleSize)
	if !ok {
		return "", fmt.Errorf("identify hue range: could not sample color")
	}

	return classifyHueRange(color[0], color[1], color[2]), nil
}

// classifyHueRange determines which Hue/Saturation color range an RGB color
// falls into, based on the hue angle in HSL space.
func classifyHueRange(r, g, b uint8) string {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	maxC := math.Max(rf, math.Max(gf, bf))
	minC := math.Min(rf, math.Min(gf, bf))
	delta := maxC - minC

	// Very low saturation = neutrals (not classifiable to a hue range).
	if delta < 0.05 {
		return "master"
	}

	var hue float64
	switch maxC {
	case rf:
		hue = 60 * math.Mod((gf-bf)/delta, 6)
	case gf:
		hue = 60 * ((bf-rf)/delta + 2)
	case bf:
		hue = 60 * ((rf-gf)/delta + 4)
	}
	if hue < 0 {
		hue += 360
	}

	// Photoshop-style hue ranges (degrees):
	// Reds:     345-15    (wraps around 0)
	// Yellows:  15-75
	// Greens:   75-165
	// Cyans:    165-195
	// Blues:    195-285
	// Magentas: 285-345
	switch {
	case hue >= 345 || hue < 15:
		return "reds"
	case hue < 75:
		return "yellows"
	case hue < 165:
		return "greens"
	case hue < 195:
		return "cyans"
	case hue < 285:
		return "blues"
	default:
		return "magentas"
	}
}

// ---------------------------------------------------------------------------
// Set Point From Sample (Curves eyedropper)
// ---------------------------------------------------------------------------

// SetPointFromSamplePayload is the JSON payload for commandSetPointFromSample.
type SetPointFromSamplePayload struct {
	LayerID string  `json:"layerId"` // target curves adjustment layer
	X       float64 `json:"x"`       // document-space sample position
	Y       float64 `json:"y"`
	Mode    string  `json:"mode"` // "black", "white", "gray", or "add" (add point)
}

// handleSetPointFromSample samples a color from the composite and updates a
// curves adjustment layer accordingly. For "black"/"white"/"gray" modes it
// adjusts the curve to map the sampled luminance to 0/255/128. For "add" mode
// it adds a control point at the sampled value.
func (inst *instance) handleSetPointFromSample(payloadJSON string) error {
	var payload SetPointFromSamplePayload
	if err := decodePayload(payloadJSON, &payload); err != nil {
		return err
	}

	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("set point from sample: no active document")
	}

	// Sample color from composite.
	surface := inst.compositeSurface(doc)
	w, h := doc.Width, doc.Height
	px := int(math.Round(payload.X))
	py := int(math.Round(payload.Y))
	if px < 0 || px >= w || py < 0 || py >= h {
		return fmt.Errorf("set point from sample: position outside document")
	}

	color, ok := sampleSurfaceColorAverage(surface, w, h, px, py, 3)
	if !ok {
		return fmt.Errorf("set point from sample: could not sample color")
	}

	// Find the target adjustment layer.
	layerID := payload.LayerID
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	node := doc.findLayer(layerID)
	if node == nil {
		return fmt.Errorf("set point from sample: layer %q not found", layerID)
	}
	adj, ok := node.(*AdjustmentLayer)
	if !ok || adj.AdjustmentKind != "curves" {
		return fmt.Errorf("set point from sample: layer %q is not a curves adjustment", layerID)
	}

	// Parse current curves params.
	var params curvesParams
	if adj.Params != nil {
		if err := json.Unmarshal(adj.Params, &params); err != nil {
			return fmt.Errorf("set point from sample: %w", err)
		}
	}
	if params.Points == nil {
		params.Points = []curvePoint{{X: 0, Y: 0}, {X: 255, Y: 255}}
	}

	// Compute luminance of sampled color.
	lum := colorLuminance([3]float64{float64(color[0]) / 255, float64(color[1]) / 255, float64(color[2]) / 255})
	sampledValue := lum * 255

	switch payload.Mode {
	case "black":
		// Set black point: map sampled luminance → 0 output.
		params.Points = setOrAddCurvePoint(params.Points, sampledValue, 0)
	case "white":
		// Set white point: map sampled luminance → 255 output.
		params.Points = setOrAddCurvePoint(params.Points, sampledValue, 255)
	case "gray":
		// Set gray point: map sampled luminance → 128 output.
		params.Points = setOrAddCurvePoint(params.Points, sampledValue, 128)
	default:
		// "add": add a point at identity (input = output).
		params.Points = setOrAddCurvePoint(params.Points, sampledValue, sampledValue)
	}

	newParams, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return inst.executeDocCommand("Set curve point from sample", func(doc *Document) error {
		return doc.SetAdjustmentLayerParams(layerID, "", json.RawMessage(newParams))
	})
}

// setOrAddCurvePoint updates an existing point near x or adds a new one.
func setOrAddCurvePoint(points []curvePoint, x, y float64) []curvePoint {
	// If a point exists within ±5 of x, update it.
	for i, p := range points {
		if math.Abs(p.X-x) < 5 {
			points[i].Y = y
			return points
		}
	}
	// Add new point and sort by X.
	points = append(points, curvePoint{X: x, Y: y})
	for i := len(points) - 1; i > 0; i-- {
		if points[i].X < points[i-1].X {
			points[i], points[i-1] = points[i-1], points[i]
		}
	}
	return points
}
