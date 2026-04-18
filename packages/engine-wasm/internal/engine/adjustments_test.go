package engine

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCoreAdjustmentKindsAffectPixels(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		params string
		check  func(t *testing.T, got, base [4]byte)
	}{
		{
			name:   "levels",
			kind:   "levels",
			params: `{"inputBlack":10,"inputWhite":110,"gamma":1,"channel":"rgb"}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] <= base[0] {
					t.Fatalf("levels red = %d, want > %d", got[0], base[0])
				}
			},
		},
		{
			name:   "curves",
			kind:   "curves",
			params: `{"points":[{"x":0,"y":0},{"x":128,"y":200},{"x":255,"y":255}]}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] <= base[0] {
					t.Fatalf("curves red = %d, want > %d", got[0], base[0])
				}
			},
		},
		{
			name:   "hue-sat",
			kind:   "huesat",
			params: `{"hueShift":180,"saturation":0,"lightness":0}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[2] <= got[0] {
					t.Fatalf("hue-sat blue/red = %d/%d, want blue > red", got[2], got[0])
				}
			},
		},
		{
			name:   "color-balance",
			kind:   "color-balance",
			params: `{"shadows":{"cyanRed":60},"preserveLuminosity":true}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] <= base[0] {
					t.Fatalf("color-balance red = %d, want > %d", got[0], base[0])
				}
			},
		},
		{
			name:   "brightness-contrast",
			kind:   "brightness-contrast",
			params: `{"brightness":50,"contrast":10}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] <= base[0] {
					t.Fatalf("brightness-contrast red = %d, want > %d", got[0], base[0])
				}
			},
		},
		{
			name:   "exposure",
			kind:   "exposure",
			params: `{"exposure":1,"offset":0,"gamma":1}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] <= base[0] {
					t.Fatalf("exposure red = %d, want > %d", got[0], base[0])
				}
			},
		},
		{
			name:   "vibrance",
			kind:   "vibrance",
			params: `{"vibrance":80,"saturation":0}`,
			check: func(t *testing.T, got, base [4]byte) {
				gotSpread := maxByte(got[0], got[1], got[2]) - minByte(got[0], got[1], got[2])
				baseSpread := maxByte(base[0], base[1], base[2]) - minByte(base[0], base[1], base[2])
				if gotSpread <= baseSpread {
					t.Fatalf("vibrance spread = %d, want > %d", gotSpread, baseSpread)
				}
			},
		},
		{
			name:   "black-white",
			kind:   "black-white",
			params: `{"reds":20,"tint":false}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] != got[1] || got[1] != got[2] {
					t.Fatalf("black-white pixel = %v, want grayscale", got)
				}
			},
		},
		{
			name:   "invert",
			kind:   "invert",
			params: `{}`,
			check: func(t *testing.T, got, base [4]byte) {
				want := [4]byte{255 - base[0], 255 - base[1], 255 - base[2], base[3]}
				if got != want {
					t.Fatalf("invert pixel = %v, want %v", got, want)
				}
			},
		},
		{
			name:   "channel-mixer",
			kind:   "channel-mixer",
			params: `{"red":[0,0,100],"green":[0,100,0],"blue":[100,0,0]}`,
			check: func(t *testing.T, got, base [4]byte) {
				want := [4]byte{base[2], base[1], base[0], base[3]}
				if got != want {
					t.Fatalf("channel-mixer pixel = %v, want %v", got, want)
				}
			},
		},
		{
			name:   "threshold",
			kind:   "threshold",
			params: `{"threshold":70}`,
			check: func(t *testing.T, got, base [4]byte) {
				want := [4]byte{255, 255, 255, base[3]}
				if got != want {
					t.Fatalf("threshold pixel = %v, want %v", got, want)
				}
			},
		},
		{
			name:   "posterize",
			kind:   "posterize",
			params: `{"levels":4}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[3] != base[3] {
					t.Fatalf("posterize alpha = %d, want %d", got[3], base[3])
				}
				if got[0] == base[0] && got[1] == base[1] && got[2] == base[2] {
					t.Fatalf("posterize pixel = %v, want quantized channels", got)
				}
				for _, channel := range got[:3] {
					if channel != 0 && channel != 85 && channel != 170 && channel != 255 {
						t.Fatalf("posterize channel = %d, want 4-level quantized value", channel)
					}
				}
			},
		},
		{
			name:   "photo-filter",
			kind:   "photo-filter",
			params: `{"color":[255,128,0,255],"density":100,"preserveLuminosity":false}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] <= got[1] || got[1] <= got[2] {
					t.Fatalf("photo-filter pixel = %v, want warm filter tint", got)
				}
				if got[3] != base[3] {
					t.Fatalf("photo-filter alpha = %d, want %d", got[3], base[3])
				}
			},
		},
		{
			name:   "selective-color",
			kind:   "selective-color",
			params: `{"mode":"absolute","blues":{"cyanRed":0,"magentaGreen":0,"yellowBlue":50,"black":-20}}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[2] <= base[2] {
					t.Fatalf("selective-color blue = %d, want > %d", got[2], base[2])
				}
				if got[3] != base[3] {
					t.Fatalf("selective-color alpha = %d, want %d", got[3], base[3])
				}
			},
		},
		{
			name:   "gradient-map",
			kind:   "gradient-map",
			params: `{"stops":[{"position":0,"color":[0,0,0,255]},{"position":1,"color":[255,0,0,255]}]}`,
			check: func(t *testing.T, got, base [4]byte) {
				if got[0] <= got[1] || got[0] <= got[2] {
					t.Fatalf("gradient-map pixel = %v, want red-dominant mapping", got)
				}
				if got[3] != base[3] {
					t.Fatalf("gradient-map alpha = %d, want source alpha %d", got[3], base[3])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := [4]byte{90, 70, 40, 255}
			if tc.kind == "huesat" {
				base = [4]byte{220, 30, 30, 255}
			}
			if tc.kind == "vibrance" {
				base = [4]byte{120, 100, 100, 255}
			}
			if tc.kind == "selective-color" {
				base = [4]byte{40, 70, 220, 255}
			}
			got := renderAdjustmentTestPixel(t, tc.kind, tc.params, base)
			tc.check(t, got, base)
		})
	}
}

func TestLevelsAutoStretchesCoveredRange(t *testing.T) {
	doc := &Document{
		Width:      3,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Auto Levels",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 3, H: 1}, []byte{
		40, 40, 40, 255,
		80, 80, 80, 255,
		120, 120, 120, 255,
	})
	adjustment := NewAdjustmentLayer("Levels", "levels", json.RawMessage(`{"auto":true,"channel":"rgb"}`))
	doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

	surface := doc.renderCompositeSurface()
	if got := surface[:4]; got[0] != 0 || got[1] != 0 || got[2] != 0 {
		t.Fatalf("shadow pixel = %v, want stretched black", got)
	}
	if got := surface[8:12]; got[0] != 255 || got[1] != 255 || got[2] != 255 {
		t.Fatalf("highlight pixel = %v, want stretched white", got)
	}
	mid := surface[4:8]
	if mid[0] <= 80 || mid[0] >= 180 {
		t.Fatalf("midtone pixel = %v, want stretched midpoint", mid)
	}
}

func TestLevelsAutoClipPercentRejectsOutlier(t *testing.T) {
	doc := &Document{
		Width:      4,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Auto Levels Clip",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 4, H: 1}, []byte{
		0, 0, 0, 255,
		50, 50, 50, 255,
		100, 100, 100, 255,
		110, 110, 110, 255,
	})
	adjustment := NewAdjustmentLayer("Levels", "levels", json.RawMessage(`{"auto":true,"channel":"rgb","shadowClipPercent":26}`))
	doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

	surface := doc.renderCompositeSurface()
	if got := surface[:4]; got[0] != 0 || got[1] != 0 || got[2] != 0 {
		t.Fatalf("outlier pixel = %v, want clipped to black", got)
	}
	if got := surface[4:8]; got[0] != 0 || got[1] != 0 || got[2] != 0 {
		t.Fatalf("first kept shadow pixel = %v, want clipped black after shadow clipping", got)
	}
}

func TestHueSaturationPerColorRangeTargetsMatchingHue(t *testing.T) {
	doc := &Document{
		Width:      2,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Hue/Sat Ranges",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		220, 40, 40, 255,
		40, 40, 220, 255,
	})
	adjustment := NewAdjustmentLayer("Hue/Sat", "huesat", json.RawMessage(`{"reds":{"hueShift":120}}`))
	doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

	surface := doc.renderCompositeSurface()
	if surface[1] <= surface[0] {
		t.Fatalf("shifted red pixel = %v, want reds range to rotate toward green", surface[:4])
	}
	if surface[6] <= surface[4] || surface[6] <= surface[5] {
		t.Fatalf("blue pixel = %v, want non-targeted blue pixel to stay blue-dominant", surface[4:8])
	}
}

func TestBlackWhiteAutoSeparatesHueBuckets(t *testing.T) {
	doc := &Document{
		Width:      2,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Auto B&W",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
		220, 40, 40, 255,
		40, 40, 220, 255,
	})
	adjustment := NewAdjustmentLayer("Black & White", "black-white", json.RawMessage(`{"auto":true}`))
	doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

	surface := doc.renderCompositeSurface()
	left := surface[0]
	right := surface[4]
	if left == right {
		t.Fatalf("auto black-white produced equal tones for red and blue pixels: %v", surface)
	}
	if surface[0] != surface[1] || surface[1] != surface[2] || surface[4] != surface[5] || surface[5] != surface[6] {
		t.Fatalf("auto black-white output = %v, want grayscale pixels", surface)
	}
}

func TestAdjustmentLayerParamsSerializeInLayerMeta(t *testing.T) {
	doc := &Document{
		Width:      1,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Meta",
		LayerRoot:  NewGroupLayer("Root"),
	}
	adjustment := NewAdjustmentLayer("Levels", "levels", json.RawMessage(`{"inputBlack":10,"inputWhite":200}`))
	doc.LayerRoot.SetChildren([]LayerNode{adjustment})

	meta := doc.LayerMeta()
	if len(meta) != 1 {
		t.Fatalf("len(meta) = %d, want 1", len(meta))
	}
	if meta[0].AdjustmentKind != "levels" {
		t.Fatalf("AdjustmentKind = %q, want levels", meta[0].AdjustmentKind)
	}
	if string(meta[0].Params) != `{"inputBlack":10,"inputWhite":200}` {
		t.Fatalf("Params = %s, want original JSON", string(meta[0].Params))
	}
}

func TestExtendedAdjustmentLayersUseNonDestructiveCompositePath(t *testing.T) {
	t.Run("visibility and masks", func(t *testing.T) {
		doc := &Document{
			Width:      2,
			Height:     1,
			Resolution: 72,
			ColorMode:  "rgb",
			BitDepth:   8,
			Background: parseBackground("transparent"),
			Name:       "Visibility",
			LayerRoot:  NewGroupLayer("Root"),
		}
		basePixels := []byte{
			10, 20, 30, 255,
			40, 50, 60, 255,
		}
		base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, append([]byte(nil), basePixels...))
		adjustment := NewAdjustmentLayer("Invert", "invert", nil)
		doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

		original := append([]byte(nil), base.Pixels...)
		visible := doc.renderCompositeSurface()
		if !bytes.Equal(base.Pixels, original) {
			t.Fatalf("base pixels mutated during render: got %v want %v", base.Pixels, original)
		}
		wantVisible := []byte{
			245, 235, 225, 255,
			215, 205, 195, 255,
		}
		if !bytes.Equal(visible[:8], wantVisible) {
			t.Fatalf("visible composite = %v, want %v", visible[:8], wantVisible)
		}

		adjustment.SetVisible(false)
		hidden := doc.renderCompositeSurface()
		if !bytes.Equal(hidden[:8], basePixels) {
			t.Fatalf("hidden composite = %v, want original base pixels %v", hidden[:8], basePixels)
		}

		adjustment.SetVisible(true)
		adjustment.SetMask(&LayerMask{
			Enabled: true,
			Width:   2,
			Height:  1,
			Data:    []byte{255, 0},
		})
		masked := doc.renderCompositeSurface()
		wantMasked := []byte{
			245, 235, 225, 255,
			40, 50, 60, 255,
		}
		if !bytes.Equal(masked[:8], wantMasked) {
			t.Fatalf("masked composite = %v, want %v", masked[:8], wantMasked)
		}
	})

	t.Run("clip to below", func(t *testing.T) {
		doc := &Document{
			Width:      2,
			Height:     1,
			Resolution: 72,
			ColorMode:  "rgb",
			BitDepth:   8,
			Background: parseBackground("transparent"),
			Name:       "Clip",
			LayerRoot:  NewGroupLayer("Root"),
		}
		bottom := NewPixelLayer("Bottom", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
			0, 0, 255, 255,
			0, 0, 255, 255,
		})
		base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 1}, []byte{
			255, 0, 0, 255,
			255, 0, 0, 0,
		})
		adjustment := NewAdjustmentLayer("Invert", "invert", nil)
		doc.LayerRoot.SetChildren([]LayerNode{bottom, base, adjustment})

		unclipped := doc.renderCompositeSurface()
		wantUnclipped := []byte{
			0, 255, 255, 255,
			255, 255, 0, 255,
		}
		if !bytes.Equal(unclipped[:8], wantUnclipped) {
			t.Fatalf("unclipped composite = %v, want %v", unclipped[:8], wantUnclipped)
		}

		if err := doc.SetLayerClipToBelow(adjustment.ID(), true); err != nil {
			t.Fatalf("set clip to below: %v", err)
		}
		if !base.ClippingBase() || !adjustment.ClipToBelow() {
			t.Fatalf("unexpected clip flags: base=%v adjustment=%v", base.ClippingBase(), adjustment.ClipToBelow())
		}

		clipped := doc.renderCompositeSurface()
		wantClipped := []byte{
			0, 255, 255, 255,
			0, 0, 255, 255,
		}
		if !bytes.Equal(clipped[:8], wantClipped) {
			t.Fatalf("clipped composite = %v, want %v", clipped[:8], wantClipped)
		}
	})
}

func TestAdjustmentLayerCacheReusesOnlyDirtyRegion(t *testing.T) {
	const kind = "test-dirty-region-cache"

	var calls int
	RegisterAdjustmentFactory(kind, func(params json.RawMessage) (AdjustmentPixelFunc, error) {
		return func(r, g, b, a uint8, _ json.RawMessage) (uint8, uint8, uint8, uint8, error) {
			calls++
			return 255 - r, g, b, a, nil
		}, nil
	})
	t.Cleanup(func() {
		RegisterAdjustmentFactory(kind, nil)
	})

	doc := &Document{
		Width:      4,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Dirty Cache",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 4, H: 1}, []byte{
		10, 0, 0, 255,
		20, 0, 0, 255,
		30, 0, 0, 255,
		40, 0, 0, 255,
	})
	adjustment := NewAdjustmentLayer("Cached", kind, nil)
	doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

	first := doc.renderCompositeSurface()
	if calls != 4 {
		t.Fatalf("first render transform calls = %d, want 4", calls)
	}
	wantFirst := []byte{
		245, 0, 0, 255,
		235, 0, 0, 255,
		225, 0, 0, 255,
		215, 0, 0, 255,
	}
	if !bytes.Equal(first[:16], wantFirst) {
		t.Fatalf("first render = %v, want %v", first[:16], wantFirst)
	}

	calls = 0
	base.Pixels[4] = 90
	doc.touchModifiedAtRect(DirtyRect{X: 1, Y: 0, W: 1, H: 1})

	second := doc.renderCompositeSurface()
	if calls != 1 {
		t.Fatalf("second render transform calls = %d, want 1 dirty pixel", calls)
	}
	wantSecond := []byte{
		245, 0, 0, 255,
		165, 0, 0, 255,
		225, 0, 0, 255,
		215, 0, 0, 255,
	}
	if !bytes.Equal(second[:16], wantSecond) {
		t.Fatalf("second render = %v, want %v", second[:16], wantSecond)
	}
}

func TestAutoLevelsBypassesDirtyRegionCacheWhenResolvedParamsDependOnSurface(t *testing.T) {
	doc := &Document{
		Width:      3,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Auto Levels Dirty",
		LayerRoot:  NewGroupLayer("Root"),
	}
	base := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 3, H: 1}, []byte{
		40, 40, 40, 255,
		80, 80, 80, 255,
		120, 120, 120, 255,
	})
	adjustment := NewAdjustmentLayer("Levels", "levels", json.RawMessage(`{"auto":true,"channel":"rgb"}`))
	doc.LayerRoot.SetChildren([]LayerNode{base, adjustment})

	before := doc.renderCompositeSurface()
	midBefore := before[4]

	base.Pixels[0] = 60
	base.Pixels[1] = 60
	base.Pixels[2] = 60
	doc.touchModifiedAtRect(DirtyRect{X: 0, Y: 0, W: 1, H: 1})

	after := doc.renderCompositeSurface()
	if after[4] == midBefore {
		t.Fatalf("auto-levels midtone stayed unchanged after upstream dirty update: before=%d after=%d", midBefore, after[4])
	}
}

func renderAdjustmentTestPixel(t *testing.T, kind, params string, base [4]byte) [4]byte {
	t.Helper()
	doc := &Document{
		Width:      1,
		Height:     1,
		Resolution: 72,
		ColorMode:  "rgb",
		BitDepth:   8,
		Background: parseBackground("transparent"),
		Name:       "Adjusted",
		LayerRoot:  NewGroupLayer("Root"),
	}
	pixel := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{base[0], base[1], base[2], base[3]})
	adjustment := NewAdjustmentLayer("Adj", kind, json.RawMessage(params))
	doc.LayerRoot.SetChildren([]LayerNode{pixel, adjustment})

	surface := doc.renderCompositeSurface()
	if len(surface) < 4 {
		t.Fatalf("renderCompositeSurface returned len=%d, want >= 4", len(surface))
	}
	return [4]byte{surface[0], surface[1], surface[2], surface[3]}
}

func maxByte(values ...uint8) uint8 {
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func minByte(values ...uint8) uint8 {
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}
