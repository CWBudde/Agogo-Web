// Package engine — brush dab rasterizer (Phase 4.1).
package engine

import (
	"math"
	"math/rand"

	agglib "github.com/cwbudde/agg_go"
)

const mixerBrushBristleCount = 7

// BrushParams describes one brush dab's visual properties.
type BrushParams struct {
	Size             float64  `json:"size"`                              // Diameter in document pixels
	Hardness         float64  `json:"hardness"`                          // 0.0 (soft/feathered) – 1.0 (hard edge)
	Flow             float64  `json:"flow"`                              // Per-dab alpha multiplier, 0–1
	Color            [4]uint8 `json:"color"`                             // RGBA paint color
	BlendMode        string   `json:"blendMode,omitempty"`               // AGG blend mode string, e.g. "multiply", "screen"
	WetEdges         bool     `json:"wetEdges,omitempty"`                // Accumulate paint at stroke edges (watercolour effect)
	Scatter          float64  `json:"scatter,omitempty"`                 // Max random dab offset as a fraction of brush diameter (0 = none)
	Stabilizer       int      `json:"stabilizer,omitempty"`              // Moving-average lag: number of past input points to average (0 = off)
	SampleMerged     bool     `json:"sampleMerged,omitempty"`            // Sample composite (all layers) rather than active layer when reading pixels
	AutoErase        bool     `json:"autoErase,omitempty"`               // If stroke starts on foreground color, paint with background color instead
	Erase            bool     `json:"erase,omitempty"`                   // Erase to transparency (uses dst-out compositing)
	EraseBackground  bool     `json:"eraseBackground,omitempty"`         // Erase only pixels matching the sampled base color
	EraseTolerance   float64  `json:"eraseTolerance,omitempty"`          // Color tolerance for background eraser (0–255 Euclidean RGB distance)
	MixerBrush       bool     `json:"mixerBrush,omitempty"`              // Mix the brush color with sampled canvas color before painting
	MixerMix         float64  `json:"mixerMix,omitempty"`                // Sampled-color mix strength, 0–1
	MixerWetness     float64  `json:"mixerWetness,omitempty"`            // Wet-paint pickup strength, 0–1
	MixerLoad        float64  `json:"mixerLoad,omitempty"`               // Initial paint load when the brush is clean, 0–1
	CloneStamp       bool     `json:"cloneStamp,omitempty"`              // Clone pixels from a source point
	CloneSourceX     float64  `json:"cloneSourceX,omitempty"`            // Source point X in document space
	CloneSourceY     float64  `json:"cloneSourceY,omitempty"`            // Source point Y in document space
	CloneAligned     bool     `json:"cloneAligned,omitempty"`            // Keep the source offset fixed across strokes until the source changes
	CloneOpacity     float64  `json:"cloneOpacity,omitempty"`            // Overall source opacity multiplier, 0–1
	CloneLoad        float64  `json:"cloneLoad,omitempty"`               // Source load carried through the stroke, 0–1
	CloneHistory     bool     `json:"cloneHistorySource,omitempty"`      // Sample from a history snapshot instead of the live document
	CloneHistoryIdx  int      `json:"cloneHistorySourceIndex,omitempty"` // History entry id used when CloneHistory is enabled
	HistoryBrush     bool     `json:"historyBrush,omitempty"`            // Restore pixels from a previous history state
	HistorySourceIdx int      `json:"historySourceIndex,omitempty"`      // History entry id used as the source state for the history brush
	HistoryOpacity   float64  `json:"historyOpacity,omitempty"`          // Overall history-source opacity multiplier, 0–1
	HistoryLoad      float64  `json:"historyLoad,omitempty"`             // History-source load carried through the stroke, 0–1
}

// applyTilt derives the dab rotation angle and minor-axis squish factor from
// stylus tilt (PointerEvent.tiltX/Y, each in degrees –90…+90).
//
//   - azimuth: counter-clockwise angle from +X in radians — the direction the
//     stylus leans toward, which becomes the major axis of the elliptical dab.
//   - squish: Y-scale factor in dab-local space [minSquish, 1.0]; 1.0 = circular
//     (pen upright), → 0 as the pen approaches horizontal.
//
// Returns (0, 1) when both tilt components are zero (no-op path).
func applyTilt(tiltX, tiltY float64) (azimuth, squish float64) {
	if tiltX == 0 && tiltY == 0 {
		return 0, 1
	}
	// Azimuth: direction the stylus leans in the document plane.
	azimuth = math.Atan2(tiltY, tiltX)

	// Altitude: angular distance from horizontal (0 = flat, 90 = vertical).
	// tiltMag is degrees from vertical, so altitude = 90 − tiltMag degrees.
	tiltMag := math.Sqrt(tiltX*tiltX + tiltY*tiltY)
	altitudeDeg := 90 - tiltMag
	if altitudeDeg < 0 {
		altitudeDeg = 0
	}
	// squish = sin(altitude): 1.0 at 90° (upright), 0 at 0° (horizontal).
	squish = math.Sin(altitudeDeg * math.Pi / 180)
	const minSquish = 0.05 // prevent degenerate zero-width dabs
	if squish < minSquish {
		squish = minSquish
	}
	return azimuth, squish
}

// applyScatter returns (cx, cy) offset by a random displacement whose maximum
// radius equals p.Scatter * p.Size (full diameter). When p.Scatter is 0 the
// position is returned unchanged.
//
// The displacement is drawn from a uniform distribution over the disc
// (random angle, radius = sqrt(u)*maxR to keep area density uniform).
func applyScatter(cx, cy float64, p BrushParams) (float64, float64) {
	if p.Scatter <= 0 {
		return cx, cy
	}
	maxR := p.Scatter * p.Size
	angle := rand.Float64() * 2 * math.Pi
	r := math.Sqrt(rand.Float64()) * maxR
	return cx + math.Cos(angle)*r, cy + math.Sin(angle)*r
}

// PaintDab renders a single brush dab centred at (cx, cy) in document space
// onto the given PixelLayer. The layer buffer is modified in place.
// cx/cy are in document coordinates; the layer's Bounds offset is subtracted.
//
// azimuth is the CCW rotation angle in radians for the dab (0 = no rotation).
// squish  is the minor-axis Y scale [minSquish, 1.0] (1.0 = circular).
// Pass azimuth=0, squish=1.0 for untilted dabs.
//
// The transform applied is: Scale(1,squish) → Rotate(azimuth) → Translate(lx,ly)
// so the dab elongates along the azimuth direction (the stylus lean).
func PaintDab(layer *PixelLayer, cx, cy float64, p BrushParams, azimuth, squish float64) {
	paintDabReuse(agglib.NewAgg2D(), layer, cx, cy, p, azimuth, squish)
}

// paintDabReuse renders a dab using a pre-allocated AGG renderer. The renderer
// is Attach'd to the layer buffer on every call (which resets transforms and
// state) but its rasterizer keeps pre-allocated cell blocks, avoiding the
// dominant allocation cost of creating a fresh Agg2D per dab.
func paintDabReuse(renderer *agglib.Agg2D, layer *PixelLayer, cx, cy float64, p BrushParams, azimuth, squish float64) {
	w := layer.Bounds.W
	h := layer.Bounds.H
	if w <= 0 || h <= 0 {
		return
	}

	// Convert to layer-local coordinates.
	lx := cx - float64(layer.Bounds.X)
	ly := cy - float64(layer.Bounds.Y)

	radius := p.Size * 0.5
	if radius < 0.5 {
		radius = 0.5
	}
	flow := clampFloat(p.Flow, 0, 1)

	renderer.Attach(layer.Pixels, w, h, w*4)
	renderer.NoLine()

	// Apply blend mode. Normal erase uses dst-out (removes destination alpha
	// proportionally to the brush shape). Other blend modes use the string map.
	if p.Erase {
		renderer.BlendMode(agglib.BlendDstOut)
	} else if p.BlendMode != "" {
		renderer.BlendMode(agglib.StringToBlendMode(p.BlendMode))
	}

	// Build the dab transform (AGG uses pre-multiplication, so call order is
	// the reverse of the logical order):
	//   logical: Scale(1,squish) → Rotate(azimuth) → Translate(lx,ly)
	//   call order: Scale first, then Rotate, then Translate
	// This squishes the dab along its local Y axis, rotates it to the tilt
	// direction, then positions it at the fractional-pixel centre.
	if squish < 1.0 {
		renderer.Scale(1, squish)
	}
	if azimuth != 0 {
		renderer.Rotate(azimuth)
	}
	renderer.Translate(lx, ly)

	renderer.ResetPath()
	renderer.AddEllipse(0, 0, radius, radius, agglib.CCW)

	r, g, b, a := p.Color[0], p.Color[1], p.Color[2], p.Color[3]
	// For dst-out erasing the color channels are ignored; only alpha drives the erasure.
	// Use white so the intent is clear and AGG's internal path is straightforward.
	if p.Erase {
		r, g, b, a = 255, 255, 255, 255
	}

	if p.WetEdges {
		// Wet edges: paint accumulates at the stroke boundary (watercolour effect).
		// The alpha profile rises from transparent at the centre to a peak near the
		// edge, then falls back to transparent at the boundary:
		//   pos 0.0  → alpha 0       (transparent centre)
		//   pos 0.55 → alpha ~30%    (slight build-up begins)
		//   pos 0.75 → alpha 100%×flow (peak ring)
		//   pos 1.0  → alpha 0       (AA fade at boundary)
		// Hardness shifts the peak inward: a hard brush has a sharper, wider ring.
		peak := 0.75 - p.Hardness*0.25 // softer → peak closer to edge (0.75); harder → 0.50
		peakAlpha := uint8(float64(a) * flow)
		transparent := agglib.NewColor(r, g, b, 0)
		semiRing := agglib.NewColor(r, g, b, uint8(float64(peakAlpha)*0.3))
		opaqueRing := agglib.NewColor(r, g, b, peakAlpha)
		stops := []agglib.GradientStop{
			{Position: 0.0, Color: transparent},
			{Position: peak * 0.73, Color: semiRing},
			{Position: peak, Color: opaqueRing},
			{Position: 1.0, Color: transparent},
		}
		renderer.FillRadialGradientStops(0, 0, radius, stops)
		renderer.DrawPath(agglib.FillOnly)
		return
	}

	if p.Hardness >= 1.0 {
		// Hard edge: solid fill; AGG provides sub-pixel AA at the ellipse boundary.
		alpha := uint8(float64(a) * flow)
		c := agglib.NewColor(r, g, b, alpha)
		renderer.FillColor(c)
		renderer.DrawPath(agglib.FillOnly)
		return
	}

	// Soft edge: radial gradient from opaque centre to transparent edge.
	// Shape defined at origin; transform carries the centre to (lx, ly).
	centerAlpha := uint8(float64(a) * flow)
	c1 := agglib.NewColor(r, g, b, centerAlpha)
	c2 := agglib.NewColor(r, g, b, 0)
	renderer.FillRadialGradient(0, 0, radius, c1, c2, 1.0)
	renderer.DrawPath(agglib.FillOnly)
}

// EraseBackgroundDab erases pixels within the dab area whose color is within
// p.EraseTolerance (Euclidean RGB distance) of baseColor. Pixels outside the
// tolerance band are left untouched. The erasure amount is modulated by the
// brush mask alpha (hardness/gradient) and p.Flow.
//
// Unlike PaintDab this is a direct per-pixel operation — no AGG compositing —
// because the erase decision depends on each pixel's existing color.
func EraseBackgroundDab(layer *PixelLayer, cx, cy float64, p BrushParams, baseColor [4]uint8) {
	w := layer.Bounds.W
	h := layer.Bounds.H
	if w <= 0 || h <= 0 {
		return
	}
	lx := cx - float64(layer.Bounds.X)
	ly := cy - float64(layer.Bounds.Y)

	radius := p.Size * 0.5
	if radius < 0.5 {
		radius = 0.5
	}
	flow := clampFloat(p.Flow, 0, 1)
	tolerance := clampFloat(p.EraseTolerance, 0, 442)

	// Axis-aligned bounding box of the dab (conservative — use full radius).
	x0 := int(lx-radius) - 1
	y0 := int(ly-radius) - 1
	x1 := int(lx+radius) + 2
	y1 := int(ly+radius) + 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y1 > h {
		y1 = h
	}

	hardness := clampFloat(p.Hardness, 0, 1)

	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			// Normalised distance from dab centre.
			dx := float64(px) - lx
			dy := float64(py) - ly
			dist := math.Sqrt(dx*dx+dy*dy) / radius
			if dist > 1.0 {
				continue
			}

			// Brush mask alpha: 1.0 in the core, falls off toward the edge.
			var maskAlpha float64
			if hardness >= 1.0 || dist <= hardness {
				maskAlpha = 1.0
			} else {
				// Linear falloff from hardness radius to edge.
				maskAlpha = 1.0 - (dist-hardness)/(1.0-hardness)
			}
			if maskAlpha <= 0 {
				continue
			}

			// Check destination color against base color.
			idx := (py*w + px) * 4
			if layer.Pixels[idx+3] == 0 {
				continue // already transparent
			}
			pix := layer.Pixels[idx : idx+4]
			dist2base := colorDistance(pix, baseColor)
			if tolerance == 0 && dist2base > 0 {
				continue
			}
			if tolerance > 0 && dist2base > tolerance {
				continue
			}

			// Soft fade: pixels closer to baseColor are erased more.
			var coverage float64
			if tolerance == 0 {
				coverage = 1.0
			} else {
				coverage = 1.0 - dist2base/tolerance
			}

			eraseAmount := maskAlpha * flow * coverage
			newAlpha := float64(pix[3]) * (1.0 - eraseAmount)
			if newAlpha < 0 {
				newAlpha = 0
			}
			layer.Pixels[idx+3] = uint8(newAlpha)
		}
	}
}

// applyPressure scales brush Size and Flow by the pointer pressure value (0–1).
// At pressure=1.0 the brush is full size; at pressure=0.0 it is 50% size.
func applyPressure(p BrushParams, pressure float64) BrushParams {
	if pressure <= 0 {
		pressure = 0.5
	}
	p.Size = p.Size * (0.5 + 0.5*pressure)
	p.Flow = clampFloat(p.Flow*pressure, 0, 1)
	return p
}

func normalizeMixerBrushParams(p BrushParams) BrushParams {
	if !p.MixerBrush {
		return p
	}
	if p.MixerWetness <= 0 && p.MixerMix > 0 {
		p.MixerWetness = p.MixerMix
	}
	if p.MixerLoad <= 0 {
		p.MixerLoad = 1
	}
	p.MixerWetness = clampFloat(p.MixerWetness, 0, 1)
	p.MixerLoad = clampFloat(p.MixerLoad, 0, 1)
	return p
}

func normalizeCloneStampParams(p BrushParams) BrushParams {
	if !p.CloneStamp {
		return p
	}
	if p.CloneOpacity <= 0 {
		p.CloneOpacity = 1
	}
	if p.CloneLoad <= 0 {
		p.CloneLoad = 1
	}
	p.CloneOpacity = clampFloat(p.CloneOpacity, 0, 1)
	p.CloneLoad = clampFloat(p.CloneLoad, 0, 1)
	if p.CloneHistoryIdx < 0 {
		p.CloneHistoryIdx = 0
	}
	return p
}

func normalizeHistoryBrushParams(p BrushParams) BrushParams {
	if !p.HistoryBrush {
		return p
	}
	if p.HistoryOpacity <= 0 {
		p.HistoryOpacity = 1
	}
	if p.HistoryLoad <= 0 {
		p.HistoryLoad = 1
	}
	p.HistoryOpacity = clampFloat(p.HistoryOpacity, 0, 1)
	p.HistoryLoad = clampFloat(p.HistoryLoad, 0, 1)
	if p.HistorySourceIdx < 0 {
		p.HistorySourceIdx = 0
	}
	return p
}

func captureStrokeSourceSurface(doc *Document, layer *PixelLayer, sampleMerged bool) ([]byte, int, int, int, int) {
	if sampleMerged {
		if doc == nil {
			return nil, 0, 0, 0, 0
		}
		surface := doc.renderCompositeSurface()
		if len(surface) == 0 {
			return nil, 0, 0, 0, 0
		}
		return surface, doc.Width, doc.Height, 0, 0
	}
	if layer == nil || len(layer.Pixels) == 0 {
		return nil, 0, 0, 0, 0
	}
	return append([]byte(nil), layer.Pixels...), layer.Bounds.W, layer.Bounds.H, layer.Bounds.X, layer.Bounds.Y
}

func captureHistorySourceSurface(state snapshot, sampleMerged bool) ([]byte, int, int, int, int) {
	doc := state.Document
	if doc == nil {
		return nil, 0, 0, 0, 0
	}
	if sampleMerged {
		surface := doc.renderCompositeSurface()
		if len(surface) == 0 {
			return nil, 0, 0, 0, 0
		}
		return surface, doc.Width, doc.Height, 0, 0
	}
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return nil, 0, 0, 0, 0
	}
	return append([]byte(nil), layer.Pixels...), layer.Bounds.W, layer.Bounds.H, layer.Bounds.X, layer.Bounds.Y
}

func (inst *instance) resetMixerBrushState() {
	inst.mixerBrush = mixerBrushState{clean: true}
}

func (inst *instance) resetCloneStampState() {
	inst.cloneStamp = cloneStampState{}
}

func (inst *instance) beginCloneStampStroke(docID string, cx, cy float64, params BrushParams) (float64, float64) {
	offsetX := params.CloneSourceX - cx
	offsetY := params.CloneSourceY - cy
	if !params.CloneAligned {
		inst.resetCloneStampState()
		return offsetX, offsetY
	}
	state := inst.cloneStamp
	if state.hasAlignedOffset &&
		state.docID == docID &&
		math.Abs(state.sourceX-params.CloneSourceX) < 0.001 &&
		math.Abs(state.sourceY-params.CloneSourceY) < 0.001 {
		return state.offsetX, state.offsetY
	}
	inst.cloneStamp = cloneStampState{
		docID:            docID,
		sourceX:          params.CloneSourceX,
		sourceY:          params.CloneSourceY,
		offsetX:          offsetX,
		offsetY:          offsetY,
		hasAlignedOffset: true,
	}
	return offsetX, offsetY
}

func (inst *instance) beginMixerBrushStroke(docID string, params BrushParams) mixerBrushState {
	state := inst.mixerBrush
	if !state.clean && state.docID == docID {
		return normalizeMixerState(state, docID, params)
	}
	state = mixerBrushState{
		docID:          docID,
		reservoirColor: params.Color,
		remainingLoad:  clampFloat(params.MixerLoad, 0, 1),
		contamination:  0,
		clean:          false,
	}
	for i := range state.bristleColors {
		state.bristleColors[i] = params.Color
		state.bristleLoads[i] = state.remainingLoad
	}
	return state
}

func normalizeMixerState(state mixerBrushState, docID string, params BrushParams) mixerBrushState {
	state.docID = docID
	state.remainingLoad = clampFloat(state.remainingLoad, 0, 1)
	state.contamination = clampFloat(state.contamination, 0, 1)
	if state.reservoirColor[3] == 0 {
		state.reservoirColor = params.Color
	}
	missing := true
	for i := range state.bristleColors {
		state.bristleLoads[i] = clampFloat(state.bristleLoads[i], 0, 1)
		if state.bristleLoads[i] > 0 || state.bristleColors[i][3] > 0 {
			missing = false
		}
	}
	if missing {
		for i := range state.bristleColors {
			state.bristleColors[i] = state.reservoirColor
			state.bristleLoads[i] = state.remainingLoad
		}
	}
	state.clean = false
	return state
}

func collapseMixerState(state *mixerBrushState) {
	if state == nil {
		return
	}
	var totalWeight float64
	var sumR, sumG, sumB, sumA float64
	var totalLoad float64
	for i := range state.bristleColors {
		load := clampFloat(state.bristleLoads[i], 0, 1)
		totalLoad += load
		weight := load + 0.08
		totalWeight += weight
		sumR += float64(state.bristleColors[i][0]) * weight
		sumG += float64(state.bristleColors[i][1]) * weight
		sumB += float64(state.bristleColors[i][2]) * weight
		sumA += float64(state.bristleColors[i][3]) * weight
	}
	if totalWeight > 0 {
		state.reservoirColor = [4]uint8{
			uint8(math.Round(sumR / totalWeight)),
			uint8(math.Round(sumG / totalWeight)),
			uint8(math.Round(sumB / totalWeight)),
			uint8(math.Round(sumA / totalWeight)),
		}
	}
	state.remainingLoad = clampFloat(totalLoad/float64(len(state.bristleLoads)), 0, 1)
	if state.remainingLoad > 0 {
		state.clean = false
	}
}

func mixerStrokeDirection(stroke *activePaintStroke, cx, cy, fallbackAzimuth float64) (float64, float64) {
	dirX := math.Cos(fallbackAzimuth)
	dirY := math.Sin(fallbackAzimuth)
	if stroke == nil {
		return dirX, dirY
	}
	if stroke.hasLastDab {
		dx := cx - stroke.lastDabX
		dy := cy - stroke.lastDabY
		if dx*dx+dy*dy > 1e-6 {
			invLen := 1 / math.Sqrt(dx*dx+dy*dy)
			dirX = dx * invLen
			dirY = dy * invLen
		} else if stroke.lastDirX != 0 || stroke.lastDirY != 0 {
			dirX = stroke.lastDirX
			dirY = stroke.lastDirY
		}
	} else if stroke.lastDirX != 0 || stroke.lastDirY != 0 {
		dirX = stroke.lastDirX
		dirY = stroke.lastDirY
	}
	return dirX, dirY
}

func updateMixerStrokeDirection(stroke *activePaintStroke, cx, cy float64) {
	if stroke == nil {
		return
	}
	if stroke.hasLastDab {
		dx := cx - stroke.lastDabX
		dy := cy - stroke.lastDabY
		if dx*dx+dy*dy > 1e-6 {
			invLen := 1 / math.Sqrt(dx*dx+dy*dy)
			stroke.lastDirX = dx * invLen
			stroke.lastDirY = dy * invLen
		}
	}
	stroke.lastDabX = cx
	stroke.lastDabY = cy
	stroke.hasLastDab = true
}

func mixerBristleOffset(i int) float64 {
	if mixerBrushBristleCount <= 1 {
		return 0
	}
	return -1 + 2*float64(i)/float64(mixerBrushBristleCount-1)
}

func mixerBristleSize(size float64) float64 {
	bristleSize := size * 0.22
	if bristleSize < 1.5 {
		return 1.5
	}
	return bristleSize
}

func mixerBristleSampleParams(p BrushParams, size float64) BrushParams {
	sample := p
	sample.Size = size * 1.18
	sample.Hardness = clampFloat(0.2+p.Hardness*0.6, 0, 1)
	return sample
}

func mixerBristlePaintParams(p BrushParams, color [4]uint8, load, edgeFactor float64, size float64) BrushParams {
	dab := p
	dab.MixerBrush = false
	dab.Color = color
	dab.Size = size
	dab.Hardness = clampFloat(0.55+p.Hardness*0.4, 0, 1)
	edgeProfile := 0.18 + 1.12*math.Pow(edgeFactor, 0.65)
	dab.Flow = clampFloat(p.Flow*(0.28+0.72*load)*edgeProfile, 0, 1)
	return dab
}

func sampleSurfaceBilinearTransparent(source []byte, sourceW, sourceH int, sampleX, sampleY float64) ([4]uint8, bool) {
	var zero [4]uint8
	if sourceW <= 0 || sourceH <= 0 || len(source) == 0 {
		return zero, false
	}
	fx := sampleX - 0.5
	fy := sampleY - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	weights := [4]float64{
		(1 - tx) * (1 - ty),
		tx * (1 - ty),
		(1 - tx) * ty,
		tx * ty,
	}
	coords := [4][2]int{
		{x0, y0},
		{x0 + 1, y0},
		{x0, y0 + 1},
		{x0 + 1, y0 + 1},
	}

	var sumR, sumG, sumB, sumA float64
	for i, coord := range coords {
		x := coord[0]
		y := coord[1]
		if x < 0 || y < 0 || x >= sourceW || y >= sourceH {
			continue
		}
		weight := weights[i]
		if weight <= 0 {
			continue
		}
		idx := (y*sourceW + x) * 4
		alpha := (float64(source[idx+3]) / 255) * weight
		if alpha <= 0 {
			continue
		}
		sumA += alpha
		sumR += (float64(source[idx]) / 255) * alpha
		sumG += (float64(source[idx+1]) / 255) * alpha
		sumB += (float64(source[idx+2]) / 255) * alpha
	}
	if sumA <= 0 {
		return zero, false
	}
	invAlpha := 1 / sumA
	return [4]uint8{
		uint8(math.Round(clampFloat(sumR*invAlpha, 0, 1) * 255)),
		uint8(math.Round(clampFloat(sumG*invAlpha, 0, 1) * 255)),
		uint8(math.Round(clampFloat(sumB*invAlpha, 0, 1) * 255)),
		uint8(math.Round(clampFloat(sumA, 0, 1) * 255)),
	}, true
}

// CloneStampDab copies pixels from a sampled source surface into the dab area
// using the same brush mask profile as PaintDab. The source is sampled in
// document space with bilinear filtering so translated/subpixel offsets and
// source edges behave more like Photoshop's clone tools.
func CloneStampDab(layer *PixelLayer, source []byte, sourceW, sourceH, sourceOriginX, sourceOriginY int, cx, cy float64, p BrushParams, sourceOffsetX, sourceOffsetY float64, remainingLoad *float64) {
	w := layer.Bounds.W
	h := layer.Bounds.H
	if w <= 0 || h <= 0 || sourceW <= 0 || sourceH <= 0 || len(source) == 0 {
		return
	}

	lx := cx - float64(layer.Bounds.X)
	ly := cy - float64(layer.Bounds.Y)
	radius := p.Size * 0.5
	if radius < 0.5 {
		radius = 0.5
	}
	flow := clampFloat(p.Flow, 0, 1)
	cloneOpacity := 1.0
	loadFactor := 1.0
	if p.CloneStamp || p.HistoryBrush {
		if p.CloneStamp {
			cloneOpacity = clampFloat(p.CloneOpacity, 0, 1)
		} else {
			cloneOpacity = clampFloat(p.HistoryOpacity, 0, 1)
		}
		if remainingLoad != nil {
			loadFactor = clampFloat(*remainingLoad, 0, 1)
		}
	}

	x0 := int(lx-radius) - 1
	y0 := int(ly-radius) - 1
	x1 := int(lx+radius) + 2
	y1 := int(ly+radius) + 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y1 > h {
		y1 = h
	}

	hardness := clampFloat(p.Hardness, 0, 1)
	painted := false
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			maskAlpha := brushMaskAlphaAt(float64(px)-lx, float64(py)-ly, radius, hardness, 0, 1)
			if maskAlpha <= 0 {
				continue
			}

			sampleX := float64(layer.Bounds.X+px) + sourceOffsetX + 0.5 - float64(sourceOriginX)
			sampleY := float64(layer.Bounds.Y+py) + sourceOffsetY + 0.5 - float64(sourceOriginY)
			srcPixel, ok := sampleSurfaceBilinearTransparent(source, sourceW, sourceH, sampleX, sampleY)
			if !ok || srcPixel[3] == 0 {
				continue
			}

			destIndex := (py*w + px) * 4
			opacity := maskAlpha * flow * cloneOpacity * loadFactor
			if opacity <= 0 {
				continue
			}

			compositePixelWithBlend(layer.Pixels[destIndex:destIndex+4], srcPixel[:], BlendMode(p.BlendMode), opacity, 0)
			painted = true
		}
	}
	if painted && remainingLoad != nil {
		decay := clampFloat(0.05+flow*0.12, 0, 1)
		*remainingLoad = clampFloat(*remainingLoad-decay, 0, 1)
	}
}

func brushMaskAlphaAt(dx, dy, radius, hardness, azimuth, squish float64) float64 {
	if radius <= 0 {
		return 0
	}
	localX := dx
	localY := dy
	if azimuth != 0 {
		cosA := math.Cos(azimuth)
		sinA := math.Sin(azimuth)
		localX = dx*cosA + dy*sinA
		localY = -dx*sinA + dy*cosA
	}
	if squish > 0 && squish < 1 {
		localY /= squish
	}
	dist := math.Sqrt(localX*localX+localY*localY) / radius
	if dist > 1.0 {
		return 0
	}
	if hardness >= 1.0 || dist <= hardness {
		return 1.0
	}
	if hardness <= 0 {
		return 1.0 - dist
	}
	return 1.0 - (dist-hardness)/(1.0-hardness)
}

func sampleSurfaceColorFootprint(source []byte, sourceW, sourceH, sourceOriginX, sourceOriginY int, cx, cy float64, p BrushParams, azimuth, squish float64) ([4]uint8, float64, bool) {
	var zero [4]uint8
	if len(source) == 0 || sourceW <= 0 || sourceH <= 0 {
		return zero, 0, false
	}
	radius := p.Size * 0.5
	if radius < 0.5 {
		radius = 0.5
	}
	hardness := clampFloat(p.Hardness, 0, 1)
	srcCX := cx - float64(sourceOriginX)
	srcCY := cy - float64(sourceOriginY)
	x0 := int(math.Floor(srcCX-radius)) - 1
	y0 := int(math.Floor(srcCY-radius)) - 1
	x1 := int(math.Ceil(srcCX+radius)) + 2
	y1 := int(math.Ceil(srcCY+radius)) + 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > sourceW {
		x1 = sourceW
	}
	if y1 > sourceH {
		y1 = sourceH
	}

	var sumR, sumG, sumB float64
	var weightedCoverage float64
	var maskSum float64
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			mask := brushMaskAlphaAt(float64(px)+0.5-srcCX, float64(py)+0.5-srcCY, radius, hardness, azimuth, squish)
			if mask <= 0 {
				continue
			}
			maskSum += mask
			idx := (py*sourceW + px) * 4
			alpha := float64(source[idx+3]) / 255
			if alpha <= 0 {
				continue
			}
			weight := mask * alpha
			weightedCoverage += weight
			sumR += float64(source[idx]) * weight
			sumG += float64(source[idx+1]) * weight
			sumB += float64(source[idx+2]) * weight
		}
	}
	if weightedCoverage <= 0 || maskSum <= 0 {
		return zero, 0, false
	}
	coverage := clampFloat(weightedCoverage/maskSum, 0, 1)
	return [4]uint8{
		uint8(math.Round(sumR / weightedCoverage)),
		uint8(math.Round(sumG / weightedCoverage)),
		uint8(math.Round(sumB / weightedCoverage)),
		uint8(math.Round(coverage * 255)),
	}, coverage, true
}

func mixColorRGBA(a, b [4]uint8, t float64) [4]uint8 {
	t = clampFloat(t, 0, 1)
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	inv := 1 - t
	return [4]uint8{
		uint8(math.Round(float64(a[0])*inv + float64(b[0])*t)),
		uint8(math.Round(float64(a[1])*inv + float64(b[1])*t)),
		uint8(math.Round(float64(a[2])*inv + float64(b[2])*t)),
		uint8(math.Round(float64(a[3])*inv + float64(b[3])*t)),
	}
}

func paintMixerBrushDab(renderer *agglib.Agg2D, layer *PixelLayer, state *mixerBrushState, source []byte, sourceW, sourceH, sourceX, sourceY int, cx, cy float64, p BrushParams, directionAzimuth, squish float64) {
	if state == nil {
		paintDabReuse(renderer, layer, cx, cy, p, directionAzimuth, squish)
		return
	}
	state.clean = false

	dirX := math.Cos(directionAzimuth)
	dirY := math.Sin(directionAzimuth)
	lateralX := -dirY
	lateralY := dirX
	radius := p.Size * 0.5
	if radius < 0.5 {
		radius = 0.5
	}
	bristleSize := mixerBristleSize(p.Size)
	sampleParams := mixerBristleSampleParams(p, bristleSize)
	squish = math.Max(0.18, squish*0.55)
	sampleLagBase := radius * (0.18 + 0.52*p.MixerWetness)

	for i := 0; i < mixerBrushBristleCount; i++ {
		offset := mixerBristleOffset(i)
		edgeFactor := math.Abs(offset)
		load := clampFloat(state.bristleLoads[i], 0, 1)
		lateralOffset := offset * radius * 0.58
		streakPhase := math.Sin(float64(i)*1.971) * radius * 0.06
		bristleCX := cx + lateralX*lateralOffset + dirX*streakPhase
		bristleCY := cy + lateralY*lateralOffset + dirY*streakPhase

		paintParams := mixerBristlePaintParams(p, state.bristleColors[i], load, edgeFactor, bristleSize)
		if paintParams.Flow > 0 {
			paintDabReuse(renderer, layer, bristleCX, bristleCY, paintParams, directionAzimuth, squish)
			if edgeFactor > 0.62 {
				offsetSign := 1.0
				if offset < 0 {
					offsetSign = -1
				}
				rim := paintParams
				rim.Flow = clampFloat(paintParams.Flow*(0.34+0.22*edgeFactor), 0, 1)
				rim.Size = math.Max(1.2, paintParams.Size*0.9)
				rimCX := bristleCX + lateralX*offsetSign*paintParams.Size*0.58
				rimCY := bristleCY + lateralY*offsetSign*paintParams.Size*0.58
				paintDabReuse(renderer, layer, rimCX, rimCY, rim, directionAzimuth, squish)
			}
		}

		deposited := paintParams.Flow * (0.2 + 0.18*edgeFactor)
		state.bristleLoads[i] = clampFloat(load-deposited, 0, 1)

		sampleLag := sampleLagBase * (0.35 + 0.65*edgeFactor)
		sampleCX := bristleCX - dirX*sampleLag
		sampleCY := bristleCY - dirY*sampleLag
		sampled, coverage, ok := sampleSurfaceColorFootprint(source, sourceW, sourceH, sourceX, sourceY, sampleCX, sampleCY, sampleParams, directionAzimuth, squish)
		if !ok {
			continue
		}

		pickup := clampFloat(p.MixerWetness*coverage*(0.34+0.66*(1-load))*(0.82+0.18*edgeFactor), 0, 1)
		if pickup <= 0 {
			continue
		}

		state.bristleColors[i] = mixColorRGBA(state.bristleColors[i], sampled, pickup)
		state.bristleLoads[i] = clampFloat(state.bristleLoads[i]+pickup*coverage*0.92, 0, 1)
		state.contamination = clampFloat(state.contamination+pickup*(1-state.contamination), 0, 1)
	}
	collapseMixerState(state)
}

// saveRowsBeforeDab saves the original (pre-paint) pixel rows that the dab at
// (cx, cy) with the given brush size will touch.  It lazily grows the saved row
// range as the dirty area expands, using buf (typically instance.undoRowBuf) as
// a reusable backing store to avoid per-stroke allocations.
func (s *activePaintStroke) saveRowsBeforeDab(layer *PixelLayer, _, cy, size float64, buf *[]byte) {
	r := int(math.Ceil(size*0.5)) + 2
	needYMin := int(cy) - layer.Bounds.Y - r
	needYMax := int(cy) - layer.Bounds.Y + r
	if needYMin < 0 {
		needYMin = 0
	}
	if needYMax > layer.Bounds.H {
		needYMax = layer.Bounds.H
	}
	if needYMax <= needYMin {
		return
	}

	rowBytes := layer.Bounds.W * 4

	if s.layerW == 0 {
		// First dab — initialise the row snapshot.
		s.layerW = layer.Bounds.W
		s.beforeRowStart = needYMin
		s.beforeRowEnd = needYMax
		needed := (needYMax - needYMin) * rowBytes
		if cap(*buf) >= needed {
			*buf = (*buf)[:needed]
		} else {
			*buf = make([]byte, needed)
		}
		copy((*buf)[:needed], layer.Pixels[needYMin*rowBytes:needYMax*rowBytes])
		s.beforeRowBuf = (*buf)[:needed]
		return
	}

	// Determine the new row range after merging the dab's Y extent.
	newMin, newMax := s.beforeRowStart, s.beforeRowEnd
	if needYMin < newMin {
		newMin = needYMin
	}
	if needYMax > newMax {
		newMax = needYMax
	}
	if newMin == s.beforeRowStart && newMax == s.beforeRowEnd {
		return // already covered
	}

	needed := (newMax - newMin) * rowBytes
	oldLen := (s.beforeRowEnd - s.beforeRowStart) * rowBytes

	if cap(*buf) < needed {
		// Need a bigger buffer — allocate and copy existing data at its new offset.
		newBuf := make([]byte, needed)
		offset := (s.beforeRowStart - newMin) * rowBytes
		copy(newBuf[offset:offset+oldLen], (*buf)[:oldLen])
		*buf = newBuf
	} else {
		*buf = (*buf)[:needed]
		if newMin < s.beforeRowStart {
			// Extending upward — shift existing data right. copy handles overlap.
			offset := (s.beforeRowStart - newMin) * rowBytes
			copy((*buf)[offset:offset+oldLen], (*buf)[:oldLen])
		}
	}

	// Copy newly-needed rows from the (still unmodified) layer pixels.
	if newMin < s.beforeRowStart {
		srcStart := newMin * rowBytes
		srcEnd := s.beforeRowStart * rowBytes
		copy((*buf)[:srcEnd-srcStart], layer.Pixels[srcStart:srcEnd])
	}
	if newMax > s.beforeRowEnd {
		dstOffset := (s.beforeRowEnd - newMin) * rowBytes
		srcStart := s.beforeRowEnd * rowBytes
		srcEnd := newMax * rowBytes
		copy((*buf)[dstOffset:], layer.Pixels[srcStart:srcEnd])
	}

	s.beforeRowStart = newMin
	s.beforeRowEnd = newMax
	s.beforeRowBuf = (*buf)[:needed]
}

// expandDirty grows the stroke's dirty bounding box to include the dab at (cx, cy).
// cx/cy are in document space.
func (s *activePaintStroke) expandDirty(layer *PixelLayer, cx, cy, size float64) {
	r := int(math.Ceil(size*0.5)) + 2 // +2 for AA fringe
	lx := int(cx) - layer.Bounds.X - r
	ly := int(cy) - layer.Bounds.Y - r
	rx := int(cx) - layer.Bounds.X + r
	ry := int(cy) - layer.Bounds.Y + r

	if lx < 0 {
		lx = 0
	}
	if ly < 0 {
		ly = 0
	}
	if rx > layer.Bounds.W {
		rx = layer.Bounds.W
	}
	if ry > layer.Bounds.H {
		ry = layer.Bounds.H
	}

	if !s.hasDirty {
		s.dirtyMin = [2]int{lx, ly}
		s.dirtyMax = [2]int{rx, ry}
		s.hasDirty = true
		return
	}
	if lx < s.dirtyMin[0] {
		s.dirtyMin[0] = lx
	}
	if ly < s.dirtyMin[1] {
		s.dirtyMin[1] = ly
	}
	if rx > s.dirtyMax[0] {
		s.dirtyMax[0] = rx
	}
	if ry > s.dirtyMax[1] {
		s.dirtyMax[1] = ry
	}
}

// findPixelLayer searches the document's layer tree for a PixelLayer with the given ID.
// Returns nil if not found or if the matching layer is not a PixelLayer.
func findPixelLayer(doc *Document, layerID string) *PixelLayer {
	if doc == nil || layerID == "" {
		return nil
	}
	var found *PixelLayer
	walkLayers(doc.LayerRoot, func(n LayerNode) bool {
		if n.ID() == layerID {
			if pl, ok := n.(*PixelLayer); ok {
				found = pl
				return false
			}
		}
		return true
	})
	return found
}

// walkLayers calls fn for each LayerNode in the tree (depth-first, pre-order).
// If fn returns false the walk stops early.
func walkLayers(root *GroupLayer, fn func(LayerNode) bool) {
	if root == nil {
		return
	}
	for _, child := range root.Children() {
		if !fn(child) {
			return
		}
		if g, ok := child.(*GroupLayer); ok {
			walkLayers(g, fn)
		}
	}
}

// stabilizerState implements a moving-average input smoother.
// The last Lag raw pointer positions are averaged before being fed to the
// spline interpolator; this removes jitter / hand-tremor at the cost of
// introducing a positional lag proportional to Lag.
//
// When len(buf)==0 (Lag=0) the input passes through unchanged.
type stabilizerState struct {
	buf  [][2]float64
	head int
	n    int
}

// newStabilizer allocates a stabilizerState with the given ring-buffer capacity.
// lag ≤ 0 returns a zero-value state that is a no-op.
func newStabilizer(lag int) stabilizerState {
	if lag <= 0 {
		return stabilizerState{}
	}
	return stabilizerState{buf: make([][2]float64, lag)}
}

// Push records a raw point and returns the smoothed position (mean of the
// buffer's valid entries). The first Push always returns the input unchanged
// so the stroke starts at the exact cursor position.
func (s *stabilizerState) Push(x, y float64) (float64, float64) {
	if len(s.buf) == 0 {
		return x, y
	}
	s.buf[s.head] = [2]float64{x, y}
	s.head = (s.head + 1) % len(s.buf)
	if s.n < len(s.buf) {
		s.n++
	}
	var sx, sy float64
	for i := range s.n {
		sx += s.buf[i][0]
		sy += s.buf[i][1]
	}
	return sx / float64(s.n), sy / float64(s.n)
}

// brushStrokeState tracks an in-progress paint stroke for dab spacing.
// Dab positions are interpolated along a Catmull-Rom spline through the raw
// input points, giving smooth curves even when pointer events arrive sparsely.
type brushStrokeState struct {
	prevPrev    [2]float64 // P0 control point for CR (point before prev)
	prev        [2]float64 // P1 — previous raw input, start of current CR segment
	hasPrev     bool
	hasPrevPrev bool
	travelled   float64 // carry-over distance since the last dab [0, interval)
}

// AddPoint takes a new pointer position and returns document-space positions
// where dabs should be placed. spacing is the dab interval as a fraction of
// brush diameter (e.g. 0.25 = every 25% of size). Always places a dab on
// the first call.
//
// Subsequent calls interpolate along a Catmull-Rom spline: the segment from
// prev→pt is smoothed using prevPrev as the before-tangent and an extrapolated
// after-tangent (2·pt − prev), so the stroke curves through every input point.
func (s *brushStrokeState) AddPoint(x, y, spacing, size float64) [][2]float64 {
	pt := [2]float64{x, y}

	// First point: always plant a dab at the exact start position.
	if !s.hasPrev {
		s.prev = pt
		s.hasPrev = true
		return [][2]float64{pt}
	}

	p1 := s.prev // segment start
	p2 := pt     // segment end

	// P0: the control point before p1.
	// Use the recorded prevPrev if available; otherwise reflect p2 around p1
	// so the tangent at the stroke start is directed away from p2.
	var p0 [2]float64
	if s.hasPrevPrev {
		p0 = s.prevPrev
	} else {
		p0 = [2]float64{2*p1[0] - p2[0], 2*p1[1] - p2[1]}
	}

	// P3: extrapolated "next" point used as the after-tangent control.
	// Extrapolating keeps the curve tangent at p2 pointed toward p3.
	p3 := [2]float64{2*p2[0] - p1[0], 2*p2[1] - p1[1]}

	positions := s.sampleCR(p0, p1, p2, p3, spacing, size)

	// Shift history.
	s.prevPrev = s.prev
	s.hasPrevPrev = true
	s.prev = pt

	return positions
}

// sampleCR places dabs along the Catmull-Rom segment from p1 to p2 (using p0
// and p3 as tangent controls) and returns their document-space positions.
// It respects the carry-over distance in s.travelled and updates it.
func (s *brushStrokeState) sampleCR(p0, p1, p2, p3 [2]float64, spacing, size float64) [][2]float64 {
	interval := spacing * size
	if interval < 1.0 {
		interval = 1.0
	}

	// Build an arc-length table by sampling the CR curve at nSamples steps.
	const nSamples = 32
	var arcLen [nSamples + 1]float64
	var crPts [nSamples + 1][2]float64
	crPts[0] = p1
	for i := 1; i <= nSamples; i++ {
		t := float64(i) / float64(nSamples)
		crPts[i] = catmullRomPoint(p0, p1, p2, p3, t)
		dx := crPts[i][0] - crPts[i-1][0]
		dy := crPts[i][1] - crPts[i-1][1]
		arcLen[i] = arcLen[i-1] + math.Sqrt(dx*dx+dy*dy)
	}
	totalLen := arcLen[nSamples]
	if totalLen == 0 {
		return nil
	}

	// prevTravelled is the carry-over from previous segments.
	prevTravelled := s.travelled
	s.travelled += totalLen

	// First dab in this segment is at arc-length offset = interval - prevTravelled.
	// This ensures even spacing with carry-over across segment boundaries.
	offset := interval - prevTravelled
	// Guard against floating-point drift pushing offset ≤ 0.
	for offset <= 0 {
		offset += interval
	}

	var positions [][2]float64
	for offset <= totalLen {
		pt := crArcLengthLookup(arcLen[:], crPts[:], offset)
		positions = append(positions, pt)
		offset += interval
	}

	// Correct s.travelled to reflect the distance since the last placed dab.
	if len(positions) > 0 {
		lastDabOffset := interval - prevTravelled + float64(len(positions)-1)*interval
		s.travelled = totalLen - lastDabOffset
	}

	return positions
}

// catmullRomPoint evaluates the standard uniform Catmull-Rom spline at parameter
// t ∈ [0, 1] for the segment p1→p2 with tangent controls p0 and p3.
func catmullRomPoint(p0, p1, p2, p3 [2]float64, t float64) [2]float64 {
	t2, t3 := t*t, t*t*t
	return [2]float64{
		0.5 * ((2 * p1[0]) + (-p0[0]+p2[0])*t + (2*p0[0]-5*p1[0]+4*p2[0]-p3[0])*t2 + (-p0[0]+3*p1[0]-3*p2[0]+p3[0])*t3),
		0.5 * ((2 * p1[1]) + (-p0[1]+p2[1])*t + (2*p0[1]-5*p1[1]+4*p2[1]-p3[1])*t2 + (-p0[1]+3*p1[1]-3*p2[1]+p3[1])*t3),
	}
}

// crArcLengthLookup returns the point on the sampled CR curve at the given
// arc-length s by binary-search into arcLen and linear interpolation.
func crArcLengthLookup(arcLen []float64, crPts [][2]float64, s float64) [2]float64 {
	n := len(arcLen) - 1
	lo, hi := 0, n
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if arcLen[mid] <= s {
			lo = mid
		} else {
			hi = mid
		}
	}
	segLen := arcLen[hi] - arcLen[lo]
	if segLen <= 0 {
		return crPts[lo]
	}
	frac := (s - arcLen[lo]) / segLen
	return [2]float64{
		crPts[lo][0] + (crPts[hi][0]-crPts[lo][0])*frac,
		crPts[lo][1] + (crPts[hi][1]-crPts[lo][1])*frac,
	}
}
