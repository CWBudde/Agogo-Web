package engine

func (inst *instance) dispatchUICommand(commandID int32, payloadJSON string) (bool, *RenderResult, error) {
	switch commandID {
	case commandSetMaskEditMode:
		var payload SetMaskEditModePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, nil, err
		}
		// Mask edit mode is UI state only and is not tracked in history.
		if payload.Editing {
			inst.maskEditLayerID = payload.LayerID
		} else {
			inst.maskEditLayerID = ""
		}
		return true, nil, nil

	case commandGetLayerThumbnails:
		// Read-only command: return a render result with thumbnails embedded.
		result := inst.render()
		doc := inst.manager.Active()
		if doc != nil {
			thumbs, err := doc.generateAllThumbnails(thumbnailSize, thumbnailSize)
			if err == nil {
				result.Thumbnails = thumbs
			}
		}
		return true, &result, nil

	case commandComputeHistogram:
		hist, err := inst.computeHistogram(payloadJSON)
		if err != nil {
			return true, nil, err
		}
		result := inst.render()
		result.Histogram = hist
		return true, &result, nil

	case commandIdentifyHueRange:
		rangeName, err := inst.identifyHueRange(payloadJSON)
		if err != nil {
			return true, nil, err
		}
		result := inst.render()
		result.IdentifiedHueRange = rangeName
		return true, &result, nil
	}

	return false, nil, nil
}
