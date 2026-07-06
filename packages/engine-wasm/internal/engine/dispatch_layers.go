package engine

import (
	"fmt"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
)

// executeDocCommand runs mutate against a private working copy of the active
// document and records the change as a single history entry.
//
// Clone economics (see the invariant on captureSnapshot): exactly ONE deep
// clone per command. Active() clones the stored document into a discardable
// working copy — kept deliberately, because mutate may fail and must not leave
// partial changes behind. On success the working copy is installed via
// ReplaceActiveNoClone (ownership transfer, no second clone), which displaces
// the previously stored object and thereby freezes it for the history entry's
// before-snapshot; the after-snapshot then pointer-captures the newly stored
// document.
func (inst *instance) executeDocCommand(description string, mutate func(*Document) error) error {
	command := newSnapshotCommand(description, func(inst *instance) (snapshot, error) {
		doc := inst.manager.Active()
		if doc == nil {
			return snapshot{}, fmt.Errorf("no active document")
		}
		if err := mutate(doc); err != nil {
			return snapshot{}, err
		}
		if err := inst.manager.ReplaceActiveNoClone(doc); err != nil {
			return snapshot{}, err
		}
		return inst.captureSnapshot(), nil
	})
	return inst.history.Execute(inst, command)
}

func (inst *instance) dispatchLayerCommand(commandID int32, payloadJSON string) (bool, error) {
	return cmdpkg.DispatchLayer(commandID, payloadJSON, cmdpkg.LayerDeps{
		Decode: decodePayloadAny,
		AddLayer: func(payload cmdpkg.LayerAddPayload) error {
			return inst.executeDocCommand(fmt.Sprintf("Add %s layer", payload.LayerType), func(doc *Document) error {
				enginePayload := AddLayerPayload{
					LayerType:          LayerType(payload.LayerType),
					Name:               payload.Name,
					ParentLayerID:      payload.ParentLayerID,
					Index:              payload.Index,
					Bounds:             LayerBounds(payload.Bounds),
					Pixels:             payload.Pixels,
					AdjustmentKind:     payload.AdjustmentKind,
					Params:             payload.Params,
					Text:               payload.Text,
					FontFamily:         payload.FontFamily,
					FontSize:           payload.FontSize,
					Color:              payload.Color,
					FillColor:          payload.FillColor,
					StrokeColor:        payload.StrokeColor,
					StrokeWidth:        payload.StrokeWidth,
					CachedRaster:       payload.CachedRaster,
					Isolated:           payload.Isolated,
					IsArtboard:         payload.IsArtboard,
					ArtboardBackground: payload.ArtboardBackground,
				}
				if payload.Path != nil {
					path := Path(*payload.Path)
					enginePayload.Path = &path
				}
				if payload.ArtboardBounds != nil {
					bounds := LayerBounds(*payload.ArtboardBounds)
					enginePayload.ArtboardBounds = &bounds
				}
				layer, err := doc.newLayerFromPayload(enginePayload)
				if err != nil {
					return err
				}
				index := -1
				if payload.Index != nil {
					index = *payload.Index
				}
				return doc.AddLayer(layer, payload.ParentLayerID, index)
			})
		},
		DeleteLayer: func(layerID string) error {
			return inst.executeDocCommand("Delete layer", func(doc *Document) error {
				return doc.DeleteLayer(layerID)
			})
		},
		MoveLayer: func(payload cmdpkg.LayerMovePayload) error {
			return inst.executeDocCommand("Move layer", func(doc *Document) error {
				index := -1
				if payload.Index != nil {
					index = *payload.Index
				}
				return doc.MoveLayer(payload.LayerID, payload.ParentLayerID, index)
			})
		},
		SetLayerVisibility: func(payload cmdpkg.LayerVisibilityPayload) error {
			return inst.executeDocCommand("Toggle layer visibility", func(doc *Document) error {
				return doc.SetLayerVisibility(payload.LayerID, payload.Visible)
			})
		},
		SetLayerOpacity: func(payload cmdpkg.LayerOpacityPayload) error {
			return inst.executeDocCommand("Set layer opacity", func(doc *Document) error {
				return doc.SetLayerOpacity(payload.LayerID, payload.Opacity, payload.FillOpacity)
			})
		},
		SetLayerBlendMode: func(payload cmdpkg.LayerBlendModePayload) error {
			return inst.executeDocCommand("Set layer blend mode", func(doc *Document) error {
				return doc.SetLayerBlendMode(payload.LayerID, BlendMode(payload.BlendMode))
			})
		},
		DuplicateLayer: func(payload cmdpkg.LayerDuplicatePayload) error {
			return inst.executeDocCommand("Duplicate layer", func(doc *Document) error {
				index := -1
				if payload.Index != nil {
					index = *payload.Index
				}
				_, err := doc.DuplicateLayer(payload.LayerID, payload.ParentLayerID, index)
				return err
			})
		},
		SetLayerLock: func(payload cmdpkg.LayerLockPayload) error {
			return inst.executeDocCommand("Set layer lock", func(doc *Document) error {
				return doc.SetLayerLock(payload.LayerID, LayerLockMode(payload.LockMode))
			})
		},
		FlattenLayer: func(layerID string) error {
			return inst.executeDocCommand("Flatten layer", func(doc *Document) error {
				return doc.FlattenLayer(layerID)
			})
		},
		MergeDown: func(layerID string) error {
			return inst.executeDocCommand("Merge down", func(doc *Document) error {
				return doc.MergeDown(layerID)
			})
		},
		MergeVisible: func() error {
			return inst.executeDocCommand("Merge visible layers", func(doc *Document) error {
				return doc.MergeVisible()
			})
		},
		AddLayerMask: func(payload cmdpkg.LayerAddMaskPayload) error {
			return inst.executeDocCommand("Add layer mask", func(doc *Document) error {
				return doc.AddLayerMask(payload.LayerID, AddLayerMaskMode(payload.Mode))
			})
		},
		DeleteLayerMask: func(layerID string) error {
			return inst.executeDocCommand("Delete layer mask", func(doc *Document) error {
				return doc.DeleteLayerMask(layerID)
			})
		},
		ApplyLayerMask: func(layerID string) error {
			return inst.executeDocCommand("Apply layer mask", func(doc *Document) error {
				return doc.ApplyLayerMask(layerID)
			})
		},
		InvertLayerMask: func(layerID string) error {
			return inst.executeDocCommand("Invert layer mask", func(doc *Document) error {
				return doc.InvertLayerMask(layerID)
			})
		},
		SetLayerMaskEnabled: func(payload cmdpkg.LayerMaskEnabledPayload) error {
			return inst.executeDocCommand("Toggle layer mask", func(doc *Document) error {
				return doc.SetLayerMaskEnabled(payload.LayerID, payload.Enabled)
			})
		},
		SetLayerClipToBelow: func(payload cmdpkg.LayerClipPayload) error {
			return inst.executeDocCommand("Set clipping mask", func(doc *Document) error {
				return doc.SetLayerClipToBelow(payload.LayerID, payload.ClipToBelow)
			})
		},
		SetActiveLayer: func(layerID string) error {
			doc := inst.manager.Active()
			if doc == nil {
				return fmt.Errorf("no active document")
			}
			if err := doc.SetActiveLayer(layerID); err != nil {
				return err
			}
			return inst.manager.ReplaceActive(doc)
		},
		SetLayerName: func(payload cmdpkg.LayerNamePayload) error {
			return inst.executeDocCommand("Rename layer", func(doc *Document) error {
				return doc.SetLayerName(payload.LayerID, payload.Name)
			})
		},
		AddVectorMask: func(payload cmdpkg.LayerAddVectorMaskPayload) error {
			return inst.executeDocCommand("Add vector mask", func(doc *Document) error {
				return doc.AddVectorMask(payload.LayerID, payload.FromActivePath)
			})
		},
		DeleteVectorMask: func(layerID string) error {
			return inst.executeDocCommand("Delete vector mask", func(doc *Document) error {
				return doc.DeleteVectorMask(layerID)
			})
		},
		SetVectorMaskPath: func(payload cmdpkg.LayerVectorMaskPathPayload) error {
			return inst.executeDocCommand("Set vector mask path", func(doc *Document) error {
				return doc.SetVectorMaskPath(payload.LayerID, payload.Path)
			})
		},
		SetAdjustmentParams: func(payload cmdpkg.LayerAdjustmentParamsPayload) error {
			return inst.executeDocCommand("Set adjustment params", func(doc *Document) error {
				return doc.SetAdjustmentLayerParams(payload.LayerID, payload.AdjustmentKind, payload.Params)
			})
		},
		SetLayerStyleStack: func(payload cmdpkg.LayerStyleStackPayload) error {
			return inst.executeDocCommand("Set layer styles", func(doc *Document) error {
				return doc.SetLayerStyleStack(payload.LayerID, layerStylePayloadsToStyles(layerStylePayloadsFromCommand(payload.Styles)))
			})
		},
		SetLayerStyleEnabled: func(payload cmdpkg.LayerStyleEnabledPayload) error {
			return inst.executeDocCommand("Toggle layer style", func(doc *Document) error {
				return doc.SetLayerStyleEnabled(payload.LayerID, LayerStyleKind(payload.Kind), payload.Enabled)
			})
		},
		SetLayerStyleParams: func(payload cmdpkg.LayerStyleParamsPayload) error {
			return inst.executeDocCommand("Set layer style params", func(doc *Document) error {
				return doc.SetLayerStyleParams(payload.LayerID, LayerStyleKind(payload.Kind), payload.Params)
			})
		},
		CopyLayerStyle: func(layerID string) error {
			doc := inst.manager.Active()
			if doc == nil {
				return fmt.Errorf("no active document")
			}
			return inst.copyLayerStyle(doc, layerID)
		},
		PasteLayerStyle: func(layerID string) error {
			return inst.executeDocCommand("Paste layer styles", func(doc *Document) error {
				return inst.pasteLayerStyle(doc, layerID)
			})
		},
		ClearLayerStyle: func(layerID string) error {
			return inst.executeDocCommand("Clear layer styles", func(doc *Document) error {
				return doc.ClearLayerStyle(layerID)
			})
		},
		CreateDocumentStylePreset: func(payload cmdpkg.DocumentStylePresetCreatePayload) error {
			return inst.createDocumentStylePreset(CreateDocumentStylePresetPayload{
				Name:   payload.Name,
				Styles: layerStylePayloadsFromCommand(payload.Styles),
			})
		},
		UpdateDocumentStylePreset: func(payload cmdpkg.DocumentStylePresetUpdatePayload) error {
			return inst.updateDocumentStylePreset(UpdateDocumentStylePresetPayload{
				PresetID: payload.PresetID,
				Name:     payload.Name,
				Styles:   layerStylePayloadsFromCommand(payload.Styles),
			})
		},
		DeleteDocumentStylePreset: func(presetID string) error {
			return inst.deleteDocumentStylePreset(DeleteDocumentStylePresetPayload{PresetID: presetID})
		},
		ApplyDocumentStylePreset: func(payload cmdpkg.DocumentStylePresetApplyPayload) error {
			return inst.applyDocumentStylePreset(ApplyDocumentStylePresetPayload{
				PresetID: payload.PresetID,
				LayerID:  payload.LayerID,
			})
		},
		SetArtboard: func(payload cmdpkg.LayerArtboardPayload) error {
			return inst.executeDocCommand("Update artboard", func(doc *Document) error {
				return doc.SetArtboard(payload.LayerID, LayerBounds(payload.Bounds), payload.Background)
			})
		},
		SetPointFromSample: func(payload cmdpkg.LayerSetPointFromSamplePayload) error {
			return inst.setPointFromSample(SetPointFromSamplePayload{
				LayerID: payload.LayerID,
				X:       payload.X,
				Y:       payload.Y,
				Mode:    payload.Mode,
			})
		},
	})
}

func layerStylePayloadsToStyles(payloads []LayerStylePayload) []LayerStyle {
	if payloads == nil {
		return nil
	}
	styles := make([]LayerStyle, 0, len(payloads))
	for _, payload := range payloads {
		if payload.Kind == LayerStyleKindBlendIf {
			continue
		}
		styles = append(styles, LayerStyle{
			Kind:    string(payload.Kind),
			Enabled: payload.Enabled,
			Params:  cloneJSONRawMessage(payload.Params),
		})
	}
	return styles
}

func layerStylePayloadsFromCommand(payloads []cmdpkg.LayerStylePayload) []LayerStylePayload {
	if payloads == nil {
		return nil
	}
	styles := make([]LayerStylePayload, 0, len(payloads))
	for _, payload := range payloads {
		styles = append(styles, LayerStylePayload{
			Kind:    LayerStyleKind(payload.Kind),
			Enabled: payload.Enabled,
			Params:  cloneJSONRawMessage(payload.Params),
		})
	}
	return styles
}
