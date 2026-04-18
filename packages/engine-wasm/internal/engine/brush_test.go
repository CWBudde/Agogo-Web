package engine

import (
	"math"
	"testing"
)

// --- Integration tests ---

func TestPaintStroke_LayerModified(t *testing.T) {
	const w, h = 200, 200
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("paint-test", "Paint", w, h)
	layer := NewPixelLayer("Paint Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	layerID := layer.ID()
	doc.ActiveLayerID = layerID
	inst.manager.Create(doc)

	// Helper: get pixels from the stored document layer.
	storedPixels := func() []byte {
		d := inst.manager.activeMut()
		if d == nil {
			t.Fatal("no active document")
		}
		l := findPixelLayer(d, layerID)
		if l == nil {
			t.Fatal("layer not found in stored document")
		}
		return l.Pixels
	}

	// All pixels must start fully transparent.
	for i, v := range storedPixels() {
		if i%4 == 3 && v != 0 {
			t.Fatalf("pixel[%d] alpha = %d before stroke, want 0", i/4, v)
		}
	}

	brush := BrushParams{Size: 30, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	cx, cy := float64(w/2), float64(h/2)

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: cx + 50, Y: cy, Pressure: 1.0})
	inst.handleEndPaintStroke()

	// At least one pixel must have non-zero alpha after the stroke.
	painted := false
	for i, v := range storedPixels() {
		if i%4 == 3 && v != 0 {
			painted = true
			break
		}
	}
	if !painted {
		t.Fatal("no pixels were painted after paint stroke")
	}

	// Undo must restore all pixels to transparent.
	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	inst.manager.activeMut().ContentVersion++

	for i, v := range storedPixels() {
		if i%4 == 3 && v != 0 {
			t.Fatalf("pixel[%d] alpha = %d after undo, want 0", i/4, v)
		}
	}
}

func TestPaintStroke_NilLayerIsNoop(t *testing.T) {
	// instance with no document — all three handlers must be no-ops
	inst := &instance{}
	inst.manager = newDocumentManager()
	inst.history = newHistoryStack(defaultHistoryMax)

	brush := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{0, 0, 0, 255}}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 50, Y: 50, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 60, Y: 50, Pressure: 1.0})
	inst.handleEndPaintStroke()
	// If we get here without panic: pass
}

func newMixerTestInstance(t *testing.T, w, h int) (*instance, *PixelLayer, *PixelLayer) {
	t.Helper()
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("mixer-test", "Mixer", w, h)
	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)
	storedDoc := inst.manager.activeMut()
	if storedDoc == nil {
		t.Fatal("stored mixer test document missing")
	}
	storedBackground := findPixelLayer(storedDoc, background.ID())
	storedActive := findPixelLayer(storedDoc, active.ID())
	if storedBackground == nil || storedActive == nil {
		t.Fatal("stored mixer test layers missing")
	}
	return inst, storedBackground, storedActive
}

func fillLayerSolid(layer *PixelLayer, color [4]uint8) {
	for i := 0; i < len(layer.Pixels); i += 4 {
		layer.Pixels[i] = color[0]
		layer.Pixels[i+1] = color[1]
		layer.Pixels[i+2] = color[2]
		layer.Pixels[i+3] = color[3]
	}
}

func fillLayerCoordinatePattern(layer *PixelLayer) {
	for y := 0; y < layer.Bounds.H; y++ {
		for x := 0; x < layer.Bounds.W; x++ {
			idx := (y*layer.Bounds.W + x) * 4
			layer.Pixels[idx] = uint8((x*17 + 11) % 251)
			layer.Pixels[idx+1] = uint8((y*29 + 7) % 251)
			layer.Pixels[idx+2] = uint8((x*13 + y*5 + 19) % 251)
			layer.Pixels[idx+3] = 255
		}
	}
}

func layerPixelAt(layer *PixelLayer, x, y int) [4]uint8 {
	idx := (y*layer.Bounds.W + x) * 4
	return [4]uint8{
		layer.Pixels[idx],
		layer.Pixels[idx+1],
		layer.Pixels[idx+2],
		layer.Pixels[idx+3],
	}
}

func maxChannelInRect(layer *PixelLayer, x0, y0, x1, y1, channel int) uint8 {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > layer.Bounds.W {
		x1 = layer.Bounds.W
	}
	if y1 > layer.Bounds.H {
		y1 = layer.Bounds.H
	}
	var best uint8
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			value := layer.Pixels[(y*layer.Bounds.W+x)*4+channel]
			if value > best {
				best = value
			}
		}
	}
	return best
}

func avgChannelInRect(layer *PixelLayer, x0, y0, x1, y1, channel int) float64 {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > layer.Bounds.W {
		x1 = layer.Bounds.W
	}
	if y1 > layer.Bounds.H {
		y1 = layer.Bounds.H
	}
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	var sum float64
	var count float64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			sum += float64(layer.Pixels[(y*layer.Bounds.W+x)*4+channel])
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / count
}

func rectHasPixel(layer *PixelLayer, x0, y0, x1, y1 int, fn func([4]uint8) bool) bool {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > layer.Bounds.W {
		x1 = layer.Bounds.W
	}
	if y1 > layer.Bounds.H {
		y1 = layer.Bounds.H
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if fn(layerPixelAt(layer, x, y)) {
				return true
			}
		}
	}
	return false
}

func TestMixerBrushStroke_SamplesMergedCanvasColor(t *testing.T) {
	const w, h = 32, 32
	inst, background, active := newMixerTestInstance(t, w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y*w + x) * 4
			if (x+y)%2 == 0 {
				background.Pixels[idx] = 255
			} else {
				background.Pixels[idx+2] = 255
			}
			background.Pixels[idx+3] = 255
		}
	}

	brush := BrushParams{
		Size:         12,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		MixerBrush:   true,
		MixerMix:     1.0,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 16, Y: 16, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 24, Y: 16, Pressure: 1.0})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after mixer stroke")
	}
	if !rectHasPixel(layer, 18, 10, 28, 22, func(px [4]uint8) bool {
		return px[0] >= 50 && px[2] >= 50 && px[1] <= 24 && px[3] > 0
	}) {
		t.Fatal("expected a later mixer dab to contain both red and blue from the checkerboard footprint")
	}
}

func TestMixerBrushStroke_PersistsReservoirAcrossStrokes(t *testing.T) {
	const w, h = 24, 24
	inst, background, active := newMixerTestInstance(t, w, h)
	fillLayerSolid(background, [4]uint8{255, 0, 0, 255})

	brush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		MixerBrush:   true,
		MixerWetness: 1.0,
		MixerLoad:    1.0,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 8, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 14, Y: 12, Pressure: 1.0})
	inst.handleEndPaintStroke()

	fillLayerSolid(background, [4]uint8{0, 0, 0, 0})

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 19, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after persistent mixer stroke")
	}
	maxRed := maxChannelInRect(layer, 15, 7, 24, 17, 0)
	maxBlue := maxChannelInRect(layer, 15, 7, 24, 17, 2)
	maxAlpha := maxChannelInRect(layer, 15, 7, 24, 17, 3)
	if maxAlpha == 0 || maxRed <= maxBlue {
		t.Fatalf("persistent stroke region max RGBA = (%d,%d,%d), want visible carried-over red contamination", maxRed, maxBlue, maxAlpha)
	}
}

func TestMixerBrushResetState_CleansReservoir(t *testing.T) {
	const w, h = 24, 24
	inst, background, active := newMixerTestInstance(t, w, h)
	fillLayerSolid(background, [4]uint8{255, 0, 0, 255})

	brush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 255, 255},
		MixerBrush:   true,
		MixerWetness: 1.0,
		MixerLoad:    1.0,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 8, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 14, Y: 12, Pressure: 1.0})
	inst.handleEndPaintStroke()

	fillLayerSolid(background, [4]uint8{0, 0, 0, 0})
	inst.resetMixerBrushState()

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 19, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after cleaned mixer stroke")
	}
	maxAlpha := maxChannelInRect(layer, 15, 7, 24, 17, 3)
	if maxAlpha == 0 || !rectHasPixel(layer, 15, 7, 24, 17, func(px [4]uint8) bool {
		return px[2] >= 80 && px[2] > px[0]+20 && px[2] > px[1]+20 && px[3] > 0
	}) {
		t.Fatalf("expected a blue-dominant painted pixel in the cleaned-stroke region, max alpha = %d", maxAlpha)
	}
}

func TestMixerBrushWetness_IncreasesPickup(t *testing.T) {
	runStroke := func(wetness float64) [4]uint8 {
		inst, background, active := newMixerTestInstance(t, 24, 24)
		fillLayerSolid(background, [4]uint8{255, 0, 0, 255})
		brush := BrushParams{
			Size:         10,
			Hardness:     1.0,
			Flow:         1.0,
			Color:        [4]uint8{0, 0, 0, 255},
			MixerBrush:   true,
			MixerWetness: wetness,
			MixerLoad:    1.0,
			SampleMerged: true,
		}
		inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 8, Y: 12, Pressure: 1.0, Brush: brush})
		inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 14, Y: 12, Pressure: 1.0})
		inst.handleEndPaintStroke()
		layer := findPixelLayer(inst.manager.activeMut(), active.ID())
		if layer == nil {
			t.Fatal("active layer not found after wetness test stroke")
		}
		return [4]uint8{
			maxChannelInRect(layer, 9, 7, 18, 17, 0),
			maxChannelInRect(layer, 9, 7, 18, 17, 1),
			maxChannelInRect(layer, 9, 7, 18, 17, 2),
			maxChannelInRect(layer, 9, 7, 18, 17, 3),
		}
	}

	low := runStroke(0.1)
	high := runStroke(1.0)
	if high[0] <= low[0]+25 {
		t.Fatalf("high-wetness red channel = %d, low-wetness red channel = %d, want substantially more pickup", high[0], low[0])
	}
}

func TestMixerBrushStroke_SampleMergedControlsPickup(t *testing.T) {
	runStroke := func(sampleMerged bool) [4]uint8 {
		inst, background, active := newMixerTestInstance(t, 24, 24)
		fillLayerSolid(background, [4]uint8{255, 0, 0, 255})
		brush := BrushParams{
			Size:         10,
			Hardness:     1.0,
			Flow:         1.0,
			Color:        [4]uint8{0, 0, 0, 255},
			MixerBrush:   true,
			MixerWetness: 1.0,
			MixerLoad:    1.0,
			SampleMerged: sampleMerged,
		}
		inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 8, Y: 12, Pressure: 1.0, Brush: brush})
		inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 14, Y: 12, Pressure: 1.0})
		inst.handleEndPaintStroke()
		layer := findPixelLayer(inst.manager.activeMut(), active.ID())
		if layer == nil {
			t.Fatal("active layer not found after sample-merged test stroke")
		}
		return [4]uint8{
			maxChannelInRect(layer, 9, 7, 18, 17, 0),
			maxChannelInRect(layer, 9, 7, 18, 17, 1),
			maxChannelInRect(layer, 9, 7, 18, 17, 2),
			maxChannelInRect(layer, 9, 7, 18, 17, 3),
		}
	}

	notMerged := runStroke(false)
	merged := runStroke(true)
	if merged[0] <= notMerged[0]+25 {
		t.Fatalf("sampleMerged red channel = %d, active-layer-only red channel = %d, want more pickup when sampling merged", merged[0], notMerged[0])
	}
}

func TestMixerBrushStroke_DirectionalBristleStreakingSeparatesBands(t *testing.T) {
	const w, h = 40, 40
	inst, background, active := newMixerTestInstance(t, w, h)
	for y := 0; y < h; y++ {
		fill := [4]uint8{255, 0, 0, 255}
		if y >= h/2 {
			fill = [4]uint8{0, 0, 255, 255}
		}
		for x := 0; x < w; x++ {
			idx := (y*w + x) * 4
			background.Pixels[idx] = fill[0]
			background.Pixels[idx+1] = fill[1]
			background.Pixels[idx+2] = fill[2]
			background.Pixels[idx+3] = fill[3]
		}
	}

	brush := BrushParams{
		Size:         18,
		Hardness:     0.9,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		MixerBrush:   true,
		MixerWetness: 1.0,
		MixerLoad:    1.0,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 10, Y: 20, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 30, Y: 20, Pressure: 1.0})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after streaking test stroke")
	}

	topRed := maxChannelInRect(layer, 22, 11, 32, 18, 0)
	topBlue := maxChannelInRect(layer, 22, 11, 32, 18, 2)
	bottomRed := maxChannelInRect(layer, 22, 22, 32, 29, 0)
	bottomBlue := maxChannelInRect(layer, 22, 22, 32, 29, 2)

	if topRed <= topBlue+20 {
		t.Fatalf("top streak max channels = red %d blue %d, want red-dominant top bristles", topRed, topBlue)
	}
	if bottomBlue <= bottomRed+20 {
		t.Fatalf("bottom streak max channels = red %d blue %d, want blue-dominant bottom bristles", bottomRed, bottomBlue)
	}
}

func TestMixerBrushStroke_EdgeAccumulationBoostsOuterBristles(t *testing.T) {
	const w, h = 40, 40
	inst, _, active := newMixerTestInstance(t, w, h)

	brush := BrushParams{
		Size:         18,
		Hardness:     0.9,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		MixerBrush:   true,
		MixerWetness: 0,
		MixerLoad:    1.0,
		SampleMerged: false,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 20, Y: 20, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after edge accumulation test stroke")
	}

	centerAlpha := avgChannelInRect(layer, 18, 18, 23, 23, 3)
	topEdgeAlpha := avgChannelInRect(layer, 18, 12, 23, 16, 3)
	bottomEdgeAlpha := avgChannelInRect(layer, 18, 24, 23, 28, 3)
	edgeAlpha := (topEdgeAlpha + bottomEdgeAlpha) * 0.5
	if edgeAlpha <= centerAlpha+4 {
		t.Fatalf("edge accumulation average alpha = %.1f, centre average alpha = %.1f, want stronger outer bristles", edgeAlpha, centerAlpha)
	}
}

func TestMixerBrushStroke_UndoRestoresPixels(t *testing.T) {
	const w, h = 24, 24
	inst, background, active := newMixerTestInstance(t, w, h)
	fillLayerSolid(background, [4]uint8{255, 0, 0, 255})

	brush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 255, 255},
		MixerBrush:   true,
		MixerWetness: 1.0,
		MixerLoad:    1.0,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 8, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 14, Y: 12, Pressure: 1.0})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after mixer stroke")
	}
	if maxAlpha := maxChannelInRect(layer, 9, 7, 18, 17, 3); maxAlpha == 0 {
		t.Fatal("expected mixer stroke to paint within the test region")
	}

	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	inst.manager.activeMut().ContentVersion++

	layer = findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after undo")
	}
	if maxAlpha := maxChannelInRect(layer, 9, 7, 18, 17, 3); maxAlpha != 0 {
		t.Fatalf("undo region alpha = %d, want transparent after undo", maxAlpha)
	}
}

func TestCloneStampStroke_ClonesMergedSourcePixels(t *testing.T) {
	const w, h = 24, 24
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-test", "Clone", w, h)

	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	for i := 0; i < len(background.Pixels); i += 4 {
		background.Pixels[i] = 255
		background.Pixels[i+3] = 255
	}
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		CloneStamp:   true,
		CloneSourceX: 6,
		CloneSourceY: 6,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 16, Y: 16, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after clone stroke")
	}
	idx := (16*24 + 16) * 4
	if layer.Pixels[idx] < 200 || layer.Pixels[idx+1] != 0 || layer.Pixels[idx+2] != 0 {
		t.Fatalf("cloned center pixel = RGBA(%d,%d,%d,%d), want red copied from source", layer.Pixels[idx], layer.Pixels[idx+1], layer.Pixels[idx+2], layer.Pixels[idx+3])
	}
	if layer.Pixels[idx+3] < 200 {
		t.Fatalf("cloned alpha = %d, want opaque", layer.Pixels[idx+3])
	}
}

func TestCloneStampStroke_AlignedModePersistsSourceOffsetAcrossStrokes(t *testing.T) {
	const w, h = 32, 24
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-aligned", "Clone Aligned", w, h)

	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	fillLayerCoordinatePattern(background)
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         1,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		CloneStamp:   true,
		CloneSourceX: 4,
		CloneSourceY: 5,
		CloneAligned: true,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 14, Y: 12, Pressure: 1.0})
	inst.handleEndPaintStroke()

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 20, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after aligned clone strokes")
	}
	got := layerPixelAt(layer, 20, 12)
	want := layerPixelAt(background, 12, 5)
	if got != want {
		t.Fatalf("aligned clone pixel = %v, want continued source pixel %v", got, want)
	}
}

func TestCloneStampStroke_NonAlignedModeRestartsFromSourcePoint(t *testing.T) {
	const w, h = 32, 24
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-nonaligned", "Clone Non-Aligned", w, h)

	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	fillLayerCoordinatePattern(background)
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         1,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		CloneStamp:   true,
		CloneSourceX: 4,
		CloneSourceY: 5,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 14, Y: 12, Pressure: 1.0})
	inst.handleEndPaintStroke()

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 20, Y: 12, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after non-aligned clone strokes")
	}
	got := layerPixelAt(layer, 20, 12)
	want := layerPixelAt(background, 4, 5)
	if got != want {
		t.Fatalf("non-aligned clone pixel = %v, want restarted source pixel %v", got, want)
	}
}

func TestCloneStampStroke_CanCloneFromSelectedHistoryState(t *testing.T) {
	const w, h = 24, 24
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-history", "Clone History", w, h)
	layer := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)

	red := BrushParams{Size: 6, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	blue := BrushParams{Size: 6, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{0, 0, 255, 255}}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 6, Y: 6, Pressure: 1.0, Brush: red})
	inst.handleEndPaintStroke()
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 6, Y: 6, Pressure: 1.0, Brush: blue})
	inst.handleEndPaintStroke()

	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	inst.manager.activeMut().ContentVersion++

	clone := BrushParams{
		Size:            1,
		Hardness:        1.0,
		Flow:            1.0,
		Color:           [4]uint8{0, 0, 0, 255},
		CloneStamp:      true,
		CloneSourceX:    6,
		CloneSourceY:    6,
		CloneHistory:    true,
		CloneHistoryIdx: 2,
		SampleMerged:    true,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 16, Y: 6, Pressure: 1.0, Brush: clone})
	inst.handleEndPaintStroke()

	painted := findPixelLayer(inst.manager.activeMut(), layer.ID())
	if painted == nil {
		t.Fatal("layer not found after clone-from-history stroke")
	}
	got := layerPixelAt(painted, 16, 6)
	if got[2] < 200 || got[0] > 60 {
		t.Fatalf("clone-from-history pixel = %v, want blue copied from selected history state", got)
	}
}

func TestCloneStampStroke_OpacityAndLoadControlsAffectDeposit(t *testing.T) {
	const w, h = 28, 20
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-load", "Clone Load", w, h)

	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	fillLayerSolid(background, [4]uint8{255, 0, 0, 255})
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         1,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		CloneStamp:   true,
		CloneSourceX: 4,
		CloneSourceY: 4,
		CloneOpacity: 0.5,
		CloneLoad:    0.35,
		SampleMerged: true,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 10, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 16, Y: 10, Pressure: 1.0})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer not found after opacity/load clone stroke")
	}
	first := layerPixelAt(layer, 12, 10)
	second := layerPixelAt(layer, 16, 10)
	if first[0] < 200 || first[3] == 0 || first[3] >= 255 {
		t.Fatalf("first clone dab = %v, want red source color with partial alpha from clone opacity control", first)
	}
	if second[3] >= first[3] {
		t.Fatalf("second clone dab alpha = %d, want less than first dab alpha %d after load decay", second[3], first[3])
	}
}

func TestCloneStampStroke_UsesDocumentSpaceOffsetsForTranslatedLayerSource(t *testing.T) {
	const w, h = 40, 28
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-translated", "Clone Translated", w, h)
	active := NewPixelLayer("Paint", LayerBounds{X: 8, Y: 6, W: 16, H: 12}, make([]byte, 16*12*4))
	fillLayerCoordinatePattern(active)
	doc.LayerRoot.SetChildren([]LayerNode{active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         1,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		CloneStamp:   true,
		CloneSourceX: 10,
		CloneSourceY: 8,
		SampleMerged: false,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 18, Y: 14, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("translated active layer missing after clone stroke")
	}
	got := layerPixelAt(layer, 10, 8)
	want := layerPixelAt(layer, 2, 2)
	if got != want {
		t.Fatalf("translated-layer clone pixel = %v, want source pixel %v from document-space offset", got, want)
	}
}

func TestCloneStampStroke_SubpixelSourceUsesBilinearSampling(t *testing.T) {
	const w, h = 24, 16
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("clone-bilinear", "Clone Bilinear", w, h)
	background := NewPixelLayer("Background", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	active := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y*w + x) * 4
			if x <= 5 {
				background.Pixels[idx] = 255
			} else {
				background.Pixels[idx+2] = 255
			}
			background.Pixels[idx+3] = 255
		}
	}
	doc.LayerRoot.SetChildren([]LayerNode{background, active})
	doc.ActiveLayerID = active.ID()
	inst.manager.Create(doc)

	brush := BrushParams{
		Size:         1,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		CloneStamp:   true,
		CloneSourceX: 5.5,
		CloneSourceY: 5,
		SampleMerged: true,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 10, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	layer := findPixelLayer(inst.manager.activeMut(), active.ID())
	if layer == nil {
		t.Fatal("active layer missing after bilinear clone stroke")
	}
	got := layerPixelAt(layer, 12, 10)
	if got[0] < 90 || got[2] < 90 || got[0] > 180 || got[2] > 180 {
		t.Fatalf("subpixel clone pixel = %v, want blended red/blue sample from bilinear source lookup", got)
	}
}

func TestHistoryBrushStroke_RestoresPreviousHistoryState(t *testing.T) {
	const w, h = 24, 24
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("history-test", "History", w, h)
	layer := NewPixelLayer("Paint", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)

	red := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	blue := BrushParams{Size: 10, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{0, 0, 255, 255}}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: red})
	inst.handleEndPaintStroke()
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: blue})
	inst.handleEndPaintStroke()

	historyBrush := BrushParams{
		Size:         10,
		Hardness:     1.0,
		Flow:         1.0,
		Color:        [4]uint8{0, 0, 0, 255},
		HistoryBrush: true,
		SampleMerged: true,
	}

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 12, Pressure: 1.0, Brush: historyBrush})
	inst.handleEndPaintStroke()

	painted := findPixelLayer(inst.manager.activeMut(), layer.ID())
	if painted == nil {
		t.Fatal("layer not found after history brush stroke")
	}
	idx := (12*24 + 12) * 4
	if painted.Pixels[idx] < 200 || painted.Pixels[idx+1] != 0 || painted.Pixels[idx+2] != 0 {
		t.Fatalf("history-brushed pixel = RGBA(%d,%d,%d,%d), want previous red state", painted.Pixels[idx], painted.Pixels[idx+1], painted.Pixels[idx+2], painted.Pixels[idx+3])
	}
}

func TestPaintDab_HardBrush_CenterPixelFilled(t *testing.T) {
	bounds := LayerBounds{X: 0, Y: 0, W: 20, H: 20}
	layer := NewPixelLayer("test", bounds, make([]byte, 20*20*4))

	params := BrushParams{
		Size:     10.0,
		Hardness: 1.0,
		Flow:     1.0,
		Color:    [4]uint8{255, 0, 0, 255},
	}

	PaintDab(layer, 10.0, 10.0, params, 0, 1)

	idx := (10*20 + 10) * 4
	if layer.Pixels[idx] < 200 {
		t.Errorf("center R = %d, want >= 200", layer.Pixels[idx])
	}
	if layer.Pixels[idx+3] < 200 {
		t.Errorf("center A = %d, want >= 200", layer.Pixels[idx+3])
	}
}

func TestPaintDab_SoftBrush_EdgePixelPartialAlpha(t *testing.T) {
	bounds := LayerBounds{X: 0, Y: 0, W: 40, H: 40}
	layer := NewPixelLayer("test", bounds, make([]byte, 40*40*4))

	params := BrushParams{
		Size:     20.0,
		Hardness: 0.0,
		Flow:     1.0,
		Color:    [4]uint8{0, 0, 255, 255},
	}

	PaintDab(layer, 20.0, 20.0, params, 0, 1)

	// Center should have blue
	centerIdx := (20*40 + 20) * 4
	if layer.Pixels[centerIdx+2] < 200 {
		t.Errorf("center B = %d, want > 200 for soft brush", layer.Pixels[centerIdx+2])
	}

	// Pixel outside the dab should be empty
	outerIdx := (32*40 + 20) * 4
	if layer.Pixels[outerIdx+2] != 0 {
		t.Errorf("pixel outside dab radius should be transparent, got B=%d", layer.Pixels[outerIdx+2])
	}
}

func TestPaintDab_FlowReducesOpacity(t *testing.T) {
	bounds := LayerBounds{X: 0, Y: 0, W: 20, H: 20}
	layer := NewPixelLayer("test", bounds, make([]byte, 20*20*4))

	params := BrushParams{
		Size:     10.0,
		Hardness: 1.0,
		Flow:     0.5,
		Color:    [4]uint8{255, 0, 0, 255},
	}

	PaintDab(layer, 10.0, 10.0, params, 0, 1)

	centerIdx := (10*20 + 10) * 4
	alpha := layer.Pixels[centerIdx+3]
	if alpha < 100 || alpha > 155 {
		t.Errorf("flow=0.5 center alpha = %d, want ~127", alpha)
	}
}

func TestBrushStrokeState_FirstPointAlwaysPlaced(t *testing.T) {
	var s brushStrokeState
	dabs := s.AddPoint(10, 10, 0.25, 20) // interval = 0.25*20 = 5px
	if len(dabs) != 1 {
		t.Fatalf("first point: want 1 dab, got %d", len(dabs))
	}
	if dabs[0] != [2]float64{10, 10} {
		t.Errorf("first dab = %v, want {10,10}", dabs[0])
	}
}

func TestBrushStrokeState_DabSpacing(t *testing.T) {
	var s brushStrokeState
	s.AddPoint(0, 0, 0.25, 20) // first dab at origin; interval = 5px

	// Move 12px right → 2 dabs (at ~5px and ~10px)
	dabs := s.AddPoint(12, 0, 0.25, 20)
	if len(dabs) != 2 {
		t.Fatalf("12px move at 5px interval: want 2 dabs, got %d", len(dabs))
	}
	if math.Abs(dabs[0][0]-5) > 0.5 {
		t.Errorf("dab[0].x = %.2f, want ~5", dabs[0][0])
	}
	if math.Abs(dabs[1][0]-10) > 0.5 {
		t.Errorf("dab[1].x = %.2f, want ~10", dabs[1][0])
	}
}

func TestBrushStrokeState_ShortMoveProducesNoDab(t *testing.T) {
	var s brushStrokeState
	s.AddPoint(0, 0, 0.25, 20) // interval = 5px
	dabs := s.AddPoint(2, 0, 0.25, 20)
	if len(dabs) != 0 {
		t.Fatalf("2px move: want 0 dabs, got %d", len(dabs))
	}
}

func TestBrushStrokeState_CarryOverAcrossSegments(t *testing.T) {
	// Two successive 4px moves with a 5px interval.
	// After first move (4px): 0 dabs, travelled=4.
	// After second move (4px total=8px): 1 dab at x≈5, travelled=3.
	var s brushStrokeState
	s.AddPoint(0, 0, 0.25, 20) // first dab; interval=5
	d1 := s.AddPoint(4, 0, 0.25, 20)
	if len(d1) != 0 {
		t.Fatalf("first 4px segment: want 0 dabs, got %d", len(d1))
	}
	d2 := s.AddPoint(8, 0, 0.25, 20)
	if len(d2) != 1 {
		t.Fatalf("second 4px segment: want 1 dab, got %d", len(d2))
	}
	if math.Abs(d2[0][0]-5) > 0.5 {
		t.Errorf("carry-over dab x = %.2f, want ~5", d2[0][0])
	}
}

func TestCatmullRomPoint_EndpointInterpolation(t *testing.T) {
	// CR at t=0 must return p1 and at t=1 must return p2 exactly.
	p0 := [2]float64{0, 0}
	p1 := [2]float64{10, 0}
	p2 := [2]float64{20, 5}
	p3 := [2]float64{30, 0}

	got0 := catmullRomPoint(p0, p1, p2, p3, 0)
	if math.Abs(got0[0]-p1[0]) > 1e-9 || math.Abs(got0[1]-p1[1]) > 1e-9 {
		t.Errorf("t=0: got %v, want %v", got0, p1)
	}
	got1 := catmullRomPoint(p0, p1, p2, p3, 1)
	if math.Abs(got1[0]-p2[0]) > 1e-9 || math.Abs(got1[1]-p2[1]) > 1e-9 {
		t.Errorf("t=1: got %v, want %v", got1, p2)
	}
}

func TestCatmullRomPoint_CollinearIsLinear(t *testing.T) {
	// Collinear control points → CR degenerates to a straight line.
	p0 := [2]float64{0, 0}
	p1 := [2]float64{10, 0}
	p2 := [2]float64{20, 0}
	p3 := [2]float64{30, 0}

	for _, tc := range []struct{ t, wantX float64 }{
		{0.25, 12.5},
		{0.5, 15},
		{0.75, 17.5},
	} {
		got := catmullRomPoint(p0, p1, p2, p3, tc.t)
		if math.Abs(got[0]-tc.wantX) > 1e-9 || math.Abs(got[1]) > 1e-9 {
			t.Errorf("t=%.2f: got (%.4f, %.4f), want (%.4f, 0)", tc.t, got[0], got[1], tc.wantX)
		}
	}
}

func TestPaintDab_BlendModeDoesNotPanic(t *testing.T) {
	// Verify PaintDab runs without panic when a blend mode is set.
	bounds := LayerBounds{X: 0, Y: 0, W: 20, H: 20}
	layer := NewPixelLayer("test", bounds, make([]byte, 20*20*4))
	params := BrushParams{
		Size:      10.0,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     [4]uint8{255, 0, 0, 255},
		BlendMode: "multiply",
	}
	PaintDab(layer, 10.0, 10.0, params, 0, 1) // must not panic
}

func TestApplyTilt_NoTilt(t *testing.T) {
	az, sq := applyTilt(0, 0)
	if az != 0 || sq != 1 {
		t.Errorf("no tilt: got azimuth=%.4f squish=%.4f, want 0 1", az, sq)
	}
}

func TestApplyTilt_VerticalPen(t *testing.T) {
	// tiltMag → 0 means pen is perfectly upright → squish ≈ sin(90°) = 1
	az, sq := applyTilt(0, 1) // nearly upright
	if math.Abs(sq-math.Sin(89*math.Pi/180)) > 0.01 {
		t.Errorf("near-upright squish = %.4f, want ≈ sin(89°)=%.4f", sq, math.Sin(89*math.Pi/180))
	}
	_ = az
}

func TestApplyTilt_HorizontalPen_MinSquish(t *testing.T) {
	// tiltMag = 90 → altitude = 0° → squish = sin(0°) = 0 → clamp to minSquish
	az, sq := applyTilt(90, 0)
	if sq > 0.1 {
		t.Errorf("flat pen squish = %.4f, want minSquish (0.05)", sq)
	}
	if math.Abs(az) > 0.001 {
		t.Errorf("flat pen leaning along X: azimuth = %.4f, want ≈0", az)
	}
}

func TestApplyTilt_AzimuthDirection(t *testing.T) {
	// tiltY > 0 only → stylus leans toward +Y → azimuth = π/2
	az, _ := applyTilt(0, 45)
	if math.Abs(az-math.Pi/2) > 0.01 {
		t.Errorf("tiltY=45: azimuth = %.4f, want π/2=%.4f", az, math.Pi/2)
	}
}

func TestPaintDab_TiltedDabIsElliptical(t *testing.T) {
	// A tilted (squish<1) dab must paint fewer pixels than an untilted one because
	// the ellipse minor axis is squished. Sample column coverage at the dab centre row.
	const sz = 60
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}

	// Untilted round dab.
	round := NewPixelLayer("round", bounds, make([]byte, sz*sz*4))
	PaintDab(round, float64(sz/2), float64(sz/2), BrushParams{Size: 40, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}}, 0, 1)

	// Maximally squished dab (squish = minSquish ≈ 0.05, azimuth = 0 → major axis along X).
	squished := NewPixelLayer("squished", bounds, make([]byte, sz*sz*4))
	PaintDab(squished, float64(sz/2), float64(sz/2), BrushParams{Size: 40, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}}, 0, 0.05)

	// Count painted pixels in both layers.
	countPainted := func(l *PixelLayer) int {
		n := 0
		for i := 3; i < len(l.Pixels); i += 4 {
			if l.Pixels[i] > 0 {
				n++
			}
		}
		return n
	}

	roundN := countPainted(round)
	squishedN := countPainted(squished)
	if squishedN >= roundN {
		t.Errorf("tilted dab painted %d px, round dab painted %d px — expected fewer for squished", squishedN, roundN)
	}
}

func TestPaintDab_WetEdges_TransparentCentre(t *testing.T) {
	// Wet edges: centre pixel should have very low alpha; ring near edge higher.
	const sz = 60
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	params := BrushParams{
		Size:     50.0,
		Hardness: 0.0,
		Flow:     1.0,
		Color:    [4]uint8{0, 0, 255, 255},
		WetEdges: true,
	}
	cx, cy := float64(sz/2), float64(sz/2)
	PaintDab(layer, cx, cy, params, 0, 1)

	// Centre pixel must be mostly transparent.
	centreIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centreIdx+3] > 30 {
		t.Errorf("wet-edges centre alpha = %d, want ≤ 30 (transparent centre)", layer.Pixels[centreIdx+3])
	}

	// A pixel at ~75% radius (ring position) must be more opaque than centre.
	ringX := int(cx + 25*0.75) // radius=25, peak at 75%
	ringIdx := (sz/2*sz + ringX) * 4
	if layer.Pixels[ringIdx+3] <= layer.Pixels[centreIdx+3] {
		t.Errorf("wet-edges ring alpha (%d) should exceed centre alpha (%d)",
			layer.Pixels[ringIdx+3], layer.Pixels[centreIdx+3])
	}
}

// ── Stabilizer ────────────────────────────────────────────────────────────────

func TestStabilizer_ZeroLag_Passthrough(t *testing.T) {
	s := newStabilizer(0)
	for _, pt := range [][2]float64{{10, 20}, {30, 40}, {50, 60}} {
		ox, oy := s.Push(pt[0], pt[1])
		if ox != pt[0] || oy != pt[1] {
			t.Fatalf("lag=0: got (%.1f,%.1f), want (%.1f,%.1f)", ox, oy, pt[0], pt[1])
		}
	}
}

func TestStabilizer_FirstPushReturnsInput(t *testing.T) {
	s := newStabilizer(5)
	ox, oy := s.Push(100, 200)
	if ox != 100 || oy != 200 {
		t.Fatalf("first push: got (%.1f,%.1f), want (100,200)", ox, oy)
	}
}

func TestStabilizer_AveragesBuffer(t *testing.T) {
	s := newStabilizer(3)
	s.Push(0, 0)
	s.Push(6, 6)
	ox, oy := s.Push(3, 3) // buf = [0,6,3] avg = 3
	if math.Abs(ox-3) > 0.01 || math.Abs(oy-3) > 0.01 {
		t.Fatalf("3-point average: got (%.4f,%.4f), want (3,3)", ox, oy)
	}
}

func TestStabilizer_SmoothsPath(t *testing.T) {
	// Feed alternating y=60/40 jitter around 50. After the buffer fills the
	// mean output should be much closer to 50 than the ±10 raw values.
	s := newStabilizer(8)
	raw := []float64{60, 40, 60, 40, 60, 40, 60, 40, 60, 40}
	var sumDev float64
	n := 0
	for i, v := range raw {
		_, oy := s.Push(0, v)
		if i >= 8 { // only measure once buffer is full
			sumDev += math.Abs(oy - 50)
			n++
		}
	}
	if n == 0 {
		t.Fatal("not enough samples to evaluate smoothing")
	}
	meanDev := sumDev / float64(n)
	if meanDev >= 5 {
		t.Errorf("mean deviation after buffer full = %.2f, want < 5 (raw jitter = 10)", meanDev)
	}
}

// ── Scatter ───────────────────────────────────────────────────────────────────

func TestApplyScatter_ZeroIsIdentity(t *testing.T) {
	p := BrushParams{Size: 40, Scatter: 0}
	for range 20 {
		ox, oy := applyScatter(100, 200, p)
		if ox != 100 || oy != 200 {
			t.Fatalf("scatter=0: got (%.2f, %.2f), want (100, 200)", ox, oy)
		}
	}
}

func TestApplyScatter_OffsetWithinRadius(t *testing.T) {
	p := BrushParams{Size: 40, Scatter: 1.0} // maxR = 40
	for range 500 {
		ox, oy := applyScatter(0, 0, p)
		dist := math.Sqrt(ox*ox + oy*oy)
		if dist > p.Size*p.Scatter+1e-9 {
			t.Fatalf("scatter offset dist=%.4f exceeds maxR=%.4f", dist, p.Size*p.Scatter)
		}
	}
}

func TestApplyScatter_CoversDisc(t *testing.T) {
	// With scatter=1 and many samples, expect offsets in all four quadrants.
	p := BrushParams{Size: 40, Scatter: 1.0}
	q := [4]int{}
	for range 1000 {
		ox, oy := applyScatter(0, 0, p)
		switch {
		case ox >= 0 && oy >= 0:
			q[0]++
		case ox < 0 && oy >= 0:
			q[1]++
		case ox < 0 && oy < 0:
			q[2]++
		default:
			q[3]++
		}
	}
	for i, cnt := range q {
		if cnt < 50 {
			t.Errorf("quadrant %d has only %d samples out of 1000 — scatter not covering disc", i, cnt)
		}
	}
}

// ── Pencil / Auto-erase ───────────────────────────────────────────────────────

// newPencilTestInstance creates a minimal instance with a single white-filled
// pixel layer ready for pencil/auto-erase tests.
func newPencilTestInstance(t *testing.T, w, h int, fillColor [4]uint8) (*instance, *PixelLayer, string) {
	t.Helper()
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("pencil-test", "Pencil", w, h)
	pixels := make([]byte, w*h*4)
	// Pre-fill every pixel with fillColor.
	for i := 0; i < w*h; i++ {
		pixels[i*4] = fillColor[0]
		pixels[i*4+1] = fillColor[1]
		pixels[i*4+2] = fillColor[2]
		pixels[i*4+3] = fillColor[3]
	}
	layer := NewPixelLayer("Pencil Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, pixels)
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)
	return inst, layer, layer.ID()
}

func TestAutoErase_SwitchedToBackgroundWhenFGMatches(t *testing.T) {
	// Layer pre-filled with red (the foreground color).
	// Auto-erase should switch to the background color (blue) for the stroke.
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, red)
	inst.backgroundColor = blue

	cx, cy := float64(w/2), float64(h/2)
	brush := BrushParams{
		Size:      10,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     red, // foreground
		AutoErase: true,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	idx := (int(cy)*w + int(cx)) * 4
	if layer.Pixels[idx] != 0 || layer.Pixels[idx+2] < 200 {
		t.Errorf("auto-erase: center R=%d B=%d, want R≈0 B≈255 (painted blue)", layer.Pixels[idx], layer.Pixels[idx+2])
	}
}

func TestAutoErase_NoPaintSwitchWhenFGDiffers(t *testing.T) {
	// Layer pre-filled with green — does NOT match the red foreground.
	// Auto-erase should NOT switch; stroke paints red.
	const w, h = 50, 50
	green := [4]uint8{0, 255, 0, 255}
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, green)
	inst.backgroundColor = blue

	cx, cy := float64(w/2), float64(h/2)
	brush := BrushParams{
		Size:      10,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     red, // foreground
		AutoErase: true,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	idx := (int(cy)*w + int(cx)) * 4
	if layer.Pixels[idx] < 200 {
		t.Errorf("no auto-erase: center R=%d, want ≥200 (painted red)", layer.Pixels[idx])
	}
}

func TestAutoErase_FalseNeverSwitches(t *testing.T) {
	// AutoErase=false: even if start pixel matches FG, stroke uses FG (red).
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, red)
	inst.backgroundColor = blue

	cx, cy := float64(w/2), float64(h/2)
	brush := BrushParams{
		Size:      10,
		Hardness:  1.0,
		Flow:      1.0,
		Color:     red,
		AutoErase: false,
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleEndPaintStroke()

	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	idx := (int(cy)*w + int(cx)) * 4
	// Should still be (approximately) red at center.
	if layer.Pixels[idx] < 200 {
		t.Errorf("autoErase=false: center R=%d, want ≥200 (no color switch)", layer.Pixels[idx])
	}
}

func TestPencilTool_HardnessOneIsFullyOpaque(t *testing.T) {
	// Pencil mode uses hardness=1.0. Center pixel must be fully opaque.
	const sz = 40
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	params := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	PaintDab(layer, float64(sz/2), float64(sz/2), params, 0, 1)

	idx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[idx+3] < 250 {
		t.Errorf("pencil center alpha = %d, want ~255 (hardness=1.0)", layer.Pixels[idx+3])
	}
}

// ── Eraser ────────────────────────────────────────────────────────────────────

func TestNormalErase_RemovesAlpha(t *testing.T) {
	// Paint a solid red dab, then erase the center — center alpha must drop.
	const sz = 40
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	paint := BrushParams{Size: 30, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	PaintDab(layer, float64(sz/2), float64(sz/2), paint, 0, 1)

	centerIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centerIdx+3] < 250 {
		t.Fatalf("pre-erase: center alpha = %d, want ~255", layer.Pixels[centerIdx+3])
	}

	erase := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, Erase: true}
	PaintDab(layer, float64(sz/2), float64(sz/2), erase, 0, 1)

	if layer.Pixels[centerIdx+3] > 10 {
		t.Errorf("after erase: center alpha = %d, want ~0", layer.Pixels[centerIdx+3])
	}
}

func TestNormalErase_PartialFlow(t *testing.T) {
	// flow=0.5 should reduce alpha to roughly half, not fully erase.
	const sz = 40
	bounds := LayerBounds{X: 0, Y: 0, W: sz, H: sz}
	layer := NewPixelLayer("test", bounds, make([]byte, sz*sz*4))

	paint := BrushParams{Size: 30, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	PaintDab(layer, float64(sz/2), float64(sz/2), paint, 0, 1)

	erase := BrushParams{Size: 20, Hardness: 1.0, Flow: 0.5, Erase: true}
	PaintDab(layer, float64(sz/2), float64(sz/2), erase, 0, 1)

	centerIdx := (sz/2*sz + sz/2) * 4
	alpha := layer.Pixels[centerIdx+3]
	// dst-out with src_alpha=~128: result = 255 * (1 - 128/255) ≈ 127
	if alpha < 100 || alpha > 160 {
		t.Errorf("partial erase: center alpha = %d, want ~127", alpha)
	}
}

func TestEraseBackground_MatchingColorErased(t *testing.T) {
	// Layer filled with red. Background erase with red as base color should erase.
	const sz = 40
	red := [4]uint8{255, 0, 0, 255}
	pixels := make([]byte, sz*sz*4)
	for i := range sz * sz {
		pixels[i*4] = red[0]
		pixels[i*4+1] = red[1]
		pixels[i*4+2] = red[2]
		pixels[i*4+3] = red[3]
	}
	layer := NewPixelLayer("test", LayerBounds{X: 0, Y: 0, W: sz, H: sz}, pixels)

	params := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, EraseBackground: true, EraseTolerance: 30}
	EraseBackgroundDab(layer, float64(sz/2), float64(sz/2), params, red)

	centerIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centerIdx+3] > 10 {
		t.Errorf("background erase: center alpha = %d, want ~0 (red matches base)", layer.Pixels[centerIdx+3])
	}
}

func TestEraseBackground_NonMatchingColorKept(t *testing.T) {
	// Layer filled with blue. Background erase with red base color should NOT erase blue.
	const sz = 40
	blue := [4]uint8{0, 0, 255, 255}
	red := [4]uint8{255, 0, 0, 255}
	pixels := make([]byte, sz*sz*4)
	for i := range sz * sz {
		pixels[i*4] = blue[0]
		pixels[i*4+1] = blue[1]
		pixels[i*4+2] = blue[2]
		pixels[i*4+3] = blue[3]
	}
	layer := NewPixelLayer("test", LayerBounds{X: 0, Y: 0, W: sz, H: sz}, pixels)

	params := BrushParams{Size: 20, Hardness: 1.0, Flow: 1.0, EraseBackground: true, EraseTolerance: 30}
	EraseBackgroundDab(layer, float64(sz/2), float64(sz/2), params, red)

	centerIdx := (sz/2*sz + sz/2) * 4
	if layer.Pixels[centerIdx+3] < 250 {
		t.Errorf("background erase: blue pixel alpha = %d, want ~255 (outside tolerance)", layer.Pixels[centerIdx+3])
	}
}

func TestMagicErase_ClearsMatchingPixels(t *testing.T) {
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}

	inst, _, layerID := newPencilTestInstance(t, w, h, red)

	payload := MagicErasePayload{
		X: float64(w / 2), Y: float64(h / 2),
		Tolerance:  30,
		Contiguous: true,
	}
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	if err := inst.handleMagicErase(payload, doc, layer); err != nil {
		t.Fatalf("handleMagicErase: %v", err)
	}

	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	for i := 3; i < len(layer.Pixels); i += 4 {
		if layer.Pixels[i] != 0 {
			t.Fatalf("pixel %d alpha = %d after magic erase, want 0", i/4, layer.Pixels[i])
		}
	}
}

func TestMagicErase_IsUndoable(t *testing.T) {
	const w, h = 50, 50
	red := [4]uint8{255, 0, 0, 255}
	inst, _, layerID := newPencilTestInstance(t, w, h, red)

	payload := MagicErasePayload{X: float64(w / 2), Y: float64(h / 2), Tolerance: 30, Contiguous: true}
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	if err := inst.handleMagicErase(payload, doc, layer); err != nil {
		t.Fatalf("handleMagicErase: %v", err)
	}

	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	inst.manager.activeMut().ContentVersion++

	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	for i := 3; i < len(layer.Pixels); i += 4 {
		if layer.Pixels[i] != red[3] {
			t.Fatalf("pixel %d alpha = %d after undo, want %d", i/4, layer.Pixels[i], red[3])
		}
	}
}
