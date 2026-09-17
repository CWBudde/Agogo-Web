package psd

import (
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture"
)

// TestPSDFixtureCorpusParsesToExternallyDerivedRecords asserts the psdRecords
// scope: the facts that do not survive into the engine model. Mask rectangles,
// mask default fill, section-divider types and channel IDs are all discarded or
// transformed by psdimport, so this is the only place they can be checked
// against an externally derived expectation.
func TestPSDFixtureCorpusParsesToExternallyDerivedRecords(t *testing.T) {
	fixtures, err := psdfixture.Load()
	if err != nil {
		t.Fatalf("load fixture corpus: %v", err)
	}
	for _, fixture := range fixtures {
		if !fixture.Spec.Asserts(psdfixture.ScopePSDRecords) {
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			result, parseErr := Parse(fixture.Data)
			if parseErr != nil {
				t.Fatalf("Parse: %v", parseErr)
			}
			for _, mismatch := range psdfixture.CompareRecords(fixture.Spec, recordViews(result.Layers)) {
				t.Errorf("%s", mismatch)
			}
		})
	}
}

// recordViews converts parsed layer records into the neutral view psdfixture
// compares against. psdfixture cannot import this package: internal/io/psd's own
// tests live in package psd, so the dependency would be a cycle.
func recordViews(layers []LayerRecord) []psdfixture.RecordView {
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
