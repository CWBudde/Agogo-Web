package psd

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
)

// boundsChannel is one entry of a crafted layer record's channel table.
type boundsChannel struct {
	id     int16
	length uint32
}

// boundsLayerRecord builds a structurally complete PSD layer record with the
// given (deliberately hostile) corner coordinates and channel table.
func boundsLayerRecord(top, left, bottom, right int32, channels []boundsChannel) []byte {
	var out bytes.Buffer
	writeInt32(&out, top)
	writeInt32(&out, left)
	writeInt32(&out, bottom)
	writeInt32(&out, right)
	writeUint16(&out, uint16(len(channels)))
	for _, channel := range channels {
		writeInt16(&out, channel.id)
		writeUint32(&out, channel.length)
	}
	writeString(&out, "8BIM")
	writeString(&out, "norm")
	out.Write([]byte{255, 0, 0, 0})

	var extra bytes.Buffer
	writeUint32(&extra, 0) // layer mask data length
	writeUint32(&extra, 0) // blending ranges length
	writePascalString4(&extra, "")
	writeUint32(&out, uint32(extra.Len()))
	out.Write(extra.Bytes())
	return out.Bytes()
}

// boundsLayerSection wraps layer records in a layer-and-mask section as
// ParseLayerAndMaskInfo expects to find it.
func boundsLayerSection(records ...[]byte) []byte {
	var layerInfo bytes.Buffer
	writeInt16(&layerInfo, int16(len(records)))
	for _, record := range records {
		layerInfo.Write(record)
	}

	var section bytes.Buffer
	writeUint32(&section, uint32(layerInfo.Len()))
	section.Write(layerInfo.Bytes())

	var out bytes.Buffer
	writeUint32(&out, uint32(section.Len()))
	out.Write(section.Bytes())
	return out.Bytes()
}

func boundsRequireError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want error containing %q", err, want)
	}
}

// A1: layer geometry must be rejected at the record, before it can size an
// allocation in parseChannelImageData.
func TestParseLayerRecordRejectsUnboundedGeometry(t *testing.T) {
	tests := []struct {
		name                     string
		top, left, bottom, right int32
		psb                      bool
		want                     string
	}{
		{
			name: "height past PSD limit",
			// bottom-top is ~2^31: unbounded, this sizes a ~17 GB row table.
			bottom: math.MaxInt32, right: 1,
			want: "invalid layer dimensions",
		},
		{
			name:  "width past PSD limit",
			right: PSDMaxDimension + 1, bottom: 1,
			want: "invalid layer dimensions 30001x1 (maximum 30000)",
		},
		{
			name:  "width past PSB limit",
			right: PSBMaxDimension + 1, bottom: 1, psb: true,
			want: "maximum 300000",
		},
		{
			name: "corner subtraction wraps int32",
			top:  math.MinInt32, bottom: math.MaxInt32, right: 1,
			want: "invalid layer dimensions",
		},
		{
			name:  "negative extent",
			right: -1, bottom: 1,
			want: "invalid layer dimensions",
		},
		{
			name:  "pixel count past decoded-size limit",
			right: PSDMaxDimension, bottom: PSDMaxDimension,
			want: "invalid pixel dimensions",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record := boundsLayerRecord(tc.top, tc.left, tc.bottom, tc.right, nil)
			_, err := parseLayerRecord(bytes.NewReader(record), tc.psb)
			boundsRequireError(t, err, tc.want)
		})
	}
}

func TestParseLayerRecordAcceptsOrdinaryGeometry(t *testing.T) {
	record := boundsLayerRecord(0, 0, 2, 3, []boundsChannel{{id: 0, length: 0}})
	got, err := parseLayerRecord(bytes.NewReader(record), false)
	if err != nil {
		t.Fatalf("parseLayerRecord: %v", err)
	}
	if got.Bounds.W != 3 || got.Bounds.H != 2 {
		t.Fatalf("bounds = %dx%d, want 3x2", got.Bounds.W, got.Bounds.H)
	}
}

// A1 end-to-end: the guard must fire on the real import path, not only when
// parseLayerRecord is called directly.
func TestParseLayerAndMaskInfoRejectsUnboundedLayerGeometry(t *testing.T) {
	section := boundsLayerSection(boundsLayerRecord(0, 0, math.MaxInt32, 1, nil))
	parser := NewParser(section)
	_, err := parser.ParseLayerAndMaskInfo(Header{
		Version: 1, Channels: 3, Width: 1, Height: 1, Depth: 8, ColorMode: ColorModeRGB,
	})
	boundsRequireError(t, err, "invalid layer dimensions")
}

// A3: a layer may not declare more channel bytes than remain in its section,
// individually or in total.
func TestParseLayerRecordRejectsChannelLengthBeyondSection(t *testing.T) {
	t.Run("single channel overruns section", func(t *testing.T) {
		record := boundsLayerRecord(0, 0, 1, 1, []boundsChannel{{id: 0, length: math.MaxUint32}})
		_, err := parseLayerRecord(bytes.NewReader(record), false)
		boundsRequireError(t, err, "exceeds remaining layer data")
	})
	t.Run("channel lengths sum past section", func(t *testing.T) {
		// Each length fits on its own; only the sum overruns.
		record := boundsLayerRecord(0, 0, 1, 1, []boundsChannel{
			{id: 0, length: 20},
			{id: 1, length: 20},
			{id: 2, length: 20},
		})
		_, err := parseLayerRecord(bytes.NewReader(record), false)
		boundsRequireError(t, err, "layer channel data length")
	})
}

// A1: every allocation inside parseChannelImageData is bounded by the
// validated geometry and by what remains to be read.
func TestParseChannelImageDataBoundsAllocations(t *testing.T) {
	rleChannel := func() []byte {
		var out bytes.Buffer
		writeUint16(&out, CompressionRLE)
		return out.Bytes()
	}

	t.Run("geometry past dimension limit", func(t *testing.T) {
		data := rleChannel()
		_, err := parseChannelImageData(bytes.NewReader(data), false, uint64(len(data)), 1, 2_000_000)
		boundsRequireError(t, err, "invalid layer dimensions")
	})

	t.Run("geometry past decoded-size limit", func(t *testing.T) {
		data := rleChannel()
		_, err := parseChannelImageData(bytes.NewReader(data), false, uint64(len(data)), PSDMaxDimension, PSDMaxDimension)
		boundsRequireError(t, err, "invalid pixel dimensions")
	})

	t.Run("row count table past remaining channel data", func(t *testing.T) {
		// In-range geometry, but nothing follows the compression word: the row
		// count table must be rejected before it is allocated.
		data := rleChannel()
		_, err := parseChannelImageData(bytes.NewReader(data), false, uint64(len(data)), 1, 20000)
		boundsRequireError(t, err, "RLE row count 20000 exceeds remaining channel data 0")
	})

	t.Run("declared channel length past remaining input", func(t *testing.T) {
		data := rleChannel()
		_, err := parseChannelImageData(bytes.NewReader(data), false, 1<<40, 1, 1)
		boundsRequireError(t, err, "channel length 1099511627776 exceeds remaining input")
	})
}

// A2: the composite row-count table is header-driven, so it must be bounded by
// the input that could hold it rather than by the header alone.
func TestParseCompositeImageDataBoundsRowCountTable(t *testing.T) {
	header := Header{
		Version: 2, PSB: true, Channels: PSDMaxChannels,
		Width: 1, Height: PSBMaxDimension, Depth: 8, ColorMode: ColorModeRGB,
	}
	// This header passes validateHeaderBounds: 300000 pixels x 56 channels is
	// well under the 512 MiB decoded-size limit.
	if err := validateHeaderBounds(header); err != nil {
		t.Fatalf("seed header must be header-valid, got %v", err)
	}
	var section bytes.Buffer
	writeUint16(&section, CompressionRLE)
	parser := NewParser(section.Bytes())
	_, err := parser.ParseCompositeImageData(header)
	boundsRequireError(t, err, "composite RLE row count 16800000 exceeds remaining input 0")
}

// A4: descriptor item counts get the same remaining-input guard as the layer
// and channel counts.
func TestParseDescriptorTextValueRejectsImpossibleItemCount(t *testing.T) {
	var data bytes.Buffer
	writeUnicodeString(&data, "")    // descriptor name
	writeDescriptorID(&data, "null") // class id
	writeUint32(&data, math.MaxUint32)

	_, _, err := ParseDescriptorTextValue(data.Bytes(), map[string]struct{}{"Txt ": {}})
	boundsRequireError(t, err, "descriptor item count 4294967295 exceeds remaining input")
}

func TestParseDescriptorTextValueStillReadsPlausibleItems(t *testing.T) {
	var data bytes.Buffer
	WriteDescriptor(&data, "", "null", []DescriptorItem{{Key: "Txt ", Type: "TEXT", Text: "hello"}})
	text, _, err := ParseDescriptorTextValue(data.Bytes(), map[string]struct{}{"Txt ": {}})
	if err != nil {
		t.Fatalf("ParseDescriptorTextValue: %v", err)
	}
	if text != "hello" {
		t.Fatalf("descriptor text = %q, want %q", text, "hello")
	}
}

// A5: a reader that cannot report its remaining input must fail closed rather
// than silently fall back to an unbounded read.
func TestReadBytesFromRejectsReaderWithoutRemainingLength(t *testing.T) {
	opaque := io.LimitReader(bytes.NewReader(make([]byte, 8)), 8)
	if _, ok := opaque.(interface{ Len() int }); ok {
		t.Fatal("test reader unexpectedly reports a remaining length")
	}
	_, err := readBytesFrom(opaque, 1<<30)
	boundsRequireError(t, err, "does not report remaining input")
	if errors.Is(err, io.EOF) {
		t.Fatal("unbounded read was attempted before the guard")
	}
}
