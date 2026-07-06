package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

// mustJSONTB marshals a payload for DispatchCommand in tests and benchmarks.
func mustJSONTB(tb testing.TB, value any) string {
	tb.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		tb.Fatalf("marshal payload: %v", err)
	}
	return string(data)
}

func uiMetaVersionOf(tb testing.TB, h int32) int64 {
	tb.Helper()
	mu.Lock()
	defer mu.Unlock()
	inst, ok := instances[h]
	if !ok {
		tb.Fatalf("no instance for handle %d", h)
	}
	return inst.uiMetaVersion
}

// TestHotDispatchReturnsAckWithoutUIMeta covers the hot-path ack contract:
// ContinuePaintStroke and move-phase PointerEvent dispatches return a result
// with nil UIMeta, a populated cursor/version, and do NOT bump uiMetaVersion.
func TestHotDispatchReturnsAckWithoutUIMeta(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	if _, err := DispatchCommand(h, commandAddLayer, mustJSONTB(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Paint",
		Bounds:    LayerBounds{W: 1920, H: 1080},
	})); err != nil {
		t.Fatalf("add layer: %v", err)
	}

	begin, err := DispatchCommand(h, commandBeginPaintStroke, mustJSONTB(t, BeginPaintStrokePayload{
		X: 100, Y: 100, Pressure: 1,
		Brush: BrushParams{Size: 10, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}},
	}))
	if err != nil {
		t.Fatalf("begin paint stroke: %v", err)
	}
	if begin.UIMeta == nil {
		t.Fatal("BeginPaintStroke (non-hot) returned no UIMeta")
	}
	versionAfterBegin := begin.UIMeta.Version

	cont, err := DispatchCommand(h, commandContinuePaintStroke, mustJSONTB(t, ContinuePaintStrokePayload{
		X: 120, Y: 100, Pressure: 1,
	}))
	if err != nil {
		t.Fatalf("continue paint stroke: %v", err)
	}
	if cont.UIMeta != nil {
		t.Fatal("ContinuePaintStroke returned full UIMeta, want minimal ack")
	}
	if cont.CursorType == "" {
		t.Fatal("ack CursorType is empty")
	}
	if cont.StatusText == "" {
		t.Fatal("ack StatusText is empty")
	}
	if cont.UIMetaVersion != versionAfterBegin {
		t.Fatalf("ContinuePaintStroke bumped uiMetaVersion: %d, want %d", cont.UIMetaVersion, versionAfterBegin)
	}
	if cont.ContentVersion == 0 {
		t.Fatal("ack ContentVersion is 0, want the painted document's version")
	}
	if cont.BufferLen != 0 {
		t.Fatalf("ack BufferLen = %d, want 0 (acks render no pixels)", cont.BufferLen)
	}

	move, err := DispatchCommand(h, commandPointerEvent, mustJSONTB(t, PointerEventPayload{
		Phase: "move", PointerID: 1, X: 10, Y: 10,
	}))
	if err != nil {
		t.Fatalf("pointer move: %v", err)
	}
	if move.UIMeta != nil {
		t.Fatal("move-phase PointerEvent returned full UIMeta, want minimal ack")
	}
	if move.CursorType == "" {
		t.Fatal("move ack CursorType is empty")
	}
	if move.UIMetaVersion != versionAfterBegin {
		t.Fatalf("move-phase PointerEvent bumped uiMetaVersion: %d, want %d", move.UIMetaVersion, versionAfterBegin)
	}
	if got := uiMetaVersionOf(t, h); got != versionAfterBegin {
		t.Fatalf("instance uiMetaVersion = %d after hot commands, want %d", got, versionAfterBegin)
	}
}

// TestNonHotDispatchBumpsUIMetaVersion verifies that a regular command (layer
// rename) bumps the version and returns a full UIMeta carrying that version.
func TestNonHotDispatchBumpsUIMetaVersion(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSONTB(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Layer 1",
		Bounds:    LayerBounds{W: 8, H: 8},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	if added.UIMeta == nil {
		t.Fatal("AddLayer returned no UIMeta")
	}
	before := added.UIMeta.Version

	renamed, err := DispatchCommand(h, commandSetLayerName, mustJSONTB(t, SetLayerNamePayload{
		LayerID: added.UIMeta.ActiveLayerID,
		Name:    "Renamed",
	}))
	if err != nil {
		t.Fatalf("rename layer: %v", err)
	}
	if renamed.UIMeta == nil {
		t.Fatal("SetLayerName returned no UIMeta")
	}
	if renamed.UIMeta.Version != before+1 {
		t.Fatalf("UIMeta.Version = %d after rename, want %d", renamed.UIMeta.Version, before+1)
	}
	if renamed.UIMetaVersion != renamed.UIMeta.Version {
		t.Fatalf("top-level UIMetaVersion %d != UIMeta.Version %d", renamed.UIMetaVersion, renamed.UIMeta.Version)
	}
}

// TestPointerUpAfterMoveDragBumpsVersion verifies the mid-gesture rule: move
// phases leave the version untouched, and the gesture-ending pointer up
// delivers the bump + full UIMeta refresh.
func TestPointerUpAfterMoveDragBumpsVersion(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	down, err := DispatchCommand(h, commandPointerEvent, mustJSONTB(t, PointerEventPayload{
		Phase: "down", PointerID: 1, X: 100, Y: 100, PanMode: true,
	}))
	if err != nil {
		t.Fatalf("pointer down: %v", err)
	}
	if down.UIMeta == nil {
		t.Fatal("pointer down returned no UIMeta")
	}
	versionAfterDown := down.UIMeta.Version

	for i := 0; i < 3; i++ {
		move, moveErr := DispatchCommand(h, commandPointerEvent, mustJSONTB(t, PointerEventPayload{
			Phase: "move", PointerID: 1, X: 110 + float64(i)*10, Y: 100, PanMode: true,
		}))
		if moveErr != nil {
			t.Fatalf("pointer move %d: %v", i, moveErr)
		}
		if move.UIMetaVersion != versionAfterDown {
			t.Fatalf("move %d bumped uiMetaVersion to %d, want %d", i, move.UIMetaVersion, versionAfterDown)
		}
	}

	up, err := DispatchCommand(h, commandPointerEvent, mustJSONTB(t, PointerEventPayload{
		Phase: "up", PointerID: 1, X: 140, Y: 100, PanMode: true,
	}))
	if err != nil {
		t.Fatalf("pointer up: %v", err)
	}
	if up.UIMeta == nil {
		t.Fatal("pointer up returned no UIMeta, want full refresh")
	}
	if up.UIMeta.Version != versionAfterDown+1 {
		t.Fatalf("UIMeta.Version after up = %d, want %d", up.UIMeta.Version, versionAfterDown+1)
	}
}

// TestAckJSONOmitsUIMetaKey is a regression test against accidental
// empty-struct emission: the marshalled ack must not contain a uiMeta key.
func TestAckJSONOmitsUIMetaKey(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	ack, err := DispatchCommand(h, commandContinuePaintStroke, mustJSONTB(t, ContinuePaintStrokePayload{
		X: 10, Y: 10, Pressure: 1,
	}))
	if err != nil {
		t.Fatalf("continue paint stroke: %v", err)
	}
	data, err := json.Marshal(ack)
	if err != nil {
		t.Fatalf("marshal ack: %v", err)
	}
	if strings.Contains(string(data), `"uiMeta"`) {
		t.Fatalf("ack JSON contains a uiMeta key: %s", data)
	}
	if !strings.Contains(string(data), `"uiMetaVersion"`) {
		t.Fatalf("ack JSON is missing uiMetaVersion: %s", data)
	}
	if !strings.Contains(string(data), `"cursorType"`) {
		t.Fatalf("ack JSON is missing cursorType: %s", data)
	}
}

// benchmarkAckFixture builds an engine with ~50 layers and ~100 history
// entries and an active brush stroke, mirroring a realistic mid-stroke UI
// state for the DispatchCommand response benchmarks.
func benchmarkAckFixture(b *testing.B) int32 {
	b.Helper()
	h := Init(`{"documentWidth":1024,"documentHeight":768,"background":"transparent","resolution":72}`)
	if h <= 0 {
		b.Fatalf("Init: invalid handle %d", h)
	}
	for i := 0; i < 50; i++ {
		result, err := DispatchCommand(h, commandAddLayer, mustJSONTB(b, AddLayerPayload{
			LayerType: LayerTypePixel,
			Name:      "Layer",
			Bounds:    LayerBounds{X: i, Y: i, W: 16, H: 16},
		}))
		if err != nil {
			b.Fatalf("add layer %d: %v", i, err)
		}
		// Rename each layer once so the fixture accumulates ~100 history
		// entries (50 adds + 50 renames).
		if _, err := DispatchCommand(h, commandSetLayerName, mustJSONTB(b, SetLayerNamePayload{
			LayerID: result.UIMeta.ActiveLayerID,
			Name:    "Layer renamed",
		})); err != nil {
			b.Fatalf("rename layer %d: %v", i, err)
		}
	}
	if _, err := DispatchCommand(h, commandBeginPaintStroke, mustJSONTB(b, BeginPaintStrokePayload{
		X: 100, Y: 100, Pressure: 1,
		Brush: BrushParams{Size: 10, Hardness: 1, Flow: 1, Color: [4]uint8{255, 0, 0, 255}},
	})); err != nil {
		b.Fatalf("begin paint stroke: %v", err)
	}
	return h
}

// BenchmarkDispatchResponseMarshal compares the marshalled DispatchCommand
// response for a ContinuePaintStroke before and after the hot-path ack:
// "full" reconstructs the old behaviour (complete render + UIMeta), "ack" is
// the new minimal response.
func BenchmarkDispatchResponseMarshal(b *testing.B) {
	h := benchmarkAckFixture(b)
	defer Free(h)

	b.Run("full", func(b *testing.B) {
		// The pre-S.4 DispatchCommand response was inst.render(), which is
		// exactly what RenderFrame still returns.
		full, err := RenderFrame(h)
		if err != nil {
			b.Fatalf("render frame: %v", err)
		}
		data, err := json.Marshal(full)
		if err != nil {
			b.Fatalf("marshal full: %v", err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := json.Marshal(full); err != nil {
				b.Fatalf("marshal full: %v", err)
			}
		}
		b.ReportMetric(float64(len(data)), "respBytes")
	})

	b.Run("ack", func(b *testing.B) {
		ack, err := DispatchCommand(h, commandContinuePaintStroke, mustJSONTB(b, ContinuePaintStrokePayload{
			X: 120, Y: 100, Pressure: 1,
		}))
		if err != nil {
			b.Fatalf("continue paint stroke: %v", err)
		}
		if ack.UIMeta != nil {
			b.Fatal("expected ack without UIMeta")
		}
		data, err := json.Marshal(ack)
		if err != nil {
			b.Fatalf("marshal ack: %v", err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := json.Marshal(ack); err != nil {
				b.Fatalf("marshal ack: %v", err)
			}
		}
		b.ReportMetric(float64(len(data)), "respBytes")
	})
}
