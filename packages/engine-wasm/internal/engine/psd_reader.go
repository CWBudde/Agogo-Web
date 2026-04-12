package engine

import (
	"compress/zlib"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"
)

const (
	psdColorModeGrayscale = 1
	psdColorModeRGB       = 3

	psdCompressionRaw           = iota
	psdCompressionRLE
	psdCompressionZip
	psdCompressionZipPrediction
)

type psdHeader struct {
	Version   uint16
	PSB       bool
	Channels  int
	Height    int
	Width     int
	Depth     int
	ColorMode int
}

type psdParser struct {
	r        *bytes.Reader
	warnings []string
}

type psdImageResources struct {
	Resolution float64
}

type psdLayerRecord struct {
	Name              string
	Bounds            LayerBounds
	Channels          []psdChannelInfo
	Opacity           float64
	Visible           bool
	ClipToBelow       bool
	BlendMode         BlendMode
	ChannelPixels     map[int16][]byte
	UnsupportedBlocks []string
}

type psdChannelInfo struct {
	ID     int16
	Length uint64
}

// LoadPSD parses a PSD/PSB byte stream and maps supported content into a Document.
func LoadPSD(data []byte) (*Document, []string, error) {
	parser := &psdParser{r: bytes.NewReader(data)}

	header, err := parser.parseHeader()
	if err != nil {
		return nil, nil, err
	}
	if header.Depth != 8 {
		return nil, nil, fmt.Errorf("unsupported PSD bit depth %d", header.Depth)
	}
	if header.ColorMode != psdColorModeRGB && header.ColorMode != psdColorModeGrayscale {
		return nil, nil, fmt.Errorf("unsupported PSD color mode %d", header.ColorMode)
	}

	if err := parser.skipColorModeData(); err != nil {
		return nil, nil, err
	}
	resources, err := parser.parseImageResources()
	if err != nil {
		return nil, nil, err
	}
	layers, err := parser.parseLayerAndMaskInfo(header)
	if err != nil {
		return nil, nil, err
	}
	compositeRGBA, err := parser.parseCompositeImageData(header)
	if err != nil {
		return nil, nil, err
	}

	doc := newImportedPSDDocument(header, resources)
	importedLayers, warnings, err := buildPSDLayerNodes(header, layers)
	if err != nil {
		return nil, nil, err
	}
	parser.warnings = append(parser.warnings, warnings...)

	if len(importedLayers) == 0 && len(compositeRGBA) > 0 {
		importedLayers = append(importedLayers, NewPixelLayer("Background", LayerBounds{
			X: 0, Y: 0, W: header.Width, H: header.Height,
		}, compositeRGBA))
	}
	doc.LayerRoot.SetChildren(importedLayers)
	doc.normalizeClippingState()
	if len(importedLayers) > 0 {
		doc.ActiveLayerID = importedLayers[len(importedLayers)-1].ID()
	}
	return doc, append([]string(nil), parser.warnings...), nil
}

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
	nodes := make([]LayerNode, 0, len(layers))
	var warnings []string
	for index, record := range layers {
		rgba, err := flattenPSDLayerPixels(header, record)
		if err != nil {
			return nil, nil, fmt.Errorf("flatten layer %q: %w", record.Name, err)
		}
		name := record.Name
		if name == "" {
			name = fmt.Sprintf("Layer %d", index+1)
		}
		layer := NewPixelLayer(name, record.Bounds, rgba)
		layer.SetOpacity(record.Opacity)
		layer.SetVisible(record.Visible)
		layer.SetBlendMode(record.BlendMode)
		layer.SetClipToBelow(record.ClipToBelow)
		for _, key := range record.UnsupportedBlocks {
			warnings = append(warnings, fmt.Sprintf("layer %q: unsupported metadata block %s imported as flattened pixel layer", name, key))
		}
		nodes = append(nodes, layer)
	}
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

func (p *psdParser) parseHeader() (psdHeader, error) {
	signature, err := p.readString(4)
	if err != nil {
		return psdHeader{}, err
	}
	if signature != "8BPS" {
		return psdHeader{}, fmt.Errorf("invalid PSD signature %q", signature)
	}
	version, err := p.readUint16()
	if err != nil {
		return psdHeader{}, err
	}
	if version != 1 && version != 2 {
		return psdHeader{}, fmt.Errorf("unsupported PSD version %d", version)
	}
	reserved, err := p.readBytes(6)
	if err != nil {
		return psdHeader{}, err
	}
	for _, b := range reserved {
		if b != 0 {
			return psdHeader{}, fmt.Errorf("invalid PSD reserved bytes")
		}
	}
	channels, err := p.readUint16()
	if err != nil {
		return psdHeader{}, err
	}
	height, err := p.readUint32()
	if err != nil {
		return psdHeader{}, err
	}
	width, err := p.readUint32()
	if err != nil {
		return psdHeader{}, err
	}
	depth, err := p.readUint16()
	if err != nil {
		return psdHeader{}, err
	}
	colorMode, err := p.readUint16()
	if err != nil {
		return psdHeader{}, err
	}
	return psdHeader{
		Version:   version,
		PSB:       version == 2,
		Channels:  int(channels),
		Height:    int(height),
		Width:     int(width),
		Depth:     int(depth),
		ColorMode: int(colorMode),
	}, nil
}

func (p *psdParser) skipColorModeData() error {
	length, err := p.readUint32()
	if err != nil {
		return err
	}
	_, err = p.readBytes(int(length))
	return err
}

func (p *psdParser) parseImageResources() (psdImageResources, error) {
	length, err := p.readUint32()
	if err != nil {
		return psdImageResources{}, err
	}
	data, err := p.readBytes(int(length))
	if err != nil {
		return psdImageResources{}, err
	}
	reader := bytes.NewReader(data)
	resources := psdImageResources{}
	for reader.Len() > 0 {
		signature, err := readStringFrom(reader, 4)
		if err != nil {
			return resources, err
		}
		if signature != "8BIM" {
			return resources, fmt.Errorf("invalid image resource signature %q", signature)
		}
		id, err := readUint16From(reader)
		if err != nil {
			return resources, err
		}
		nameLen, err := reader.ReadByte()
		if err != nil {
			return resources, err
		}
		if _, err := io.CopyN(io.Discard, reader, int64(nameLen)); err != nil {
			return resources, err
		}
		if (1+int(nameLen))%2 != 0 {
			if _, err := reader.ReadByte(); err != nil {
				return resources, err
			}
		}
		size, err := readUint32From(reader)
		if err != nil {
			return resources, err
		}
		payload, err := readBytesFrom(reader, int(size))
		if err != nil {
			return resources, err
		}
		if size%2 != 0 {
			if _, err := reader.ReadByte(); err != nil {
				return resources, err
			}
		}
		switch id {
		case 0x03ed:
			if len(payload) >= 4 {
				fixed := binary.BigEndian.Uint32(payload[:4])
				resources.Resolution = float64(fixed) / 65536.0
			}
		}
	}
	return resources, nil
}

func (p *psdParser) parseLayerAndMaskInfo(header psdHeader) ([]psdLayerRecord, error) {
	length, err := p.readSectionLength(header.PSB)
	if err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	data, err := p.readBytes(int(length))
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	layerInfoLen, err := readSectionLengthFrom(reader, header.PSB)
	if err != nil {
		return nil, err
	}
	if layerInfoLen == 0 {
		return nil, nil
	}
	layerInfoData, err := readBytesFrom(reader, int(layerInfoLen))
	if err != nil {
		return nil, err
	}
	layerReader := bytes.NewReader(layerInfoData)
	layerCountRaw, err := readInt16From(layerReader)
	if err != nil {
		return nil, err
	}
	layerCount := int(layerCountRaw)
	if layerCount < 0 {
		layerCount = -layerCount
	}
	layers := make([]psdLayerRecord, 0, layerCount)
	for i := 0; i < layerCount; i++ {
		record, err := parsePSDLayerRecord(layerReader, header.PSB)
		if err != nil {
			return nil, err
		}
		layers = append(layers, record)
	}
	for i := range layers {
		channelPixels := make(map[int16][]byte, len(layers[i].Channels))
		for _, channel := range layers[i].Channels {
			pixels, err := parsePSDChannelImageData(layerReader, header.PSB, channel.Length, layers[i].Bounds.W, layers[i].Bounds.H)
			if err != nil {
				return nil, fmt.Errorf("decode layer %q channel %d: %w", layers[i].Name, channel.ID, err)
			}
			channelPixels[channel.ID] = pixels
		}
		layers[i].ChannelPixels = channelPixels
	}
	return layers, nil
}

func parsePSDLayerRecord(reader *bytes.Reader, psb bool) (psdLayerRecord, error) {
	top, err := readInt32From(reader)
	if err != nil {
		return psdLayerRecord{}, err
	}
	left, err := readInt32From(reader)
	if err != nil {
		return psdLayerRecord{}, err
	}
	bottom, err := readInt32From(reader)
	if err != nil {
		return psdLayerRecord{}, err
	}
	right, err := readInt32From(reader)
	if err != nil {
		return psdLayerRecord{}, err
	}
	channelCount, err := readUint16From(reader)
	if err != nil {
		return psdLayerRecord{}, err
	}
	record := psdLayerRecord{
		Bounds: LayerBounds{
			X: int(left),
			Y: int(top),
			W: int(right - left),
			H: int(bottom - top),
		},
		Opacity:   1,
		Visible:   true,
		BlendMode: BlendModeNormal,
		Channels:  make([]psdChannelInfo, 0, int(channelCount)),
	}
	for i := 0; i < int(channelCount); i++ {
		id, err := readInt16From(reader)
		if err != nil {
			return record, err
		}
		length, err := readSectionLengthFrom(reader, psb)
		if err != nil {
			return record, err
		}
		record.Channels = append(record.Channels, psdChannelInfo{ID: id, Length: length})
	}
	blendSig, err := readStringFrom(reader, 4)
	if err != nil {
		return record, err
	}
	if blendSig != "8BIM" {
		return record, fmt.Errorf("invalid layer blend signature %q", blendSig)
	}
	blendKey, err := readStringFrom(reader, 4)
	if err != nil {
		return record, err
	}
	record.BlendMode = mapPSDBlendMode(blendKey)
	opacity, err := reader.ReadByte()
	if err != nil {
		return record, err
	}
	record.Opacity = float64(opacity) / 255.0
	clipping, err := reader.ReadByte()
	if err != nil {
		return record, err
	}
	record.ClipToBelow = clipping != 0
	flags, err := reader.ReadByte()
	if err != nil {
		return record, err
	}
	record.Visible = (flags & 0x02) == 0
	if _, err := reader.ReadByte(); err != nil {
		return record, err
	}
	extraLen, err := readUint32From(reader)
	if err != nil {
		return record, err
	}
	extra, err := readBytesFrom(reader, int(extraLen))
	if err != nil {
		return record, err
	}
	if err := parsePSDLayerExtraData(extra, &record); err != nil {
		return record, err
	}
	return record, nil
}

func parsePSDLayerExtraData(data []byte, record *psdLayerRecord) error {
	reader := bytes.NewReader(data)
	maskLen, err := readUint32From(reader)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(io.Discard, reader, int64(maskLen)); err != nil {
		return err
	}
	blendRangeLen, err := readUint32From(reader)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(io.Discard, reader, int64(blendRangeLen)); err != nil {
		return err
	}
	name, err := readPascalString4(reader)
	if err != nil {
		return err
	}
	record.Name = name
	for reader.Len() > 0 {
		signature, err := readStringFrom(reader, 4)
		if err != nil {
			return err
		}
		if signature != "8BIM" && signature != "8B64" {
			return fmt.Errorf("invalid layer info signature %q", signature)
		}
		key, err := readStringFrom(reader, 4)
		if err != nil {
			return err
		}
		length, err := readUint32From(reader)
		if err != nil {
			return err
		}
		payload, err := readBytesFrom(reader, int(length))
		if err != nil {
			return err
		}
		if length%2 != 0 {
			if _, err := reader.ReadByte(); err != nil {
				return err
			}
		}
		switch key {
		case "luni":
			if unicodeName, err := parsePSDUnicodeString(payload); err == nil && unicodeName != "" {
				record.Name = unicodeName
			}
		case "TySh", "tySh", "vmsk", "vsms", "PlLd", "SoLd", "lfx2", "lrFX", "levl", "curv", "hue2":
			record.UnsupportedBlocks = append(record.UnsupportedBlocks, key)
		}
	}
	return nil
}

func (p *psdParser) parseCompositeImageData(header psdHeader) ([]byte, error) {
	compression, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	pixelsPerPlane := header.Width * header.Height
	if pixelsPerPlane == 0 {
		return nil, nil
	}
	planes := make([][]byte, header.Channels)
	switch compression {
	case psdCompressionRaw:
		for i := 0; i < header.Channels; i++ {
			planes[i], err = p.readBytes(pixelsPerPlane)
			if err != nil {
				return nil, err
			}
		}
	case psdCompressionRLE:
		counts := make([]int, header.Channels*header.Height)
		for i := range counts {
			if header.PSB {
				value, err := p.readUint32()
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			} else {
				value, err := p.readUint16()
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			}
		}
		for planeIndex := 0; planeIndex < header.Channels; planeIndex++ {
			plane := make([]byte, 0, pixelsPerPlane)
			for row := 0; row < header.Height; row++ {
				size := counts[planeIndex*header.Height+row]
				encoded, err := p.readBytes(size)
				if err != nil {
					return nil, err
				}
				decoded, err := decodePackBits(encoded, header.Width)
				if err != nil {
					return nil, err
				}
				plane = append(plane, decoded...)
			}
			planes[planeIndex] = plane
		}
	case psdCompressionZip, psdCompressionZipPrediction:
		compressed, err := p.readBytes(p.r.Len())
		if err != nil {
			return nil, err
		}
		flat, err := decodeZipImageData(compressed, header.Channels, pixelsPerPlane, header.Width, header.Height, compression == psdCompressionZipPrediction)
		if err != nil {
			return nil, err
		}
		for i := 0; i < header.Channels; i++ {
			plane := flat[i*pixelsPerPlane : (i+1)*pixelsPerPlane]
			planes[i] = append([]byte(nil), plane...)
		}
	default:
		return nil, fmt.Errorf("unsupported PSD composite compression %d", compression)
	}
	return compositePSDPlanesToRGBA(header.ColorMode, planes, pixelsPerPlane)
}

func parsePSDChannelImageData(reader *bytes.Reader, psb bool, declaredLength uint64, width, height int) ([]byte, error) {
	data, err := readBytesFrom(reader, int(declaredLength))
	if err != nil {
		return nil, err
	}
	channelReader := bytes.NewReader(data)
	compression, err := readUint16From(channelReader)
	if err != nil {
		return nil, err
	}
	switch compression {
	case psdCompressionRaw:
		return readBytesFrom(channelReader, width*height)
	case psdCompressionRLE:
		if width <= 0 || height <= 0 {
			return nil, nil
		}
		counts := make([]int, height)
		for i := range counts {
			if psb {
				value, err := readUint32From(channelReader)
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			} else {
				value, err := readUint16From(channelReader)
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			}
		}
		decoded := make([]byte, 0, width*height)
		for _, count := range counts {
			rowData, err := readBytesFrom(channelReader, count)
			if err != nil {
				return nil, err
			}
			row, err := decodePackBits(rowData, width)
			if err != nil {
				return nil, err
			}
			decoded = append(decoded, row...)
		}
		if len(decoded) != width*height {
			return nil, fmt.Errorf("decoded RLE channel length %d, want %d", len(decoded), width*height)
		}
		return decoded, nil
	case psdCompressionZip, psdCompressionZipPrediction:
		compressed, err := readBytesFrom(channelReader, int(declaredLength))
		if err != nil {
			return nil, err
		}
		return decodeZipChannel(compressed, width, height, compression == psdCompressionZipPrediction)
	default:
		return nil, fmt.Errorf("unsupported PSD layer compression %d", compression)
	}
}

func decodeZipChannel(data []byte, width, height int, withPrediction bool) ([]byte, error) {
	pixels, err := decodeZipPayload(data)
	if err != nil {
		return nil, err
	}
	pixelCount := width * height
	if len(pixels) != pixelCount {
		return nil, fmt.Errorf("decoded zip channel length %d, want %d", len(pixels), pixelCount)
	}
	if withPrediction {
		applyZipPredictionInPlace(pixels, width, height)
	}
	return pixels, nil
}

func decodeZipImageData(compressed []byte, channelCount, pixelCount, width, height int, withPrediction bool) ([]byte, error) {
	decoded, err := decodeZipPayload(compressed)
	if err != nil {
		return nil, err
	}
	expected := channelCount * pixelCount
	if len(decoded) != expected {
		return nil, fmt.Errorf("decoded zip image length %d, want %d", len(decoded), expected)
	}
	if withPrediction {
		for i := 0; i < channelCount; i++ {
			start := i * pixelCount
			end := start + pixelCount
			applyZipPredictionInPlace(decoded[start:end], width, height)
		}
	}
	return decoded, nil
}

func decodeZipPayload(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to init zip stream: %w", err)
	}
	defer zr.Close()
	decoded, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode zip stream: %w", err)
	}
	return decoded, nil
}

func applyZipPredictionInPlace(data []byte, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	for row := 0; row < height; row++ {
		rowStart := row * width
		rowEnd := rowStart + width
		if rowEnd > len(data) {
			break
		}
		for col := 1; col < width && rowStart+col < rowEnd; col++ {
			data[rowStart+col] = data[rowStart+col] + data[rowStart+col-1]
		}
	}
}

func compositePSDPlanesToRGBA(colorMode int, planes [][]byte, pixelCount int) ([]byte, error) {
	if pixelCount == 0 {
		return nil, nil
	}
	rgba := make([]byte, pixelCount*4)
	switch colorMode {
	case psdColorModeRGB:
		if len(planes) < 3 {
			return nil, fmt.Errorf("composite image missing RGB planes")
		}
		for i := 0; i < pixelCount; i++ {
			rgba[i*4] = planes[0][i]
			rgba[i*4+1] = planes[1][i]
			rgba[i*4+2] = planes[2][i]
			rgba[i*4+3] = 255
			if len(planes) > 3 && len(planes[3]) == pixelCount {
				rgba[i*4+3] = planes[3][i]
			}
		}
	case psdColorModeGrayscale:
		if len(planes) == 0 {
			return nil, fmt.Errorf("composite image missing grayscale plane")
		}
		for i := 0; i < pixelCount; i++ {
			value := planes[0][i]
			rgba[i*4] = value
			rgba[i*4+1] = value
			rgba[i*4+2] = value
			rgba[i*4+3] = 255
			if len(planes) > 1 && len(planes[1]) == pixelCount {
				rgba[i*4+3] = planes[1][i]
			}
		}
	default:
		return nil, fmt.Errorf("unsupported composite color mode %d", colorMode)
	}
	return rgba, nil
}

func parsePSDUnicodeString(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	length, err := readUint32From(reader)
	if err != nil {
		return "", err
	}
	if int(length)*2 > reader.Len() {
		return "", fmt.Errorf("invalid PSD unicode string length %d", length)
	}
	chars := make([]uint16, length)
	for i := range chars {
		value, err := readUint16From(reader)
		if err != nil {
			return "", err
		}
		chars[i] = value
	}
	return string(utf16.Decode(chars)), nil
}

func decodePackBits(data []byte, expectedLen int) ([]byte, error) {
	out := make([]byte, 0, expectedLen)
	for i := 0; i < len(data) && len(out) < expectedLen; {
		control := int(int8(data[i]))
		i++
		switch {
		case control >= 0:
			count := control + 1
			if i+count > len(data) {
				return nil, fmt.Errorf("packbits literal overruns row")
			}
			out = append(out, data[i:i+count]...)
			i += count
		case control >= -127:
			count := 1 - control
			if i >= len(data) {
				return nil, fmt.Errorf("packbits repeat overruns row")
			}
			value := data[i]
			i++
			for range count {
				out = append(out, value)
			}
		default:
		}
	}
	if len(out) != expectedLen {
		return nil, fmt.Errorf("packbits decoded %d bytes, want %d", len(out), expectedLen)
	}
	return out, nil
}

func psdDocumentColorMode(colorMode int) string {
	switch colorMode {
	case psdColorModeGrayscale:
		return "gray"
	default:
		return "rgb"
	}
}

func layerPixelCount(bounds LayerBounds) int {
	if bounds.W <= 0 || bounds.H <= 0 {
		return 0
	}
	return bounds.W * bounds.H
}

func mapPSDBlendMode(key string) BlendMode {
	switch strings.TrimSpace(key) {
	case "mul":
		return BlendModeMultiply
	case "scrn":
		return BlendModeScreen
	case "over":
		return BlendModeOverlay
	case "diff":
		return BlendModeDifference
	case "smud":
		return BlendModeExclusion
	case "dark":
		return BlendModeDarken
	case "lite":
		return BlendModeLighten
	default:
		return BlendModeNormal
	}
}

func (p *psdParser) readBytes(n int) ([]byte, error) {
	return readBytesFrom(p.r, n)
}

func (p *psdParser) readString(n int) (string, error) {
	return readStringFrom(p.r, n)
}

func (p *psdParser) readUint16() (uint16, error) {
	return readUint16From(p.r)
}

func (p *psdParser) readUint32() (uint32, error) {
	return readUint32From(p.r)
}

func (p *psdParser) readSectionLength(psb bool) (uint64, error) {
	return readSectionLengthFrom(p.r, psb)
}

func readBytesFrom(r io.Reader, n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("invalid read length %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func readStringFrom(r io.Reader, n int) (string, error) {
	buf, err := readBytesFrom(r, n)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func readUint16From(r io.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readInt16From(r io.Reader) (int16, error) {
	var value int16
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readUint32From(r io.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readInt32From(r io.Reader) (int32, error) {
	var value int32
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readSectionLengthFrom(r io.Reader, psb bool) (uint64, error) {
	if psb {
		var value uint64
		err := binary.Read(r, binary.BigEndian, &value)
		return value, err
	}
	value, err := readUint32From(r)
	return uint64(value), err
}

func readPascalString4(r *bytes.Reader) (string, error) {
	length, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	buf, err := readBytesFrom(r, int(length))
	if err != nil {
		return "", err
	}
	padding := (4 - ((1 + int(length)) % 4)) % 4
	if padding > 0 {
		if _, err := io.CopyN(io.Discard, r, int64(padding)); err != nil {
			return "", err
		}
	}
	return string(buf), nil
}
