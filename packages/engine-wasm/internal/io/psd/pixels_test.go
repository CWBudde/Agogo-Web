package psd

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
)

func TestCompressionConstantsMatchPSDSpec(t *testing.T) {
	if CompressionRaw != 0 {
		t.Fatalf("CompressionRaw = %d, want 0", CompressionRaw)
	}
	if CompressionRLE != 1 {
		t.Fatalf("CompressionRLE = %d, want 1", CompressionRLE)
	}
	if CompressionZip != 2 {
		t.Fatalf("CompressionZip = %d, want 2", CompressionZip)
	}
	if CompressionZipPrediction != 3 {
		t.Fatalf("CompressionZipPrediction = %d, want 3", CompressionZipPrediction)
	}
}

func TestParseCompositeImageDataAcceptsSpecCompressionIDs(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "raw",
			data: appendUint16(nil, 0, 10, 20, 30),
		},
		{
			name: "rle",
			data: specRLECompositeData([]byte{10}, []byte{20}, []byte{30}),
		},
		{
			name: "zip",
			data: specZipCompositeData(2, []byte{10, 20, 30}),
		},
		{
			name: "zip prediction",
			data: specZipCompositeData(3, []byte{10, 20, 30}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewParser(tc.data)
			rgba, err := parser.ParseCompositeImageData(Header{
				Channels:  3,
				Width:     1,
				Height:    1,
				Depth:     8,
				ColorMode: ColorModeRGB,
			})
			if err != nil {
				t.Fatalf("ParseCompositeImageData: %v", err)
			}
			if !bytes.Equal(rgba, []byte{10, 20, 30, 255}) {
				t.Fatalf("rgba = %v, want [10 20 30 255]", rgba)
			}
		})
	}
}

func TestParseChannelImageDataDecodesZip(t *testing.T) {
	pixels := []byte{10, 20, 30, 40}
	channel := specZipCompositeData(CompressionZip, pixels)
	reader := bytes.NewReader(channel)

	decoded, err := parseChannelImageData(reader, false, uint64(len(channel)), len(pixels), 1)
	if err != nil {
		t.Fatalf("parseChannelImageData: %v", err)
	}
	if !bytes.Equal(decoded, pixels) {
		t.Fatalf("decoded = %v, want %v", decoded, pixels)
	}
}

func TestEncodeImageDataWritesSpecRLECompressionID(t *testing.T) {
	channel, err := EncodeChannelData([]byte{10}, 1, 1, false)
	if err != nil {
		t.Fatalf("EncodeChannelData: %v", err)
	}
	if got := binary.BigEndian.Uint16(channel[:2]); got != 1 {
		t.Fatalf("channel compression = %d, want PSD RLE id 1", got)
	}

	composite, err := EncodeCompositeImageData([][]byte{{10}, {20}, {30}}, 1, 1, false)
	if err != nil {
		t.Fatalf("EncodeCompositeImageData: %v", err)
	}
	if got := binary.BigEndian.Uint16(composite[:2]); got != 1 {
		t.Fatalf("composite compression = %d, want PSD RLE id 1", got)
	}
}

func specRLECompositeData(planes ...[]byte) []byte {
	var out bytes.Buffer
	writeUint16(&out, 1)
	rows := make([][]byte, 0, len(planes))
	for _, plane := range planes {
		row := EncodePackBitsRow(plane)
		rows = append(rows, row)
		writeUint16(&out, uint16(len(row)))
	}
	for _, row := range rows {
		out.Write(row)
	}
	return out.Bytes()
}

func specZipCompositeData(compression uint16, flat []byte) []byte {
	var payload bytes.Buffer
	zw := zlib.NewWriter(&payload)
	if _, err := zw.Write(flat); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	out := appendUint16(nil, compression)
	return append(out, payload.Bytes()...)
}

func appendUint16(dst []byte, value uint16, tail ...byte) []byte {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], value)
	dst = append(dst, buf[:]...)
	return append(dst, tail...)
}
