package engine

import (
	"encoding/json"
	"fmt"
)

// ApplyFilterPayload is the JSON payload for the ApplyFilter command.
type ApplyFilterPayload struct {
	LayerID  string          `json:"layerId"`
	FilterID string          `json:"filterId"`
	Params   json.RawMessage `json:"params"`
}

// lastFilterState records the most recently applied filter so that
// ReapplyFilter can replay it without user interaction.
type lastFilterState struct {
	FilterID string
	Params   json.RawMessage
}

func (inst *instance) dispatchFilterCommand(commandID int32, payloadJSON string) (bool, error) {
	switch commandID {
	case commandApplyFilter:
		var payload ApplyFilterPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}

		// Default to active layer when no explicit layer ID is provided.
		layerID := payload.LayerID
		if layerID == "" {
			doc := inst.manager.Active()
			if doc == nil {
				return true, fmt.Errorf("apply filter: no active document")
			}
			layerID = doc.ActiveLayerID
		}

		entry := lookupFilter(payload.FilterID)
		if entry == nil {
			return true, fmt.Errorf("apply filter: unknown filter %q", payload.FilterID)
		}

		if err := inst.executeDocCommand(entry.Def.Name, func(doc *Document) error {
			return doc.ApplyFilter(layerID, payload.FilterID, payload.Params)
		}); err != nil {
			return true, err
		}

		// Remember as last filter for ReapplyFilter.
		inst.lastFilter = &lastFilterState{
			FilterID: payload.FilterID,
			Params:   payload.Params,
		}
		return true, nil

	case commandReapplyFilter:
		if inst.lastFilter == nil {
			return true, fmt.Errorf("reapply filter: no previous filter to reapply")
		}

		doc := inst.manager.Active()
		if doc == nil {
			return true, fmt.Errorf("reapply filter: no active document")
		}

		entry := lookupFilter(inst.lastFilter.FilterID)
		if entry == nil {
			return true, fmt.Errorf("reapply filter: last filter %q no longer registered", inst.lastFilter.FilterID)
		}

		layerID := doc.ActiveLayerID
		params := inst.lastFilter.Params

		if err := inst.executeDocCommand(entry.Def.Name, func(doc *Document) error {
			return doc.ApplyFilter(layerID, inst.lastFilter.FilterID, params)
		}); err != nil {
			return true, err
		}
		return true, nil

	default:
		return false, nil
	}
}
