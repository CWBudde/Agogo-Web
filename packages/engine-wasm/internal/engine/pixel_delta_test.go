package engine

import (
	"bytes"
	"testing"
)

func TestNewPixelDeltaCapturesOnlyDirtyRect(t *testing.T) {
	before := makeSurface(4, 4, 0x11)
	after := append([]byte(nil), before...)

	// Mutate a 2x2 block in the middle.
	writePixel(after, 4, 1, 1, [4]byte{0xaa, 0xbb, 0xcc, 0xdd})
	writePixel(after, 4, 2, 1, [4]byte{0xde, 0xad, 0xbe, 0xef})
	writePixel(after, 4, 1, 2, [4]byte{0x01, 0x02, 0x03, 0x04})
	writePixel(after, 4, 2, 2, [4]byte{0x10, 0x20, 0x30, 0x40})

	delta, err := NewPixelDelta(before, after, 4, 4, DirtyRect{X: 1, Y: 1, W: 2, H: 2})
	if err != nil {
		t.Fatalf("NewPixelDelta: %v", err)
	}

	if got, want := len(delta.Before), 2*2*4; got != want {
		t.Fatalf("len(delta.Before) = %d, want %d", got, want)
	}
	if bytes.Equal(delta.Before, delta.After) {
		t.Fatal("expected before/after dirty rect bytes to differ")
	}
}

func TestPixelDeltaApplyAndUndo(t *testing.T) {
	before := makeSurface(3, 3, 0x20)
	after := append([]byte(nil), before...)
	writePixel(after, 3, 1, 1, [4]byte{0xff, 0x00, 0x00, 0xff})

	delta, err := NewPixelDelta(before, after, 3, 3, DirtyRect{X: 1, Y: 1, W: 1, H: 1})
	if err != nil {
		t.Fatalf("NewPixelDelta: %v", err)
	}

	target := append([]byte(nil), before...)
	if err := delta.Apply(target); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !bytes.Equal(target, after) {
		t.Fatal("Apply did not patch the dirty rect to the expected after state")
	}

	if err := delta.Undo(target); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if !bytes.Equal(target, before) {
		t.Fatal("Undo did not restore the original pixels")
	}
}

func TestPixelDeltaCommandUsesDirtyRectDiff(t *testing.T) {
	before := makeSurface(2, 2, 0x30)
	after := append([]byte(nil), before...)
	writePixel(after, 2, 0, 1, [4]byte{0x99, 0x88, 0x77, 0xff})

	delta, err := NewPixelDelta(before, after, 2, 2, DirtyRect{X: 0, Y: 1, W: 1, H: 1})
	if err != nil {
		t.Fatalf("NewPixelDelta: %v", err)
	}

	inst := &instance{pixels: append([]byte(nil), before...)}
	command := &pixelDeltaCommand{
		description: "Paint dirty rect",
		target: func(inst *instance) []byte {
			return inst.pixels
		},
		delta: delta,
	}

	if err := command.Apply(inst); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !bytes.Equal(inst.pixels, after) {
		t.Fatal("Apply did not update instance pixels")
	}

	if err := command.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if !bytes.Equal(inst.pixels, before) {
		t.Fatal("Undo did not restore instance pixels")
	}
}

// TestPixelDeltaCommand_UndoRedoBumpsContentVersion is a regression test for
// PLAN.md Phase S.2 item 1: undo/redo of a brush stroke must bump the active
// document's ContentVersion so the render cache (see rawFrameKey in
// render.go) invalidates and the canvas actually repaints. Previously
// pixelDeltaCommand.Apply/Undo patched layer bytes without touching
// ContentVersion, so Ctrl+Z appeared dead until an unrelated edit occurred.
func TestPixelDeltaCommand_UndoRedoBumpsContentVersion(t *testing.T) {
	const w, h = 32, 32
	inst := &instance{
		manager:  newDocumentManager(),
		viewport: ViewportState{Zoom: 1, CanvasW: w, CanvasH: h, DevicePixelRatio: 1},
		history:  newHistoryStack(defaultHistoryMax),
	}
	doc := testDocumentFixture("pixel-delta-test", "Pixel Delta", w, h)
	layer := NewPixelLayer("Paint Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, make([]byte, w*h*4))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	layerID := layer.ID()
	doc.ActiveLayerID = layerID
	inst.manager.Create(doc)

	storedPixels := func() []byte {
		l := findPixelLayer(inst.manager.activeMut(), layerID)
		if l == nil {
			t.Fatal("layer not found in stored document")
		}
		return append([]byte(nil), l.Pixels...)
	}
	hasPaintedAlpha := func(pixels []byte) bool {
		for i := 3; i < len(pixels); i += 4 {
			if pixels[i] != 0 {
				return true
			}
		}
		return false
	}

	brush := BrushParams{Size: 8, Hardness: 1.0, Flow: 1.0, Color: [4]uint8{255, 0, 0, 255}}
	cx, cy := float64(w/2), float64(h/2)

	inst.handleBeginPaintStroke(BeginPaintStrokePayload{X: cx, Y: cy, Pressure: 1.0, Brush: brush})
	inst.handleContinuePaintStroke(ContinuePaintStrokePayload{X: cx + 4, Y: cy, Pressure: 1.0})
	inst.handleEndPaintStroke()

	paintedPixels := storedPixels()
	if !hasPaintedAlpha(paintedPixels) {
		t.Fatal("expected stroke to paint some pixels before testing undo/redo")
	}
	versionAfterPaint := inst.manager.activeMut().ContentVersion

	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	versionAfterUndo := inst.manager.activeMut().ContentVersion
	if versionAfterUndo == versionAfterPaint {
		t.Fatalf("ContentVersion unchanged after undo: before=%d after=%d, render cache would serve stale frame", versionAfterPaint, versionAfterUndo)
	}
	if hasPaintedAlpha(storedPixels()) {
		t.Fatal("expected pixels to be reverted to transparent after undo")
	}

	if err := inst.history.Redo(inst); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	versionAfterRedo := inst.manager.activeMut().ContentVersion
	if versionAfterRedo == versionAfterUndo {
		t.Fatalf("ContentVersion unchanged after redo: undo=%d after=%d, render cache would serve stale frame", versionAfterUndo, versionAfterRedo)
	}
	if !bytes.Equal(storedPixels(), paintedPixels) {
		t.Fatal("expected redo to restore the painted pixels")
	}
}

func makeSurface(width, height int, value byte) []byte {
	pixels := make([]byte, width*height*4)
	for i := range pixels {
		pixels[i] = value
	}
	return pixels
}

func writePixel(pixels []byte, width, x, y int, rgba [4]byte) {
	start := (y*width + x) * 4
	copy(pixels[start:start+4], rgba[:])
}
