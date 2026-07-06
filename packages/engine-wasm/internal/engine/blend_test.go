package engine

import (
	"math"
	"testing"
)

func TestCompositePixelWithBlendModes(t *testing.T) {
	base := []byte{64, 128, 192, 255}
	top := []byte{192, 96, 32, 255}

	tests := []struct {
		name   string
		mode   BlendMode
		expect [4]uint8
	}{
		{name: "normal", mode: BlendModeNormal, expect: [4]uint8{192, 96, 32, 255}},
		{name: "dissolve", mode: BlendModeDissolve, expect: [4]uint8{192, 96, 32, 255}},
		{name: "multiply", mode: BlendModeMultiply, expect: [4]uint8{48, 48, 24, 255}},
		{name: "color-burn", mode: BlendModeColorBurn, expect: [4]uint8{1, 0, 0, 255}},
		{name: "linear-burn", mode: BlendModeLinearBurn, expect: [4]uint8{1, 0, 0, 255}},
		{name: "darken", mode: BlendModeDarken, expect: [4]uint8{64, 96, 32, 255}},
		{name: "darker-color", mode: BlendModeDarkerColor, expect: [4]uint8{64, 128, 192, 255}},
		{name: "screen", mode: BlendModeScreen, expect: [4]uint8{208, 176, 200, 255}},
		{name: "color-dodge", mode: BlendModeColorDodge, expect: [4]uint8{255, 205, 220, 255}},
		{name: "linear-dodge", mode: BlendModeLinearDodge, expect: [4]uint8{255, 224, 224, 255}},
		{name: "lighten", mode: BlendModeLighten, expect: [4]uint8{192, 128, 192, 255}},
		{name: "lighter-color", mode: BlendModeLighterColor, expect: [4]uint8{192, 96, 32, 255}},
		{name: "overlay", mode: BlendModeOverlay, expect: [4]uint8{96, 97, 145, 255}},
		{name: "soft-light", mode: BlendModeSoftLight, expect: [4]uint8{96, 112, 156, 255}},
		{name: "hard-light", mode: BlendModeHardLight, expect: [4]uint8{161, 96, 48, 255}},
		{name: "vivid-light", mode: BlendModeVividLight, expect: [4]uint8{130, 86, 4, 255}},
		{name: "linear-light", mode: BlendModeLinearLight, expect: [4]uint8{193, 65, 1, 255}},
		{name: "pin-light", mode: BlendModePinLight, expect: [4]uint8{129, 128, 64, 255}},
		{name: "hard-mix", mode: BlendModeHardMix, expect: [4]uint8{255, 0, 0, 255}},
		{name: "difference", mode: BlendModeDifference, expect: [4]uint8{128, 32, 160, 255}},
		{name: "exclusion", mode: BlendModeExclusion, expect: [4]uint8{160, 128, 176, 255}},
		{name: "subtract", mode: BlendModeSubtract, expect: [4]uint8{0, 32, 160, 255}},
		{name: "divide", mode: BlendModeDivide, expect: [4]uint8{85, 255, 255, 255}},
		{name: "hue", mode: BlendModeHue, expect: [4]uint8{175, 98, 47, 255}},
		{name: "saturation", mode: BlendModeSaturation, expect: [4]uint8{51, 131, 211, 255}},
		{name: "color", mode: BlendModeColor, expect: [4]uint8{190, 94, 30, 255}},
		{name: "luminosity", mode: BlendModeLuminosity, expect: [4]uint8{66, 130, 194, 255}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dest := append([]byte(nil), base...)
			compositePixelWithBlend(dest, top, test.mode, 1, 1234)
			for index := range dest {
				if dest[index] != test.expect[index] {
					t.Fatalf("pixel[%d] = %d, want %d", index, dest[index], test.expect[index])
				}
			}
		})
	}
}

func TestCompositePixelWithBlendRespectsOpacity(t *testing.T) {
	base := []byte{64, 128, 192, 255}
	top := []byte{192, 96, 32, 255}
	dest := append([]byte(nil), base...)

	compositePixelWithBlend(dest, top, BlendModeMultiply, 0.5, 0)

	expect := [4]uint8{56, 88, 108, 255}
	for index := range dest {
		if diff := math.Abs(float64(dest[index]) - float64(expect[index])); diff > 1 {
			t.Fatalf("pixel[%d] = %d, want %d", index, dest[index], expect[index])
		}
	}
}

func TestComponentBlendModesPreserveExpectedAttributes(t *testing.T) {
	base := []byte{64, 128, 192, 255}
	top := []byte{192, 96, 32, 255}
	baseRGB := [3]float64{float64(base[0]) / 255, float64(base[1]) / 255, float64(base[2]) / 255}
	topRGB := [3]float64{float64(top[0]) / 255, float64(top[1]) / 255, float64(top[2]) / 255}

	colorResult := append([]byte(nil), base...)
	compositePixelWithBlend(colorResult, top, BlendModeColor, 1, 0)
	colorRGB := [3]float64{float64(colorResult[0]) / 255, float64(colorResult[1]) / 255, float64(colorResult[2]) / 255}
	if diff := math.Abs(luminosity(colorRGB) - luminosity(baseRGB)); diff > 0.02 {
		t.Fatalf("color blend luminosity diff = %.4f, want <= 0.02", diff)
	}
	if saturation(colorRGB) < saturation(topRGB)-0.05 {
		t.Fatalf("color blend saturation = %.4f, want close to source saturation %.4f", saturation(colorRGB), saturation(topRGB))
	}

	luminosityResult := append([]byte(nil), base...)
	compositePixelWithBlend(luminosityResult, top, BlendModeLuminosity, 1, 0)
	lumRGB := [3]float64{float64(luminosityResult[0]) / 255, float64(luminosityResult[1]) / 255, float64(luminosityResult[2]) / 255}
	if diff := math.Abs(luminosity(lumRGB) - luminosity(topRGB)); diff > 0.02 {
		t.Fatalf("luminosity blend luminosity diff = %.4f, want <= 0.02", diff)
	}
	if diff := math.Abs(saturation(lumRGB) - saturation(baseRGB)); diff > 0.05 {
		t.Fatalf("luminosity blend saturation diff = %.4f, want <= 0.05", diff)
	}
}

func TestDissolveBlendModeIsDeterministic(t *testing.T) {
	destA := []byte{0, 0, 0, 255}
	destB := []byte{0, 0, 0, 255}
	src := []byte{255, 128, 64, 128}

	compositePixelWithBlend(destA, src, BlendModeDissolve, 1, 1234)
	compositePixelWithBlend(destB, src, BlendModeDissolve, 1, 1234)

	for index := range destA {
		if destA[index] != destB[index] {
			t.Fatalf("dissolve mismatch at channel %d: %d != %d", index, destA[index], destB[index])
		}
	}
}

func TestGroupIsolationAffectsCompositing(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}
	bottom := NewPixelLayer("Bottom", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{0, 0, 255, 255})
	group := NewGroupLayer("Group")
	group.Isolated = false
	group.SetOpacity(1)
	multiply := NewPixelLayer("Multiply", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{255, 0, 0, 255})
	multiply.SetBlendMode(BlendModeMultiply)
	screen := NewPixelLayer("Screen", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{0, 255, 0, 255})
	screen.SetBlendMode(BlendModeScreen)
	group.SetChildren([]LayerNode{multiply, screen})
	bufferPassThrough := make([]byte, 4)
	if err := doc.compositeLayerOnto(bufferPassThrough, bottom); err != nil {
		t.Fatalf("composite bottom: %v", err)
	}
	if err := doc.compositeLayerOnto(bufferPassThrough, group); err != nil {
		t.Fatalf("composite pass-through group: %v", err)
	}

	isolatedGroup := NewGroupLayer("Isolated")
	isolatedGroup.Isolated = true
	isolatedMultiply := NewPixelLayer("Multiply", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{255, 0, 0, 255})
	isolatedMultiply.SetBlendMode(BlendModeMultiply)
	isolatedScreen := NewPixelLayer("Screen", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{0, 255, 0, 255})
	isolatedScreen.SetBlendMode(BlendModeScreen)
	isolatedGroup.SetChildren([]LayerNode{isolatedMultiply, isolatedScreen})
	bufferIsolated := make([]byte, 4)
	if err := doc.compositeLayerOnto(bufferIsolated, bottom); err != nil {
		t.Fatalf("composite bottom isolated: %v", err)
	}
	if err := doc.compositeLayerOnto(bufferIsolated, isolatedGroup); err != nil {
		t.Fatalf("composite isolated group: %v", err)
	}

	if bufferPassThrough[0] == bufferIsolated[0] && bufferPassThrough[1] == bufferIsolated[1] && bufferPassThrough[2] == bufferIsolated[2] {
		t.Fatal("isolated and pass-through groups should not produce the same composite for blended children")
	}
	if bufferPassThrough[0] >= bufferIsolated[0] {
		t.Fatalf("expected isolated composite to preserve more red than pass-through: %v vs %v", bufferPassThrough, bufferIsolated)
	}
}

// TestBlendColorDodgeExtremes verifies the W3C/Photoshop compositing spec for
// color dodge: if Cb==0 -> 0; else if Cs==1 -> 1; else min(1, Cb/(1-Cs)).
// The Cb==0 check takes precedence over Cs==1, so black backdrop stays black
// even under a white source.
func TestBlendColorDodgeExtremes(t *testing.T) {
	tests := []struct {
		name     string
		backdrop float64
		source   float64
		expect   float64
	}{
		{name: "black-backdrop-white-source", backdrop: 0, source: 1, expect: 0},
		{name: "black-backdrop-black-source", backdrop: 0, source: 0, expect: 0},
		{name: "black-backdrop-mid-source", backdrop: 0, source: 0.5, expect: 0},
		{name: "white-backdrop-white-source", backdrop: 1, source: 1, expect: 1},
		{name: "mid-backdrop-white-source", backdrop: 0.5, source: 1, expect: 1},
		{name: "white-backdrop-black-source", backdrop: 1, source: 0, expect: 1},
		{name: "quarter-backdrop-mid-source", backdrop: 0.25, source: 0.5, expect: 0.5},
		{name: "mid-backdrop-clamps-to-one", backdrop: 0.5, source: 0.75, expect: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := blendColorDodge(test.backdrop, test.source)
			if math.Abs(got-test.expect) > 1e-12 {
				t.Fatalf("blendColorDodge(%v, %v) = %v, want %v", test.backdrop, test.source, got, test.expect)
			}
		})
	}
}

// TestBlendColorBurnExtremes verifies the W3C/Photoshop compositing spec for
// color burn: if Cb==1 -> 1; else if Cs==0 -> 0; else 1-min(1, (1-Cb)/Cs).
// The Cb==1 check takes precedence over Cs==0, so white backdrop stays white
// even under a black source.
func TestBlendColorBurnExtremes(t *testing.T) {
	tests := []struct {
		name     string
		backdrop float64
		source   float64
		expect   float64
	}{
		{name: "white-backdrop-black-source", backdrop: 1, source: 0, expect: 1},
		{name: "white-backdrop-white-source", backdrop: 1, source: 1, expect: 1},
		{name: "white-backdrop-mid-source", backdrop: 1, source: 0.5, expect: 1},
		{name: "black-backdrop-black-source", backdrop: 0, source: 0, expect: 0},
		{name: "mid-backdrop-black-source", backdrop: 0.5, source: 0, expect: 0},
		{name: "black-backdrop-white-source", backdrop: 0, source: 1, expect: 0},
		{name: "three-quarter-backdrop-mid-source", backdrop: 0.75, source: 0.5, expect: 0.5},
		{name: "quarter-backdrop-clamps-to-zero", backdrop: 0.25, source: 0.5, expect: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := blendColorBurn(test.backdrop, test.source)
			if math.Abs(got-test.expect) > 1e-12 {
				t.Fatalf("blendColorBurn(%v, %v) = %v, want %v", test.backdrop, test.source, got, test.expect)
			}
		})
	}
}

// TestCompositeDodgeBurnCornerPixels exercises the spec corner cases through
// the full byte-level compositing path.
func TestCompositeDodgeBurnCornerPixels(t *testing.T) {
	tests := []struct {
		name   string
		mode   BlendMode
		base   []byte
		top    []byte
		expect [4]uint8
	}{
		// Photoshop: dodging a black backdrop never brightens it.
		{name: "dodge-black-under-white", mode: BlendModeColorDodge, base: []byte{0, 0, 0, 255}, top: []byte{255, 255, 255, 255}, expect: [4]uint8{0, 0, 0, 255}},
		// Photoshop: burning a white backdrop never darkens it.
		{name: "burn-white-under-black", mode: BlendModeColorBurn, base: []byte{255, 255, 255, 255}, top: []byte{0, 0, 0, 255}, expect: [4]uint8{255, 255, 255, 255}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dest := append([]byte(nil), test.base...)
			compositePixelWithBlend(dest, test.top, test.mode, 1, 0)
			for index := range dest {
				if dest[index] != test.expect[index] {
					t.Fatalf("pixel[%d] = %d, want %d (full pixel %v)", index, dest[index], test.expect[index], dest)
				}
			}
		})
	}
}

// TestClipColorDegenerateInputsProduceNoNaN feeds clipColor colors for which
// the spec scale-factor denominators (lum-min when min<0, max-lum when max>1)
// are exactly zero in float64, which the unguarded division turns into NaN or
// -Inf. The constants below satisfy luminosity(v,v,v) == v bit-exactly.
// clipColor must always return finite values clamped to [0, 1].
func TestClipColorDegenerateInputsProduceNoNaN(t *testing.T) {
	tests := []struct {
		name  string
		color [3]float64
	}{
		{name: "huge-positive-equal", color: [3]float64{1e300, 1e300, 1e300}},
		{name: "huge-negative-equal", color: [3]float64{-1e300, -1e300, -1e300}},
		{name: "negative-equal-lum-collision", color: [3]float64{-0.5301086108675604, -0.5301086108675604, -0.5301086108675604}},
		{name: "above-one-equal-lum-collision", color: [3]float64{1.2061542891037402, 1.2061542891037402, 1.2061542891037402}},
		{name: "negative-near-equal-lum-collision", color: [3]float64{-0.3686426883719394, -0.3686426883719394, -0.36864268837193936}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := clipColor(test.color)
			for index, value := range result {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					t.Fatalf("clipColor(%v)[%d] = %v, want finite", test.color, index, value)
				}
				if value < 0 || value > 1 {
					t.Fatalf("clipColor(%v)[%d] = %v, want in [0, 1]", test.color, index, value)
				}
			}
		})
	}
}

// TestNonSeparableBlendExtremesStayFinite sweeps every pure channel-extreme
// backdrop/source pair through the Hue/Saturation/Color/Luminosity blend
// modes and asserts no NaN/Inf or out-of-range component ever escapes.
func TestNonSeparableBlendExtremesStayFinite(t *testing.T) {
	modes := []struct {
		name string
		mode BlendMode
	}{
		{name: "hue", mode: BlendModeHue},
		{name: "saturation", mode: BlendModeSaturation},
		{name: "color", mode: BlendModeColor},
		{name: "luminosity", mode: BlendModeLuminosity},
	}

	channelValue := func(bits, shift int) float64 {
		if bits&(1<<shift) != 0 {
			return 1
		}
		return 0
	}

	for _, entry := range modes {
		t.Run(entry.name, func(t *testing.T) {
			for backdropBits := 0; backdropBits < 8; backdropBits++ {
				for sourceBits := 0; sourceBits < 8; sourceBits++ {
					backdrop := rgbaColor{r: channelValue(backdropBits, 0), g: channelValue(backdropBits, 1), b: channelValue(backdropBits, 2), a: 1}
					source := rgbaColor{r: channelValue(sourceBits, 0), g: channelValue(sourceBits, 1), b: channelValue(sourceBits, 2), a: 1}
					result := blendRGB(backdrop, source, entry.mode)
					for index, value := range [3]float64{result.r, result.g, result.b} {
						if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
							t.Fatalf("blendRGB(%v, %v, %s) channel %d = %v, want finite in [0, 1]", backdrop, source, entry.name, index, value)
						}
					}
				}
			}
		})
	}
}
