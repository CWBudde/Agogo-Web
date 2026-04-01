package engine

import (
	"bytes"
	"testing"

	agglib "github.com/MeKo-Christian/agg_go"
)

func makeHighFrequencyFixturePixels() []byte {
	buf := make([]byte, 4*4*4)
	colors := [][4]byte{
		{255, 0, 0, 255},
		{0, 255, 0, 255},
		{0, 0, 255, 255},
		{255, 255, 0, 255},
	}
	for y := range 4 {
		for x := range 4 {
			color := colors[(x+y)%len(colors)]
			i := (y*4 + x) * 4
			buf[i] = color[0]
			buf[i+1] = color[1]
			buf[i+2] = color[2]
			buf[i+3] = color[3]
		}
	}
	return buf
}

func renderDirectAGGSimple(t *testing.T, pixels []byte, filter agglib.ImageFilter, resample agglib.ImageResample, dstX1, dstY1, dstX2, dstY2 float64) []byte {
	t.Helper()
	buf := make([]byte, 8*8*4)
	renderer := agglib.NewAgg2D()
	renderer.Attach(buf, 8, 8, 8*4)
	renderer.ImageResample(resample)
	renderer.ImageFilter(filter)
	src := agglib.NewImage(pixels, 4, 4, 4*4)
	if err := renderer.TransformImageSimple(src, dstX1, dstY1, dstX2, dstY2); err != nil {
		t.Fatalf("TransformImageSimple: %v", err)
	}
	return buf
}

func renderDirectAGGParallelogram(t *testing.T, pixels []byte, filter agglib.ImageFilter, resample agglib.ImageResample, parallelogram []float64) []byte {
	t.Helper()
	buf := make([]byte, 8*8*4)
	renderer := agglib.NewAgg2D()
	renderer.Attach(buf, 8, 8, 8*4)
	renderer.ImageResample(resample)
	renderer.ImageFilter(filter)
	src := agglib.NewImage(pixels, 4, 4, 4*4)
	if err := renderer.TransformImageParallelogram(src, 0, 0, 4, 4, parallelogram); err != nil {
		t.Fatalf("TransformImageParallelogram: %v", err)
	}
	return buf
}

func diffByteCount(left, right []byte) int {
	count := 0
	for i := range left {
		if left[i] != right[i] {
			count++
		}
	}
	return count
}

func nonZeroAlphaStats(pixels []byte) (count int, alphaSum int) {
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] == 0 {
			continue
		}
		count++
		alphaSum += int(pixels[i])
	}
	return count, alphaSum
}

func TestDirectAGGAffineInterpolationReproducer(t *testing.T) {
	filters := []struct {
		name  string
		value agglib.ImageFilter
	}{
		{name: "nearest", value: agglib.NoFilter},
		{name: "bilinear", value: agglib.Bilinear},
		{name: "bicubic", value: agglib.Bicubic},
	}
	fixtures := []struct {
		name   string
		pixels []byte
	}{
		{name: "smooth-gradient", pixels: func() []byte { pixels, _ := makeInterpolationFixturePixels(); return pixels }()},
		{name: "high-frequency", pixels: makeHighFrequencyFixturePixels()},
	}
	resamples := []struct {
		name  string
		value agglib.ImageResample
	}{
		{name: "no-resample", value: agglib.NoResample},
		{name: "resample-always", value: agglib.ResampleAlways},
	}

	t.Run("TransformImageSimple", func(t *testing.T) {
		for _, fixture := range fixtures {
			t.Run(fixture.name, func(t *testing.T) {
				for _, resample := range resamples {
					t.Run(resample.name, func(t *testing.T) {
						outputs := make(map[string][]byte, len(filters))
						for _, filter := range filters {
							outputs[filter.name] = renderDirectAGGSimple(t, fixture.pixels, filter.value, resample.value, 0.35, 0.4, 6.75, 5.8)
						}
						nonZero, alphaSum := nonZeroAlphaStats(outputs["nearest"])
						if nonZero == 0 {
							t.Fatal("TransformImageSimple rendered no visible pixels")
						}
						t.Logf("nearest visible pixels=%d alphaSum=%d", nonZero, alphaSum)
						t.Logf("nearest vs bilinear: equal=%v diffBytes=%d", bytes.Equal(outputs["nearest"], outputs["bilinear"]), diffByteCount(outputs["nearest"], outputs["bilinear"]))
						t.Logf("bilinear vs bicubic: equal=%v diffBytes=%d", bytes.Equal(outputs["bilinear"], outputs["bicubic"]), diffByteCount(outputs["bilinear"], outputs["bicubic"]))
					})
				}
			})
		}
	})

	t.Run("TransformImageParallelogram", func(t *testing.T) {
		parallelogram := []float64{0.6, -0.15, 5.32, 1.53, 4.0, 6.61}
		for _, fixture := range fixtures {
			t.Run(fixture.name, func(t *testing.T) {
				for _, resample := range resamples {
					t.Run(resample.name, func(t *testing.T) {
						outputs := make(map[string][]byte, len(filters))
						for _, filter := range filters {
							outputs[filter.name] = renderDirectAGGParallelogram(t, fixture.pixels, filter.value, resample.value, parallelogram)
						}
						nonZero, alphaSum := nonZeroAlphaStats(outputs["nearest"])
						if nonZero == 0 {
							t.Fatal("TransformImageParallelogram rendered no visible pixels")
						}
						t.Logf("nearest visible pixels=%d alphaSum=%d", nonZero, alphaSum)
						t.Logf("nearest vs bilinear: equal=%v diffBytes=%d", bytes.Equal(outputs["nearest"], outputs["bilinear"]), diffByteCount(outputs["nearest"], outputs["bilinear"]))
						t.Logf("bilinear vs bicubic: equal=%v diffBytes=%d", bytes.Equal(outputs["bilinear"], outputs["bicubic"]), diffByteCount(outputs["bilinear"], outputs["bicubic"]))
					})
				}
			})
		}
	})
}
