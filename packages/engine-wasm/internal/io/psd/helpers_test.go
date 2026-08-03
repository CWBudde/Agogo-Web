package psd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func TestPSDBlendModeMappingsRoundTripByteExactly(t *testing.T) {
	testCases := []struct {
		mode model.BlendMode
		key  string
	}{
		{model.BlendModeNormal, "norm"},
		{model.BlendModeDissolve, "diss"},
		{model.BlendModeMultiply, "mul "},
		{model.BlendModeColorBurn, "idiv"},
		{model.BlendModeLinearBurn, "lbrn"},
		{model.BlendModeDarken, "dark"},
		{model.BlendModeDarkerColor, "dkCl"},
		{model.BlendModeScreen, "scrn"},
		{model.BlendModeColorDodge, "div "},
		{model.BlendModeLinearDodge, "lddg"},
		{model.BlendModeLighten, "lite"},
		{model.BlendModeLighterColor, "lgCl"},
		{model.BlendModeOverlay, "over"},
		{model.BlendModeSoftLight, "sLit"},
		{model.BlendModeHardLight, "hLit"},
		{model.BlendModeVividLight, "vLit"},
		{model.BlendModeLinearLight, "lLit"},
		{model.BlendModePinLight, "pLit"},
		{model.BlendModeHardMix, "hMix"},
		{model.BlendModeDifference, "diff"},
		{model.BlendModeExclusion, "smud"},
		{model.BlendModeSubtract, "fsub"},
		{model.BlendModeDivide, "fdiv"},
		{model.BlendModeHue, "hue "},
		{model.BlendModeSaturation, "sat "},
		{model.BlendModeColor, "colr"},
		{model.BlendModeLuminosity, "lum "},
	}

	canonicalCount := 0
	for _, mapping := range psdBlendModeMappings {
		if mapping.canonical {
			canonicalCount++
		}
	}
	if canonicalCount != len(testCases) {
		t.Fatalf("canonical PSD blend mappings = %d, want %d", canonicalCount, len(testCases))
	}

	for _, tc := range testCases {
		t.Run(string(tc.mode), func(t *testing.T) {
			key := BlendKey(tc.mode)
			if key != tc.key {
				t.Fatalf("BlendKey(%q) = %q (% x), want %q (% x)", tc.mode, key, []byte(key), tc.key, []byte(tc.key))
			}
			if len(key) != 4 {
				t.Fatalf("BlendKey(%q) has %d bytes, want 4", tc.mode, len(key))
			}
			if got := MapBlendMode(key); got != tc.mode {
				t.Fatalf("MapBlendMode(%q) = %q, want %q", key, got, tc.mode)
			}
		})
	}
}

func TestMapBlendModeRecognizesPassThrough(t *testing.T) {
	mode, known := mapBlendMode("pass")
	if !known {
		t.Fatal("pass-through PSD key reported as unknown")
	}
	if mode != model.BlendModeNormal {
		t.Fatalf("MapBlendMode(\"pass\") = %q, want %q", mode, model.BlendModeNormal)
	}
}

func TestParseLayerRecordWarnsOnUnknownBlendMode(t *testing.T) {
	var data bytes.Buffer
	for range 4 {
		writeInt32(&data, 0)
	}
	writeUint16(&data, 0)
	writeString(&data, "8BIM")
	writeString(&data, "what")
	data.Write([]byte{255, 0, 0, 0})

	var extra bytes.Buffer
	writeUint32(&extra, 0)
	writeUint32(&extra, 0)
	writePascalString4(&extra, "")
	writeUint32(&data, uint32(extra.Len()))
	data.Write(extra.Bytes())

	record, err := parseLayerRecord(bytes.NewReader(data.Bytes()), false)
	if err != nil {
		t.Fatalf("parseLayerRecord: %v", err)
	}
	if record.BlendMode != model.BlendModeNormal {
		t.Fatalf("unknown blend mode imported as %q, want %q", record.BlendMode, model.BlendModeNormal)
	}
	if len(record.MetadataWarnings) != 1 || !strings.Contains(record.MetadataWarnings[0], `"what"`) {
		t.Fatalf("metadata warnings = %q, want warning naming unknown key", record.MetadataWarnings)
	}
}

func TestMapBlendModeDoesNotTrimSignificantBytes(t *testing.T) {
	if _, known := mapBlendMode("mul"); known {
		t.Fatal("three-byte key must not match four-byte PSD key \"mul \"")
	}
	if _, known := mapBlendMode(" div"); known {
		t.Fatal("misplaced padding must not match PSD key \"div \"")
	}
}
