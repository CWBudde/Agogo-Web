package engine

import (
	"strings"
	"testing"
)

// BenchmarkTextEditKeystroke measures the per-keystroke cost of live text
// editing on a ~200-character point-text layer. Each iteration dispatches
// commandTextEditInput — the exact command the frontend sends on every
// keystroke — with a full ~200-char string whose trailing rune cycles, so
// glyph caches stay warm (realistic) but the whole line re-shapes and the
// layer re-rasterizes every time.
func BenchmarkTextEditKeystroke(b *testing.B) {
	h := Init(`{"documentWidth":1024,"documentHeight":768,"background":"transparent","resolution":72}`)
	if h <= 0 {
		b.Fatalf("Init returned invalid handle %d", h)
	}
	defer Free(h)

	// Create a point-text layer; AddTextLayer also enters text edit mode.
	addResult, err := DispatchCommand(h, commandAddTextLayer, mustJSONTB(b, AddTextLayerPayload{
		X:        20,
		Y:        60,
		FontSize: 24,
		Color:    [4]uint8{0, 0, 0, 255},
		TextType: "point",
	}))
	if err != nil {
		b.Fatalf("AddTextLayer: %v", err)
	}
	if addResult.UIMeta.EditingTextLayerID == "" {
		b.Fatal("expected AddTextLayer to enter text edit mode")
	}

	// ~200-character base line; the final rune cycles per iteration so every
	// keystroke carries a slightly different full string, like real typing.
	base := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 5)[:199]
	payloads := make([]string, 26)
	for i := range payloads {
		payloads[i] = mustJSONTB(b, TextEditInputPayload{Text: base + string(rune('a'+i))})
	}

	// Warm up shaping/raster paths once outside the timed loop.
	if _, err := DispatchCommand(h, commandTextEditInput, payloads[0]); err != nil {
		b.Fatalf("TextEditInput warm-up: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DispatchCommand(h, commandTextEditInput, payloads[i%len(payloads)]); err != nil {
			b.Fatalf("TextEditInput: %v", err)
		}
	}
}
