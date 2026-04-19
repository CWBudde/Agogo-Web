package psd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func (p *Parser) readBytes(n int) ([]byte, error) {
	return readBytesFrom(p.r, n)
}

func (p *Parser) readString(n int) (string, error) {
	return readStringFrom(p.r, n)
}

func (p *Parser) readUint16() (uint16, error) {
	return readUint16From(p.r)
}

func (p *Parser) readUint32() (uint32, error) {
	return readUint32From(p.r)
}

func (p *Parser) readSectionLength(psb bool) (uint64, error) {
	return readSectionLengthFrom(p.r, psb)
}

func (p *Parser) ParseHeader() (Header, error) {
	signature, err := p.readString(4)
	if err != nil {
		return Header{}, err
	}
	if signature != "8BPS" {
		return Header{}, fmt.Errorf("invalid PSD signature %q", signature)
	}
	version, err := p.readUint16()
	if err != nil {
		return Header{}, err
	}
	if version != 1 && version != 2 {
		return Header{}, fmt.Errorf("unsupported PSD version %d", version)
	}
	reserved, err := p.readBytes(6)
	if err != nil {
		return Header{}, err
	}
	for _, b := range reserved {
		if b != 0 {
			return Header{}, fmt.Errorf("invalid PSD reserved bytes")
		}
	}
	channels, err := p.readUint16()
	if err != nil {
		return Header{}, err
	}
	height, err := p.readUint32()
	if err != nil {
		return Header{}, err
	}
	width, err := p.readUint32()
	if err != nil {
		return Header{}, err
	}
	depth, err := p.readUint16()
	if err != nil {
		return Header{}, err
	}
	colorMode, err := p.readUint16()
	if err != nil {
		return Header{}, err
	}
	return Header{
		Version:   version,
		PSB:       version == 2,
		Channels:  int(channels),
		Height:    int(height),
		Width:     int(width),
		Depth:     int(depth),
		ColorMode: int(colorMode),
	}, nil
}

func (p *Parser) SkipColorModeData() error {
	length, err := p.readUint32()
	if err != nil {
		return err
	}
	_, err = p.readBytes(int(length))
	return err
}

func (p *Parser) ParseImageResources() (ImageResources, error) {
	length, err := p.readUint32()
	if err != nil {
		return ImageResources{}, err
	}
	data, err := p.readBytes(int(length))
	if err != nil {
		return ImageResources{}, err
	}
	reader := bytes.NewReader(data)
	resources := ImageResources{}
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
		case ImageResourceDPI:
			if len(payload) >= 4 {
				fixed := binary.BigEndian.Uint32(payload[:4])
				resources.Resolution = float64(fixed) / 65536.0
			}
		case ImageResourceICCProfile:
			resources.HasICCProfile = true
		case ImageResourceGuides:
			resources.HasGuides = true
		case ImageResourceSlices:
			resources.HasSlices = true
		case ImageResourceLayerComps:
			resources.HasLayerComps = true
		case ImageResourceAgogoProject:
			resources.AgogoProject = append([]byte(nil), payload...)
		}
	}
	return resources, nil
}

func (p *Parser) ParseLayerAndMaskInfo(header Header) ([]LayerRecord, error) {
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
	layers := make([]LayerRecord, 0, layerCount)
	for i := 0; i < layerCount; i++ {
		record, err := parseLayerRecord(layerReader, header.PSB)
		if err != nil {
			p.warnf("failed parsing layer %d: %v", i+1, err)
			return layers, err
		}
		layers = append(layers, record)
	}
	for i := range layers {
		channelPixels := make(map[int16][]byte, len(layers[i].Channels))
		for _, channel := range layers[i].Channels {
			pixels, err := parseChannelImageData(layerReader, header.PSB, channel.Length, layers[i].Bounds.W, layers[i].Bounds.H)
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

func parseLayerRecord(reader *bytes.Reader, psb bool) (LayerRecord, error) {
	top, err := readInt32From(reader)
	if err != nil {
		return LayerRecord{}, err
	}
	left, err := readInt32From(reader)
	if err != nil {
		return LayerRecord{}, err
	}
	bottom, err := readInt32From(reader)
	if err != nil {
		return LayerRecord{}, err
	}
	right, err := readInt32From(reader)
	if err != nil {
		return LayerRecord{}, err
	}
	channelCount, err := readUint16From(reader)
	if err != nil {
		return LayerRecord{}, err
	}
	record := LayerRecord{
		Bounds: model.LayerBounds{
			X: int(left),
			Y: int(top),
			W: int(right - left),
			H: int(bottom - top),
		},
		Opacity:   1,
		Visible:   true,
		BlendMode: model.BlendModeNormal,
		Channels:  make([]ChannelInfo, 0, int(channelCount)),
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
		record.Channels = append(record.Channels, ChannelInfo{ID: id, Length: length})
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
	record.BlendMode = MapBlendMode(blendKey)
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
	record.Visible = flags&0x02 == 0
	if _, err := reader.ReadByte(); err != nil {
		return record, err
	}
	extraLen, err := readUint32From(reader)
	if err != nil {
		return record, err
	}
	extraData, err := readBytesFrom(reader, int(extraLen))
	if err != nil {
		return record, err
	}
	if err := parseLayerRecordExtra(bytes.NewReader(extraData), &record, psb); err != nil {
		return record, err
	}
	return record, nil
}

func parseLayerRecordExtra(reader *bytes.Reader, record *LayerRecord, psb bool) error {
	maskLen, err := readUint32From(reader)
	if err != nil {
		return err
	}
	if maskLen > 0 {
		maskData, err := readBytesFrom(reader, int(maskLen))
		if err != nil {
			return err
		}
		parseLayerMaskData(maskData, record)
	}
	blendingRangesLen, err := readUint32From(reader)
	if err != nil {
		return err
	}
	if blendingRangesLen > 0 {
		if _, err := readBytesFrom(reader, int(blendingRangesLen)); err != nil {
			return err
		}
	}
	name, err := readPascalString4(reader)
	if err != nil {
		return err
	}
	record.Name = strings.TrimRight(name, "\x00")
	for reader.Len() > 0 {
		signature, err := readStringFrom(reader, 4)
		if err != nil {
			return err
		}
		if signature != "8BIM" && signature != "8B64" {
			return fmt.Errorf("invalid additional layer info signature %q", signature)
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
		if err := parseLayerAdditionalInfo(signature, key, payload, record); err != nil {
			record.MetadataWarnings = append(record.MetadataWarnings, fmt.Sprintf("metadata %s: %v", key, err))
		}
	}
	return nil
}

func ParseLayerExtraData(data []byte, record *LayerRecord) error {
	return parseLayerRecordExtra(bytes.NewReader(data), record, false)
}

func parseLayerMaskData(payload []byte, record *LayerRecord) {
	if len(payload) < 18 {
		return
	}
	reader := bytes.NewReader(payload)
	top, err := readInt32From(reader)
	if err != nil {
		return
	}
	left, err := readInt32From(reader)
	if err != nil {
		return
	}
	bottom, err := readInt32From(reader)
	if err != nil {
		return
	}
	right, err := readInt32From(reader)
	if err != nil {
		return
	}
	if _, err := readUint16From(reader); err != nil {
		return
	}
	flags, err := readUint16From(reader)
	if err != nil {
		return
	}
	record.HasLayerMask = true
	record.LayerMaskEnabled = flags&0x0001 == 0
	record.LayerMaskBounds = model.LayerBounds{
		X: int(left),
		Y: int(top),
		W: int(right - left),
		H: int(bottom - top),
	}
}

func parseLayerAdditionalInfo(_ string, key string, payload []byte, record *LayerRecord) error {
	switch key {
	case "luni":
		name, err := ParseUnicodeString(payload)
		if err == nil && name != "" {
			record.Name = name
		}
		return err
	case "lyid":
		if len(payload) >= 4 {
			record.LayerID = binary.BigEndian.Uint32(payload[:4])
		}
		return nil
	case "lclr":
		record.LayerColorTag = ParseLayerColorTag(payload)
		return nil
	case "lsct":
		if len(payload) >= 4 {
			record.SectionType = binary.BigEndian.Uint32(payload[:4])
		}
		return nil
	case "lrFX":
		return parseLayerLegacyEffectsPayload(payload, record)
	case "lfx2":
		return parseLayerObjectEffectsPayload(payload, record)
	case "TySh":
		return parseTextLayerMetadata(key, payload, record)
	case "levl", "curv", "hue2", "AgAJ":
		return parseLayerAdjustmentMetadata(key, payload, record)
	case "SoLd", "PlLd", "plLd":
		return parseLayerSmartObjectMetadata(key, payload, record)
	case "vmsk", "vsms":
		return parseLayerVectorMaskMetadata(key, payload, record)
	default:
		record.UnsupportedBlocks = append(record.UnsupportedBlocks, key)
		return fmt.Errorf("unsupported metadata block")
	}
}
