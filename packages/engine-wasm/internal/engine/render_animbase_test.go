package engine

import (
	"bytes"
	"encoding/json"
	"testing"
)

// setupSelectionInstance returns an instance handle whose active document is
// small enough that the selection maps fully onto the canvas (so marching ants
// are visible) with an active rectangular selection, ready for renderRaw.
func setupSelectionInstance(t *testing.T, size int) (int32, *instance) {
	t.Helper()
	h := Init("")
	if h <= 0 {
		t.Fatalf("Init: invalid handle %d", h)
	}
	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name:       "SelDoc",
		Width:      size,
		Height:     size,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: "white",
	})); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if _, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{
		CanvasW:          size,
		CanvasH:          size,
		DevicePixelRatio: 1,
	})); err != nil {
		t.Fatalf("resize: %v", err)
	}
	inst := instances[h]
	// Force 1:1 zoom so the full document (and its selection edges) map onto the
	// canvas; createDocument otherwise picks a fit-to-screen zoom that can push
	// the selection offscreen and hide the marching ants.
	inst.viewport.Zoom = 1
	inst.viewport.CenterX = float64(size) / 2
	inst.viewport.CenterY = float64(size) / 2
	doc := inst.manager.activeMut()
	doc.Selection = newRectSelection(doc.Width, doc.Height, LayerBounds{
		X: size / 4, Y: size / 4, W: size / 2, H: size / 2,
	})
	return h, inst
}

// TestRenderRawAntsPathSkipsRecomposite locks in the core Phase S.4 win: once a
// base frame is cached, subsequent frames with an active selection take the
// cheap ants path (memcpy + overlay) instead of a full recomposite, and the
// result equals the cached base with the current ants phase stamped on top.
func TestRenderRawAntsPathSkipsRecomposite(t *testing.T) {
	h, inst := setupSelectionInstance(t, 64)
	defer Free(h)

	base := inst.fullRecompositeCount
	r1 := inst.renderRaw()
	if got := inst.fullRecompositeCount; got != base+1 {
		t.Fatalf("first frame recomposite count = %d, want %d", got, base+1)
	}
	if !inst.hasCachedAnimBase {
		t.Fatal("first frame should populate the animated base cache")
	}
	frame1 := append([]byte(nil), inst.pixels...)

	r2 := inst.renderRaw()
	if got := inst.fullRecompositeCount; got != base+1 {
		t.Fatalf("second frame triggered a recomposite: count = %d, want %d", got, base+1)
	}
	if r1.Reused || r2.Reused {
		t.Fatalf("ants frames must not be Reused: r1=%v r2=%v", r1.Reused, r2.Reused)
	}

	// The cheap-path frame must equal the cached base plus the ants stamped at
	// this frame's phase — nothing else changed.
	doc := inst.manager.activeMut()
	want := append([]byte(nil), inst.cachedAnimBase...)
	want = RenderSelectionOverlay(doc, &inst.viewport, want, doc.Selection, r2.FrameID, inst.selectionViewMode)
	if !bytes.Equal(want, inst.pixels) {
		t.Fatal("cheap-path frame != base + ants(frameID)")
	}
	// Sanity: the base itself carries no ants, so it must differ from a frame.
	if bytes.Equal(inst.cachedAnimBase, frame1) {
		t.Fatal("cached base unexpectedly identical to a rendered ants frame")
	}
}

// TestRenderRawSelectionReusedFalse verifies the Reused flag semantics: a frame
// with an active selection is cheap but not byte-identical (ants move), so it
// must report Reused=false or the frontend would skip blitting the animation.
func TestRenderRawSelectionReusedFalse(t *testing.T) {
	h, inst := setupSelectionInstance(t, 64)
	defer Free(h)

	_ = inst.renderRaw()
	before := inst.fullRecompositeCount
	r := inst.renderRaw()
	if r.Reused {
		t.Fatal("frame with active selection reported Reused=true")
	}
	if inst.fullRecompositeCount != before {
		t.Fatalf("cheap ants frame recomposited: count %d, want %d", inst.fullRecompositeCount, before)
	}
	if r.BufferLen != int32(len(inst.pixels)) || r.BufferPtr == 0 {
		t.Fatalf("cheap frame returned invalid buffer: ptr=%d len=%d", r.BufferPtr, r.BufferLen)
	}
}

// TestRenderRawNoSelectionZeroCopyIntact guards the pre-existing zero-copy reuse
// behaviour for the no-selection case: the second identical frame is handed back
// byte-for-byte (Reused=true) without a recomposite.
func TestRenderRawNoSelectionZeroCopyIntact(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)
	if _, err := DispatchCommand(h, commandResize, mustJSON(t, ResizePayload{
		CanvasW: 200, CanvasH: 200, DevicePixelRatio: 1,
	})); err != nil {
		t.Fatalf("resize: %v", err)
	}
	inst := instances[h]

	r1 := inst.renderRaw()
	before := inst.fullRecompositeCount
	r2 := inst.renderRaw()
	if !r2.Reused {
		t.Fatal("second identical no-selection frame should be zero-copy Reused=true")
	}
	if inst.fullRecompositeCount != before {
		t.Fatalf("zero-copy frame recomposited: count %d, want %d", inst.fullRecompositeCount, before)
	}
	if r2.BufferPtr != r1.BufferPtr {
		t.Fatalf("zero-copy frame changed buffer ptr: %d -> %d", r1.BufferPtr, r2.BufferPtr)
	}
}

// TestRenderRawDeselectReturnsToZeroCopy verifies that after clearing a
// selection, frame handling returns to the zero-copy reuse path.
func TestRenderRawDeselectReturnsToZeroCopy(t *testing.T) {
	h, inst := setupSelectionInstance(t, 64)
	defer Free(h)

	_ = inst.renderRaw() // full render, caches base
	_ = inst.renderRaw() // cheap ants path

	inst.manager.activeMut().Selection = nil

	// First frame after deselect rebuilds the zero-copy cache (full render).
	countBefore := inst.fullRecompositeCount
	rFull := inst.renderRaw()
	if rFull.Reused {
		t.Fatal("first frame after deselect should not be Reused")
	}
	if inst.fullRecompositeCount != countBefore+1 {
		t.Fatalf("deselect frame should recomposite once: count %d, want %d", inst.fullRecompositeCount, countBefore+1)
	}

	// Subsequent identical frames are zero-copy again.
	rReuse := inst.renderRaw()
	if !rReuse.Reused {
		t.Fatal("post-deselect steady-state frame should be zero-copy Reused=true")
	}
	if inst.fullRecompositeCount != countBefore+1 {
		t.Fatalf("post-deselect reuse recomposited: count %d, want %d", inst.fullRecompositeCount, countBefore+1)
	}
}

func BenchmarkRenderFrameWithActiveSelection(b *testing.B) {
	h := Init(`{"documentWidth":1920,"documentHeight":1080,"background":"transparent","resolution":72}`)
	if h <= 0 {
		b.Fatalf("init: invalid handle %d", h)
	}
	defer Free(h)
	resizeJSON, err := json.Marshal(ResizePayload{CanvasW: 1280, CanvasH: 720, DevicePixelRatio: 1})
	if err != nil {
		b.Fatalf("marshal resize: %v", err)
	}
	if _, err := DispatchCommand(h, commandResize, string(resizeJSON)); err != nil {
		b.Fatalf("resize: %v", err)
	}
	inst := instances[h]
	doc := inst.manager.activeMut()
	doc.Selection = newRectSelection(doc.Width, doc.Height, LayerBounds{
		X: doc.Width / 4, Y: doc.Height / 4, W: doc.Width / 2, H: doc.Height / 2,
	})
	inst.renderRaw() // prime the base cache

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inst.renderRaw()
	}
}
