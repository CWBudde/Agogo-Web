package psd

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"strings"
	"testing"
)

func TestReadBytesFromRejectsDeclaredLengthBeforeAllocation(t *testing.T) {
	reader := bytes.NewReader([]byte{1, 2, 3})
	if _, err := readBytesFrom(reader, 1<<30); err == nil || !strings.Contains(err.Error(), "exceeds remaining input") {
		t.Fatalf("readBytesFrom error = %v, want remaining-input error", err)
	}
	if reader.Len() != 3 {
		t.Fatalf("reader consumed %d bytes after rejected length", 3-reader.Len())
	}
}

func TestParseHeaderRejectsUnsafeDimensionsAndCounts(t *testing.T) {
	tests := []struct {
		name     string
		version  uint16
		channels uint16
		width    uint32
		height   uint32
		want     string
	}{
		{name: "zero width", version: 1, channels: 3, width: 0, height: 1, want: "invalid PSD dimensions"},
		{name: "PSD dimension limit", version: 1, channels: 3, width: PSDMaxDimension + 1, height: 1, want: "maximum 30000"},
		{name: "PSB dimension limit", version: 2, channels: 3, width: PSBMaxDimension + 1, height: 1, want: "maximum 300000"},
		{name: "channel limit", version: 1, channels: PSDMaxChannels + 1, width: 1, height: 1, want: "channel count"},
		{name: "decoded size limit", version: 2, channels: 4, width: 20000, height: 20000, want: "safety limit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewParser(testPSDHeader(tc.version, tc.channels, tc.width, tc.height, 8))
			if _, err := parser.ParseHeader(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseHeader error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDecodeZipPayloadStopsAtExpectedOutput(t *testing.T) {
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(make([]byte, 1<<20)); err != nil {
		t.Fatalf("zlib write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zlib close: %v", err)
	}
	if _, err := decodeZipPayload(compressed.Bytes(), 4); err == nil || !strings.Contains(err.Error(), "exceeds expected length") {
		t.Fatalf("decodeZipPayload error = %v, want bounded-output error", err)
	}
}

func TestParseRejectsEveryTruncationOfValidMinimalPSD(t *testing.T) {
	valid := validMinimalPSD()
	if _, err := Parse(valid); err != nil {
		t.Fatalf("valid seed Parse: %v", err)
	}
	for offset := 0; offset < len(valid); offset++ {
		if _, err := Parse(valid[:offset]); err == nil {
			t.Fatalf("Parse unexpectedly accepted truncation at offset %d/%d", offset, len(valid))
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add(validMinimalPSD())
	f.Add([]byte("8BPS"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Parse(data)
	})
}

func FuzzDecodePackBits(f *testing.F) {
	f.Add([]byte{0, 'A'}, uint16(1))
	f.Add([]byte{0x80}, uint16(0))
	f.Fuzz(func(t *testing.T, data []byte, expected uint16) {
		_, _ = DecodePackBits(data, int(expected%4096))
	})
}

func FuzzParseLayerExtraData(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		record := LayerRecord{}
		_ = ParseLayerExtraData(data, &record)
	})
}

func FuzzParseLayerAndMaskInfo(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		parser := NewParser(data)
		_, _ = parser.ParseLayerAndMaskInfo(Header{
			Version: 1, Channels: 3, Width: 1, Height: 1, Depth: 8, ColorMode: ColorModeRGB,
		})
	})
}

func FuzzParseDescriptorTextValue(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = ParseDescriptorTextValue(data, map[string]struct{}{"Txt ": {}})
	})
}

func FuzzParseCompositeImageData(f *testing.F) {
	f.Add([]byte{0, CompressionRaw, 1, 2, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		parser := NewParser(data)
		_, _ = parser.ParseCompositeImageData(Header{
			Version: 1, Channels: 3, Width: 1, Height: 1, Depth: 8, ColorMode: ColorModeRGB,
		})
	})
}

func validMinimalPSD() []byte {
	data := testPSDHeader(1, 3, 1, 1, 8)
	data = append(
		data,
		0, 0, 0, 0, // color-mode data length
		0, 0, 0, 0, // image-resources length
		0, 0, 0, 0, // layer-and-mask length
		0, CompressionRaw, // composite compression
		1, 2, 3, // RGB planes
	)
	return data
}

func testPSDHeader(version, channels uint16, width, height uint32, depth uint16) []byte {
	var out bytes.Buffer
	out.WriteString("8BPS")
	_ = binary.Write(&out, binary.BigEndian, version)
	out.Write(make([]byte, 6))
	_ = binary.Write(&out, binary.BigEndian, channels)
	_ = binary.Write(&out, binary.BigEndian, height)
	_ = binary.Write(&out, binary.BigEndian, width)
	_ = binary.Write(&out, binary.BigEndian, depth)
	_ = binary.Write(&out, binary.BigEndian, uint16(ColorModeRGB))
	return out.Bytes()
}
