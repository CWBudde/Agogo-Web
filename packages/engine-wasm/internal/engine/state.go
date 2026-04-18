package engine

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"
)

func (inst *instance) nextFrameID() int64 {
	inst.frameID++
	return inst.frameID
}

func (inst *instance) captureSnapshot() snapshot {
	return snapshot{
		DocumentID: inst.manager.ActiveID(),
		Document:   inst.manager.Active(),
		Viewport:   inst.viewport,
	}
}

func (inst *instance) restoreSnapshot(state snapshot) error {
	inst.viewport = state.Viewport
	inst.manager = newDocumentManager()
	inst.resetMixerBrushState()
	inst.resetCloneStampState()
	if state.Document == nil {
		return nil
	}
	inst.manager.Create(state.Document)
	if state.DocumentID != "" && inst.manager.activeID != state.DocumentID {
		inst.manager.activeID = state.DocumentID
	}
	return nil
}

func (inst *instance) fitViewportToActiveDocument() {
	doc := inst.manager.Active()
	if doc == nil {
		return
	}
	inst.viewport.CenterX = float64(doc.Width) * 0.5
	inst.viewport.CenterY = float64(doc.Height) * 0.5

	canvasW := maxInt(inst.viewport.CanvasW, 1)
	canvasH := maxInt(inst.viewport.CanvasH, 1)
	scaleX := float64(canvasW) * 0.84 / float64(maxInt(doc.Width, 1))
	scaleY := float64(canvasH) * 0.84 / float64(maxInt(doc.Height, 1))
	inst.viewport.Zoom = clampZoom(math.Min(scaleX, scaleY))
}

func (inst *instance) handlePointerEvent(event PointerEventPayload) {
	switch event.Phase {
	case "down":
		if !event.PanMode {
			inst.pointer = pointerDragState{}
			return
		}
		inst.history.BeginTransaction(inst, "Pan viewport")
		inst.pointer = pointerDragState{
			PointerID: event.PointerID,
			StartX:    event.X,
			StartY:    event.Y,
			CenterX:   inst.viewport.CenterX,
			CenterY:   inst.viewport.CenterY,
			Zoom:      clampZoom(inst.viewport.Zoom),
			Rotation:  inst.viewport.Rotation,
			Active:    true,
		}
	case "move":
		if !inst.pointer.Active || inst.pointer.PointerID != event.PointerID {
			return
		}
		deltaX := event.X - inst.pointer.StartX
		deltaY := event.Y - inst.pointer.StartY
		docDX, docDY := screenDeltaToDocument(deltaX, deltaY, inst.pointer.Zoom, inst.pointer.Rotation)
		inst.viewport.CenterX = inst.pointer.CenterX - docDX
		inst.viewport.CenterY = inst.pointer.CenterY - docDY
	case "up":
		if inst.pointer.PointerID == event.PointerID {
			inst.pointer = pointerDragState{}
			inst.history.EndTransaction(true)
		}
	}
}

func (inst *instance) cursorType() string {
	if inst.pointer.Active {
		return "grabbing"
	}
	return "default"
}

func (inst *instance) statusText(doc *Document) string {
	return fmt.Sprintf("%s  %d x %d px  %.0f%%  %.0f°",
		doc.Name,
		doc.Width,
		doc.Height,
		inst.viewport.Zoom*100,
		inst.viewport.Rotation,
	)
}

func (inst *instance) newDocument(payload CreateDocumentPayload) *Document {
	width := valueOrDefault(payload.Width, defaultDocWidth)
	height := valueOrDefault(payload.Height, defaultDocHeight)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	return &Document{
		Width:      width,
		Height:     height,
		Resolution: floatValueOrDefault(payload.Resolution, defaultResolutionDPI),
		ColorMode:  stringValueOrDefault(payload.ColorMode, "rgb"),
		BitDepth:   valueOrDefault(payload.BitDepth, 8),
		Background: parseBackground(payload.Background),
		ID:         fmt.Sprintf("doc-%04d", atomic.AddInt64(&nextDocID, 1)),
		Name:       defaultDocumentName(payload.Name),
		CreatedAt:  timestamp,
		CreatedBy:  "agogo-web",
		ModifiedAt: timestamp,
		LayerRoot:  NewGroupLayer("Root"),
	}
}
