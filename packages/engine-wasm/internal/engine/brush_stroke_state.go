package engine

import "math"

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
	randomState uint64
	dabCount    int
	tipResource *brushTipResource
	tipRGBA     []byte
}

func (s *brushStrokeState) initRandom(x, y float64, p BrushParams) {
	// Derive a repeatable, stroke-local seed from immutable begin-stroke input.
	// This avoids the package-global random stream while ensuring two equivalent
	// event/coalescing sequences render identically.
	seed := uint64(0x6a09e667f3bcc909)
	for _, value := range []uint64{
		math.Float64bits(x), math.Float64bits(y), math.Float64bits(p.Size),
		math.Float64bits(p.Flow), math.Float64bits(p.Angle),
	} {
		seed ^= value + 0x9e3779b97f4a7c15 + (seed << 6) + (seed >> 2)
	}
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	s.randomState = seed
	s.dabCount = 0
}

func (s *brushStrokeState) nextRandomFloat() float64 {
	// SplitMix64 is compact, deterministic, and has ample statistical quality
	// for brush jitter. Convert the upper 53 bits to an IEEE-754 unit fraction.
	s.randomState += 0x9e3779b97f4a7c15
	z := s.randomState
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	return float64(z>>11) * (1.0 / (1 << 53))
}

func (s *brushStrokeState) nextDabParams(p BrushParams, pressure, tiltX, tiltY float64) (BrushParams, dabRandomValues) {
	if s.randomState == 0 {
		s.randomState = 0x9e3779b97f4a7c15
	}
	random := dabRandomValues{
		sizeJitter:    s.nextRandomFloat(),
		opacityJitter: s.nextRandomFloat(),
		flowJitter:    s.nextRandomFloat(),
		scatterAngle:  s.nextRandomFloat(),
		scatterRadius: s.nextRandomFloat(),
	}
	effective := applyStrokeDynamics(p, pressure, tiltX, tiltY, s.dabCount, random, true)
	s.dabCount++
	return effective, random
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
