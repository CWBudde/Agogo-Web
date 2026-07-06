package engine

import (
	"encoding/json"
	"testing"
)

// applyAdjustmentToPixel runs an adjustment factory on a single pixel and
// returns the transformed RGBA values.
func applyAdjustmentToPixel(t *testing.T, factory func(json.RawMessage) (AdjustmentPixelFunc, error), params string, r, g, b, a uint8) (uint8, uint8, uint8, uint8) {
	t.Helper()
	fn, err := factory(json.RawMessage(params))
	if err != nil {
		t.Fatalf("factory(%s): %v", params, err)
	}
	rr, gg, bb, aa, err := fn(r, g, b, a, nil)
	if err != nil {
		t.Fatalf("pixel func: %v", err)
	}
	return rr, gg, bb, aa
}

// Photoshop Levels convention: the gamma slider value G maps midtones via
// v^(1/G), so G > 1 must BRIGHTEN midtones and G < 1 must DARKEN them.
func TestLevelsGammaDirectionMatchesPhotoshop(t *testing.T) {
	t.Run("gamma 2.0 brightens mid-gray", func(t *testing.T) {
		r, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"gamma":2.0}`, 128, 128, 128, 255)
		if r <= 128 {
			t.Fatalf("levels gamma=2.0 on 128 = %d, want > 128 (brighter)", r)
		}
	})
	t.Run("gamma 0.5 darkens mid-gray", func(t *testing.T) {
		r, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"gamma":0.5}`, 128, 128, 128, 255)
		if r >= 128 {
			t.Fatalf("levels gamma=0.5 on 128 = %d, want < 128 (darker)", r)
		}
	})
	t.Run("gamma 2.0 exact midpoint value", func(t *testing.T) {
		// 128/255 = 0.50196; 0.50196^(1/2) = 0.70849; *255 = 180.67 -> 181
		r, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"gamma":2.0}`, 128, 128, 128, 255)
		if r != 181 {
			t.Fatalf("levels gamma=2.0 on 128 = %d, want 181", r)
		}
	})
	t.Run("gamma leaves endpoints fixed", func(t *testing.T) {
		lo, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"gamma":2.0}`, 0, 0, 0, 255)
		hi, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"gamma":2.0}`, 255, 255, 255, 255)
		if lo != 0 || hi != 255 {
			t.Fatalf("levels gamma=2.0 endpoints = (%d, %d), want (0, 255)", lo, hi)
		}
	})
}

// Exposure's gamma-correction parameter follows the same Photoshop
// convention: v^(1/G), so G > 1 brightens.
func TestExposureGammaDirectionMatchesPhotoshop(t *testing.T) {
	t.Run("gamma 2.0 brightens mid-gray", func(t *testing.T) {
		r, _, _, _ := applyAdjustmentToPixel(t, exposureAdjustmentFactory, `{"exposure":0,"offset":0,"gamma":2.0}`, 128, 128, 128, 255)
		if r <= 128 {
			t.Fatalf("exposure gamma=2.0 on 128 = %d, want > 128 (brighter)", r)
		}
	})
	t.Run("gamma 0.5 darkens mid-gray", func(t *testing.T) {
		r, _, _, _ := applyAdjustmentToPixel(t, exposureAdjustmentFactory, `{"exposure":0,"offset":0,"gamma":0.5}`, 128, 128, 128, 255)
		if r >= 128 {
			t.Fatalf("exposure gamma=0.5 on 128 = %d, want < 128 (darker)", r)
		}
	})
}

// Partial Levels payloads must behave like Photoshop defaults for every
// omitted parameter (inputBlack=0, inputWhite=255, gamma=1, outputBlack=0,
// outputWhite=255). In particular {"outputBlack":10} alone must yield an
// output range of [10..255], not [10..0].
func TestLevelsPartialPayloadUsesPhotoshopDefaults(t *testing.T) {
	t.Run("outputBlack alone maps to range 10..255", func(t *testing.T) {
		lo, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"outputBlack":10}`, 0, 0, 0, 255)
		hi, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"outputBlack":10}`, 255, 255, 255, 255)
		if lo != 10 {
			t.Fatalf("levels {outputBlack:10} on 0 = %d, want 10", lo)
		}
		if hi != 255 {
			t.Fatalf("levels {outputBlack:10} on 255 = %d, want 255 (outputWhite must default to 255)", hi)
		}
		mid, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"outputBlack":10}`, 128, 128, 128, 255)
		if mid <= lo || mid >= hi {
			t.Fatalf("levels {outputBlack:10} on 128 = %d, want strictly between %d and %d", mid, lo, hi)
		}
	})
	t.Run("inputBlack alone keeps input white default 255", func(t *testing.T) {
		hi, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"inputBlack":10}`, 255, 255, 255, 255)
		if hi != 255 {
			t.Fatalf("levels {inputBlack:10} on 255 = %d, want 255", hi)
		}
		lo, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"inputBlack":10}`, 10, 10, 10, 255)
		if lo != 0 {
			t.Fatalf("levels {inputBlack:10} on 10 = %d, want 0", lo)
		}
	})
	t.Run("empty payload is identity", func(t *testing.T) {
		for _, v := range []uint8{0, 64, 128, 200, 255} {
			got, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{}`, v, v, v, 255)
			if got != v {
				t.Fatalf("levels {} on %d = %d, want identity", v, got)
			}
		}
	})
	t.Run("outputBlack with explicit outputWhite still honored", func(t *testing.T) {
		hi, _, _, _ := applyAdjustmentToPixel(t, levelsAdjustmentFactory, `{"outputBlack":10,"outputWhite":200}`, 255, 255, 255, 255)
		if hi != 200 {
			t.Fatalf("levels {outputBlack:10,outputWhite:200} on 255 = %d, want 200", hi)
		}
	})
}
