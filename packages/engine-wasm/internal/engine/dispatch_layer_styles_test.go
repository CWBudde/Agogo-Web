package engine

import (
	"encoding/json"
	"testing"
)

func mustRawJSON(t *testing.T, raw string) json.RawMessage {
	t.Helper()
	return json.RawMessage(raw)
}

func TestLayerStyleCommands_RoundTripThroughDispatch(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	added, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel,
		Name:      "Styled",
		Bounds:    LayerBounds{X: 0, Y: 0, W: 2, H: 2},
		Pixels: []byte{
			255, 0, 0, 255, 255, 0, 0, 255,
			255, 0, 0, 255, 255, 0, 0, 255,
		},
	}))
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	layerID := added.UIMeta.ActiveLayerID
	_, err = DispatchCommand(h, commandSetLayerStyleStack, mustJSON(t, SetLayerStyleStackPayload{
		LayerID: layerID,
		Styles: []LayerStylePayload{
			{
				Kind:    LayerStyleKindDropShadow,
				Enabled: true,
				Params:  mustRawJSON(t, `{"blendMode":"multiply","opacity":0.75,"distance":4,"size":6}`),
			},
		},
	}))
	if err == nil {
		t.Fatal("expected missing layer-style command implementation to fail before wiring")
	}
}
