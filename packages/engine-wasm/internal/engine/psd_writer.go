package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	psdImageResourceAgogoProject = 0x0fa0
	psdPSDMaxDimension           = 30000
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
	return doc.Width > psdPSDMaxDimension || doc.Height > psdPSDMaxDimension
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

func (w *psdWriter) write() ([]byte, error) {
	var out bytes.Buffer

	writePSDString(&out, "8BPS")
	if w.psb {
		writePSDUint16(&out, 2)
	} else {
		writePSDUint16(&out, 1)
	}
	out.Write(make([]byte, 6))

	channelCount := 4
	colorMode := psdColorModeRGB
	if strings.EqualFold(w.doc.ColorMode, "gray") {
		channelCount = 2
		colorMode = psdColorModeGrayscale
	}
	writePSDUint16(&out, uint16(channelCount))
	writePSDUint32(&out, uint32(w.doc.Height))
	writePSDUint32(&out, uint32(w.doc.Width))
	writePSDUint16(&out, 8)
	writePSDUint16(&out, uint16(colorMode))

	// Color mode data section.
	writePSDUint32(&out, 0)

	imageResources, err := w.buildImageResources()
	if err != nil {
		return nil, err
	}
	writePSDUint32(&out, uint32(len(imageResources)))
	out.Write(imageResources)

	layerAndMaskInfo, err := w.buildLayerAndMaskInfo()
	if err != nil {
		return nil, err
	}
	writePSDSectionLength(&out, w.psb, uint64(len(layerAndMaskInfo)))
	out.Write(layerAndMaskInfo)

	composite, err := w.buildCompositeImageData()
	if err != nil {
		return nil, err
	}
	out.Write(composite)

	return out.Bytes(), nil
}

func (w *psdWriter) buildImageResources() ([]byte, error) {
	var out bytes.Buffer

	writePSDImageResource(&out, psdImageResourceDPI, "", buildPSDResolutionInfo(w.doc.Resolution))

	projectArchive, err := SaveProject(w.doc, nil)
	if err != nil {
		return nil, fmt.Errorf("build embedded project archive: %w", err)
	}
	writePSDImageResource(&out, psdImageResourceAgogoProject, "Agogo", projectArchive)

	return out.Bytes(), nil
}

func (w *psdWriter) buildLayerAndMaskInfo() ([]byte, error) {
	records, err := w.buildLayerRecords()
	if err != nil {
		return nil, err
	}

	var layerInfo bytes.Buffer
	writePSDInt16(&layerInfo, int16(len(records)))
	for _, record := range records {
		writePSDLayerRecord(&layerInfo, w.psb, record)
	}
	for _, record := range records {
		for _, channel := range record.channels {
			layerInfo.Write(channel.payload)
		}
	}

	var out bytes.Buffer
	writePSDSectionLength(&out, w.psb, uint64(layerInfo.Len()))
	out.Write(layerInfo.Bytes())
	return out.Bytes(), nil
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
			*records = append(*records, newPSDGroupRecord(group, psdLayerSectionOpenFolder))
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
		opacity:     unitToPSDOpacity(group.Opacity()),
		visible:     group.Visible(),
		clipToBelow: group.ClipToBelow(),
		blendKey:    psdBlendKey(group.BlendMode()),
		sectionType: sectionType,
		mask:        cloneLayerMask(group.Mask()),
		extraBlocks: buildPSDLayerExtraBlocks(group),
	}
}

func newPSDGroupEndRecord(group *GroupLayer) psdExportLayerRecord {
	record := newPSDGroupRecord(group, psdLayerSectionCloseFolder)
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
		opacity:     unitToPSDOpacity(layer.Opacity()),
		visible:     layer.Visible(),
		clipToBelow: layer.ClipToBelow(),
		blendKey:    psdBlendKey(layer.BlendMode()),
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
		planes := rgbaToPSDPlanes(strings.EqualFold(w.doc.ColorMode, "gray"), rgba)
		if strings.EqualFold(w.doc.ColorMode, "gray") {
			grayPayload, err := encodePSDChannelData(planes[0], bounds.W, bounds.H, w.psb)
			if err != nil {
				return nil, err
			}
			alphaPayload, err := encodePSDChannelData(planes[1], bounds.W, bounds.H, w.psb)
			if err != nil {
				return nil, err
			}
			channels = append(channels,
				psdExportChannel{id: 0, length: uint64(len(grayPayload)), payload: grayPayload},
				psdExportChannel{id: -1, length: uint64(len(alphaPayload)), payload: alphaPayload},
			)
		} else {
			for index, channelID := range []int16{0, 1, 2, -1} {
				payload, err := encodePSDChannelData(planes[index], bounds.W, bounds.H, w.psb)
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
		payload, err := encodePSDChannelData(mask.Data, mask.Width, mask.Height, w.psb)
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

func rgbaToPSDPlanes(grayscale bool, rgba []byte) [][]byte {
	pixelCount := len(rgba) / 4
	if grayscale {
		gray := make([]byte, pixelCount)
		alpha := make([]byte, pixelCount)
		for i := 0; i < pixelCount; i++ {
			r := float64(rgba[i*4])
			g := float64(rgba[i*4+1])
			b := float64(rgba[i*4+2])
			gray[i] = byte(math.Round((0.299 * r) + (0.587 * g) + (0.114 * b)))
			alpha[i] = rgba[i*4+3]
		}
		return [][]byte{gray, alpha}
	}
	red := make([]byte, pixelCount)
	green := make([]byte, pixelCount)
	blue := make([]byte, pixelCount)
	alpha := make([]byte, pixelCount)
	for i := 0; i < pixelCount; i++ {
		red[i] = rgba[i*4]
		green[i] = rgba[i*4+1]
		blue[i] = rgba[i*4+2]
		alpha[i] = rgba[i*4+3]
	}
	return [][]byte{red, green, blue, alpha}
}

func encodePSDChannelData(data []byte, width, height int, psb bool) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return []byte{0, psdCompressionRLE}, nil
	}
	if len(data) != width*height {
		return nil, fmt.Errorf("channel length %d does not match %dx%d", len(data), width, height)
	}

	rows := make([][]byte, 0, height)
	for row := 0; row < height; row++ {
		start := row * width
		rows = append(rows, encodePSDPackBitsRow(data[start:start+width]))
	}

	var out bytes.Buffer
	writePSDUint16(&out, psdCompressionRLE)
	for _, row := range rows {
		if psb {
			writePSDUint32(&out, uint32(len(row)))
		} else {
			if len(row) > math.MaxUint16 {
				return nil, fmt.Errorf("RLE row length %d exceeds PSD limit", len(row))
			}
			writePSDUint16(&out, uint16(len(row)))
		}
	}
	for _, row := range rows {
		out.Write(row)
	}
	return out.Bytes(), nil
}

func encodePSDPackBitsRow(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}
	out := make([]byte, 0, len(data)+(len(data)/128)+1)
	for i := 0; i < len(data); {
		runLen := 1
		for i+runLen < len(data) && runLen < 128 && data[i+runLen] == data[i] {
			runLen++
		}
		if runLen >= 3 {
			out = append(out, byte(257-runLen), data[i])
			i += runLen
			continue
		}

		literalStart := i
		i += runLen
		for i < len(data) {
			runLen = 1
			for i+runLen < len(data) && runLen < 128 && data[i+runLen] == data[i] {
				runLen++
			}
			if runLen >= 3 || i-literalStart >= 128 {
				break
			}
			i += runLen
		}
		literalLen := i - literalStart
		for literalLen > 0 {
			chunkLen := literalLen
			if chunkLen > 128 {
				chunkLen = 128
			}
			out = append(out, byte(chunkLen-1))
			out = append(out, data[literalStart:literalStart+chunkLen]...)
			literalStart += chunkLen
			literalLen -= chunkLen
		}
	}
	return out
}

func (w *psdWriter) buildCompositeImageData() ([]byte, error) {
	surface := w.doc.renderCompositeSurface()
	if surface == nil {
		surface = make([]byte, w.doc.Width*w.doc.Height*4)
	}
	planes := rgbaToPSDPlanes(strings.EqualFold(w.doc.ColorMode, "gray"), surface)

	var out bytes.Buffer
	writePSDUint16(&out, psdCompressionRLE)
	for _, plane := range planes {
		rows := make([][]byte, 0, w.doc.Height)
		for row := 0; row < w.doc.Height; row++ {
			start := row * w.doc.Width
			rows = append(rows, encodePSDPackBitsRow(plane[start:start+w.doc.Width]))
		}
		for _, row := range rows {
			if w.psb {
				writePSDUint32(&out, uint32(len(row)))
			} else {
				if len(row) > math.MaxUint16 {
					return nil, fmt.Errorf("composite RLE row length %d exceeds PSD limit", len(row))
				}
				writePSDUint16(&out, uint16(len(row)))
			}
		}
		for _, row := range rows {
			out.Write(row)
		}
	}
	return out.Bytes(), nil
}

func buildPSDResolutionInfo(resolution float64) []byte {
	dpi := resolution
	if dpi <= 0 {
		dpi = defaultResolutionDPI
	}
	fixed := uint32(math.Round(dpi * 65536.0))
	var out bytes.Buffer
	writePSDUint32(&out, fixed)
	writePSDUint16(&out, 1)
	writePSDUint16(&out, 1)
	writePSDUint32(&out, fixed)
	writePSDUint16(&out, 1)
	writePSDUint16(&out, 1)
	return out.Bytes()
}

func writePSDLayerRecord(out *bytes.Buffer, psb bool, record psdExportLayerRecord) {
	writePSDInt32(out, int32(record.bounds.Y))
	writePSDInt32(out, int32(record.bounds.X))
	writePSDInt32(out, int32(record.bounds.Y+record.bounds.H))
	writePSDInt32(out, int32(record.bounds.X+record.bounds.W))
	writePSDUint16(out, uint16(len(record.channels)))
	for _, channel := range record.channels {
		writePSDInt16(out, channel.id)
		writePSDSectionLength(out, psb, channel.length)
	}
	writePSDString(out, "8BIM")
	writePSDString(out, record.blendKey)
	out.WriteByte(record.opacity)
	if record.clipToBelow {
		out.WriteByte(1)
	} else {
		out.WriteByte(0)
	}
	flags := byte(0)
	if !record.visible {
		flags |= 0x02
	}
	out.WriteByte(flags)
	out.WriteByte(0)

	var extra bytes.Buffer
	writePSDLayerMaskData(&extra, record.mask)
	writePSDUint32(&extra, 0) // blending ranges
	writePSDPascalString4(&extra, record.name)
	writePSDUnicodeLayerNameBlock(&extra, record.name)
	if record.sectionType != 0 {
		writePSDAdditionalLayerInfoBlock(&extra, "8BIM", "lsct", buildPSDSectionDivider(record.sectionType))
	}
	for _, block := range record.extraBlocks {
		writePSDAdditionalLayerInfoBlock(&extra, block.signature, block.key, block.payload)
	}
	writePSDUint32(out, uint32(extra.Len()))
	out.Write(extra.Bytes())
}

func writePSDLayerMaskData(out *bytes.Buffer, mask *LayerMask) {
	if mask == nil || mask.Width <= 0 || mask.Height <= 0 || len(mask.Data) != mask.Width*mask.Height {
		writePSDUint32(out, 0)
		return
	}
	var payload bytes.Buffer
	writePSDInt32(&payload, 0)
	writePSDInt32(&payload, 0)
	writePSDInt32(&payload, int32(mask.Height))
	writePSDInt32(&payload, int32(mask.Width))
	writePSDUint16(&payload, 0)
	flags := uint16(0)
	if !mask.Enabled {
		flags = 1
	}
	writePSDUint16(&payload, flags)
	writePSDUint32(out, uint32(payload.Len()))
	out.Write(payload.Bytes())
}

func buildPSDSectionDivider(sectionType uint32) []byte {
	var out bytes.Buffer
	writePSDUint32(&out, sectionType)
	return out.Bytes()
}

func writePSDUnicodeLayerNameBlock(out *bytes.Buffer, name string) {
	if name == "" {
		return
	}
	encoded := encodePSDUnicodeString(name)
	writePSDAdditionalLayerInfoBlock(out, "8BIM", "luni", encoded)
}

func encodePSDUnicodeString(value string) []byte {
	encoded := utf16Encode(value)
	var out bytes.Buffer
	writePSDUint32(&out, uint32(len(encoded)))
	for _, r := range encoded {
		writePSDUint16(&out, r)
	}
	return out.Bytes()
}

func utf16Encode(value string) []uint16 {
	runes := []rune(value)
	encoded := make([]uint16, 0, len(runes))
	for _, r := range runes {
		if r <= math.MaxUint16 {
			encoded = append(encoded, uint16(r))
			continue
		}
		r -= 0x10000
		encoded = append(encoded,
			uint16(0xd800+((r>>10)&0x3ff)),
			uint16(0xdc00+(r&0x3ff)),
		)
	}
	return encoded
}

func buildPSDLayerExtraBlocks(layer LayerNode) []psdExportTaggedBlock {
	if layer == nil {
		return nil
	}
	blocks := make([]psdExportTaggedBlock, 0, 4)
	if payload := buildPSDLayerEffectsPayload(layer.StyleStack()); len(payload) > 0 {
		blocks = append(blocks, psdExportTaggedBlock{
			signature: "8BIM",
			key:       "lfx2",
			payload:   payload,
		})
	}
	switch typed := layer.(type) {
	case *TextLayer:
		if payload := buildPSDTextLayerPayload(typed); len(payload) > 0 {
			blocks = append(blocks, psdExportTaggedBlock{
				signature: "8BIM",
				key:       "TySh",
				payload:   payload,
			})
		}
	case *AdjustmentLayer:
		if payload := buildPSDAdjustmentLayerPayload(typed); len(payload) > 0 {
			blocks = append(blocks, psdExportTaggedBlock{
				signature: "8BIM",
				key:       "AgAJ",
				payload:   payload,
			})
		}
	}
	return blocks
}

func buildPSDLayerEffectsPayload(styles []LayerStyle) []byte {
	filtered := make([]map[string]any, 0, len(styles))
	for _, style := range styles {
		token := psdEffectDescriptorToken(style.Kind)
		if token == "" {
			continue
		}
		filtered = append(filtered, map[string]any{
			"token":   token,
			"kind":    style.Kind,
			"enabled": style.Enabled,
			"params":  string(style.Params),
		})
	}
	if len(filtered) == 0 {
		return nil
	}

	items := make([]psdDescriptorItem, 0, len(filtered)+1)
	items = append(items, psdDescriptorItem{Key: "masterFXSwitch", Type: "bool", Bool: true})
	for index, style := range filtered {
		items = append(items, psdDescriptorItem{
			Key:  style["token"].(string),
			Type: "TEXT",
			Text: marshalPSDJSON(style),
		})
		items = append(items, psdDescriptorItem{
			Key:  fmt.Sprintf("fx%d", index),
			Type: "TEXT",
			Text: style["kind"].(string),
		})
	}

	var out bytes.Buffer
	writePSDUint32(&out, 0)
	writePSDUint32(&out, 16)
	writePSDDescriptor(&out, "", "lfx2", items)
	return out.Bytes()
}

func psdEffectDescriptorToken(kind string) string {
	switch LayerStyleKind(kind) {
	case LayerStyleKindDropShadow:
		return "dropshadow"
	case LayerStyleKindInnerShadow:
		return "innershadow"
	case LayerStyleKindOuterGlow:
		return "outerglow"
	case LayerStyleKindInnerGlow:
		return "innerglow"
	case LayerStyleKindBevelEmboss:
		return "bevelemboss"
	case LayerStyleKindStroke:
		return "strokestyle"
	case LayerStyleKindColorOverlay:
		return "coloroverlay"
	case LayerStyleKindGradientOverlay:
		return "gradientoverlay"
	case LayerStyleKindPatternOverlay:
		return "patternoverlay"
	case LayerStyleKindSatin:
		return "satin"
	default:
		return ""
	}
}

func buildPSDTextLayerPayload(layer *TextLayer) []byte {
	if layer == nil {
		return nil
	}
	var out bytes.Buffer

	// Type tool payload layout:
	// version, affine transform, text descriptor, warp descriptor, and bounds.
	writePSDUint16(&out, 1)
	for _, value := range []float64{1, 0, 0, 1, 0, 0} {
		writePSDFloat64(&out, value)
	}
	writePSDUint16(&out, 50)
	writePSDUint32(&out, 16)
	textItems := []psdDescriptorItem{
		{Key: "Txt ", Type: "TEXT", Text: layer.Text},
		{Key: "font", Type: "TEXT", Text: layer.FontFamily},
		{Key: "fontStyle", Type: "TEXT", Text: layer.FontStyle},
		{Key: "antiAlias", Type: "TEXT", Text: layer.AntiAlias},
		{Key: "alignment", Type: "TEXT", Text: layer.Alignment},
		{Key: "textType", Type: "TEXT", Text: layer.TextType},
		{Key: "orientation", Type: "TEXT", Text: layer.Orientation},
		{Key: "fontSize", Type: "doub", Float64: layer.FontSize},
		{Key: "tracking", Type: "doub", Float64: layer.Tracking},
		{Key: "leading", Type: "doub", Float64: layer.Leading},
		{Key: "baselineShift", Type: "doub", Float64: layer.BaselineShift},
		{Key: "kerning", Type: "doub", Float64: layer.Kerning},
		{Key: "color", Type: "TEXT", Text: marshalPSDJSON(layer.Color)},
		{Key: "styleJSON", Type: "TEXT", Text: marshalPSDJSON(map[string]any{
			"bold":          layer.Bold,
			"italic":        layer.Italic,
			"language":      layer.Language,
			"superscript":   layer.Superscript,
			"subscript":     layer.Subscript,
			"underline":     layer.Underline,
			"strikethrough": layer.Strikethrough,
			"allCaps":       layer.AllCaps,
			"smallCaps":     layer.SmallCaps,
			"indentLeft":    layer.IndentLeft,
			"indentRight":   layer.IndentRight,
			"indentFirst":   layer.IndentFirst,
			"spaceBefore":   layer.SpaceBefore,
			"spaceAfter":    layer.SpaceAfter,
		})},
	}
	writePSDDescriptor(&out, "", "TxLr", textItems)

	writePSDUint16(&out, 1)
	writePSDUint32(&out, 16)
	writePSDDescriptor(&out, "", "warp", []psdDescriptorItem{
		{Key: "warpStyle", Type: "TEXT", Text: "warpNone"},
		{Key: "warpValue", Type: "doub", Float64: 0},
		{Key: "warpPerspective", Type: "doub", Float64: 0},
		{Key: "warpPerspectiveOther", Type: "doub", Float64: 0},
	})

	writePSDInt32(&out, int32(layer.Bounds.X))
	writePSDInt32(&out, int32(layer.Bounds.Y))
	writePSDInt32(&out, int32(layer.Bounds.X+layer.Bounds.W))
	writePSDInt32(&out, int32(layer.Bounds.Y+layer.Bounds.H))
	return out.Bytes()
}

func buildPSDAdjustmentLayerPayload(layer *AdjustmentLayer) []byte {
	if layer == nil {
		return nil
	}
	payload := map[string]any{
		"kind": stringValueOrDefault(layer.AdjustmentKind, ""),
		"params": func() any {
			if len(layer.Params) == 0 {
				return map[string]any{}
			}
			var parsed any
			if err := json.Unmarshal(layer.Params, &parsed); err == nil {
				return parsed
			}
			return string(layer.Params)
		}(),
	}
	var out bytes.Buffer
	writePSDUint16(&out, 1)
	out.WriteString(marshalPSDJSON(payload))
	return out.Bytes()
}

func marshalPSDJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

type psdDescriptorItem struct {
	Key     string
	Type    string
	Text    string
	Bool    bool
	Float64 float64
	Int32   int32
}

func writePSDDescriptor(out *bytes.Buffer, name, classID string, items []psdDescriptorItem) {
	writePSDUnicodeString(out, name)
	writePSDDescriptorID(out, classID)
	writePSDUint32(out, uint32(len(items)))
	for _, item := range items {
		writePSDDescriptorID(out, item.Key)
		writePSDString(out, item.Type)
		switch item.Type {
		case "TEXT":
			writePSDUnicodeString(out, item.Text)
		case "bool":
			if item.Bool {
				out.WriteByte(1)
			} else {
				out.WriteByte(0)
			}
		case "doub":
			writePSDFloat64(out, item.Float64)
		case "long":
			writePSDInt32(out, item.Int32)
		default:
			writePSDUnicodeString(out, item.Text)
		}
	}
}

func writePSDUnicodeString(out *bytes.Buffer, value string) {
	encoded := utf16Encode(value)
	writePSDUint32(out, uint32(len(encoded)))
	for _, r := range encoded {
		writePSDUint16(out, r)
	}
}

func writePSDDescriptorID(out *bytes.Buffer, value string) {
	if len(value) == 4 {
		writePSDUint32(out, 0)
		writePSDString(out, value)
		return
	}
	writePSDUint32(out, uint32(len(value)))
	writePSDString(out, value)
}

func writePSDFloat64(out *bytes.Buffer, value float64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(value))
	out.Write(buf[:])
}

func writePSDImageResource(out *bytes.Buffer, resourceID uint16, name string, payload []byte) {
	writePSDString(out, "8BIM")
	writePSDUint16(out, resourceID)
	writePSDPascalString2(out, name)
	writePSDUint32(out, uint32(len(payload)))
	out.Write(payload)
	if len(payload)%2 != 0 {
		out.WriteByte(0)
	}
}

func writePSDAdditionalLayerInfoBlock(out *bytes.Buffer, signature, key string, payload []byte) {
	writePSDString(out, signature)
	writePSDString(out, key)
	writePSDUint32(out, uint32(len(payload)))
	out.Write(payload)
	if len(payload)%2 != 0 {
		out.WriteByte(0)
	}
}

func writePSDPascalString2(out *bytes.Buffer, value string) {
	if len(value) > math.MaxUint8 {
		value = value[:math.MaxUint8]
	}
	out.WriteByte(byte(len(value)))
	out.WriteString(value)
	if (1+len(value))%2 != 0 {
		out.WriteByte(0)
	}
}

func writePSDPascalString4(out *bytes.Buffer, value string) {
	if len(value) > math.MaxUint8 {
		value = value[:math.MaxUint8]
	}
	out.WriteByte(byte(len(value)))
	out.WriteString(value)
	for (1+len(value))%4 != 0 {
		out.WriteByte(0)
		value += "\x00"
	}
}

func writePSDSectionLength(out *bytes.Buffer, psb bool, length uint64) {
	if psb {
		writePSDUint64(out, length)
		return
	}
	writePSDUint32(out, uint32(length))
}

func writePSDString(out *bytes.Buffer, value string) {
	out.WriteString(value)
}

func writePSDUint16(out *bytes.Buffer, value uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], value)
	out.Write(buf[:])
}

func writePSDUint32(out *bytes.Buffer, value uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], value)
	out.Write(buf[:])
}

func writePSDUint64(out *bytes.Buffer, value uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	out.Write(buf[:])
}

func writePSDInt16(out *bytes.Buffer, value int16) {
	writePSDUint16(out, uint16(value))
}

func writePSDInt32(out *bytes.Buffer, value int32) {
	writePSDUint32(out, uint32(value))
}

func unitToPSDOpacity(value float64) uint8 {
	return uint8(math.Round(clampUnit(value) * 255))
}

func psdBlendKey(mode BlendMode) string {
	switch mode {
	case BlendModeMultiply:
		return "mul "
	case BlendModeScreen:
		return "scrn"
	case BlendModeOverlay:
		return "over"
	case BlendModeDifference:
		return "diff"
	case BlendModeExclusion:
		return "smud"
	case BlendModeDarken:
		return "dark"
	case BlendModeLighten:
		return "lite"
	default:
		return "norm"
	}
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
