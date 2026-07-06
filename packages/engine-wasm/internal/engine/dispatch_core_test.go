package engine

import (
	"testing"

	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
)

// TestPointerEventButtonRoundTrip verifies the ABI contract for pointer
// events: the `button` field the frontend sends (PointerEventCommand in
// packages/proto) survives the JSON decode into CorePointerEventPayload and
// the conversion into the engine's PointerEventPayload.
func TestPointerEventButtonRoundTrip(t *testing.T) {
	payloadJSON := mustJSONTB(t, map[string]any{
		"phase":     "down",
		"pointerId": 7,
		"x":         12.5,
		"y":         -3.0,
		"button":    2,
		"buttons":   2,
		"panMode":   false,
		"pressure":  0.5,
	})

	var got PointerEventPayload
	handled, err := cmdpkg.DispatchCore(commandPointerEvent, payloadJSON, cmdpkg.CoreDeps{
		Decode: decodePayloadAny,
		PointerEvent: func(payload cmdpkg.CorePointerEventPayload) error {
			got = corePointerEventToEngine(payload)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("DispatchCore(PointerEvent): %v", err)
	}
	if !handled {
		t.Fatal("DispatchCore did not handle commandPointerEvent")
	}

	want := PointerEventPayload{
		Phase:     "down",
		PointerID: 7,
		X:         12.5,
		Y:         -3.0,
		Button:    2,
		Buttons:   2,
		PanMode:   false,
		Pressure:  0.5,
	}
	if got != want {
		t.Fatalf("engine payload = %+v, want %+v", got, want)
	}
}
