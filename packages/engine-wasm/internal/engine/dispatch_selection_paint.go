package engine

import (
	"fmt"
	"math"
	"strings"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

func (inst *instance) dispatchSelectionPaintCommand(commandID int32, payloadJSON string, suggestedPath []SelectionPoint) (bool, *RenderResult, []SelectionPoint, error) {
	if handled, err := cmdpkg.DispatchSelection(commandID, payloadJSON, cmdpkg.SelectionDeps{
		Decode: decodePayloadAny,
		PickLayerAtPoint: func(x, y int) error {
			doc := inst.manager.Active()
			if doc == nil {
				return fmt.Errorf("no active document")
			}
			if _, err := doc.PickLayerAtPoint(x, y); err != nil {
				return err
			}
			return inst.manager.ReplaceActive(doc)
		},
		CreateSelection: func(shape, mode string, rect cmdpkg.SelectionRect, polygon []cmdpkg.SelectionPoint, antiAlias bool) error {
			enginePolygon := make([]SelectionPoint, len(polygon))
			for i := range polygon {
				enginePolygon[i] = SelectionPoint{X: polygon[i].X, Y: polygon[i].Y}
			}
			return inst.executeDocCommand("Set selection", func(doc *Document) error {
				return doc.CreateSelection(
					SelectionShape(shape),
					LayerBounds{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H},
					enginePolygon,
					SelectionCombineMode(mode),
					antiAlias,
				)
			})
		},
		SelectAll: func() error {
			return inst.executeDocCommand("Select all", func(doc *Document) error {
				doc.Selection = docpkg.SelectAll(doc.Width, doc.Height)
				return nil
			})
		},
		Deselect: func() error {
			return inst.executeDocCommand("Deselect", func(doc *Document) error {
				doc.Selection, doc.LastSelection = docpkg.Deselect(doc.Selection, doc.LastSelection)
				return nil
			})
		},
		Reselect: func() error {
			return inst.executeDocCommand("Reselect", func(doc *Document) error {
				selection, err := docpkg.Reselect(doc.LastSelection)
				if err != nil {
					return err
				}
				doc.Selection = selection
				return nil
			})
		},
		InvertSelection: func() error {
			return inst.executeDocCommand("Invert selection", func(doc *Document) error {
				doc.Selection = docpkg.InvertSelection(doc.Selection, doc.Width, doc.Height)
				return nil
			})
		},
		FeatherSelection: func(radius float64) error {
			return inst.executeDocCommand("Feather selection", func(doc *Document) error {
				return doc.FeatherSelection(radius)
			})
		},
		ExpandSelection: func(pixels int) error {
			return inst.executeDocCommand("Expand selection", func(doc *Document) error {
				return doc.ExpandSelection(pixels)
			})
		},
		ContractSelection: func(pixels int) error {
			return inst.executeDocCommand("Contract selection", func(doc *Document) error {
				return doc.ContractSelection(pixels)
			})
		},
		SmoothSelection: func(radius int) error {
			return inst.executeDocCommand("Smooth selection", func(doc *Document) error {
				return doc.SmoothSelection(radius)
			})
		},
		BorderSelection: func(width int) error {
			return inst.executeDocCommand("Border selection", func(doc *Document) error {
				return doc.BorderSelection(width)
			})
		},
		TransformSelection: func(a, b, c, d, tx, ty float64) error {
			return inst.executeDocCommand("Transform selection", func(doc *Document) error {
				return doc.TransformSelection(a, b, c, d, tx, ty)
			})
		},
		SelectColorRange: func(layerID string, targetColor [4]uint8, fuzziness float64, sampleMerged bool, mode string) error {
			return inst.executeDocCommand("Color range selection", func(doc *Document) error {
				return doc.SelectColorRange(layerID, targetColor, fuzziness, sampleMerged, SelectionCombineMode(mode))
			})
		},
		QuickSelect: func(x, y int, tolerance, edgeSensitivity float64, layerID string, sampleMerged bool, mode string) error {
			return inst.executeDocCommand("Quick selection", func(doc *Document) error {
				return doc.QuickSelect(x, y, tolerance, edgeSensitivity, layerID, sampleMerged, SelectionCombineMode(mode))
			})
		},
		MagicWand: func(x, y int, tolerance float64, layerID string, sampleMerged, contiguous, antiAlias bool, mode string) error {
			return inst.executeDocCommand("Magic wand selection", func(doc *Document) error {
				return doc.MagicWand(x, y, tolerance, layerID, sampleMerged, contiguous, antiAlias, SelectionCombineMode(mode))
			})
		},
		SaveSelectionToChannel: func(name string) error {
			return inst.executeDocCommand("Save selection to channel", func(doc *Document) error {
				channels, err := docpkg.SaveSelectionToChannel(doc.Selection, doc.SavedSelections, name)
				if err != nil {
					return err
				}
				doc.SavedSelections = channels
				return nil
			})
		},
		LoadSelectionFromChannel: func(name, mode string) error {
			return inst.executeDocCommand("Load selection from channel", func(doc *Document) error {
				selection, err := docpkg.LoadSelectionFromChannel(doc.Selection, doc.SavedSelections, name, func(current, next *Selection) *Selection {
					return combineSelection(current, next, SelectionCombineMode(mode))
				})
				if err != nil {
					return err
				}
				doc.Selection = selection
				return nil
			})
		},
		RefineSelection: func(smartRadius, contrast float64, layerID string, sampleMerged bool) error {
			return inst.executeDocCommand("Refine selection", func(doc *Document) error {
				return doc.RefineSelectionEdges(smartRadius, contrast, layerID, sampleMerged)
			})
		},
		OutputSelection: func(mode, layerID, name string, sampleMerged bool) error {
			command := &snapshotCommand{
				description: "Output selection",
				applyFn: func(inst *instance) (snapshot, error) {
					doc := inst.manager.Active()
					if doc == nil {
						return snapshot{}, fmt.Errorf("no active document")
					}
					if err := inst.outputSelection(doc, OutputSelectionPayload{
						Mode:         OutputSelectionMode(mode),
						LayerID:      layerID,
						Name:         name,
						SampleMerged: sampleMerged,
					}); err != nil {
						return snapshot{}, err
					}
					return inst.captureSnapshot(), nil
				},
			}
			return inst.history.Execute(inst, command)
		},
	}); handled || err != nil {
		return handled, nil, suggestedPath, err
	}
	if handled, err := cmdpkg.DispatchPaint(commandID, payloadJSON, cmdpkg.PaintDeps{
		Decode: decodePayloadAny,
		BeginPaintStroke: func(payload cmdpkg.PaintBeginStrokePayload) error {
			inst.handleBeginPaintStroke(BeginPaintStrokePayload{
				X:        payload.X,
				Y:        payload.Y,
				Pressure: payload.Pressure,
				TiltX:    payload.TiltX,
				TiltY:    payload.TiltY,
				Brush: BrushParams{
					Size:             payload.Brush.Size,
					Hardness:         payload.Brush.Hardness,
					Flow:             payload.Brush.Flow,
					Color:            payload.Brush.Color,
					BlendMode:        payload.Brush.BlendMode,
					WetEdges:         payload.Brush.WetEdges,
					Scatter:          payload.Brush.Scatter,
					Stabilizer:       payload.Brush.Stabilizer,
					SampleMerged:     payload.Brush.SampleMerged,
					AutoErase:        payload.Brush.AutoErase,
					Erase:            payload.Brush.Erase,
					EraseBackground:  payload.Brush.EraseBackground,
					EraseTolerance:   payload.Brush.EraseTolerance,
					MixerBrush:       payload.Brush.MixerBrush,
					MixerMix:         payload.Brush.MixerMix,
					MixerWetness:     payload.Brush.MixerWetness,
					MixerLoad:        payload.Brush.MixerLoad,
					CloneStamp:       payload.Brush.CloneStamp,
					CloneSourceX:     payload.Brush.CloneSourceX,
					CloneSourceY:     payload.Brush.CloneSourceY,
					CloneAligned:     payload.Brush.CloneAligned,
					CloneOpacity:     payload.Brush.CloneOpacity,
					CloneLoad:        payload.Brush.CloneLoad,
					CloneHistory:     payload.Brush.CloneHistory,
					CloneHistoryIdx:  payload.Brush.CloneHistoryIdx,
					HistoryBrush:     payload.Brush.HistoryBrush,
					HistorySourceIdx: payload.Brush.HistorySourceIdx,
					HistoryOpacity:   payload.Brush.HistoryOpacity,
					HistoryLoad:      payload.Brush.HistoryLoad,
					PressureSize:     payload.Brush.PressureSize,
					PressureOpacity:  payload.Brush.PressureOpacity,
					PressureFlow:     payload.Brush.PressureFlow,
				},
			})
			return nil
		},
		ContinuePaintStroke: func(payload cmdpkg.PaintContinueStrokePayload) error {
			inst.handleContinuePaintStroke(ContinuePaintStrokePayload{
				X:        payload.X,
				Y:        payload.Y,
				Pressure: payload.Pressure,
				TiltX:    payload.TiltX,
				TiltY:    payload.TiltY,
			})
			return nil
		},
		EndPaintStroke: func() error {
			inst.handleEndPaintStroke()
			return nil
		},
		SetForegroundColor: func(color [4]uint8) error {
			inst.foregroundColor = color
			return nil
		},
		SetBackgroundColor: func(color [4]uint8) error {
			inst.backgroundColor = color
			return nil
		},
		MagicErase: func(payload cmdpkg.PaintMagicErasePayload) error {
			doc := inst.manager.Active()
			if doc == nil {
				return nil
			}
			layer := findPixelLayer(doc, doc.ActiveLayerID)
			if layer == nil {
				return nil
			}
			return inst.handleMagicErase(MagicErasePayload{
				X:            payload.X,
				Y:            payload.Y,
				Tolerance:    payload.Tolerance,
				Contiguous:   payload.Contiguous,
				SampleMerged: payload.SampleMerged,
			}, doc, layer)
		},
		Fill: func(payload cmdpkg.PaintFillPayload) error {
			doc := inst.manager.Active()
			if doc == nil {
				return nil
			}
			return inst.handleFill(FillPayload{
				HasPoint:     payload.HasPoint,
				X:            payload.X,
				Y:            payload.Y,
				Tolerance:    payload.Tolerance,
				Contiguous:   payload.Contiguous,
				SampleMerged: payload.SampleMerged,
				Source:       payload.Source,
				Color:        payload.Color,
				CreateLayer:  payload.CreateLayer,
			})
		},
		ApplyGradient: func(payload cmdpkg.PaintApplyGradientPayload) error {
			doc := inst.manager.Active()
			if doc == nil {
				return nil
			}
			stops := make([]GradientStopPayload, len(payload.Stops))
			for i := range payload.Stops {
				stops[i] = GradientStopPayload{
					Position: payload.Stops[i].Position,
					Color:    payload.Stops[i].Color,
				}
			}
			return inst.handleApplyGradient(ApplyGradientPayload{
				StartX:      payload.StartX,
				StartY:      payload.StartY,
				EndX:        payload.EndX,
				EndY:        payload.EndY,
				Type:        GradientType(payload.Type),
				Reverse:     payload.Reverse,
				Dither:      payload.Dither,
				CreateLayer: payload.CreateLayer,
				Stops:       stops,
			})
		},
		ResetMixerBrushState: func() error {
			inst.resetMixerBrushState()
			return nil
		},
	}); handled || err != nil {
		return handled, nil, suggestedPath, err
	}
	if handled, response, err := cmdpkg.DispatchSelectionPaintRender(commandID, payloadJSON, cmdpkg.SelectionPaintRenderDeps{
		Decode: decodePayloadAny,
		MagneticLassoSuggestPath: func(payload cmdpkg.SelectionPaintSuggestedPathPayload) (*cmdpkg.SelectionPaintRenderResponse, error) {
			doc := inst.manager.Active()
			if doc == nil {
				return nil, fmt.Errorf("no active document")
			}
			surface, err := doc.selectionSourceSurface(payload.LayerID, payload.SampleMerged)
			if err != nil {
				return nil, err
			}
			engineSuggestedPath := suggestMagneticPath(surface, doc.Width, doc.Height, payload.X1, payload.Y1, payload.X2, payload.Y2)
			commandSuggestedPath := make([]cmdpkg.SelectionPoint, len(engineSuggestedPath))
			for i := range engineSuggestedPath {
				commandSuggestedPath[i] = cmdpkg.SelectionPoint{X: engineSuggestedPath[i].X, Y: engineSuggestedPath[i].Y}
			}
			return &cmdpkg.SelectionPaintRenderResponse{SuggestedPath: commandSuggestedPath}, nil
		},
		SampleMergedColor: func(payload cmdpkg.SelectionPaintSampleColorPayload) (*cmdpkg.SelectionPaintRenderResponse, error) {
			doc := inst.manager.Active()
			if doc == nil {
				return nil, nil
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
			px := int(math.Round(payload.X)) - offsetX
			py := int(math.Round(payload.Y)) - offsetY
			if surface == nil || px < 0 || py < 0 || px >= width || py >= height {
				return nil, nil
			}
			sampleSize := payload.SampleSize
			if sampleSize <= 0 {
				sampleSize = 1
			}
			color, ok := sampleSurfaceColorAverage(surface, width, height, px, py, sampleSize)
			if !ok {
				return nil, nil
			}
			return &cmdpkg.SelectionPaintRenderResponse{SampledColor: &color}, nil
		},
	}); handled || err != nil {
		if err != nil || response == nil {
			return handled, nil, suggestedPath, err
		}
		result := inst.render()
		if response.SuggestedPath != nil {
			suggestedPath = make([]SelectionPoint, len(response.SuggestedPath))
			for i := range response.SuggestedPath {
				suggestedPath[i] = SelectionPoint{X: response.SuggestedPath[i].X, Y: response.SuggestedPath[i].Y}
			}
		}
		result.SuggestedPath = suggestedPath
		result.SampledColor = response.SampledColor
		return handled, &result, suggestedPath, nil
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
