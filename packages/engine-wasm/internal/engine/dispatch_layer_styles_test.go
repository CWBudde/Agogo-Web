package engine

import (
	"encoding/json"
	"fmt"
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
	tests := []struct {
		name      string
		commandID int32
		payload   any
	}{
		{
			name:      "SetLayerStyleStack",
			commandID: commandSetLayerStyleStack,
			payload: SetLayerStyleStackPayload{
				LayerID: layerID,
				Styles: []LayerStylePayload{
					{
						Kind:    LayerStyleKindDropShadow,
						Enabled: true,
						Params:  mustRawJSON(t, `{"blendMode":"multiply","opacity":0.75,"distance":4,"size":6}`),
					},
				},
			},
		},
		{
			name:      "SetLayerStyleEnabled",
			commandID: commandSetLayerStyleEnabled,
			payload: SetLayerStyleEnabledPayload{
				LayerID: layerID,
				Kind:    LayerStyleKindDropShadow,
				Enabled: true,
			},
		},
		{
			name:      "SetLayerStyleParams",
			commandID: commandSetLayerStyleParams,
			payload: SetLayerStyleParamsPayload{
				LayerID: layerID,
				Kind:    LayerStyleKindDropShadow,
				Params:  mustRawJSON(t, `{"blendMode":"multiply","opacity":0.75,"distance":4,"size":6}`),
			},
		},
		{
			name:      "CopyLayerStyle",
			commandID: commandCopyLayerStyle,
			payload:   CopyLayerStylePayload{LayerID: layerID},
		},
		{
			name:      "PasteLayerStyle",
			commandID: commandPasteLayerStyle,
			payload:   PasteLayerStylePayload{LayerID: layerID},
		},
		{
			name:      "ClearLayerStyle",
			commandID: commandClearLayerStyle,
			payload:   ClearLayerStylePayload{LayerID: layerID},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DispatchCommand(h, tc.commandID, mustJSON(t, tc.payload))
			if err == nil {
				t.Fatalf("expected %s to fail before handlers are wired", tc.name)
			}
			wantErr := fmt.Sprintf("unsupported layer command id 0x%04x", tc.commandID)
			if err.Error() != wantErr {
				t.Fatalf("%s error = %q, want %q", tc.name, err.Error(), wantErr)
			}
		})
	}
}
