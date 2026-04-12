package engine

import (
	"bytes"
	"compress/zlib"
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

	psdCompressionRaw = iota
	psdCompressionRLE
	psdCompressionZip
	psdCompressionZipPrediction
)

const (
	psdImageResourceDPI        = 0x03ed
	psdImageResourceICCProfile = 0x040f
	psdImageResourceGuides     = 0x0408
	psdImageResourceSlices     = 0x041a
	psdImageResourceLayerComps = 0x0435
)

const (
	psdLayerSectionNormal      = 0
	psdLayerSectionOpenFolder  = 1
	psdLayerSectionCloseFolder = 2
	psdLayerSectionNested      = 3
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
	Resolution    float64
	HasICCProfile bool
	HasGuides     bool
	HasSlices     bool
	HasLayerComps bool
}

type psdLayerEffectsMeta struct {
	Legacy *psdLegacyLayerEffectsMeta
	Object *psdObjectLayerEffectsMeta
}

type psdLegacyLayerEffectsMeta struct {
	Version     uint16
	EffectCount uint16
	EffectKeys  []string
	Malformed   bool
}

type psdObjectLayerEffectsMeta struct {
	ObjectVersion     uint32
	DescriptorVersion uint32
	HasDescriptor     bool
	Malformed         bool
	EffectKeys        []string
}

type psdAdjustmentMeta struct {
	Key        string
	Kind       string
	Version    uint16
	HasVersion bool
	PayloadLen int
	Malformed  bool
}

type psdSmartObjectMeta struct {
	Key           string
	Version       uint32
	Identifier    string
	UniqueID      string
	PayloadLen    int
	HasDescriptor bool
	HasVersion    bool
	Malformed     bool
	PageNumber    *uint32
	TotalPages    *uint32
	PlacedType    *uint32
}

type psdVectorMaskMeta struct {
	Key          string
	PayloadLen   int
	HasBounds    bool
	Bounds       LayerBounds
	DefaultColor uint16
	Flags        uint16
	Malformed    bool
}

type psdTextLayerMeta struct {
	Key               string
	PayloadLen        int
	ParsedText        string
	DescriptorVersion uint32
	HasDescriptor     bool
	Malformed         bool
}

type psdLayerRecord struct {
	Name              string
	Bounds            LayerBounds
	Channels          []psdChannelInfo
	Opacity           float64
	Visible           bool
	ClipToBelow       bool
	BlendMode         BlendMode
	LayerID           uint32
	LayerColorTag     string
	SectionType       uint32
	HasLayerMask      bool
	LayerMaskBounds   LayerBounds
	LayerMaskEnabled  bool
	HasVectorMask     bool
	VectorMask        *psdVectorMaskMeta
	Effects           *psdLayerEffectsMeta
	Adjustments       []psdAdjustmentMeta
	SmartObject       *psdSmartObjectMeta
	Text              *psdTextLayerMeta
	ChannelPixels     map[int16][]byte
	UnsupportedBlocks []string
	MetadataWarnings  []string
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
		if len(layers) == 0 {
			return nil, nil, err
		}
		parser.warnings = append(parser.warnings, fmt.Sprintf("partial layer info: %v", err))
	}
	compositeRGBA, err := parser.parseCompositeImageData(header)
	if err != nil {
		if len(layers) == 0 {
			return nil, nil, err
		}
		parser.warnings = append(parser.warnings, fmt.Sprintf("partial composite image: %v", err))
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

func (p *psdParser) warnf(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
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
		for _, warning := range record.MetadataWarnings {
			warnings = append(warnings, warning)
		}
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
			p.warnf("invalid image resource signature %q", signature)
			return resources, nil
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
		case psdImageResourceDPI:
			if len(payload) >= 4 {
				fixed := binary.BigEndian.Uint32(payload[:4])
				resources.Resolution = float64(fixed) / 65536.0
			}
		case psdImageResourceICCProfile:
			resources.HasICCProfile = true
		case psdImageResourceGuides:
			resources.HasGuides = true
		case psdImageResourceSlices:
			resources.HasSlices = true
		case psdImageResourceLayerComps:
			resources.HasLayerComps = true
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
			p.warnf("failed parsing layer %d: %v", i+1, err)
			return layers, err
		}
		layers = append(layers, record)
	}
	for i := range layers {
		channelPixels := make(map[int16][]byte, len(layers[i].Channels))
		for _, channel := range layers[i].Channels {
			pixels, err := parsePSDChannelImageData(layerReader, header.PSB, channel.Length, layers[i].Bounds.W, layers[i].Bounds.H)
			if err != nil {
				p.warnf("decode layer %q channel %d failed: %v", layers[i].Name, channel.ID, err)
				continue
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
	if maskLen > 0 {
		maskData, err := readBytesFrom(reader, int(maskLen))
		if err != nil {
			return err
		}
		if len(maskData) >= 18 {
			maskReader := bytes.NewReader(maskData)
			top, err := readInt32From(maskReader)
			if err != nil {
				return err
			}
			left, err := readInt32From(maskReader)
			if err != nil {
				return err
			}
			bottom, err := readInt32From(maskReader)
			if err != nil {
				return err
			}
			right, err := readInt32From(maskReader)
			if err != nil {
				return err
			}
			width := int(right - left)
			height := int(bottom - top)
			if width < 0 {
				width = 0
			}
			if height < 0 {
				height = 0
			}
			record.LayerMaskBounds = LayerBounds{
				X: int(left),
				Y: int(top),
				W: width,
				H: height,
			}
			record.LayerMaskEnabled = (maskData[16] & 0x01) == 0
			record.HasLayerMask = true
		}
	}
	blendRangeLen, err := readUint32From(reader)
	if err != nil {
		return err
	}
	if blendRangeLen > 0 {
		if _, err := io.CopyN(io.Discard, reader, int64(blendRangeLen)); err != nil {
			return err
		}
	}
	name, err := readPascalString4(reader)
	if err != nil {
		return err
	}
	record.Name = name
	addMetadataWarning := func(format string, args ...any) {
		record.MetadataWarnings = append(record.MetadataWarnings, fmt.Sprintf(format, args...))
	}
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
		case "lyid":
			if len(payload) >= 4 {
				record.LayerID = binary.BigEndian.Uint32(payload[:4])
			}
		case "lclr":
			record.LayerColorTag = parsePSDLayerColorTag(payload)
		case "lsct":
			if len(payload) >= 4 {
				record.SectionType = binary.BigEndian.Uint32(payload[:4])
			}
		case "vmsk", "vsms":
			if err := parsePSDLayerVectorMaskMetadata(key, payload, record); err != nil {
				addMetadataWarning("layer %q: malformed vector mask metadata (%s) ignored", record.Name, key)
			}
		case "lfx2":
			if err := parsePSDLayerObjectEffectsPayload(payload, record); err != nil {
				addMetadataWarning("layer %q: malformed modern layer effects metadata (%v) ignored", record.Name, err)
			}
		case "lrFX":
			if err := parsePSDLayerLegacyEffectsPayload(payload, record); err != nil {
				addMetadataWarning("layer %q: malformed legacy layer effects metadata (%v) ignored", record.Name, err)
			}
		case "levl", "curv", "hue2":
			if err := parsePSDLayerAdjustmentMetadata(key, payload, record); err != nil {
				addMetadataWarning("layer %q: malformed adjustment metadata (%s) ignored", record.Name, key)
			}
		case "plLd", "PlLd", "SoLd":
			if err := parsePSDLayerSmartObjectMetadata(key, payload, record); err != nil {
				addMetadataWarning("layer %q: malformed smart object metadata (%s) ignored", record.Name, key)
			}
		case "TySh", "tySh":
			if err := parsePSDTextLayerMetadata(key, payload, record); err != nil {
				addMetadataWarning("layer %q: malformed text metadata (%s) ignored", record.Name, key)
			}
			record.UnsupportedBlocks = append(record.UnsupportedBlocks, key)
		}
	}
	return nil
}

func parsePSDLayerObjectEffectsPayload(payload []byte, record *psdLayerRecord) error {
	if record.Effects == nil {
		record.Effects = &psdLayerEffectsMeta{}
	}
	meta := &psdObjectLayerEffectsMeta{}
	record.Effects.Object = meta
	if len(payload) < 8 {
		meta.Malformed = true
		return fmt.Errorf("lfx2 payload too short")
	}
	meta.ObjectVersion = binary.BigEndian.Uint32(payload[0:4])
	meta.DescriptorVersion = binary.BigEndian.Uint32(payload[4:8])
	meta.HasDescriptor = len(payload) > 8
	if len(payload) > 8 {
		meta.EffectKeys = scanObjectEffectStyleKeys(payload[8:])
	}
	return nil
}

func parsePSDLayerLegacyEffectsPayload(payload []byte, record *psdLayerRecord) error {
	reader := bytes.NewReader(payload)
	if record.Effects == nil {
		record.Effects = &psdLayerEffectsMeta{}
	}
	if record.Effects.Legacy == nil {
		record.Effects.Legacy = &psdLegacyLayerEffectsMeta{}
	}
	meta := record.Effects.Legacy
	version, err := readUint16From(reader)
	if err != nil {
		meta.Malformed = true
		meta.Version = 0
		meta.EffectCount = 0
		return nil
	}
	meta.Version = version
	count, err := readUint16From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	meta.EffectCount = count
	for i := uint16(0); i < count; i++ {
		signature, err := readStringFrom(reader, 4)
		if err != nil {
			meta.Malformed = true
			return err
		}
		if signature != "8BIM" && signature != "8B64" {
			meta.Malformed = true
			return fmt.Errorf("invalid legacy effect signature %q", signature)
		}
		key, err := readStringFrom(reader, 4)
		if err != nil {
			meta.Malformed = true
			return err
		}
		meta.EffectKeys = append(meta.EffectKeys, key)
		size, err := readUint32From(reader)
		if err != nil {
			meta.Malformed = true
			return err
		}
		if int(size) > reader.Len() {
			meta.Malformed = true
			return fmt.Errorf("legacy effect %q data truncated", key)
		}
		if _, err := readBytesFrom(reader, int(size)); err != nil {
			meta.Malformed = true
			return err
		}
	}
	return nil
}

func (meta *psdLayerEffectsMeta) GetLegacyStyleStack() []LayerStyle {
	if meta == nil || meta.Legacy == nil {
		return nil
	}
	return mapLegacyEffectKeysToLayerStyles(meta.Legacy.EffectKeys)
}

func (meta *psdLayerEffectsMeta) GetStyleStack() []LayerStyle {
	if meta == nil {
		return nil
	}
	styles := make([]LayerStyle, 0)
	seen := make(map[LayerStyleKind]struct{})
	appendUniqueStyles := func(layerStyles []LayerStyle) {
		for _, style := range layerStyles {
			kind := LayerStyleKind(style.Kind)
			if _, ok := seen[kind]; ok {
				continue
			}
			if kind == "" {
				continue
			}
			styles = append(styles, style)
			seen[kind] = struct{}{}
		}
	}
	appendUniqueStyles(meta.GetLegacyStyleStack())
	if meta.Object != nil {
		appendUniqueStyles(mapObjectEffectKeysToLayerStyles(meta.Object.EffectKeys))
	}
	return styles
}

func mapLegacyEffectKeysToLayerStyles(keys []string) []LayerStyle {
	if len(keys) == 0 {
		return nil
	}
	styles := make([]LayerStyle, 0, len(keys))
	for _, key := range keys {
		kind, ok := legacyEffectStyleKind(key)
		if !ok {
			continue
		}
		styles = append(styles, LayerStyle{
			Kind:    string(kind),
			Enabled: false,
		})
	}
	return styles
}

func mapObjectEffectKeysToLayerStyles(keys []string) []LayerStyle {
	if len(keys) == 0 {
		return nil
	}
	styles := make([]LayerStyle, 0, len(keys))
	for _, key := range keys {
		kind, ok := objectEffectStyleKind(key)
		if !ok {
			continue
		}
		styles = append(styles, LayerStyle{
			Kind:    string(kind),
			Enabled: false,
		})
	}
	return styles
}

func legacyEffectStyleKind(key string) (LayerStyleKind, bool) {
	switch key {
	case "drSh":
		return LayerStyleKindDropShadow, true
	case "dsSh":
		return LayerStyleKindInnerShadow, true
	case "eglw":
		return LayerStyleKindOuterGlow, true
	case "iglw":
		return LayerStyleKindInnerGlow, true
	case "ebbl":
		return LayerStyleKindBevelEmboss, true
	default:
		return "", false
	}
}

func objectEffectStyleKind(key string) (LayerStyleKind, bool) {
	switch key {
	case "drsh", "dropshadow":
		return LayerStyleKindDropShadow, true
	case "dssh", "innershadow":
		return LayerStyleKindInnerShadow, true
	case "eglw", "outerglow":
		return LayerStyleKindOuterGlow, true
	case "iglw", "innerglow":
		return LayerStyleKindInnerGlow, true
	case "ebbl", "bevelemboss":
		return LayerStyleKindBevelEmboss, true
	case "stroke", "strokestyle":
		return LayerStyleKindStroke, true
	case "coloroverlay":
		return LayerStyleKindColorOverlay, true
	case "gradientoverlay":
		return LayerStyleKindGradientOverlay, true
	case "patternoverlay":
		return LayerStyleKindPatternOverlay, true
	case "satin":
		return LayerStyleKindSatin, true
	default:
		return "", false
	}
}

func scanObjectEffectStyleKeys(payload []byte) []string {
	if len(payload) == 0 {
		return nil
	}
	normalized := strings.ToLower(string(payload))
	var keys []string
	for _, pattern := range []string{
		"drsh",
		"dssh",
		"eglw",
		"iglw",
		"ebbl",
		"dropshadow",
		"innershadow",
		"outerglow",
		"innerglow",
		"bevelemboss",
		"strokestyle",
		"coloroverlay",
		"gradientoverlay",
		"patternoverlay",
		"satin",
	} {
		if strings.Contains(normalized, pattern) {
			keys = append(keys, pattern)
		}
	}
	return keys
}

func parsePSDLayerAdjustmentMetadata(key string, payload []byte, record *psdLayerRecord) error {
	adjustment := psdAdjustmentMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
	switch key {
	case "levl":
		adjustment.Kind = "levels"
	case "curv":
		adjustment.Kind = "curves"
	case "hue2":
		adjustment.Kind = "hue-saturation"
	default:
		adjustment.Kind = strings.ToLower(key)
	}
	if len(payload) >= 2 {
		version, err := readUint16From(bytes.NewReader(payload))
		if err == nil {
			adjustment.Version = version
			adjustment.HasVersion = true
		}
	}
	record.Adjustments = append(record.Adjustments, adjustment)
	return nil
}

func parsePSDLayerSmartObjectMetadata(key string, payload []byte, record *psdLayerRecord) error {
	meta := &psdSmartObjectMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
	record.SmartObject = meta
	reader := bytes.NewReader(payload)
	version, err := readUint32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	meta.HasVersion = true
	meta.Version = version

	if reader.Len() == 0 {
		return nil
	}
	identifierLen, err := readUint8From(reader)
	if err != nil {
		meta.Malformed = true
		return fmt.Errorf("smart object identifier length missing")
	}
	if int(identifierLen) > reader.Len() {
		meta.Malformed = true
		return fmt.Errorf("smart object identifier length exceeds payload")
	}
	identifierBytes, err := readBytesFrom(reader, int(identifierLen))
	if err != nil {
		meta.Malformed = true
		return fmt.Errorf("smart object identifier missing")
	}
	meta.Identifier = string(identifierBytes)
	if key == "PlLd" || key == "plLd" {
		meta.HasDescriptor = true
	}
	if key == "SoLd" {
		if reader.Len() >= 4 {
			pageNumber, err := readUint32From(reader)
			if err == nil {
				meta.PageNumber = &pageNumber
			}
		}
		if reader.Len() >= 4 {
			totalPages, err := readUint32From(reader)
			if err == nil {
				meta.TotalPages = &totalPages
			}
		}
		if reader.Len() >= 4 {
			placedType, err := readUint32From(reader)
			if err == nil {
				meta.PlacedType = &placedType
			}
		}
	}
	if reader.Len() > 0 {
		meta.UniqueID = meta.Identifier
	}
	return nil
}

func parsePSDLayerVectorMaskMetadata(key string, payload []byte, record *psdLayerRecord) error {
	meta := &psdVectorMaskMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
	record.HasVectorMask = true
	record.VectorMask = meta
	reader := bytes.NewReader(payload)
	if len(payload) < 20 {
		meta.Malformed = true
		return nil
	}
	top, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	left, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	bottom, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	right, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	width := int(right - left)
	height := int(bottom - top)
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	meta.Bounds = LayerBounds{
		X: int(left),
		Y: int(top),
		W: width,
		H: height,
	}
	meta.HasBounds = true
	meta.DefaultColor = uint16(payload[16])<<8 | uint16(payload[17])
	meta.Flags = uint16(payload[18])<<8 | uint16(payload[19])
	return nil
}

func parsePSDTextLayerMetadata(key string, payload []byte, record *psdLayerRecord) error {
	meta := &psdTextLayerMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
	record.Text = meta
	if len(payload) == 0 {
		meta.Malformed = true
		return nil
	}
	textPayload := payload
	if len(payload) >= 4 {
		version, err := readUint32From(bytes.NewReader(payload))
		if err == nil {
			meta.DescriptorVersion = version
			meta.HasDescriptor = true
			textPayload = payload[4:]
		}
	}
	if text, err := parsePSDUnicodeString(textPayload); err == nil {
		meta.ParsedText = text
	}
	return nil
}

func readUint8From(r io.Reader) (uint8, error) {
	var value uint8
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
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

func parsePSDLayerColorTag(payload []byte) string {
	if len(payload) == 0 {
		return "none"
	}
	switch payload[0] {
	case 1:
		return "red"
	case 2:
		return "orange"
	case 3:
		return "yellow"
	case 4:
		return "green"
	case 5:
		return "blue"
	case 6:
		return "violet"
	case 7:
		return "gray"
	default:
		return fmt.Sprintf("unknown(%d)", payload[0])
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
