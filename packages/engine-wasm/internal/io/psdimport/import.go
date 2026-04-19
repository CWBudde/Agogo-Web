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
	nodes := make([]model.LayerNode, 0, len(layers))
	groups := make([]*model.GroupLayer, 0)
	stacks := [][]model.LayerNode{nodes}

	resolveName := func(record psdio.LayerRecord, index int) string {
		if record.Name != "" {
			return record.Name
		}
		return fmt.Sprintf("Layer %d", index+1)
	}

	pushStack := func() {
		stacks = append(stacks, make([]model.LayerNode, 0))
	}

	popStack := func() (*model.GroupLayer, error) {
		if len(stacks) <= 1 || len(groups) == 0 {
			return nil, fmt.Errorf("unbalanced group close marker")
		}
		lastGroupIdx := len(groups) - 1
		group := groups[lastGroupIdx]
		children := stacks[len(stacks)-1]
		stacks = stacks[:len(stacks)-1]
		groups = groups[:lastGroupIdx]
		group.SetChildren(children)
		return group, nil
	}

	addToCurrent := func(node model.LayerNode) {
		top := len(stacks) - 1
		stacks[top] = append(stacks[top], node)
	}

	beginGroup := func(record psdio.LayerRecord, name string) {
		group := model.NewGroupLayer(name)
		group.SetVisible(record.Visible)
		group.SetOpacity(record.Opacity)
		group.SetBlendMode(record.BlendMode)
		group.SetClipToBelow(record.ClipToBelow)
		addToCurrent(group)
		groups = append(groups, group)
		pushStack()
	}

	for index, record := range layers {
		name := resolveName(record, index)
		if record.SectionType == psdio.LayerSectionOpenFolder || record.SectionType == psdio.LayerSectionNested {
			beginGroup(record, name)
			continue
		}
		if record.SectionType == psdio.LayerSectionCloseFolder {
			if _, err := popStack(); err != nil {
				warnings = append(warnings, "unbalanced group end marker")
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
		if record.HasLayerMask && record.LayerMaskBounds.W > 0 && record.LayerMaskBounds.H > 0 {
			layer.SetMask(&model.LayerMask{
				Enabled: record.LayerMaskEnabled,
				Width:   record.LayerMaskBounds.W,
				Height:  record.LayerMaskBounds.H,
			})
		}
		addToCurrent(layer)
	}
	for len(stacks) > 1 {
		group, err := popStack()
		if err != nil {
			break
		}
		if group != nil {
			warnings = append(warnings, fmt.Sprintf("group %q was not explicitly closed", group.Name()))
		}
	}
	nodes = stacks[0]
	return nodes, warnings, nil
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
