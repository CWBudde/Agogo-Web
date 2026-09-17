package psd

import (
	"bytes"
	"testing"
)

// buildTyShPayload assembles the TySh envelope by hand: version, the six-double
// transform, the text version, the descriptor version, then the descriptor.
func buildTyShPayload(t *testing.T, descriptorBody []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	writeUint16(&b, 1)
	for range 6 {
		writeFloat64(&b, 0)
	}
	writeUint16(&b, 50)
	writeUint32(&b, 16)
	b.Write(descriptorBody)
	return b.Bytes()
}

func buildTxLrDescriptor(t *testing.T, extra func(*bytes.Buffer)) []byte {
	t.Helper()
	var b bytes.Buffer
	writeUnicodeString(&b, "")
	writeDescriptorID(&b, "TxLr")
	count := 1
	if extra != nil {
		count = 2
	}
	writeUint32(&b, uint32(count))
	if extra != nil {
		extra(&b)
	}
	writeDescriptorID(&b, "Txt ")
	b.WriteString("TEXT")
	writeUnicodeString(&b, "Hello")
	return b.Bytes()
}

func TestParseTextLayerMetadataReadsTheDescriptorText(t *testing.T) {
	record := &LayerRecord{}
	if err := parseTextLayerMetadata("TySh", buildTyShPayload(t, buildTxLrDescriptor(t, nil)), record); err != nil {
		t.Fatalf("parseTextLayerMetadata: %v", err)
	}
	if record.Text == nil || record.Text.ParsedText != "Hello" {
		t.Fatalf("parsed text = %+v, want Hello", record.Text)
	}
}

// The reason the old reader had to go. It understood only TEXT, bool, doub and
// long, so it errored on the first Objc and never reached the text - and every
// real Photoshop text layer carries Objc, enum and tdta values before it.
func TestParseTextLayerMetadataReadsTextPastRicherValueTypes(t *testing.T) {
	record := &LayerRecord{}
	payload := buildTyShPayload(t, buildTxLrDescriptor(t, func(b *bytes.Buffer) {
		writeDescriptorID(b, "bounds")
		b.WriteString("Objc")
		writeUnicodeString(b, "")
		writeDescriptorID(b, "bounds")
		writeUint32(b, 1)
		writeDescriptorID(b, "Top ")
		b.WriteString("UntF")
		b.WriteString("#Pnt")
		writeFloat64(b, 12)
	}))

	if err := parseTextLayerMetadata("TySh", payload, record); err != nil {
		t.Fatalf("parseTextLayerMetadata: %v", err)
	}
	if record.Text == nil || record.Text.ParsedText != "Hello" {
		t.Fatalf("parsed text = %+v, want Hello past the Objc and UntF values", record.Text)
	}
}

func TestParseTextLayerMetadataSurvivesAMalformedDescriptor(t *testing.T) {
	record := &LayerRecord{}
	payload := buildTyShPayload(t, []byte{0xFF, 0xFF, 0xFF, 0xFF})
	if err := parseTextLayerMetadata("TySh", payload, record); err != nil {
		t.Fatalf("parseTextLayerMetadata: %v", err)
	}
	if record.Text == nil {
		t.Fatal("record.Text is nil; a malformed descriptor must still record the block")
	}
}
