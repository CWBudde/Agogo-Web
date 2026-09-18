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
	addWholeFileSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Parse(data)
	})
}

func FuzzDecodePackBits(f *testing.F) {
	f.Add([]byte{0, 'A'}, uint16(1))
	f.Add([]byte{0x80}, uint16(0))
	// Real scanlines from every RLE-compressed composite in the corpus, on top
	// of the synthetic edge cases above and the pathological-run coverage in
	// compression_test.go.
	addPackBitsRowSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte, expected uint16) {
		_, _ = DecodePackBits(data, int(expected%4096))
	})
}

func FuzzParseLayerExtraData(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	addSectionSeeds(f, extractFirstLayerExtraData)
	f.Fuzz(func(t *testing.T, data []byte) {
		record := LayerRecord{}
		_ = ParseLayerExtraData(data, &record)
	})
}

func FuzzParseLayerAndMaskInfo(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0})
	// The fixture sections belong to larger documents than the 1x1 header below.
	// That mismatch is deliberate: a section paired with the wrong header is
	// exactly the hostile shape this target exists to reject safely.
	addSectionSeeds(f, extractLayerAndMaskSection)
	f.Fuzz(func(t *testing.T, data []byte) {
		parser := NewParser(data)
		_, _ = parser.ParseLayerAndMaskInfo(Header{
			Version: 1, Channels: 3, Width: 1, Height: 1, Depth: 8, ColorMode: ColorModeRGB,
		})
	})
}

// FuzzParseTextLayerMetadata replaces the old FuzzParseDescriptorTextValue.
// The descriptor parser it used to target now lives in internal/io/descriptor
// and is fuzzed there; what is left here, and is the more useful surface, is
// the TySh envelope that wraps it - the transform, the two versions and the
// trailing bounds, none of which the descriptor package sees.
//
// Still seeded synthetically: a TySh payload only exists inside a text layer,
// and the text-layer fixture is deferred (PLAN.md S.10.7). Add a seed extractor
// when one lands.
func FuzzParseTextLayerMetadata(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{0, 1})
	f.Fuzz(func(t *testing.T, data []byte) {
		record := &LayerRecord{}
		_ = parseTextLayerMetadata("TySh", data, record)
		if record.Text == nil {
			t.Fatal("parseTextLayerMetadata must always record the block")
		}
	})
}

// FuzzParseAdjustmentBlock drives every adjustment reconstruction with the same
// bytes, so each parser sees payloads shaped for a different block. That is the
// point: the dispatch hands a parser whatever the file put under its key, and a
// levl parser fed a descriptor must refuse it rather than read 29 records out of
// six bytes.
//
// The invariant is only that nothing panics and that a refusal leaves no
// half-built adjustment behind — a parser that errors must not also have
// written to the record.
func FuzzParseAdjustmentBlock(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 2})
	f.Add([]byte{0, 0, 0, 16})
	addAdjustmentBlockSeeds(f)

	keys := []string{
		"levl", "curv", "hue2", "blnc", "mixr", "selc",
		"thrs", "post", "nvrt", "phfl", "blwh", "brit", "CgEd",
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, key := range keys {
			record := &LayerRecord{}
			if err := parseLayerAdjustmentBlock(key, data, record); err != nil {
				if record.Adjustment != nil {
					t.Fatalf("%s failed with %v but still set an adjustment: %+v",
						key, err, record.Adjustment)
				}
				continue
			}
			if record.Adjustment != nil && record.Adjustment.Kind == "" {
				t.Fatalf("%s produced an adjustment with no kind", key)
			}
		}
	})
}

func FuzzParseCompositeImageData(f *testing.F) {
	f.Add([]byte{0, CompressionRaw, 1, 2, 3})
	// As above: the seeded sections do not match the 1x1 header, on purpose.
	addSectionSeeds(f, extractCompositeSection)
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
