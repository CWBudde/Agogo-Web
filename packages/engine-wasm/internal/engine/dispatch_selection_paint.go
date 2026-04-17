package engine

import (
	"fmt"
	"math"
	"strings"
)

func (inst *instance) dispatchSelectionPaintCommand(commandID int32, payloadJSON string, suggestedPath []SelectionPoint) (bool, *RenderResult, []SelectionPoint, error) {
	switch commandID {
	case commandPickLayerAtPoint:
		var payload PickLayerAtPointPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, nil, suggestedPath, fmt.Errorf("no active document")
		}
		if _, err := doc.PickLayerAtPoint(payload.X, payload.Y); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandNewSelection:
		var payload CreateSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Set selection", func(doc *Document) error {
			return doc.CreateSelection(payload.Shape, payload.Rect, payload.Polygon, payload.Mode, payload.AntiAlias)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandSelectAll:
		if err := inst.executeDocCommand("Select all", func(doc *Document) error {
			return doc.SelectAll()
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandDeselect:
		if err := inst.executeDocCommand("Deselect", func(doc *Document) error {
			return doc.Deselect()
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandReselect:
		if err := inst.executeDocCommand("Reselect", func(doc *Document) error {
			return doc.Reselect()
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandInvertSelection:
		if err := inst.executeDocCommand("Invert selection", func(doc *Document) error {
			return doc.InvertSelection()
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandFeatherSelection:
		var payload FeatherSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Feather selection", func(doc *Document) error {
			return doc.FeatherSelection(payload.Radius)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandExpandSelection:
		var payload ExpandSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Expand selection", func(doc *Document) error {
			return doc.ExpandSelection(payload.Pixels)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandContractSelection:
		var payload ContractSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Contract selection", func(doc *Document) error {
			return doc.ContractSelection(payload.Pixels)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandSmoothSelection:
		var payload SmoothSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Smooth selection", func(doc *Document) error {
			return doc.SmoothSelection(payload.Radius)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandBorderSelection:
		var payload BorderSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Border selection", func(doc *Document) error {
			return doc.BorderSelection(payload.Width)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandTransformSelection:
		var payload TransformSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Transform selection", func(doc *Document) error {
			return doc.TransformSelection(payload.A, payload.B, payload.C, payload.D, payload.TX, payload.TY)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandSelectColorRange:
		var payload SelectColorRangePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Color range selection", func(doc *Document) error {
			return doc.SelectColorRange(payload.LayerID, payload.TargetColor, payload.Fuzziness, payload.SampleMerged, payload.Mode)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandQuickSelect:
		var payload QuickSelectPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Quick selection", func(doc *Document) error {
			return doc.QuickSelect(payload.X, payload.Y, payload.Tolerance, payload.EdgeSensitivity, payload.LayerID, payload.SampleMerged, payload.Mode)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandMagicWand:
		var payload MagicWandPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Magic wand selection", func(doc *Document) error {
			return doc.MagicWand(payload.X, payload.Y, payload.Tolerance, payload.LayerID, payload.SampleMerged, payload.Contiguous, payload.AntiAlias, payload.Mode)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandMagneticLassoSuggestPath:
		var payload MagneticLassoSuggestPathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, nil, suggestedPath, fmt.Errorf("no active document")
		}
		surface, err := doc.selectionSourceSurface(payload.LayerID, payload.SampleMerged)
		if err != nil {
			return true, nil, suggestedPath, err
		}
		result := inst.render()
		suggestedPath = suggestMagneticPath(surface, doc.Width, doc.Height, payload.X1, payload.Y1, payload.X2, payload.Y2)
		result.SuggestedPath = suggestedPath
		return true, &result, suggestedPath, nil

	case commandSaveSelectionToChannel:
		var payload SaveSelectionToChannelPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Save selection to channel", func(doc *Document) error {
			return doc.SaveSelectionToChannel(payload.Name)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandLoadSelectionFromChannel:
		var payload LoadSelectionFromChannelPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Load selection from channel", func(doc *Document) error {
			return doc.LoadSelectionFromChannel(payload.Name, payload.Mode)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandRefineSelection:
		var payload RefineSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		if err := inst.executeDocCommand("Refine selection", func(doc *Document) error {
			return doc.RefineSelectionEdges(payload.SmartRadius, payload.Contrast, payload.LayerID, payload.SampleMerged)
		}); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandOutputSelection:
		var payload OutputSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		command := &snapshotCommand{
			description: "Output selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := inst.outputSelection(doc, payload); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return true, nil, suggestedPath, err
		}
		return true, nil, suggestedPath, nil

	case commandBeginPaintStroke:
		var payload BeginPaintStrokePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		inst.handleBeginPaintStroke(payload)
		return true, nil, suggestedPath, nil

	case commandContinuePaintStroke:
		var payload ContinuePaintStrokePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		inst.handleContinuePaintStroke(payload)
		return true, nil, suggestedPath, nil

	case commandEndPaintStroke:
		inst.handleEndPaintStroke()
		return true, nil, suggestedPath, nil

	case commandSetForegroundColor:
		var payload SetColorPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		inst.foregroundColor = payload.Color
		return true, nil, suggestedPath, nil

	case commandSetBackgroundColor:
		var payload SetColorPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		inst.backgroundColor = payload.Color
		return true, nil, suggestedPath, nil

	case commandSampleMergedColor:
		var payload SampleMergedColorPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return true, nil, suggestedPath, nil
		}
		var surface []byte
		var width, height int
		var offsetX, offsetY int
		if payload.SampleMerged {
			surface = inst.compositeSurface(doc)
			width, height = doc.Width, doc.Height
		} else if layer := findPixelLayer(doc, doc.ActiveLayerID); layer != nil {
			surface = layer.Pixels
			width, height = layer.Bounds.W, layer.Bounds.H
			offsetX = layer.Bounds.X
			offsetY = layer.Bounds.Y
		}
		px := int(math.Round(payload.X))
		py := int(math.Round(payload.Y))
		px -= offsetX
		py -= offsetY
		if surface != nil && px >= 0 && py >= 0 && px < width && py < height {
			sampleSize := payload.SampleSize
			if sampleSize <= 0 {
				sampleSize = 1
			}
			if color, ok := sampleSurfaceColorAverage(surface, width, height, px, py, sampleSize); ok {
				result := inst.render()
				result.SuggestedPath = suggestedPath
				result.SampledColor = &color
				return true, &result, suggestedPath, nil
			}
		}
		return true, nil, suggestedPath, nil

	case commandMagicErase:
		var payload MagicErasePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		doc := inst.manager.Active()
		if doc != nil {
			layer := findPixelLayer(doc, doc.ActiveLayerID)
			if layer != nil {
				if err := inst.handleMagicErase(payload, doc, layer); err != nil {
					return true, nil, suggestedPath, err
				}
			}
		}
		return true, nil, suggestedPath, nil

	case commandFill:
		var payload FillPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		doc := inst.manager.Active()
		if doc != nil {
			if err := inst.handleFill(payload); err != nil {
				return true, nil, suggestedPath, err
			}
		}
		return true, nil, suggestedPath, nil

	case commandApplyGradient:
		var payload ApplyGradientPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, suggestedPath, err
		}
		doc := inst.manager.Active()
		if doc != nil {
			if err := inst.handleApplyGradient(payload); err != nil {
				return true, nil, suggestedPath, err
			}
		}
		return true, nil, suggestedPath, nil
	}

	return false, nil, suggestedPath, nil
}

func (inst *instance) outputSelection(doc *Document, payload OutputSelectionPayload) error {
	mode := payload.Mode
	if mode == "" {
		mode = OutputSelectionSelection
	}
	switch mode {
	case OutputSelectionSelection:
		return nil
	case OutputSelectionLayerMask:
		layerID := payload.LayerID
		if layerID == "" {
			layerID = doc.ActiveLayerID
		}
		if layerID == "" {
			return fmt.Errorf("no active layer")
		}
		if err := doc.AddLayerMask(layerID, AddLayerMaskFromSelection); err != nil {
			return err
		}
		doc.touchModifiedAt()
		return inst.manager.ReplaceActive(doc)
	case OutputSelectionNewLayer:
		return inst.outputSelectionToNewLayer(doc, payload, false)
	case OutputSelectionNewLayerWithMask:
		return inst.outputSelectionToNewLayer(doc, payload, true)
	case OutputSelectionDocument:
		return inst.outputSelectionToDocument(doc, payload)
	default:
		return fmt.Errorf("unsupported output selection mode %q", mode)
	}
}

func (inst *instance) outputSelectionToNewLayer(doc *Document, payload OutputSelectionPayload, withMask bool) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	layerID := payload.LayerID
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	if layerID == "" {
		return fmt.Errorf("no active layer")
	}
	surface, err := doc.selectionSourceSurface(layerID, payload.SampleMerged)
	if err != nil {
		return err
	}
	bounds, ok := selectionBounds(selection)
	if !ok {
		return fmt.Errorf("selection has no bounds")
	}
	var pixels []byte
	if withMask {
		pixels = cropSurfaceBounds(surface, doc.Width, doc.Height, bounds)
	} else {
		var extractedBounds LayerBounds
		pixels, extractedBounds, ok = extractSelectionFromSurface(surface, doc.Width, doc.Height, selection)
		if !ok {
			return fmt.Errorf("selection contains no source pixels")
		}
		bounds = extractedBounds
	}
	newLayer := NewPixelLayer(outputSelectionLayerName(doc, layerID, payload.Name, withMask), bounds, pixels)
	_, parent, index, found := findLayerByID(doc.ensureLayerRoot(), layerID)
	if !found || parent == nil {
		parent = doc.ensureLayerRoot()
		index = len(parent.Children()) - 1
	}
	insertChild(parent, newLayer, index+1)
	doc.ActiveLayerID = newLayer.ID()
	if withMask {
		if err := doc.AddLayerMask(newLayer.ID(), AddLayerMaskFromSelection); err != nil {
			return err
		}
	}
	doc.touchModifiedAt()
	return inst.manager.ReplaceActive(doc)
}

func (inst *instance) outputSelectionToDocument(doc *Document, payload OutputSelectionPayload) error {
	selection := normalizeSelection(cloneSelection(doc.Selection))
	if selection == nil {
		return fmt.Errorf("no active selection")
	}
	layerID := payload.LayerID
	if layerID == "" {
		layerID = doc.ActiveLayerID
	}
	surface, err := doc.selectionSourceSurface(layerID, payload.SampleMerged)
	if err != nil {
		return err
	}
	pixels, bounds, ok := extractSelectionFromSurface(surface, doc.Width, doc.Height, selection)
	if !ok {
		return fmt.Errorf("selection contains no source pixels")
	}
	newDoc := inst.newDocument(CreateDocumentPayload{
		Name:       outputSelectionDocumentName(doc, payload.Name),
		Width:      bounds.W,
		Height:     bounds.H,
		Resolution: doc.Resolution,
		ColorMode:  doc.ColorMode,
		BitDepth:   doc.BitDepth,
		Background: "transparent",
	})
	layer := NewPixelLayer(outputSelectionLayerName(doc, layerID, "", false), LayerBounds{X: 0, Y: 0, W: bounds.W, H: bounds.H}, pixels)
	insertChild(newDoc.ensureLayerRoot(), layer, 0)
	newDoc.ActiveLayerID = layer.ID()
	inst.manager.Create(newDoc)
	inst.viewport.CenterX = float64(newDoc.Width) * 0.5
	inst.viewport.CenterY = float64(newDoc.Height) * 0.5
	inst.fitViewportToActiveDocument()
	return nil
}

func outputSelectionLayerName(doc *Document, layerID, explicitName string, withMask bool) string {
	name := strings.TrimSpace(explicitName)
	if name != "" {
		return name
	}
	if doc != nil {
		if layer := doc.findLayer(layerID); layer != nil {
			suffix := " Selection"
			if withMask {
				suffix = " Masked"
			}
			return layer.Name() + suffix
		}
	}
	if withMask {
		return "Masked Selection"
	}
	return "Selection"
}

func outputSelectionDocumentName(doc *Document, explicitName string) string {
	if name := strings.TrimSpace(explicitName); name != "" {
		return name
	}
	if doc != nil && doc.Name != "" {
		return doc.Name + " Selection"
	}
	return "Selection Document"
}
