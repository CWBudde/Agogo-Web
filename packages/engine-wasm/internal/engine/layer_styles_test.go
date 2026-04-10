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
