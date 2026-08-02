package engine

import (
	"bytes"
	"math"
	"testing"

	agglib "github.com/cwbudde/agg_go"
)

func TestBrushDynamicsZeroJitterMatchesLegacyStroke(t *testing.T) {
	legacy, legacyLayerID := newStrokeTestInstance(t, 96, 48, [4]uint8{})
	dynamic, dynamicLayerID := newStrokeTestInstance(t, 96, 48, [4]uint8{})

	base := BrushParams{
		Size: 18, Hardness: 0.65, Flow: 0.8,
		Color: [4]uint8{30, 120, 220, 210},
	}
	explicit := base
	explicit.TipShape = "round"
	explicit.Roundness = 1
	explicit.Spacing = defaultBrushSpacing
	explicit.SizeDynamics = &BrushDynamic{Control: "pressure"}
	explicit.OpacityDynamics = &BrushDynamic{Control: "off"}
	explicit.FlowDynamics = &BrushDynamic{Control: "pressure"}

	paint := func(inst *instance, params BrushParams) {
		inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 12, Y: 24, Pressure: 0.42, Brush: params})
		inst.handleContinuePaintStrokePoints([]StrokePoint{
			{X: 28, Y: 19, Pressure: 0.42},
			{X: 50, Y: 27, Pressure: 0.42},
			{X: 80, Y: 22, Pressure: 0.42},
		})
		if err := inst.handleEndPaintStroke(); err != nil {
			t.Fatalf("end stroke: %v", err)
		}
	}
	paint(legacy, base)
	paint(dynamic, explicit)

	legacyPixels := findPixelLayer(legacy.manager.activeMut(), legacyLayerID).Pixels
	dynamicPixels := findPixelLayer(dynamic.manager.activeMut(), dynamicLayerID).Pixels
	if !bytes.Equal(legacyPixels, dynamicPixels) {
		t.Fatal("zero-jitter explicit dynamics changed legacy stroke bytes")
	}
}

func TestBrushDynamicsDeterministicAcrossCoalescedBatches(t *testing.T) {
	individual, individualLayerID := newStrokeTestInstance(t, 128, 64, [4]uint8{})
	coalesced, coalescedLayerID := newStrokeTestInstance(t, 128, 64, [4]uint8{})
	params := BrushParams{
		Size: 24, Hardness: 0.7, Flow: 0.9, Spacing: 0.12, Scatter: 0.2,
		Color:           [4]uint8{240, 90, 20, 230},
		SizeDynamics:    &BrushDynamic{Jitter: 0.6, Control: "pressure"},
		OpacityDynamics: &BrushDynamic{Jitter: 0.45, Control: "off"},
		FlowDynamics:    &BrushDynamic{Jitter: 0.35, Control: "tilt"},
	}
	points := []StrokePoint{
		{X: 28, Y: 27, Pressure: 0.75, TiltX: 8, TiltY: 3},
		{X: 46, Y: 33, Pressure: 0.6, TiltX: 12, TiltY: 5},
		{X: 70, Y: 24, Pressure: 0.8, TiltX: 17, TiltY: 9},
		{X: 104, Y: 35, Pressure: 0.5, TiltX: 22, TiltY: 11},
	}
	begin := BeginPaintStrokePayload{X: 10, Y: 30, Pressure: 0.7, TiltX: 5, TiltY: 2, Brush: params}
	individual.handleBeginPaintStroke(begin)
	for _, point := range points {
		individual.handleContinuePaintStrokePoints([]StrokePoint{point})
	}
	coalesced.handleBeginPaintStroke(begin)
	coalesced.handleContinuePaintStrokePoints(points)
	if err := individual.handleEndPaintStroke(); err != nil {
		t.Fatal(err)
	}
	if err := coalesced.handleEndPaintStroke(); err != nil {
		t.Fatal(err)
	}

	a := findPixelLayer(individual.manager.activeMut(), individualLayerID).Pixels
	b := findPixelLayer(coalesced.manager.activeMut(), coalescedLayerID).Pixels
	if !bytes.Equal(a, b) {
		t.Fatal("equivalent single-point and coalesced input produced different jitter")
	}
}

func TestBrushDynamicsJitterRangeAndMean(t *testing.T) {
	params := BrushParams{
		Size: 100, Flow: 0.8, Color: [4]uint8{0, 0, 0, 200},
		SizeDynamics:    &BrushDynamic{Jitter: 0.6, Control: "off"},
		OpacityDynamics: &BrushDynamic{Jitter: 0.5, Control: "off"},
		FlowDynamics:    &BrushDynamic{Jitter: 0.4, Control: "off"},
	}
	var state brushStrokeState
	state.initRandom(12, 34, params)
	const samples = 10000
	var sizeSum, alphaSum, flowSum float64
	for range samples {
		dab, _ := state.nextDabParams(params, 1, 0, 0)
		if dab.Size < 40 || dab.Size > 100 {
			t.Fatalf("size jitter out of range: %v", dab.Size)
		}
		if dab.Color[3] < 100 || dab.Color[3] > 200 {
			t.Fatalf("opacity jitter out of range: %v", dab.Color[3])
		}
		if dab.Flow < 0.48 || dab.Flow > 0.8 {
			t.Fatalf("flow jitter out of range: %v", dab.Flow)
		}
		sizeSum += dab.Size
		alphaSum += float64(dab.Color[3])
		flowSum += dab.Flow
	}
	if math.Abs(sizeSum/samples-70) > 1 {
		t.Errorf("size mean = %.2f, want about 70", sizeSum/samples)
	}
	if math.Abs(alphaSum/samples-150) > 2 {
		t.Errorf("alpha mean = %.2f, want about 150", alphaSum/samples)
	}
	if math.Abs(flowSum/samples-0.64) > 0.01 {
		t.Errorf("flow mean = %.3f, want about 0.64", flowSum/samples)
	}
	if state.dabCount != samples {
		t.Fatalf("PRNG advanced for %d dabs, want %d", state.dabCount, samples)
	}
}

func TestProceduralBrushTipsAngleAndRoundness(t *testing.T) {
	newLayer := func() *PixelLayer {
		return NewPixelLayer("tip", LayerBounds{W: 64, H: 64}, make([]byte, 64*64*4))
	}
	base := BrushParams{Size: 24, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}}
	round := newLayer()
	square := newLayer()
	PaintDab(round, 32, 32, base, 0, 1)
	base.TipShape = "square"
	PaintDab(square, 32, 32, base, 0, 1)
	corner := (41*64 + 41) * 4
	if round.Pixels[corner+3] != 0 || square.Pixels[corner+3] == 0 {
		t.Fatalf("square corner coverage does not differ from round: round=%d square=%d", round.Pixels[corner+3], square.Pixels[corner+3])
	}

	horizontal := newLayer()
	vertical := newLayer()
	line := BrushParams{Size: 30, Hardness: 1, Flow: 1, Color: [4]uint8{0, 0, 0, 255}, TipShape: "line", Roundness: 0.5}
	PaintDab(horizontal, 32, 32, line, 0, 1)
	line.Angle = 90
	PaintDab(vertical, 32, 32, line, 0, 1)
	horizontalProbe := (32*64 + 44) * 4
	verticalProbe := (44*64 + 32) * 4
	if horizontal.Pixels[horizontalProbe+3] == 0 || horizontal.Pixels[verticalProbe+3] != 0 {
		t.Fatal("line roundness did not produce a horizontal narrow tip")
	}
	if vertical.Pixels[verticalProbe+3] == 0 || vertical.Pixels[horizontalProbe+3] != 0 {
		t.Fatal("tip angle did not rotate the line tip")
	}
}

func TestVariableSizeDabsUndoExactly(t *testing.T) {
	fill := [4]uint8{245, 245, 245, 255}
	inst, layerID := newStrokeTestInstance(t, 100, 60, fill)
	layer := findPixelLayer(inst.manager.activeMut(), layerID)
	before := append([]byte(nil), layer.Pixels...)
	params := BrushParams{
		Size: 30, Hardness: 1, Flow: 1, Spacing: 0.08,
		Color:        [4]uint8{10, 20, 30, 255},
		TipShape:     "square",
		Angle:        35,
		SizeDynamics: &BrushDynamic{Jitter: 0.75, Control: "off"},
	}
	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: 10, Y: 30, Pressure: 1, Brush: params})
	inst.handleContinuePaintStrokePoints([]StrokePoint{{X: 35, Y: 20, Pressure: 1}, {X: 65, Y: 40, Pressure: 1}, {X: 90, Y: 30, Pressure: 1}})
	if err := inst.handleEndPaintStroke(); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, layer.Pixels) {
		t.Fatal("jitter stroke painted no pixels")
	}
	if err := inst.history.Undo(inst); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, layer.Pixels) {
		t.Fatal("undo did not restore all variable-size/rotated dab pixels")
	}
}

func TestSampledBrushTipRendersAlphaThroughAGG(t *testing.T) {
	layer := NewPixelLayer("sampled", LayerBounds{W: 48, H: 48}, make([]byte, 48*48*4))
	resource := &brushTipResource{
		ID: "tip-test", Width: 3, Height: 3,
		Alpha: []byte{
			0, 0, 0,
			0, 255, 0,
			0, 0, 0,
		},
	}
	params := BrushParams{
		Size: 24, Flow: 1, Hardness: 1, Color: [4]uint8{20, 180, 70, 255},
		TipResourceID: resource.ID,
	}
	var scratch []byte
	paintDabReuseTip(agglib.NewAgg2D(), layer, 24, 24, params, 0, 1, resource, &scratch)
	center := (24*48 + 24) * 4
	edge := (24*48 + 34) * 4
	if layer.Pixels[center+3] == 0 {
		t.Fatal("opaque sampled-tip centre did not paint")
	}
	paintedAlpha := layer.Pixels[center+3]
	if layer.Pixels[edge+3] != 0 {
		t.Fatalf("transparent sampled-tip edge painted alpha %d", layer.Pixels[edge+3])
	}
	if len(scratch) != resource.Width*resource.Height*4 {
		t.Fatalf("sampled-tip RGBA scratch len = %d", len(scratch))
	}
	params.Erase = true
	paintDabReuseTip(agglib.NewAgg2D(), layer, 24, 24, params, 0, 1, resource, &scratch)
	if layer.Pixels[center+3] >= paintedAlpha {
		t.Fatalf("sampled-tip eraser did not reduce centre alpha: before=%d after=%d", paintedAlpha, layer.Pixels[center+3])
	}
}

func BenchmarkBrushDynamicsPerDab(b *testing.B) {
	params := BrushParams{
		Size: 64, Flow: 0.8, Color: [4]uint8{10, 20, 30, 220},
		SizeDynamics:    &BrushDynamic{Jitter: 0.5, Control: "pressure"},
		OpacityDynamics: &BrushDynamic{Jitter: 0.5, Control: "tilt"},
		FlowDynamics:    &BrushDynamic{Jitter: 0.5, Control: "fade", FadeDabs: 500},
	}
	var state brushStrokeState
	state.initRandom(10, 20, params)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		state.nextDabParams(params, 0.7, 15, 5)
	}
}
