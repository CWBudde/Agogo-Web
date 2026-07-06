package engine

import (
	"encoding/json"
	"testing"
)

func TestHandleFillRespectsSelection(t *testing.T) {
	inst, doc, layerID := newFillGradientTestInstance(t)
	layer := findPixelLayer(doc, layerID)
	if layer == nil {
		t.Fatal("layer not found")
	}
	doc.Selection = &Selection{Width: doc.Width, Height: doc.Height, Mask: []byte{255, 255, 0, 0}}

	if err := inst.handleFill(FillPayload{
		HasPoint:     true,
		X:            0,
		Y:            0,
		Tolerance:    0,
		Contiguous:   true,
		SampleMerged: false,
		Source:       "foreground",
	}); err != nil {
		t.Fatalf("handleFill: %v", err)
	}

	updated := inst.manager.Active()
	if updated == nil {
		t.Fatal("active document missing after fill")
	}
	layer = findPixelLayer(updated, layerID)
	if layer == nil {
		t.Fatal("updated layer missing")
	}
	if got := layer.Pixels[0:4]; got[0] != 20 || got[1] != 30 || got[2] != 40 || got[3] != 255 {
		t.Fatalf("filled pixel = %v, want foreground color", got)
	}
	if got := layer.Pixels[8:12]; got[0] != 120 || got[1] != 130 || got[2] != 140 || got[3] != 255 {
		t.Fatalf("masked-out pixel = %v, want untouched", got)
	}
}

func TestHandleApplyGradientCreatesFillLayer(t *testing.T) {
	inst, _, _ := newFillGradientTestInstance(t)
	doc := inst.manager.Active()
	if doc == nil {
		t.Fatal("missing active doc")
	}

	if err := inst.handleApplyGradient(ApplyGradientPayload{
		StartX:      0,
		StartY:      0,
		EndX:        float64(doc.Width - 1),
		EndY:        0,
		Type:        GradientTypeLinear,
		CreateLayer: true,
	}); err != nil {
		t.Fatalf("handleApplyGradient: %v", err)
	}

	updated := inst.manager.Active()
	if updated == nil {
		t.Fatal("active document missing after gradient")
	}
	layer := findPixelLayer(updated, updated.ActiveLayerID)
	if layer == nil {
		t.Fatal("active gradient layer missing")
	}
	left := layer.Pixels[0:4]
	right := layer.Pixels[(layer.Bounds.W-1)*4 : layer.Bounds.W*4]
	if left[0] >= right[0] {
		t.Fatalf("gradient left pixel = %v, right pixel = %v, want left to start near foreground", left, right)
	}
}

func TestHandleApplyGradientUsesStops(t *testing.T) {
	inst, _, _ := newFillGradientTestInstance(t)
	doc := inst.manager.Active()
	if doc == nil {
		t.Fatal("missing active doc")
	}

	if err := inst.handleApplyGradient(ApplyGradientPayload{
		StartX: float64(0),
		StartY: 0,
		EndX:   float64(doc.Width - 1),
		EndY:   0,
		Type:   GradientTypeLinear,
		Stops: []GradientStopPayload{
			{Position: 0, Color: [4]uint8{255, 0, 0, 255}},
			{Position: 0.5, Color: [4]uint8{0, 255, 0, 255}},
			{Position: 1, Color: [4]uint8{0, 0, 255, 255}},
		},
		CreateLayer: true,
	}); err != nil {
		t.Fatalf("handleApplyGradient: %v", err)
	}

	updated := inst.manager.Active()
	if updated == nil {
		t.Fatal("active document missing after gradient")
	}
	layer := findPixelLayer(updated, updated.ActiveLayerID)
	if layer == nil {
		t.Fatal("active gradient layer missing")
	}
	mid := layer.Pixels[4:8]
	if mid[1] <= mid[0] || mid[1] <= mid[2] {
		t.Fatalf("gradient mid pixel = %v, want green-dominant midpoint", mid)
	}
}

func TestRenderGradientSurfaceRadial(t *testing.T) {
	stops := []GradientStopPayload{
		{Position: 0, Color: [4]uint8{255, 0, 0, 255}},
		{Position: 1, Color: [4]uint8{0, 0, 255, 255}},
	}
	p := ApplyGradientPayload{
		StartX: 2,
		StartY: 0,
		EndX:   4,
		EndY:   0,
		Type:   GradientTypeRadial,
		Stops:  stops,
	}

	buffer := renderGradientSurface(5, 1, p, [4]uint8{}, [4]uint8{})
	if buffer == nil {
		t.Fatal("renderGradientSurface returned nil")
	}

	center := buffer[2*4 : 2*4+4]
	if center[0] != 255 || center[2] != 0 {
		t.Fatalf("center pixel = %v, want start color [255 0 0 255]", center)
	}
	left := buffer[0:4]
	right := buffer[4*4 : 4*4+4]
	if left[0] != 0 || left[2] != 255 {
		t.Fatalf("left edge pixel = %v, want end color [0 0 255 255] (distance == radius)", left)
	}
	if right[0] != 0 || right[2] != 255 {
		t.Fatalf("right edge pixel = %v, want end color [0 0 255 255] (distance == radius)", right)
	}
	for i := 0; i < 4; i++ {
		if left[i] != right[i] {
			t.Fatalf("radial gradient not symmetric around start: left=%v right=%v", left, right)
		}
	}
	mid := buffer[1*4 : 1*4+4]
	if mid[0] >= 255 || mid[0] == 0 || mid[2] >= 255 || mid[2] == 0 {
		t.Fatalf("halfway pixel = %v, want blend of start and end colors", mid)
	}

	p.Reverse = true
	reversed := renderGradientSurface(5, 1, p, [4]uint8{}, [4]uint8{})
	if reversed == nil {
		t.Fatal("renderGradientSurface (reverse) returned nil")
	}
	revCenter := reversed[2*4 : 2*4+4]
	if revCenter[0] != 0 || revCenter[2] != 255 {
		t.Fatalf("reversed center pixel = %v, want end color [0 0 255 255]", revCenter)
	}
}

func TestSampleMergedColorAverage(t *testing.T) {
	inst, _, _ := newFillGradientTestInstance(t)
	handle := int32(98765)
	mu.Lock()
	instances[handle] = inst
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(instances, handle)
		mu.Unlock()
	}()

	result, err := DispatchCommand(handle, commandSampleMergedColor, mustJSONFill(t, SampleMergedColorPayload{
		X:            1,
		Y:            0,
		SampleSize:   3,
		SampleMerged: true,
	}))
	if err != nil {
		t.Fatalf("DispatchCommand: %v", err)
	}
	if result.SampledColor == nil {
		t.Fatal("sampled color missing")
	}
	if got := result.SampledColor; got[0] != 93 || got[1] != 103 || got[2] != 113 || got[3] != 255 {
		t.Fatalf("sampled color = %v, want [93 103 113 255]", got)
	}
}

func newFillGradientTestInstance(t *testing.T) (*instance, *Document, string) {
	t.Helper()
	layer := NewPixelLayer("Layer", LayerBounds{X: 0, Y: 0, W: 4, H: 1}, []byte{
		80, 90, 100, 255,
		80, 90, 100, 255,
		120, 130, 140, 255,
		120, 130, 140, 255,
	})
	doc := &Document{
		ID:            "doc-fill-gradient",
		Width:         4,
		Height:        1,
		LayerRoot:     NewGroupLayer("Root"),
		ActiveLayerID: layer.ID(),
	}
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	inst := &instance{
		manager:         newDocumentManager(),
		history:         newHistoryStack(16),
		viewport:        ViewportState{CanvasW: 4, CanvasH: 1, Zoom: 1, DevicePixelRatio: 1},
		foregroundColor: [4]uint8{20, 30, 40, 255},
		backgroundColor: [4]uint8{220, 230, 240, 255},
	}
	inst.manager.Create(doc)
	return inst, inst.manager.activeMut(), layer.ID()
}

func mustJSONFill(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}
