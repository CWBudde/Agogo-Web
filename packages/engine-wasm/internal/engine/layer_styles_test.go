package engine

import (
	"encoding/json"
	"testing"
)

func TestDecodeLayerStyles_NormalizesInvalidParams(t *testing.T) {
	styles := []LayerStyle{
		{
			Kind:    string(LayerStyleKindDropShadow),
			Enabled: true,
			Params:  json.RawMessage(`{"blendMode":"bad-mode","opacity":9,"distance":-3,"size":-1}`),
		},
		{
			Kind:    "future-style",
			Enabled: true,
			Params:  json.RawMessage(`{"foo":"bar"}`),
		},
	}

	decoded := decodeLayerStyles(styles)
	if len(decoded) != 2 {
		t.Fatalf("decoded len = %d, want 2", len(decoded))
	}
	if decoded[0].Kind != string(LayerStyleKindDropShadow) || !decoded[0].Enabled {
		t.Fatalf("decoded[0] = %+v", decoded[0])
	}
	if decoded[0].DropShadow.BlendMode != BlendModeMultiply {
		t.Fatalf("drop shadow blend mode = %q, want %q", decoded[0].DropShadow.BlendMode, BlendModeMultiply)
	}
	if decoded[0].DropShadow.Opacity != 1 || decoded[0].DropShadow.Distance != 0 || decoded[0].DropShadow.Size != 0 {
		t.Fatalf("normalized params = %+v", decoded[0].DropShadow)
	}
	if decoded[1].Enabled {
		t.Fatal("unknown style kinds must decode as disabled no-ops")
	}
}

func TestDecodeLayerStyles_MalformedParamsFailSafeToDefaults(t *testing.T) {
	styles := []LayerStyle{
		{
			Kind:    string(LayerStyleKindDropShadow),
			Enabled: true,
			Params:  json.RawMessage(`{"opacity":"bad"`),
		},
	}

	decoded := decodeLayerStyles(styles)
	if len(decoded) != 1 {
		t.Fatalf("decoded len = %d, want 1", len(decoded))
	}
	if !decoded[0].Enabled {
		t.Fatal("known style kind should remain enabled when params are malformed")
	}
	if decoded[0].DropShadow.BlendMode != BlendModeMultiply {
		t.Fatalf("drop shadow blend mode = %q, want %q", decoded[0].DropShadow.BlendMode, BlendModeMultiply)
	}
	if decoded[0].DropShadow.Opacity != 0.75 || decoded[0].DropShadow.Angle != 120 {
		t.Fatalf("drop shadow defaults = %+v, want default params", decoded[0].DropShadow)
	}
}

func TestDecodeLayerStyles_InvalidFieldTypesFailSafeToDefaults(t *testing.T) {
	styles := []LayerStyle{
		{
			Kind:    string(LayerStyleKindDropShadow),
			Enabled: true,
			Params:  json.RawMessage(`{"angle":45,"opacity":"bad"}`),
		},
	}

	decoded := decodeLayerStyles(styles)
	if len(decoded) != 1 {
		t.Fatalf("decoded len = %d, want 1", len(decoded))
	}
	if !decoded[0].Enabled {
		t.Fatal("known style kind should remain enabled when params have invalid field types")
	}
	if decoded[0].DropShadow.BlendMode != BlendModeMultiply {
		t.Fatalf("drop shadow blend mode = %q, want %q", decoded[0].DropShadow.BlendMode, BlendModeMultiply)
	}
	if decoded[0].DropShadow.Opacity != 0.75 || decoded[0].DropShadow.Angle != 120 {
		t.Fatalf("drop shadow defaults = %+v, want default params", decoded[0].DropShadow)
	}
}

func TestDecodeLayerStyles_StrokeInvalidEnumsAndShortColorFallbackToDefaults(t *testing.T) {
	styles := []LayerStyle{
		{
			Kind:    string(LayerStyleKindStroke),
			Enabled: true,
			Params:  json.RawMessage(`{"position":"sideways","fillType":"noise","color":[255,0,0]}`),
		},
	}

	decoded := decodeLayerStyles(styles)
	if len(decoded) != 1 {
		t.Fatalf("decoded len = %d, want 1", len(decoded))
	}
	if decoded[0].Stroke.Position != "outside" || decoded[0].Stroke.FillType != "color" {
		t.Fatalf("stroke enums = %+v, want default normalized values", decoded[0].Stroke)
	}
	if decoded[0].Stroke.Color != ([4]uint8{0, 0, 0, 255}) {
		t.Fatalf("stroke color = %v, want default color", decoded[0].Stroke.Color)
	}
}

func TestDecodeLayerStyles_BevelEmbossInvalidEnumsFallbackToDefaults(t *testing.T) {
	styles := []LayerStyle{
		{
			Kind:    string(LayerStyleKindBevelEmboss),
			Enabled: true,
			Params:  json.RawMessage(`{"style":"weird","technique":"rough","direction":"sideways","contour":"stairs"}`),
		},
	}

	decoded := decodeLayerStyles(styles)
	if len(decoded) != 1 {
		t.Fatalf("decoded len = %d, want 1", len(decoded))
	}
	if decoded[0].BevelEmboss.Style != "inner-bevel" ||
		decoded[0].BevelEmboss.Technique != "smooth" ||
		decoded[0].BevelEmboss.Direction != "up" ||
		decoded[0].BevelEmboss.Contour != "linear" {
		t.Fatalf("bevel enums = %+v, want default normalized values", decoded[0].BevelEmboss)
	}
}

func TestRenderStyledLayerSurface_UsesFillOpacityForBaseButNotEffects(t *testing.T) {
	doc := &Document{Width: 2, Height: 1, LayerRoot: NewGroupLayer("Root")}
	layer := NewPixelLayer("Styled", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{
		255, 0, 0, 255,
	})
	layer.SetFillOpacity(0)
	layer.SetStyleStack([]LayerStyle{
		{
			Kind:    string(LayerStyleKindColorOverlay),
			Enabled: true,
			Params:  jsonRawMessage(`{"color":[0,255,0,255],"opacity":1}`),
		},
		{
			Kind:    string(LayerStyleKindDropShadow),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"normal","color":[0,0,255,255],"opacity":1,"angle":0,"distance":1,"size":0}`),
		},
	})

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render styled layer: %v", err)
	}

	if got := surface[:4]; got[0] != 0 || got[1] != 255 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("base pixel = %v, want opaque green color overlay with fill-hidden source", got)
	}
	if got := surface[4:8]; got[0] != 0 || got[1] != 0 || got[2] != 255 || got[3] != 255 {
		t.Fatalf("shadow pixel = %v, want opaque blue drop shadow", got)
	}
}

func TestRenderStyledLayerSurface_AppliesEffectsInStableOrder(t *testing.T) {
	doc := &Document{
		Width:      7,
		Height:     7,
		Background: parseBackground("transparent"),
		LayerRoot:  NewGroupLayer("Root"),
	}

	layer := NewPixelLayer("Styled", LayerBounds{X: 3, Y: 3, W: 1, H: 1}, []byte{
		255, 255, 255, 255,
	})
	layer.SetStyleStack([]LayerStyle{
		{
			Kind:    string(LayerStyleKindDropShadow),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"normal","color":[0,0,255,255],"opacity":1,"distance":1,"angle":0,"size":0}`),
		},
		{
			Kind:    string(LayerStyleKindColorOverlay),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"normal","color":[255,0,0,255],"opacity":1}`),
		},
		{
			Kind:    string(LayerStyleKindStroke),
			Enabled: true,
			Params:  jsonRawMessage(`{"size":1,"position":"outside","fillType":"color","blendMode":"normal","color":[0,255,0,255],"opacity":1}`),
		},
	})

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render styled layer: %v", err)
	}

	if got := rgbaAt(surface, doc.Width, 3, 3); got != ([4]uint8{255, 0, 0, 255}) {
		t.Fatalf("base pixel = %v, want opaque red fill effect result", got)
	}
	if got := rgbaAt(surface, doc.Width, 2, 3); got != ([4]uint8{0, 255, 0, 255}) {
		t.Fatalf("stroke-only pixel = %v, want opaque green stroke result", got)
	}
	if got := rgbaAt(surface, doc.Width, 4, 3); got != ([4]uint8{0, 0, 255, 255}) {
		t.Fatalf("overlap pixel = %v, want outer-effect blue to win over stroke on stable grouped order", got)
	}
}

func TestRenderStyledLayerSurface_PreservesStableOrderForDuplicateKinds(t *testing.T) {
	doc := &Document{
		Width:      3,
		Height:     3,
		Background: parseBackground("transparent"),
		LayerRoot:  NewGroupLayer("Root"),
	}

	layer := NewPixelLayer("Styled", LayerBounds{X: 1, Y: 1, W: 1, H: 1}, []byte{
		255, 255, 255, 255,
	})
	layer.SetStyleStack([]LayerStyle{
		{
			Kind:    string(LayerStyleKindColorOverlay),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"normal","color":[255,0,0,255],"opacity":1}`),
		},
		{
			Kind:    string(LayerStyleKindColorOverlay),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"normal","color":[0,255,0,255],"opacity":1}`),
		},
	})

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render styled layer: %v", err)
	}

	if got := rgbaAt(surface, doc.Width, 1, 1); got != ([4]uint8{0, 255, 0, 255}) {
		t.Fatalf("center pixel = %v, want second same-kind overlay to remain later in stable order", got)
	}
}

func TestRenderStyledLayerSurface_RespectsEffectOpacityAndBlendMode(t *testing.T) {
	doc := &Document{
		Width:      1,
		Height:     1,
		Background: parseBackground("transparent"),
		LayerRoot:  NewGroupLayer("Root"),
	}

	layer := NewPixelLayer("Styled", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{
		255, 255, 255, 255,
	})
	layer.SetStyleStack([]LayerStyle{
		{
			Kind:    string(LayerStyleKindColorOverlay),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"multiply","color":[0,0,0,255],"opacity":0.5}`),
		},
	})

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render styled layer: %v", err)
	}

	if got := rgbaAt(surface, doc.Width, 0, 0); got != ([4]uint8{128, 128, 128, 255}) {
		t.Fatalf("pixel = %v, want 50%% multiply over white to produce mid gray", got)
	}
}

func TestRenderStyledLayerSurface_ComposesPartialOverlapInGroupedOrder(t *testing.T) {
	doc := &Document{
		Width:      7,
		Height:     7,
		Background: parseBackground("transparent"),
		LayerRoot:  NewGroupLayer("Root"),
	}

	layer := NewPixelLayer("Styled", LayerBounds{X: 3, Y: 3, W: 1, H: 1}, []byte{
		255, 255, 255, 255,
	})
	layer.SetStyleStack([]LayerStyle{
		{
			Kind:    string(LayerStyleKindStroke),
			Enabled: true,
			Params:  jsonRawMessage(`{"size":1,"position":"outside","fillType":"color","blendMode":"normal","color":[0,255,0,255],"opacity":0.5}`),
		},
		{
			Kind:    string(LayerStyleKindDropShadow),
			Enabled: true,
			Params:  jsonRawMessage(`{"blendMode":"normal","color":[0,0,255,255],"opacity":0.5,"distance":1,"angle":0,"size":0}`),
		},
	})

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("render styled layer: %v", err)
	}

	if got := rgbaAt(surface, doc.Width, 4, 3); got != ([4]uint8{0, 85, 170, 192}) {
		t.Fatalf("overlap pixel = %v, want exact blue-over-green partial composite in grouped order", got)
	}
}

func TestRenderStyledLayerSurface_RendersSupportedEffectCatalog(t *testing.T) {
	tests := []struct {
		name   string
		style  LayerStyle
		assert func(t *testing.T, surface []byte, width int)
	}{
		{
			name: "color overlay",
			style: LayerStyle{
				Kind:    string(LayerStyleKindColorOverlay),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","color":[255,0,0,255],"opacity":1}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 4, 4); got != ([4]uint8{255, 0, 0, 255}) {
					t.Fatalf("center pixel = %v, want opaque red color overlay", got)
				}
			},
		},
		{
			name: "gradient overlay",
			style: LayerStyle{
				Kind:    string(LayerStyleKindGradientOverlay),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","opacity":1,"angle":0,"scale":1}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 4, 4); got == ([4]uint8{255, 255, 255, 255}) {
					t.Fatalf("center pixel = %v, want deterministic non-white gradient result", got)
				}
			},
		},
		{
			name: "pattern overlay",
			style: LayerStyle{
				Kind:    string(LayerStyleKindPatternOverlay),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","opacity":1,"scale":1}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 4, 4); got == ([4]uint8{255, 255, 255, 255}) {
					t.Fatalf("center pixel = %v, want deterministic patterned fill result", got)
				}
			},
		},
		{
			name: "stroke",
			style: LayerStyle{
				Kind:    string(LayerStyleKindStroke),
				Enabled: true,
				Params:  jsonRawMessage(`{"size":1,"position":"outside","fillType":"color","blendMode":"normal","color":[0,255,0,255],"opacity":1}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 2, 4); got != ([4]uint8{0, 255, 0, 255}) {
					t.Fatalf("stroke pixel = %v, want opaque green stroke", got)
				}
			},
		},
		{
			name: "drop shadow",
			style: LayerStyle{
				Kind:    string(LayerStyleKindDropShadow),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","color":[0,0,255,255],"opacity":1,"distance":1,"angle":0,"size":0}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 6, 4); got != ([4]uint8{0, 0, 255, 255}) {
					t.Fatalf("shadow pixel = %v, want opaque blue drop shadow", got)
				}
			},
		},
		{
			name: "inner shadow",
			style: LayerStyle{
				Kind:    string(LayerStyleKindInnerShadow),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","color":[0,0,0,255],"opacity":1,"distance":1,"angle":0,"size":0}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 3, 4); got != ([4]uint8{0, 0, 0, 255}) {
					t.Fatalf("inner-shadow pixel = %v, want opaque black interior edge", got)
				}
			},
		},
		{
			name: "outer glow",
			style: LayerStyle{
				Kind:    string(LayerStyleKindOuterGlow),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","color":[255,128,0,255],"opacity":1,"spread":1,"size":1}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 2, 4); got[3] == 0 || got[0] == 0 {
					t.Fatalf("outer-glow pixel = %v, want non-transparent warm glow outside the layer", got)
				}
			},
		},
		{
			name: "inner glow",
			style: LayerStyle{
				Kind:    string(LayerStyleKindInnerGlow),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","color":[0,255,255,255],"opacity":1,"spread":0,"size":1}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				if got := rgbaAt(surface, width, 3, 4); got[1] == 0 || got[2] == 0 {
					t.Fatalf("inner-glow pixel = %v, want visible cyan glow on the inner edge", got)
				}
			},
		},
		{
			name: "bevel emboss",
			style: LayerStyle{
				Kind:    string(LayerStyleKindBevelEmboss),
				Enabled: true,
				Params:  jsonRawMessage(`{"style":"inner-bevel","technique":"smooth","depth":1,"direction":"up","size":1,"soften":0,"angle":0,"altitude":30,"highlightBlendMode":"normal","highlightColor":[255,0,0,255],"highlightOpacity":1,"shadowBlendMode":"normal","shadowColor":[0,0,255,255],"shadowOpacity":1}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				left := rgbaAt(surface, width, 3, 4)
				right := rgbaAt(surface, width, 5, 4)
				if left[0] == 0 || right[2] == 0 {
					t.Fatalf("bevel pixels = left %v right %v, want colored highlight and shadow lobes", left, right)
				}
			},
		},
		{
			name: "satin",
			style: LayerStyle{
				Kind:    string(LayerStyleKindSatin),
				Enabled: true,
				Params:  jsonRawMessage(`{"blendMode":"normal","color":[255,0,255,255],"opacity":1,"distance":1,"angle":0,"size":0,"invert":false}`),
			},
			assert: func(t *testing.T, surface []byte, width int) {
				left := rgbaAt(surface, width, 3, 4)
				right := rgbaAt(surface, width, 5, 4)
				if left[0] == 0 || right[0] == 0 {
					t.Fatalf("satin pixels = left %v right %v, want visible interior lobes", left, right)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := &Document{
				Width:      9,
				Height:     9,
				Background: parseBackground("transparent"),
				LayerRoot:  NewGroupLayer("Root"),
			}

			pixels := make([]byte, 3*3*4)
			for offset := 0; offset < len(pixels); offset += 4 {
				copy(pixels[offset:offset+4], []byte{255, 255, 255, 255})
			}

			layer := NewPixelLayer("Styled", LayerBounds{X: 3, Y: 3, W: 3, H: 3}, pixels)
			layer.SetStyleStack([]LayerStyle{test.style})

			surface, err := doc.renderLayerToSurface(layer)
			if err != nil {
				t.Fatalf("render styled layer: %v", err)
			}

			test.assert(t, surface, doc.Width)
		})
	}
}

func TestRenderStyledLayerSurface_RejectsNonRasterizableStyledLayers(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}

	group := NewGroupLayer("Styled Group")
	group.SetStyleStack([]LayerStyle{{
		Kind:    string(LayerStyleKindColorOverlay),
		Enabled: true,
		Params:  jsonRawMessage(`{"color":[0,255,0,255],"opacity":1}`),
	}})
	group.SetChildren([]LayerNode{
		NewPixelLayer("Child", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{255, 0, 0, 255}),
	})
	if _, err := doc.renderLayerToSurface(group); err == nil {
		t.Fatal("expected styled group render to fail until non-rasterizable layer styles are supported")
	}

	adjustment := NewAdjustmentLayer("Styled Adjustment", "invert", nil)
	adjustment.SetStyleStack([]LayerStyle{{
		Kind:    string(LayerStyleKindColorOverlay),
		Enabled: true,
		Params:  jsonRawMessage(`{"color":[0,255,0,255],"opacity":1}`),
	}})
	if _, err := doc.renderLayerToSurface(adjustment); err == nil {
		t.Fatal("expected styled adjustment render to fail until non-rasterizable layer styles are supported")
	}
}

func TestRenderStyledLayerSurface_AllowsNonRasterizableLayersWithDisabledStyles(t *testing.T) {
	doc := &Document{Width: 1, Height: 1, LayerRoot: NewGroupLayer("Root")}

	group := NewGroupLayer("Disabled Styled Group")
	group.SetStyleStack([]LayerStyle{{
		Kind:    string(LayerStyleKindColorOverlay),
		Enabled: false,
		Params:  jsonRawMessage(`{"color":[0,255,0,255],"opacity":1}`),
	}})
	group.SetChildren([]LayerNode{
		NewPixelLayer("Child", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{255, 0, 0, 255}),
	})
	if _, err := doc.renderLayerToSurface(group); err != nil {
		t.Fatalf("render disabled styled group: %v", err)
	}

	adjustment := NewAdjustmentLayer("Disabled Styled Adjustment", "invert", nil)
	adjustment.SetStyleStack([]LayerStyle{{
		Kind:    string(LayerStyleKindColorOverlay),
		Enabled: false,
		Params:  jsonRawMessage(`{"color":[0,255,0,255],"opacity":1}`),
	}})
	if _, err := doc.renderLayerToSurface(adjustment); err != nil {
		t.Fatalf("render disabled styled adjustment: %v", err)
	}
}

func TestRenderStyledLayerSurface_DefaultsMalformedStylesInsteadOfFailing(t *testing.T) {
	doc := &Document{
		Width:      1,
		Height:     1,
		Background: parseBackground("transparent"),
		LayerRoot:  NewGroupLayer("Root"),
	}
	layer := NewPixelLayer("Bad", LayerBounds{X: 0, Y: 0, W: 1, H: 1}, []byte{
		255, 255, 255, 255,
	})
	layer.SetStyleStack([]LayerStyle{{
		Kind:    string(LayerStyleKindDropShadow),
		Enabled: true,
		Params:  jsonRawMessage(`{"opacity":"bad"}`),
	}})

	surface, err := doc.renderLayerToSurface(layer)
	if err != nil {
		t.Fatalf("renderLayerToSurface should fail safe, got %v", err)
	}
	if got := rgbaAt(surface, doc.Width, 0, 0); got != ([4]uint8{64, 64, 64, 255}) {
		t.Fatalf("malformed style should fall back to default drop shadow, got %v", got)
	}
}

func rgbaAt(surface []byte, width, x, y int) [4]uint8 {
	offset := (y*width + x) * 4
	return [4]uint8{
		surface[offset],
		surface[offset+1],
		surface[offset+2],
		surface[offset+3],
	}
}
