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

// captureSnapshot records the current engine state for history purposes. It
// stores the active document POINTER — deliberately without cloning.
//
// INVARIANT (immutability by replacement): documents stored in the Manager are
// only ever *replaced* by snapshot-based commands (working copy = Active()
// clone, mutate, ReplaceActiveNoClone/ReplaceActive), never mutated in place by
// them. The displaced object is therefore immutable from the moment of
// replacement, and history may reference it directly without a defensive copy.
// In-place mutations of the live stored object (brush strokes via activeMut(),
// live text editing, filter previews) are byte-exactly reverted by their own
// LIFO-ordered history commands (pixelDeltaCommand) or by explicit
// revert-before-commit (filter preview OrigPixels, text edit side table) — and
// restoreSnapshot additionally reverts any still-in-flight preview mutation
// before installing a snapshot — so a referenced state is always byte-correct
// at the moment history traversal reaches it.
//
// Corollary for consumers: a snapshot's Document must be treated as read-only.
// restoreSnapshot installs a CLONE of it (Manager.Replace clones) precisely so
// that post-restore in-place mutations cannot corrupt the referenced object.
func (inst *instance) captureSnapshot() snapshot {
	return snapshot{
		DocumentID: inst.manager.ActiveID(),
		Document:   inst.manager.activeMut(),
		Viewport:   inst.viewport,
	}
}

// restoreSnapshot rewinds the engine to a previously captured snapshot. A
// snapshot only records the single active document (see captureSnapshot), so
// restoring must touch nothing but that one document: it replaces (or
// re-inserts) the snapshot's document inside the existing manager and leaves
// every other open document — and their order — untouched. Historically this
// recreated the manager from scratch, which silently destroyed all other open
// documents on every undo/redo; that data-loss bug is what this guards against.
//
// It deliberately does NOT restore state.Viewport: navigation (zoom/pan/
// rotate) is never pushed onto inst.history (see dispatch_core.go), so
// undo/redo of a document edit must never yank the user's current view back
// to wherever it happened to be when the edit was made — Photoshop doesn't do
// this either. The Viewport field stays on the snapshot struct itself (see
// captureSnapshot) since other callers still read the struct as a whole; it
// simply carries the viewport that was live at capture time and is never fed
// back into inst.viewport.
//
// Because snapshots hold direct pointers into history (see captureSnapshot),
// restoring MUST install a clone of state.Document — never the referenced
// object itself. Manager.Replace clones on store, which provides exactly that:
// post-restore in-place mutations (brush strokes, previews) hit the fresh
// clone, leaving the history's referenced object untouched.
func (inst *instance) restoreSnapshot(state snapshot) error {
	inst.resetMixerBrushState()
	inst.resetCloneStampState()

	// History snapshots reference stored documents directly (see
	// captureSnapshot). In-flight preview flows (active brush stroke, filter
	// preview, live text edit) mutate the stored document in place and only
	// revert/record on their own end events — which have not happened yet if a
	// history restore arrives mid-flight. Revert them here first so the
	// document that is about to be displaced returns to the exact byte state
	// its referencing snapshots captured.
	inst.revertInFlightPreviewMutations()

	if state.Document == nil {
		// The snapshot captured a state with no active document. We must not
		// wipe the other open documents, so we only clear the active selection.
		return inst.manager.SetActiveID("")
	}

	// Replace only the snapshot's document. If it was closed since the snapshot
	// was taken, Replace re-inserts it (appended to the document order).
	if err := inst.manager.Replace(state.Document); err != nil {
		return err
	}

	id := state.DocumentID
	if id == "" {
		id = state.Document.ID
	}
	return inst.manager.SetActiveID(id)
}

// revertInFlightPreviewMutations byte-exactly rolls back every in-place
// mutation of the stored active document that belongs to a still-open preview
// flow, and discards the corresponding transient state. It is called by
// restoreSnapshot so that pointer-based history snapshots (see captureSnapshot)
// never observe half-finished preview bytes.
func (inst *instance) revertInFlightPreviewMutations() {
	inst.revertActivePaintStroke()
	if inst.filterPreview != nil {
		_, _ = inst.handleCancelFilterPreview()
	}
	inst.revertLiveTextEdit()
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
		// Panning is pure navigation and must never enter document history
		// (see restoreSnapshot / dispatch_core.go), so this no longer opens a
		// history transaction — it just tracks the drag-start state below.
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
	return fmt.Sprintf(
		"%s  %d x %d px  %.0f%%  %.0f°",
		doc.Name,
		doc.Width,
		doc.Height,
		inst.viewport.Zoom*100,
		inst.viewport.Rotation,
	)
}

func (inst *instance) newDocument(payload CreateDocumentPayload) *Document {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	return newDocumentWithCore(newDocumentCore(DocumentCreateParams{
		Width:      payload.Width,
		Height:     payload.Height,
		Resolution: payload.Resolution,
		ColorMode:  payload.ColorMode,
		BitDepth:   payload.BitDepth,
		Background: payload.Background,
		ID:         fmt.Sprintf("doc-%04d", atomic.AddInt64(&nextDocID, 1)),
		Name:       payload.Name,
		CreatedAt:  timestamp,
		CreatedBy:  "agogo-web",
		ModifiedAt: timestamp,
	}))
}
