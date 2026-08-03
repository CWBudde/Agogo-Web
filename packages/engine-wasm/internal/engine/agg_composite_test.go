package engine

import (
	"testing"

	agg "github.com/cwbudde/agg_go"
)

func TestAggBlendModeMapsEveryDocumentMode(t *testing.T) {
	tests := []struct {
		mode BlendMode
		want agg.BlendMode
	}{
		{BlendModeNormal, agg.BlendSrcOver},
		{BlendModeDissolve, agg.BlendDissolve},
		{BlendModeMultiply, agg.BlendMultiply},
		{BlendModeColorBurn, agg.BlendColorBurnPhotoshop},
		{BlendModeLinearBurn, agg.BlendLinearBurn},
		{BlendModeDarken, agg.BlendDarken},
		{BlendModeDarkerColor, agg.BlendDarkerColor},
		{BlendModeScreen, agg.BlendScreen},
		{BlendModeColorDodge, agg.BlendColorDodge},
		{BlendModeLinearDodge, agg.BlendLinearDodge},
		{BlendModeLighten, agg.BlendLighten},
		{BlendModeLighterColor, agg.BlendLighterColor},
		{BlendModeOverlay, agg.BlendOverlay},
		{BlendModeSoftLight, agg.BlendSoftLightPhotoshop},
		{BlendModeHardLight, agg.BlendHardLight},
		{BlendModeVividLight, agg.BlendVividLight},
		{BlendModeLinearLight, agg.BlendLinearLight},
		{BlendModePinLight, agg.BlendPinLight},
		{BlendModeHardMix, agg.BlendHardMix},
		{BlendModeDifference, agg.BlendDifference},
		{BlendModeExclusion, agg.BlendExclusion},
		{BlendModeSubtract, agg.BlendSubtract},
		{BlendModeDivide, agg.BlendDivide},
		{BlendModeHue, agg.BlendHue},
		{BlendModeSaturation, agg.BlendSaturation},
		{BlendModeColor, agg.BlendColor},
		{BlendModeLuminosity, agg.BlendLuminosity},
	}
	for _, test := range tests {
		got := aggBlendMode(test.mode)
		if got != test.want {
			t.Errorf("aggBlendMode(%q) = %v, want %v", test.mode, got, test.want)
		}
	}
	if got := aggBlendMode(BlendMode("future-mode")); got != agg.BlendSrcOver {
		t.Fatalf("unknown mode fallback = %v, want %v", got, agg.BlendSrcOver)
	}
}

func TestAggCompositeOpaqueBlendModesMatchLegacyOracle(t *testing.T) {
	modes := []BlendMode{
		BlendModeNormal, BlendModeDissolve, BlendModeMultiply, BlendModeColorBurn,
		BlendModeLinearBurn, BlendModeDarken, BlendModeDarkerColor, BlendModeScreen,
		BlendModeColorDodge, BlendModeLinearDodge, BlendModeLighten, BlendModeLighterColor,
		BlendModeOverlay, BlendModeSoftLight, BlendModeHardLight, BlendModeVividLight,
		BlendModeLinearLight, BlendModePinLight, BlendModeHardMix, BlendModeDifference,
		BlendModeExclusion, BlendModeSubtract, BlendModeDivide, BlendModeHue,
		BlendModeSaturation, BlendModeColor, BlendModeLuminosity,
	}
	base := []byte{64, 128, 192, 255}
	source := []byte{192, 96, 32, 255}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			legacy := append([]byte(nil), base...)
			compositePixelWithBlend(legacy, source, mode, 1, 0)

			got := append([]byte(nil), base...)
			err := compositeImageStraight(
				got, 1, 1, source, 1, 1,
				agg.Rect{X2: 1, Y2: 1}, agg.PointI{},
				mode, 1, nil, agg.PointI{}, nil,
				func(_, _ int) uint32 { return 0 },
			)
			if err != nil {
				t.Fatal(err)
			}
			for channel := range got {
				if got[channel] != legacy[channel] {
					t.Fatalf("channel %d = %d, legacy oracle %d; pixel=%v oracle=%v", channel, got[channel], legacy[channel], got, legacy)
				}
			}
		})
	}
}

func TestAggCompositeTranslucentBlendModesStayWithinOneByteOfLegacyOracle(t *testing.T) {
	modes := []BlendMode{
		BlendModeNormal, BlendModeMultiply, BlendModeColorBurn, BlendModeLinearBurn,
		BlendModeDarken, BlendModeDarkerColor, BlendModeScreen, BlendModeColorDodge,
		BlendModeLinearDodge, BlendModeLighten, BlendModeLighterColor, BlendModeOverlay,
		BlendModeSoftLight, BlendModeHardLight, BlendModeVividLight, BlendModeLinearLight,
		BlendModePinLight, BlendModeHardMix, BlendModeDifference, BlendModeExclusion,
		BlendModeSubtract, BlendModeDivide, BlendModeHue, BlendModeSaturation,
		BlendModeColor, BlendModeLuminosity,
	}
	base := []byte{64, 128, 192, 173}
	source := []byte{192, 96, 32, 137}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			legacy := append([]byte(nil), base...)
			compositePixelWithBlend(legacy, source, mode, 0.63, 0)
			got := append([]byte(nil), base...)
			if err := compositeImageStraight(
				got, 1, 1, source, 1, 1,
				agg.Rect{X2: 1, Y2: 1}, agg.PointI{},
				mode, 0.63, nil, agg.PointI{}, nil, nil,
			); err != nil {
				t.Fatal(err)
			}
			for channel := range got {
				delta := int(got[channel]) - int(legacy[channel])
				if delta < -1 || delta > 1 {
					t.Fatalf("channel %d = %d, legacy oracle %d; pixel=%v oracle=%v", channel, got[channel], legacy[channel], got, legacy)
				}
			}
		})
	}
}

func TestAggCompositeTranslucentSoftLightMatchesLegacyOracle(t *testing.T) {
	// The engine selects agg_go's additive Photoshop-specific variant while the
	// historical public BlendSoftLight keeps its established AGG behavior.
	base := []byte{64, 128, 192, 173}
	source := []byte{192, 96, 32, 137}
	legacy := append([]byte(nil), base...)
	compositePixelWithBlend(legacy, source, BlendModeSoftLight, 0.63, 0)
	if want := [4]byte{91, 119, 160, 201}; [4]byte(legacy) != want {
		t.Fatalf("legacy Soft Light = %v, want pinned %v", legacy, want)
	}

	got := append([]byte(nil), base...)
	if err := compositeImageStraight(
		got, 1, 1, source, 1, 1,
		agg.Rect{X2: 1, Y2: 1}, agg.PointI{},
		BlendModeSoftLight, 0.63, nil, agg.PointI{}, nil, nil,
	); err != nil {
		t.Fatal(err)
	}
	if want := [4]byte{91, 119, 160, 201}; [4]byte(got) != want {
		t.Fatalf("agg_go Soft Light = %v, want pinned %v", got, want)
	}
}
