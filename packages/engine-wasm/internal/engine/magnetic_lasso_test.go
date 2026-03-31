package engine

import (
	"container/heap"
	"math"
	"testing"
)

func TestSurfaceReaderAndLuminance(t *testing.T) {
	reader := &surfaceReader{
		pixels: []byte{
			10, 20, 30, 255,
			40, 50, 60, 255,
		},
		w: 2,
		h: 1,
	}

	if reader.Width() != 2 || reader.Height() != 1 {
		t.Fatalf("reader size = %dx%d, want 2x1", reader.Width(), reader.Height())
	}
	if got := reader.Pixel(1, 0); got != [4]byte{40, 50, 60, 255} {
		t.Fatalf("reader pixel = %v, want [40 50 60 255]", got)
	}
	if got := surfaceLuminance([4]byte{0, 0, 0, 255}); got != 0 {
		t.Fatalf("black luminance = %f, want 0", got)
	}
	if got := surfaceLuminance([4]byte{255, 255, 255, 255}); math.Abs(got-1) > 1e-9 {
		t.Fatalf("white luminance = %f, want 1", got)
	}
}

func TestMLHeapOrdersByDistance(t *testing.T) {
	pq := &mlHeap{{idx: 1, dist: 3}, {idx: 2, dist: 1}}
	heap.Init(pq)
	heap.Push(pq, mlItem{idx: 3, dist: 2})

	first := heap.Pop(pq).(mlItem)
	second := heap.Pop(pq).(mlItem)
	third := heap.Pop(pq).(mlItem)

	if first.idx != 2 || second.idx != 3 || third.idx != 1 {
		t.Fatalf("heap order = [%d %d %d], want [2 3 1]", first.idx, second.idx, third.idx)
	}
	if got := edgeCostFromGrad(0.75); math.Abs(got-0.251) > 1e-9 {
		t.Fatalf("edge cost = %f, want 0.251", got)
	}
}

func TestSuggestMagneticPathSamePoint(t *testing.T) {
	surface := makeSolidPixels(4, 4, 255, 255, 255, 255)
	path := suggestMagneticPath(surface, 4, 4, 2, 1, 2, 1)
	if len(path) != 1 {
		t.Fatalf("path length = %d, want 1", len(path))
	}
	if path[0] != (SelectionPoint{X: 2, Y: 1}) {
		t.Fatalf("path[0] = %+v, want {2 1}", path[0])
	}
}

func TestSuggestMagneticPathClampsEndpoints(t *testing.T) {
	surface := makeSolidPixels(4, 4, 255, 255, 255, 255)
	path := suggestMagneticPath(surface, 4, 4, -5, -2, 9, 8)
	if len(path) < 2 {
		t.Fatalf("path length = %d, want at least 2", len(path))
	}
	if path[0] != (SelectionPoint{X: 0, Y: 0}) {
		t.Fatalf("path start = %+v, want {0 0}", path[0])
	}
	last := path[len(path)-1]
	if last != (SelectionPoint{X: 3, Y: 3}) {
		t.Fatalf("path end = %+v, want {3 3}", last)
	}
	for _, point := range path {
		if point.X < 0 || point.X > 3 || point.Y < 0 || point.Y > 3 {
			t.Fatalf("path point %+v out of bounds", point)
		}
	}
}
