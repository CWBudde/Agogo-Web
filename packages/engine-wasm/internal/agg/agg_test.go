package agg

import (
	"math"
	"testing"

	agglib "github.com/cwbudde/agg_go"
)

func TestRenderViewportBaseTransparentBackgroundUsesCheckerboard(t *testing.T) {
	doc := &Document{Width: 48, Height: 48, Background: "transparent"}
	vp := &Viewport{CenterX: 24, CenterY: 24, Zoom: 1, CanvasW: 48, CanvasH: 48}

	pixels := RenderViewportBase(doc, vp, nil)
	if got, want := len(pixels), 48*48*4; got != want {
		t.Fatalf("len(pixels) = %d, want %d", got, want)
	}

	if got := rgbaAt(pixels, 48, 12, 12); got != [4]uint8{57, 64, 76, 255} {
		t.Fatalf("checkerboard tile A = %v, want [57 64 76 255]", got)
	}
	if got := rgbaAt(pixels, 48, 36, 12); got != [4]uint8{42, 48, 59, 255} {
		t.Fatalf("checkerboard tile B = %v, want [42 48 59 255]", got)
	}
}

func TestRenderViewportBaseSolidBackgroundModesFillDocument(t *testing.T) {
	tests := []struct {
		name       string
		background string
		expect     [4]uint8
	}{
		{name: "white", background: "white", expect: [4]uint8{244, 246, 250, 255}},
		{name: "color", background: "color", expect: [4]uint8{236, 147, 92, 255}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := &Document{Width: 32, Height: 32, Background: test.background}
			vp := &Viewport{CenterX: 16, CenterY: 16, Zoom: 1, CanvasW: 32, CanvasH: 32}

			pixels := RenderViewportBase(doc, vp, nil)
			if got := rgbaAt(pixels, 32, 16, 16); got != test.expect {
				t.Fatalf("solid background pixel = %v, want %v", got, test.expect)
			}
		})
	}
}

func TestRenderViewportOverlaysDrawGuidesAndBorder(t *testing.T) {
	doc := &Document{Width: 48, Height: 48, Background: "white"}
	vp := &Viewport{CenterX: 24, CenterY: 24, Zoom: 1, CanvasW: 48, CanvasH: 48}

	base := RenderViewportBase(doc, vp, nil)
	withOverlays := RenderViewportOverlays(doc, vp, append([]byte(nil), base...))

	guideBefore := rgbaAt(base, 48, 24, 12)
	guideAfter := rgbaAt(withOverlays, 48, 24, 12)
	if guideBefore == guideAfter {
		t.Fatalf("vertical guide pixel did not change: %v", guideAfter)
	}

	borderBefore := rgbaAt(base, 48, 24, 0)
	borderAfter := rgbaAt(withOverlays, 48, 24, 0)
	if borderBefore == borderAfter {
		t.Fatalf("border pixel did not change: %v", borderAfter)
	}
}

func TestVisibleWorldBoundsRespectsViewportTransform(t *testing.T) {
	renderer := agglib.NewAgg2D()
	buffer := make([]byte, 100*50*4)
	renderer.Attach(buffer, 100, 50, 100*4)

	vp := &Viewport{CenterX: 100, CenterY: 50, Zoom: 1, Rotation: 0, CanvasW: 100, CanvasH: 50}
	doc := &Document{Width: 200, Height: 100}

	configureViewportTransform(renderer, 100, 50, vp)
	minX, minY, maxX, maxY := visibleWorldBounds(renderer, 100, 50, doc)

	assertClose(t, "minX", minX, 50)
	assertClose(t, "minY", minY, 25)
	assertClose(t, "maxX", maxX, 150)
	assertClose(t, "maxY", maxY, 75)
}

func rgbaAt(pixels []byte, width, x, y int) [4]uint8 {
	index := (y*width + x) * 4
	return [4]uint8{pixels[index], pixels[index+1], pixels[index+2], pixels[index+3]}
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("%s = %.4f, want %.4f", name, got, want)
	}
}
