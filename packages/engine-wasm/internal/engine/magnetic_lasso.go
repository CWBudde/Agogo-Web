package engine

import (
	"container/heap"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

// ---------------------------------------------------------------------------
// PixelReader adapter for the engine's flat RGBA surface format.
//
// surfaceReader wraps a flat []byte pixel buffer and implements
// agglib.PixelReader[[4]byte], making it usable with agglib.SobelGradient
// (and any future analysis algorithm) without going through a raw-byte
// convenience wrapper.  This is the Go equivalent of constructing a
// pixfmt_alpha_blend_rgba over a rendering_buffer in C++ AGG.
// ---------------------------------------------------------------------------

type surfaceReader struct {
	pixels []byte
	w, h   int
}

func (s *surfaceReader) Width() int  { return s.w }
func (s *surfaceReader) Height() int { return s.h }

func (s *surfaceReader) Pixel(x, y int) [4]byte {
	i := (y*s.w + x) * 4
	return [4]byte{s.pixels[i], s.pixels[i+1], s.pixels[i+2], s.pixels[i+3]}
}

// surfaceLuminance converts a [4]byte RGBA pixel to BT.709 luminance in [0,1].
// Matches the weights used by agglib.LuminanceRGBA8Linear.
func surfaceLuminance(p [4]byte) float64 {
	return (float64(p[0])*0.2126 + float64(p[1])*0.7152 + float64(p[2])*0.0722) / 255.0
}

// MagneticLassoSuggestPathPayload is the JSON payload for commandMagneticLassoSuggestPath.
type MagneticLassoSuggestPathPayload struct {
	X1           int    `json:"x1"`
	Y1           int    `json:"y1"`
	X2           int    `json:"x2"`
	Y2           int    `json:"y2"`
	LayerID      string `json:"layerId"`
	SampleMerged bool   `json:"sampleMerged"`
}

type mlItem struct {
	idx  int
	dist float64
}

type mlHeap []mlItem

func (h mlHeap) Len() int           { return len(h) }
func (h mlHeap) Less(i, j int) bool { return h[i].dist < h[j].dist }
func (h mlHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *mlHeap) Push(x any)        { *h = append(*h, x.(mlItem)) }
func (h *mlHeap) Pop() any          { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }

// edgeCostFromGrad converts a pre-computed Sobel gradient magnitude to a
// Dijkstra traversal cost: near-zero at strong edges, near-1 at flat areas.
// The +0.001 floor prevents zero-cost paths through perfectly flat regions.
func edgeCostFromGrad(mag float32) float64 {
	return 1.0 - float64(mag) + 0.001
}

// suggestMagneticPath returns a path from (x1,y1)→(x2,y2) that follows
// high-contrast edges via Dijkstra on a Sobel gradient map.
//
// The gradient map is computed once for the padded bounding box using
// agglib.SobelGradient, then reused across all Dijkstra iterations.
// The raw pixel path is simplified with agglib.SimplifyPolyline (RDP, ε=1.5px).
func suggestMagneticPath(surface []byte, width, height, x1, y1, x2, y2 int) []SelectionPoint {
	x1 = clampInt(x1, 0, width-1)
	y1 = clampInt(y1, 0, height-1)
	x2 = clampInt(x2, 0, width-1)
	y2 = clampInt(y2, 0, height-1)
	if x1 == x2 && y1 == y2 {
		return []SelectionPoint{{X: float64(x1), Y: float64(y1)}}
	}

	const pad = 50
	minX := clampInt(minInt(x1, x2)-pad, 0, width-1)
	maxX := clampInt(maxInt(x1, x2)+pad, 0, width-1)
	minY := clampInt(minInt(y1, y2)-pad, 0, height-1)
	maxY := clampInt(maxInt(y1, y2)+pad, 0, height-1)
	boxW := maxX - minX + 1
	boxH := maxY - minY + 1

	// Extract the bounding-box sub-image for gradient computation.
	subPixels := make([]byte, boxW*boxH*4)
	for y := range boxH {
		for x := range boxW {
			srcIdx := ((y+minY)*width + (x + minX)) * 4
			dstIdx := (y*boxW + x) * 4
			copy(subPixels[dstIdx:dstIdx+4], surface[srcIdx:srcIdx+4])
		}
	}

	// Compute Sobel gradient map once; reused in O(1) per Dijkstra edge.
	grad := agglib.SobelGradient(&surfaceReader{subPixels, boxW, boxH}, surfaceLuminance)

	lx1, ly1 := x1-minX, y1-minY
	lx2, ly2 := x2-minX, y2-minY

	const inf = math.MaxFloat64
	dist := make([]float64, boxW*boxH)
	prev := make([]int, boxW*boxH)
	for i := range dist {
		dist[i] = inf
		prev[i] = -1
	}

	startIdx := ly1*boxW + lx1
	dist[startIdx] = 0
	pq := &mlHeap{{startIdx, 0}}
	heap.Init(pq)

	goalIdx := ly2*boxW + lx2

	// 8-connected directions: [dx, dy, step-length multiplier]
	dirs := [8][3]float64{
		{-1, 0, 1},
		{1, 0, 1},
		{0, -1, 1},
		{0, 1, 1},
		{-1, -1, math.Sqrt2},
		{1, -1, math.Sqrt2},
		{-1, 1, math.Sqrt2},
		{1, 1, math.Sqrt2},
	}

	for pq.Len() > 0 {
		curr := heap.Pop(pq).(mlItem)
		if curr.idx == goalIdx {
			break
		}
		if curr.dist > dist[curr.idx] {
			continue // stale entry
		}
		cx, cy := curr.idx%boxW, curr.idx/boxW
		baseCost := edgeCostFromGrad(grad[cy*boxW+cx])
		for _, d := range dirs {
			nx, ny := cx+int(d[0]), cy+int(d[1])
			if nx < 0 || nx >= boxW || ny < 0 || ny >= boxH {
				continue
			}
			nIdx := ny*boxW + nx
			nCost := edgeCostFromGrad(grad[nIdx])
			newDist := dist[curr.idx] + (baseCost+nCost)*0.5*d[2]
			if newDist < dist[nIdx] {
				dist[nIdx] = newDist
				prev[nIdx] = curr.idx
				heap.Push(pq, mlItem{nIdx, newDist})
			}
		}
	}

	// Unreachable goal — return straight line as fallback.
	if dist[goalIdx] == inf {
		return []SelectionPoint{{X: float64(x1), Y: float64(y1)}, {X: float64(x2), Y: float64(y2)}}
	}

	// Trace back from goal to start.
	var rawPath []SelectionPoint
	for idx := goalIdx; idx != -1; idx = prev[idx] {
		rawPath = append(rawPath, SelectionPoint{
			X: float64(idx%boxW + minX),
			Y: float64(idx/boxW + minY),
		})
	}
	for i, j := 0, len(rawPath)-1; i < j; i, j = i+1, j-1 {
		rawPath[i], rawPath[j] = rawPath[j], rawPath[i]
	}

	// Convert to agg.Point, simplify with Ramer–Douglas–Peucker (ε = 1.5 px),
	// then convert back to SelectionPoint.
	aggPts := make([]agglib.Point, len(rawPath))
	for i, p := range rawPath {
		aggPts[i] = agglib.Point{X: p.X, Y: p.Y}
	}
	simplified := agglib.SimplifyPolyline(aggPts, 1.5)
	result := make([]SelectionPoint, len(simplified))
	for i, p := range simplified {
		result[i] = SelectionPoint{X: p.X, Y: p.Y}
	}
	return result
}
