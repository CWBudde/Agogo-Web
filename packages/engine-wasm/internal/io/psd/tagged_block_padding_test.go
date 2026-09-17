package psd

import (
	"bytes"
	"strings"
	"testing"
)

// An odd-length tagged block inside a layer record is written both ways in the
// wild: pytoshop and psd-tools write it unpadded, the spec's "rounded up to an
// even byte count" reading pads it, and Agogo itself used to pad. The parser
// has to read both, because consuming a pad byte that is not there eats the
// first byte of the next block's signature and the record errors out.
func TestParseLayerRecordReadsOddLengthBlocksPaddedOrNot(t *testing.T) {
	testCases := []struct {
		name   string
		padded bool
	}{
		{name: "unpadded", padded: false},
		{name: "padded", padded: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// The odd block sits between two others so a desync corrupts the
			// block that follows rather than merely running off the end.
			record := parseSyntheticLayerRecord(
				t, "Fallback",
				taggedBlock{key: "lclr", payload: []byte{3, 0, 0, 0, 0, 0, 0, 0}},
				taggedBlock{key: "iOpa", payload: []byte{128}, padded: testCase.padded},
				taggedBlock{key: "lyid", payload: []byte{0, 0, 0, 7}},
			)
			if got, want := record.FillOpacity, 128.0/255.0; got != want {
				t.Fatalf("fill opacity = %v, want %v", got, want)
			}
			if got, want := record.LayerID, uint32(7); got != want {
				t.Fatalf("layer id after the odd block = %d, want %d", got, want)
			}
			if got, want := record.LayerColorTag, "yellow"; got != want {
				t.Fatalf("layer color tag = %q, want %q", got, want)
			}
			if len(record.MetadataWarnings) != 0 {
				t.Fatalf("metadata warnings = %q, want none", record.MetadataWarnings)
			}
		})
	}
}

// An unpadded odd block that ends the extra data used to make ReadByte fail and
// take the whole record with it.
func TestParseLayerRecordAcceptsAnOddLengthBlockAtTheEndOfARecord(t *testing.T) {
	for _, padded := range []bool{false, true} {
		record := parseSyntheticLayerRecord(
			t, "Trailing",
			taggedBlock{key: "lyid", payload: []byte{0, 0, 0, 9}},
			taggedBlock{key: "iOpa", payload: []byte{64}, padded: padded},
		)
		if got, want := record.FillOpacity, 64.0/255.0; got != want {
			t.Fatalf("padded=%v: fill opacity = %v, want %v", padded, got, want)
		}
	}
}

// Agogo's own writer must emit the form psd-tools and ImageMagick accept.
// Measured 2026-09-17: both refuse a layer-record block whose odd payload is
// followed by a pad byte (see WriteAdditionalLayerInfoBlock), and Agogo's AgAJ
// block is odd-length often enough that ImageMagick rejected those files
// outright.
func TestWriteAdditionalLayerInfoBlockDoesNotPadAnOddPayload(t *testing.T) {
	var out bytes.Buffer
	WriteAdditionalLayerInfoBlock(&out, "8BIM", "iOpa", []byte{128})
	if got, want := out.Len(), 13; got != want {
		t.Fatalf("block length = %d, want %d (12 header bytes + 1 payload byte, no padding)", got, want)
	}
	if got := out.Bytes()[12]; got != 128 {
		t.Fatalf("payload byte = %d, want 128", got)
	}

	// A second block must start immediately, with no gap before its signature.
	WriteAdditionalLayerInfoBlock(&out, "8BIM", "lyid", []byte{0, 0, 0, 1})
	if got := string(out.Bytes()[13:17]); got != "8BIM" {
		t.Fatalf("bytes after the odd block = %q, want the next signature", got)
	}
}

// Image resource blocks are a different section with a different rule, and that
// one really is padded to an even length. Guard against the fix above being
// copied into it.
func TestWriteImageResourceStillPadsAnOddPayload(t *testing.T) {
	var out bytes.Buffer
	WriteImageResource(&out, ImageResourceDPI, "", []byte{1})
	if got, want := out.Len(), 14; got != want {
		t.Fatalf("resource length = %d, want %d (12 header bytes + 1 payload byte + 1 pad)", got, want)
	}
	if got := out.Bytes()[out.Len()-1]; got != 0 {
		t.Fatalf("last byte = %d, want a zero pad byte", got)
	}
}

func TestParseLayerRecordStillRejectsAnInvalidBlockSignature(t *testing.T) {
	data := buildLayerRecordBytes("Broken", taggedBlock{key: "lyid", payload: []byte{0, 0, 0, 1}})
	// Corrupt the block signature in the extra data.
	index := bytes.LastIndex(data, []byte("8BIM"))
	copy(data[index:index+4], "XXXX")
	if _, err := parseLayerRecord(bytes.NewReader(data), false); err == nil ||
		!strings.Contains(err.Error(), "invalid additional layer info signature") {
		t.Fatalf("error = %v, want an invalid signature error", err)
	}
}
