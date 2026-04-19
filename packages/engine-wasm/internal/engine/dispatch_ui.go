package engine

import (
	"fmt"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
)

func (inst *instance) dispatchUICommand(commandID int32, payloadJSON string) (bool, *RenderResult, error) {
	result, err := cmdpkg.DispatchUI(commandID, payloadJSON, cmdpkg.UIDeps{
		Decode:                   decodePayloadAny,
		DefaultSelectionViewMode: string(SelectionViewModeMarchingAnts),
		GenerateThumbnails: func() (map[string]ThumbnailEntry, error) {
			doc := inst.manager.Active()
			if doc == nil {
				return nil, nil
			}
			return doc.generateAllThumbnails(thumbnailSize, thumbnailSize)
		},
		ComputeHistogram: func(payloadJSON string) (any, error) {
			return inst.computeHistogram(payloadJSON)
		},
		IdentifyHueRange: inst.identifyHueRange,
	})
	if err != nil {
		return result.Handled, nil, err
	}

	if !result.Handled {
		return false, nil, nil
	}
	if result.MaskEditLayerID != nil {
		inst.maskEditLayerID = *result.MaskEditLayerID
	}
	if result.SelectionViewMode != nil {
		inst.selectionViewMode = SelectionViewMode(*result.SelectionViewMode)
	}
	if !result.HasCustomRender {
		return true, nil, nil
	}

	renderResult := inst.render()
	if result.Thumbnails != nil {
		renderResult.Thumbnails = result.Thumbnails
	}
	if result.Histogram != nil {
		histogram, ok := result.Histogram.(*HistogramData)
		if !ok {
			return true, nil, fmt.Errorf("unexpected histogram result type %T", result.Histogram)
		}
		renderResult.Histogram = histogram
	}
	renderResult.IdentifiedHueRange = result.IdentifiedHueRange
	return true, &renderResult, nil
}
