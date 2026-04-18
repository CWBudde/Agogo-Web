package engine

import (
	"fmt"
	"sync/atomic"
	"time"
)

func newImportedPSDDocument(header psdHeader, resources psdImageResources) *Document {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	resolution := resources.Resolution
	if resolution <= 0 {
		resolution = defaultResolutionDPI
	}
	return &Document{
		Width:      header.Width,
		Height:     header.Height,
		Resolution: resolution,
		ColorMode:  psdDocumentColorMode(header.ColorMode),
		BitDepth:   header.Depth,
		Background: parseBackground("transparent"),
		ID:         fmt.Sprintf("doc-%04d", atomic.AddInt64(&nextDocID, 1)),
		Name:       "Imported PSD",
		CreatedAt:  timestamp,
		CreatedBy:  "agogo-web",
		ModifiedAt: timestamp,
		LayerRoot:  NewGroupLayer("Root"),
	}
}

func buildPSDLayerNodes(header psdHeader, layers []psdLayerRecord) ([]LayerNode, []string, error) {
	if len(layers) == 0 {
		return nil, nil, nil
	}
	var warnings []string
	nodes := make([]LayerNode, 0, len(layers))
	groups := make([]*GroupLayer, 0)
	stacks := [][]LayerNode{nodes}

	resolveName := func(record psdLayerRecord, index int) string {
		if record.Name != "" {
			return record.Name
		}
		return fmt.Sprintf("Layer %d", index+1)
	}

	pushStack := func() {
		stacks = append(stacks, make([]LayerNode, 0))
	}

	popStack := func() (*GroupLayer, error) {
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

	addToCurrent := func(node LayerNode) {
		top := len(stacks) - 1
		stacks[top] = append(stacks[top], node)
	}

	beginGroup := func(record psdLayerRecord, name string) {
		group := NewGroupLayer(name)
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
		if record.SectionType == psdLayerSectionOpenFolder || record.SectionType == psdLayerSectionNested {
			beginGroup(record, name)
			continue
		}
		if record.SectionType == psdLayerSectionCloseFolder {
			if _, err := popStack(); err != nil {
				warnings = append(warnings, "unbalanced group end marker")
				continue
			}
			continue
		}

		rgba, err := flattenPSDLayerPixels(header, record)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("layer %q skipped: %v", name, err))
			continue
		}
		layer := NewPixelLayer(name, record.Bounds, rgba)
		layer.SetOpacity(record.Opacity)
		layer.SetVisible(record.Visible)
		layer.SetBlendMode(record.BlendMode)
		layer.SetClipToBelow(record.ClipToBelow)
		if len(record.Effects.GetStyleStack()) > 0 {
			layer.SetStyleStack(record.Effects.GetStyleStack())
		}
		for _, key := range record.UnsupportedBlocks {
			warnings = append(warnings, fmt.Sprintf("layer %q: unsupported metadata block %s imported as flattened pixel layer", name, key))
		}
		warnings = append(warnings, record.MetadataWarnings...)
		if record.HasLayerMask && record.LayerMaskBounds.W > 0 && record.LayerMaskBounds.H > 0 {
			layer.SetMask(&LayerMask{
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

func flattenPSDLayerPixels(header psdHeader, layer psdLayerRecord) ([]byte, error) {
	if layer.Bounds.W <= 0 || layer.Bounds.H <= 0 {
		return nil, nil
	}
	size := layer.Bounds.W * layer.Bounds.H
	rgba := make([]byte, size*4)
	switch header.ColorMode {
	case psdColorModeRGB:
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
	case psdColorModeGrayscale:
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
