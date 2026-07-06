package engine

import (
	"bytes"
	"testing"
)

// addInvalidationTestTextLayer creates a text layer via the dispatch ABI and
// returns its layer ID.
func addInvalidationTestTextLayer(t *testing.T, h int32, text string, fontSize float64, bounds LayerBounds) string {
	t.Helper()
	addResult, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypeText,
		Name:      "Text",
		Bounds:    bounds,
		Text:      text,
		FontSize:  fontSize,
		Color:     [4]uint8{0, 0, 0, 255},
	}))
	if err != nil {
		t.Fatalf("AddLayer: %v", err)
	}
	return addResult.UIMeta.ActiveLayerID
}

// TestTextMutationCommands_InvalidateRenderCache is a regression test for text
// commands (SetTextContent, SetTextStyle, ConvertTextToPath) mutating the text
// layer and re-rasterizing WITHOUT bumping doc.ContentVersion: the raw-frame
// reuse path and the composite cache both key on ContentVersion, so the canvas
// silently never repainted (e.g. a Character-panel Bold toggle showed nothing).
func TestTextMutationCommands_InvalidateRenderCache(t *testing.T) {
	boldOn := true
	cases := []struct {
		name string
		// dispatch performs the text mutation and returns the RenderResult.
		dispatch func(t *testing.T, h int32, layerID string) RenderResult
		// wantPixelChange asserts the rendered frame bytes actually changed.
		// ConvertTextToPath intentionally produces (near-)identical ink, so it
		// only asserts the frame was not reused.
		wantPixelChange bool
	}{
		{
			name: "SetTextContent",
			dispatch: func(t *testing.T, h int32, layerID string) RenderResult {
				t.Helper()
				res, err := DispatchCommand(h, commandSetTextContent, mustJSON(t, SetTextContentPayload{
					LayerID: layerID,
					Text:    "Changed",
				}))
				if err != nil {
					t.Fatalf("SetTextContent: %v", err)
				}
				return res
			},
			wantPixelChange: true,
		},
		{
			name: "SetTextStyle_BoldToggle",
			dispatch: func(t *testing.T, h int32, layerID string) RenderResult {
				t.Helper()
				res, err := DispatchCommand(h, commandSetTextStyle, mustJSON(t, SetTextStylePayload{
					LayerID: layerID,
					Bold:    &boldOn,
				}))
				if err != nil {
					t.Fatalf("SetTextStyle: %v", err)
				}
				return res
			},
			wantPixelChange: true,
		},
		{
			name: "ConvertTextToPath",
			dispatch: func(t *testing.T, h int32, layerID string) RenderResult {
				t.Helper()
				res, err := DispatchCommand(h, commandConvertTextToPath, mustJSON(t, ConvertTextToPathPayload{
					LayerID: layerID,
				}))
				if err != nil {
					t.Fatalf("ConvertTextToPath: %v", err)
				}
				return res
			},
			wantPixelChange: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := initTextTestDoc(t)
			defer Free(h)

			layerID := addInvalidationTestTextLayer(t, h, "Hello", 24, LayerBounds{X: 10, Y: 40, W: 180, H: 50})

			inst, ok := instances[h]
			if !ok {
				t.Fatalf("no instance for handle %d", h)
			}

			// Paint the baseline frame so the reuse cache is primed.
			first := inst.renderRaw()
			if first.Error != "" || first.BufferLen == 0 {
				t.Fatalf("baseline render failed: %+v", first)
			}
			before := append([]byte(nil), inst.pixels...)

			res := tc.dispatch(t, h, layerID)

			// DispatchCommand renders internally; a reused (stale) frame carries
			// no dirty rects, so the frontend would never repaint the canvas.
			if len(res.DirtyRects) == 0 {
				t.Fatal("text mutation returned no dirty rects: frame was reused, canvas never repaints")
			}
			if tc.wantPixelChange && bytes.Equal(before, inst.pixels) {
				t.Fatal("rendered pixel bytes unchanged after text mutation")
			}
		})
	}
}

// countOpaqueInk counts pixels with non-zero alpha in an RGBA surface.
func countOpaqueInk(rgba []byte) int {
	n := 0
	for i := 3; i < len(rgba); i += 4 {
		if rgba[i] > 0 {
			n++
		}
	}
	return n
}

// TestSetTextContent_ShrinkingBounds_LeavesNoStaleInk verifies that when a
// SetTextContent shrinks a point-text layer's tight bounds (e.g. "WWWW" -> "i"
// on center-aligned text), the vacated region is invalidated too: the cached
// composite must be byte-identical to a from-scratch composite of the new
// document state. Bumping only the NEW bounds would leave stale "W" ink
// outside the shrunken rect.
func TestSetTextContent_ShrinkingBounds_LeavesNoStaleInk(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	layerID := addInvalidationTestTextLayer(t, h, "WWWW", 32, LayerBounds{X: 100, Y: 50, W: 90, H: 40})

	// Center-align so the tight bounds shrink symmetrically around the anchor.
	alignment := "center"
	if _, err := DispatchCommand(h, commandSetTextStyle, mustJSON(t, SetTextStylePayload{
		LayerID:   layerID,
		Alignment: &alignment,
	})); err != nil {
		t.Fatalf("SetTextStyle(center): %v", err)
	}

	inst, ok := instances[h]
	if !ok {
		t.Fatalf("no instance for handle %d", h)
	}

	doc := inst.manager.activeMut()
	wide, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("baseline composite: %v", err)
	}
	wideInk := countOpaqueInk(wide)
	if wideInk == 0 {
		t.Fatal("test setup: baseline composite has no ink")
	}

	if _, err := DispatchCommand(h, commandSetTextContent, mustJSON(t, SetTextContentPayload{
		LayerID: layerID,
		Text:    "i",
	})); err != nil {
		t.Fatalf("SetTextContent: %v", err)
	}

	// The dispatch installs a new working copy; re-fetch the live document.
	doc = inst.manager.activeMut()
	cached, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		t.Fatalf("cached composite: %v", err)
	}
	fresh, err := doc.renderCompositeSurfaceChecked()
	if err != nil {
		t.Fatalf("from-scratch composite: %v", err)
	}

	freshInk := countOpaqueInk(fresh)
	if freshInk == 0 || freshInk >= wideInk {
		t.Fatalf("test setup: expected ink to shrink, got %d -> %d opaque pixels", wideInk, freshInk)
	}
	if !bytes.Equal(cached, fresh) {
		cachedInk := countOpaqueInk(cached)
		t.Fatalf("cached composite differs from from-scratch composite after shrinking text: stale ink left in vacated region (cached ink=%d, fresh ink=%d)", cachedInk, freshInk)
	}
}
