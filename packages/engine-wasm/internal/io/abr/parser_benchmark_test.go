package abr

import "testing"

func BenchmarkParseLargeSyntheticLibrary(b *testing.B) {
	const tipCount = 256
	pixels := make([]byte, 64*64)
	blocks := make([][]byte, 0, tipCount)
	for index := range tipCount {
		for pixel := range pixels {
			pixels[pixel] = byte(index + pixel)
		}
		blocks = append(blocks, fixtureBlock("samp", fixtureSampleRecord(1, 64, 64, 0, pixels)))
	}
	data := fixtureABR(6, 1, blocks...)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		if _, err := Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}
