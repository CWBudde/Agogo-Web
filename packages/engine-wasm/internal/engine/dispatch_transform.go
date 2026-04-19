package engine

import (
	"fmt"
	"math"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
)

func (inst *instance) dispatchTransformCommand(commandID int32, payloadJSON string) (bool, error) {
	return cmdpkg.DispatchTransform(commandID, payloadJSON, cmdpkg.TransformDeps{
		Decode: decodePayloadAny,
		BeginFreeTransform: func(payload cmdpkg.TransformBeginFreePayload) error {
			return inst.beginFreeTransform(BeginFreeTransformPayload{
				LayerID: payload.LayerID,
				Mode:    payload.Mode,
			})
		},
		UpdateFreeTransform: func(payload cmdpkg.TransformUpdateFreePayload) error {
			return inst.updateFreeTransform(UpdateFreeTransformPayload{
				A:             payload.A,
				B:             payload.B,
				C:             payload.C,
				D:             payload.D,
				TX:            payload.TX,
				TY:            payload.TY,
				PivotX:        payload.PivotX,
				PivotY:        payload.PivotY,
				Interpolation: payload.Interpolation,
				Corners:       payload.Corners,
				WarpGrid:      payload.WarpGrid,
			})
		},
		CommitFreeTransform: inst.commitFreeTransform,
		CancelFreeTransform: inst.cancelFreeTransform,
		DiscreteTransform:   inst.applyDiscreteTransform,
		TransformAgain:      inst.transformAgain,
		BeginCrop:           inst.beginCrop,
		UpdateCrop: func(payload cmdpkg.TransformUpdateCropPayload) error {
			return inst.updateCrop(UpdateCropPayload{
				X:                payload.X,
				Y:                payload.Y,
				W:                payload.W,
				H:                payload.H,
				Rotation:         payload.Rotation,
				DeletePixels:     payload.DeletePixels,
				ContentAwareFill: payload.ContentAwareFill,
				Resolution:       payload.Resolution,
				OverlayType:      payload.OverlayType,
			})
		},
		CommitCrop: inst.commitCrop,
		CancelCrop: inst.cancelCrop,
		ResizeCanvas: func(payload cmdpkg.TransformResizeCanvasPayload) error {
			return inst.resizeCanvas(ResizeCanvasPayload{
				Width:  payload.Width,
				Height: payload.Height,
				Anchor: payload.Anchor,
			})
		},
	})
}

func (inst *instance) beginFreeTransform(payload BeginFreeTransformPayload) error {
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layerID := payload.LayerID
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	layer := doc.findLayer(layerID)
	if layer == nil {
		return fmt.Errorf("layer %q not found", layerID)
	}
	pl, ok := layer.(*PixelLayer)
	if !ok {
		return fmt.Errorf("free transform only supported on pixel layers")
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
			doc.touchModifiedAtRect(DirtyRect(floatBounds))
			if err := inst.manager.ReplaceActive(doc); err != nil {
				return err
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
				A:                    1,
				B:                    0,
				C:                    0,
				D:                    1,
				TX:                   float64(floatBounds.X),
				TY:                   float64(floatBounds.Y),
				PivotX:               float64(floatBounds.X) + float64(floatBounds.W)*0.5,
				PivotY:               float64(floatBounds.Y) + float64(floatBounds.H)*0.5,
				Interpolation:        InterpolBilinear,
			}
			if payload.Mode == "warp" {
				inst.freeTransform.WarpGrid = initWarpGridFromBounds(floatBounds)
			}
			return nil
		}
	}
	inst.freeTransform = &FreeTransformState{
		Active:         true,
		LayerID:        layerID,
		OriginalPixels: append([]byte(nil), pl.Pixels...),
		OriginalBounds: pl.Bounds,
		A:              1,
		B:              0,
		C:              0,
		D:              1,
		TX:             float64(pl.Bounds.X),
		TY:             float64(pl.Bounds.Y),
		PivotX:         float64(pl.Bounds.X) + float64(pl.Bounds.W)*0.5,
		PivotY:         float64(pl.Bounds.Y) + float64(pl.Bounds.H)*0.5,
		Interpolation:  InterpolBilinear,
	}
	if payload.Mode == "warp" {
		inst.freeTransform.WarpGrid = initWarpGridFromBounds(pl.Bounds)
	}
	return nil
}

func (inst *instance) updateFreeTransform(payload UpdateFreeTransformPayload) error {
	if inst.freeTransform == nil || !inst.freeTransform.Active {
		return fmt.Errorf("no active free transform")
	}
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer := doc.findLayer(inst.freeTransform.LayerID)
	pl, ok := layer.(*PixelLayer)
	if !ok || pl == nil {
		return fmt.Errorf("transform layer not found or wrong type")
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
	beforeBounds := pl.Bounds
	previewPixels, previewBounds := applyPixelTransform(inst.freeTransform, InterpolBilinear)
	pl.Pixels = previewPixels
	pl.Bounds = previewBounds
	doc.touchModifiedAtBounds(beforeBounds, previewBounds)
	if err := inst.manager.ReplaceActive(doc); err != nil {
		return err
	}
	inst.cachedDocContentVersion = -1
	return nil
}

func (inst *instance) commitFreeTransform() error {
	if inst.freeTransform == nil || !inst.freeTransform.Active {
		return fmt.Errorf("no active free transform")
	}
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer := doc.findLayer(inst.freeTransform.LayerID)
	pl, ok := layer.(*PixelLayer)
	if !ok || pl == nil {
		return fmt.Errorf("transform layer not found or wrong type")
	}
	finalPixels, finalBounds := applyPixelTransform(inst.freeTransform, inst.freeTransform.Interpolation)
	ft := inst.freeTransform
	if ft.IsFloating {
		if err := inst.restoreSnapshot(*ft.PreBeginSnapshot); err != nil {
			return err
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
				d.touchModifiedAtLayer(sl)
				if err := inst.manager.ReplaceActive(d); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return err
		}
		inst.lastTransform = recordLastFreeTransform(ft)
		inst.freeTransform = nil
		inst.cachedDocContentVersion = -1
		return nil
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
			beforeBounds := p.Bounds
			p.Pixels = finalPixels
			p.Bounds = finalBounds
			d.touchModifiedAtBounds(beforeBounds, finalBounds)
			if err := inst.manager.ReplaceActive(d); err != nil {
				return snapshot{}, err
			}
			return inst.captureSnapshot(), nil
		},
	}
	if err := inst.history.Execute(inst, command); err != nil {
		return err
	}
	inst.lastTransform = recordLastFreeTransform(ft)
	inst.freeTransform = nil
	inst.cachedDocContentVersion = -1
	return nil
}

func (inst *instance) cancelFreeTransform() error {
	if inst.freeTransform == nil || !inst.freeTransform.Active {
		inst.freeTransform = nil
		return nil
	}
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	ft := inst.freeTransform
	var floatingBounds LayerBounds
	if ft.IsFloating {
		if layer := doc.findLayer(ft.LayerID); layer != nil {
			if pl, ok := layer.(*PixelLayer); ok {
				floatingBounds = pl.Bounds
			}
		}
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
			currentBounds := pl.Bounds
			pl.Pixels = ft.OriginalPixels
			pl.Bounds = ft.OriginalBounds
			doc.touchModifiedAtBounds(currentBounds, ft.OriginalBounds)
		}
	}
	if ft.IsFloating {
		doc.touchModifiedAtBounds(floatingBounds, ft.OriginalSourceBounds)
	}
	if err := inst.manager.ReplaceActive(doc); err != nil {
		return err
	}
	inst.freeTransform = nil
	inst.cachedDocContentVersion = -1
	return nil
}

func (inst *instance) applyDiscreteTransform(kind, layerID string) error {
	command := &snapshotCommand{
		description: kindDescription(kind),
		applyFn: func(inst *instance) (snapshot, error) {
			doc := inst.manager.Active()
			if doc == nil {
				return snapshot{}, fmt.Errorf("no active document")
			}
			if layerID == "" {
				layerID = doc.ActiveLayerID
			}
			l := doc.findLayer(layerID)
			pl, ok := l.(*PixelLayer)
			if !ok || pl == nil {
				return snapshot{}, fmt.Errorf("layer %q is not a pixel layer", layerID)
			}
			beforeBounds := pl.Bounds
			applyDiscreteTransformToLayer(pl, kind)
			doc.touchModifiedAtBounds(beforeBounds, pl.Bounds)
			if err := inst.manager.ReplaceActive(doc); err != nil {
				return snapshot{}, err
			}
			return inst.captureSnapshot(), nil
		},
	}
	if err := inst.history.Execute(inst, command); err != nil {
		return err
	}
	inst.lastTransform = &LastTransformRecord{Kind: kind}
	inst.cachedDocContentVersion = -1
	return nil
}

func (inst *instance) transformAgain() error {
	if inst.lastTransform == nil {
		return fmt.Errorf("no previous transform to repeat")
	}
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	lt := inst.lastTransform
	if lt.Kind == "free" {
		l := doc.findLayer(doc.ActiveLayerID)
		pl, ok := l.(*PixelLayer)
		if !ok || pl == nil {
			return fmt.Errorf("active layer is not a pixel layer")
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
				beforeBounds := p.Bounds
				p.Pixels = finalPixels
				p.Bounds = finalBounds
				d.touchModifiedAtBounds(beforeBounds, finalBounds)
				if err := inst.manager.ReplaceActive(d); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return err
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
				beforeBounds := p.Bounds
				applyDiscreteTransformToLayer(p, kind)
				d.touchModifiedAtBounds(beforeBounds, p.Bounds)
				if err := inst.manager.ReplaceActive(d); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return err
		}
	}
	inst.cachedDocContentVersion = -1
	return nil
}

func (inst *instance) beginCrop() error {
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
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
	return nil
}

func (inst *instance) updateCrop(payload UpdateCropPayload) error {
	if inst.crop == nil || !inst.crop.Active {
		return fmt.Errorf("no active crop tool")
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
	return nil
}

func (inst *instance) commitCrop() error {
	if inst.crop == nil || !inst.crop.Active {
		return fmt.Errorf("no active crop tool")
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
			doc.touchModifiedAt()
			if err := inst.manager.ReplaceActive(doc); err != nil {
				return snapshot{}, err
			}
			return inst.captureSnapshot(), nil
		},
	}
	if err := inst.history.Execute(inst, command); err != nil {
		return err
	}
	inst.crop = nil
	inst.cachedDocContentVersion = -1
	return nil
}

func (inst *instance) cancelCrop() error {
	inst.crop = nil
	return nil
}

func (inst *instance) resizeCanvas(payload ResizeCanvasPayload) error {
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
			doc.touchModifiedAt()
			if err := inst.manager.ReplaceActive(doc); err != nil {
				return snapshot{}, err
			}
			return inst.captureSnapshot(), nil
		},
	}
	if err := inst.history.Execute(inst, command); err != nil {
		return err
	}
	inst.cachedDocContentVersion = -1
	return nil
}
