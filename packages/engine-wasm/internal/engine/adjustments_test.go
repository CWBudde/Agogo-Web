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
			got := renderAdjustmentTestPixel(t, tc.kind, tc.params, base)
			tc.check(t, got, base)
		})
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
