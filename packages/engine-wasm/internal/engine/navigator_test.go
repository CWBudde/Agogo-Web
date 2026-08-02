package engine

import (
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
