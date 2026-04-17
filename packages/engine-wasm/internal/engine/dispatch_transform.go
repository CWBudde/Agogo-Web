package engine

import (
	"fmt"
	"math"
)

func (inst *instance) dispatchTransformCommand(commandID int32, payloadJSON string) (bool, error) {
	switch commandID {
	case commandBeginFreeTransform:
		var payload BeginFreeTransformPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		layerID := payload.LayerID
		if layerID == "" {
			layerID = doc.ActiveLayerID
		}
		layer := doc.findLayer(layerID)
		if layer == nil {
			return true, fmt.Errorf("layer %q not found", layerID)
		}
		pl, ok := layer.(*PixelLayer)
		if !ok {
			return true, fmt.Errorf("free transform only supported on pixel layers")
		}
		if sel := doc.Selection; sel != nil {
			floatPixels, floatBounds, hasContent := extractSelectionContent(pl, sel)
			if hasContent {
				preBegin := inst.captureSnapshot()
				origSrcPixels := append([]byte(nil), pl.Pixels...)
				origSrcBounds := pl.Bounds
				clearSelectionContent(pl, sel)
				floatingLayer := NewPixelLayer("Floating Selection", floatBounds, floatPixels)
				if _, srcParent, srcIndex, ok2 := findLayerByID(doc.ensureLayerRoot(), layerID); ok2 {
					insertChild(srcParent, floatingLayer, srcIndex+1)
				}
				doc.ActiveLayerID = floatingLayer.ID()
				doc.ContentVersion++
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return true, err
				}
				inst.cachedDocContentVersion = -1
				inst.freeTransform = &FreeTransformState{
					Active:               true,
					LayerID:              floatingLayer.ID(),
					OriginalPixels:       append([]byte(nil), floatPixels...),
					OriginalBounds:       floatBounds,
					IsFloating:           true,
					SourceLayerID:        layerID,
					OriginalSourcePixels: origSrcPixels,
					OriginalSourceBounds: origSrcBounds,
					PreBeginSnapshot:     &preBegin,
					A:                    1, B: 0, C: 0, D: 1,
					TX:            float64(floatBounds.X),
					TY:            float64(floatBounds.Y),
					PivotX:        float64(floatBounds.X) + float64(floatBounds.W)*0.5,
					PivotY:        float64(floatBounds.Y) + float64(floatBounds.H)*0.5,
					Interpolation: InterpolBilinear,
				}
				if payload.Mode == "warp" {
					inst.freeTransform.WarpGrid = initWarpGridFromBounds(floatBounds)
				}
				return true, nil
			}
		}
		inst.freeTransform = &FreeTransformState{
			Active:         true,
			LayerID:        layerID,
			OriginalPixels: append([]byte(nil), pl.Pixels...),
			OriginalBounds: pl.Bounds,
			A:              1, B: 0, C: 0, D: 1,
			TX:            float64(pl.Bounds.X),
			TY:            float64(pl.Bounds.Y),
			PivotX:        float64(pl.Bounds.X) + float64(pl.Bounds.W)*0.5,
			PivotY:        float64(pl.Bounds.Y) + float64(pl.Bounds.H)*0.5,
			Interpolation: InterpolBilinear,
		}
		if payload.Mode == "warp" {
			inst.freeTransform.WarpGrid = initWarpGridFromBounds(pl.Bounds)
		}
		return true, nil

	case commandUpdateFreeTransform:
		var payload UpdateFreeTransformPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if inst.freeTransform == nil || !inst.freeTransform.Active {
			return true, fmt.Errorf("no active free transform")
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		layer := doc.findLayer(inst.freeTransform.LayerID)
		pl, ok := layer.(*PixelLayer)
		if !ok || pl == nil {
			return true, fmt.Errorf("transform layer not found or wrong type")
		}
		inst.freeTransform.A = payload.A
		inst.freeTransform.B = payload.B
		inst.freeTransform.C = payload.C
		inst.freeTransform.D = payload.D
		inst.freeTransform.TX = payload.TX
		inst.freeTransform.TY = payload.TY
		inst.freeTransform.PivotX = payload.PivotX
		inst.freeTransform.PivotY = payload.PivotY
		if payload.Interpolation != "" {
			inst.freeTransform.Interpolation = InterpolMode(payload.Interpolation)
		}
		if payload.WarpGrid != nil {
			inst.freeTransform.WarpGrid = payload.WarpGrid
			inst.freeTransform.DistortCorners = nil
		} else if payload.Corners != nil {
			inst.freeTransform.DistortCorners = payload.Corners
			inst.freeTransform.WarpGrid = nil
		} else {
			inst.freeTransform.DistortCorners = nil
			inst.freeTransform.WarpGrid = nil
		}
		previewPixels, previewBounds := applyPixelTransform(inst.freeTransform, InterpolBilinear)
		pl.Pixels = previewPixels
		pl.Bounds = previewBounds
		doc.ContentVersion++
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return true, err
		}
		inst.cachedDocContentVersion = -1
		return true, nil

	case commandCommitFreeTransform:
		if inst.freeTransform == nil || !inst.freeTransform.Active {
			return true, fmt.Errorf("no active free transform")
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		layer := doc.findLayer(inst.freeTransform.LayerID)
		pl, ok := layer.(*PixelLayer)
		if !ok || pl == nil {
			return true, fmt.Errorf("transform layer not found or wrong type")
		}
		finalPixels, finalBounds := applyPixelTransform(inst.freeTransform, inst.freeTransform.Interpolation)
		ft := inst.freeTransform
		if ft.IsFloating {
			if err := inst.restoreSnapshot(*ft.PreBeginSnapshot); err != nil {
				return true, err
			}
			command := &snapshotCommand{
				description: "Transform Selection",
				applyFn: func(inst *instance) (snapshot, error) {
					d := inst.manager.Active()
					if d == nil {
						return snapshot{}, fmt.Errorf("no active document")
					}
					srcLayer := d.findLayer(ft.SourceLayerID)
					sl, ok := srcLayer.(*PixelLayer)
					if !ok || sl == nil {
						return snapshot{}, fmt.Errorf("source layer not found")
					}
					mergePixelLayerOnto(sl, finalPixels, finalBounds)
					d.Selection = nil
					d.ActiveLayerID = ft.SourceLayerID
					d.ContentVersion++
					if err := inst.manager.ReplaceActive(d); err != nil {
						return snapshot{}, err
					}
					return inst.captureSnapshot(), nil
				},
			}
			if err := inst.history.Execute(inst, command); err != nil {
				return true, err
			}
			inst.lastTransform = recordLastFreeTransform(ft)
			inst.freeTransform = nil
			inst.cachedDocContentVersion = -1
			return true, nil
		}
		command := &snapshotCommand{
			description: "Free Transform",
			applyFn: func(inst *instance) (snapshot, error) {
				d := inst.manager.Active()
				if d == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				l := d.findLayer(ft.LayerID)
				p, ok := l.(*PixelLayer)
				if !ok || p == nil {
					return snapshot{}, fmt.Errorf("layer not found")
				}
				p.Pixels = finalPixels
				p.Bounds = finalBounds
				d.ContentVersion++
				if err := inst.manager.ReplaceActive(d); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		inst.lastTransform = recordLastFreeTransform(ft)
		inst.freeTransform = nil
		inst.cachedDocContentVersion = -1
		return true, nil

	case commandCancelFreeTransform:
		if inst.freeTransform == nil || !inst.freeTransform.Active {
			inst.freeTransform = nil
			return true, nil
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		ft := inst.freeTransform
		if ft.IsFloating {
			if srcLayer := doc.findLayer(ft.SourceLayerID); srcLayer != nil {
				if sl, ok := srcLayer.(*PixelLayer); ok {
					sl.Pixels = ft.OriginalSourcePixels
					sl.Bounds = ft.OriginalSourceBounds
				}
			}
			_ = doc.DeleteLayer(ft.LayerID)
			doc.ActiveLayerID = ft.SourceLayerID
		} else {
			layer := doc.findLayer(ft.LayerID)
			if pl, ok := layer.(*PixelLayer); ok && pl != nil {
				pl.Pixels = ft.OriginalPixels
				pl.Bounds = ft.OriginalBounds
			}
		}
		doc.ContentVersion++
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return true, err
		}
		inst.freeTransform = nil
		inst.cachedDocContentVersion = -1
		return true, nil

	case commandFlipLayerH, commandFlipLayerV,
		commandRotateLayer90CW, commandRotateLayer90CCW, commandRotateLayer180:
		var payload DiscreteTransformPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		kind := map[int32]string{
			commandFlipLayerH:       "flipH",
			commandFlipLayerV:       "flipV",
			commandRotateLayer90CW:  "rotate90cw",
			commandRotateLayer90CCW: "rotate90ccw",
			commandRotateLayer180:   "rotate180",
		}[commandID]
		command := &snapshotCommand{
			description: kindDescription(kind),
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				layerID := payload.LayerID
				if layerID == "" {
					layerID = doc.ActiveLayerID
				}
				l := doc.findLayer(layerID)
				pl, ok := l.(*PixelLayer)
				if !ok || pl == nil {
					return snapshot{}, fmt.Errorf("layer %q is not a pixel layer", layerID)
				}
				applyDiscreteTransformToLayer(pl, kind)
				doc.ContentVersion++
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		inst.lastTransform = &LastTransformRecord{Kind: kind}
		inst.cachedDocContentVersion = -1
		return true, nil

	case commandTransformAgain:
		if inst.lastTransform == nil {
			return true, fmt.Errorf("no previous transform to repeat")
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		lt := inst.lastTransform
		if lt.Kind == "free" {
			l := doc.findLayer(doc.ActiveLayerID)
			pl, ok := l.(*PixelLayer)
			if !ok || pl == nil {
				return true, fmt.Errorf("active layer is not a pixel layer")
			}
			finalPixels, finalBounds := applyLastFreeTransform(lt, pl)
			command := &snapshotCommand{
				description: "Transform Again",
				applyFn: func(inst *instance) (snapshot, error) {
					d := inst.manager.Active()
					if d == nil {
						return snapshot{}, fmt.Errorf("no active document")
					}
					layer := d.findLayer(d.ActiveLayerID)
					p, ok := layer.(*PixelLayer)
					if !ok || p == nil {
						return snapshot{}, fmt.Errorf("layer not found")
					}
					p.Pixels = finalPixels
					p.Bounds = finalBounds
					d.ContentVersion++
					if err := inst.manager.ReplaceActive(d); err != nil {
						return snapshot{}, err
					}
					return inst.captureSnapshot(), nil
				},
			}
			if err := inst.history.Execute(inst, command); err != nil {
				return true, err
			}
		} else {
			kind := lt.Kind
			command := &snapshotCommand{
				description: kindDescription(kind) + " Again",
				applyFn: func(inst *instance) (snapshot, error) {
					d := inst.manager.Active()
					if d == nil {
						return snapshot{}, fmt.Errorf("no active document")
					}
					layer := d.findLayer(d.ActiveLayerID)
					p, ok := layer.(*PixelLayer)
					if !ok || p == nil {
						return snapshot{}, fmt.Errorf("active layer is not a pixel layer")
					}
					applyDiscreteTransformToLayer(p, kind)
					d.ContentVersion++
					if err := inst.manager.ReplaceActive(d); err != nil {
						return snapshot{}, err
					}
					return inst.captureSnapshot(), nil
				},
			}
			if err := inst.history.Execute(inst, command); err != nil {
				return true, err
			}
		}
		inst.cachedDocContentVersion = -1
		return true, nil

	case commandBeginCrop:
		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("no active document")
		}
		inst.crop = &CropState{
			Active:           true,
			X:                0,
			Y:                0,
			W:                float64(doc.Width),
			H:                float64(doc.Height),
			Rotation:         0,
			DeletePixels:     false,
			ContentAwareFill: false,
			Resolution:       normalizeCropResolution(doc.Resolution, defaultResolutionDPI),
			OverlayType:      cropOverlayThirds,
		}
		return true, nil

	case commandUpdateCrop:
		var payload UpdateCropPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		if inst.crop == nil || !inst.crop.Active {
			return true, fmt.Errorf("no active crop tool")
		}
		inst.crop.X = payload.X
		inst.crop.Y = payload.Y
		inst.crop.W = payload.W
		inst.crop.H = payload.H
		inst.crop.Rotation = payload.Rotation
		inst.crop.DeletePixels = payload.DeletePixels
		inst.crop.ContentAwareFill = payload.ContentAwareFill
		inst.crop.Resolution = normalizeCropResolution(payload.Resolution, inst.crop.Resolution)
		inst.crop.OverlayType = normalizeCropOverlayType(payload.OverlayType)
		return true, nil

	case commandCommitCrop:
		if inst.crop == nil || !inst.crop.Active {
			return true, fmt.Errorf("no active crop tool")
		}
		cropX := inst.crop.X
		cropY := inst.crop.Y
		cropW := inst.crop.W
		cropH := inst.crop.H
		cropRot := inst.crop.Rotation
		deletePixels := inst.crop.DeletePixels
		contentAwareFill := inst.crop.ContentAwareFill
		cropResolution := normalizeCropResolution(inst.crop.Resolution, defaultResolutionDPI)
		command := &snapshotCommand{
			description: "Crop Document",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				preCropSurface := doc.renderCompositeSurface()
				preCropWidth := doc.Width
				preCropHeight := doc.Height
				activeLayerID := doc.ActiveLayerID

				w := int(math.Round(cropW))
				h := int(math.Round(cropH))
				if w <= 0 || h <= 0 {
					return snapshot{}, fmt.Errorf("invalid crop dimensions: %dx%d", w, h)
				}
				rotRad := cropRot * math.Pi / 180
				cx := cropX + cropW/2
				cy := cropY + cropH/2
				if cropRot != 0 {
					walkLayerTree(doc.LayerRoot, func(n LayerNode) {
						if pl, ok := n.(*PixelLayer); ok {
							newPixels, newBounds := applyRotatedCropToPixelLayer(pl, cx, cy, cropW, cropH, rotRad)
							pl.Pixels = newPixels
							pl.Bounds = newBounds
						}
					})
				} else {
					x := int(math.Round(cropX))
					y := int(math.Round(cropY))
					walkLayerTree(doc.LayerRoot, func(n LayerNode) {
						if pl, ok := n.(*PixelLayer); ok {
							pl.Bounds.X -= x
							pl.Bounds.Y -= y
							if deletePixels {
								trimPixelLayerToBounds(pl, w, h)
							}
						}
					})
				}
				doc.Width = w
				doc.Height = h
				doc.Resolution = cropResolution
				if contentAwareFill {
					fillPixels, ok := buildContentAwareCropFillLayer(preCropSurface, preCropWidth, preCropHeight, cropX, cropY, cropW, cropH, rotRad)
					if ok {
						fillLayer := NewPixelLayer("Content-Aware Crop Fill", LayerBounds{X: 0, Y: 0, W: w, H: h}, fillPixels)
						if err := doc.AddLayer(fillLayer, doc.ensureLayerRoot().ID(), -1); err != nil {
							return snapshot{}, err
						}
						doc.ActiveLayerID = activeLayerID
					}
				}
				doc.ContentVersion++
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		inst.crop = nil
		inst.cachedDocContentVersion = -1
		return true, nil

	case commandCancelCrop:
		inst.crop = nil
		return true, nil

	case commandResizeCanvas:
		var payload ResizeCanvasPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		command := &snapshotCommand{
			description: "Canvas Size",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := applyResizeCanvas(doc, payload.Width, payload.Height, payload.Anchor); err != nil {
					return snapshot{}, err
				}
				doc.ContentVersion++
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, err
		}
		inst.cachedDocContentVersion = -1
		return true, nil
	}

	return false, nil
}
