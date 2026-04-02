package engine

import (
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
