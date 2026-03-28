package engine

import (
	"container/heap"
	"math"
)

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

// edgeCostAt returns the traversal cost at (x,y): low at high-gradient edges.
// Sobel max magnitude for 8-bit input ≈ 1442; we normalise to [0,1].
func edgeCostAt(surface []byte, width, height, x, y int) float64 {
	var gx, gy float64
	for ky := -1; ky <= 1; ky++ {
		for kx := -1; kx <= 1; kx++ {
			nx := clampInt(x+kx, 0, width-1)
			ny := clampInt(y+ky, 0, height-1)
			idx := (ny*width + nx) * 4
			lum := float64(surface[idx])*0.299 + float64(surface[idx+1])*0.587 + float64(surface[idx+2])*0.114
			// Sobel X: [-1,0,1],[-2,0,2],[-1,0,1]  Y: [-1,-2,-1],[0,0,0],[1,2,1]
			sobelXW := [3][3]float64{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
			sobelYW := [3][3]float64{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}
			gx += lum * sobelXW[ky+1][kx+1]
			gy += lum * sobelYW[ky+1][kx+1]
		}
	}
	mag := math.Sqrt(gx*gx+gy*gy) / 1442.0
	if mag > 1.0 {
		mag = 1.0
	}
	return 1.0 - mag + 0.001
}

// suggestMagneticPath returns a path from (x1,y1)→(x2,y2) that follows
// high-contrast edges via Dijkstra within a padded bounding box.
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

	// 8-connected directions: [dx, dy, step-cost-multiplier]
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
			continue
		}
		cx, cy := curr.idx%boxW, curr.idx/boxW
		baseCost := edgeCostAt(surface, width, height, cx+minX, cy+minY)
		for _, d := range dirs {
			nx, ny := cx+int(d[0]), cy+int(d[1])
			if nx < 0 || nx >= boxW || ny < 0 || ny >= boxH {
				continue
			}
			nIdx := ny*boxW + nx
			nCost := edgeCostAt(surface, width, height, nx+minX, ny+minY)
			newDist := dist[curr.idx] + (baseCost+nCost)*0.5*d[2]
			if newDist < dist[nIdx] {
				dist[nIdx] = newDist
				prev[nIdx] = curr.idx
				heap.Push(pq, mlItem{nIdx, newDist})
			}
		}
	}

	// Trace back from goal to start.
	if dist[goalIdx] == inf {
		return []SelectionPoint{{X: float64(x1), Y: float64(y1)}, {X: float64(x2), Y: float64(y2)}}
	}
	var path []SelectionPoint
	for idx := goalIdx; idx != -1; idx = prev[idx] {
		path = append(path, SelectionPoint{X: float64(idx%boxW + minX), Y: float64(idx/boxW + minY)})
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return mlSimplifyPath(path, 1.5)
}

func mlSimplifyPath(pts []SelectionPoint, epsilon float64) []SelectionPoint {
	if len(pts) <= 2 {
		return pts
	}
	a, b := pts[0], pts[len(pts)-1]
	maxDist, maxIdx := 0.0, 0
	for i := 1; i < len(pts)-1; i++ {
		if d := mlPointLineDist(pts[i], a, b); d > maxDist {
			maxDist, maxIdx = d, i
		}
	}
	if maxDist > epsilon {
		l := mlSimplifyPath(pts[:maxIdx+1], epsilon)
		r := mlSimplifyPath(pts[maxIdx:], epsilon)
		return append(l[:len(l)-1], r...)
	}
	return []SelectionPoint{a, b}
}

func mlPointLineDist(p, a, b SelectionPoint) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	return math.Hypot(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
}
