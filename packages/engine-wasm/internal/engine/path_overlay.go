package engine

import "math"

// PathOverlay is the JSON-serialisable overlay data sent to the frontend so it
// can render path outlines, anchor handles and a rubber-band preview on top of
// the canvas. All coordinates are in viewport (screen) space.
type PathOverlay struct {
	Segments    []OverlayPolyline `json:"segments,omitempty"`
	Anchors     []OverlayAnchor   `json:"anchors,omitempty"`
	HandleLines []OverlayLine     `json:"handleLines,omitempty"`
	RubberBand  *OverlayPolyline  `json:"rubberBand,omitempty"`
}

// OverlayAnchor represents a single anchor point on the overlay.
type OverlayAnchor struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Selected bool    `json:"selected"`
	First    bool    `json:"first"`
}

// OverlayLine represents a handle line (anchor -> control point).
type OverlayLine struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
}

// OverlayPolyline is a sequence of connected points used for path segments
// and the rubber-band preview.
type OverlayPolyline struct {
	Points [][2]float64 `json:"points"`
}

// docToViewport converts document coordinates to viewport (screen) coordinates.
func (inst *instance) docToViewport(docX, docY float64) (float64, float64) {
	vp := &inst.viewport
	zoom := clampZoom(vp.Zoom)
	const degToRad = math.Pi / 180
	radians := vp.Rotation * degToRad
	cosTheta := math.Cos(radians)
	sinTheta := math.Sin(radians)

	// Translate relative to viewport center in document space.
	dx := docX - vp.CenterX
	dy := docY - vp.CenterY

	// Apply zoom then rotation, then offset by half canvas.
	sx := (dx*cosTheta - dy*sinTheta) * zoom
	sy := (dx*sinTheta + dy*cosTheta) * zoom

	halfW := float64(vp.CanvasW) * 0.5
	halfH := float64(vp.CanvasH) * 0.5
	return sx + halfW, sy + halfH
}

// buildPathOverlay generates the overlay for the active path when a path tool
// is active. Returns nil when there is nothing to show.
func (inst *instance) buildPathOverlay() *PathOverlay {
	doc := inst.manager.Active()
	if doc == nil || inst.pathTool == nil || inst.pathTool.activeTool == "" {
		return nil
	}
	if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
		return nil
	}

	p := &doc.Paths[doc.ActivePathIdx].Path
	overlay := &PathOverlay{}

	for spIdx, sp := range p.Subpaths {
		// Flatten subpath to polyline for segment display.
		pts := flattenSubpathToPolyline(&sp, 16)
		if len(pts) > 0 {
			// Convert all points to viewport coordinates.
			vpPts := make([][2]float64, len(pts))
			for i, pt := range pts {
				vx, vy := inst.docToViewport(pt[0], pt[1])
				vpPts[i] = [2]float64{vx, vy}
			}
			overlay.Segments = append(overlay.Segments, OverlayPolyline{Points: vpPts})
		}

		// Add anchor points.
		for aIdx, pt := range sp.Points {
			vx, vy := inst.docToViewport(pt.X, pt.Y)
			anchor := OverlayAnchor{
				X:        vx,
				Y:        vy,
				Selected: inst.pathTool.selectedAnchors[spIdx*10000+aIdx],
				First:    aIdx == 0 && !sp.Closed,
			}
			overlay.Anchors = append(overlay.Anchors, anchor)

			// Handle lines (only when handles differ from anchor position).
			if pt.InX != pt.X || pt.InY != pt.Y {
				ix, iy := inst.docToViewport(pt.InX, pt.InY)
				overlay.HandleLines = append(overlay.HandleLines, OverlayLine{vx, vy, ix, iy})
			}
			if pt.OutX != pt.X || pt.OutY != pt.Y {
				ox, oy := inst.docToViewport(pt.OutX, pt.OutY)
				overlay.HandleLines = append(overlay.HandleLines, OverlayLine{vx, vy, ox, oy})
			}
		}
	}

	// Rubber band from the last anchor of the last open subpath to the cursor.
	if inst.pathTool.activeTool == "pen" && len(p.Subpaths) > 0 {
		lastSP := &p.Subpaths[len(p.Subpaths)-1]
		if !lastSP.Closed && len(lastSP.Points) > 0 {
			lastPt := lastSP.Points[len(lastSP.Points)-1]
			ax, ay := inst.docToViewport(lastPt.X, lastPt.Y)
			cx, cy := inst.docToViewport(inst.pathTool.cursorDocX, inst.pathTool.cursorDocY)
			overlay.RubberBand = &OverlayPolyline{
				Points: [][2]float64{{ax, ay}, {cx, cy}},
			}
		}
	}

	// If the overlay is completely empty, return nil.
	if len(overlay.Segments) == 0 && len(overlay.Anchors) == 0 && len(overlay.HandleLines) == 0 && overlay.RubberBand == nil {
		return nil
	}

	return overlay
}
