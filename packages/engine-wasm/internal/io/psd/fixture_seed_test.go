package psd

import (
	"encoding/binary"
	"strconv"
	"sync"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdfixture"
)

// maxFuzzSeedBytes caps what goes into the seed corpus. f.Add seeds run on every
// plain `go test`, and oversized seeds slow mutation during `-fuzz` without
// covering more parser surface.
const maxFuzzSeedBytes = 64 << 10

var (
	fixtureSeedOnce  sync.Once
	fixtureSeedFiles []psdfixture.Fixture
	fixtureSeedErr   error
)

// fuzzSeedFixtures returns the fixtures that opted into seeding via
// "fuzz": {"seed": true} and fit under maxFuzzSeedBytes. It is computed once per
// test binary because every Fuzz* target calls it.
func fuzzSeedFixtures(tb testing.TB) []psdfixture.Fixture {
	tb.Helper()
	fixtureSeedOnce.Do(func() {
		all, err := psdfixture.Load()
		if err != nil {
			fixtureSeedErr = err
			return
		}
		for _, fixture := range all {
			// A sidecar that omits "fuzz" opts out; only an explicit
			// {"seed": true} puts a fixture into the seed corpus.
			if fixture.Spec.Fuzz == nil || !fixture.Spec.Fuzz.Seed {
				continue
			}
			if len(fixture.Data) > maxFuzzSeedBytes {
				continue
			}
			fixtureSeedFiles = append(fixtureSeedFiles, fixture)
		}
	})
	if fixtureSeedErr != nil {
		tb.Fatalf("load fixture corpus for fuzz seeds: %v", fixtureSeedErr)
	}
	return fixtureSeedFiles
}

// addWholeFileSeeds seeds targets that consume a complete PSD/PSB stream.
func addWholeFileSeeds(f *testing.F) {
	f.Helper()
	for _, fixture := range fuzzSeedFixtures(f) {
		f.Add(fixture.Data)
	}
}

// addSectionSeeds seeds targets that consume one section of a stream. extract is
// best-effort by design: a fixture that does not contain the section is skipped
// rather than failing, so the corpus can grow into a target over time.
func addSectionSeeds(f *testing.F, extract func([]byte) ([]byte, bool)) {
	f.Helper()
	for _, fixture := range fuzzSeedFixtures(f) {
		if section, ok := extract(fixture.Data); ok && len(section) > 0 {
			f.Add(section)
		}
	}
}

// addPackBitsRowSeeds seeds FuzzDecodePackBits from the real RLE scanlines in
// the corpus. Each seed is a packed row plus the decoded byte width that row
// claims, because the target takes both.
//
// A seed is not required to decode cleanly inside the target — the target folds
// the width down with %4096, so a wide row such as near-psd-limit's 30000-byte
// scanline is presented as a long run against a short output buffer. That is a
// useful input, not a broken seed.
func addPackBitsRowSeeds(f *testing.F) {
	f.Helper()
	seen := make(map[string]struct{})
	for _, fixture := range fuzzSeedFixtures(f) {
		rows, ok := extractRLERows(fixture.Data)
		if !ok {
			continue
		}
		for _, row := range rows {
			if len(row.packed) == 0 || len(row.packed) > maxFuzzSeedBytes {
				continue
			}
			key := string(row.packed) + "\x00" + strconv.Itoa(int(row.decodedLen))
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			f.Add(row.packed, row.decodedLen)
		}
	}
}

// packBitsRow is one RLE scanline: the packed bytes and the number of bytes it
// decodes to.
type packBitsRow struct {
	packed     []byte
	decodedLen uint16
}

// psdSections is a byte-level split of a PSD/PSB stream. It deliberately does
// not reuse Parser: a fuzz seed must remain extractable from a file the parser
// itself rejects, and reusing the parser here would silently drop such a file.
type psdSections struct {
	psb bool
	// width, height, channels and depth come straight from the header. The RLE
	// row table is sized by channels*height, and a row's decoded length by
	// width*depth/8, so extraction needs all four.
	width             int
	height            int
	channels          int
	depth             int
	colorMode         []byte
	resources         []byte
	layerAndMask      []byte
	layerAndMaskStart int
	layerAndMaskEnd   int
	imageData         []byte
}

const psdHeaderLen = 26

// splitSections slices a stream into its four top-level sections. ok is false for
// anything that is not a plausible 8BPS container.
func splitSections(data []byte) (psdSections, bool) {
	var out psdSections
	if len(data) < psdHeaderLen || string(data[:4]) != "8BPS" {
		return out, false
	}
	out.psb = binary.BigEndian.Uint16(data[4:6]) == 2
	out.channels = int(binary.BigEndian.Uint16(data[12:14]))
	out.height = int(binary.BigEndian.Uint32(data[14:18]))
	out.width = int(binary.BigEndian.Uint32(data[18:22]))
	out.depth = int(binary.BigEndian.Uint16(data[22:24]))

	offset := psdHeaderLen
	section := func(lengthBytes int) ([]byte, int, bool) {
		start := offset
		if offset+lengthBytes > len(data) {
			return nil, start, false
		}
		var length uint64
		if lengthBytes == 8 {
			length = binary.BigEndian.Uint64(data[offset : offset+8])
		} else {
			length = uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		}
		offset += lengthBytes
		if length > uint64(len(data)-offset) {
			return nil, start, false
		}
		payload := data[offset : offset+int(length)]
		offset += int(length)
		return payload, start, true
	}

	var ok bool
	if out.colorMode, _, ok = section(4); !ok {
		return out, false
	}
	if out.resources, _, ok = section(4); !ok {
		return out, false
	}
	layerLengthBytes := 4
	if out.psb {
		layerLengthBytes = 8
	}
	if out.layerAndMask, out.layerAndMaskStart, ok = section(layerLengthBytes); !ok {
		return out, false
	}
	out.layerAndMaskEnd = offset
	out.imageData = data[offset:]
	return out, true
}

// extractLayerAndMaskSection returns the layer-and-mask section INCLUDING its
// length prefix, because ParseLayerAndMaskInfo reads that length itself.
func extractLayerAndMaskSection(data []byte) ([]byte, bool) {
	sections, ok := splitSections(data)
	if !ok || len(sections.layerAndMask) == 0 {
		return nil, false
	}
	return data[sections.layerAndMaskStart:sections.layerAndMaskEnd], true
}

// extractCompositeSection returns the trailing image-data section, which starts
// with the 2-byte compression id ParseCompositeImageData expects.
func extractCompositeSection(data []byte) ([]byte, bool) {
	sections, ok := splitSections(data)
	if !ok || len(sections.imageData) == 0 {
		return nil, false
	}
	return sections.imageData, true
}

// extractRLERows returns the first packed scanline of each channel of an
// RLE-compressed composite, paired with the byte length that row decodes to.
//
// Layout after the 2-byte compression id: channels*height row byte counts
// (uint16 for PSD, uint32 for PSB), then the packed rows back to back in the
// same order. Only composites with compression id 1 yield rows; everything
// else — RAW, ZIP, a short section, a header this function cannot trust —
// returns false, in keeping with the best-effort contract of addSectionSeeds.
func extractRLERows(data []byte) ([]packBitsRow, bool) {
	sections, ok := splitSections(data)
	if !ok || len(sections.imageData) < 2 {
		return nil, false
	}
	if binary.BigEndian.Uint16(sections.imageData[:2]) != CompressionRLE {
		return nil, false
	}
	if sections.channels <= 0 || sections.height <= 0 || sections.width <= 0 {
		return nil, false
	}
	// Decoded bytes per row. The corpus is 8- and 16-bit; anything else is not
	// a whole number of bytes per sample and is left to the parser to reject.
	if sections.depth != 8 && sections.depth != 16 {
		return nil, false
	}
	decodedLen := sections.width * (sections.depth / 8)
	if decodedLen > int(^uint16(0)) {
		return nil, false
	}

	countWidth := 2
	if sections.psb {
		countWidth = 4
	}
	rowCount := sections.channels * sections.height
	tableEnd := 2 + rowCount*countWidth
	if tableEnd < 2 || tableEnd > len(sections.imageData) {
		return nil, false
	}

	counts := make([]int, rowCount)
	for i := range counts {
		offset := 2 + i*countWidth
		if countWidth == 2 {
			counts[i] = int(binary.BigEndian.Uint16(sections.imageData[offset : offset+2]))
		} else {
			counts[i] = int(binary.BigEndian.Uint32(sections.imageData[offset : offset+4]))
		}
	}

	// Walk every row so the offsets stay exact, but keep only the first row of
	// each channel: the planes differ from one another, while rows within a
	// plane are near-identical and would only pad the seed corpus.
	rows := make([]packBitsRow, 0, sections.channels)
	offset := tableEnd
	for i, count := range counts {
		if count < 0 || count > len(sections.imageData)-offset {
			return nil, false
		}
		if i%sections.height == 0 {
			rows = append(rows, packBitsRow{
				packed:     sections.imageData[offset : offset+count],
				decodedLen: uint16(decodedLen),
			})
		}
		offset += count
	}
	return rows, len(rows) > 0
}

// extractFirstLayerExtraData returns the extra-data block of the first layer
// record, which is what ParseLayerExtraData consumes.
func extractFirstLayerExtraData(data []byte) ([]byte, bool) {
	sections, ok := splitSections(data)
	if !ok || len(sections.layerAndMask) == 0 {
		return nil, false
	}
	return firstLayerExtraData(sections.layerAndMask, sections.psb)
}

// firstLayerExtraData walks just far enough into the layer info to reach the
// first record's variable-length extra data. Every bound is checked against the
// remaining slice, because this runs on files the parser may reject.
func firstLayerExtraData(layerAndMask []byte, psb bool) ([]byte, bool) {
	cursor := 0
	readUint32 := func() (uint32, bool) {
		if cursor+4 > len(layerAndMask) {
			return 0, false
		}
		value := binary.BigEndian.Uint32(layerAndMask[cursor : cursor+4])
		cursor += 4
		return value, true
	}

	// Layer info length: 4 bytes for PSD, 8 for PSB.
	if psb {
		if cursor+8 > len(layerAndMask) {
			return nil, false
		}
		cursor += 8
	} else if _, ok := readUint32(); !ok {
		return nil, false
	}

	// Layer count is signed; a negative count means the first alpha channel
	// holds the merged-result transparency, which does not change the layout.
	if cursor+2 > len(layerAndMask) {
		return nil, false
	}
	count := int16(binary.BigEndian.Uint16(layerAndMask[cursor : cursor+2]))
	cursor += 2
	if count == 0 {
		return nil, false
	}

	// Bounds (4 x int32) then the channel table.
	cursor += 16
	if cursor+2 > len(layerAndMask) {
		return nil, false
	}
	channelCount := int(binary.BigEndian.Uint16(layerAndMask[cursor : cursor+2]))
	cursor += 2
	channelEntry := 6 // int16 id + uint32 length
	if psb {
		channelEntry = 10 // int16 id + uint64 length
	}
	cursor += channelCount * channelEntry

	// "8BIM" + blend key + opacity + clipping + flags + filler.
	cursor += 12
	extraLen, ok := readUint32()
	if !ok || uint64(extraLen) > uint64(len(layerAndMask)-cursor) {
		return nil, false
	}
	return layerAndMask[cursor : cursor+int(extraLen)], true
}
