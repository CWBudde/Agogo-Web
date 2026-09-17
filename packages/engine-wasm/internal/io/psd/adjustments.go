package psd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/descriptor"
)

// Adjustment tagged blocks, reconstructed into live engine adjustment layers.
//
// Every parser here works on a payload that has already been read whole, so
// there is no length field to trust: the only bound that matters is the slice
// it is handed. Each one therefore states the exact size the spec gives and
// refuses anything shorter, rather than reading until it runs out — a file that
// declares a structure it does not contain is malformed, not partially valid.
//
// A parser returns an error only for a payload it can diagnose. The caller
// records that as a MetadataWarning and keeps going, because one unreadable
// adjustment block must not cost the rest of the layer.

const (
	// levelsRecordCount is fixed by the format: a levl block always carries 29
	// records, whatever the document's channel count.
	levelsRecordCount = 29
	levelsRecordSize  = 10
	// hueSatRangeCount is likewise fixed: reds, yellows, greens, cyans, blues,
	// magentas, in that order.
	hueSatRangeCount = 6
	// selectiveColorPlateCount counts the unused leading plate.
	selectiveColorPlateCount = 10
)

// hueSatBucketNames and selectiveColorBucketNames are the engine's JSON keys,
// in the order the blocks store them.
var (
	hueSatBucketNames = [hueSatRangeCount]string{
		"reds", "yellows", "greens", "cyans", "blues", "magentas",
	}
	selectiveColorBucketNames = [selectiveColorPlateCount - 1]string{
		"reds", "yellows", "greens", "cyans", "blues", "magentas",
		"whites", "neutrals", "blacks",
	}
	// curveChannelKeys maps the curv channel bitmap onto the engine's per-
	// channel point lists. Bit 0 is the composite curve.
	curveChannelKeys = [4]string{"points", "redPoints", "greenPoints", "bluePoints"}
)

// setAdjustment records a reconstructed adjustment on the record. A record
// carrying two adjustment blocks keeps the first that produced a payload, which
// is what makes the brit/CgEd pair below work: CgEd is preferred explicitly
// rather than by block order.
func setAdjustment(record *LayerRecord, payload *AdjustmentPayload) {
	if payload == nil {
		return
	}
	record.Adjustment = payload
}

// parseLayerAdjustmentBlock reconstructs one adjustment block. It is called
// after parseLayerAdjustmentMetadata, which keeps the block-level description
// for every key including the ones that reconstruct.
func parseLayerAdjustmentBlock(key string, payload []byte, record *LayerRecord) error {
	var (
		result *AdjustmentPayload
		err    error
	)
	switch key {
	case "levl":
		result, err = parseLevels(payload)
	case "curv":
		result, err = parseCurves(payload)
	case "hue2":
		result, err = parseHueSaturation(payload)
	case "blnc":
		result, err = parseColorBalance(payload)
	case "mixr":
		result, err = parseChannelMixer(payload)
	case "selc":
		result, err = parseSelectiveColor(payload)
	case "thrs":
		result, err = parseThreshold(payload)
	case "post":
		result, err = parsePosterize(payload)
	case "nvrt":
		result, err = parseInvert(payload)
	case "phfl":
		result, err = parsePhotoFilter(payload)
	case "blwh":
		result, err = parseBlackAndWhite(payload)
	case "CgEd":
		result, err = parseBrightnessContrast(payload)
	case "brit":
		// The obsolete block. Photoshop writes it alongside CgEd for readers
		// that predate the descriptor, and the two disagree whenever the layer
		// has been edited since. CgEd wins, so this is only used when no CgEd
		// block appeared on the same record.
		result, err = parseLegacyBrightnessContrast(payload)
		if err == nil && record.Adjustment != nil &&
			record.Adjustment.Kind == adjustmentKindBrightnessContrast {
			return nil
		}
	default:
		return nil
	}
	if err != nil {
		return err
	}
	setAdjustment(record, result)
	return nil
}

const adjustmentKindBrightnessContrast = "brightness-contrast"

// ── binary blocks ─────────────────────────────────────────────────────────────

// parseLevels reads a levl block: version 2, then 29 records of five uint16.
//
// The engine holds ONE set of level values plus a channel selector, so only the
// composite record (index 0) has anywhere to go. The per-channel records are
// reported as lost rather than silently discarded or averaged in.
func parseLevels(payload []byte) (*AdjustmentPayload, error) {
	const want = 2 + levelsRecordCount*levelsRecordSize
	if len(payload) < want {
		return nil, fmt.Errorf("levl needs %d bytes, got %d", want, len(payload))
	}
	if version := binary.BigEndian.Uint16(payload[:2]); version != 2 {
		return nil, fmt.Errorf("levl version %d is not 2", version)
	}

	record := func(index int) [5]uint16 {
		at := 2 + index*levelsRecordSize
		var out [5]uint16
		for field := range out {
			out[field] = binary.BigEndian.Uint16(payload[at+field*2 : at+field*2+2])
		}
		return out
	}

	composite := record(0)
	params := map[string]any{
		"channel":     "rgb",
		"inputBlack":  int(composite[0]),
		"inputWhite":  int(composite[1]),
		"outputBlack": int(composite[2]),
		"outputWhite": int(composite[3]),
		// A short from 10..999 standing for 0.1..9.99.
		"gamma": roundTo(float64(composite[4])/100, 4),
	}

	var lost []string
	for index := 1; index < levelsRecordCount; index++ {
		if !isIdentityLevelRecord(record(index)) {
			lost = append(lost, "levl per-channel level records")
			break
		}
	}
	return adjustment("levels", params, lost)
}

// isIdentityLevelRecord reports whether a level record leaves the channel
// untouched. Photoshop fills the unused records with zeroes rather than with
// identity values, so an all-zero record counts as identity too.
func isIdentityLevelRecord(record [5]uint16) bool {
	if record == [5]uint16{} {
		return true
	}
	return record == [5]uint16{0, 255, 0, 255, 100}
}

// parseCurves reads a curv block: an is-map flag, a version, a channel bitmap
// or count, then the curves themselves.
//
// Only version 1 is reconstructed. Version 4 replaces the bitmap with a bare
// count and so does not say which channel each curve belongs to; guessing would
// put a red curve on the composite.
func parseCurves(payload []byte) (*AdjustmentPayload, error) {
	const headerSize = 1 + 2 + 4
	if len(payload) < headerSize {
		return nil, fmt.Errorf("curv needs %d header bytes, got %d", headerSize, len(payload))
	}
	isMap := payload[0] != 0
	version := binary.BigEndian.Uint16(payload[1:3])
	bitmap := binary.BigEndian.Uint32(payload[3:7])
	if version != 1 && version != 4 {
		return nil, fmt.Errorf("curv version %d is not 1 or 4", version)
	}
	if isMap {
		return nil, fmt.Errorf("curv stores a 256-entry lookup map, which has no engine equivalent")
	}
	if version != 1 {
		return nil, fmt.Errorf("curv version 4 does not identify its channels")
	}

	reader := bytes.NewReader(payload[headerSize:])
	params := map[string]any{}
	var lost []string
	for slot := range 32 {
		if bitmap&(1<<uint(slot)) == 0 {
			continue
		}
		points, err := readCurvePoints(reader)
		if err != nil {
			return nil, fmt.Errorf("curv channel %d: %w", slot, err)
		}
		if slot >= len(curveChannelKeys) {
			lost = append(lost, fmt.Sprintf("curv curve for channel %d", slot))
			continue
		}
		params[curveChannelKeys[slot]] = points
	}
	if len(params) == 0 {
		return nil, fmt.Errorf("curv carries no curves")
	}
	return adjustment("curves", params, lost)
}

// curvePoint mirrors the engine's JSON shape. PSD stores a point as
// (output, input); the engine stores it as {x: input, y: output}.
type curvePoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func readCurvePoints(reader *bytes.Reader) ([]curvePoint, error) {
	count, err := readUint16From(reader)
	if err != nil {
		return nil, err
	}
	// The format caps a curve at 19 points. The cap is what bounds the
	// allocation: without it a two-byte count reserves 64k points per channel.
	if count < 2 || count > 19 {
		return nil, fmt.Errorf("point count %d is not in [2, 19]", count)
	}
	points := make([]curvePoint, 0, count)
	for range count {
		y, err := readUint16From(reader)
		if err != nil {
			return nil, err
		}
		x, err := readUint16From(reader)
		if err != nil {
			return nil, err
		}
		points = append(points, curvePoint{X: int(x), Y: int(y)})
	}
	return points, nil
}

// parseHueSaturation reads a hue2 block: version 2, an enable byte and a pad,
// the colorization triple, the master triple, then six ranges of four hue edges
// and three settings.
func parseHueSaturation(payload []byte) (*AdjustmentPayload, error) {
	const want = 2 + 1 + 1 + 3*2 + 3*2 + hueSatRangeCount*(4*2+3*2)
	if len(payload) < want {
		return nil, fmt.Errorf("hue2 needs %d bytes, got %d", want, len(payload))
	}
	if version := binary.BigEndian.Uint16(payload[:2]); version != 2 {
		return nil, fmt.Errorf("hue2 version %d is not 2", version)
	}
	reader := bytes.NewReader(payload[2:])
	enable, _ := reader.ReadByte()
	_, _ = reader.ReadByte() // pad

	if _, err := readInt16Triple(reader); err != nil { // colorization
		return nil, err
	}
	master, err := readInt16Triple(reader)
	if err != nil {
		return nil, err
	}

	params := map[string]any{
		"hueShift":   int(master[0]),
		"saturation": int(master[1]),
		"lightness":  int(master[2]),
		// Photoshop stores colorize as "not enabled": the master triple means
		// colorize values when the enable byte is 0.
		"colorize": enable == 0,
	}

	var lost []string
	edgesCarried := false
	for bucket := range hueSatRangeCount {
		edges, err := readInt16Quad(reader)
		if err != nil {
			return nil, err
		}
		settings, err := readInt16Triple(reader)
		if err != nil {
			return nil, err
		}
		if !isDefaultHueEdges(bucket, edges) {
			edgesCarried = true
		}
		if settings == [3]int16{} {
			continue
		}
		params[hueSatBucketNames[bucket]] = map[string]any{
			"hueShift":   int(settings[0]),
			"saturation": int(settings[1]),
			"lightness":  int(settings[2]),
		}
	}
	if edgesCarried {
		// The engine's six buckets are fixed cosine-weighted sectors; it has no
		// field for Photoshop's four movable hue edges.
		lost = append(lost, "hue2 per-range hue edges")
	}
	return adjustment("huesat", params, lost)
}

// isDefaultHueEdges reports whether a range's four hue edges are the untouched
// defaults for that bucket: the sector centred on bucket*60 degrees, with a
// 30-degree falloff on each side.
func isDefaultHueEdges(bucket int, edges [4]int16) bool {
	if edges == [4]int16{} {
		return true
	}
	centre := bucket * 60
	want := [4]int16{
		int16(mod360(centre - 45)),
		int16(mod360(centre - 15)),
		int16(mod360(centre + 15)),
		int16(mod360(centre + 45)),
	}
	return edges == want
}

func mod360(value int) int {
	return ((value % 360) + 360) % 360
}

// parseColorBalance reads a blnc block: three tone triples of int16 followed by
// the preserve-luminosity byte.
func parseColorBalance(payload []byte) (*AdjustmentPayload, error) {
	const want = 3*3*2 + 1
	if len(payload) < want {
		return nil, fmt.Errorf("blnc needs %d bytes, got %d", want, len(payload))
	}
	reader := bytes.NewReader(payload)
	tones := make(map[string]any, 3)
	for _, name := range []string{"shadows", "midtones", "highlights"} {
		values, err := readInt16Triple(reader)
		if err != nil {
			return nil, err
		}
		tones[name] = map[string]any{
			"cyanRed":      int(values[0]),
			"magentaGreen": int(values[1]),
			"yellowBlue":   int(values[2]),
		}
	}
	luminosity, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	tones["preserveLuminosity"] = luminosity != 0
	return adjustment("color-balance", tones, nil)
}

// parseChannelMixer reads a mixr block: version, the monochrome flag, then five
// int16 for one output row — four source weights in percent and a constant.
//
// The engine has a row per output channel and no constant term at all, so the
// constant is reported as lost. Photoshop repeats the five-value group per
// output channel; anything past the first group is reported too rather than
// guessed at, because the repeat count is not in the block.
func parseChannelMixer(payload []byte) (*AdjustmentPayload, error) {
	const want = 2 + 2 + 5*2
	if len(payload) < want {
		return nil, fmt.Errorf("mixr needs %d bytes, got %d", want, len(payload))
	}
	if version := binary.BigEndian.Uint16(payload[:2]); version != 1 {
		return nil, fmt.Errorf("mixr version %d is not 1", version)
	}
	monochrome := binary.BigEndian.Uint16(payload[2:4]) != 0
	values := make([]int16, 5)
	for index := range values {
		at := 4 + index*2
		values[index] = int16(binary.BigEndian.Uint16(payload[at : at+2]))
	}

	params := map[string]any{
		"monochrome": monochrome,
		"red":        []int{int(values[0]), int(values[1]), int(values[2])},
	}
	var lost []string
	if values[4] != 0 {
		lost = append(lost, "mixr constant term")
	}
	if len(payload) > want {
		lost = append(lost, "mixr rows past the first output channel")
	}
	return adjustment("channel-mixer", params, lost)
}

// parseSelectiveColor reads a selc block: version, the relative/absolute
// method, then ten CMYK plates of int16. The first plate is unused.
func parseSelectiveColor(payload []byte) (*AdjustmentPayload, error) {
	const want = 2 + 2 + selectiveColorPlateCount*4*2
	if len(payload) < want {
		return nil, fmt.Errorf("selc needs %d bytes, got %d", want, len(payload))
	}
	if version := binary.BigEndian.Uint16(payload[:2]); version != 1 {
		return nil, fmt.Errorf("selc version %d is not 1", version)
	}
	mode := "relative"
	if binary.BigEndian.Uint16(payload[2:4]) != 0 {
		mode = "absolute"
	}
	params := map[string]any{"mode": mode}
	for plate := 1; plate < selectiveColorPlateCount; plate++ {
		at := 4 + plate*8
		var values [4]int16
		for field := range values {
			values[field] = int16(binary.BigEndian.Uint16(payload[at+field*2 : at+field*2+2]))
		}
		if values == [4]int16{} {
			continue
		}
		params[selectiveColorBucketNames[plate-1]] = map[string]any{
			"cyanRed":      int(values[0]),
			"magentaGreen": int(values[1]),
			"yellowBlue":   int(values[2]),
			"black":        int(values[3]),
		}
	}
	return adjustment("selective-color", params, nil)
}

// parseThreshold reads a thrs block: one uint16 level, padded to four bytes.
func parseThreshold(payload []byte) (*AdjustmentPayload, error) {
	level, err := readPaddedShort(payload, "thrs")
	if err != nil {
		return nil, err
	}
	return adjustment("threshold", map[string]any{"threshold": int(level)}, nil)
}

// parsePosterize reads a post block: one uint16 level count, padded to four.
func parsePosterize(payload []byte) (*AdjustmentPayload, error) {
	levels, err := readPaddedShort(payload, "post")
	if err != nil {
		return nil, err
	}
	return adjustment("posterize", map[string]any{"levels": int(levels)}, nil)
}

// parseInvert reads an nvrt block, which has no payload: Invert has nothing to
// configure. A non-empty payload is not an error — it is a newer writer with
// something to say that this reader does not need.
func parseInvert(_ []byte) (*AdjustmentPayload, error) {
	return adjustment("invert", map[string]any{}, nil)
}

// parsePhotoFilter reads a phfl block. Version 2 stores a colour space and four
// 16-bit components; version 3 stores XYZ instead and has no engine equivalent.
func parsePhotoFilter(payload []byte) (*AdjustmentPayload, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("phfl needs at least 2 bytes, got %d", len(payload))
	}
	version := binary.BigEndian.Uint16(payload[:2])
	if version == 3 {
		return nil, fmt.Errorf("phfl version 3 stores an XYZ colour, which the engine cannot represent")
	}
	if version != 2 {
		return nil, fmt.Errorf("phfl version %d is not 2 or 3", version)
	}
	const want = 2 + 2 + 4*2 + 4 + 1
	if len(payload) < want {
		return nil, fmt.Errorf("phfl needs %d bytes, got %d", want, len(payload))
	}

	colorSpace := binary.BigEndian.Uint16(payload[2:4])
	var components [4]uint16
	for index := range components {
		at := 4 + index*2
		components[index] = binary.BigEndian.Uint16(payload[at : at+2])
	}
	density := binary.BigEndian.Uint32(payload[12:16])
	luminosity := payload[16]

	var lost []string
	if colorSpace != 0 {
		// Space 0 is RGB. Anything else would need a colour conversion the
		// engine does not have, so the filter colour cannot be trusted.
		lost = append(lost, fmt.Sprintf("phfl colour space %d (not RGB)", colorSpace))
	}
	params := map[string]any{
		"color": []int{
			int(components[0] / 257),
			int(components[1] / 257),
			int(components[2] / 257),
			255,
		},
		"density":            int(density),
		"preserveLuminosity": luminosity != 0,
	}
	return adjustment("photo-filter", params, lost)
}

// readPaddedShort reads the single uint16 that thrs and post carry. Photoshop
// pads the payload to four bytes; two is accepted because the pad is padding.
func readPaddedShort(payload []byte, key string) (uint16, error) {
	if len(payload) < 2 {
		return 0, fmt.Errorf("%s needs at least 2 bytes, got %d", key, len(payload))
	}
	return binary.BigEndian.Uint16(payload[:2]), nil
}

func readInt16Triple(reader *bytes.Reader) ([3]int16, error) {
	var out [3]int16
	for index := range out {
		value, err := readUint16From(reader)
		if err != nil {
			return out, err
		}
		out[index] = int16(value)
	}
	return out, nil
}

func readInt16Quad(reader *bytes.Reader) ([4]int16, error) {
	var out [4]int16
	for index := range out {
		value, err := readUint16From(reader)
		if err != nil {
			return out, err
		}
		out[index] = int16(value)
	}
	return out, nil
}

// ── descriptor blocks ─────────────────────────────────────────────────────────

// parseAdjustmentDescriptor reads the uint32 version that precedes a
// descriptor-valued adjustment block, then the descriptor itself.
func parseAdjustmentDescriptor(payload []byte, key string) (descriptor.Descriptor, error) {
	if len(payload) < 4 {
		return descriptor.Descriptor{}, fmt.Errorf("%s needs at least 4 bytes, got %d", key, len(payload))
	}
	if version := binary.BigEndian.Uint32(payload[:4]); version != 16 {
		return descriptor.Descriptor{}, fmt.Errorf("%s descriptor version %d is not 16", key, version)
	}
	parsed, _, err := descriptor.Parse(payload[4:], descriptorLimits())
	if err != nil {
		return descriptor.Descriptor{}, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

// parseBlackAndWhite reads a blwh block: six long percentages plus the tint.
func parseBlackAndWhite(payload []byte) (*AdjustmentPayload, error) {
	parsed, err := parseAdjustmentDescriptor(payload, "blwh")
	if err != nil {
		return nil, err
	}
	params := map[string]any{}
	for key, name := range map[string]string{
		"Rd  ": "reds",
		"Yllw": "yellows",
		"Grn ": "greens",
		"Cyn ": "cyans",
		"Bl  ": "blues",
		"Mgnt": "magentas",
	} {
		if value, ok := parsed.Get(key); ok && value.Type == descriptor.TypeInteger {
			params[name] = int(value.Integer)
		}
	}
	if len(params) == 0 {
		return nil, fmt.Errorf("blwh carries none of the six colour mixes")
	}
	// Recorded whenever the descriptor carries it, including when it is false.
	// The engine treats an absent key and an explicit false alike, but the file
	// distinguishes "this writer said no tint" from "this writer said nothing",
	// and the importer's job is to report what the file says.
	if _, ok := parsed.Get("useTint"); ok {
		params["tint"] = parsed.Bool("useTint", false)
	}
	return adjustment("black-white", params, nil)
}

// parseBrightnessContrast reads a CgEd block, the descriptor form Photoshop
// writes today.
func parseBrightnessContrast(payload []byte) (*AdjustmentPayload, error) {
	parsed, err := parseAdjustmentDescriptor(payload, "CgEd")
	if err != nil {
		return nil, err
	}
	brightness, hasBrightness := parsed.Get("Brgh")
	contrast, hasContrast := parsed.Get("Cntr")
	if !hasBrightness && !hasContrast {
		return nil, fmt.Errorf("CgEd carries neither Brgh nor Cntr")
	}
	params := map[string]any{
		"brightness": int(brightness.Integer),
		"contrast":   int(contrast.Integer),
	}
	if _, ok := parsed.Get("useLegacy"); ok {
		params["legacy"] = parsed.Bool("useLegacy", false)
	}
	return adjustment(adjustmentKindBrightnessContrast, params, nil)
}

// parseLegacyBrightnessContrast reads the obsolete brit block: brightness,
// contrast and mean as uint16, then the Lab-only byte.
func parseLegacyBrightnessContrast(payload []byte) (*AdjustmentPayload, error) {
	const want = 3*2 + 1
	if len(payload) < want {
		return nil, fmt.Errorf("brit needs %d bytes, got %d", want, len(payload))
	}
	brightness := int16(binary.BigEndian.Uint16(payload[:2]))
	contrast := int16(binary.BigEndian.Uint16(payload[2:4]))
	params := map[string]any{
		"brightness": int(brightness),
		"contrast":   int(contrast),
		// The legacy block predates the non-legacy algorithm, so a file that
		// has only this block is describing the legacy one.
		"legacy": true,
	}
	return adjustment(adjustmentKindBrightnessContrast, params, nil)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// adjustment marshals the parameter object once, here, so that every parser
// above can work in plain Go maps and none of them has to think about JSON.
func adjustment(kind string, params map[string]any, lost []string) (*AdjustmentPayload, error) {
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("%s parameters: %w", kind, err)
	}
	return &AdjustmentPayload{Kind: kind, Params: encoded, Lost: lost}, nil
}

// roundTo trims a converted value to a fixed number of decimal places, so that
// a gamma of 120/100 is 1.2 and not 1.2000000000000002.
func roundTo(value float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(value*scale) / scale
}
