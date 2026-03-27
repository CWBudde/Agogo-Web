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
