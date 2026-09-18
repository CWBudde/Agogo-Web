package psd

import (
	"encoding/binary"
	"encoding/json"
	"math"
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

// hue2Payload builds a hue2 block. `colorize` drives the mode byte, and the two
// triples are given separately so a test can make them differ.
func hue2Payload(colorize bool, colorization, master [3]int, ranges [6][7]int) []byte {
	enable := 0
	if colorize {
		enable = 1
	}
	out := be16(2)
	out = append(out, byte(enable), 0)
	out = append(out, be16(colorization[0], colorization[1], colorization[2])...)
	out = append(out, be16(master[0], master[1], master[2])...)
	for _, entry := range ranges {
		out = append(out, be16(entry[0], entry[1], entry[2], entry[3])...)
		out = append(out, be16(entry[4], entry[5], entry[6])...)
	}
	return out
}

// defaultHueRanges are the six untouched Photoshop sectors with no settings.
func defaultHueRanges() [6][7]int {
	var out [6][7]int
	for bucket := range out {
		centre := bucket * 60
		out[bucket] = [7]int{
			mod360(centre - 45), mod360(centre - 15),
			mod360(centre + 15), mod360(centre + 45),
			0, 0, 0,
		}
	}
	return out
}

// TestParseHueSaturationSelectsTheLiveTriple pins the mode byte and the triple
// it selects.
//
// Both are easy to get backwards, and a fixture alone cannot catch it: the
// generator and the deriver are two implementations, but the polarity is a
// single fact that a wrong reading would apply consistently to both. psd-tools'
// compositor is the reference - a non-zero byte means colorize, and it then
// uses the colorization triple and ignores master and every range.
func TestParseHueSaturationSelectsTheLiveTriple(t *testing.T) {
	colorization := [3]int{210, 40, -15}
	master := [3]int{25, -30, 10}

	t.Run("colorize mode uses the colorization triple", func(t *testing.T) {
		got, err := parseHueSaturation(hue2Payload(true, colorization, master, defaultHueRanges()))
		if err != nil {
			t.Fatalf("parseHueSaturation: %v", err)
		}
		params := decodeParams(t, got)
		if params["colorize"] != true {
			t.Errorf("colorize = %v, want true", params["colorize"])
		}
		if params["hueShift"] != float64(210) {
			t.Errorf("hueShift = %v, want 210 (colorization), not 25 (master)", params["hueShift"])
		}
		if params["saturation"] != float64(40) {
			t.Errorf("saturation = %v, want 40 (colorization), not -30 (master)", params["saturation"])
		}
	})

	t.Run("hue-adjustment mode uses the master triple", func(t *testing.T) {
		got, err := parseHueSaturation(hue2Payload(false, colorization, master, defaultHueRanges()))
		if err != nil {
			t.Fatalf("parseHueSaturation: %v", err)
		}
		params := decodeParams(t, got)
		if params["colorize"] != false {
			t.Errorf("colorize = %v, want false", params["colorize"])
		}
		if params["hueShift"] != float64(25) {
			t.Errorf("hueShift = %v, want 25 (master), not 210 (colorization)", params["hueShift"])
		}
	})

	t.Run("colorize mode ignores the per-range entries", func(t *testing.T) {
		ranges := defaultHueRanges()
		ranges[0] = [7]int{300, 330, 20, 50, 12, -5, 3} // moved edges AND settings
		got, err := parseHueSaturation(hue2Payload(true, colorization, master, ranges))
		if err != nil {
			t.Fatalf("parseHueSaturation: %v", err)
		}
		params := decodeParams(t, got)
		if _, present := params["reds"]; present {
			t.Error("a per-range entry was carried although colorize ignores them")
		}
		// Nothing is lost: Photoshop does not read them in this mode either.
		if len(got.Lost) != 0 {
			t.Errorf("Lost = %v, want none: the ranges are inert, not lost", got.Lost)
		}
	})
}

// TestParseHueSaturationReportsMovedEdges: the engine's six buckets are fixed
// sectors, so a file that moved an edge loses that and has to say so.
func TestParseHueSaturationReportsMovedEdges(t *testing.T) {
	master := [3]int{25, -30, 10}

	clean, err := parseHueSaturation(hue2Payload(false, [3]int{}, master, defaultHueRanges()))
	if err != nil {
		t.Fatalf("parseHueSaturation: %v", err)
	}
	if len(clean.Lost) != 0 {
		t.Fatalf("Lost = %v, want none when every edge is at its default", clean.Lost)
	}

	moved := defaultHueRanges()
	moved[0] = [7]int{300, 330, 20, 50, 0, 0, 0}
	got, err := parseHueSaturation(hue2Payload(false, [3]int{}, master, moved))
	if err != nil {
		t.Fatalf("parseHueSaturation: %v", err)
	}
	if len(got.Lost) != 1 || !strings.Contains(got.Lost[0], "hue edges") {
		t.Fatalf("Lost = %v, want one entry naming the hue edges", got.Lost)
	}
}

// TestParseChannelMixerReadsEveryOutputRow is the regression behind the review:
// channelMixerAdjustmentFactory builds its matrix straight from Red/Green/Blue
// with no defaulting, so a params object carrying only "red" gives the other two
// outputs an all-zero row and the imported layer blacks out green and blue.
func TestParseChannelMixerReadsEveryOutputRow(t *testing.T) {
	payload := be16(1, 0) // version, not monochrome
	payload = append(payload, be16(80, 10, 10, 0, 0)...)
	payload = append(payload, be16(-20, 120, 0, 0, 0)...)
	payload = append(payload, be16(5, -15, 110, 0, 0)...)

	got, err := parseChannelMixer(payload)
	if err != nil {
		t.Fatalf("parseChannelMixer: %v", err)
	}
	params := decodeParams(t, got)
	for name, want := range map[string][]float64{
		"red":   {80, 10, 10},
		"green": {-20, 120, 0},
		"blue":  {5, -15, 110},
	} {
		row, ok := params[name].([]any)
		if !ok {
			t.Fatalf("%s = %v, want a three-element row", name, params[name])
		}
		for index, value := range want {
			if row[index] != value {
				t.Errorf("%s[%d] = %v, want %v", name, index, row[index], value)
			}
		}
	}
	if len(got.Lost) != 0 {
		t.Errorf("Lost = %v, want none: every row was present", got.Lost)
	}
}

// TestParseChannelMixerFillsAbsentRowsWithIdentity: a short block must leave the
// missing channels alone, never zero them.
func TestParseChannelMixerFillsAbsentRowsWithIdentity(t *testing.T) {
	payload := append(be16(1, 0), be16(80, 10, 10, 0, 0)...)

	got, err := parseChannelMixer(payload)
	if err != nil {
		t.Fatalf("parseChannelMixer: %v", err)
	}
	params := decodeParams(t, got)
	green, _ := params["green"].([]any)
	blue, _ := params["blue"].([]any)
	if len(green) != 3 || green[1] != float64(100) || green[0] != float64(0) {
		t.Errorf("green = %v, want the identity row [0 100 0]", params["green"])
	}
	if len(blue) != 3 || blue[2] != float64(100) {
		t.Errorf("blue = %v, want the identity row [0 0 100]", params["blue"])
	}
	if len(got.Lost) != 2 {
		t.Fatalf("Lost = %v, want one entry per absent row", got.Lost)
	}
}

// TestParseChannelMixerMonochromeReplicatesTheGreyRow: Photoshop writes one grey
// row, and the engine runs the full matrix then takes the luminance of the
// result - so the row has to reach all three outputs for the luminance to come
// out as the row intended.
func TestParseChannelMixerMonochromeReplicatesTheGreyRow(t *testing.T) {
	payload := append(be16(1, 1), be16(40, 40, 20, 0, 0)...)

	got, err := parseChannelMixer(payload)
	if err != nil {
		t.Fatalf("parseChannelMixer: %v", err)
	}
	params := decodeParams(t, got)
	if params["monochrome"] != true {
		t.Errorf("monochrome = %v, want true", params["monochrome"])
	}
	for _, name := range []string{"red", "green", "blue"} {
		row, ok := params[name].([]any)
		if !ok || len(row) != 3 || row[0] != float64(40) || row[2] != float64(20) {
			t.Errorf("%s = %v, want the grey row [40 40 20]", name, params[name])
		}
	}
	if len(got.Lost) != 0 {
		t.Errorf("Lost = %v, want none: one row IS the whole monochrome block", got.Lost)
	}
}

// TestParseChannelMixerReportsUnrepresentableFields covers the two fields the
// engine's 3x3 matrix has no room for.
func TestParseChannelMixerReportsUnrepresentableFields(t *testing.T) {
	payload := be16(1, 0)
	payload = append(payload, be16(80, 10, 10, 7, 5)...) // fourth weight 7, constant 5
	payload = append(payload, be16(0, 100, 0, 0, 0)...)
	payload = append(payload, be16(0, 0, 100, 0, 0)...)

	got, err := parseChannelMixer(payload)
	if err != nil {
		t.Fatalf("parseChannelMixer: %v", err)
	}
	joined := strings.Join(got.Lost, "; ")
	for _, want := range []string{"fourth source weight", "constant term"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Lost = %v, want it to mention %q", got.Lost, want)
		}
	}
}

// TestParsePhotoFilterRefusesNonRGBColourSpaces: reading a CMYK or Lab triple as
// RGB gives a filter colour unrelated to the file's. The fallback contract is
// that a layer is either reconstructed correctly or reported - never
// reconstructed wrongly and warned about.
func TestParsePhotoFilterRefusesNonRGBColourSpaces(t *testing.T) {
	payload := be16(2, 2) // version 2, colour space 2 (not RGB)
	payload = append(payload, be16(0, 0, 0, 0)...)
	payload = append(payload, 0, 0, 0, 35, 1)

	record := &LayerRecord{}
	err := parseLayerAdjustmentBlock("phfl", payload, record)
	if err == nil {
		t.Fatal("a non-RGB photo filter was reconstructed as though it were RGB")
	}
	if !strings.Contains(err.Error(), "not RGB") {
		t.Errorf("error = %q, want it to name the colour space problem", err)
	}
	if record.Adjustment != nil {
		t.Errorf("a refused block still produced an adjustment: %+v", record.Adjustment)
	}
}

// TestParseInvertReportsAnUnexpectedPayload: Invert cannot be misconfigured, so
// extra bytes cannot make the layer wrong - but discarding them silently is how
// a future field disappears without trace.
func TestParseInvertReportsAnUnexpectedPayload(t *testing.T) {
	empty, err := parseInvert(nil)
	if err != nil {
		t.Fatalf("parseInvert: %v", err)
	}
	if len(empty.Lost) != 0 {
		t.Errorf("Lost = %v, want none for the defined empty payload", empty.Lost)
	}

	got, err := parseInvert([]byte{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("parseInvert: %v", err)
	}
	if len(got.Lost) != 1 || !strings.Contains(got.Lost[0], "nvrt payload") {
		t.Fatalf("Lost = %v, want one entry naming the unexpected payload", got.Lost)
	}
}

// TestParseBlackAndWhiteReadsTheTintColour: blackWhiteAdjustmentFactory
// substitutes a fixed brown when tintColor is absent, so importing the flag
// without the colour renders a different tint from Photoshop while looking
// entirely successful.
func TestParseBlackAndWhiteReadsTheTintColour(t *testing.T) {
	t.Run("the colour is carried when present", func(t *testing.T) {
		got, err := parseBlackAndWhite(blackAndWhiteDescriptor(t, true, &[3]float64{12, 34, 56}))
		if err != nil {
			t.Fatalf("parseBlackAndWhite: %v", err)
		}
		params := decodeParams(t, got)
		colour, ok := params["tintColor"].([]any)
		if !ok || len(colour) != 3 {
			t.Fatalf("tintColor = %v, want a three-element colour", params["tintColor"])
		}
		if colour[0] != float64(12) || colour[1] != float64(34) || colour[2] != float64(56) {
			t.Errorf("tintColor = %v, want [12 34 56]", colour)
		}
		if len(got.Lost) != 0 {
			t.Errorf("Lost = %v, want none", got.Lost)
		}
	})

	t.Run("a tint without a colour is reported", func(t *testing.T) {
		got, err := parseBlackAndWhite(blackAndWhiteDescriptor(t, true, nil))
		if err != nil {
			t.Fatalf("parseBlackAndWhite: %v", err)
		}
		if len(got.Lost) != 1 || !strings.Contains(got.Lost[0], "tint colour") {
			t.Fatalf("Lost = %v, want one entry naming the missing tint colour", got.Lost)
		}
	})

	t.Run("no tint means no colour to look for", func(t *testing.T) {
		got, err := parseBlackAndWhite(blackAndWhiteDescriptor(t, false, nil))
		if err != nil {
			t.Fatalf("parseBlackAndWhite: %v", err)
		}
		if len(got.Lost) != 0 {
			t.Errorf("Lost = %v, want none when the file says there is no tint", got.Lost)
		}
	})
}

// blackAndWhiteDescriptor builds a blwh payload: the six mixes, useTint, and
// optionally an RGBC tint colour object.
func blackAndWhiteDescriptor(t *testing.T, useTint bool, tint *[3]float64) []byte {
	t.Helper()
	mixes := []struct {
		key   string
		value int
	}{
		{"Rd  ", 40},
		{"Yllw", 60},
		{"Grn ", 40},
		{"Cyn ", 60},
		{"Bl  ", 20},
		{"Mgnt", 80},
	}
	count := len(mixes) + 1
	if tint != nil {
		count++
	}

	out := binary.BigEndian.AppendUint32(nil, 16)
	out = append(out, descriptorUnicode("")...)
	out = append(out, descriptorKey("null")...)
	out = binary.BigEndian.AppendUint32(out, uint32(count)) //nolint:gosec // test data
	for _, mix := range mixes {
		out = append(out, descriptorKey(mix.key)...)
		out = append(out, []byte("long")...)
		out = binary.BigEndian.AppendUint32(out, uint32(mix.value)) //nolint:gosec // test data
	}
	out = append(out, descriptorKey("useTint")...)
	out = append(out, []byte("bool")...)
	if useTint {
		out = append(out, 1)
	} else {
		out = append(out, 0)
	}
	if tint != nil {
		out = append(out, descriptorKey("tintColor")...)
		out = append(out, []byte("Objc")...)
		out = append(out, descriptorUnicode("")...)
		out = append(out, descriptorKey("RGBC")...)
		out = binary.BigEndian.AppendUint32(out, 3)
		for index, component := range [3]string{"Rd  ", "Grn ", "Bl  "} {
			out = append(out, descriptorKey(component)...)
			out = append(out, []byte("doub")...)
			out = binary.BigEndian.AppendUint64(out, math.Float64bits(tint[index]))
		}
	}
	return out
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
