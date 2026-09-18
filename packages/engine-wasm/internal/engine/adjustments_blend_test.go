package engine

import (
	"encoding/json"
	"testing"
)

// Adjustment layers honour their own opacity and blend mode.
//
// Until S.10.7 they did not: applyAdjustmentLayerToSurface took its coverage
// from the clip alpha and the layer mask only and never read Opacity() or
// BlendMode(), so a layer at 50% Multiply rendered full-strength Normal. The
// tests below are written against the composite, not against the transform, so
// they fail if the properties stop being threaded through at any point in the
// pipeline.

// renderAdjustedPixelWith renders one pixel through the real compositor with an
// adjustment layer configured by `configure`.
func renderAdjustedPixelWith(t *testing.T, kind, params string, base [4]byte, configure func(*AdjustmentLayer)) [4]byte {
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
	if configure != nil {
		configure(adjustment)
	}
	doc.LayerRoot.SetChildren([]LayerNode{pixel, adjustment})

	surface := doc.renderCompositeSurface()
	if len(surface) < 4 {
		t.Fatalf("renderCompositeSurface returned len=%d, want >= 4", len(surface))
	}
	return [4]byte{surface[0], surface[1], surface[2], surface[3]}
}

// TestAdjustmentLayerOpacityScalesTheEffect: invert at half opacity must land
// halfway between the original and the inverted colour.
func TestAdjustmentLayerOpacityScalesTheEffect(t *testing.T) {
	base := [4]byte{200, 100, 50, 255}

	full := renderAdjustedPixelWith(t, "invert", `{}`, base, nil)
	if full != [4]byte{55, 155, 205, 255} {
		t.Fatalf("invert at full opacity = %v, want the exact inverse {55 155 205 255}", full)
	}

	half := renderAdjustedPixelWith(t, "invert", `{}`, base, func(layer *AdjustmentLayer) {
		layer.SetOpacity(0.5)
	})
	for channel, want := range [3]int{127, 127, 127} {
		got := int(half[channel])
		if got < want-2 || got > want+2 {
			t.Errorf("channel %d at 50%% opacity = %d, want ~%d (halfway between %d and %d)",
				channel, got, want, base[channel], full[channel])
		}
	}
	if half[3] != 255 {
		t.Errorf("alpha = %d, want 255: opacity must not change the backdrop's coverage", half[3])
	}
}

// TestAdjustmentLayerAtZeroOpacityIsANoOp is the degenerate case, and the one a
// user notices first when the property is ignored.
func TestAdjustmentLayerAtZeroOpacityIsANoOp(t *testing.T) {
	base := [4]byte{200, 100, 50, 255}
	got := renderAdjustedPixelWith(t, "invert", `{}`, base, func(layer *AdjustmentLayer) {
		layer.SetOpacity(0)
	})
	if got != base {
		t.Fatalf("invert at zero opacity = %v, want the backdrop %v unchanged", got, base)
	}
}

// TestAdjustmentLayerBlendModeAppliesToTheResult: Multiply of the inverted
// colour with the backdrop is base*inverted/255, which differs from both the
// backdrop and the plain inverted result in every channel.
func TestAdjustmentLayerBlendModeAppliesToTheResult(t *testing.T) {
	base := [4]byte{200, 100, 50, 255}
	got := renderAdjustedPixelWith(t, "invert", `{}`, base, func(layer *AdjustmentLayer) {
		layer.SetBlendMode(BlendModeMultiply)
	})

	for channel := range 3 {
		inverted := 255 - int(base[channel])
		want := int(base[channel]) * inverted / 255
		if diff := int(got[channel]) - want; diff < -2 || diff > 2 {
			t.Errorf("channel %d under Multiply = %d, want ~%d (%d*%d/255)",
				channel, got[channel], want, base[channel], inverted)
		}
	}
	if got[3] != 255 {
		t.Errorf("alpha = %d, want 255: a blend mode must not change coverage", got[3])
	}
}

// TestAdjustmentLayerPreservesBackdropAlpha is the reason the merge is not a
// plain source-over composite. An adjustment recolours what is there; it adds no
// coverage of its own, so a half-transparent backdrop must stay half
// transparent whatever the blend mode and opacity are.
func TestAdjustmentLayerPreservesBackdropAlpha(t *testing.T) {
	base := [4]byte{200, 100, 50, 128}

	for _, test := range []struct {
		name      string
		configure func(*AdjustmentLayer)
	}{
		{"normal at full opacity", nil},
		{"reduced opacity", func(l *AdjustmentLayer) { l.SetOpacity(0.5) }},
		{"multiply", func(l *AdjustmentLayer) { l.SetBlendMode(BlendModeMultiply) }},
		{"screen at reduced opacity", func(l *AdjustmentLayer) {
			l.SetBlendMode(BlendModeScreen)
			l.SetOpacity(0.4)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := renderAdjustedPixelWith(t, "invert", `{}`, base, test.configure)
			if got[3] != base[3] {
				t.Errorf("alpha = %d, want the backdrop's %d: an adjustment layer must not add or remove coverage",
					got[3], base[3])
			}
		})
	}
}

// TestAdjustmentBlendPathMatchesDirectPathAtNormalFullOpacity pins the fast
// path against the general one. They are two implementations of the same
// operation, and the cheap one is only safe while it agrees.
func TestAdjustmentBlendPathMatchesDirectPathAtNormalFullOpacity(t *testing.T) {
	const docW, docH = 4, 3
	kinds := []struct {
		kind   string
		params string
	}{
		{"invert", `{}`},
		{"levels", `{"inputBlack":10,"inputWhite":240,"gamma":1.3,"channel":"rgb"}`},
		{"threshold", `{"threshold":120}`},
	}

	for _, entry := range kinds {
		t.Run(entry.kind, func(t *testing.T) {
			transform, err := lookupAdjustmentTransform(entry.kind, json.RawMessage(entry.params))
			if err != nil || transform == nil {
				t.Fatalf("lookupAdjustmentTransform(%q): %v", entry.kind, err)
			}
			layer := NewAdjustmentLayer("Adj", entry.kind, json.RawMessage(entry.params))
			rect := DirtyRect{X: 0, Y: 0, W: docW, H: docH}

			direct := gradientSurface(docW, docH)
			if err := applyAdjustmentRectDirect(direct, docW, layer, nil, json.RawMessage(entry.params), transform, rect); err != nil {
				t.Fatalf("applyAdjustmentRectDirect: %v", err)
			}

			blended := gradientSurface(docW, docH)
			if err := applyAdjustmentRectBlended(blended, docW, layer, nil, json.RawMessage(entry.params), transform, rect, BlendModeNormal, 1); err != nil {
				t.Fatalf("applyAdjustmentRectBlended: %v", err)
			}

			for index := range direct {
				if diff := int(direct[index]) - int(blended[index]); diff < -1 || diff > 1 {
					t.Fatalf("byte %d: direct=%d blended=%d; the fast path and the general path disagree",
						index, direct[index], blended[index])
				}
			}
		})
	}
}

// gradientSurface builds a deterministic RGBA surface with varying alpha, so a
// comparison covers translucent pixels rather than only opaque ones.
func gradientSurface(width, height int) []byte {
	surface := make([]byte, width*height*4)
	for index := range width * height {
		surface[index*4] = uint8((index * 37) % 256)    //nolint:gosec // deterministic test data
		surface[index*4+1] = uint8((index * 11) % 256)  //nolint:gosec // deterministic test data
		surface[index*4+2] = uint8((index * 73) % 256)  //nolint:gosec // deterministic test data
		surface[index*4+3] = uint8(128 + (index*7)%128) //nolint:gosec // deterministic test data
	}
	return surface
}

// TestAdjustmentCacheNoticesOpacityAndBlendChanges: the cache stores the surface
// AFTER the layer has been merged, so opacity and blend mode are part of its
// key. Without that, dragging the opacity slider would redraw from a cache
// computed at the old value.
func TestAdjustmentCacheNoticesOpacityAndBlendChanges(t *testing.T) {
	layer := NewAdjustmentLayer("Adj", "invert", json.RawMessage(`{}`))
	params := json.RawMessage(`{}`)
	const docW, docH = 2, 2
	surface := gradientSurface(docW, docH)

	updateAdjustmentCache(layer, "invert", params, docW, docH, surface)
	if !adjustmentCacheMatches(layer, "invert", params, docW, docH) {
		t.Fatal("the cache does not match the state it was just written from")
	}

	layer.SetOpacity(0.5)
	if adjustmentCacheMatches(layer, "invert", params, docW, docH) {
		t.Error("the cache still matches after the opacity changed")
	}

	updateAdjustmentCache(layer, "invert", params, docW, docH, surface)
	layer.SetBlendMode(BlendModeMultiply)
	if adjustmentCacheMatches(layer, "invert", params, docW, docH) {
		t.Error("the cache still matches after the blend mode changed")
	}
}
