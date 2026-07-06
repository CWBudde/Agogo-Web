package engine

import (
	"fmt"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
)

func (inst *instance) dispatchCoreCommand(commandID int32, payloadJSON string) (bool, error) {
	return cmdpkg.DispatchCore(commandID, payloadJSON, cmdpkg.CoreDeps{
		Decode: decodePayloadAny,
		CreateDocument: func(payload cmdpkg.CoreCreateDocumentPayload) error {
			command := newSnapshotCommand(fmt.Sprintf("New document: %s", defaultDocumentName(payload.Name)), func(inst *instance) (snapshot, error) {
				doc := inst.newDocument(CreateDocumentPayload{
					Name:       payload.Name,
					Width:      payload.Width,
					Height:     payload.Height,
					Resolution: payload.Resolution,
					ColorMode:  payload.ColorMode,
					BitDepth:   payload.BitDepth,
					Background: payload.Background,
				})
				inst.resetMixerBrushState()
				inst.manager.Create(doc)
				inst.viewport.CenterX = float64(doc.Width) * 0.5
				inst.viewport.CenterY = float64(doc.Height) * 0.5
				inst.fitViewportToActiveDocument()
				return inst.captureSnapshot(), nil
			})
			return inst.history.Execute(inst, command)
		},
		CloseDocument: func() error {
			command := newSnapshotCommand("Close document", func(inst *instance) (snapshot, error) {
				inst.resetMixerBrushState()
				if err := inst.manager.CloseActive(); err != nil {
					return snapshot{}, err
				}
				if doc := inst.manager.Active(); doc != nil {
					inst.viewport.CenterX = float64(doc.Width) * 0.5
					inst.viewport.CenterY = float64(doc.Height) * 0.5
					inst.fitViewportToActiveDocument()
				}
				return inst.captureSnapshot(), nil
			})
			return inst.history.Execute(inst, command)
		},
		// ZoomSet, PanSet and RotateViewSet are pure viewport (camera) changes.
		// Photoshop never treats navigation as an undoable action, so these
		// mutate inst.viewport directly and never touch inst.history.
		ZoomSet: func(payload cmdpkg.CoreZoomPayload) error {
			nextZoom := clampZoom(payload.Zoom)
			if payload.HasAnchor {
				inst.viewport.CenterX = payload.AnchorX - (payload.AnchorX-inst.viewport.CenterX)*(inst.viewport.Zoom/nextZoom)
				inst.viewport.CenterY = payload.AnchorY - (payload.AnchorY-inst.viewport.CenterY)*(inst.viewport.Zoom/nextZoom)
			}
			inst.viewport.Zoom = nextZoom
			return nil
		},
		PanSet: func(payload cmdpkg.CorePanPayload) error {
			inst.viewport.CenterX = payload.CenterX
			inst.viewport.CenterY = payload.CenterY
			return nil
		},
		RotateViewSet: func(payload cmdpkg.CoreRotatePayload) error {
			inst.viewport.Rotation = normalizeRotation(payload.Rotation)
			return nil
		},
		Resize: func(payload cmdpkg.CoreResizePayload) error {
			inst.viewport.CanvasW = maxInt(payload.CanvasW, 1)
			inst.viewport.CanvasH = maxInt(payload.CanvasH, 1)
			inst.viewport.DevicePixelRatio = floatValueOrDefault(payload.DevicePixelRatio, defaultDevicePixelRat)
			return nil
		},
		PointerEvent: func(payload cmdpkg.CorePointerEventPayload) error {
			inst.handlePointerEvent(corePointerEventToEngine(payload))
			return nil
		},
		BeginTxn: func(description string) error {
			inst.history.BeginTransaction(inst, stringValueOrDefault(description, "Transaction"))
			return nil
		},
		EndTxn: func(commit bool) error {
			if commit {
				inst.history.EndTransaction(true)
				return nil
			}
			// commit=false means the gesture was aborted (Escape, pointer
			// cancel): revert the document to its pre-transaction state
			// instead of merely discarding the grouped history entry.
			return inst.history.CancelTransaction(inst)
		},
		JumpHistory: func(historyIndex int) error {
			return inst.history.JumpTo(inst, historyIndex)
		},
		SetShowGuides: func(show bool) error {
			inst.viewport.ShowGuides = show
			return nil
		},
		ClearHistory: func() error {
			inst.history.Clear()
			return nil
		},
		// FitToView only recenters/rescales the viewport to the active
		// document; like the other navigation commands it is not undoable.
		FitToView: func() error {
			inst.fitViewportToActiveDocument()
			return nil
		},
		Undo: func() error {
			return inst.history.Undo(inst)
		},
		Redo: func() error {
			return inst.history.Redo(inst)
		},
		FlattenImage: func() error {
			command := newSnapshotCommand("Flatten image", func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.FlattenImage(); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			})
			return inst.history.Execute(inst, command)
		},
		OpenImageFile: func(payload cmdpkg.CoreOpenImageFilePayload) error {
			command := newSnapshotCommand(fmt.Sprintf("Open image: %s", payload.Name), func(inst *instance) (snapshot, error) {
				doc := inst.newDocument(CreateDocumentPayload{
					Name:   payload.Name,
					Width:  payload.Width,
					Height: payload.Height,
				})
				bounds := LayerBounds{X: 0, Y: 0, W: payload.Width, H: payload.Height}
				layer := NewPixelLayer("Background", bounds, payload.Pixels)
				if err := doc.AddLayer(layer, doc.LayerRoot.ID(), -1); err != nil {
					return snapshot{}, err
				}
				inst.manager.Create(doc)
				inst.viewport.CenterX = float64(doc.Width) * 0.5
				inst.viewport.CenterY = float64(doc.Height) * 0.5
				inst.fitViewportToActiveDocument()
				return inst.captureSnapshot(), nil
			})
			return inst.history.Execute(inst, command)
		},
		TranslateLayer: func(payload cmdpkg.CoreTranslateLayerPayload) error {
			command := newSnapshotCommand("Move layer", func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.TranslateLayer(payload.LayerID, payload.DX, payload.DY); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			})
			return inst.history.Execute(inst, command)
		},
	})
}

// corePointerEventToEngine converts the ABI-level pointer payload into the
// engine's internal pointer event, preserving every field — notably Button,
// which distinguishes the pressed button (0 primary, 1 middle, 2 secondary)
// from the Buttons bitmask.
func corePointerEventToEngine(payload cmdpkg.CorePointerEventPayload) PointerEventPayload {
	return PointerEventPayload{
		Phase:     payload.Phase,
		PointerID: payload.PointerID,
		X:         payload.X,
		Y:         payload.Y,
		Button:    payload.Button,
		Buttons:   payload.Buttons,
		PanMode:   payload.PanMode,
		Pressure:  payload.Pressure,
	}
}
