package engine

import (
	"bytes"
	"testing"
)

// buildPartialViewportTestDoc creates a 320×240 document with an opaque-ish
// base layer and a semi-transparent overlapping layer so the viewport
// composite exercises blending, not just copies.
func buildPartialViewportTestDoc() *Document {
	const docW, docH = 320, 240
	doc := testDocumentFixture("partial-doc", "Partial", docW, docH)
	addPixelLayerForTest(doc, "base", LayerBounds{X: 0, Y: 0, W: docW, H: docH}, 21)
	upper := addPixelLayerForTest(doc, "upper", LayerBounds{X: 100, Y: 70, W: 140, H: 110}, 22)
	upper.SetOpacity(0.8)
	upper.SetBlendMode(BlendModeMultiply)
	return doc
}

func TestPartialViewportResampleEquivalence(t *testing.T) {
	const canvasW, canvasH = 200, 160

	cases := []struct {
		name          string
		zoom          float64
		rotation      float64
		centerX       float64
		centerY       float64
		showGuides    bool
		dirty         DirtyRect // doc-space rect to mutate (bounds-local == doc-space for the full-doc base layer)
		expectPartial bool
	}{
		{name: "Zoom050", zoom: 0.5, centerX: 160, centerY: 120, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: true},
		{name: "Zoom100Aligned", zoom: 1.0, centerX: 160, centerY: 120, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: true},
		{name: "Zoom100Unaligned", zoom: 1.0, centerX: 160.37, centerY: 120.21, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: true},
		{name: "Zoom137", zoom: 1.37, centerX: 160, centerY: 120, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: true},
		{name: "Zoom450NearestCanvasEdgeClip", zoom: 4.5, centerX: 155, centerY: 115, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: true},
		{name: "Zoom137Rot15", zoom: 1.37, rotation: 15, centerX: 160, centerY: 120, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: true},
		{name: "Zoom450Rot15", zoom: 4.5, rotation: 15, centerX: 155, centerY: 115, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: true},
		{name: "GuidesOnRectClear", zoom: 1.37, centerX: 100, centerY: 70, showGuides: true, dirty: DirtyRect{X: 40, Y: 40, W: 20, H: 16}, expectPartial: true},
		{name: "GuidesOnRectCrossesGuide", zoom: 1.37, centerX: 160, centerY: 120, showGuides: true, dirty: DirtyRect{X: 150, Y: 110, W: 20, H: 16}, expectPartial: false},
		{name: "RectAtDocEdgeFallsBack", zoom: 1.0, centerX: 160, centerY: 120, dirty: DirtyRect{X: 0, Y: 0, W: 20, H: 16}, expectPartial: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vp := ViewportState{
				CenterX:          tc.centerX,
				CenterY:          tc.centerY,
				Zoom:             tc.zoom,
				Rotation:         tc.rotation,
				CanvasW:          canvasW,
				CanvasH:          canvasH,
				DevicePixelRatio: 1,
				ShowGuides:       tc.showGuides,
			}
			inst, doc := newDirtyRectTestInstance(t, buildPartialViewportTestDoc(), vp)
			base := doc.Layers()[0].(*PixelLayer)

			// Frame 1: full render primes every cache.
			first := inst.renderRaw()
			if first.Error != "" || first.BufferLen == 0 {
				t.Fatalf("first render failed: %+v", first)
			}

			// Paint batch: mutate the doc-space rect, bump, render again.
			mutateLayerRect(doc, base, tc.dirty, 777)
			second := inst.renderRaw()
			if second.Error != "" || second.BufferLen == 0 {
				t.Fatalf("second render failed: %+v", second)
			}

			if tc.expectPartial && inst.partialViewportUpdateCount != 1 {
				t.Errorf("partialViewportUpdateCount = %d, want 1", inst.partialViewportUpdateCount)
			}
			if !tc.expectPartial && inst.partialViewportUpdateCount != 0 {
				t.Errorf("partialViewportUpdateCount = %d, want 0 (expected fallback to full render)", inst.partialViewportUpdateCount)
			}
			if tc.expectPartial {
				if len(second.DirtyRects) != 1 {
					t.Fatalf("partial frame DirtyRects = %v, want exactly one rect", second.DirtyRects)
				}
				r := second.DirtyRects[0]
				if r.W <= 0 || r.H <= 0 || r.W > canvasW || r.H > canvasH {
					t.Fatalf("partial dirty rect out of range: %+v", r)
				}
				if r.W == canvasW && r.H == canvasH {
					t.Fatalf("partial dirty rect unexpectedly covers the full canvas: %+v", r)
				}
			}

			// Byte-identical equivalence: a from-scratch full render of the
			// same document state and viewport must match the updated frame.
			refInst, _ := newDirtyRectTestInstance(t, doc, vp)
			ref := refInst.renderRaw()
			if ref.Error != "" || ref.BufferLen == 0 {
				t.Fatalf("reference render failed: %+v", ref)
			}
			if !bytes.Equal(inst.pixels, refInst.pixels) {
				diff := firstPixelDiff(inst.pixels, refInst.pixels, canvasW)
				t.Fatalf("frame after partial update differs from full render (first diff at %s)", diff)
			}
		})
	}
}

func TestPartialViewportResampleOffCanvasChangeReusesFrame(t *testing.T) {
	const canvasW, canvasH = 200, 160
	vp := ViewportState{
		CenterX: 155, CenterY: 115, Zoom: 4.5,
		CanvasW: canvasW, CanvasH: canvasH, DevicePixelRatio: 1,
	}
	inst, doc := newDirtyRectTestInstance(t, buildPartialViewportTestDoc(), vp)
	base := doc.Layers()[0].(*PixelLayer)

	first := inst.renderRaw()
	if first.Error != "" || first.BufferLen == 0 {
		t.Fatalf("first render failed: %+v", first)
	}
	snapshot := append([]byte(nil), inst.pixels...)

	// Change a region far outside the visible doc window (~132..178 × 97..133).
	mutateLayerRect(doc, base, DirtyRect{X: 20, Y: 20, W: 10, H: 10}, 555)
	second := inst.renderRaw()
	if !second.Reused {
		t.Fatalf("expected off-canvas change to reuse the previous frame, got %+v", second)
	}
	if len(second.DirtyRects) != 0 {
		t.Fatalf("reused frame DirtyRects = %v, want none", second.DirtyRects)
	}
	if !bytes.Equal(snapshot, inst.pixels) {
		t.Fatal("reused frame pixels changed")
	}
	// And the frame must still equal a from-scratch render of the new state.
	refInst, _ := newDirtyRectTestInstance(t, doc, vp)
	refInst.renderRaw()
	if !bytes.Equal(inst.pixels, refInst.pixels) {
		t.Fatal("off-canvas-reused frame differs from full render")
	}
}

func TestRawRenderDirtyRectsFullAndReused(t *testing.T) {
	const canvasW, canvasH = 120, 90
	vp := ViewportState{CenterX: 160, CenterY: 120, Zoom: 0.35, CanvasW: canvasW, CanvasH: canvasH, DevicePixelRatio: 1}
	inst, doc := newDirtyRectTestInstance(t, buildPartialViewportTestDoc(), vp)
	_ = doc

	first := inst.renderRaw()
	if len(first.DirtyRects) != 1 || first.DirtyRects[0] != (DirtyRect{X: 0, Y: 0, W: canvasW, H: canvasH}) {
		t.Fatalf("full render DirtyRects = %v, want single full-canvas rect", first.DirtyRects)
	}

	second := inst.renderRaw()
	if !second.Reused {
		t.Fatalf("expected unchanged frame to be reused, got %+v", second)
	}
	if len(second.DirtyRects) != 0 {
		t.Fatalf("reused frame DirtyRects = %v, want none", second.DirtyRects)
	}
}

// firstPixelDiff formats the first differing pixel between two RGBA canvases.
func firstPixelDiff(a, b []byte, canvasW int) string {
	if len(a) != len(b) {
		return "length mismatch"
	}
	for i := range a {
		if a[i] != b[i] {
			px := i / 4
			return formatPixelDiff(px%canvasW, px/canvasW, a[px*4:px*4+4], b[px*4:px*4+4])
		}
	}
	return "no diff"
}

func formatPixelDiff(x, y int, got, want []byte) string {
	return "(" + itoa(x) + "," + itoa(y) + ") got=" + fmtRGBA(got) + " want=" + fmtRGBA(want)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func fmtRGBA(p []byte) string {
	return "[" + itoa(int(p[0])) + " " + itoa(int(p[1])) + " " + itoa(int(p[2])) + " " + itoa(int(p[3])) + "]"
}
