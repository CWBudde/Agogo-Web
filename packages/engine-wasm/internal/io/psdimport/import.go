package psdimport

import (
	"fmt"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func BuildLayerNodes(header psdio.Header, layers []psdio.LayerRecord) ([]model.LayerNode, []string, error) {
	if len(layers) == 0 {
		return nil, nil, nil
	}
	var warnings []string
	stacks := [][]model.LayerNode{make([]model.LayerNode, 0, len(layers))}

	resolveName := func(record psdio.LayerRecord, index int) string {
		if record.Name != "" {
			return record.Name
		}
		return fmt.Sprintf("Layer %d", index+1)
	}

	pushStack := func() {
		stacks = append(stacks, make([]model.LayerNode, 0))
	}

	popStack := func(record psdio.LayerRecord, name string) (*model.GroupLayer, error) {
		if len(stacks) <= 1 {
			return nil, fmt.Errorf("folder record has no bounding divider")
		}
		children := stacks[len(stacks)-1]
		stacks = stacks[:len(stacks)-1]
		group := model.NewGroupLayer(name)
		group.SetVisible(record.Visible)
		group.SetOpacity(record.Opacity)
		group.SetFillOpacity(record.FillOpacity)
		group.SetBlendMode(record.BlendMode)
		group.SetClipToBelow(record.ClipToBelow)
		group.Isolated = !record.PassThrough
		// lsct 1 (open folder) vs lsct 2 (closed folder). popStack is only
		// reached for those two section types, so anything that is not an open
		// folder is a closed one.
		group.Expanded = record.SectionType == psdio.LayerSectionOpenFolder
		group.SetMask(buildLayerMask(header, record))
		group.SetChildren(children)
		top := len(stacks) - 1
		stacks[top] = append(stacks[top], group)
		return group, nil
	}

	addToCurrent := func(node model.LayerNode) {
		top := len(stacks) - 1
		stacks[top] = append(stacks[top], node)
	}

	for index, record := range layers {
		name := resolveName(record, index)
		if record.SectionType == psdio.LayerSectionBoundingDivider {
			pushStack()
			continue
		}
		if record.SectionType == psdio.LayerSectionOpenFolder || record.SectionType == psdio.LayerSectionClosedFolder {
			warnings = append(warnings, record.MetadataWarnings...)
			// A group's unreconstructed blocks used to be dropped here while
			// the same blocks on a raster layer were reported. Nothing about a
			// group makes its blocks less lost.
			warnings = append(warnings, unreconstructedBlockWarnings(record, name, outcomeGroup)...)
			if _, err := popStack(record, name); err != nil {
				warnings = append(warnings, fmt.Sprintf("group %q: %v", name, err))
				continue
			}
			continue
		}

		// An adjustment layer is reconstructed before the raster branch is
		// tried at all. Photoshop writes one with an empty rect and no colour
		// channels, and flattenLayerPixels returns (nil, nil) for a zero-area
		// rect rather than an error — so before this the layer arrived as an
		// EMPTY PIXEL LAYER carrying the right name, the right blend mode and
		// nothing else, with no warning anywhere. A layer that looks imported
		// and is not is the worst of the three outcomes below, which is why it
		// is worth naming: the loss was invisible from both sides.
		if record.Adjustment != nil {
			layer := model.NewAdjustmentLayer(name, record.Adjustment.Kind, record.Adjustment.Params)
			applyRecordFlags(layer, record)
			// An adjustment layer can carry lrFX/lfx2 like any other. Those are
			// already parsed into representable style kinds, so not copying
			// them here would discard work the reader had already done.
			if len(record.Effects.StyleStack()) > 0 {
				layer.SetStyleStack(record.Effects.StyleStack())
			}
			layer.SetMask(buildLayerMask(header, record))
			for _, lost := range record.Adjustment.Lost {
				warnings = append(warnings, fmt.Sprintf(
					"layer %q: %s has no engine equivalent and was not imported", name, lost,
				))
			}
			warnings = append(warnings, unreconstructedBlockWarnings(record, name, outcomeAdjustment)...)
			warnings = append(warnings, record.MetadataWarnings...)
			addToCurrent(layer)
			continue
		}

		rgba, err := flattenLayerPixels(header, record)
		if err != nil {
			// The layer is gone. Say so, and say what was in it, because a
			// dropped layer with no account of its contents is the silent loss
			// this contract exists to prevent.
			warnings = append(warnings, fmt.Sprintf("layer %q skipped: %v", name, err))
			warnings = append(warnings, unreconstructedBlockWarnings(record, name, outcomeSkipped)...)
			warnings = append(warnings, record.MetadataWarnings...)
			continue
		}
		layer := model.NewPixelLayer(name, record.Bounds, rgba)
		applyRecordFlags(layer, record)
		if len(record.Effects.StyleStack()) > 0 {
			layer.SetStyleStack(record.Effects.StyleStack())
		}
		warnings = append(warnings, unreconstructedBlockWarnings(record, name, outcomeFlattened)...)
		warnings = append(warnings, record.MetadataWarnings...)
		layer.SetMask(buildLayerMask(header, record))
		addToCurrent(layer)
	}
	for len(stacks) > 1 {
		children := stacks[len(stacks)-1]
		stacks = stacks[:len(stacks)-1]
		top := len(stacks) - 1
		stacks[top] = append(stacks[top], children...)
		warnings = append(warnings, "unbalanced group bounding divider; imported its contents without a group")
	}
	return stacks[0], warnings, nil
}

// The fallback contract.
//
// When a tagged block cannot be reconstructed, the rule is that the import says
// so, names the block, and says accurately what became of the layer it was on.
// Before this the messages claimed every such layer was "imported as flattened
// pixel layer", which was wrong in three ways: an adjustment layer has no pixels
// to flatten and in fact arrived empty; a layer whose channels were missing was
// dropped outright and the message still said "imported"; and a block the parser
// DID recognise said nothing at all — levl, curv and hue2 never reached the
// unsupported-block list, so their loss was completely silent.
const (
	// outcomeFlattened: the layer arrived as pixels; the block did not.
	outcomeFlattened = "imported as a flattened pixel layer"
	// outcomeAdjustment: the layer arrived as a live adjustment; this extra
	// block on it did not.
	outcomeAdjustment = "not imported; the layer was reconstructed from another block"
	// outcomeGroup: groups carry blocks too, and have no raster to fall back to.
	outcomeGroup = "not imported; the group itself was preserved"
	// outcomeSkipped: no pixels and no reconstruction — the layer is gone.
	outcomeSkipped = "not imported; the layer carried no pixels and was dropped"
)

// unreconstructedBlockWarnings reports every block on the record that the
// reader recognised but could not turn into engine state, phrased for what
// actually happened to the layer.
func unreconstructedBlockWarnings(record psdio.LayerRecord, name, outcome string) []string {
	var out []string
	for _, key := range unreconstructedBlockKeys(record) {
		out = append(out, fmt.Sprintf("layer %q: metadata block %s %s", name, key, outcome))
	}
	if record.SmartObject != nil {
		out = append(out, fmt.Sprintf(
			"layer %q: metadata block %s is a smart object; its flattened composite was imported "+
				"but the placed source document, its transform and its smart filters were not",
			name, record.SmartObject.Key,
		))
	}
	return out
}

// unreconstructedBlockKeys lists the blocks whose content did not survive, in
// file order: the ones the parser could not interpret at all, then the ones it
// understood but has nowhere to put.
func unreconstructedBlockKeys(record psdio.LayerRecord) []string {
	keys := append([]string(nil), record.UnsupportedBlocks...)
	if record.Text != nil {
		// A TySh block whose text was parsed is still lost: nothing builds a
		// model.TextLayer from it yet.
		keys = append(keys, record.Text.Key)
	}
	// Smart objects are deliberately out of scope, but the warning has to be
	// accurate about what that costs. It is reported separately because the
	// generic wording understates it: the layer's own composite does arrive,
	// so "not imported" would be wrong, while the placed source document, its
	// transform and its filters are gone and "imported" would be wrong too.
	for _, meta := range record.Adjustments {
		// An adjustment block that produced no reconstruction. AgAJ is Agogo's
		// own private block and is reported like any other: it is not a PSD
		// feature, and a file that carries it is relying on something no other
		// application can read.
		if record.Adjustment == nil || !adjustmentKeyWasUsed(record, meta.Key) {
			keys = append(keys, meta.Key)
		}
	}
	return keys
}

// adjustmentKeyWasUsed reports whether the reconstructed adjustment came from
// this block key.
//
// The record keeps one reconstruction but may carry several adjustment blocks —
// Photoshop writes brit beside CgEd, and the pair is the case that matters. The
// legacy block is genuinely superseded rather than lost, so it is not reported.
func adjustmentKeyWasUsed(record psdio.LayerRecord, key string) bool {
	if record.Adjustment == nil {
		return false
	}
	if key == "brit" || key == "CgEd" {
		return record.Adjustment.Kind == "brightness-contrast"
	}
	return adjustmentKindForKey(key) == record.Adjustment.Kind
}

// adjustmentKindForKey maps a tagged-block key onto the engine adjustment kind
// the reader builds from it. Keys with no mapping return "", which never
// matches a reconstruction and so are always reported.
func adjustmentKindForKey(key string) string {
	switch key {
	case "levl":
		return "levels"
	case "curv":
		return "curves"
	case "hue2":
		return "huesat"
	case "blnc":
		return "color-balance"
	case "mixr":
		return "channel-mixer"
	case "selc":
		return "selective-color"
	case "thrs":
		return "threshold"
	case "post":
		return "posterize"
	case "nvrt":
		return "invert"
	case "phfl":
		return "photo-filter"
	case "blwh":
		return "black-white"
	default:
		return ""
	}
}

// applyRecordFlags copies the fields every layer record carries, whatever the
// layer turns out to be. An adjustment layer is a layer record like any other:
// it has an opacity, a blend mode, a clipping flag and a mask, and an importer
// that special-cases it must not quietly drop them.
func applyRecordFlags(layer model.LayerNode, record psdio.LayerRecord) {
	layer.SetOpacity(record.Opacity)
	layer.SetFillOpacity(record.FillOpacity)
	layer.SetVisible(record.Visible)
	layer.SetBlendMode(record.BlendMode)
	layer.SetClipToBelow(record.ClipToBelow)
}

// buildLayerMask converts PSD's independently positioned mask rectangle into
// the engine's document-sized mask representation. Pixels outside the stored
// rectangle use the PSD default color.
func buildLayerMask(header psdio.Header, record psdio.LayerRecord) *model.LayerMask {
	if !record.HasLayerMask || header.Width <= 0 || header.Height <= 0 {
		return nil
	}
	data := make([]byte, header.Width*header.Height)
	defaultColor := record.LayerMaskDefault
	if record.LayerMaskInverted {
		defaultColor = 255 - defaultColor
	}
	for index := range data {
		data[index] = defaultColor
	}
	maskPixels := record.ChannelPixels[-2]
	bounds := record.LayerMaskBounds
	if bounds.W > 0 && bounds.H > 0 && len(maskPixels) == bounds.W*bounds.H {
		for maskY := 0; maskY < bounds.H; maskY++ {
			docY := bounds.Y + maskY
			if docY < 0 || docY >= header.Height {
				continue
			}
			for maskX := 0; maskX < bounds.W; maskX++ {
				docX := bounds.X + maskX
				if docX < 0 || docX >= header.Width {
					continue
				}
				value := maskPixels[maskY*bounds.W+maskX]
				if record.LayerMaskInverted {
					value = 255 - value
				}
				data[docY*header.Width+docX] = value
			}
		}
	}
	return &model.LayerMask{
		Enabled: record.LayerMaskEnabled,
		Width:   header.Width,
		Height:  header.Height,
		Data:    data,
	}
}

func flattenLayerPixels(header psdio.Header, layer psdio.LayerRecord) ([]byte, error) {
	if layer.Bounds.W <= 0 || layer.Bounds.H <= 0 {
		return nil, nil
	}
	size := layer.Bounds.W * layer.Bounds.H
	rgba := make([]byte, size*4)
	switch header.ColorMode {
	case psdio.ColorModeRGB:
		red := layer.ChannelPixels[0]
		green := layer.ChannelPixels[1]
		blue := layer.ChannelPixels[2]
		alpha := layer.ChannelPixels[-1]
		if len(red) == 0 || len(green) == 0 || len(blue) == 0 {
			return nil, fmt.Errorf("missing RGB channels")
		}
		for i := 0; i < size; i++ {
			rgba[i*4] = red[i]
			rgba[i*4+1] = green[i]
			rgba[i*4+2] = blue[i]
			rgba[i*4+3] = 255
			if len(alpha) == size {
				rgba[i*4+3] = alpha[i]
			}
		}
	case psdio.ColorModeGrayscale:
		gray := layer.ChannelPixels[0]
		alpha := layer.ChannelPixels[-1]
		if len(gray) == 0 {
			return nil, fmt.Errorf("missing grayscale channel")
		}
		for i := 0; i < size; i++ {
			rgba[i*4] = gray[i]
			rgba[i*4+1] = gray[i]
			rgba[i*4+2] = gray[i]
			rgba[i*4+3] = 255
			if len(alpha) == size {
				rgba[i*4+3] = alpha[i]
			}
		}
	default:
		return nil, fmt.Errorf("unsupported color mode %d", header.ColorMode)
	}
	return rgba, nil
}
