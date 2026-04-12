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
		return true, inst.dispatchPathBoolean(payloadJSON, PathBoolCombine, "Combine paths")
	case commandPathSubtract:
		return true, inst.dispatchPathBoolean(payloadJSON, PathBoolSubtract, "Subtract paths")
	case commandPathIntersect:
		return true, inst.dispatchPathBoolean(payloadJSON, PathBoolIntersect, "Intersect paths")
	case commandPathExclude:
		return true, inst.dispatchPathBoolean(payloadJSON, PathBoolExclude, "Exclude paths")
	case commandFlattenPath:
		return true, inst.dispatchFlattenPath()
	case commandRasterizePath:
		var payload MakeSelectionFromPathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		pathIdx := doc.ActivePathIdx
		if payload.PathIndex != nil {
			pathIdx = *payload.PathIndex
		}
		if err := inst.executeDocCommand("Rasterize path", func(doc *Document) error {
			return doc.makeSelectionFromPath(pathIdx)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandRasterizeLayer:
		var payload RasterizeLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.rasterizeLayer(payload)

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
		var payload MakeSelectionFromPathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		pathIdx := doc.ActivePathIdx
		if payload.PathIndex != nil {
			pathIdx = *payload.PathIndex
		}
		if err := inst.executeDocCommand("Make selection from path", func(doc *Document) error {
			return doc.makeSelectionFromPath(pathIdx)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandFillPath:
		var payload FillPathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		pathIdx := doc.ActivePathIdx
		if payload.PathIndex != nil {
			pathIdx = *payload.PathIndex
		}
		color := payload.Color
		if color == [4]uint8{} {
			color = inst.foregroundColor
		}
		if err := inst.executeDocCommand("Fill path", func(doc *Document) error {
			return fillPathOnDoc(doc, pathIdx, color)
		}); err != nil {
			return true, err
		}
		return true, nil

	case commandStrokePath:
		var payload StrokePathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		pathIdx := doc.ActivePathIdx
		if payload.PathIndex != nil {
			pathIdx = *payload.PathIndex
		}
		color := payload.Color
		if color == [4]uint8{} {
			color = inst.foregroundColor
		}
		width := payload.ToolWidth
		if width <= 0 {
			width = 1.0
		}
		if err := inst.executeDocCommand("Stroke path", func(doc *Document) error {
			return strokePathOnDoc(doc, pathIdx, width, color)
		}); err != nil {
			return true, err
		}
		return true, nil

	default:
		return false, nil
	}
}

// dispatchPathBoolean handles combine/subtract/intersect/exclude commands.
func (inst *instance) dispatchPathBoolean(payloadJSON string, op PathBoolOp, description string) error {
	var payload PathBooleanPayload
	if err := decodePayload(payloadJSON, &payload); err != nil {
		return err
	}

	return inst.executeDocCommand(description, func(doc *Document) error {
		if len(doc.Paths) < 2 {
			return fmt.Errorf("%s requires at least 2 paths", description)
		}

		idxA := payload.PathIndexA
		idxB := payload.PathIndexB

		// Default: active path and the next path.
		if idxA == 0 && idxB == 0 {
			idxA = doc.ActivePathIdx
			idxB = idxA + 1
			if idxB >= len(doc.Paths) {
				idxB = 0
			}
		}

		if idxA < 0 || idxA >= len(doc.Paths) {
			return fmt.Errorf("path index A (%d) out of range", idxA)
		}
		if idxB < 0 || idxB >= len(doc.Paths) {
			return fmt.Errorf("path index B (%d) out of range", idxB)
		}
		if idxA == idxB {
			return fmt.Errorf("path indices must differ")
		}

		result, err := pathBoolean(&doc.Paths[idxA].Path, &doc.Paths[idxB].Path, op)
		if err != nil {
			return err
		}

		// Replace path A with the result.
		doc.Paths[idxA].Path = *result

		// Remove path B.
		doc.Paths = append(doc.Paths[:idxB], doc.Paths[idxB+1:]...)

		// Adjust active index if needed.
		if doc.ActivePathIdx >= len(doc.Paths) {
			doc.ActivePathIdx = len(doc.Paths) - 1
		}
		// Keep A active.
		if idxA < len(doc.Paths) {
			doc.ActivePathIdx = idxA
		}

		return nil
	})
}

// dispatchFlattenPath merges all paths into a single path.
func (inst *instance) dispatchFlattenPath() error {
	return inst.executeDocCommand("Flatten paths", func(doc *Document) error {
		if len(doc.Paths) == 0 {
			return fmt.Errorf("no paths to flatten")
		}

		merged := flattenPaths(doc.Paths)
		name := doc.Paths[0].Name
		doc.Paths = []NamedPath{{Name: name, Path: *merged}}
		doc.ActivePathIdx = 0
		return nil
	})
}
