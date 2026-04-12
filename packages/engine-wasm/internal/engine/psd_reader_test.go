package engine

import (
	"compress/zlib"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func TestLoadPSDImportsCompositeImageAndResolution(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:      1,
		height:     1,
		channels:   3,
		resolution: 300,
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{255},
				{0},
				{0},
			},
		},
	})

	doc, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if doc.Width != 1 || doc.Height != 1 {
		t.Fatalf("doc size = %dx%d, want 1x1", doc.Width, doc.Height)
	}
	if doc.ColorMode != "rgb" {
		t.Fatalf("doc color mode = %q, want rgb", doc.ColorMode)
	}
	if doc.BitDepth != 8 {
		t.Fatalf("doc bit depth = %d, want 8", doc.BitDepth)
	}
	if doc.Resolution != 300 {
		t.Fatalf("doc resolution = %v, want 300", doc.Resolution)
	}

	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	layer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if layer.Name() != "Background" {
		t.Fatalf("layer name = %q, want Background", layer.Name())
	}
	if !bytes.Equal(layer.Pixels, []byte{255, 0, 0, 255}) {
		t.Fatalf("layer pixels = %v, want red RGBA pixel", layer.Pixels)
	}
}

func TestLoadPSDImportsLayerRasterDataAndWarnsForUnsupportedTextMetadata(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Pixel",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRLE, data: []byte{10}},
					{id: 1, compression: psdCompressionRLE, data: []byte{20}},
					{id: 2, compression: psdCompressionRLE, data: []byte{30}},
					{id: -1, compression: psdCompressionRLE, data: []byte{255}},
				},
			},
			{
				name: "Title",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				extraBlocks: []psdTaggedBlock{
					{signature: "8BIM", key: "TySh", data: buildTypeToolInfoBlock()},
				},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{200}},
					{id: 1, compression: psdCompressionRaw, data: []byte{150}},
					{id: 2, compression: psdCompressionRaw, data: []byte{100}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{200},
				{150},
				{100},
			},
		},
	})

	doc, warnings, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one warning", warnings)
	}
	if !strings.Contains(warnings[0], "TySh") {
		t.Fatalf("warning = %q, want TySh context", warnings[0])
	}

	children := doc.LayerRoot.Children()
	if len(children) != 2 {
		t.Fatalf("imported child count = %d, want 2", len(children))
	}

	first, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("first layer = %T, want *PixelLayer", children[0])
	}
	if first.Name() != "Pixel" {
		t.Fatalf("first layer name = %q, want Pixel", first.Name())
	}
	if !bytes.Equal(first.Pixels, []byte{10, 20, 30, 255}) {
		t.Fatalf("first layer pixels = %v, want [10 20 30 255]", first.Pixels)
	}

	second, ok := children[1].(*PixelLayer)
	if !ok {
		t.Fatalf("second layer = %T, want *PixelLayer fallback", children[1])
	}
	if second.Name() != "Title" {
		t.Fatalf("second layer name = %q, want Title", second.Name())
	}
	if !bytes.Equal(second.Pixels, []byte{200, 150, 100, 255}) {
		t.Fatalf("second layer pixels = %v, want [200 150 100 255]", second.Pixels)
	}
}

func TestImportProjectAcceptsBase64PSD(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{0},
				{255},
				{0},
			},
		},
	})

	h := Init("")
	defer Free(h)

	result, err := ImportProject(h, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatalf("ImportProject: %v", err)
	}
	if result.UIMeta.ActiveDocumentName == "" {
		t.Fatal("active document name should not be empty after PSD import")
	}
	if result.UIMeta.DocumentWidth != 1 || result.UIMeta.DocumentHeight != 1 {
		t.Fatalf("imported dimensions = %dx%d, want 1x1", result.UIMeta.DocumentWidth, result.UIMeta.DocumentHeight)
	}

	doc := instances[h].manager.Active()
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	layer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if !bytes.Equal(layer.Pixels, []byte{0, 255, 0, 255}) {
		t.Fatalf("layer pixels = %v, want green RGBA pixel", layer.Pixels)
	}
}

func TestImportProjectSurfacesPSDWarnings(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    1,
		height:   1,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Title",
				rect: LayerBounds{X: 0, Y: 0, W: 1, H: 1},
				extraBlocks: []psdTaggedBlock{
					{signature: "8BIM", key: "TySh", data: buildTypeToolInfoBlock()},
				},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionRaw, data: []byte{40}},
					{id: 1, compression: psdCompressionRaw, data: []byte{50}},
					{id: 2, compression: psdCompressionRaw, data: []byte{60}},
					{id: -1, compression: psdCompressionRaw, data: []byte{255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{40},
				{50},
				{60},
			},
		},
	})

	h := Init("")
	defer Free(h)

	result, err := ImportProject(h, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatalf("ImportProject: %v", err)
	}
	if len(result.UIMeta.ImportWarnings) != 1 {
		t.Fatalf("import warnings = %v, want single warning", result.UIMeta.ImportWarnings)
	}
	if !strings.Contains(result.UIMeta.ImportWarnings[0], "TySh") {
		t.Fatalf("warning = %q, want TySh context", result.UIMeta.ImportWarnings[0])
	}
}

func TestLoadPSDImportsZipPredictionLayerData(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    2,
		height:   2,
		channels: 3,
		layers: []minimalPSDLayer{
			{
				name: "Predicted",
				rect: LayerBounds{X: 0, Y: 0, W: 2, H: 2},
				channels: []psdLayerChannel{
					{id: 0, compression: psdCompressionZipPrediction, data: []byte{20, 10, 30, 40}},
					{id: 1, compression: psdCompressionZipPrediction, data: []byte{5, 8, 12, 16}},
					{id: 2, compression: psdCompressionZipPrediction, data: []byte{60, 70, 80, 90}},
					{id: -1, compression: psdCompressionZipPrediction, data: []byte{255, 255, 255, 255}},
				},
			},
		},
		composite: psdImageData{
			compression: psdCompressionRaw,
			planes: [][]byte{
				{20, 10, 30, 40},
				{5, 8, 12, 16},
				{60, 70, 80, 90},
			},
		},
	})

	doc, _, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	if doc.Width != 2 || doc.Height != 2 {
		t.Fatalf("doc size = %dx%d, want 2x2", doc.Width, doc.Height)
	}
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	pixelLayer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if !bytes.Equal(pixelLayer.Pixels, []byte{
		20, 5, 60, 255,
		10, 8, 70, 255,
		30, 12, 80, 255,
		40, 16, 90, 255,
	}) {
		t.Fatalf("unexpected pixels: %v", pixelLayer.Pixels)
	}
}

func TestLoadPSDImportsZipCompositeData(t *testing.T) {
	data := buildMinimalPSD(t, minimalPSDConfig{
		width:    2,
		height:   2,
		channels: 3,
		composite: psdImageData{
			compression: psdCompressionZip,
			planes: [][]byte{
				{20, 5, 30, 40},
				{10, 8, 12, 16},
				{60, 70, 80, 90},
			},
		},
	})
	doc, _, err := LoadPSD(data)
	if err != nil {
		t.Fatalf("LoadPSD: %v", err)
	}
	children := doc.LayerRoot.Children()
	if len(children) != 1 {
		t.Fatalf("imported child count = %d, want 1", len(children))
	}
	pixelLayer, ok := children[0].(*PixelLayer)
	if !ok {
		t.Fatalf("imported layer = %T, want *PixelLayer", children[0])
	}
	if !bytes.Equal(pixelLayer.Pixels, []byte{
		20, 10, 60, 255,
		5, 8, 70, 255,
		30, 12, 80, 255,
		40, 16, 90, 255,
	}) {
		t.Fatalf("unexpected pixels: %v", pixelLayer.Pixels)
	}
}

type minimalPSDConfig struct {
	width      int
	height     int
	channels   int
	resolution int
	layers     []minimalPSDLayer
	composite  psdImageData
}

type minimalPSDLayer struct {
	name        string
	rect        LayerBounds
	channels    []psdLayerChannel
	extraBlocks []psdTaggedBlock
}

type psdLayerChannel struct {
	id          int16
	compression uint16
	data        []byte
}

type psdImageData struct {
	compression uint16
	planes      [][]byte
}

type psdTaggedBlock struct {
	signature string
	key       string
	data      []byte
}

func buildMinimalPSD(t *testing.T, cfg minimalPSDConfig) []byte {
	t.Helper()

	if cfg.width <= 0 || cfg.height <= 0 {
		t.Fatalf("invalid PSD dimensions: %dx%d", cfg.width, cfg.height)
	}
	if cfg.channels <= 0 {
		t.Fatal("PSD must have at least one channel")
	}

	var out bytes.Buffer
	out.WriteString("8BPS")
	writeUint16(&out, 1)
	out.Write(make([]byte, 6))
	writeUint16(&out, uint16(cfg.channels))
	writeUint32(&out, uint32(cfg.height))
	writeUint32(&out, uint32(cfg.width))
	writeUint16(&out, 8)
	writeUint16(&out, 3)

	writeUint32(&out, 0)

	var resources bytes.Buffer
	if cfg.resolution > 0 {
		writeImageResourceBlock(&resources, 0x03ed, nil, buildResolutionInfoBlock(cfg.resolution))
	}
	writeUint32(&out, uint32(resources.Len()))
	out.Write(resources.Bytes())

	layerMask := buildLayerAndMaskSection(t, cfg.layers)
	writeUint32(&out, uint32(len(layerMask)))
	out.Write(layerMask)

	out.Write(buildImageDataSection(t, cfg.composite, cfg.width, cfg.height))
	return out.Bytes()
}

func buildLayerAndMaskSection(t *testing.T, layers []minimalPSDLayer) []byte {
	t.Helper()

	var layerRecords bytes.Buffer
	var channelData bytes.Buffer
	writeInt16(&layerRecords, int16(len(layers)))
	for _, layer := range layers {
		top := layer.rect.Y
		left := layer.rect.X
		bottom := layer.rect.Y + layer.rect.H
		right := layer.rect.X + layer.rect.W
		writeUint32(&layerRecords, uint32(top))
		writeUint32(&layerRecords, uint32(left))
		writeUint32(&layerRecords, uint32(bottom))
		writeUint32(&layerRecords, uint32(right))
		writeUint16(&layerRecords, uint16(len(layer.channels)))

		encodedChannels := make([][]byte, len(layer.channels))
		for i, channel := range layer.channels {
			writeInt16(&layerRecords, channel.id)
			encoded := buildChannelImageData(t, channel.compression, channel.data, layer.rect.W, layer.rect.H)
			encodedChannels[i] = encoded
			writeUint32(&layerRecords, uint32(len(encoded)))
		}

		layerRecords.WriteString("8BIM")
		layerRecords.WriteString("norm")
		layerRecords.WriteByte(255)
		layerRecords.WriteByte(0)
		layerRecords.WriteByte(0)
		layerRecords.WriteByte(0)

		var extra bytes.Buffer
		writeUint32(&extra, 0)
		writeUint32(&extra, 0)
		writePascalString4(&extra, layer.name)
		for _, block := range layer.extraBlocks {
			writeTaggedBlock(&extra, block)
		}
		writeUint32(&layerRecords, uint32(extra.Len()))
		layerRecords.Write(extra.Bytes())

		for _, encoded := range encodedChannels {
			channelData.Write(encoded)
		}
	}

	var layerInfo bytes.Buffer
	layerInfo.Write(layerRecords.Bytes())
	layerInfo.Write(channelData.Bytes())
	if layerInfo.Len()%2 != 0 {
		layerInfo.WriteByte(0)
	}

	var section bytes.Buffer
	writeUint32(&section, uint32(layerInfo.Len()))
	section.Write(layerInfo.Bytes())
	writeUint32(&section, 0)
	return section.Bytes()
}

func buildImageDataSection(t *testing.T, data psdImageData, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	writeUint16(&out, data.compression)
	switch data.compression {
	case psdCompressionRaw:
		for _, plane := range data.planes {
			out.Write(plane)
		}
	case psdCompressionRLE:
		for _, plane := range data.planes {
			encodedRows := encodeRLEByRow(t, plane, width, height)
			for _, row := range encodedRows {
				writeUint16(&out, uint16(len(row)))
			}
			for _, row := range encodedRows {
				out.Write(row)
			}
		}
	case psdCompressionZip, psdCompressionZipPrediction:
		composite := make([]byte, 0, width*height*len(data.planes))
		for _, plane := range data.planes {
			composite = append(composite, plane...)
		}
		if len(composite) != width*height*len(data.planes) {
			t.Fatalf("invalid composite plane data for zip: planes=%d width=%d height=%d", len(data.planes), width, height)
		}
		if data.compression == psdCompressionZipPrediction {
			for i := 0; i < len(data.planes); i++ {
				start := i * width * height
				end := start + width * height
				if end > len(composite) {
					t.Fatalf("invalid composite plane bounds start=%d end=%d len=%d", start, end, len(composite))
				}
				applyZipPredictionEncodeInPlace(composite[start:end], width, height)
			}
		}
		compressed, err := compressZipData(composite)
		if err != nil {
			t.Fatalf("compress zip: %v", err)
		}
		out.Write(compressed)
	default:
		t.Fatalf("unsupported compression %d in test fixture", data.compression)
	}
	return out.Bytes()
}

func buildChannelImageData(t *testing.T, compression uint16, data []byte, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	writeUint16(&out, compression)
	switch compression {
	case psdCompressionRaw:
		out.Write(data)
	case psdCompressionRLE:
		encodedRows := encodeRLEByRow(t, data, maxInt(len(data)/maxInt(height, 1), 1), height)
		for _, row := range encodedRows {
			writeUint16(&out, uint16(len(row)))
		}
		for _, row := range encodedRows {
			out.Write(row)
		}
	case psdCompressionZip, psdCompressionZipPrediction:
		payload := append([]byte(nil), data...)
		if compression == psdCompressionZipPrediction {
			applyZipPredictionEncodeInPlace(payload, width, height)
		}
		compressed, err := compressZipData(payload)
		if err != nil {
			t.Fatalf("compress zip channel: %v", err)
		}
		out.Write(compressed)
	default:
		t.Fatalf("unsupported compression %d in test fixture", compression)
	}
	return out.Bytes()
}

func applyZipPredictionEncodeInPlace(data []byte, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	for row := 0; row < height; row++ {
		rowStart := row * width
		rowEnd := rowStart + width
		if rowEnd > len(data) {
			return
		}
		for col := width - 1; col >= 1; col-- {
			data[rowStart+col] = data[rowStart+col] - data[rowStart+col-1]
		}
	}
}

func compressZipData(data []byte) ([]byte, error) {
	var out bytes.Buffer
	z := zlib.NewWriter(&out)
	if _, err := io.Copy(z, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encodeRLEByRow(t *testing.T, data []byte, width, height int) [][]byte {
	t.Helper()
	if height <= 0 {
		t.Fatal("height must be positive")
	}
	if len(data) != width*height {
		t.Fatalf("RLE source length = %d, want %d", len(data), width*height)
	}
	rows := make([][]byte, 0, height)
	for row := 0; row < height; row++ {
		start := row * width
		rows = append(rows, encodePackBitsLiteral(data[start:start+width]))
	}
	return rows
}

func encodePackBitsLiteral(data []byte) []byte {
	out := []byte{byte(len(data) - 1)}
	out = append(out, data...)
	return out
}

func buildResolutionInfoBlock(dpi int) []byte {
	var out bytes.Buffer
	writeUint32(&out, uint32(dpi<<16))
	writeUint16(&out, 1)
	writeUint16(&out, 1)
	writeUint32(&out, uint32(dpi<<16))
	writeUint16(&out, 1)
	writeUint16(&out, 1)
	return out.Bytes()
}

func buildTypeToolInfoBlock() []byte {
	var out bytes.Buffer
	writeUint16(&out, 1)
	for range 6 {
		writeFloat64(&out, 0)
	}
	writeUint16(&out, 50)
	writeUint32(&out, 16)
	writeUint32(&out, 0)
	writeUint32(&out, 16)
	writeUint32(&out, 0)
	return out.Bytes()
}

func writeImageResourceBlock(out *bytes.Buffer, id uint16, name []byte, data []byte) {
	out.WriteString("8BIM")
	writeUint16(out, id)
	if len(name) == 0 {
		out.Write([]byte{0, 0})
	} else {
		out.WriteByte(byte(len(name)))
		out.Write(name)
		if (1+len(name))%2 != 0 {
			out.WriteByte(0)
		}
	}
	writeUint32(out, uint32(len(data)))
	out.Write(data)
	if len(data)%2 != 0 {
		out.WriteByte(0)
	}
}

func writeTaggedBlock(out *bytes.Buffer, block psdTaggedBlock) {
	out.WriteString(block.signature)
	out.WriteString(block.key)
	writeUint32(out, uint32(len(block.data)))
	out.Write(block.data)
	if len(block.data)%2 != 0 {
		out.WriteByte(0)
	}
}

func writePascalString4(out *bytes.Buffer, value string) {
	if len(value) > 255 {
		value = value[:255]
	}
	out.WriteByte(byte(len(value)))
	out.WriteString(value)
	for out.Len()%4 != 0 {
		out.WriteByte(0)
	}
}

func writeUint16(out *bytes.Buffer, value uint16) {
	_ = binary.Write(out, binary.BigEndian, value)
}

func writeInt16(out *bytes.Buffer, value int16) {
	_ = binary.Write(out, binary.BigEndian, value)
}

func writeUint32(out *bytes.Buffer, value uint32) {
	_ = binary.Write(out, binary.BigEndian, value)
}

func writeFloat64(out *bytes.Buffer, value float64) {
	_ = binary.Write(out, binary.BigEndian, value)
}
