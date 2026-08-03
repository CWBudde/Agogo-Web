package psd

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
)

func TestDecodePackBitsHandWrittenScanline(t *testing.T) {
	// Literal ABC, a three-byte D run, a no-op, then literal E.
	encoded := []byte{2, 'A', 'B', 'C', 0xfe, 'D', 0x80, 0, 'E'}
	decoded, err := DecodePackBits(encoded, 7)
	if err != nil {
		t.Fatalf("DecodePackBits: %v", err)
	}
	if want := []byte("ABCDDDE"); !bytes.Equal(decoded, want) {
		t.Fatalf("decoded = %q, want %q", decoded, want)
	}
}

func TestEncodePackBitsPathologicalRows(t *testing.T) {
	alternating128 := make([]byte, 128)
	for i := range alternating128 {
		alternating128[i] = byte(i % 2)
	}
	run128 := bytes.Repeat([]byte{0x5a}, 128)
	run129 := bytes.Repeat([]byte{0x5a}, 129)

	tests := []struct {
		name       string
		input      []byte
		wantPrefix []byte
		wantLen    int
	}{
		{name: "empty", input: nil, wantLen: 0},
		{name: "single byte", input: []byte{0x7a}, wantPrefix: []byte{0, 0x7a}, wantLen: 2},
		{name: "alternating 128-byte literal", input: alternating128, wantPrefix: []byte{127}, wantLen: 129},
		{name: "128-byte run", input: run128, wantPrefix: []byte{129, 0x5a}, wantLen: 2},
		{name: "129-byte run", input: run129, wantPrefix: []byte{129, 0x5a, 0, 0x5a}, wantLen: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := EncodePackBitsRow(tc.input)
			if len(encoded) != tc.wantLen {
				t.Fatalf("encoded length = %d, want %d (%v)", len(encoded), tc.wantLen, encoded)
			}
			if !bytes.HasPrefix(encoded, tc.wantPrefix) {
				t.Fatalf("encoded prefix = %v, want %v", encoded, tc.wantPrefix)
			}
			decoded, err := DecodePackBits(encoded, len(tc.input))
			if err != nil {
				t.Fatalf("DecodePackBits(EncodePackBitsRow): %v", err)
			}
			if !bytes.Equal(decoded, tc.input) {
				t.Fatalf("round-trip mismatch: got %v, want %v", decoded, tc.input)
			}
		})
	}
}

func TestDecodePackBitsRejectsMalformedRows(t *testing.T) {
	tests := []struct {
		name        string
		encoded     []byte
		expectedLen int
	}{
		{name: "negative expected length", encoded: nil, expectedLen: -1},
		{name: "truncated literal", encoded: []byte{1, 0xaa}, expectedLen: 2},
		{name: "truncated repeat", encoded: []byte{0xff}, expectedLen: 2},
		{name: "literal exceeds row", encoded: []byte{1, 0xaa, 0xbb}, expectedLen: 1},
		{name: "repeat exceeds row", encoded: []byte{0x81, 0xaa}, expectedLen: 127},
		{name: "trailing packet exceeds row", encoded: []byte{0, 0xaa, 0, 0xbb}, expectedLen: 1},
		{name: "short decoded row", encoded: []byte{0, 0xaa}, expectedLen: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodePackBits(tc.encoded, tc.expectedLen); err == nil {
				t.Fatal("DecodePackBits unexpectedly succeeded")
			}
		})
	}
}

func TestZipPredictionResetsAtEveryRow(t *testing.T) {
	predicted := []byte{10, 10, 10, 100, 10, 10}
	channel := specZipCompositeData(CompressionZipPrediction, predicted)
	decoded, err := parseChannelImageData(bytes.NewReader(channel), false, uint64(len(channel)), 3, 2)
	if err != nil {
		t.Fatalf("parseChannelImageData: %v", err)
	}
	if want := []byte{10, 20, 30, 100, 110, 120}; !bytes.Equal(decoded, want) {
		t.Fatalf("decoded = %v, want %v", decoded, want)
	}
}

func TestZipCompositePredictionResetsForRowsAndPlanes(t *testing.T) {
	predicted := []byte{
		10, 10, 10, 100, 10, 10,
		1, 1, 1, 20, 1, 1,
	}
	data := specZipCompositeData(CompressionZipPrediction, predicted)
	parser := NewParser(data)
	rgba, err := parser.ParseCompositeImageData(Header{
		Channels: 2, Width: 3, Height: 2, Depth: 8, ColorMode: ColorModeGrayscale,
	})
	if err != nil {
		t.Fatalf("ParseCompositeImageData: %v", err)
	}
	want := []byte{
		10, 10, 10, 1,
		20, 20, 20, 2,
		30, 30, 30, 3,
		100, 100, 100, 20,
		110, 110, 110, 21,
		120, 120, 120, 22,
	}
	if !bytes.Equal(rgba, want) {
		t.Fatalf("rgba = %v, want %v", rgba, want)
	}
}

func TestRLEChannelRowCountsUsePSDAndPSBWidths(t *testing.T) {
	pixels := []byte{1, 2, 3, 4, 5, 6}
	for _, tc := range []struct {
		name       string
		psb        bool
		countWidth int
	}{
		{name: "PSD", psb: false, countWidth: 2},
		{name: "PSB", psb: true, countWidth: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeChannelData(pixels, 3, 2, tc.psb)
			if err != nil {
				t.Fatalf("EncodeChannelData: %v", err)
			}
			firstRowLen := readTestRowCount(t, encoded[2:], tc.countWidth)
			secondRowLen := readTestRowCount(t, encoded[2+tc.countWidth:], tc.countWidth)
			if firstRowLen != 4 || secondRowLen != 4 {
				t.Fatalf("row lengths = %d, %d; want 4, 4", firstRowLen, secondRowLen)
			}
			decoded, err := parseChannelImageData(bytes.NewReader(encoded), tc.psb, uint64(len(encoded)), 3, 2)
			if err != nil {
				t.Fatalf("parseChannelImageData: %v", err)
			}
			if !bytes.Equal(decoded, pixels) {
				t.Fatalf("decoded = %v, want %v", decoded, pixels)
			}
		})
	}
}

func TestRLECompositeWritesAllRowCountsBeforePixelData(t *testing.T) {
	planes := [][]byte{{1, 2, 3, 4}, {10, 20, 30, 40}}
	for _, tc := range []struct {
		name       string
		psb        bool
		countWidth int
	}{
		{name: "PSD", psb: false, countWidth: 2},
		{name: "PSB", psb: true, countWidth: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeCompositeImageData(planes, 2, 2, tc.psb)
			if err != nil {
				t.Fatalf("EncodeCompositeImageData: %v", err)
			}
			for row := 0; row < 4; row++ {
				if got := readTestRowCount(t, encoded[2+row*tc.countWidth:], tc.countWidth); got != 3 {
					t.Fatalf("row %d encoded length = %d, want 3", row, got)
				}
			}

			parser := NewParser(encoded)
			rgba, err := parser.ParseCompositeImageData(Header{
				PSB: tc.psb, Channels: 2, Width: 2, Height: 2, Depth: 8, ColorMode: ColorModeGrayscale,
			})
			if err != nil {
				t.Fatalf("ParseCompositeImageData: %v", err)
			}
			want := []byte{
				1, 1, 1, 10,
				2, 2, 2, 20,
				3, 3, 3, 30,
				4, 4, 4, 40,
			}
			if !bytes.Equal(rgba, want) {
				t.Fatalf("rgba = %v, want %v", rgba, want)
			}
		})
	}
}

func TestUnsupportedBitDepthFailsClearly(t *testing.T) {
	for _, depth := range []int{1, 16, 32} {
		t.Run(strconv.Itoa(depth), func(t *testing.T) {
			data := minimalPSDHeader(depth)
			result, err := Parse(data)
			if err == nil {
				t.Fatal("Parse unexpectedly succeeded")
			}
			if result.Header.Depth != depth {
				t.Fatalf("reported header depth = %d, want %d", result.Header.Depth, depth)
			}
			if !strings.Contains(err.Error(), "only 8-bit channels are currently supported") {
				t.Fatalf("error = %q, want actionable 8-bit support message", err)
			}
		})
	}

	parser := NewParser([]byte{0, CompressionRaw})
	if _, err := parser.ParseCompositeImageData(Header{Depth: 16}); err == nil || !strings.Contains(err.Error(), "bit depth 16") {
		t.Fatalf("ParseCompositeImageData error = %v, want unsupported bit-depth error", err)
	}
}

func TestEncodeCompositeImageDataRejectsWrongPlaneLength(t *testing.T) {
	if _, err := EncodeCompositeImageData([][]byte{{1, 2, 3}}, 2, 2, false); err == nil {
		t.Fatal("EncodeCompositeImageData unexpectedly accepted a short plane")
	}
}

func readTestRowCount(t *testing.T, data []byte, width int) int {
	t.Helper()
	if len(data) < width {
		t.Fatalf("row-count buffer has %d bytes, need %d", len(data), width)
	}
	if width == 2 {
		return int(binary.BigEndian.Uint16(data[:2]))
	}
	return int(binary.BigEndian.Uint32(data[:4]))
}

func minimalPSDHeader(depth int) []byte {
	var out bytes.Buffer
	out.WriteString("8BPS")
	writeUint16(&out, 1)
	out.Write(make([]byte, 6))
	writeUint16(&out, 3)
	writeUint32(&out, 1)
	writeUint32(&out, 1)
	writeUint16(&out, uint16(depth))
	writeUint16(&out, ColorModeRGB)
	return out.Bytes()
}
