package engine

import (
	"bytes"
	"strings"
	"testing"
)

// ── S.5: pixel/all lock must be enforced for every pixel-mutating command ────

// newLockTestInstance builds an instance with a single 20×20 red pixel layer
// whose lock mode is set to the given value, plus one (empty) named path so
// fill-path/stroke-path commands have a target.
func newLockTestInstance(t *testing.T, lockMode LayerLockMode) (*instance, *Document, *PixelLayer) {
	t.Helper()
	inst, layerID := newStrokeTestInstance(t, 20, 20, [4]uint8{255, 0, 0, 255})
	doc := inst.manager.activeMut()
	layer := findPixelLayer(doc, layerID)
	if layer == nil {
		t.Fatalf("fixture layer %q not found", layerID)
	}
	layer.SetLockMode(lockMode)
	doc.Paths = append(doc.Paths, NamedPath{Name: "P"})
	doc.ActivePathIdx = 0
	return inst, doc, layer
}

// lockedOps enumerates every command-level entry point that destructively
// mutates a pixel layer's pixels. Each op runs the full command path and
// returns the ABI error (nil on success).
var lockedOps = []struct {
	name string
	// setup performs prerequisite edits that are NOT under test (run before
	// the no-op baselines are captured).
	setup func(t *testing.T, inst *instance, doc *Document, layer *PixelLayer)
	run   func(inst *instance, doc *Document, layer *PixelLayer) error
}{
	{
		name: "paint stroke",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			inst.handleBeginPaintStroke(BeginPaintStrokePayload{
				X: 10, Y: 10, Pressure: 1,
				Brush: BrushParams{Size: 6, Hardness: 1, Flow: 1, Color: [4]uint8{0, 0, 255, 255}},
			})
			inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: 12, Y: 12, Pressure: 1})
			return inst.handleEndPaintStroke()
		},
	},
	{
		name: "fill",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			return inst.handleFill(FillPayload{Source: "color", Color: [4]uint8{0, 255, 0, 255}})
		},
	},
	{
		name: "gradient",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			return inst.handleApplyGradient(ApplyGradientPayload{
				StartX: 0, StartY: 0, EndX: 20, EndY: 0, Type: "linear",
			})
		},
	},
	{
		name: "apply filter",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			_, err := inst.handleApplyFilter(ApplyFilterPayload{FilterID: "invert"})
			return err
		},
	},
	{
		name: "preview filter",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			_, err := inst.handlePreviewFilter(PreviewFilterPayload{FilterID: "invert", Scale: 1})
			return err
		},
	},
	{
		name: "fade filter",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			inst.preFadeSnapshot = &fadeSnapshot{
				LayerID:    layer.ID(),
				OrigPixels: append([]byte(nil), layer.Pixels...),
			}
			_, err := inst.handleFadeFilter(FadeFilterPayload{Opacity: 50})
			return err
		},
	},
	{
		name: "magic erase",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			return inst.handleMagicErase(MagicErasePayload{X: 5, Y: 5, Tolerance: 0, Contiguous: false}, doc, layer)
		},
	},
	{
		name: "cut",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			return inst.cutPixels()
		},
	},
	{
		name: "fill path",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			return inst.executeDocCommand("Fill path", func(doc *Document) error {
				return fillPathOnDoc(doc, 0, [4]uint8{0, 0, 0, 255})
			})
		},
	},
	{
		name: "stroke path",
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			return inst.executeDocCommand("Stroke path", func(doc *Document) error {
				return strokePathOnDoc(doc, 0, 2, [4]uint8{0, 0, 0, 255})
			})
		},
	},
	{
		name: "apply layer mask",
		// Adding a mask only attaches metadata; applying it bakes the mask
		// into the pixels and is the destructive step under test.
		setup: func(t *testing.T, inst *instance, doc *Document, layer *PixelLayer) {
			if err := doc.AddLayerMask(layer.ID(), AddLayerMaskRevealAll); err != nil {
				t.Fatalf("add layer mask: %v", err)
			}
		},
		run: func(inst *instance, doc *Document, layer *PixelLayer) error {
			return inst.executeDocCommand("Apply layer mask", func(doc *Document) error {
				return doc.ApplyLayerMask(layer.ID())
			})
		},
	},
}

// TestPixelLockRejectsPixelEdits asserts that every pixel-mutating command is
// rejected with a "locked" ABI error on a pixels- or all-locked layer, and
// that the rejection is a strict no-op: pixels, history, and the document
// content version are unchanged.
func TestPixelLockRejectsPixelEdits(t *testing.T) {
	for _, mode := range []LayerLockMode{LayerLockPixels, LayerLockAll} {
		for _, op := range lockedOps {
			t.Run(string(mode)+"/"+op.name, func(t *testing.T) {
				inst, doc, layer := newLockTestInstance(t, mode)
				layerID := layer.ID()
				if op.setup != nil {
					op.setup(t, inst, doc, layer)
				}
				beforePixels := append([]byte(nil), layer.Pixels...)
				beforeHistory := len(inst.history.Entries())
				beforeVersion := doc.ContentVersion

				err := op.run(inst, doc, layer)
				if err == nil {
					t.Fatalf("%s on a %s-locked layer succeeded, want lock error", op.name, mode)
				}
				if !strings.Contains(err.Error(), "locked") {
					t.Errorf("error %q does not mention the lock", err.Error())
				}

				after := inst.manager.activeMut()
				afterLayer := findPixelLayer(after, layerID)
				if afterLayer == nil {
					t.Fatalf("layer %q vanished after rejected %s", layerID, op.name)
				}
				if !bytes.Equal(afterLayer.Pixels, beforePixels) {
					t.Errorf("pixels changed after rejected %s on %s-locked layer", op.name, mode)
				}
				if got := len(inst.history.Entries()); got != beforeHistory {
					t.Errorf("history length = %d after rejected %s, want %d", got, op.name, beforeHistory)
				}
				if after.ContentVersion != beforeVersion {
					t.Errorf("ContentVersion = %d after rejected %s, want %d", after.ContentVersion, op.name, beforeVersion)
				}
			})
		}
	}
}

// TestPixelEditsAllowedWhenNotPixelLocked asserts that "none" and "position"
// lock modes do not interfere with pixel-mutating commands.
func TestPixelEditsAllowedWhenNotPixelLocked(t *testing.T) {
	for _, mode := range []LayerLockMode{LayerLockNone, LayerLockPosition} {
		for _, op := range lockedOps {
			t.Run(string(mode)+"/"+op.name, func(t *testing.T) {
				inst, doc, layer := newLockTestInstance(t, mode)
				if op.setup != nil {
					op.setup(t, inst, doc, layer)
				}
				if err := op.run(inst, doc, layer); err != nil {
					t.Fatalf("%s on a %s-locked layer failed: %v", op.name, mode, err)
				}
			})
		}
	}
}

// TestTranslateLayerLockEnforcement asserts that position and all locks block
// TranslateLayer while pixel lock does not.
func TestTranslateLayerLockEnforcement(t *testing.T) {
	cases := []struct {
		mode    LayerLockMode
		wantErr bool
	}{
		{LayerLockNone, false},
		{LayerLockPixels, false},
		{LayerLockPosition, true},
		{LayerLockAll, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			_, doc, layer := newLockTestInstance(t, tc.mode)
			beforeX := layer.Bounds.X
			err := doc.TranslateLayer(layer.ID(), 3, 4)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("TranslateLayer on %s-locked layer succeeded, want lock error", tc.mode)
				}
				if !strings.Contains(err.Error(), "locked") {
					t.Errorf("error %q does not mention the lock", err.Error())
				}
				if layer.Bounds.X != beforeX {
					t.Errorf("layer moved despite %s lock", tc.mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("TranslateLayer on %s-locked layer failed: %v", tc.mode, err)
			}
			if layer.Bounds.X != beforeX+3 {
				t.Errorf("layer X = %d, want %d", layer.Bounds.X, beforeX+3)
			}
		})
	}
}
