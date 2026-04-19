package engine

import (
	"encoding/base64"
	"fmt"
	"strings"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
)

func SavePSD(doc *Document) ([]byte, error) {
	return savePSDDocument(doc, false)
}

func SavePSB(doc *Document) ([]byte, error) {
	return savePSDDocument(doc, true)
}

func savePSDDocument(doc *Document, forcePSB bool) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	if doc.Width <= 0 || doc.Height <= 0 {
		return nil, fmt.Errorf("document must have positive dimensions")
	}
	if doc.BitDepth != 0 && doc.BitDepth != 8 {
		return nil, fmt.Errorf("unsupported bit depth %d", doc.BitDepth)
	}
	if doc.ColorMode != "" && doc.ColorMode != "rgb" && doc.ColorMode != "gray" {
		return nil, fmt.Errorf("unsupported color mode %q", doc.ColorMode)
	}

	writer := &psdWriter{
		doc: doc,
		psb: forcePSB || psdRequiresPSB(doc),
	}
	return writer.write()
}

func psdRequiresPSB(doc *Document) bool {
	if doc == nil {
		return false
	}
	return psdio.RequiresPSB(doc.Width, doc.Height)
}

type psdWriter struct {
	doc *Document
	psb bool
}

type psdExportLayerRecord struct {
	name        string
	bounds      LayerBounds
	opacity     uint8
	visible     bool
	clipToBelow bool
	blendKey    string
	sectionType uint32
	mask        *LayerMask
	channels    []psdExportChannel
	extraBlocks []psdExportTaggedBlock
}

type psdExportChannel struct {
	id      int16
	length  uint64
	payload []byte
}

type psdExportTaggedBlock struct {
	signature string
	key       string
	payload   []byte
}

func exportPSDLayerRecords(records []psdExportLayerRecord) []psdio.ExportLayerRecord {
	exported := make([]psdio.ExportLayerRecord, 0, len(records))
	for _, record := range records {
		channels := make([]psdio.ExportChannel, 0, len(record.channels))
		for _, channel := range record.channels {
			channels = append(channels, psdio.ExportChannel{
				ID:      channel.id,
				Length:  channel.length,
				Payload: channel.payload,
			})
		}
		extraBlocks := make([]psdio.ExportTaggedBlock, 0, len(record.extraBlocks))
		for _, block := range record.extraBlocks {
			extraBlocks = append(extraBlocks, psdio.ExportTaggedBlock{
				Signature: block.signature,
				Key:       block.key,
				Payload:   block.payload,
			})
		}
		exported = append(exported, psdio.ExportLayerRecord{
			Name:        record.name,
			Bounds:      record.bounds,
			Opacity:     record.opacity,
			Visible:     record.visible,
			ClipToBelow: record.clipToBelow,
			BlendKey:    record.blendKey,
			SectionType: record.sectionType,
			Mask:        record.mask,
			Channels:    channels,
			ExtraBlocks: extraBlocks,
		})
	}
	return exported
}

func (w *psdWriter) write() ([]byte, error) {
	channelCount := 4
	colorMode := psdio.ColorModeRGB
	if strings.EqualFold(w.doc.ColorMode, "gray") {
		channelCount = 2
		colorMode = psdio.ColorModeGrayscale
	}
	imageResources, err := w.buildImageResourceBlocks()
	if err != nil {
		return nil, err
	}
	layerRecords, err := w.buildLayerRecords()
	if err != nil {
		return nil, err
	}
	composite, err := w.buildCompositeImageData()
	if err != nil {
		return nil, err
	}
	return psdio.Write(psdio.WriteParams{
		PSB:            w.psb,
		Width:          w.doc.Width,
		Height:         w.doc.Height,
		ChannelCount:   channelCount,
		ColorMode:      colorMode,
		Depth:          8,
		ImageResources: imageResources,
		Layers:         exportPSDLayerRecords(layerRecords),
		CompositeData:  composite,
	})
}

func (w *psdWriter) buildImageResourceBlocks() ([]psdio.ImageResourceBlock, error) {
	blocks := []psdio.ImageResourceBlock{{
		ID:      psdio.ImageResourceDPI,
		Name:    "",
		Payload: psdio.BuildResolutionInfo(w.doc.Resolution),
	}}
	projectArchive, err := SaveProject(w.doc, nil)
	if err != nil {
		return nil, fmt.Errorf("build embedded project archive: %w", err)
	}
	blocks = append(blocks, psdio.ImageResourceBlock{
		ID:      psdio.ImageResourceAgogoProject,
		Name:    "Agogo",
		Payload: projectArchive,
	})
	return blocks, nil
}

func (w *psdWriter) buildLayerRecords() ([]psdExportLayerRecord, error) {
	root := w.doc.ensureLayerRoot()
	records := make([]psdExportLayerRecord, 0, len(root.Children()))
	if err := w.appendLayerRecords(&records, root.Children()); err != nil {
		return nil, err
	}
	return records, nil
}

func (w *psdWriter) appendLayerRecords(records *[]psdExportLayerRecord, layers []LayerNode) error {
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		if group, ok := layer.(*GroupLayer); ok {
			*records = append(*records, newPSDGroupRecord(group, psdio.LayerSectionOpenFolder))
			if err := w.appendLayerRecords(records, group.Children()); err != nil {
				return err
			}
			*records = append(*records, newPSDGroupEndRecord(group))
			continue
		}
		record, err := w.newPSDRasterRecord(layer)
		if err != nil {
			return err
		}
		*records = append(*records, record)
	}
	return nil
}

func newPSDGroupRecord(group *GroupLayer, sectionType uint32) psdExportLayerRecord {
	return psdExportLayerRecord{
		name:        group.Name(),
		opacity:     psdio.UnitOpacity(group.Opacity()),
		visible:     group.Visible(),
		clipToBelow: group.ClipToBelow(),
		blendKey:    psdio.BlendKey(group.BlendMode()),
		sectionType: sectionType,
		mask:        cloneLayerMask(group.Mask()),
		extraBlocks: buildPSDLayerExtraBlocks(group),
	}
}

func newPSDGroupEndRecord(group *GroupLayer) psdExportLayerRecord {
	record := newPSDGroupRecord(group, psdio.LayerSectionCloseFolder)
	record.name = group.Name() + " End"
	record.mask = nil
	return record
}

func (w *psdWriter) newPSDRasterRecord(layer LayerNode) (psdExportLayerRecord, error) {
	bounds, pixels, err := w.exportLayerRaster(layer)
	if err != nil {
		return psdExportLayerRecord{}, fmt.Errorf("export layer %q: %w", layer.Name(), err)
	}
	channels, err := w.encodeLayerChannels(bounds, pixels, layer.Mask())
	if err != nil {
		return psdExportLayerRecord{}, fmt.Errorf("encode layer %q: %w", layer.Name(), err)
	}
	return psdExportLayerRecord{
		name:        layer.Name(),
		bounds:      bounds,
		opacity:     psdio.UnitOpacity(layer.Opacity()),
		visible:     layer.Visible(),
		clipToBelow: layer.ClipToBelow(),
		blendKey:    psdio.BlendKey(layer.BlendMode()),
		mask:        cloneLayerMask(layer.Mask()),
		channels:    channels,
		extraBlocks: buildPSDLayerExtraBlocks(layer),
	}, nil
}

func (w *psdWriter) exportLayerRaster(layer LayerNode) (LayerBounds, []byte, error) {
	switch typed := layer.(type) {
	case *PixelLayer:
		if canUseNativeLayerRaster(typed.Bounds, typed.Pixels, typed.StyleStack(), typed.Mask(), typed.ClipToBelow(), typed.BlendIf()) {
			return typed.Bounds, append([]byte(nil), typed.Pixels...), nil
		}
	case *TextLayer:
		if canUseNativeLayerRaster(typed.Bounds, typed.CachedRaster, typed.StyleStack(), typed.Mask(), typed.ClipToBelow(), typed.BlendIf()) {
			return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
		}
	case *VectorLayer:
		if canUseNativeLayerRaster(typed.Bounds, typed.CachedRaster, typed.StyleStack(), typed.Mask(), typed.ClipToBelow(), typed.BlendIf()) {
			return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
		}
	case *AdjustmentLayer:
		return LayerBounds{}, nil, nil
	}

	surface, err := w.doc.renderLayerToSurface(layer)
	if err == nil {
		if bounds, cropped := cropPSDDocumentSurface(surface, w.doc.Width, w.doc.Height); bounds.W > 0 && bounds.H > 0 {
			return bounds, cropped, nil
		}
	}

	switch typed := layer.(type) {
	case *PixelLayer:
		return typed.Bounds, append([]byte(nil), typed.Pixels...), nil
	case *TextLayer:
		return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
	case *VectorLayer:
		return typed.Bounds, append([]byte(nil), typed.CachedRaster...), nil
	default:
		return LayerBounds{}, nil, nil
	}
}

func canUseNativeLayerRaster(bounds LayerBounds, raster []byte, styles []LayerStyle, mask *LayerMask, clipToBelow bool, blendIf *BlendIfConfig) bool {
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

func cropPSDDocumentSurface(surface []byte, width, height int) (LayerBounds, []byte) {
	if width <= 0 || height <= 0 || len(surface) != width*height*4 {
		return LayerBounds{}, nil
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
		return LayerBounds{}, nil
	}
	bounds := LayerBounds{
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

func (w *psdWriter) encodeLayerChannels(bounds LayerBounds, rgba []byte, mask *LayerMask) ([]psdExportChannel, error) {
	channels := make([]psdExportChannel, 0, 5)
	if bounds.W > 0 && bounds.H > 0 && len(rgba) == bounds.W*bounds.H*4 {
		planes := psdio.RGBAToPlanes(strings.EqualFold(w.doc.ColorMode, "gray"), rgba)
		if strings.EqualFold(w.doc.ColorMode, "gray") {
			grayPayload, err := psdio.EncodeChannelData(planes[0], bounds.W, bounds.H, w.psb)
			if err != nil {
				return nil, err
			}
			alphaPayload, err := psdio.EncodeChannelData(planes[1], bounds.W, bounds.H, w.psb)
			if err != nil {
				return nil, err
			}
			channels = append(channels,
				psdExportChannel{id: 0, length: uint64(len(grayPayload)), payload: grayPayload},
				psdExportChannel{id: -1, length: uint64(len(alphaPayload)), payload: alphaPayload},
			)
		} else {
			for index, channelID := range []int16{0, 1, 2, -1} {
				payload, err := psdio.EncodeChannelData(planes[index], bounds.W, bounds.H, w.psb)
				if err != nil {
					return nil, err
				}
				channels = append(channels, psdExportChannel{
					id:      channelID,
					length:  uint64(len(payload)),
					payload: payload,
				})
			}
		}
	}
	if mask != nil && mask.Width > 0 && mask.Height > 0 && len(mask.Data) == mask.Width*mask.Height {
		payload, err := psdio.EncodeChannelData(mask.Data, mask.Width, mask.Height, w.psb)
		if err != nil {
			return nil, err
		}
		channels = append(channels, psdExportChannel{
			id:      -2,
			length:  uint64(len(payload)),
			payload: payload,
		})
	}
	return channels, nil
}

func (w *psdWriter) buildCompositeImageData() ([]byte, error) {
	surface := w.doc.renderCompositeSurface()
	if surface == nil {
		surface = make([]byte, w.doc.Width*w.doc.Height*4)
	}
	planes := psdio.RGBAToPlanes(strings.EqualFold(w.doc.ColorMode, "gray"), surface)
	return psdio.EncodeCompositeImageData(planes, w.doc.Width, w.doc.Height, w.psb)
}

func buildPSDLayerExtraBlocks(layer LayerNode) []psdExportTaggedBlock {
	if layer == nil {
		return nil
	}
	blocks := make([]psdExportTaggedBlock, 0, 4)
	if payload := psdio.BuildLayerEffectsPayload(layer.StyleStack()); len(payload) > 0 {
		blocks = append(blocks, psdExportTaggedBlock{
			signature: "8BIM",
			key:       "lfx2",
			payload:   payload,
		})
	}
	switch typed := layer.(type) {
	case *TextLayer:
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
			blocks = append(blocks, psdExportTaggedBlock{
				signature: "8BIM",
				key:       "TySh",
				payload:   payload,
			})
		}
	case *AdjustmentLayer:
		if payload := psdio.BuildAdjustmentLayerPayload(psdio.AdjustmentLayerPayload{
			Kind:   typed.AdjustmentKind,
			Params: typed.Params,
		}); len(payload) > 0 {
			blocks = append(blocks, psdExportTaggedBlock{
				signature: "8BIM",
				key:       "AgAJ",
				payload:   payload,
			})
		}
	}
	return blocks
}

func exportDocumentPayload(doc *Document, format string) (string, error) {
	var data []byte
	var err error

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "archive", "agp":
		data, err = SaveProjectZip(doc, nil)
	case "psd":
		data, err = SavePSD(doc)
	case "psb":
		data, err = SavePSB(doc)
	default:
		return "", fmt.Errorf("unsupported export format %q", format)
	}
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
