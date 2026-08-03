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
		group.SetBlendMode(record.BlendMode)
		group.SetClipToBelow(record.ClipToBelow)
		group.Isolated = !record.PassThrough
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
			if _, err := popStack(record, name); err != nil {
				warnings = append(warnings, fmt.Sprintf("group %q: %v", name, err))
				continue
			}
			continue
		}

		rgba, err := flattenLayerPixels(header, record)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("layer %q skipped: %v", name, err))
			continue
		}
		layer := model.NewPixelLayer(name, record.Bounds, rgba)
		layer.SetOpacity(record.Opacity)
		layer.SetVisible(record.Visible)
		layer.SetBlendMode(record.BlendMode)
		layer.SetClipToBelow(record.ClipToBelow)
		if len(record.Effects.StyleStack()) > 0 {
			layer.SetStyleStack(record.Effects.StyleStack())
		}
		if record.Text != nil {
			warnings = append(warnings, fmt.Sprintf("layer %q: unsupported metadata block TySh imported as flattened pixel layer", name))
		}
		for _, key := range record.UnsupportedBlocks {
			warnings = append(warnings, fmt.Sprintf("layer %q: unsupported metadata block %s imported as flattened pixel layer", name, key))
		}
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
