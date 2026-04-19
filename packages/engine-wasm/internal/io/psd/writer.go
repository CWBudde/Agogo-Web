package psd

import (
	"bytes"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func Write(params WriteParams) ([]byte, error) {
	var out bytes.Buffer

	writeString(&out, "8BPS")
	if params.PSB {
		writeUint16(&out, 2)
	} else {
		writeUint16(&out, 1)
	}
	out.Write(make([]byte, 6))

	writeUint16(&out, uint16(params.ChannelCount))
	writeUint32(&out, uint32(params.Height))
	writeUint32(&out, uint32(params.Width))
	writeUint16(&out, uint16(params.Depth))
	writeUint16(&out, uint16(params.ColorMode))

	writeUint32(&out, 0)

	var imageResources bytes.Buffer
	for _, resource := range params.ImageResources {
		WriteImageResource(&imageResources, resource.ID, resource.Name, resource.Payload)
	}
	writeUint32(&out, uint32(imageResources.Len()))
	out.Write(imageResources.Bytes())

	layerAndMaskInfo := buildLayerAndMaskInfo(params.PSB, params.Layers)
	writeSectionLength(&out, params.PSB, uint64(len(layerAndMaskInfo)))
	out.Write(layerAndMaskInfo)

	out.Write(params.CompositeData)
	return out.Bytes(), nil
}

func buildLayerAndMaskInfo(psb bool, records []ExportLayerRecord) []byte {
	var layerInfo bytes.Buffer
	writeInt16(&layerInfo, int16(len(records)))
	for _, record := range records {
		writeLayerRecord(&layerInfo, psb, record)
	}
	for _, record := range records {
		for _, channel := range record.Channels {
			layerInfo.Write(channel.Payload)
		}
	}

	var out bytes.Buffer
	writeSectionLength(&out, psb, uint64(layerInfo.Len()))
	out.Write(layerInfo.Bytes())
	return out.Bytes()
}

func writeLayerRecord(out *bytes.Buffer, psb bool, record ExportLayerRecord) {
	writeInt32(out, int32(record.Bounds.Y))
	writeInt32(out, int32(record.Bounds.X))
	writeInt32(out, int32(record.Bounds.Y+record.Bounds.H))
	writeInt32(out, int32(record.Bounds.X+record.Bounds.W))
	writeUint16(out, uint16(len(record.Channels)))
	for _, channel := range record.Channels {
		writeInt16(out, channel.ID)
		writeSectionLength(out, psb, channel.Length)
	}
	writeString(out, "8BIM")
	writeString(out, record.BlendKey)
	out.WriteByte(record.Opacity)
	if record.ClipToBelow {
		out.WriteByte(1)
	} else {
		out.WriteByte(0)
	}
	flags := byte(0)
	if !record.Visible {
		flags |= 0x02
	}
	out.WriteByte(flags)
	out.WriteByte(0)

	var extra bytes.Buffer
	writeLayerMaskData(&extra, record.Mask)
	writeUint32(&extra, 0)
	writePascalString4(&extra, record.Name)
	writeUnicodeLayerNameBlock(&extra, record.Name)
	if record.SectionType != 0 {
		WriteAdditionalLayerInfoBlock(&extra, "8BIM", "lsct", buildSectionDivider(record.SectionType))
	}
	for _, block := range record.ExtraBlocks {
		WriteAdditionalLayerInfoBlock(&extra, block.Signature, block.Key, block.Payload)
	}
	writeUint32(out, uint32(extra.Len()))
	out.Write(extra.Bytes())
}

func writeLayerMaskData(out *bytes.Buffer, mask *model.LayerMask) {
	if mask == nil || mask.Width <= 0 || mask.Height <= 0 || len(mask.Data) != mask.Width*mask.Height {
		writeUint32(out, 0)
		return
	}
	var payload bytes.Buffer
	writeInt32(&payload, 0)
	writeInt32(&payload, 0)
	writeInt32(&payload, int32(mask.Height))
	writeInt32(&payload, int32(mask.Width))
	writeUint16(&payload, 0)
	flags := uint16(0)
	if !mask.Enabled {
		flags = 1
	}
	writeUint16(&payload, flags)
	writeUint32(out, uint32(payload.Len()))
	out.Write(payload.Bytes())
}

func writeUnicodeLayerNameBlock(out *bytes.Buffer, name string) {
	if name == "" {
		return
	}
	WriteAdditionalLayerInfoBlock(out, "8BIM", "luni", EncodeUnicodeString(name))
}

func buildSectionDivider(sectionType uint32) []byte {
	var out bytes.Buffer
	writeUint32(&out, sectionType)
	return out.Bytes()
}
