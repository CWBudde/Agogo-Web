package engine

import "fmt"

func (inst *instance) dispatchPathCommand(commandID int32, payloadJSON string) (bool, error) {
	doc := inst.manager.Active()
	if doc == nil {
		return true, fmt.Errorf("no active document")
	}

	switch commandID {
	case commandSetActiveTool:
		var payload SetActiveToolPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		// Tool switching is UI state only — no history entry.
		inst.pathTool.activeTool = payload.Tool
		return true, nil

	case commandPenToolClick:
		var payload PenToolClickPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.penToolClick(payload)

	case commandPenToolClose:
		return true, inst.penToolClose()

	case commandDirectSelectMove:
		var payload DirectSelectMovePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.directSelectMove(payload)

	case commandDirectSelectMarquee:
		var payload DirectSelectMarqueePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.directSelectMarquee(payload)

	case commandBreakHandle:
		var payload BreakHandlePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.breakHandle(payload)

	case commandDeleteAnchor:
		var payload DeleteAnchorPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.deleteAnchor(payload)

	case commandAddAnchorOnSegment:
		var payload AddAnchorOnSegmentPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.addAnchorOnSegment(payload)

	case commandPathCombine:
		return true, fmt.Errorf("path combine: not yet implemented")
	case commandPathSubtract:
		return true, fmt.Errorf("path subtract: not yet implemented")
	case commandPathIntersect:
		return true, fmt.Errorf("path intersect: not yet implemented")
	case commandPathExclude:
		return true, fmt.Errorf("path exclude: not yet implemented")
	case commandFlattenPath:
		return true, fmt.Errorf("flatten path: not yet implemented")
	case commandRasterizePath:
		return true, fmt.Errorf("rasterize path: not yet implemented")

	case commandCreatePath:
		var payload CreatePathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Create path", func(doc *Document) error {
			doc.CreatePath(payload.Name)
			return nil
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandDeletePath:
		var payload DeletePathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Delete path", func(doc *Document) error {
			return doc.DeletePath(payload.PathIndex)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandRenamePath:
		var payload RenamePathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Rename path", func(doc *Document) error {
			return doc.RenamePath(payload.PathIndex, payload.Name)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandDuplicatePath:
		var payload DuplicatePathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if err := inst.executeDocCommand("Duplicate path", func(doc *Document) error {
			return doc.DuplicatePath(payload.PathIndex)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandMakeSelectionFromPath:
		return true, fmt.Errorf("make selection from path: not yet implemented")
	case commandStrokePath:
		return true, fmt.Errorf("stroke path: not yet implemented")
	case commandFillPath:
		return true, fmt.Errorf("fill path: not yet implemented")

	default:
		return false, nil
	}
}
