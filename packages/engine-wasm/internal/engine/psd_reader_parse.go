package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

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
		case psdImageResourceAgogoProject:
			resources.AgogoProject = append([]byte(nil), payload...)
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
		case "AgAJ":
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
