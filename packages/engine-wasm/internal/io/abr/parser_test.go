package abr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func TestParseV6RawSample(t *testing.T) {
	pixels := []byte{0, 64, 128, 255, 255, 128, 64, 0}
	data := fixtureABR(6, 1, fixtureBlock("samp", fixtureSampleRecord(1, 4, 2, 0, pixels)))

	lib, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if lib.Version != 6 || lib.Subversion != 1 {
		t.Fatalf("version = %d.%d, want 6.1", lib.Version, lib.Subversion)
	}
	if len(lib.Sampled) != 1 {
		t.Fatalf("sample count = %d, want 1", len(lib.Sampled))
	}
	brush := lib.Sampled[0]
	if brush.Key != fixtureUUID || brush.Width != 4 || brush.Height != 2 || brush.Depth != 8 || brush.Compression != 0 {
		t.Fatalf("sample metadata = %#v", brush)
	}
	if !bytes.Equal(brush.Pixels, pixels) {
		t.Fatalf("pixels = %v, want %v", brush.Pixels, pixels)
	}

	data[len(data)-1] ^= 0xff
	if bytes.Equal(brush.Pixels, data[len(data)-len(pixels):]) {
		t.Fatal("returned pixels alias input")
	}
}

func TestParseV7PackBitsSample(t *testing.T) {
	want := []byte{
		7, 7, 7, 7, 7,
		1, 2, 3, 4, 5,
	}
	// Each scanline has its own encoded byte count. The first is a repeat
	// run, the second is a literal run.
	compressed := []byte{0, 2, 0, 6, 0xfc, 7, 4, 1, 2, 3, 4, 5}
	data := fixtureABR(7, 2, fixtureBlock("samp", fixtureSampleRecord(2, 5, 2, 1, compressed)))

	lib, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if lib.Version != 7 || lib.Subversion != 2 {
		t.Fatalf("version = %d.%d, want 7.2", lib.Version, lib.Subversion)
	}
	if got := lib.Sampled[0].Pixels; !bytes.Equal(got, want) {
		t.Fatalf("pixels = %v, want %v", got, want)
	}
}

func TestParseComputedBrushDescriptor(t *testing.T) {
	desc := fixtureDescriptor()
	data := fixtureABR(6, 1, fixtureBlock("desc", desc))

	lib, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(lib.Descriptors) != 1 {
		t.Fatalf("descriptor count = %d, want 1", len(lib.Descriptors))
	}
	d := lib.Descriptors[0]
	if d.Name != "Synthetic Computed" || d.ClassID != "Brsh" || len(d.Items) != 6 {
		t.Fatalf("descriptor = %#v", d)
	}
	if got := d.Items[0].Value.String; got != "Round 24" {
		t.Fatalf("name = %q", got)
	}
	if got := d.Items[1].Value; got.Unit != "#Pxl" || got.Float != 24 {
		t.Fatalf("diameter = %#v", got)
	}
	if !d.Items[2].Value.Bool {
		t.Fatal("enabled = false, want true")
	}
	if got := d.Items[3].Value.Enum; got.Type != "BlnM" || got.Value != "Nrml" {
		t.Fatalf("enum = %#v", got)
	}
	if got := d.Items[4].Value.Object; got == nil || got.Items[0].Value.Integer != 75 {
		t.Fatalf("nested object = %#v", got)
	}
	if got := d.Items[5].Value.List; len(got) != 2 || got[0].Float != 0.25 || got[1].Float != 0.75 {
		t.Fatalf("list = %#v", got)
	}
}

func TestParseMultipleSectionsAndUnknownSection(t *testing.T) {
	raw := fixtureSampleRecord(1, 1, 1, 0, []byte{99})
	data := fixtureABR(6, 1,
		fixtureBlock("patt", []byte{1, 2, 3, 4}),
		fixtureBlock("desc", fixtureDescriptor()),
		fixtureBlock("samp", raw),
	)
	lib, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(lib.Sections) != 3 || len(lib.Descriptors) != 1 || len(lib.Sampled) != 1 {
		t.Fatalf("library = %#v", lib)
	}
}

func TestParseRejectsEveryTruncationWithoutPartialResult(t *testing.T) {
	sample := fixtureBlock("samp", fixtureSampleRecord(1, 2, 2, 0, []byte{1, 2, 3, 4}))
	desc := fixtureBlock("desc", fixtureDescriptor())
	data := fixtureABR(6, 1, sample, desc)
	// The prefix ending exactly after the first block is itself a valid ABR.
	// Every partial byte of the following block must instead invalidate the
	// entire parse, without exposing the already decoded sampled brush.
	secondBlockStart := 4 + len(sample)
	for n := secondBlockStart + 1; n < len(data); n++ {
		lib, err := Parse(data[:n])
		if err == nil {
			t.Fatalf("Parse(data[:%d]) unexpectedly succeeded", n)
		}
		if lib != nil {
			t.Fatalf("Parse(data[:%d]) returned partial result %#v", n, lib)
		}
	}
}

func TestParseRejectsMalformedPackBits(t *testing.T) {
	tests := map[string][]byte{
		"short row":       {0, 2, 0, 42},
		"long repeat":     {0, 2, 0xf8, 42},
		"missing literal": {0, 2, 3, 42},
	}
	for name, compressed := range tests {
		t.Run(name, func(t *testing.T) {
			data := fixtureABR(7, 1, fixtureBlock("samp", fixtureSampleRecord(1, 4, 1, 1, compressed)))
			lib, err := Parse(data)
			if !errors.Is(err, ErrMalformed) || lib != nil {
				t.Fatalf("Parse() = (%#v, %v), want nil malformed error", lib, err)
			}
		})
	}
}

func TestParseRejectsLimitsBeforeAllocation(t *testing.T) {
	data := fixtureABR(6, 1, fixtureBlock("samp", fixtureSampleRecord(1, 4, 2, 0, make([]byte, 8))))
	tests := map[string]Limits{
		"file":      {MaxFileBytes: len(data) - 1},
		"section":   {MaxSectionBytes: 8},
		"dimension": {MaxDimension: 3},
		"pixels":    {MaxPixels: 7},
	}
	for name, limits := range tests {
		t.Run(name, func(t *testing.T) {
			lib, err := ParseWithLimits(data, limits)
			if !errors.Is(err, ErrLimit) || lib != nil {
				t.Fatalf("ParseWithLimits() = (%#v, %v), want nil limit error", lib, err)
			}
		})
	}
}

func TestParseRejectsUnsupportedFeatures(t *testing.T) {
	tests := map[string][]byte{
		"major version": fixtureABR(5, 1, fixtureBlock("desc", fixtureDescriptor())),
		"subversion":    fixtureABR(6, 3, fixtureBlock("desc", fixtureDescriptor())),
		"depth":         fixtureABR(6, 1, fixtureBlock("samp", fixtureSampleRecordWithDepth(1, 1, 1, 16, 0, []byte{0, 0}))),
		"compression":   fixtureABR(6, 1, fixtureBlock("samp", fixtureSampleRecordWithDepth(1, 1, 1, 8, 2, []byte{0}))),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			lib, err := Parse(data)
			if !errors.Is(err, ErrUnsupported) || lib != nil {
				t.Fatalf("Parse() = (%#v, %v), want nil unsupported error", lib, err)
			}
		})
	}
}

func TestParseRejectsBadBoundsAndPadding(t *testing.T) {
	record := fixtureSampleRecord(1, 1, 1, 0, []byte{1})
	// right is the fourth int32 after the subversion-1 prefix.
	binary.BigEndian.PutUint32(record[4+samplePrefixV1+12:], 0)
	lib, err := Parse(fixtureABR(6, 1, fixtureBlock("samp", record)))
	if !errors.Is(err, ErrMalformed) || lib != nil {
		t.Fatalf("bad bounds Parse() = (%#v, %v)", lib, err)
	}

	block := fixtureABR(6, 1,
		fixtureBlock("desc", fixtureDescriptor()),
		fixtureBlock("patt", []byte{42}),
	)
	block[len(block)-1] = 1
	lib, err = Parse(block)
	if !errors.Is(err, ErrMalformed) || lib != nil {
		t.Fatalf("bad padding Parse() = (%#v, %v)", lib, err)
	}
}

const fixtureUUID = "12345678-1234-1234-1234-123456789abc"

func fixtureABR(version, subversion uint16, blocks ...[]byte) []byte {
	var b bytes.Buffer
	writeU16(&b, version)
	writeU16(&b, subversion)
	for _, block := range blocks {
		b.Write(block)
	}
	return b.Bytes()
}

func fixtureBlock(key string, payload []byte) []byte {
	var b bytes.Buffer
	b.WriteString("8BIM")
	b.WriteString(key)
	writeU32(&b, uint32(len(payload)))
	b.Write(payload)
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
	return b.Bytes()
}

func fixtureSampleRecord(subversion uint16, width, height int, compression byte, pixels []byte) []byte {
	return fixtureSampleRecordWithDepth(subversion, width, height, 8, compression, pixels)
}

func fixtureSampleRecordWithDepth(subversion uint16, width, height int, depth uint16, compression byte, pixels []byte) []byte {
	var payload bytes.Buffer
	payload.WriteByte(byte(len(fixtureUUID)))
	payload.WriteString(fixtureUUID)
	prefixLen := samplePrefixV1
	if subversion == 2 {
		prefixLen = samplePrefixV2
	}
	payload.Write(make([]byte, prefixLen-payload.Len()))
	writeI32(&payload, 0)
	writeI32(&payload, 0)
	writeI32(&payload, int32(height))
	writeI32(&payload, int32(width))
	writeU16(&payload, depth)
	payload.WriteByte(compression)
	payload.Write(pixels)

	var record bytes.Buffer
	writeU32(&record, uint32(payload.Len()))
	record.Write(payload.Bytes())
	for record.Len()%4 != 0 {
		record.WriteByte(0)
	}
	return record.Bytes()
}

func fixtureDescriptor() []byte {
	var b bytes.Buffer
	writeU32(&b, 16)
	writeDescriptorHeader(&b, "Synthetic Computed", "Brsh", 6)
	writeClassID(&b, "Nm  ")
	b.WriteString("TEXT")
	writeUnicode(&b, "Round 24")
	writeClassID(&b, "Dmtr")
	b.WriteString("UntF")
	b.WriteString("#Pxl")
	writeF64(&b, 24)
	writeClassID(&b, "enab")
	b.WriteString("bool")
	b.WriteByte(1)
	writeClassID(&b, "mode")
	b.WriteString("enum")
	writeClassID(&b, "BlnM")
	writeClassID(&b, "Nrml")
	writeClassID(&b, "shape")
	b.WriteString("Objc")
	writeDescriptorHeader(&b, "", "Shp ", 1)
	writeClassID(&b, "Hrdn")
	b.WriteString("long")
	writeI32(&b, 75)
	writeClassID(&b, "points")
	b.WriteString("VlLs")
	writeU32(&b, 2)
	b.WriteString("doub")
	writeF64(&b, 0.25)
	b.WriteString("doub")
	writeF64(&b, 0.75)
	return b.Bytes()
}

func writeDescriptorHeader(b *bytes.Buffer, name, classID string, count uint32) {
	writeUnicode(b, name)
	writeClassID(b, classID)
	writeU32(b, count)
}

func writeUnicode(b *bytes.Buffer, s string) {
	writeU32(b, uint32(len([]rune(s))))
	for _, r := range s {
		writeU16(b, uint16(r))
	}
}

func writeClassID(b *bytes.Buffer, id string) {
	if len(id) == 4 {
		writeU32(b, 0)
	} else {
		writeU32(b, uint32(len(id)))
	}
	b.WriteString(id)
}

func writeU16(b *bytes.Buffer, v uint16)  { _ = binary.Write(b, binary.BigEndian, v) }
func writeU32(b *bytes.Buffer, v uint32)  { _ = binary.Write(b, binary.BigEndian, v) }
func writeI32(b *bytes.Buffer, v int32)   { _ = binary.Write(b, binary.BigEndian, v) }
func writeF64(b *bytes.Buffer, v float64) { _ = binary.Write(b, binary.BigEndian, math.Float64bits(v)) }
