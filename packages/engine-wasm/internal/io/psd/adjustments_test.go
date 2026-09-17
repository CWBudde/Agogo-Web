package psd

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

// The payload builders below are a THIRD implementation of these layouts,
// after the Python generator and the reader under test. That is deliberate for
// the malformed cases: a test that built its input with the reader's own helpers
// could not tell a layout error from a matching layout error. The well-formed
// cases are covered against real files by the fixture corpus; these cover the
// shapes no generator will produce.

func be16(values ...int) []byte {
	out := make([]byte, 0, len(values)*2)
	for _, value := range values {
		out = binary.BigEndian.AppendUint16(out, uint16(value)) //nolint:gosec // test data
	}
	return out
}

// levelsPayload builds a levl block whose first record is the given one and
// whose remaining 28 are identity.
func levelsPayload(composite ...int) []byte {
	out := be16(2)
	out = append(out, be16(composite...)...)
	for range levelsRecordCount - 1 {
		out = append(out, be16(0, 255, 0, 255, 100)...)
	}
	return out
}

func decodeParams(t *testing.T, payload *AdjustmentPayload) map[string]any {
	t.Helper()
	var params map[string]any
	if err := json.Unmarshal(payload.Params, &params); err != nil {
		t.Fatalf("params are not a JSON object: %v", err)
	}
	return params
}

func TestParseAdjustmentBlocksMapOntoEngineKinds(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		payload  []byte
		wantKind string
		// check runs against the decoded parameter object.
		check func(t *testing.T, params map[string]any)
	}{
		{
			name:     "levels converts the stored gamma",
			key:      "levl",
			payload:  levelsPayload(10, 245, 5, 250, 120),
			wantKind: "levels",
			check: func(t *testing.T, params map[string]any) {
				// 120 stands for 1.20, not for 120.
				if got := params["gamma"]; got != 1.2 {
					t.Errorf("gamma = %v, want 1.2", got)
				}
				if got := params["inputBlack"]; got != float64(10) {
					t.Errorf("inputBlack = %v, want 10", got)
				}
				if got := params["outputWhite"]; got != float64(250) {
					t.Errorf("outputWhite = %v, want 250", got)
				}
			},
		},
		{
			name:     "threshold reads its padded short",
			key:      "thrs",
			payload:  append(be16(96), 0, 0),
			wantKind: "threshold",
			check: func(t *testing.T, params map[string]any) {
				if got := params["threshold"]; got != float64(96) {
					t.Errorf("threshold = %v, want 96", got)
				}
			},
		},
		{
			name:     "posterize reads its padded short",
			key:      "post",
			payload:  append(be16(6), 0, 0),
			wantKind: "posterize",
			check: func(t *testing.T, params map[string]any) {
				if got := params["levels"]; got != float64(6) {
					t.Errorf("levels = %v, want 6", got)
				}
			},
		},
		{
			name:     "invert accepts an empty payload",
			key:      "nvrt",
			payload:  nil,
			wantKind: "invert",
			check: func(t *testing.T, params map[string]any) {
				if len(params) != 0 {
					t.Errorf("params = %v, want an empty object", params)
				}
			},
		},
		{
			name:     "color balance keeps negative tone values signed",
			key:      "blnc",
			payload:  append(be16(20, -10, 5, -15, 25, 0, 0, 5, -30), 1),
			wantKind: "color-balance",
			check: func(t *testing.T, params map[string]any) {
				shadows, ok := params["shadows"].(map[string]any)
				if !ok {
					t.Fatalf("shadows = %v, want an object", params["shadows"])
				}
				if got := shadows["magentaGreen"]; got != float64(-10) {
					t.Errorf("shadows.magentaGreen = %v, want -10", got)
				}
				if got := params["preserveLuminosity"]; got != true {
					t.Errorf("preserveLuminosity = %v, want true", got)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := &LayerRecord{}
			if err := parseLayerAdjustmentBlock(test.key, test.payload, record); err != nil {
				t.Fatalf("parseLayerAdjustmentBlock: %v", err)
			}
			if record.Adjustment == nil {
				t.Fatal("no adjustment was reconstructed")
			}
			if record.Adjustment.Kind != test.wantKind {
				t.Fatalf("kind = %q, want %q", record.Adjustment.Kind, test.wantKind)
			}
			test.check(t, decodeParams(t, record.Adjustment))
		})
	}
}

// TestParseLevelsReportsPerChannelRecordsAsLost pins the gap rather than the
// feature: the engine has one set of level values, so a file that sets the red
// channel separately loses that, and must say so.
func TestParseLevelsReportsPerChannelRecordsAsLost(t *testing.T) {
	payload := be16(2)
	payload = append(payload, be16(10, 245, 5, 250, 120)...) // composite
	payload = append(payload, be16(20, 235, 0, 255, 90)...)  // red, not identity
	for range levelsRecordCount - 2 {
		payload = append(payload, be16(0, 255, 0, 255, 100)...)
	}

	got, err := parseLevels(payload)
	if err != nil {
		t.Fatalf("parseLevels: %v", err)
	}
	if len(got.Lost) != 1 || !strings.Contains(got.Lost[0], "per-channel") {
		t.Fatalf("Lost = %v, want one entry naming the per-channel records", got.Lost)
	}

	// The same block with identity per-channel records loses nothing.
	clean, err := parseLevels(levelsPayload(10, 245, 5, 250, 120))
	if err != nil {
		t.Fatalf("parseLevels (identity channels): %v", err)
	}
	if len(clean.Lost) != 0 {
		t.Fatalf("Lost = %v, want none when only the composite record is set", clean.Lost)
	}
}

// TestParseCurvesKeepsChannelIdentity is the assertion the corpus fixture
// cannot make on its own: a reader that ignores the channel bitmap still
// produces two curves, just attached to the wrong channels.
func TestParseCurvesKeepsChannelIdentity(t *testing.T) {
	// Bitmap 0b1100 - bits 2 and 3, green and blue, deliberately skipping the
	// composite so that a reader filling the slots in order rather than by bit
	// puts the green curve on "points".
	payload := []byte{0}
	payload = append(payload, be16(1)...)
	payload = binary.BigEndian.AppendUint32(payload, 0b1100)
	payload = append(payload, be16(2, 0, 0, 255, 255)...)  // green
	payload = append(payload, be16(2, 10, 0, 245, 255)...) // blue

	got, err := parseCurves(payload)
	if err != nil {
		t.Fatalf("parseCurves: %v", err)
	}
	params := map[string]any{}
	if err := json.Unmarshal(got.Params, &params); err != nil {
		t.Fatalf("params: %v", err)
	}
	if _, ok := params["points"]; ok {
		t.Error("a composite curve appeared although bit 0 is clear")
	}
	if _, ok := params["greenPoints"]; !ok {
		t.Error("greenPoints missing although bit 2 is set")
	}
	if _, ok := params["bluePoints"]; !ok {
		t.Error("bluePoints missing although bit 3 is set")
	}
}

// TestParseAdjustmentBlocksRejectMalformedPayloads covers the shapes a writer
// will not produce. Each must be refused with a diagnosable error, never
// panic and never yield a half-built adjustment.
func TestParseAdjustmentBlocksRejectMalformedPayloads(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		payload []byte
		wantErr string
	}{
		{"levels truncated", "levl", be16(2), "levl needs"},
		{
			"levels wrong version", "levl",
			// Full length, but declaring version 3. Length alone must not be
			// taken as evidence that the layout is the one being read.
			append(be16(3), levelsPayload(0, 255, 0, 255, 100)[2:]...),
			"is not 2",
		},
		{"curves header truncated", "curv", []byte{0, 0}, "curv needs"},
		{"curves unknown version", "curv", append([]byte{0}, append(be16(9), 0, 0, 0, 1)...), "not 1 or 4"},
		{"hue2 truncated", "hue2", be16(2), "hue2 needs"},
		{"color balance truncated", "blnc", be16(1, 2, 3), "blnc needs"},
		{"channel mixer truncated", "mixr", be16(1, 0), "mixr needs"},
		{"selective color truncated", "selc", be16(1, 0), "selc needs"},
		{"photo filter truncated", "phfl", be16(2), "phfl needs"},
		{"photo filter xyz version", "phfl", be16(3), "XYZ"},
		{"black and white not a descriptor", "blwh", []byte{0, 0}, "blwh needs"},
		{"brightness contrast not a descriptor", "CgEd", []byte{0, 0}, "CgEd needs"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := &LayerRecord{}
			err := parseLayerAdjustmentBlock(test.key, test.payload, record)
			if err == nil {
				t.Fatalf("parseLayerAdjustmentBlock accepted a malformed %s payload", test.key)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err, test.wantErr)
			}
			if record.Adjustment != nil {
				t.Fatalf("a malformed payload still produced an adjustment: %+v", record.Adjustment)
			}
		})
	}
}

// TestCurvePointCountIsBoundedBeforeAllocating pins the allocation guard. The
// point count is two bytes, so without the format's 19-point cap a hostile file
// reserves 65535 points per channel from a payload that contains none.
func TestCurvePointCountIsBoundedBeforeAllocating(t *testing.T) {
	payload := []byte{0}
	payload = append(payload, be16(1)...)
	payload = binary.BigEndian.AppendUint32(payload, 1)
	payload = append(payload, be16(0xFFFF)...) // a count with no points behind it

	if _, err := parseCurves(payload); err == nil {
		t.Fatal("parseCurves accepted a 65535-point curve with no point data")
	} else if !strings.Contains(err.Error(), "[2, 19]") {
		t.Fatalf("error = %q, want it to name the point-count bound", err)
	}
}

// TestBrightnessContrastPrefersTheDescriptorOverTheObsoleteBlock is the reason
// the descriptor fixture writes both blocks with different numbers. Photoshop
// writes brit for readers that predate CgEd, and the two disagree as soon as
// the layer is edited.
func TestBrightnessContrastPrefersTheDescriptorOverTheObsoleteBlock(t *testing.T) {
	legacy := append(be16(11, 0xFFFF-7+1, 127), 0, 0)
	modern := brightnessContrastDescriptor(t, 30, -20)

	for _, order := range []struct {
		name  string
		first [2]any
	}{
		{"brit first", [2]any{"brit", "CgEd"}},
		{"CgEd first", [2]any{"CgEd", "brit"}},
	} {
		t.Run(order.name, func(t *testing.T) {
			record := &LayerRecord{}
			for _, key := range order.first {
				payload := legacy
				if key == "CgEd" {
					payload = modern
				}
				if err := parseLayerAdjustmentBlock(key.(string), payload, record); err != nil {
					t.Fatalf("parseLayerAdjustmentBlock(%v): %v", key, err)
				}
			}
			if record.Adjustment == nil {
				t.Fatal("no adjustment was reconstructed")
			}
			params := decodeParams(t, record.Adjustment)
			if got := params["brightness"]; got != float64(30) {
				t.Errorf("brightness = %v, want 30 (the CgEd value, not brit's 11)", got)
			}
			if got := params["contrast"]; got != float64(-20) {
				t.Errorf("contrast = %v, want -20 (the CgEd value, not brit's -7)", got)
			}
		})
	}
}

// TestLegacyBrightnessContrastAloneIsUsed: with no CgEd block there is nothing
// better, and the legacy values are the only ones the file has.
func TestLegacyBrightnessContrastAloneIsUsed(t *testing.T) {
	record := &LayerRecord{}
	payload := append(be16(11, 0xFFFF-7+1, 127), 0, 0)
	if err := parseLayerAdjustmentBlock("brit", payload, record); err != nil {
		t.Fatalf("parseLayerAdjustmentBlock: %v", err)
	}
	if record.Adjustment == nil {
		t.Fatal("no adjustment was reconstructed")
	}
	params := decodeParams(t, record.Adjustment)
	if got := params["brightness"]; got != float64(11) {
		t.Errorf("brightness = %v, want 11", got)
	}
	if got := params["legacy"]; got != true {
		t.Errorf("legacy = %v, want true: the block predates the current algorithm", got)
	}
}

// brightnessContrastDescriptor builds a CgEd payload: the uint32 version, then
// a descriptor of long and bool items.
func brightnessContrastDescriptor(t *testing.T, brightness, contrast int) []byte {
	t.Helper()
	out := binary.BigEndian.AppendUint32(nil, 16)
	out = append(out, descriptorUnicode("")...)
	out = append(out, descriptorKey("null")...)
	out = binary.BigEndian.AppendUint32(out, 3)
	out = append(out, descriptorKey("Brgh")...)
	out = append(out, []byte("long")...)
	out = binary.BigEndian.AppendUint32(out, uint32(brightness)) //nolint:gosec // test data
	out = append(out, descriptorKey("Cntr")...)
	out = append(out, []byte("long")...)
	out = binary.BigEndian.AppendUint32(out, uint32(contrast)) //nolint:gosec // test data
	out = append(out, descriptorKey("useLegacy")...)
	out = append(out, []byte("bool")...)
	out = append(out, 0)
	return out
}

func descriptorUnicode(text string) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(text)+1)) //nolint:gosec // test data
	for _, r := range text {
		out = binary.BigEndian.AppendUint16(out, uint16(r)) //nolint:gosec // test data
	}
	return append(out, 0, 0)
}

func descriptorKey(key string) []byte {
	if len(key) == 4 {
		return append([]byte{0, 0, 0, 0}, key...)
	}
	out := binary.BigEndian.AppendUint32(nil, uint32(len(key))) //nolint:gosec // test data
	return append(out, key...)
}
