package psd

import (
	"bytes"
	"math"
	"testing"
)

// taggedBlock is one additional-layer-info block to place in a synthetic layer
// record's extra data, written exactly as given so a test can control the
// declared length and whether a pad byte follows.
type taggedBlock struct {
	key     string
	payload []byte
	padded  bool
}

// buildLayerRecordBytes assembles a minimal, channel-free layer record carrying
// the given blocks, in the shape parseLayerRecord consumes.
func buildLayerRecordBytes(name string, blocks ...taggedBlock) []byte {
	var data bytes.Buffer
	for range 4 {
		writeInt32(&data, 0)
	}
	writeUint16(&data, 0)
	writeString(&data, "8BIM")
	writeString(&data, "norm")
	data.Write([]byte{255, 0, 0, 0})

	var extra bytes.Buffer
	writeUint32(&extra, 0) // mask data
	writeUint32(&extra, 0) // blending ranges
	writePascalString4(&extra, name)
	for _, block := range blocks {
		writeString(&extra, "8BIM")
		writeString(&extra, block.key)
		writeUint32(&extra, uint32(len(block.payload)))
		extra.Write(block.payload)
		if block.padded && len(block.payload)%2 != 0 {
			extra.WriteByte(0)
		}
	}
	writeUint32(&data, uint32(extra.Len()))
	data.Write(extra.Bytes())
	return data.Bytes()
}

func parseSyntheticLayerRecord(t *testing.T, name string, blocks ...taggedBlock) LayerRecord {
	t.Helper()
	record, err := parseLayerRecord(bytes.NewReader(buildLayerRecordBytes(name, blocks...)), false)
	if err != nil {
		t.Fatalf("parseLayerRecord: %v", err)
	}
	return record
}

func TestParseLayerRecordReadsFillOpacityFromIOpa(t *testing.T) {
	// Four bytes is the form psd-tools reads and writes ("B3x"); one and two
	// byte blocks exist in the wild and must read identically.
	testCases := []struct {
		name    string
		payload []byte
	}{
		{name: "four bytes", payload: []byte{128, 0, 0, 0}},
		{name: "two bytes", payload: []byte{128, 0}},
		{name: "one byte", payload: []byte{128}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			record := parseSyntheticLayerRecord(t, "Filled", taggedBlock{key: "iOpa", payload: testCase.payload})
			if got, want := record.FillOpacity, 128.0/255.0; math.Abs(got-want) > 1e-9 {
				t.Fatalf("fill opacity = %v, want %v", got, want)
			}
			// iOpa used to fall through to the default branch, which recorded
			// an unsupported block and a metadata warning that psdimport turned
			// into an import warning. Every fixture asserts its warnings as an
			// exact set, so the silence is part of the contract.
			if len(record.UnsupportedBlocks) != 0 {
				t.Fatalf("unsupported blocks = %q, want none", record.UnsupportedBlocks)
			}
			if len(record.MetadataWarnings) != 0 {
				t.Fatalf("metadata warnings = %q, want none", record.MetadataWarnings)
			}
		})
	}
}

func TestParseLayerRecordDefaultsFillOpacityToOpaque(t *testing.T) {
	record := parseSyntheticLayerRecord(t, "Plain")
	if record.FillOpacity != 1 {
		t.Fatalf("fill opacity without iOpa = %v, want 1", record.FillOpacity)
	}
}

func TestParseLayerRecordKeepsFillOpacityOpaqueForEmptyIOpaPayload(t *testing.T) {
	// A truncated block must be a no-op, not a fully transparent layer.
	record := parseSyntheticLayerRecord(t, "Empty", taggedBlock{key: "iOpa", payload: []byte{}})
	if record.FillOpacity != 1 {
		t.Fatalf("fill opacity for an empty iOpa payload = %v, want 1", record.FillOpacity)
	}
}

func TestParseLayerRecordReadsFullyTransparentFillOpacity(t *testing.T) {
	record := parseSyntheticLayerRecord(t, "Hidden fill", taggedBlock{key: "iOpa", payload: []byte{0, 0, 0, 0}})
	if record.FillOpacity != 0 {
		t.Fatalf("fill opacity = %v, want 0", record.FillOpacity)
	}
}

// LayerRecord stores fill opacity as a unit float while the format stores a
// byte. That is only safe because the conversion is lossless in both
// directions over every value the file can hold.
func TestFillOpacityByteSurvivesTheUnitFloatRoundTrip(t *testing.T) {
	for value := range 256 {
		if got := UnitOpacity(float64(value) / 255.0); int(got) != value {
			t.Fatalf("UnitOpacity(%d/255) = %d, want %d", value, got, value)
		}
	}
}
