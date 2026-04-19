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

	switch commandID {
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

	case commandResetMixerBrushState:
		inst.resetMixerBrushState()
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
