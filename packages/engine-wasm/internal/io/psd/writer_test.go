package psd

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func TestBuildSectionDividerIncludesBlendSignatureAndKey(t *testing.T) {
	payload := buildSectionDivider(LayerSectionOpenFolder, "pass")
	if len(payload) != 12 {
		t.Fatalf("payload length = %d, want 12", len(payload))
	}
	if got := binary.BigEndian.Uint32(payload[:4]); got != LayerSectionOpenFolder {
		t.Fatalf("section type = %d, want %d", got, LayerSectionOpenFolder)
	}
	if got := string(payload[4:8]); got != "8BIM" {
		t.Fatalf("signature = %q, want 8BIM", got)
	}
	if got := string(payload[8:12]); got != "pass" {
		t.Fatalf("blend key = %q, want pass", got)
	}
}

func TestWriteLayerMaskDataUsesDocumentSpaceRectAndByteFlags(t *testing.T) {
	var out bytes.Buffer
	writeLayerMaskData(&out, &model.LayerMask{
		Enabled: false,
		Width:   3,
		Height:  2,
		Data:    make([]byte, 6),
	})
	payload := out.Bytes()
	if got := binary.BigEndian.Uint32(payload[:4]); got != 18 {
		t.Fatalf("mask payload length = %d, want 18", got)
	}
	if got := binary.BigEndian.Uint32(payload[12:16]); got != 2 {
		t.Fatalf("bottom = %d, want 2", got)
	}
	if got := binary.BigEndian.Uint32(payload[16:20]); got != 3 {
		t.Fatalf("right = %d, want 3", got)
	}
	if got := payload[20]; got != 255 {
		t.Fatalf("default color = %d, want 255", got)
	}
	if got := payload[21]; got != 0x02 {
		t.Fatalf("flags = %#x, want disabled bit", got)
	}
}
