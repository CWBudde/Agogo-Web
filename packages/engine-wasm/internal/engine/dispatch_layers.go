package engine

import "fmt"

func (inst *instance) executeDocCommand(description string, mutate func(*Document) error) error {
	command := &snapshotCommand{
		description: description,
		applyFn: func(inst *instance) (snapshot, error) {
			doc := inst.manager.Active()
			if doc == nil {
				return snapshot{}, fmt.Errorf("no active document")
			}
			if err := mutate(doc); err != nil {
				return snapshot{}, err
			}
			if err := inst.manager.ReplaceActive(doc); err != nil {
				return snapshot{}, err
			}
			return inst.captureSnapshot(), nil
		},
	}
	return inst.history.Execute(inst, command)
}

func (inst *instance) dispatchLayerCommand(commandID int32, payloadJSON string) (bool, error) {
	switch commandID {
	case commandAddLayer:
		var payload AddLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand(fmt.Sprintf("Add %s layer", payload.LayerType), func(doc *Document) error {
			layer, err := doc.newLayerFromPayload(payload)
			if err != nil {
				return err
			}
			index := -1
			if payload.Index != nil {
				index = *payload.Index
			}
			if err := doc.AddLayer(layer, payload.ParentLayerID, index); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandDeleteLayer:
		var payload DeleteLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Delete layer", func(doc *Document) error {
			return doc.DeleteLayer(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandMoveLayer:
		var payload MoveLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Move layer", func(doc *Document) error {
			index := -1
			if payload.Index != nil {
				index = *payload.Index
			}
			return doc.MoveLayer(payload.LayerID, payload.ParentLayerID, index)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerVis:
		var payload SetLayerVisibilityPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Toggle layer visibility", func(doc *Document) error {
			return doc.SetLayerVisibility(payload.LayerID, payload.Visible)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerOp:
		var payload SetLayerOpacityPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Set layer opacity", func(doc *Document) error {
			return doc.SetLayerOpacity(payload.LayerID, payload.Opacity, payload.FillOpacity)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerBlend:
		var payload SetLayerBlendModePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Set layer blend mode", func(doc *Document) error {
			return doc.SetLayerBlendMode(payload.LayerID, payload.BlendMode)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandDuplicateLayer:
		var payload DuplicateLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Duplicate layer", func(doc *Document) error {
			index := -1
			if payload.Index != nil {
				index = *payload.Index
			}
			_, err := doc.DuplicateLayer(payload.LayerID, payload.ParentLayerID, index)
			return err
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerLock:
		var payload SetLayerLockPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Set layer lock", func(doc *Document) error {
			return doc.SetLayerLock(payload.LayerID, payload.LockMode)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandFlattenLayer:
		var payload FlattenLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Flatten layer", func(doc *Document) error {
			return doc.FlattenLayer(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandMergeDown:
		var payload MergeDownPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Merge down", func(doc *Document) error {
			return doc.MergeDown(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandMergeVisible:
		if err := inst.executeDocCommand("Merge visible layers", func(doc *Document) error {
			return doc.MergeVisible()
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandAddLayerMask:
		var payload AddLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Add layer mask", func(doc *Document) error {
			return doc.AddLayerMask(payload.LayerID, payload.Mode)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandDeleteLayerMask:
		var payload DeleteLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Delete layer mask", func(doc *Document) error {
			return doc.DeleteLayerMask(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandApplyLayerMask:
		var payload ApplyLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Apply layer mask", func(doc *Document) error {
			return doc.ApplyLayerMask(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandInvertLayerMask:
		var payload InvertLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Invert layer mask", func(doc *Document) error {
			return doc.InvertLayerMask(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetMaskEnabled:
		var payload SetLayerMaskEnabledPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Toggle layer mask", func(doc *Document) error {
			return doc.SetLayerMaskEnabled(payload.LayerID, payload.Enabled)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerClip:
		var payload SetLayerClipToBelowPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Set clipping mask", func(doc *Document) error {
			return doc.SetLayerClipToBelow(payload.LayerID, payload.ClipToBelow)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerName:
		var payload SetLayerNamePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Rename layer", func(doc *Document) error {
			return doc.SetLayerName(payload.LayerID, payload.Name)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetAdjustmentParams:
		var payload SetAdjustmentParamsPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Set adjustment params", func(doc *Document) error {
			return doc.SetAdjustmentLayerParams(payload.LayerID, payload.AdjustmentKind, payload.Params)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerStyleStack:
		var payload SetLayerStyleStackPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Set layer styles", func(doc *Document) error {
			return doc.SetLayerStyleStack(payload.LayerID, layerStylePayloadsToStyles(payload.Styles))
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerStyleEnabled:
		var payload SetLayerStyleEnabledPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Toggle layer style", func(doc *Document) error {
			return doc.SetLayerStyleEnabled(payload.LayerID, payload.Kind, payload.Enabled)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetLayerStyleParams:
		var payload SetLayerStyleParamsPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Set layer style params", func(doc *Document) error {
			return doc.SetLayerStyleParams(payload.LayerID, payload.Kind, payload.Params)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandCopyLayerStyle:
		var payload CopyLayerStylePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		if err := inst.copyLayerStyle(doc, payload.LayerID); err != nil {
			return true, err
		}
		return true, nil

	case commandPasteLayerStyle:
		var payload PasteLayerStylePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Paste layer styles", func(doc *Document) error {
			return inst.pasteLayerStyle(doc, payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandClearLayerStyle:
		var payload ClearLayerStylePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Clear layer styles", func(doc *Document) error {
			return doc.ClearLayerStyle(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandAddVectorMask:
		var payload AddVectorMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Add vector mask", func(doc *Document) error {
			return doc.AddVectorMask(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandDeleteVectorMask:
		var payload DeleteVectorMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Delete vector mask", func(doc *Document) error {
			return doc.DeleteVectorMask(payload.LayerID)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandSetActiveLayer:
		var payload SetActiveLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		if err := doc.SetActiveLayer(payload.LayerID); err != nil {
			return true, err
		}
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return true, err
		}
		return true, nil

	case commandSetPointFromSample:
		if err := inst.handleSetPointFromSample(payloadJSON); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}

func layerStylePayloadsToStyles(payloads []LayerStylePayload) []LayerStyle {
	if len(payloads) == 0 {
		return nil
	}
	styles := make([]LayerStyle, len(payloads))
	for i, payload := range payloads {
		styles[i] = LayerStyle{
			Kind:    string(payload.Kind),
			Enabled: payload.Enabled,
			Params:  cloneJSONRawMessage(payload.Params),
		}
	}
	return styles
}
