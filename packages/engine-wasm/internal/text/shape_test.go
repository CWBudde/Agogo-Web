package text

import (
	"math"
	"reflect"
	"testing"

	"codeberg.org/go-fonts/dejavu/dejavusans"
)

func shapeWidth(t *testing.T, face *Face, s string, opts ShapeOptions) float64 {
	t.Helper()
	_, w := face.ShapeLine(s, opts)
	return w
}

func TestBoldAdvanceDiffers(t *testing.T) {
	reg := DefaultRegistry()
	regular := reg.Resolve("DejaVu Sans", false, false)
	bold := reg.Resolve("DejaVu Sans", true, false)

	opts := ShapeOptions{Size: 24}
	wRegular := shapeWidth(t, regular, "H", opts)
	wBold := shapeWidth(t, bold, "H", opts)
	if wRegular <= 0 || wBold <= 0 {
		t.Fatalf("advances must be positive: regular=%f bold=%f", wRegular, wBold)
	}
	if wRegular == wBold {
		t.Errorf("bold 'H' advance %f must differ from regular %f at size 24", wBold, wRegular)
	}
}

func TestPairKerning(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	opts := ShapeOptions{Size: 48}

	wAV := shapeWidth(t, face, "AV", opts)
	wA := shapeWidth(t, face, "A", opts)
	wV := shapeWidth(t, face, "V", opts)
	if wAV >= wA+wV {
		t.Errorf("kerned width(\"AV\")=%f, want < width(\"A\")+width(\"V\")=%f (DejaVu has a kern table)", wAV, wA+wV)
	}
}

func TestManualKerning(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	const size = 48.0

	tests := []struct {
		name  string
		text  string
		pairs int
	}{
		{"two glyphs", "AV", 1},
		{"three glyphs", "AVA", 2},
		{"five glyphs", "Hello", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := shapeWidth(t, face, tt.text, ShapeOptions{Size: size})
			kerned := shapeWidth(t, face, tt.text, ShapeOptions{Size: size, Kerning: 100})
			want := float64(tt.pairs) * 0.1 * size // 100/1000 em per pair
			if got := kerned - base; math.Abs(got-want) > 1e-9 {
				t.Errorf("manual kerning delta = %f, want %f", got, want)
			}
		})
	}
}

func TestTrackingArithmetic(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	const size = 24.0

	tests := []struct {
		name     string
		text     string
		tracking float64
		glyphs   int
	}{
		{"five glyphs", "Hello", 3.5, 5},
		{"two glyphs", "Hi", 10, 2},
		{"single glyph no tracking applied", "X", 100, 1},
		{"negative tracking", "abc", -2, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := shapeWidth(t, face, tt.text, ShapeOptions{Size: size})
			tracked := shapeWidth(t, face, tt.text, ShapeOptions{Size: size, Tracking: tt.tracking})
			want := float64(tt.glyphs-1) * tt.tracking
			if got := tracked - base; math.Abs(got-want) > 1e-9 {
				t.Errorf("tracking delta = %f, want (n-1)*T = %f", got, want)
			}
		})
	}
}

func TestSmallCaps(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	const size = 40.0

	upperA, err := face.GlyphIndex('A')
	if err != nil {
		t.Fatalf("GlyphIndex('A'): %v", err)
	}

	glyphs, _ := face.ShapeLine("a", ShapeOptions{Size: size, SmallCaps: true})
	if len(glyphs) != 1 {
		t.Fatalf("shaped %d glyphs, want 1", len(glyphs))
	}
	if glyphs[0].Glyph != upperA {
		t.Errorf("small-caps 'a' shaped glyph %d, want 'A' glyph %d", glyphs[0].Glyph, upperA)
	}
	if math.Abs(glyphs[0].Size-0.7*size) > 1e-9 {
		t.Errorf("small-caps glyph size = %f, want %f", glyphs[0].Size, 0.7*size)
	}

	// Uppercase runes keep the full size.
	capGlyphs, _ := face.ShapeLine("A", ShapeOptions{Size: size, SmallCaps: true})
	if len(capGlyphs) != 1 || capGlyphs[0].Size != size {
		t.Fatalf("small-caps 'A' must keep full size, got %+v", capGlyphs)
	}

	// Ink height of small-cap 'a' must be below the full-size 'A' ink height.
	small := newBBoxSink()
	if err := face.AppendGlyphOutline(glyphs[0].Glyph, glyphs[0].Size, 0, 0, small); err != nil {
		t.Fatalf("AppendGlyphOutline small: %v", err)
	}
	full := newBBoxSink()
	if err := face.AppendGlyphOutline(upperA, size, 0, 0, full); err != nil {
		t.Fatalf("AppendGlyphOutline full: %v", err)
	}
	smallH := small.maxY - small.minY
	fullH := full.maxY - full.minY
	if smallH >= fullH {
		t.Errorf("small-cap ink height %f, want < full-size 'A' height %f", smallH, fullH)
	}
}

func TestMissingGlyphShapedAsNotdef(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	// U+E000 is a private-use rune DejaVu Sans does not cover.
	glyphs, width := face.ShapeLine("AB", ShapeOptions{Size: 24})
	if len(glyphs) != 3 {
		t.Fatalf("shaped %d glyphs, want 3 (missing rune still rendered as .notdef)", len(glyphs))
	}
	if glyphs[1].Glyph != 0 {
		t.Errorf("missing rune glyph = %d, want 0 (.notdef)", glyphs[1].Glyph)
	}
	if width <= 0 {
		t.Errorf("width = %f, want > 0", width)
	}
}

func TestShapeEmptyLine(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	glyphs, width := face.ShapeLine("", ShapeOptions{Size: 24})
	if len(glyphs) != 0 || width != 0 {
		t.Errorf("empty text: glyphs=%d width=%f, want 0/0", len(glyphs), width)
	}
}

func TestShapeCacheConsistency(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	opts := ShapeOptions{Size: 33, Tracking: 1.25, Kerning: 40}

	first, w1 := face.ShapeLine("Wave AVATAR", opts)
	second, w2 := face.ShapeLine("Wave AVATAR", opts)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("repeated shaping differs:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if w1 != w2 {
		t.Errorf("repeated shaping widths differ: %f vs %f", w1, w2)
	}
}

// TestSegmentsLoadOnce proves the glyph outline cache: loading the same
// glyph outline twice hits sfnt.Font.LoadGlyph only once.
func TestSegmentsLoadOnce(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("Fresh", false, false, dejavusans.TTF); err != nil {
		t.Fatalf("Register: %v", err)
	}
	face := reg.Resolve("Fresh", false, false)
	glyph, err := face.GlyphIndex('Q')
	if err != nil {
		t.Fatalf("GlyphIndex: %v", err)
	}

	a := newBBoxSink()
	if err := face.AppendGlyphOutline(glyph, 24, 0, 0, a); err != nil {
		t.Fatalf("AppendGlyphOutline: %v", err)
	}
	loadsAfterFirst := face.segmentLoads()
	if loadsAfterFirst == 0 {
		t.Fatal("expected at least one LoadGlyph call")
	}

	b := newBBoxSink()
	if err := face.AppendGlyphOutline(glyph, 24, 0, 0, b); err != nil {
		t.Fatalf("AppendGlyphOutline: %v", err)
	}
	if got := face.segmentLoads(); got != loadsAfterFirst {
		t.Errorf("second outline pass hit LoadGlyph again: loads %d -> %d, want cached", loadsAfterFirst, got)
	}
	if *a != *b {
		t.Errorf("cached outline differs from fresh outline: %+v vs %+v", a, b)
	}
}
