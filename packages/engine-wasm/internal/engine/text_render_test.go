package engine

import (
	"bytes"
	"testing"
)

func TestRasterizeTextLayer_PointTextProducesPixels(t *testing.T) {
	layer := NewTextLayer("Test", LayerBounds{X: 0, Y: 0, W: 200, H: 50}, "Hello", nil)
	layer.FontSize = 24
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.TextType = "point"

	buf, err := rasterizeTextLayer(layer, 200, 50)
	if err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
	if len(buf) != 200*50*4 {
		t.Errorf("buf len = %d, want %d", len(buf), 200*50*4)
	}
	// At least some pixels must be non-transparent for visible text.
	hasInk := false
	for i := 3; i < len(buf); i += 4 {
		if buf[i] > 0 {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("expected ink pixels in rasterized text, got fully transparent buffer")
	}
}

func TestRasterizeTextLayer_EmptyTextReturnsTransparent(t *testing.T) {
	layer := NewTextLayer("Empty", LayerBounds{X: 0, Y: 0, W: 100, H: 50}, "", nil)
	layer.FontSize = 16

	buf, err := rasterizeTextLayer(layer, 100, 50)
	if err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
	if len(buf) != 100*50*4 {
		t.Errorf("buf len = %d, want %d", len(buf), 100*50*4)
	}
	// Empty text → fully transparent buffer.
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 0 {
			t.Errorf("expected alpha=0 at index %d, got %d", i, buf[i])
			return
		}
	}
}

func TestRasterizeTextLayer_AreaTextProducesPixels(t *testing.T) {
	layer := NewTextLayer("Area", LayerBounds{X: 0, Y: 0, W: 100, H: 100}, "Hello world this is area text", nil)
	layer.FontSize = 16
	layer.TextType = "area"
	layer.Color = [4]uint8{0, 0, 0, 255}

	buf, err := rasterizeTextLayer(layer, 100, 100)
	if err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
	hasInk := false
	for i := 3; i < len(buf); i += 4 {
		if buf[i] > 0 {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("expected ink pixels in area text, got fully transparent buffer")
	}
}

func TestRasterizeTextLayer_DifferentTextsProduceDifferentBuffers(t *testing.T) {
	layerA := NewTextLayer("A", LayerBounds{X: 0, Y: 0, W: 200, H: 50}, "Hello", nil)
	layerA.FontSize = 24
	layerA.Color = [4]uint8{0, 0, 0, 255}

	layerB := NewTextLayer("B", LayerBounds{X: 0, Y: 0, W: 200, H: 50}, "World", nil)
	layerB.FontSize = 24
	layerB.Color = [4]uint8{0, 0, 0, 255}

	bufA, _ := rasterizeTextLayer(layerA, 200, 50)
	bufB, _ := rasterizeTextLayer(layerB, 200, 50)

	if bytes.Equal(bufA, bufB) {
		t.Error("expected different rasters for different text strings")
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"  spaces  ", []string{"spaces"}},
		{"single", []string{"single"}},
		{"", nil},
		{"a b c", []string{"a", "b", "c"}},
	}
	for _, tc := range tests {
		got := splitWords(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("splitWords(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitWords(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestMeasureTextWidth(t *testing.T) {
	w := measureTextWidth("Hello", 16)
	if w <= 0 {
		t.Errorf("measureTextWidth returned %v, want > 0", w)
	}
	w0 := measureTextWidth("", 16)
	if w0 != 0 {
		t.Errorf("measureTextWidth empty string = %v, want 0", w0)
	}
}

func TestRasterizeTextLayer_TrackingProducesDifferentWidth(t *testing.T) {
	// Without tracking.
	layerA := NewTextLayer("A", LayerBounds{X: 10, Y: 0, W: 400, H: 60}, "Hello", nil)
	layerA.FontSize = 24
	layerA.Color = [4]uint8{0, 0, 0, 255}
	layerA.TextType = "point"

	// With tracking.
	layerB := NewTextLayer("B", LayerBounds{X: 10, Y: 0, W: 400, H: 60}, "Hello", nil)
	layerB.FontSize = 24
	layerB.Color = [4]uint8{0, 0, 0, 255}
	layerB.TextType = "point"
	layerB.Tracking = 10

	bufA, err := rasterizeTextLayer(layerA, 400, 60)
	if err != nil {
		t.Fatalf("rasterize without tracking: %v", err)
	}
	bufB, err := rasterizeTextLayer(layerB, 400, 60)
	if err != nil {
		t.Fatalf("rasterize with tracking: %v", err)
	}
	if bytes.Equal(bufA, bufB) {
		t.Error("expected tracking to produce different raster output")
	}
}

func TestRasterizeTextLayer_UnderlineProducesPixels(t *testing.T) {
	layer := NewTextLayer("U", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layer.FontSize = 24
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.Underline = true

	buf, err := rasterizeTextLayer(layer, 300, 60)
	if err != nil {
		t.Fatalf("rasterize underline: %v", err)
	}
	hasInk := false
	for i := 3; i < len(buf); i += 4 {
		if buf[i] > 0 {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("expected ink pixels with underline text")
	}

	// Compare with non-underlined version.
	layerNoU := NewTextLayer("NU", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layerNoU.FontSize = 24
	layerNoU.Color = [4]uint8{0, 0, 0, 255}
	bufNoU, _ := rasterizeTextLayer(layerNoU, 300, 60)

	if bytes.Equal(buf, bufNoU) {
		t.Error("underline text should differ from non-underlined text")
	}
}

func TestRasterizeTextLayer_StrikethroughProducesPixels(t *testing.T) {
	layer := NewTextLayer("S", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layer.FontSize = 24
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.Strikethrough = true

	buf, err := rasterizeTextLayer(layer, 300, 60)
	if err != nil {
		t.Fatalf("rasterize strikethrough: %v", err)
	}

	// Compare with non-strikethrough version.
	layerNoS := NewTextLayer("NS", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layerNoS.FontSize = 24
	layerNoS.Color = [4]uint8{0, 0, 0, 255}
	bufNoS, _ := rasterizeTextLayer(layerNoS, 300, 60)

	if bytes.Equal(buf, bufNoS) {
		t.Error("strikethrough text should differ from plain text")
	}
}

func TestRasterizeTextLayer_AllCapsTransformsText(t *testing.T) {
	// AllCaps should produce the same raster as manually uppercased text.
	layerCaps := NewTextLayer("Caps", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "hello", nil)
	layerCaps.FontSize = 24
	layerCaps.Color = [4]uint8{0, 0, 0, 255}
	layerCaps.AllCaps = true

	layerUpper := NewTextLayer("Upper", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "HELLO", nil)
	layerUpper.FontSize = 24
	layerUpper.Color = [4]uint8{0, 0, 0, 255}

	bufCaps, err := rasterizeTextLayer(layerCaps, 300, 60)
	if err != nil {
		t.Fatalf("rasterize allCaps: %v", err)
	}
	bufUpper, err := rasterizeTextLayer(layerUpper, 300, 60)
	if err != nil {
		t.Fatalf("rasterize upper: %v", err)
	}
	if !bytes.Equal(bufCaps, bufUpper) {
		t.Error("AllCaps should produce same output as manually uppercased text")
	}
}

func TestRasterizeTextLayer_JustifyAlignment(t *testing.T) {
	layer := NewTextLayer("J", LayerBounds{X: 0, Y: 0, W: 200, H: 200}, "one two three four five six", nil)
	layer.FontSize = 16
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.TextType = "area"
	layer.Alignment = "justify"

	buf, err := rasterizeTextLayer(layer, 200, 200)
	if err != nil {
		t.Fatalf("rasterize justify: %v", err)
	}
	hasInk := false
	for i := 3; i < len(buf); i += 4 {
		if buf[i] > 0 {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("expected ink pixels with justified text")
	}

	// Should differ from left-aligned.
	layerLeft := NewTextLayer("L", LayerBounds{X: 0, Y: 0, W: 200, H: 200}, "one two three four five six", nil)
	layerLeft.FontSize = 16
	layerLeft.Color = [4]uint8{0, 0, 0, 255}
	layerLeft.TextType = "area"
	layerLeft.Alignment = "left"
	bufLeft, _ := rasterizeTextLayer(layerLeft, 200, 200)

	if bytes.Equal(buf, bufLeft) {
		t.Error("justified text should differ from left-aligned text")
	}
}

func TestRasterizeTextLayer_ParagraphSpacing(t *testing.T) {
	// Single paragraph.
	layerSingle := NewTextLayer("S", LayerBounds{X: 0, Y: 0, W: 300, H: 300}, "Line one\nLine two", nil)
	layerSingle.FontSize = 16
	layerSingle.Color = [4]uint8{0, 0, 0, 255}
	layerSingle.TextType = "area"

	// Two paragraphs with spacing.
	layerPara := NewTextLayer("P", LayerBounds{X: 0, Y: 0, W: 300, H: 300}, "Line one\n\nLine two", nil)
	layerPara.FontSize = 16
	layerPara.Color = [4]uint8{0, 0, 0, 255}
	layerPara.TextType = "area"
	layerPara.SpaceBefore = 10
	layerPara.SpaceAfter = 10

	bufSingle, err := rasterizeTextLayer(layerSingle, 300, 300)
	if err != nil {
		t.Fatalf("rasterize single: %v", err)
	}
	bufPara, err := rasterizeTextLayer(layerPara, 300, 300)
	if err != nil {
		t.Fatalf("rasterize para: %v", err)
	}
	if bytes.Equal(bufSingle, bufPara) {
		t.Error("paragraph spacing should produce different output than single-paragraph text")
	}
}

func TestApplyCapsTransform(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		allCaps   bool
		smallCaps bool
		want      string
	}{
		{"no transform", "Hello World", false, false, "Hello World"},
		{"allCaps", "Hello World", true, false, "HELLO WORLD"},
		{"smallCaps", "Hello World", false, true, "HELLO WORLD"},
		{"both", "Hello World", true, true, "HELLO WORLD"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyCapsTransform(tc.text, tc.allCaps, tc.smallCaps)
			if got != tc.want {
				t.Errorf("applyCapsTransform(%q, %v, %v) = %q, want %q",
					tc.text, tc.allCaps, tc.smallCaps, got, tc.want)
			}
		})
	}
}

func TestSplitParagraphs(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 1},
		{"hello\nworld", 1},
		{"hello\n\nworld", 2},
		{"a\n\nb\n\nc", 3},
	}
	for _, tc := range tests {
		got := splitParagraphs(tc.input)
		if len(got) != tc.want {
			t.Errorf("splitParagraphs(%q) = %d parts, want %d", tc.input, len(got), tc.want)
		}
	}
}
