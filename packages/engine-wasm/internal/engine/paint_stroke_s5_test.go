// Phase S.5 regression tests: selection clipping for brush strokes, neutral
// pressure dynamics for mouse input, mixer rim-dab undo coverage, magic-eraser
// version/dirty-rect bookkeeping, and painting on non-pixel layers.
package engine

import (
	"math"
	"strings"
	"testing"
)

// newStrokeTestInstance builds a minimal instance with one opaque pixel layer
// filled with fillColor. Identical in spirit to newPencilTestInstance but kept
// local to the S.5 regression suite so it can pick its own document size.
func newStrokeTestInstance(t *testing.T, w, h int, fillColor [4]uint8) (*instance, string) {
	t.Helper()
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("s5-test", "S5", w, h)
	pixels := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pixels[i*4] = fillColor[0]
		pixels[i*4+1] = fillColor[1]
		pixels[i*4+2] = fillColor[2]
		pixels[i*4+3] = fillColor[3]
	}
	layer := NewPixelLayer("S5 Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, pixels)
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()
	inst.manager.Create(doc)
	return inst, layer.ID()
}

// halfSelection returns a selection whose mask is 255 for x < split, feather
// (a partial-coverage value) at x == split, and 0 beyond.
func halfSelection(w, h, split int, feather byte) *Selection {
	sel := newSelection(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x < split:
				sel.Mask[y*w+x] = 255
			case x == split:
				sel.Mask[y*w+x] = feather
			}
		}
	}
	return sel
}

// ── Task 1: strokes must clip to the active selection ────────────────────────

func TestPaintStroke_ClipsToActiveSelection(t *testing.T) {
	const w, h = 50, 50
	white := [4]uint8{255, 255, 255, 255}
	inst, layerID := newStrokeTestInstance(t, w, h, white)
	doc := inst.manager.activeMut()
	doc.Selection = halfSelection(w, h, 25, 128)

	layer := findPixelLayer(doc, layerID)
	before := append([]byte(nil), layer.Pixels...)

	// Hard black dab straddling the selection boundary at x=25.
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{
		X: 25, Y: 25, Pressure: 1,
		Brush: BrushParams{Size: 20, Hardness: 1, Flow: 1, Color: [4]uint8{0, 0, 0, 255}},
	})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end stroke: %v", err)
	}

	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	if got := layerPixelAt(layer, 20, 25); got[0] > 50 {
		t.Errorf("inside selection (20,25) = %v, want painted black", got)
	}
	if got := layerPixelAt(layer, 30, 25); got != white {
		t.Errorf("outside selection (30,25) = %v, want untouched white", got)
	}
	// Feathered boundary column (coverage 128): paint weighted ~50%.
	if got := layerPixelAt(layer, 25, 25); got[0] < 100 || got[0] > 160 {
		t.Errorf("feathered edge (25,25) R = %d, want ≈127 (coverage-weighted)", got[0])
	}

	// Undo must restore the layer byte-exactly (no corrupted undo rows).
	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("undo: %v", err)
	}
	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	for i := range before {
		if layer.Pixels[i] != before[i] {
			t.Fatalf("undo mismatch at byte %d (pixel %d,%d): got %d want %d",
				i, (i/4)%w, (i/4)/w, layer.Pixels[i], before[i])
		}
	}
}

// A dab landing on fully transparent pixels through partial selection
// coverage must keep the paint colour at full strength and only scale alpha.
// A straight per-channel lerp instead dragged RGB towards the meaningless
// colour of the transparent "before" pixels (red → dark red fringe).
func TestPaintStroke_SelectionCoverageOverTransparencyKeepsColor(t *testing.T) {
	const w, h = 40, 40
	transparent := [4]uint8{0, 0, 0, 0}
	inst, layerID := newStrokeTestInstance(t, w, h, transparent)
	doc := inst.manager.activeMut()
	// Uniform 50% selection coverage everywhere.
	sel := newSelection(w, h)
	for i := range sel.Mask {
		sel.Mask[i] = 128
	}
	doc.Selection = sel

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{
		X: 20, Y: 20, Pressure: 1,
		Brush: BrushParams{Size: 12, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}},
	})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end stroke: %v", err)
	}

	layer := findPixelLayer(inst.manager.activeMut(), layerID)
	got := layerPixelAt(layer, 20, 20)
	if got[0] < 250 {
		t.Errorf("dab centre R = %d, want ≈255 (full-strength colour, premultiplied lerp)", got[0])
	}
	if got[3] < 120 || got[3] > 136 {
		t.Errorf("dab centre alpha = %d, want ≈128 (coverage-halved)", got[3])
	}
}

func TestEraseStroke_ClipsToActiveSelection(t *testing.T) {
	const w, h = 50, 50
	white := [4]uint8{255, 255, 255, 255}
	inst, layerID := newStrokeTestInstance(t, w, h, white)
	doc := inst.manager.activeMut()
	doc.Selection = halfSelection(w, h, 25, 0)

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{
		X: 25, Y: 25, Pressure: 1,
		Brush: BrushParams{Size: 20, Hardness: 1, Flow: 1, Color: [4]uint8{0, 0, 0, 255}, Erase: true},
	})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end stroke: %v", err)
	}

	layer := findPixelLayer(inst.manager.activeMut(), layerID)
	if got := layerPixelAt(layer, 20, 25); got[3] > 30 {
		t.Errorf("inside selection (20,25) alpha = %d, want erased ≈0", got[3])
	}
	if got := layerPixelAt(layer, 30, 25); got[3] != 255 {
		t.Errorf("outside selection (30,25) alpha = %d, want untouched 255", got[3])
	}
}

func TestCloneStampStroke_ClipsToActiveSelection(t *testing.T) {
	const w, h = 60, 40
	white := [4]uint8{255, 255, 255, 255}
	inst, layerID := newStrokeTestInstance(t, w, h, white)
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	// Left 20 columns black, rest white.
	for y := 0; y < h; y++ {
		for x := 0; x < 20; x++ {
			idx := (y*w + x) * 4
			layer.Pixels[idx] = 0
			layer.Pixels[idx+1] = 0
			layer.Pixels[idx+2] = 0
		}
	}
	// Selection: only a 10x10 box at x∈[30,40) y∈[15,25).
	sel := newSelection(w, h)
	for y := 15; y < 25; y++ {
		for x := 30; x < 40; x++ {
			sel.Mask[y*w+x] = 255
		}
	}
	doc.Selection = sel

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{
		X: 35, Y: 20, Pressure: 1,
		Brush: BrushParams{
			Size: 16, Hardness: 1, Flow: 1, Color: [4]uint8{0, 0, 0, 255},
			CloneStamp: true, CloneSourceX: 5, CloneSourceY: 20, CloneOpacity: 1, CloneLoad: 1,
		},
	})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end stroke: %v", err)
	}

	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	if got := layerPixelAt(layer, 35, 20); got[0] > 80 {
		t.Errorf("inside selection (35,20) = %v, want cloned black", got)
	}
	if got := layerPixelAt(layer, 35, 27); got != white {
		t.Errorf("outside selection (35,27) = %v, want untouched white", got)
	}
}

func TestMagicErase_RespectsActiveSelection(t *testing.T) {
	const w, h = 20, 20
	red := [4]uint8{255, 0, 0, 255}
	inst, layerID := newStrokeTestInstance(t, w, h, red)
	doc := inst.manager.activeMut()
	doc.Selection = halfSelection(w, h, 10, 0)
	layer := findPixelLayer(doc, layerID)

	if err := inst.handleMagicErase(MagicErasePayload{X: 5, Y: 5, Tolerance: 0, Contiguous: false}, doc, layer); err != nil {
		t.Fatalf("magic erase: %v", err)
	}
	if got := layerPixelAt(layer, 5, 5); got[3] != 0 {
		t.Errorf("inside selection (5,5) alpha = %d, want erased 0", got[3])
	}
	if got := layerPixelAt(layer, 15, 5); got[3] != 255 {
		t.Errorf("outside selection (15,5) alpha = %d, want untouched 255", got[3])
	}
}

// ── Task 2: pressure dynamics must be neutral without a pressure device ──────

func TestApplyPressure_NeutralWhenPressureUnreported(t *testing.T) {
	base := BrushParams{Size: 20, Flow: 1, Color: [4]uint8{10, 20, 30, 200}}
	got := applyPressure(base, 0)
	if math.Abs(got.Size-20) > 0.001 {
		t.Errorf("size = %.3f, want neutral 20 when no pressure is reported", got.Size)
	}
	if math.Abs(got.Flow-1) > 0.001 {
		t.Errorf("flow = %.3f, want neutral 1 when no pressure is reported", got.Flow)
	}
}

func TestPaintStroke_FullSizeWithoutPressureDevice(t *testing.T) {
	const w, h = 60, 60
	white := [4]uint8{255, 255, 255, 255}
	inst, layerID := newStrokeTestInstance(t, w, h, white)

	// Pressure 0 = device reported nothing → dynamics must act as pressure 1.
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{
		X: 30, Y: 30, Pressure: 0,
		Brush: BrushParams{Size: 20, Hardness: 1, Flow: 1, Color: [4]uint8{0, 0, 0, 255}},
	})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end stroke: %v", err)
	}

	layer := findPixelLayer(inst.manager.activeMut(), layerID)
	// A full-size (radius 10) dab covers (39,30); the legacy 0.5 default
	// shrank the size to 15 (radius 7.5) leaving this pixel white.
	if got := layerPixelAt(layer, 39, 30); got[0] > 100 {
		t.Errorf("(39,30) R = %d, want painted (full-size dab, no pressure weakening)", got[0])
	}
	// Flow must also be neutral: the dab centre is fully black, not 50%% grey.
	if got := layerPixelAt(layer, 30, 30); got[0] > 20 {
		t.Errorf("(30,30) R = %d, want ≈0 (flow not halved)", got[0])
	}
}

// ── Task 3: mixer rim dabs must stay within the saved undo region ─────────────

func TestMixerBrushStroke_UndoRestoresAllPixels(t *testing.T) {
	const w, h = 400, 400
	white := [4]uint8{255, 255, 255, 255}
	inst, layerID := newStrokeTestInstance(t, w, h, white)
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	before := append([]byte(nil), layer.Pixels...)

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{
		X: 200, Y: 200, Pressure: 1,
		Brush: BrushParams{
			Size: 300, Hardness: 0.5, Flow: 1, Color: [4]uint8{255, 0, 0, 255},
			MixerBrush: true, MixerWetness: 0.4, MixerLoad: 1,
		},
	})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatalf("end stroke: %v", err)
	}
	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("undo: %v", err)
	}

	layer = findPixelLayer(inst.manager.activeMut(), layerID)
	for i := range before {
		if layer.Pixels[i] != before[i] {
			t.Fatalf("undo failed to restore pixel (%d,%d) channel %d: got %d want %d — dab wrote outside the saved undo region",
				(i/4)%w, (i/4)/w, i%4, layer.Pixels[i], before[i])
		}
	}
}

// ── Task 4: magic eraser must use the atomic version counter + dirty rect ────

func TestMagicErase_BumpsAtomicVersionAndMarksDirtyRect(t *testing.T) {
	const w, h = 20, 20
	red := [4]uint8{255, 0, 0, 255}
	inst, layerID := newStrokeTestInstance(t, w, h, red)
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)

	// Advance the global atomic version counter past this document's current
	// value; a plain doc.ContentVersion++ can never catch up to it.
	probe := testDocumentFixture("probe", "Probe", 4, 4)
	probe.bumpContentVersionRect(DirtyRect{X: 0, Y: 0, W: 1, H: 1})
	baseline := probe.ContentVersion

	if err := inst.handleMagicErase(MagicErasePayload{X: 5, Y: 5, Tolerance: 0, Contiguous: false}, doc, layer); err != nil {
		t.Fatalf("magic erase: %v", err)
	}
	if doc.ContentVersion <= baseline {
		t.Errorf("ContentVersion = %d, want > %d (must use the atomic counter, not ++)", doc.ContentVersion, baseline)
	}
	if doc.currentDirtyCompositeRect() == nil {
		t.Errorf("dirty composite rect not marked after magic erase")
	}
}

// ── Task 5: painting on non-pixel layers must return a clear ABI error ───────

func TestPaintStroke_OnNonPixelLayerReturnsError(t *testing.T) {
	cases := []struct {
		name  string
		layer LayerNode
	}{
		{"text", NewTextLayer("Headline", LayerBounds{X: 0, Y: 0, W: 40, H: 40}, "hi", nil)},
		{"adjustment", NewAdjustmentLayer("Levels", "levels", nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inst := &instance{
				manager:  newDocumentManager(),
				viewport: ViewportState{Zoom: 1, CanvasW: 40, CanvasH: 40, DevicePixelRatio: 1},
				history:  newHistoryStack(defaultHistoryMax),
			}
			doc := testDocumentFixture("s5-nonpixel", "S5", 40, 40)
			doc.LayerRoot.SetChildren([]LayerNode{tc.layer})
			doc.ActiveLayerID = tc.layer.ID()
			inst.manager.Create(doc)

			inst.handleBeginPaintStroke(BeginPaintStrokePayload{
				X: 20, Y: 20, Pressure: 1,
				Brush: BrushParams{Size: 10, Hardness: 1, Flow: 1, Color: [4]uint8{0, 0, 0, 255}},
			})
			// Continuing must be a safe no-op.
			inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 22, Y: 22, Pressure: 1})

			err := inst.handleEndPaintStroke()
			if err == nil {
				t.Fatalf("painting on a %s layer succeeded silently, want rasterize error", tc.name)
			}
			if !strings.Contains(err.Error(), "rasterized") {
				t.Errorf("error %q does not mention rasterization", err.Error())
			}
		})
	}
}
