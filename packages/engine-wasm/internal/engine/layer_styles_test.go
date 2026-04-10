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
