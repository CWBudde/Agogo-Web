//go:build !js

// Package psdrecords converts parsed PSD layer records into the neutral
// RecordView that psdfixture compares against.
//
// It exists as its own package because the conversion has two callers on
// opposite sides of an import cycle: internal/io/psd asserts the reader against
// the external psdRecords expectation, and internal/engine asserts the writer's
// re-export against the same expectation. psdfixture itself cannot host the
// conversion — it would have to import internal/io/psd, whose own tests import
// psdfixture — and duplicating it per caller would let the reader check and the
// writer check drift apart on exactly the mapping they both depend on.
package psdrecords

import (
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture"
)

// Views converts parsed layer records into psdfixture record views, preserving
// file order.
func Views(layers []psd.LayerRecord) []psdfixture.RecordView {
	views := make([]psdfixture.RecordView, 0, len(layers))
	for _, layer := range layers {
		channelIDs := make([]int, 0, len(layer.Channels))
		for _, channel := range layer.Channels {
			channelIDs = append(channelIDs, int(channel.ID))
		}
		views = append(views, psdfixture.RecordView{
			Name:              layer.Name,
			SectionType:       int(layer.SectionType),
			PassThrough:       layer.PassThrough,
			BlendMode:         string(layer.BlendMode),
			BlendKey:          layer.BlendKey,
			Opacity:           layer.Opacity,
			Visible:           layer.Visible,
			ClipToBelow:       layer.ClipToBelow,
			Bounds:            layer.Bounds,
			ChannelIDs:        channelIDs,
			HasMask:           layer.HasLayerMask,
			MaskEnabled:       layer.LayerMaskEnabled,
			MaskBounds:        layer.LayerMaskBounds,
			MaskDefault:       int(layer.LayerMaskDefault),
			MaskInverted:      layer.LayerMaskInverted,
			UnsupportedBlocks: layer.UnsupportedBlocks,
		})
	}
	return views
}

// Parse parses PSD/PSB bytes and returns the record views for their layers.
func Parse(data []byte) ([]psdfixture.RecordView, error) {
	result, err := psd.Parse(data)
	if err != nil {
		return nil, err
	}
	return Views(result.Layers), nil
}
