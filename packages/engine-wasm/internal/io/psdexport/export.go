package psdexport

import (
	"fmt"
	"strings"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

type Params struct {
	Width           int
	Height          int
	Resolution      float64
	ColorMode       string
	BitDepth        int
	ForcePSB        bool
	ProjectArchive  []byte
	Layers          []model.LayerNode
	RenderLayer     func(model.LayerNode) ([]byte, error)
	RenderComposite func() []byte
}

func Export(params Params) ([]byte, error) {
	if params.Width <= 0 || params.Height <= 0 {
		return nil, fmt.Errorf("document must have positive dimensions")
	}
	if params.BitDepth != 0 && params.BitDepth != 8 {
		return nil, fmt.Errorf("unsupported bit depth %d", params.BitDepth)
	}
	if params.ColorMode != "" && params.ColorMode != "rgb" && params.ColorMode != "gray" {
		return nil, fmt.Errorf("unsupported color mode %q", params.ColorMode)
	}
	if params.RenderLayer == nil {
		return nil, fmt.Errorf("render layer callback is required")
	}
	if params.RenderComposite == nil {
		return nil, fmt.Errorf("render composite callback is required")
	}

	psb := params.ForcePSB || psdio.RequiresPSB(params.Width, params.Height)
	channelCount := 4
	colorMode := psdio.ColorModeRGB
	if strings.EqualFold(params.ColorMode, "gray") {
		channelCount = 2
		colorMode = psdio.ColorModeGrayscale
	}

	imageResources := []psdio.ImageResourceBlock{{
		ID:      psdio.ImageResourceDPI,
		Name:    "",
		Payload: psdio.BuildResolutionInfo(params.Resolution),
	}}
	if len(params.ProjectArchive) > 0 {
		imageResources = append(imageResources, psdio.ImageResourceBlock{
			ID:      psdio.ImageResourceAgogoProject,
			Name:    "Agogo",
			Payload: append([]byte(nil), params.ProjectArchive...),
		})
	}

	records, err := buildLayerRecords(params, psb)
	if err != nil {
		return nil, err
	}
	composite, err := buildCompositeImageData(params, psb)
	if err != nil {
		return nil, err
	}

	return psdio.Write(psdio.WriteParams{
		PSB:            psb,
		Width:          params.Width,
		Height:         params.Height,
		ChannelCount:   channelCount,
		ColorMode:      colorMode,
		Depth:          8,
		ImageResources: imageResources,
		Layers:         records,
		CompositeData:  composite,
	})
}

func buildLayerRecords(params Params, psb bool) ([]psdio.ExportLayerRecord, error) {
	records := make([]psdio.ExportLayerRecord, 0, len(params.Layers))
	if err := appendLayerRecords(&records, params, psb, params.Layers); err != nil {
		return nil, err
	}
	return records, nil
}

func appendLayerRecords(records *[]psdio.ExportLayerRecord, params Params, psb bool, layers []model.LayerNode) error {
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		if group, ok := layer.(*model.GroupLayer); ok {
			*records = append(*records, newGroupRecord(group, psdio.LayerSectionOpenFolder))
			if err := appendLayerRecords(records, params, psb, group.Children()); err != nil {
				return err
			}
			*records = append(*records, newGroupEndRecord(group))
			continue
		}
		record, err := newRasterRecord(params, psb, layer)
		if err != nil {
			return err
		}
		*records = append(*records, record)
	}
	return nil
}

func newGroupRecord(group *model.GroupLayer, sectionType uint32) psdio.ExportLayerRecord {
	return psdio.ExportLayerRecord{
		Name:        group.Name(),
		Opacity:     psdio.UnitOpacity(group.Opacity()),
		Visible:     group.Visible(),
		ClipToBelow: group.ClipToBelow(),
		BlendKey:    psdio.BlendKey(group.BlendMode()),
		SectionType: sectionType,
		Mask:        model.CloneLayerMask(group.Mask()),
		ExtraBlocks: buildLayerExtraBlocks(group),
	}
}

func newGroupEndRecord(group *model.GroupLayer) psdio.ExportLayerRecord {
	record := newGroupRecord(group, psdio.LayerSectionCloseFolder)
	record.Name = group.Name() + " End"
	record.Mask = nil
	return record
}

func newRasterRecord(params Params, psb bool, layer model.LayerNode) (psdio.ExportLayerRecord, error) {
	bounds, pixels, err := exportLayerRaster(params, layer)
	if err != nil {
		return psdio.ExportLayerRecord{}, fmt.Errorf("export layer %q: %w", layer.Name(), err)
	}
	channels, err := encodeLayerChannels(params.ColorMode, psb, bounds, pixels, layer.Mask())
	if err != nil {
		return psdio.ExportLayerRecord{}, fmt.Errorf("encode layer %q: %w", layer.Name(), err)
	}
	return psdio.ExportLayerRecord{
		Name:        layer.Name(),
		Bounds:      bounds,
		Opacity:     psdio.UnitOpacity(layer.Opacity()),
		Visible:     layer.Visible(),
		ClipToBelow: layer.ClipToBelow(),
		BlendKey:    psdio.BlendKey(layer.BlendMode()),
		Mask:        model.CloneLayerMask(layer.Mask()),
		Channels:    channels,
		ExtraBlocks: buildLayerExtraBlocks(layer),
	}, nil
}

func exportLayerRaster(params Params, layer model.LayerNode) (model.LayerBounds, []byte, error) {
	switch typed := layer.(type) {
	case *model.PixelLayer:
		if canUseNativeLayerRaster(typed.Bounds, typed.Pixels, typed.StyleStack(), typed.Mask(), typed.ClipToBelow(), typed.BlendIf()) {
			return typed.Bounds, append([]byte(nil), typed.Pixels...), nil
		}
	case *model.TextLayer:
		if canUseNativeLayerRaster(typed.Bounds, typed.CachedRaster, typed.StyleStack(), typed.Mask(), typed.ClipToBelow(), typed.BlendIf()) {
			return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
		}
	case *model.VectorLayer:
		if canUseNativeLayerRaster(typed.Bounds, typed.CachedRaster, typed.StyleStack(), typed.Mask(), typed.ClipToBelow(), typed.BlendIf()) {
			return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
		}
	case *model.AdjustmentLayer:
		return model.LayerBounds{}, nil, nil
	}

	surface, err := params.RenderLayer(layer)
	if err == nil {
		if bounds, cropped := cropDocumentSurface(surface, params.Width, params.Height); bounds.W > 0 && bounds.H > 0 {
			return bounds, cropped, nil
		}
	}

	switch typed := layer.(type) {
	case *model.PixelLayer:
		return typed.Bounds, append([]byte(nil), typed.Pixels...), nil
	case *model.TextLayer:
		return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
	case *model.VectorLayer:
		return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
	default:
		return model.LayerBounds{}, nil, nil
	}
}

func canUseNativeLayerRaster(bounds model.LayerBounds, raster []byte, styles []model.LayerStyle, mask *model.LayerMask, clipToBelow bool, blendIf *model.BlendIfConfig) bool {
	if bounds.W <= 0 || bounds.H <= 0 {
		return false
	}
	if len(raster) != bounds.W*bounds.H*4 {
		return false
	}
	if hasAnyEnabledLayerStyleEntry(styles) || mask != nil || clipToBelow || !blendIfIsIdentity(blendIf) {
		return false
	}
	return true
}

func cropDocumentSurface(surface []byte, width, height int) (model.LayerBounds, []byte) {
	if width <= 0 || height <= 0 || len(surface) != width*height*4 {
		return model.LayerBounds{}, nil
	}
	minX, minY := width, height
	maxX, maxY := -1, -1
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := (y*width + x) * 4
			if surface[index+3] == 0 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		return model.LayerBounds{}, nil
	}
	bounds := model.LayerBounds{
		X: minX,
		Y: minY,
		W: maxX - minX + 1,
		H: maxY - minY + 1,
	}
	cropped := make([]byte, bounds.W*bounds.H*4)
	for row := 0; row < bounds.H; row++ {
		srcStart := ((bounds.Y+row)*width + bounds.X) * 4
		srcEnd := srcStart + bounds.W*4
		dstStart := row * bounds.W * 4
		copy(cropped[dstStart:dstStart+bounds.W*4], surface[srcStart:srcEnd])
	}
	return bounds, cropped
}

func encodeLayerChannels(colorMode string, psb bool, bounds model.LayerBounds, rgba []byte, mask *model.LayerMask) ([]psdio.ExportChannel, error) {
	channels := make([]psdio.ExportChannel, 0, 5)
	if bounds.W > 0 && bounds.H > 0 && len(rgba) == bounds.W*bounds.H*4 {
		planes := psdio.RGBAToPlanes(strings.EqualFold(colorMode, "gray"), rgba)
		if strings.EqualFold(colorMode, "gray") {
			grayPayload, err := psdio.EncodeChannelData(planes[0], bounds.W, bounds.H, psb)
			if err != nil {
				return nil, err
			}
			alphaPayload, err := psdio.EncodeChannelData(planes[1], bounds.W, bounds.H, psb)
			if err != nil {
				return nil, err
			}
			channels = append(channels,
				psdio.ExportChannel{ID: 0, Length: uint64(len(grayPayload)), Payload: grayPayload},
				psdio.ExportChannel{ID: -1, Length: uint64(len(alphaPayload)), Payload: alphaPayload},
			)
		} else {
			for index, channelID := range []int16{0, 1, 2, -1} {
				payload, err := psdio.EncodeChannelData(planes[index], bounds.W, bounds.H, psb)
				if err != nil {
					return nil, err
				}
				channels = append(channels, psdio.ExportChannel{
					ID:      channelID,
					Length:  uint64(len(payload)),
					Payload: payload,
				})
			}
		}
	}
	if mask != nil && mask.Width > 0 && mask.Height > 0 && len(mask.Data) == mask.Width*mask.Height {
		payload, err := psdio.EncodeChannelData(mask.Data, mask.Width, mask.Height, psb)
		if err != nil {
			return nil, err
		}
		channels = append(channels, psdio.ExportChannel{
			ID:      -2,
			Length:  uint64(len(payload)),
			Payload: payload,
		})
	}
	return channels, nil
}

func buildCompositeImageData(params Params, psb bool) ([]byte, error) {
	surface := params.RenderComposite()
	if surface == nil {
		surface = make([]byte, params.Width*params.Height*4)
	}
	planes := psdio.RGBAToPlanes(strings.EqualFold(params.ColorMode, "gray"), surface)
	return psdio.EncodeCompositeImageData(planes, params.Width, params.Height, psb)
}

func buildLayerExtraBlocks(layer model.LayerNode) []psdio.ExportTaggedBlock {
	if layer == nil {
		return nil
	}
	blocks := make([]psdio.ExportTaggedBlock, 0, 4)
	if payload := psdio.BuildLayerEffectsPayload(layer.StyleStack()); len(payload) > 0 {
		blocks = append(blocks, psdio.ExportTaggedBlock{
			Signature: "8BIM",
			Key:       "lfx2",
			Payload:   payload,
		})
	}
	switch typed := layer.(type) {
	case *model.TextLayer:
		if payload := psdio.BuildTextLayerPayload(psdio.TextLayerPayload{
			Bounds:        typed.Bounds,
			Text:          typed.Text,
			FontFamily:    typed.FontFamily,
			FontStyle:     typed.FontStyle,
			FontSize:      typed.FontSize,
			Bold:          typed.Bold,
			Italic:        typed.Italic,
			AntiAlias:     typed.AntiAlias,
			Color:         typed.Color,
			TextType:      typed.TextType,
			Alignment:     typed.Alignment,
			BaselineShift: typed.BaselineShift,
			Leading:       typed.Leading,
			Tracking:      typed.Tracking,
			Kerning:       typed.Kerning,
			Language:      typed.Language,
			Orientation:   typed.Orientation,
			Superscript:   typed.Superscript,
			Subscript:     typed.Subscript,
			Underline:     typed.Underline,
			Strikethrough: typed.Strikethrough,
			AllCaps:       typed.AllCaps,
			SmallCaps:     typed.SmallCaps,
			IndentLeft:    typed.IndentLeft,
			IndentRight:   typed.IndentRight,
			IndentFirst:   typed.IndentFirst,
			SpaceBefore:   typed.SpaceBefore,
			SpaceAfter:    typed.SpaceAfter,
		}); len(payload) > 0 {
			blocks = append(blocks, psdio.ExportTaggedBlock{
				Signature: "8BIM",
				Key:       "TySh",
				Payload:   payload,
			})
		}
	case *model.AdjustmentLayer:
		if payload := psdio.BuildAdjustmentLayerPayload(psdio.AdjustmentLayerPayload{
			Kind:   typed.AdjustmentKind,
			Params: typed.Params,
		}); len(payload) > 0 {
			blocks = append(blocks, psdio.ExportTaggedBlock{
				Signature: "8BIM",
				Key:       "AgAJ",
				Payload:   payload,
			})
		}
	}
	return blocks
}

func hasAnyEnabledLayerStyleEntry(styles []model.LayerStyle) bool {
	for _, style := range styles {
		if style.Enabled {
			return true
		}
	}
	return false
}

func blendIfIsIdentity(cfg *model.BlendIfConfig) bool {
	if cfg == nil {
		return true
	}
	if !cfg.Channels.R || !cfg.Channels.G || !cfg.Channels.B {
		return false
	}
	return rangeIsIdentity(cfg.ThisLayer) && rangeIsIdentity(cfg.UnderlyingLayer)
}

func rangeIsIdentity(r model.BlendIfRange) bool {
	return channelIsIdentity(r.Gray) &&
		channelIsIdentity(r.Red) &&
		channelIsIdentity(r.Green) &&
		channelIsIdentity(r.Blue)
}

func channelIsIdentity(c model.BlendIfChannel) bool {
	return c[0] == 0 && c[1] == 0 && c[2] == 255 && c[3] == 255
}
