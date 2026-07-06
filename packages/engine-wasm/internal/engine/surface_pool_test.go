package engine

import (
	"testing"
)

// maskedGroupDoc builds a document whose single top-level layer is a
// non-passthrough (masked) group. The masked group forces compositing through
// the transient scratch buffer in compositeLayerOntoWithClipOptions (the
// surfacePool path), rather than the pass-through fast path.
func maskedGroupDoc(w, h int, childPixels []byte) *Document {
	doc := &Document{Width: w, Height: h, LayerRoot: NewGroupLayer("Root")}
	child := NewPixelLayer("Child", LayerBounds{X: 0, Y: 0, W: w, H: h}, childPixels)
	group := NewGroupLayer("Group")
	maskData := make([]byte, w*h)
	for i := range maskData {
		maskData[i] = 255 // fully visible mask => still non-passthrough
	}
	group.SetMask(&LayerMask{Enabled: true, Width: w, Height: h, Data: maskData})
	group.SetChildren([]LayerNode{child})
	doc.LayerRoot.SetChildren([]LayerNode{group})
	return doc
}

// TestSurfacePoolRecompositeNotPolluted renders two different composites in
// sequence that both exercise the pooled scratch buffer. The second render's
// content is a strict subset of the first; if acquireSurface failed to zero a
// recycled buffer, the first render's pixels would leak into the transparent
// region of the second. Asserting the transparent region stays transparent
// verifies the pool's clear-on-acquire behaviour.
func TestSurfacePoolRecompositeNotPolluted(t *testing.T) {
	const w, h = 4, 1

	// First composite: fully opaque green across the whole row.
	full := []byte{
		0, 255, 0, 255,
		0, 255, 0, 255,
		0, 255, 0, 255,
		0, 255, 0, 255,
	}
	doc1 := maskedGroupDoc(w, h, full)
	if _, err := doc1.renderLayersToSurface(doc1.LayerRoot.Children()); err != nil {
		t.Fatalf("first render: %v", err)
	}

	// Second composite: green only in the left half, transparent on the right.
	partial := []byte{
		0, 255, 0, 255,
		0, 255, 0, 255,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
	doc2 := maskedGroupDoc(w, h, partial)
	surface, err := doc2.renderLayersToSurface(doc2.LayerRoot.Children())
	if err != nil {
		t.Fatalf("second render: %v", err)
	}

	// Left half must be opaque green.
	if surface[3] != 255 || surface[1] != 255 {
		t.Fatalf("left pixel = %v, want opaque green", surface[0:4])
	}
	// Right half must be fully transparent; any non-zero alpha means the pooled
	// scratch buffer leaked content from the first render.
	for offset := 8; offset < len(surface); offset += 4 {
		if surface[offset+3] != 0 {
			t.Fatalf("pixel at offset %d = %v, want transparent (pool not zeroed?)", offset, surface[offset:offset+4])
		}
	}
}

// TestSurfacePoolNestedGroupRecursion exercises recursive compositing: a
// non-passthrough outer group containing a non-passthrough inner group. Each
// level acquires its own scratch buffer from the pool while an outer buffer is
// still checked out, so this verifies the pool never hands a live buffer to a
// nested call (which would corrupt the frame).
func TestSurfacePoolNestedGroupRecursion(t *testing.T) {
	const w, h = 2, 1
	doc := &Document{Width: w, Height: h, LayerRoot: NewGroupLayer("Root")}

	child := NewPixelLayer("Child", LayerBounds{X: 0, Y: 0, W: w, H: h}, []byte{
		10, 20, 30, 255,
		40, 50, 60, 255,
	})

	inner := NewGroupLayer("Inner")
	inner.SetMask(&LayerMask{Enabled: true, Width: w, Height: h, Data: []byte{255, 255}})
	inner.SetChildren([]LayerNode{child})

	outer := NewGroupLayer("Outer")
	outer.SetMask(&LayerMask{Enabled: true, Width: w, Height: h, Data: []byte{255, 0}})
	outer.SetChildren([]LayerNode{inner})

	doc.LayerRoot.SetChildren([]LayerNode{outer})

	surface, err := doc.renderLayersToSurface(doc.LayerRoot.Children())
	if err != nil {
		t.Fatalf("nested group render: %v", err)
	}

	// Outer mask keeps pixel 0, hides pixel 1.
	if surface[0] != 10 || surface[1] != 20 || surface[2] != 30 || surface[3] != 255 {
		t.Fatalf("first pixel = %v, want child color passed through", surface[0:4])
	}
	if surface[7] != 0 {
		t.Fatalf("second pixel = %v, want masked out by outer group", surface[4:8])
	}
}

// TestSurfacePoolAcquireZeroesAndSizes is a direct unit check of the pool
// helpers: acquireSurface must always return a zeroed slice of the requested
// length, even when a dirty buffer was released back into the pool.
func TestSurfacePoolAcquireZeroesAndSizes(t *testing.T) {
	buf := acquireSurface(16)
	if len(buf) != 16 {
		t.Fatalf("len = %d, want 16", len(buf))
	}
	for i := range buf {
		buf[i] = 0xAB
	}
	releaseSurface(buf)

	next := acquireSurface(16)
	if len(next) != 16 {
		t.Fatalf("recycled len = %d, want 16", len(next))
	}
	for i, b := range next {
		if b != 0 {
			t.Fatalf("recycled buffer byte %d = %#x, want 0 (not zeroed)", i, b)
		}
	}
}

func benchmarkCompositeDoc(w, h int) *Document {
	doc := &Document{Width: w, Height: h, LayerRoot: NewGroupLayer("Root")}

	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = byte(i)
		pixels[i+1] = byte(i >> 3)
		pixels[i+2] = byte(i >> 5)
		pixels[i+3] = 200
	}

	group := NewGroupLayer("Group")
	maskData := make([]byte, w*h)
	for i := range maskData {
		maskData[i] = byte(i)
	}
	group.SetMask(&LayerMask{Enabled: true, Width: w, Height: h, Data: maskData})
	group.SetChildren([]LayerNode{
		NewPixelLayer("A", LayerBounds{X: 0, Y: 0, W: w, H: h}, append([]byte(nil), pixels...)),
		NewPixelLayer("B", LayerBounds{X: 0, Y: 0, W: w, H: h}, append([]byte(nil), pixels...)),
	})
	doc.LayerRoot.SetChildren([]LayerNode{group})
	return doc
}

// BenchmarkRenderCompositeSurface measures the recomposite path on a moderate
// document whose masked group forces the transient scratch-buffer path. Run
// with -benchmem to observe the alloc reduction from surfacePool.
func BenchmarkRenderCompositeSurface(b *testing.B) {
	doc := benchmarkCompositeDoc(512, 512)
	layers := doc.LayerRoot.Children()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := doc.renderLayersToSurface(layers); err != nil {
			b.Fatalf("render: %v", err)
		}
	}
}
