package engine

import (
	"fmt"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

func (inst *instance) dispatchPathCommand(commandID int32, payloadJSON string) (bool, error) {
	doc := inst.manager.Active()
	if doc == nil {
		return true, fmt.Errorf("no active document")
	}
	if handled, err := cmdpkg.DispatchPath(commandID, payloadJSON, cmdpkg.PathDeps{
		Decode: decodePayloadAny,
		SetActiveTool: func(tool string) error {
			inst.pathTool.activeTool = tool
			return nil
		},
		PenToolClick: func(payload cmdpkg.PathPenToolClickPayload) error {
			return inst.penToolClick(PenToolClickPayload{
				X: payload.X, Y: payload.Y, DragX: payload.DragX, DragY: payload.DragY, Shift: payload.Shift,
			})
		},
		PenToolClose: inst.penToolClose,
		DirectSelectMove: func(payload cmdpkg.PathDirectSelectMovePayload) error {
			return inst.directSelectMove(DirectSelectMovePayload{
				SubpathIndex: payload.SubpathIndex,
				AnchorIndex:  payload.AnchorIndex,
				HandleKind:   payload.HandleKind,
				X:            payload.X,
				Y:            payload.Y,
			})
		},
		DirectSelectMarquee: func(payload cmdpkg.PathDirectSelectMarqueePayload) error {
			return inst.directSelectMarquee(DirectSelectMarqueePayload{
				X1: payload.X1, Y1: payload.Y1, X2: payload.X2, Y2: payload.Y2, Shift: payload.Shift,
			})
		},
		BreakHandle: func(payload cmdpkg.PathBreakHandlePayload) error {
			return inst.breakHandle(BreakHandlePayload{SubpathIndex: payload.SubpathIndex, AnchorIndex: payload.AnchorIndex})
		},
		DeleteAnchor: func(payload cmdpkg.PathDeleteAnchorPayload) error {
			return inst.deleteAnchor(DeleteAnchorPayload{SubpathIndex: payload.SubpathIndex, AnchorIndices: payload.AnchorIndices})
		},
		AddAnchorOnSegment: func(payload cmdpkg.PathAddAnchorOnSegmentPayload) error {
			return inst.addAnchorOnSegment(AddAnchorOnSegmentPayload{SubpathIndex: payload.SubpathIndex, SegmentIndex: payload.SegmentIndex, T: payload.T})
		},
		PathBoolean: func(op string, payload cmdpkg.PathBooleanPayload) error {
			opMap := map[string]PathBoolOp{
				"combine":   PathBoolCombine,
				"subtract":  PathBoolSubtract,
				"intersect": PathBoolIntersect,
				"exclude":   PathBoolExclude,
			}
			descriptionMap := map[string]string{
				"combine":   "Combine paths",
				"subtract":  "Subtract paths",
				"intersect": "Intersect paths",
				"exclude":   "Exclude paths",
			}
			return inst.dispatchPathBooleanPayload(PathBooleanPayload{
				PathIndexA: payload.PathIndexA,
				PathIndexB: payload.PathIndexB,
			}, opMap[op], descriptionMap[op])
		},
		FlattenPath: inst.dispatchFlattenPath,
		RasterizePath: func(pathIndex *int) error {
			idx := doc.ActivePathIdx
			if pathIndex != nil {
				idx = *pathIndex
			}
			return inst.executeDocCommand("Rasterize path", func(doc *Document) error {
				return doc.makeSelectionFromPath(idx)
			})
		},
		RasterizeLayer: func(layerID string) error {
			return inst.rasterizeLayer(RasterizeLayerPayload{LayerID: layerID})
		},
		CreatePath: func(name string) error {
			return inst.executeDocCommand("Create path", func(doc *Document) error {
				doc.Paths, doc.ActivePathIdx = docpkg.CreatePath(doc.Paths, name)
				return nil
			})
		},
		DeletePath: func(pathIndex int) error {
			return inst.executeDocCommand("Delete path", func(doc *Document) error {
				var err error
				doc.Paths, doc.ActivePathIdx, err = docpkg.DeletePath(doc.Paths, doc.ActivePathIdx, pathIndex)
				return err
			})
		},
		RenamePath: func(pathIndex int, name string) error {
			return inst.executeDocCommand("Rename path", func(doc *Document) error {
				return docpkg.RenamePath(doc.Paths, pathIndex, name)
			})
		},
		DuplicatePath: func(pathIndex int) error {
			return inst.executeDocCommand("Duplicate path", func(doc *Document) error {
				var err error
				doc.Paths, doc.ActivePathIdx, err = docpkg.DuplicatePath(doc.Paths, pathIndex)
				return err
			})
		},
		MakeSelectionFromPath: func(pathIndex *int) error {
			idx := doc.ActivePathIdx
			if pathIndex != nil {
				idx = *pathIndex
			}
			return inst.executeDocCommand("Make selection from path", func(doc *Document) error {
				return doc.makeSelectionFromPath(idx)
			})
		},
		FillPath: func(pathIndex *int, color [4]uint8) error {
			idx := doc.ActivePathIdx
			if pathIndex != nil {
				idx = *pathIndex
			}
			if color == [4]uint8{} {
				color = inst.foregroundColor
			}
			return inst.executeDocCommand("Fill path", func(doc *Document) error {
				return fillPathOnDoc(doc, idx, color)
			})
		},
		StrokePath: func(pathIndex *int, toolWidth float64, color [4]uint8) error {
			idx := doc.ActivePathIdx
			if pathIndex != nil {
				idx = *pathIndex
			}
			if color == [4]uint8{} {
				color = inst.foregroundColor
			}
			if toolWidth <= 0 {
				toolWidth = 1.0
			}
			return inst.executeDocCommand("Stroke path", func(doc *Document) error {
				return strokePathOnDoc(doc, idx, toolWidth, color)
			})
		},
		SetActivePath: func(pathIndex int) error {
			// Path activation is UI state, not an undoable edit (Photoshop
			// semantics). Like SetActiveLayer, use the clone-and-replace
			// pattern instead of executeDocCommand so no history entry is
			// created and the latest history snapshot is not mutated in place.
			doc := inst.manager.Active()
			if doc == nil {
				return fmt.Errorf("no active document")
			}
			if pathIndex < 0 || pathIndex >= len(doc.Paths) {
				return fmt.Errorf("path index %d out of range (have %d paths)", pathIndex, len(doc.Paths))
			}
			doc.ActivePathIdx = pathIndex
			return inst.manager.ReplaceActive(doc)
		},
	}); handled || err != nil {
		return handled, err
	}

	return false, nil
}

func (inst *instance) dispatchPathBooleanPayload(payload PathBooleanPayload, op PathBoolOp, description string) error {
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

		// Remove path B; the result in A's slot shifts down if B was below it.
		doc.Paths = append(doc.Paths[:idxB], doc.Paths[idxB+1:]...)
		if idxB < idxA {
			idxA--
		}
		// Keep the result active.
		doc.ActivePathIdx = idxA

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
