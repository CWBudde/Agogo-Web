package abr

import "testing"

func FuzzParse(f *testing.F) {
	f.Add(fixtureABR(6, 1, fixtureBlock("samp", fixtureSampleRecord(1, 2, 2, 0, []byte{0, 1, 2, 3}))))
	f.Add(fixtureABR(7, 2, fixtureBlock("samp", fixtureSampleRecord(2, 3, 1, 1, []byte{0, 2, 1, 2, 3}))))
	f.Add(fixtureABR(6, 1, fixtureBlock("desc", fixtureDescriptor())))
	f.Fuzz(func(t *testing.T, data []byte) {
		lib, err := ParseWithLimits(data, Limits{
			MaxFileBytes:       1 << 20,
			MaxSectionBytes:    1 << 20,
			MaxRecords:         128,
			MaxDimension:       1024,
			MaxPixels:          1 << 20,
			MaxDescriptorDepth: 16,
			MaxDescriptorItems: 4096,
			MaxStringBytes:     64 << 10,
			MaxDataBytes:       64 << 10,
		})
		if err != nil && lib != nil {
			t.Fatalf("ParseWithLimits returned partial result on error: %#v, %v", lib, err)
		}
	})
}

func FuzzDecodePackBitsRow(f *testing.F) {
	f.Add([]byte{3, 1, 2, 3, 4}, uint8(4))
	f.Add([]byte{0xfc, 9}, uint8(5))
	f.Add([]byte{0x80, 0, 7}, uint8(1))
	f.Fuzz(func(t *testing.T, src []byte, width uint8) {
		dst := make([]byte, int(width))
		_ = decodePackBitsRow(src, dst)
	})
}
