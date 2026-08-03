package engine

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestNavigatorThumbnailPreservesAspectAndCachesByContent(t *testing.T) {
	h := Init("")
	defer Free(h)
	created, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Wide", Width: 4, Height: 2, Resolution: 72, ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	result, err := DispatchCommand(h, commandGetNavigatorThumbnail, `{"width":8,"height":8,"background":"transparent"}`)
	if err != nil {
		t.Fatalf("thumbnail: %v", err)
	}
	thumbnail := result.NavigatorThumbnail
	if thumbnail == nil {
		t.Fatal("missing navigator thumbnail")
	}
	if thumbnail.DocumentID != created.UIMeta.ActiveDocumentID || thumbnail.Width != 8 || thumbnail.Height != 4 {
		t.Fatalf("thumbnail metadata = %+v, want active 8x4", thumbnail)
	}
	pixels, err := base64.StdEncoding.DecodeString(thumbnail.RGBA)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(pixels) != 8*4*4 {
		t.Fatalf("pixel length = %d, want %d", len(pixels), 8*4*4)
	}
	inst := instances[h]
	cached := inst.navigatorCache
	if _, err := DispatchCommand(h, commandGetNavigatorThumbnail, `{"width":8,"height":8,"background":"transparent"}`); err != nil {
		t.Fatalf("cached thumbnail: %v", err)
	}
	if inst.navigatorCache != cached {
		t.Fatal("identical request should reuse the cached thumbnail")
	}

	if _, err := DispatchCommand(h, commandAddLayer, mustJSON(t, AddLayerPayload{
		LayerType: LayerTypePixel, Name: "Pixel", Bounds: LayerBounds{W: 1, H: 1}, Pixels: []byte{255, 0, 0, 255},
	})); err != nil {
		t.Fatalf("edit: %v", err)
	}
	updated, err := DispatchCommand(h, commandGetNavigatorThumbnail, `{"width":8,"height":8,"background":"transparent"}`)
	if err != nil {
		t.Fatalf("updated thumbnail: %v", err)
	}
	if updated.NavigatorThumbnail.ContentVersion == thumbnail.ContentVersion {
		t.Fatal("content mutation must invalidate navigator cache")
	}
}

func TestNavigatorThumbnailRejectsMissingDocument(t *testing.T) {
	h := Init("")
	defer Free(h)
	if _, err := DispatchCommand(h, commandGetNavigatorThumbnail, `{"width":100,"height":100}`); err == nil {
		t.Fatal("expected missing document error")
	}
}

func TestNavigatorThumbnailRendersRequestedBackground(t *testing.T) {
	h := Init("")
	defer Free(h)
	if _, err := DispatchCommand(h, commandCreateDocument, mustJSON(t, CreateDocumentPayload{
		Name: "Transparent", Width: 16, Height: 8, Resolution: 72, ColorMode: "rgb", BitDepth: 8, Background: "transparent",
	})); err != nil {
		t.Fatalf("create: %v", err)
	}

	thumbnailPixels := func(background string) []byte {
		t.Helper()
		result, err := DispatchCommand(h, commandGetNavigatorThumbnail, mustJSON(t, NavigatorThumbnailPayload{
			Width: 16, Height: 8, Background: background,
		}))
		if err != nil {
			t.Fatalf("thumbnail %s: %v", background, err)
		}
		pixels, err := base64.StdEncoding.DecodeString(result.NavigatorThumbnail.RGBA)
		if err != nil {
			t.Fatalf("decode %s: %v", background, err)
		}
		return pixels
	}

	transparent := thumbnailPixels("transparent")
	if !bytes.Equal(transparent[:4], []byte{0, 0, 0, 0}) {
		t.Fatalf("transparent background pixel = %v", transparent[:4])
	}
	white := thumbnailPixels("white")
	if !bytes.Equal(white[:4], []byte{255, 255, 255, 255}) {
		t.Fatalf("white background pixel = %v", white[:4])
	}
	checkerboard := thumbnailPixels("checkerboard")
	if checkerboard[3] != 255 || checkerboard[8*4+3] != 255 {
		t.Fatal("checkerboard pixels must be opaque")
	}
	if bytes.Equal(checkerboard[:4], checkerboard[8*4:8*4+4]) {
		t.Fatalf("checkerboard tiles should differ: %v and %v", checkerboard[:4], checkerboard[8*4:8*4+4])
	}
}

func TestNavigatorThumbnailRejectsUnsupportedBackground(t *testing.T) {
	h := Init(mustJSON(t, EngineConfig{DocumentWidth: 2, DocumentHeight: 2, Background: "transparent"}))
	defer Free(h)
	if _, err := DispatchCommand(h, commandGetNavigatorThumbnail, `{"width":2,"height":2,"background":"pink"}`); err == nil {
		t.Fatal("expected unsupported background error")
	}
}
