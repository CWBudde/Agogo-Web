package engine

import "fmt"

func (inst *instance) dispatchCoreCommand(commandID int32, payloadJSON string) (bool, error) {
	switch commandID {
	case commandCreateDocument:
		var payload CreateDocumentPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("New document: %s", defaultDocumentName(payload.Name)),
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.newDocument(payload)
				inst.manager.Create(doc)
				inst.viewport.CenterX = float64(doc.Width) * 0.5
				inst.viewport.CenterY = float64(doc.Height) * 0.5
				inst.fitViewportToActiveDocument()
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandCloseDocument:
		command := &snapshotCommand{
			description: "Close document",
			applyFn: func(inst *instance) (snapshot, error) {
				if err := inst.manager.CloseActive(); err != nil {
					return snapshot{}, err
				}
				if doc := inst.manager.Active(); doc != nil {
					inst.viewport.CenterX = float64(doc.Width) * 0.5
					inst.viewport.CenterY = float64(doc.Height) * 0.5
					inst.fitViewportToActiveDocument()
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandZoomSet:
		var payload ZoomPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("Zoom to %.0f%%", payload.Zoom*100),
			applyFn: func(inst *instance) (snapshot, error) {
				nextZoom := clampZoom(payload.Zoom)
				if payload.HasAnchor {
					inst.viewport.CenterX = payload.AnchorX - (payload.AnchorX-inst.viewport.CenterX)*(inst.viewport.Zoom/nextZoom)
					inst.viewport.CenterY = payload.AnchorY - (payload.AnchorY-inst.viewport.CenterY)*(inst.viewport.Zoom/nextZoom)
				}
				inst.viewport.Zoom = nextZoom
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandPanSet:
		var payload PanPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		command := &snapshotCommand{
			description: "Pan viewport",
			applyFn: func(inst *instance) (snapshot, error) {
				inst.viewport.CenterX = payload.CenterX
				inst.viewport.CenterY = payload.CenterY
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandRotateViewSet:
		var payload RotatePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("Rotate view to %.0f°", payload.Rotation),
			applyFn: func(inst *instance) (snapshot, error) {
				inst.viewport.Rotation = normalizeRotation(payload.Rotation)
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandResize:
		var payload ResizePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		inst.viewport.CanvasW = maxInt(payload.CanvasW, 1)
		inst.viewport.CanvasH = maxInt(payload.CanvasH, 1)
		inst.viewport.DevicePixelRatio = floatValueOrDefault(payload.DevicePixelRatio, defaultDevicePixelRat)
		return true, nil

	case commandPointerEvent:
		var payload PointerEventPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		inst.handlePointerEvent(payload)
		return true, nil

	case commandBeginTxn:
		var payload BeginTransactionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		inst.history.BeginTransaction(inst, stringValueOrDefault(payload.Description, "Transaction"))
		return true, nil

	case commandEndTxn:
		var payload EndTransactionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		commit := payload.Commit
		if payloadJSON == "" {
			commit = true
		}
		inst.history.EndTransaction(commit)
		return true, nil

	case commandJumpHistory:
		var payload JumpHistoryPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.history.JumpTo(inst, payload.HistoryIndex); err != nil {
			return true, err
		}
		return true, nil

	case commandSetShowGuides:
		var payload SetShowGuidesPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		inst.viewport.ShowGuides = payload.Show
		return true, nil

	case commandClearHistory:
		inst.history.Clear()
		return true, nil

	case commandFitToView:
		command := &snapshotCommand{
			description: "Fit document on screen",
			applyFn: func(inst *instance) (snapshot, error) {
				inst.fitViewportToActiveDocument()
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandUndo:
		if err := inst.history.Undo(inst); err != nil {
			return true, err
		}
		return true, nil

	case commandRedo:
		if err := inst.history.Redo(inst); err != nil {
			return true, err
		}
		return true, nil

	case commandFlattenImage:
		command := &snapshotCommand{
			description: "Flatten image",
			applyFn: func(inst *instance) (snapshot, error) {
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
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandOpenImageFile:
		var payload OpenImageFilePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("Open image: %s", payload.Name),
			applyFn: func(inst *instance) (snapshot, error) {
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
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil

	case commandTranslateLayer:
		var payload TranslateLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		command := &snapshotCommand{
			description: "Move layer",
			applyFn: func(inst *instance) (snapshot, error) {
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
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}
