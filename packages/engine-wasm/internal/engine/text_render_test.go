package engine

import (
	"bytes"
	"testing"
)

// hasInk reports whether an RGBA buffer contains any non-transparent pixel.
func hasInk(buf []byte) bool {
	for i := 3; i < len(buf); i += 4 {
		if buf[i] > 0 {
			return true
		}
	}
	return false
}

func TestRasterizeTextLayer_PointTextProducesPixels(t *testing.T) {
	layer := NewTextLayer("Test", LayerBounds{X: 0, Y: 0, W: 200, H: 50}, "Hello", nil)
	layer.FontSize = 24
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.TextType = "point"

	if err := rasterizeTextLayer(layer); err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
	if got, want := len(layer.CachedRaster), layer.Bounds.W*layer.Bounds.H*4; got != want {
		t.Errorf("raster len = %d, want bounds-local %d (%dx%d)", got, want, layer.Bounds.W, layer.Bounds.H)
	}
	if !hasInk(layer.CachedRaster) {
		t.Error("expected ink pixels in rasterized text, got fully transparent buffer")
	}
}

func TestRasterizeTextLayer_EmptyTextReturnsTransparent(t *testing.T) {
	layer := NewTextLayer("Empty", LayerBounds{X: 0, Y: 0, W: 100, H: 50}, "", nil)
	layer.FontSize = 16

	if err := rasterizeTextLayer(layer); err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
	if got, want := len(layer.CachedRaster), layer.Bounds.W*layer.Bounds.H*4; got != want {
		t.Errorf("raster len = %d, want %d", got, want)
	}
	// Empty text → fully transparent buffer.
	if hasInk(layer.CachedRaster) {
		t.Error("expected fully transparent buffer for empty text")
	}
}

func TestRasterizeTextLayer_AreaTextProducesPixels(t *testing.T) {
	layer := NewTextLayer("Area", LayerBounds{X: 0, Y: 0, W: 100, H: 100}, "Hello world this is area text", nil)
	layer.FontSize = 16
	layer.TextType = "area"
	layer.Color = [4]uint8{0, 0, 0, 255}

	if err := rasterizeTextLayer(layer); err != nil {
		t.Fatalf("rasterizeTextLayer: %v", err)
	}
	if layer.Bounds != (LayerBounds{X: 0, Y: 0, W: 100, H: 100}) {
		t.Errorf("area bounds = %+v, want user frame preserved", layer.Bounds)
	}
	if !hasInk(layer.CachedRaster) {
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

	if err := rasterizeTextLayer(layerA); err != nil {
		t.Fatalf("rasterize A: %v", err)
	}
	if err := rasterizeTextLayer(layerB); err != nil {
		t.Fatalf("rasterize B: %v", err)
	}

	if layerA.Bounds == layerB.Bounds && bytes.Equal(layerA.CachedRaster, layerB.CachedRaster) {
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

func TestRasterizeTextLayer_TrackingWidensPointText(t *testing.T) {
	layerA := NewTextLayer("A", LayerBounds{X: 10, Y: 0, W: 400, H: 60}, "Hello", nil)
	layerA.FontSize = 24
	layerA.Color = [4]uint8{0, 0, 0, 255}
	layerA.TextType = "point"

	layerB := NewTextLayer("B", LayerBounds{X: 10, Y: 0, W: 400, H: 60}, "Hello", nil)
	layerB.FontSize = 24
	layerB.Color = [4]uint8{0, 0, 0, 255}
	layerB.TextType = "point"
	layerB.Tracking = 10

	if err := rasterizeTextLayer(layerA); err != nil {
		t.Fatalf("rasterize without tracking: %v", err)
	}
	if err := rasterizeTextLayer(layerB); err != nil {
		t.Fatalf("rasterize with tracking: %v", err)
	}
	if layerB.Bounds.W <= layerA.Bounds.W {
		t.Errorf("tracked bounds width = %d, want > untracked %d", layerB.Bounds.W, layerA.Bounds.W)
	}
}

func TestRasterizeTextLayer_UnderlineProducesPixels(t *testing.T) {
	layer := NewTextLayer("U", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layer.FontSize = 24
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.Underline = true

	if err := rasterizeTextLayer(layer); err != nil {
		t.Fatalf("rasterize underline: %v", err)
	}
	if !hasInk(layer.CachedRaster) {
		t.Error("expected ink pixels with underline text")
	}

	// Compare with non-underlined version.
	layerNoU := NewTextLayer("NU", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layerNoU.FontSize = 24
	layerNoU.Color = [4]uint8{0, 0, 0, 255}
	if err := rasterizeTextLayer(layerNoU); err != nil {
		t.Fatalf("rasterize plain: %v", err)
	}

	if layer.Bounds == layerNoU.Bounds && bytes.Equal(layer.CachedRaster, layerNoU.CachedRaster) {
		t.Error("underline text should differ from non-underlined text")
	}
}

func TestRasterizeTextLayer_StrikethroughProducesPixels(t *testing.T) {
	layer := NewTextLayer("S", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layer.FontSize = 24
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.Strikethrough = true

	if err := rasterizeTextLayer(layer); err != nil {
		t.Fatalf("rasterize strikethrough: %v", err)
	}

	layerNoS := NewTextLayer("NS", LayerBounds{X: 10, Y: 0, W: 300, H: 60}, "Hello", nil)
	layerNoS.FontSize = 24
	layerNoS.Color = [4]uint8{0, 0, 0, 255}
	if err := rasterizeTextLayer(layerNoS); err != nil {
		t.Fatalf("rasterize plain: %v", err)
	}

	if layer.Bounds == layerNoS.Bounds && bytes.Equal(layer.CachedRaster, layerNoS.CachedRaster) {
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

	if err := rasterizeTextLayer(layerCaps); err != nil {
		t.Fatalf("rasterize allCaps: %v", err)
	}
	if err := rasterizeTextLayer(layerUpper); err != nil {
		t.Fatalf("rasterize upper: %v", err)
	}
	if layerCaps.Bounds != layerUpper.Bounds || !bytes.Equal(layerCaps.CachedRaster, layerUpper.CachedRaster) {
		t.Error("AllCaps should produce same output as manually uppercased text")
	}
}

func TestRasterizeTextLayer_JustifyAlignment(t *testing.T) {
	layer := NewTextLayer("J", LayerBounds{X: 0, Y: 0, W: 200, H: 200}, "one two three four five six", nil)
	layer.FontSize = 16
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.TextType = "area"
	layer.Alignment = "justify"

	if err := rasterizeTextLayer(layer); err != nil {
		t.Fatalf("rasterize justify: %v", err)
	}
	if !hasInk(layer.CachedRaster) {
		t.Error("expected ink pixels with justified text")
	}

	// Should differ from left-aligned.
	layerLeft := NewTextLayer("L", LayerBounds{X: 0, Y: 0, W: 200, H: 200}, "one two three four five six", nil)
	layerLeft.FontSize = 16
	layerLeft.Color = [4]uint8{0, 0, 0, 255}
	layerLeft.TextType = "area"
	layerLeft.Alignment = "left"
	if err := rasterizeTextLayer(layerLeft); err != nil {
		t.Fatalf("rasterize left: %v", err)
	}

	if bytes.Equal(layer.CachedRaster, layerLeft.CachedRaster) {
		t.Error("justified text should differ from left-aligned text")
	}
}

func TestRasterizeTextLayer_IndentsShiftAreaText(t *testing.T) {
	plain := NewTextLayer("P", LayerBounds{X: 0, Y: 0, W: 200, H: 200}, "indented words wrap here nicely", nil)
	plain.FontSize = 16
	plain.Color = [4]uint8{0, 0, 0, 255}
	plain.TextType = "area"
	if err := rasterizeTextLayer(plain); err != nil {
		t.Fatalf("rasterize plain: %v", err)
	}
	pMinX, _, _, _, _, ok := inkBBox(plain.CachedRaster, plain.Bounds.W, plain.Bounds.H)
	if !ok {
		t.Fatal("plain area text has no ink")
	}

	indented := NewTextLayer("I", LayerBounds{X: 0, Y: 0, W: 200, H: 200}, "indented words wrap here nicely", nil)
	indented.FontSize = 16
	indented.Color = [4]uint8{0, 0, 0, 255}
	indented.TextType = "area"
	indented.IndentLeft = 25
	if err := rasterizeTextLayer(indented); err != nil {
		t.Fatalf("rasterize indented: %v", err)
	}
	iMinX, _, _, _, _, ok := inkBBox(indented.CachedRaster, indented.Bounds.W, indented.Bounds.H)
	if !ok {
		t.Fatal("indented area text has no ink")
	}

	if iMinX < pMinX+20 {
		t.Errorf("indented leftmost ink = %d, want >= %d (plain %d + ~25 indent)", iMinX, pMinX+20, pMinX)
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

	if err := rasterizeTextLayer(layerSingle); err != nil {
		t.Fatalf("rasterize single: %v", err)
	}
	if err := rasterizeTextLayer(layerPara); err != nil {
		t.Fatalf("rasterize para: %v", err)
	}
	if bytes.Equal(layerSingle.CachedRaster, layerPara.CachedRaster) {
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
